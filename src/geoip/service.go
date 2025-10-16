package geoip

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/geoip2-golang"
)

const (
	// GeoIP databases from sapics/ip-location-db via jsdelivr CDN
	cityIPv4URL = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv4.mmdb"
	cityIPv6URL = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv6.mmdb"
	countryURL  = "https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb"
	asnURL      = "https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb"
)

// Service provides GeoIP lookup functionality
type Service struct {
	cityIPv4DB *geoip2.Reader // City database for IPv4
	cityIPv6DB *geoip2.Reader // City database for IPv6
	countryDB  *geoip2.Reader // Country database (combined IPv4/IPv6)
	asnDB      *geoip2.Reader // ASN database (combined IPv4/IPv6)
	dataDir    string
}

// GeoLocation represents the result of a GeoIP lookup
type GeoLocation struct {
	IP           string  `json:"ip"`
	Country      string  `json:"country"`
	CountryName  string  `json:"country_name"`
	Region       string  `json:"region,omitempty"`
	RegionName   string  `json:"region_name,omitempty"`
	City         string  `json:"city,omitempty"`
	PostalCode   string  `json:"postal_code,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	Timezone     string  `json:"timezone,omitempty"`
	ASN          uint    `json:"asn,omitempty"`
	Organization string  `json:"organization,omitempty"`
}

// NewService creates a new GeoIP service
func NewService(configDir string) (*Service, error) {
	dataDir := filepath.Join(configDir, "geoip")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create geoip directory: %w", err)
	}

	s := &Service{
		dataDir: dataDir,
	}

	// Check if databases exist
	if !s.databasesExist() {
		log.Println("GeoIP databases not found, downloading...")
		if err := s.DownloadDatabases(); err != nil {
			return nil, fmt.Errorf("failed to download databases: %w", err)
		}
	} else {
		log.Println("Loading GeoIP databases...")
		if err := s.LoadDatabases(); err != nil {
			return nil, fmt.Errorf("failed to load databases: %w", err)
		}
	}

	log.Println("GeoIP databases loaded successfully")
	return s, nil
}

// databasesExist checks if all required databases exist
func (s *Service) databasesExist() bool {
	files := []string{
		"geolite2-city-ipv4.mmdb",
		"geolite2-city-ipv6.mmdb",
		"geo-whois-asn-country.mmdb",
		"asn.mmdb",
	}

	for _, file := range files {
		path := filepath.Join(s.dataDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// DownloadDatabases downloads all GeoIP databases from the CDN
func (s *Service) DownloadDatabases() error {
	databases := map[string]string{
		"geolite2-city-ipv4.mmdb":    cityIPv4URL,
		"geolite2-city-ipv6.mmdb":    cityIPv6URL,
		"geo-whois-asn-country.mmdb": countryURL,
		"asn.mmdb":                   asnURL,
	}

	for filename, url := range databases {
		path := filepath.Join(s.dataDir, filename)
		log.Printf("  Downloading %s...", filename)
		if err := downloadFile(path, url); err != nil {
			return fmt.Errorf("failed to download %s: %w", filename, err)
		}
	}

	return s.LoadDatabases()
}

// downloadFile downloads a file from a URL to a local path
func downloadFile(filepath string, url string) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// Download the file
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// LoadDatabases loads all GeoIP databases into memory
func (s *Service) LoadDatabases() error {
	var err error

	// Load city IPv4 database
	cityIPv4Path := filepath.Join(s.dataDir, "geolite2-city-ipv4.mmdb")
	s.cityIPv4DB, err = geoip2.Open(cityIPv4Path)
	if err != nil {
		return fmt.Errorf("failed to open city IPv4 database: %w", err)
	}

	// Load city IPv6 database
	cityIPv6Path := filepath.Join(s.dataDir, "geolite2-city-ipv6.mmdb")
	s.cityIPv6DB, err = geoip2.Open(cityIPv6Path)
	if err != nil {
		return fmt.Errorf("failed to open city IPv6 database: %w", err)
	}

	// Load country database (supports both IPv4 and IPv6)
	countryPath := filepath.Join(s.dataDir, "geo-whois-asn-country.mmdb")
	s.countryDB, err = geoip2.Open(countryPath)
	if err != nil {
		return fmt.Errorf("failed to open country database: %w", err)
	}

	// Load ASN database (supports both IPv4 and IPv6)
	asnPath := filepath.Join(s.dataDir, "asn.mmdb")
	s.asnDB, err = geoip2.Open(asnPath)
	if err != nil {
		return fmt.Errorf("failed to open ASN database: %w", err)
	}

	return nil
}

// Lookup performs a GeoIP lookup for the given IP address
func (s *Service) Lookup(ip net.IP) (*GeoLocation, error) {
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address")
	}

	location := &GeoLocation{
		IP: ip.String(),
	}

	// Determine which city database to use based on IP version
	var cityDB *geoip2.Reader
	if ip.To4() != nil {
		// IPv4 address - use IPv4 city database
		cityDB = s.cityIPv4DB
	} else {
		// IPv6 address - use IPv6 city database
		cityDB = s.cityIPv6DB
	}

	// Try city lookup first
	city, err := cityDB.City(ip)
	if err == nil {
		// City lookup successful
		location.Country = city.Country.IsoCode
		location.CountryName = city.Country.Names["en"]
		if len(city.Subdivisions) > 0 {
			location.Region = city.Subdivisions[0].IsoCode
			location.RegionName = city.Subdivisions[0].Names["en"]
		}
		location.City = city.City.Names["en"]
		location.PostalCode = city.Postal.Code
		location.Latitude = city.Location.Latitude
		location.Longitude = city.Location.Longitude
		location.Timezone = city.Location.TimeZone
	} else {
		// Fallback to country database
		country, err := s.countryDB.Country(ip)
		if err != nil {
			return nil, fmt.Errorf("lookup failed: %w", err)
		}
		location.Country = country.Country.IsoCode
		location.CountryName = country.Country.Names["en"]
	}

	// Look up ASN information
	asn, err := s.asnDB.ASN(ip)
	if err == nil {
		location.ASN = asn.AutonomousSystemNumber
		location.Organization = asn.AutonomousSystemOrganization
	}

	return location, nil
}

// UpdateDatabases re-downloads all databases
func (s *Service) UpdateDatabases() error {
	log.Println("Updating GeoIP databases...")

	// Close existing databases
	if s.cityIPv4DB != nil {
		s.cityIPv4DB.Close()
	}
	if s.cityIPv6DB != nil {
		s.cityIPv6DB.Close()
	}
	if s.countryDB != nil {
		s.countryDB.Close()
	}
	if s.asnDB != nil {
		s.asnDB.Close()
	}

	// Download new databases
	if err := s.DownloadDatabases(); err != nil {
		// Try to reload old databases if download fails
		loadErr := s.LoadDatabases()
		if loadErr != nil {
			return fmt.Errorf("update failed and could not reload old databases: %w", err)
		}
		return fmt.Errorf("update failed but old databases still loaded: %w", err)
	}

	log.Println("GeoIP databases updated successfully")
	return nil
}

// Close closes all database connections
func (s *Service) Close() error {
	if s.cityIPv4DB != nil {
		s.cityIPv4DB.Close()
	}
	if s.cityIPv6DB != nil {
		s.cityIPv6DB.Close()
	}
	if s.countryDB != nil {
		s.countryDB.Close()
	}
	if s.asnDB != nil {
		s.asnDB.Close()
	}
	return nil
}
