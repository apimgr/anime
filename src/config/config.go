package config

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/anime/src/common/i18n"
)

// Config represents the complete server configuration
type Config struct {
	ConfigDir   string            `yaml:"-"`
	Server      ServerConfig      `yaml:"server"`
	WebUI       WebUIConfig       `yaml:"web-ui"`
	WebRobots   WebRobotsConfig   `yaml:"web-robots"`
	WebSecurity WebSecurityConfig `yaml:"web-security"`
	WebLLMS     WebLLMSConfig     `yaml:"web-llms"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	Port               string                    `yaml:"port"`
	Token              string                    `yaml:"token"`
	FQDN               string                    `yaml:"fqdn"`
	Address            string                    `yaml:"address"`
	BaseURL            string                    `yaml:"baseurl"`
	Mode               string                    `yaml:"mode"`
	Update             UpdateConfig              `yaml:"update"`
	LegacyUpdateBranch string                    `yaml:"update_branch,omitempty"`
	Schedule           ScheduleConfig            `yaml:"schedule"`
	Metrics            MetricsConfig             `yaml:"metrics"`
	Logging            LoggingConfig             `yaml:"logging"`
	Database           DatabaseConfig            `yaml:"database"`
	Notifications      ServerNotificationsConfig `yaml:"notifications"`
	Security           ServerSecurityConfig      `yaml:"security"`
	Privacy            PrivacyConfig             `yaml:"privacy"`
	Contact            ContactConfig             `yaml:"contact"`
	Pages              PagesConfig               `yaml:"pages"`
	I18n               I18nConfig                `yaml:"i18n"`
	Branding           BrandingConfig            `yaml:"branding"`
	SEO                SEOConfig                 `yaml:"seo"`
	Tokens             TokensConfig              `yaml:"tokens"`
	RateLimit          RateLimitConfig           `yaml:"rate_limit"`
}

type I18nConfig struct {
	DefaultLanguage    string   `yaml:"default_language"`
	AvailableLanguages []string `yaml:"available_languages"`
}

type BrandingConfig struct {
	Title       string                         `yaml:"title"`
	Tagline     string                         `yaml:"tagline"`
	Description string                         `yaml:"description"`
	Favicon     string                         `yaml:"favicon"`
	Logo        string                         `yaml:"logo"`
	Content     map[string]BrandingPageContent `yaml:"content"`
}

type BrandingPageContent struct {
	Format string               `yaml:"format"`
	Body   LocalizedContentBody `yaml:"body"`
}

type SEOConfig struct {
	Keywords       []string              `yaml:"keywords"`
	Author         string                `yaml:"author"`
	OGImage        string                `yaml:"og_image"`
	TwitterHandle  string                `yaml:"twitter_handle"`
	TitleSeparator string                `yaml:"title_separator"`
	Canonical      string                `yaml:"canonical"`
	Noindex        Boolean               `yaml:"noindex"`
	MetaTags       map[string]string     `yaml:"meta_tags"`
	OpenGraph      OpenGraphSEOConfig    `yaml:"opengraph"`
	Twitter        TwitterSEOConfig      `yaml:"twitter"`
	Verification   VerificationSEOConfig `yaml:"verification"`
	Sitemap        SitemapSEOConfig      `yaml:"sitemap"`
	Feed           FeedSEOConfig         `yaml:"feed"`
}

type SitemapSEOConfig struct {
	MaxURLs       int  `yaml:"max_urls"`
	IncludeImages bool `yaml:"include_images"`
}

type FeedSEOConfig struct {
	MaxItems int    `yaml:"max_items"`
	Title    string `yaml:"title"`
}

type VerificationSEOConfig struct {
	Google    string                   `yaml:"google"`
	Bing      string                   `yaml:"bing"`
	Yandex    string                   `yaml:"yandex"`
	Baidu     string                   `yaml:"baidu"`
	Pinterest string                   `yaml:"pinterest"`
	Facebook  string                   `yaml:"facebook"`
	Custom    []CustomVerificationMeta `yaml:"custom"`
}

type CustomVerificationMeta struct {
	Name     string `yaml:"name"`
	Property string `yaml:"property"`
	Content  string `yaml:"content"`
}

type OpenGraphSEOConfig struct {
	Enabled  Boolean `yaml:"enabled"`
	Type     string  `yaml:"type"`
	SiteName string  `yaml:"site_name"`
}

type TwitterSEOConfig struct {
	Enabled Boolean `yaml:"enabled"`
	Card    string  `yaml:"card"`
	Creator string  `yaml:"creator"`
}

type RateLimitConfig struct {
	Read   RateLimitWindow `yaml:"read"`
	Strict RateLimitWindow `yaml:"strict"`
}

type RateLimitWindow struct {
	Requests int `yaml:"requests"`
	Window   int `yaml:"window"`
}

type TokensConfig struct {
	MaxLinked     int    `yaml:"max_linked"`
	CookieMaxAge  string `yaml:"cookie_max_age"`
	IdlePurgeDays int    `yaml:"idle_purge_days"`
}

// ContactConfig contains public privacy contact details.
type ContactConfig struct {
	DPO      DPOContactConfig     `yaml:"dpo"`
	General  GeneralContactConfig `yaml:"general"`
	Abuse    GeneralContactConfig `yaml:"abuse"`
	Admin    GeneralContactConfig `yaml:"admin"`
	Security GeneralContactConfig `yaml:"security"`
}

type ServerSecurityConfig struct {
	EncryptionKey        string                `yaml:"encryption_key"`
	EncryptionKeyVersion int                   `yaml:"encryption_key_version"`
	Reports              SecurityReportsConfig `yaml:"reports"`
}

type SecurityReportsConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

type PrivacyConfig struct {
	Data       PrivacyDataConfig       `yaml:"data"`
	Retention  PrivacyRetentionConfig  `yaml:"retention"`
	Consent    PrivacyConsentConfig    `yaml:"consent"`
	Cookies    PrivacyCookieCategories `yaml:"cookies"`
	ThirdParty PrivacyThirdPartyConfig `yaml:"third_party"`
}

type PrivacyDataConfig struct {
	Sold           bool                      `yaml:"sold"`
	StoredOnServer bool                      `yaml:"stored_on_server"`
	Sharing        []PrivacySharingCondition `yaml:"sharing"`
}

type PrivacySharingCondition struct {
	Condition string `yaml:"condition"`
	When      string `yaml:"when"`
	Data      string `yaml:"data"`
}

type PrivacyRetentionConfig struct {
	Period            string `yaml:"period"`
	ExportAvailable   bool   `yaml:"export_available"`
	DeletionAvailable bool   `yaml:"deletion_available"`
}

type PrivacyConsentConfig struct {
	ShowUntilAcknowledged bool   `yaml:"show_until_acknowledged"`
	Message               string `yaml:"message"`
	MessageIfSold         string `yaml:"message_if_sold"`
	Policy                struct {
		Text string `yaml:"text"`
		URL  string `yaml:"url"`
	} `yaml:"policy"`
	Buttons struct {
		Decline string `yaml:"decline"`
		Accept  string `yaml:"accept"`
	} `yaml:"buttons"`
	Position        string `yaml:"position"`
	ShowPreferences bool   `yaml:"show_preferences"`
	PreferencesText string `yaml:"preferences_text"`
}

type PrivacyCookieCategories struct {
	Essential   PrivacyCookieCategory  `yaml:"essential"`
	Preferences PrivacyCookieCategory  `yaml:"preferences"`
	Analytics   PrivacyAnalyticsCookie `yaml:"analytics"`
}

type PrivacyCookieCategory struct {
	Enabled     bool   `yaml:"enabled"`
	Description string `yaml:"description"`
}

type PrivacyAnalyticsCookie struct {
	PrivacyCookieCategory    `yaml:",inline"`
	DescriptionSuffixNotSold string `yaml:"description_suffix_not_sold"`
	DescriptionSuffixSold    string `yaml:"description_suffix_sold"`
}

type PrivacyThirdPartyConfig struct {
	Services []PrivacyThirdPartyService `yaml:"services"`
}

type PrivacyThirdPartyService struct {
	Name      string `yaml:"name"`
	Purpose   string `yaml:"purpose"`
	DataSent  string `yaml:"data_sent"`
	PolicyURL string `yaml:"policy_url"`
}

type PagesConfig struct {
	Contact ContactPageConfig `yaml:"contact"`
}

type ContactPageConfig struct {
	Enabled          Boolean               `yaml:"enabled"`
	Captcha          string                `yaml:"captcha"`
	CaptchaSiteKey   string                `yaml:"captcha_site_key"`
	CaptchaSecretKey string                `yaml:"captcha_secret_key"`
	Antispam         ContactAntispamConfig `yaml:"antispam"`
	SuccessMessage   string                `yaml:"success_message"`
}

type ContactAntispamConfig struct {
	Honeypot             Boolean  `yaml:"honeypot"`
	MinFillSeconds       int      `yaml:"min_fill_seconds"`
	MaxAgeSeconds        int      `yaml:"max_age_seconds"`
	PerIP                int      `yaml:"per_ip"`
	PerEmail             int      `yaml:"per_email"`
	Global               int      `yaml:"global"`
	Window               string   `yaml:"window"`
	MaxLinks             int      `yaml:"max_links"`
	RejectMarkup         Boolean  `yaml:"reject_markup"`
	BlockedWords         []string `yaml:"blocked_words"`
	BlockedEmailDomains  []string `yaml:"blocked_email_domains"`
	Strikes              int      `yaml:"strikes"`
	BanHours             int      `yaml:"ban_hours"`
	ReviewMinSeconds     int      `yaml:"review_min_seconds"`
	ReviewMaxSeconds     int      `yaml:"review_max_seconds"`
	DuplicateWindowHours int      `yaml:"duplicate_window_hours"`
}

// DPOContactConfig contains the configured data protection officer contact.
type DPOContactConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// GeneralContactConfig contains the public general contact fallback.
type GeneralContactConfig struct {
	Email    string                `yaml:"email"`
	Webhooks ContactWebhooksConfig `yaml:"webhooks"`
}

type ContactWebhooksConfig struct {
	Telegram string `yaml:"telegram"`
	Discord  string `yaml:"discord"`
	Slack    string `yaml:"slack"`
	Generic  string `yaml:"generic"`
}

type ServerNotificationsConfig struct {
	Email EmailConfig `yaml:"email"`
}

type EmailConfig struct {
	SMTP EmailSMTPConfig `yaml:"smtp"`
	From EmailFromConfig `yaml:"from"`
}

type EmailSMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	TLS      string `yaml:"tls"`
}

type EmailFromConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

type DatabaseConfig struct {
	Driver string       `yaml:"driver"`
	URL    string       `yaml:"url"`
	Pool   DatabasePool `yaml:"pool"`
}

type DatabasePool struct {
	MaxOpen     int    `yaml:"max_open"`
	MaxIdle     int    `yaml:"max_idle"`
	MaxLifetime string `yaml:"max_lifetime"`
	MaxIdleTime string `yaml:"max_idle_time"`
}

type UpdateConfig struct {
	Branch        string  `yaml:"branch"`
	AutoInstall   Boolean `yaml:"auto_install"`
	DeferDays     int     `yaml:"defer_days"`
	CLIMinVersion string  `yaml:"cli_min_version"`
}

// ScheduleConfig contains scheduler settings
type ScheduleConfig struct {
	Enabled       Boolean `yaml:"enabled"`
	Notifications string  `yaml:"notifications"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled       Boolean `yaml:"enabled"`
	Endpoint      string  `yaml:"endpoint"`
	IncludeSystem Boolean `yaml:"include_system"`
	IncludeApp    Boolean `yaml:"include_app"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	AccessFormat string `yaml:"access_format"`
	Level        string `yaml:"level"`
}

// WebUIConfig contains web UI settings
type WebUIConfig struct {
	Theme         string              `yaml:"theme"`
	Logo          string              `yaml:"logo"`
	Favicon       string              `yaml:"favicon"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

// NotificationsConfig contains notification settings
type NotificationsConfig struct {
	Enabled       Boolean  `yaml:"enabled"`
	Announcements []string `yaml:"announcements"`
}

// WebRobotsConfig contains robots.txt settings
type WebRobotsConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// WebLLMSConfig controls AI discovery output.
type WebLLMSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	IncludeEndpoints bool     `yaml:"include_endpoints"`
	IncludeSchemas   bool     `yaml:"include_schemas"`
	CustomSections   []string `yaml:"custom_sections"`
}

// WebSecurityConfig contains security settings
type WebSecurityConfig struct {
	Admin         string   `yaml:"admin"`
	CORS          string   `yaml:"cors"`
	ReportURL     string   `yaml:"report_url"`
	Contact       string   `yaml:"contact"`
	Policy        string   `yaml:"policy"`
	Keyservers    []string `yaml:"keyservers"`
	PublishPGPKey Boolean  `yaml:"publish_pgp_key"`
}

var (
	current    *Config
	mu         sync.RWMutex
	configPath string
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    "",
			FQDN:    "",
			Address: "[::]",
			BaseURL: "/",
			Mode:    "production",
			I18n:    I18nConfig{DefaultLanguage: "en", AvailableLanguages: i18n.Languages()},
			Branding: BrandingConfig{
				Title:       "anime",
				Description: "A full-stack application serving anime quotes from a curated collection through a public read-only API and browser interface.",
			},
			SEO: SEOConfig{
				Keywords: []string{"anime", "quotes", "api", "anime quotes", "random quotes"}, Author: "apimgr", TitleSeparator: " - ",
				OpenGraph: OpenGraphSEOConfig{Enabled: true, Type: "website"},
				Twitter:   TwitterSEOConfig{Enabled: true, Card: "summary_large_image"},
				Sitemap:   SitemapSEOConfig{MaxURLs: 50000},
				Feed:      FeedSEOConfig{MaxItems: 50},
			},
			Privacy: PrivacyConfig{
				Data:      PrivacyDataConfig{StoredOnServer: true, Sharing: []PrivacySharingCondition{{Condition: "user_initiated", When: "When a visitor submits a report", Data: "The name, reply address, and report content supplied by the visitor."}}},
				Retention: PrivacyRetentionConfig{Period: "Quote data is embedded in the application. Browser preferences remain in first-party cookies. Operational log retention is controlled by the operator's deployment configuration.", ExportAvailable: true, DeletionAvailable: true},
				Consent: PrivacyConsentConfig{ShowUntilAcknowledged: true, Position: "bottom", ShowPreferences: true, PreferencesText: "Manage preferences", Policy: struct {
					Text string `yaml:"text"`
					URL  string `yaml:"url"`
				}{Text: "Privacy policy", URL: "/server/privacy"}, Buttons: struct {
					Decline string `yaml:"decline"`
					Accept  string `yaml:"accept"`
				}{Decline: "Decline", Accept: "I Agree"}},
				Cookies:    PrivacyCookieCategories{Essential: PrivacyCookieCategory{Enabled: true, Description: "Required for browser form security and service operation."}, Preferences: PrivacyCookieCategory{Enabled: true, Description: "Remember theme and language choices in this browser."}, Analytics: PrivacyAnalyticsCookie{PrivacyCookieCategory: PrivacyCookieCategory{Enabled: true, Description: "Analytics tracking is not configured for this service."}, DescriptionSuffixNotSold: "Analytics data is anonymized and never sold.", DescriptionSuffixSold: "Analytics data may be shared with third parties."}},
				ThirdParty: PrivacyThirdPartyConfig{Services: []PrivacyThirdPartyService{}},
			},
			Security: ServerSecurityConfig{Reports: SecurityReportsConfig{RetentionDays: 365}},
			Pages: PagesConfig{Contact: ContactPageConfig{
				Enabled: true, Captcha: "simple", SuccessMessage: "Thank you for your message. We'll respond soon.",
				Antispam: ContactAntispamConfig{Honeypot: true, MinFillSeconds: 3, MaxAgeSeconds: 43200, PerIP: 5, PerEmail: 3, Global: 60, Window: "1h", MaxLinks: 10, RejectMarkup: true, BlockedWords: []string{}, BlockedEmailDomains: []string{}, Strikes: 3, BanHours: 24, ReviewMinSeconds: 2, ReviewMaxSeconds: 900, DuplicateWindowHours: 24},
			}},
			Tokens:    TokensConfig{MaxLinked: 25, CookieMaxAge: "1y", IdlePurgeDays: 0},
			RateLimit: RateLimitConfig{Read: RateLimitWindow{Requests: 120, Window: 60}, Strict: RateLimitWindow{Requests: 10, Window: 900}},
			Database: DatabaseConfig{
				Driver: "sqlite",
				Pool: DatabasePool{
					MaxOpen: 25, MaxIdle: 5, MaxLifetime: "5m", MaxIdleTime: "1m",
				},
			},
			Notifications: ServerNotificationsConfig{Email: EmailConfig{SMTP: EmailSMTPConfig{Port: 587, TLS: "auto"}}},
			Update:        UpdateConfig{Branch: "stable"},
			Schedule: ScheduleConfig{
				Enabled:       true,
				Notifications: "hourly",
			},
			Metrics: MetricsConfig{
				Enabled:       false,
				Endpoint:      "/metrics",
				IncludeSystem: true,
				IncludeApp:    true,
			},
			Logging: LoggingConfig{
				AccessFormat: "apache",
				Level:        "info",
			},
		},
		WebUI: WebUIConfig{
			Theme:   "dark",
			Logo:    "",
			Favicon: "",
			Notifications: NotificationsConfig{
				Enabled:       true,
				Announcements: []string{},
			},
		},
		WebRobots: WebRobotsConfig{
			Allow: []string{"/", "/api"},
			Deny:  []string{"/debug"},
		},
		WebSecurity: WebSecurityConfig{
			Admin:         "",
			CORS:          "*",
			ReportURL:     "https://github.com/apimgr/anime/security/advisories/new",
			Contact:       "",
			Policy:        "",
			Keyservers:    []string{},
			PublishPGPKey: false,
		},
		WebLLMS: WebLLMSConfig{Enabled: true, IncludeEndpoints: true},
	}
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	// Migrate .yaml to .yml if needed
	path = migrateYamlToYml(path)
	configPath = path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		cfg.ConfigDir = filepath.Dir(path)
		port, err := SelectAvailablePort()
		if err != nil {
			return nil, fmt.Errorf("failed to select a default server port: %w", err)
		}
		cfg.Server.Port = port
		token, err := GenerateOperatorToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate operator token: %w", err)
		}
		cfg.Server.Token = token
		if err := loadOrCreateSecrets(path, cfg); err != nil {
			return nil, err
		}
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		if err := migrateAreaFiles(path); err != nil {
			log.Printf("configuration area migration skipped: %v", err)
		}
		applySMTPEnvironment(cfg)
		if err := validateServerConfig(cfg); err != nil {
			return nil, err
		}
		current = cfg
		return cfg, nil
	}

	cfg := DefaultConfig()
	cfg.ConfigDir = filepath.Dir(path)
	cfg.Server.Update.Branch = ""
	if err := unmarshalMergedConfig(loadMergedValues(path), cfg); err != nil {
		log.Printf("configuration values could not be decoded; using built-in defaults: %v", err)
		cfg = DefaultConfig()
		cfg.ConfigDir = filepath.Dir(path)
	}
	if err := loadOrCreateSecrets(path, cfg); err != nil {
		return nil, err
	}
	if err := migrateAreaFiles(path); err != nil {
		log.Printf("configuration area migration skipped: %v", err)
	}
	applySMTPEnvironment(cfg)
	if err := validateServerConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = cfg.Server.LegacyUpdateBranch
	}
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = "stable"
	}
	needsSave := false
	if strings.TrimSpace(cfg.Server.Port) == "" {
		port, err := SelectAvailablePort()
		if err != nil {
			return nil, fmt.Errorf("failed to select a default server port: %w", err)
		}
		cfg.Server.Port = port
		needsSave = true
	}
	if strings.TrimSpace(cfg.Server.Token) == "" {
		token, err := GenerateOperatorToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate operator token: %w", err)
		}
		cfg.Server.Token = token
		needsSave = true
	}
	if needsSave {
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to save generated configuration defaults: %w", err)
		}
	}
	cfg.Server.LegacyUpdateBranch = ""

	current = cfg
	return cfg, nil
}

// GenerateOperatorToken returns a fresh server-wide token using the required base62 alphabet.
func GenerateOperatorToken() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const tokenLength = 32
	token := make([]byte, tokenLength)
	limit := big.NewInt(int64(len(alphabet)))
	for i := range token {
		value, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		token[i] = alphabet[value.Int64()]
	}
	return "adm_" + string(token), nil
}

// SelectAvailablePort returns an unused port in the default 64000–64999 range.
func SelectAvailablePort() (string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		var randomValue [4]byte
		if _, err := rand.Read(randomValue[:]); err != nil {
			return "", fmt.Errorf("generate random port candidate: %w", err)
		}
		randomNumber := int(randomValue[0])*256 + int(randomValue[1])
		portNumber := 64000 + randomNumber%1000
		listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(portNumber)))
		if err != nil {
			continue
		}
		if err := listener.Close(); err != nil {
			return "", fmt.Errorf("release default port candidate: %w", err)
		}
		return strconv.Itoa(portNumber), nil
	}
	return "", fmt.Errorf("no unused port found in default range 64000–64999")
}

// migrateYamlToYml migrates config from .yaml to .yml extension
func migrateYamlToYml(path string) string {
	// If path ends with .yaml, check if we need to migrate
	if strings.HasSuffix(path, ".yaml") {
		ymlPath := strings.TrimSuffix(path, ".yaml") + ".yml"
		// If .yml doesn't exist but .yaml does, rename
		if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
			if _, err := os.Stat(path); err == nil {
				if err := os.Rename(path, ymlPath); err == nil {
					fmt.Printf("Migrated config from %s to %s\n", path, ymlPath)
				}
			}
		}
		return ymlPath
	}
	// If path ends with .yml, check for .yaml to migrate
	if strings.HasSuffix(path, ".yml") {
		yamlPath := strings.TrimSuffix(path, ".yml") + ".yaml"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if _, err := os.Stat(yamlPath); err == nil {
				if err := os.Rename(yamlPath, path); err == nil {
					fmt.Printf("Migrated config from %s to %s\n", yamlPath, path)
				}
			}
		}
	}
	return path
}

// Get returns the current configuration
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return DefaultConfig()
	}
	return current
}

// Save saves the specified configuration to the specified path
func Save(path string, cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	current = cfg
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = cfg.Server.LegacyUpdateBranch
	}
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = "stable"
	}
	cfg.Server.LegacyUpdateBranch = ""
	if path != "" {
		configPath = path
	}
	return saveConfig(cfg, path)
}

// SaveCurrent saves the current configuration to file
func SaveCurrent() error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil || configPath == "" {
		return fmt.Errorf("no configuration loaded")
	}
	return saveConfig(current, configPath)
}

// SetPort updates the port in configuration
func SetPort(port string) error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = DefaultConfig()
	}
	current.Server.Port = port
	if configPath != "" {
		return saveConfig(current, configPath)
	}
	return nil
}

// saveConfig writes configuration to a YAML file
func saveConfig(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	dirMode := os.FileMode(0700)
	fileMode := os.FileMode(0600)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	content := generateConfigYAML(cfg)
	if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	if err := os.Chmod(path, fileMode); err != nil {
		return fmt.Errorf("failed to set configuration file permissions: %w", err)
	}

	return nil
}

func applySMTPEnvironment(cfg *Config) {
	if value, ok := os.LookupEnv("SMTP_HOST"); ok {
		cfg.Server.Notifications.Email.SMTP.Host = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("SMTP_PORT"); ok {
		if port, err := strconv.Atoi(value); err == nil && port > 0 && port <= 65535 {
			cfg.Server.Notifications.Email.SMTP.Port = port
		}
	}
	if value, ok := os.LookupEnv("SMTP_USERNAME"); ok {
		cfg.Server.Notifications.Email.SMTP.Username = value
	}
	if value, ok := os.LookupEnv("SMTP_PASSWORD"); ok {
		cfg.Server.Notifications.Email.SMTP.Password = value
	}
	if value, ok := os.LookupEnv("SMTP_TLS"); ok {
		cfg.Server.Notifications.Email.SMTP.TLS = strings.ToLower(strings.TrimSpace(value))
	}
	if value, ok := os.LookupEnv("SMTP_FROM_NAME"); ok {
		cfg.Server.Notifications.Email.From.Name = value
	}
	if value, ok := os.LookupEnv("SMTP_FROM_EMAIL"); ok {
		cfg.Server.Notifications.Email.From.Email = strings.TrimSpace(value)
	}
}

func validateServerConfig(cfg *Config) error {
	validateI18nConfig(&cfg.Server.I18n)
	validatePrivacyConfig(&cfg.Server.Privacy)
	if strings.TrimSpace(cfg.Server.Branding.Title) == "" {
		cfg.Server.Branding.Title = DefaultConfig().Server.Branding.Title
	}
	if strings.TrimSpace(cfg.Server.Branding.Description) == "" {
		cfg.Server.Branding.Description = DefaultConfig().Server.Branding.Description
	}
	if cfg.Server.SEO.TitleSeparator == "" {
		cfg.Server.SEO.TitleSeparator = " - "
	}
	if len(cfg.Server.SEO.TitleSeparator) > 8 {
		log.Printf("invalid configuration server.seo.title_separator; using default")
		cfg.Server.SEO.TitleSeparator = " - "
	}
	filteredMetaTags := make(map[string]string, min(len(cfg.Server.SEO.MetaTags), 20))
	for name, content := range cfg.Server.SEO.MetaTags {
		if !validMetaTagName(name) || len(filteredMetaTags) >= 20 {
			log.Printf("invalid configuration server.seo.meta_tags; invalid or excess entry ignored")
			continue
		}
		filteredMetaTags[name] = content
	}
	cfg.Server.SEO.MetaTags = filteredMetaTags
	validateVerificationCode(&cfg.Server.SEO.Verification.Google, "server.seo.verification.google", func(value string) bool {
		return len(value) <= 43 && asciiRunes(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-")
	})
	validateVerificationCode(&cfg.Server.SEO.Verification.Bing, "server.seo.verification.bing", func(value string) bool {
		return len(value) <= 32 && asciiRunes(value, "ABCDEF0123456789")
	})
	validateVerificationCode(&cfg.Server.SEO.Verification.Yandex, "server.seo.verification.yandex", func(value string) bool {
		return len(value) <= 32 && asciiRunes(value, "abcdef0123456789")
	})
	validateVerificationCode(&cfg.Server.SEO.Verification.Baidu, "server.seo.verification.baidu", func(value string) bool {
		return len(value) <= 32 && asciiRunes(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	})
	validateVerificationCode(&cfg.Server.SEO.Verification.Pinterest, "server.seo.verification.pinterest", func(value string) bool {
		return len(value) <= 32 && asciiRunes(value, "abcdef0123456789")
	})
	validateVerificationCode(&cfg.Server.SEO.Verification.Facebook, "server.seo.verification.facebook", func(value string) bool {
		return len(value) <= 64 && asciiRunes(value, "abcdefghijklmnopqrstuvwxyz0123456789")
	})
	validCustom := make([]CustomVerificationMeta, 0, len(cfg.Server.SEO.Verification.Custom))
	for _, tag := range cfg.Server.SEO.Verification.Custom {
		name := tag.Name != "" && tag.Property == "" && validVerificationAttribute(tag.Name)
		property := tag.Property != "" && tag.Name == "" && validVerificationAttribute(tag.Property)
		if (!name && !property) || tag.Content == "" || len(tag.Content) > 256 {
			log.Printf("invalid configuration server.seo.verification.custom; invalid tag ignored")
			continue
		}
		validCustom = append(validCustom, tag)
	}
	cfg.Server.SEO.Verification.Custom = validCustom
	if cfg.Server.SEO.Canonical != "" {
		canonicalURL, err := url.Parse(cfg.Server.SEO.Canonical)
		if err != nil || canonicalURL.Scheme != "https" || canonicalURL.Host == "" || canonicalURL.User != nil {
			log.Printf("invalid configuration server.seo.canonical; using request origin")
			cfg.Server.SEO.Canonical = ""
		}
	}
	if cfg.Server.SEO.OpenGraph.Type == "" {
		cfg.Server.SEO.OpenGraph.Type = "website"
	}
	if cfg.Server.SEO.Twitter.Card == "" {
		cfg.Server.SEO.Twitter.Card = "summary_large_image"
	}
	if cfg.Server.SEO.Sitemap.MaxURLs < 1 || cfg.Server.SEO.Sitemap.MaxURLs > 50000 {
		log.Printf("invalid configuration server.seo.sitemap.max_urls; using default 50000")
		cfg.Server.SEO.Sitemap.MaxURLs = 50000
	}
	if cfg.Server.SEO.Feed.MaxItems < 1 || cfg.Server.SEO.Feed.MaxItems > 200 {
		log.Printf("invalid configuration server.seo.feed.max_items; using default 50")
		cfg.Server.SEO.Feed.MaxItems = 50
	}
	if cfg.Server.Security.Reports.RetentionDays <= 0 {
		cfg.Server.Security.Reports.RetentionDays = 365
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Server.Pages.Contact.Captcha))
	if provider == "" {
		provider = "simple"
		cfg.Server.Pages.Contact.Captcha = provider
	}
	if provider != "simple" && provider != "recaptcha" && provider != "hcaptcha" {
		log.Printf("invalid configuration server.pages.contact.captcha; using default simple")
		provider = "simple"
		cfg.Server.Pages.Contact.Captcha = provider
	}
	if provider != "simple" && (strings.TrimSpace(cfg.Server.Pages.Contact.CaptchaSiteKey) == "" || strings.TrimSpace(cfg.Server.Pages.Contact.CaptchaSecretKey) == "") {
		log.Printf("invalid configuration server.pages.contact.captcha; missing provider keys, using default simple")
		cfg.Server.Pages.Contact.Captcha = "simple"
	}
	window := strings.TrimSpace(cfg.Server.Pages.Contact.Antispam.Window)
	if window == "" {
		window = "1h"
		cfg.Server.Pages.Contact.Antispam.Window = window
	}
	if _, err := time.ParseDuration(window); err != nil {
		log.Printf("invalid configuration server.pages.contact.antispam.window; using default 1h")
		cfg.Server.Pages.Contact.Antispam.Window = "1h"
	}
	if cfg.Server.Pages.Contact.Antispam.MinFillSeconds < 0 || cfg.Server.Pages.Contact.Antispam.MaxAgeSeconds < 0 || cfg.Server.Pages.Contact.Antispam.MaxLinks < 0 || cfg.Server.Pages.Contact.Antispam.Strikes < 0 || cfg.Server.Pages.Contact.Antispam.BanHours < 0 || cfg.Server.Pages.Contact.Antispam.ReviewMinSeconds < 0 || cfg.Server.Pages.Contact.Antispam.ReviewMaxSeconds < 0 || cfg.Server.Pages.Contact.Antispam.DuplicateWindowHours < 0 {
		log.Printf("invalid configuration server.pages.contact.antispam; negative values use defaults")
		cfg.Server.Pages.Contact.Antispam = DefaultConfig().Server.Pages.Contact.Antispam
	}
	tlsMode := strings.ToLower(strings.TrimSpace(cfg.Server.Notifications.Email.SMTP.TLS))
	if tlsMode == "" {
		tlsMode = "auto"
		cfg.Server.Notifications.Email.SMTP.TLS = tlsMode
	}
	if tlsMode != "auto" && tlsMode != "starttls" && tlsMode != "tls" && tlsMode != "none" {
		log.Printf("invalid configuration server.notifications.email.smtp.tls; using default auto")
		cfg.Server.Notifications.Email.SMTP.TLS = "auto"
	}
	port := cfg.Server.Notifications.Email.SMTP.Port
	if port < 0 || port > 65535 {
		log.Printf("invalid configuration server.notifications.email.smtp.port; using default 587")
		cfg.Server.Notifications.Email.SMTP.Port = 587
	}
	return nil
}

func validatePrivacyConfig(privacy *PrivacyConfig) {
	defaults := DefaultConfig().Server.Privacy
	if privacy.Consent.Position != "top" && privacy.Consent.Position != "bottom" {
		log.Printf("invalid configuration server.privacy.consent.position; using default bottom")
		privacy.Consent.Position = "bottom"
	}
	policyURL := strings.TrimSpace(privacy.Consent.Policy.URL)
	if policyURL == "" {
		privacy.Consent.Policy.URL = defaults.Consent.Policy.URL
	} else {
		parsed, err := url.Parse(policyURL)
		validRelative := err == nil && !parsed.IsAbs() && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(parsed.Path, "//")
		validAbsolute := err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
		if !validRelative && !validAbsolute {
			log.Printf("invalid configuration server.privacy.consent.policy.url; using default")
			privacy.Consent.Policy.URL = defaults.Consent.Policy.URL
		}
	}
	if privacy.Consent.Policy.Text == "" {
		privacy.Consent.Policy.Text = defaults.Consent.Policy.Text
	}
	if privacy.Consent.Buttons.Decline == "" {
		privacy.Consent.Buttons.Decline = defaults.Consent.Buttons.Decline
	}
	if privacy.Consent.Buttons.Accept == "" {
		privacy.Consent.Buttons.Accept = defaults.Consent.Buttons.Accept
	}
	if privacy.Retention.Period == "" {
		privacy.Retention.Period = defaults.Retention.Period
	}
	if privacy.Cookies.Essential.Description == "" {
		privacy.Cookies.Essential = defaults.Cookies.Essential
	}
	privacy.Cookies.Essential.Enabled = true
	if privacy.Cookies.Preferences.Description == "" {
		privacy.Cookies.Preferences = defaults.Cookies.Preferences
	}
	if privacy.Cookies.Analytics.Description == "" {
		privacy.Cookies.Analytics = defaults.Cookies.Analytics
	}
	if privacy.Cookies.Analytics.DescriptionSuffixNotSold == "" {
		privacy.Cookies.Analytics.DescriptionSuffixNotSold = defaults.Cookies.Analytics.DescriptionSuffixNotSold
	}
	if privacy.Cookies.Analytics.DescriptionSuffixSold == "" {
		privacy.Cookies.Analytics.DescriptionSuffixSold = defaults.Cookies.Analytics.DescriptionSuffixSold
	}
}

func validateVerificationCode(value *string, key string, valid func(string) bool) {
	if *value != "" && !valid(*value) {
		log.Printf("invalid configuration %s; tag will not be rendered", key)
		*value = ""
	}
}

func asciiRunes(value, allowed string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune(allowed, char) {
			return false
		}
	}
	return true
}

func validVerificationAttribute(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func validMetaTagName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && !strings.ContainsRune(":_.-", char) {
			return false
		}
	}
	switch name {
	case "description", "keywords", "author", "robots", "viewport":
		return false
	default:
		return true
	}
}

func validateI18nConfig(cfg *I18nConfig) {
	defaults := i18n.Languages()
	if len(cfg.AvailableLanguages) == 0 {
		cfg.AvailableLanguages = defaults
	}
	available := make(map[string]bool, len(cfg.AvailableLanguages))
	valid := true
	for _, language := range cfg.AvailableLanguages {
		canonical, ok := i18n.CanonicalLanguage(language)
		if !ok || canonical != language || available[language] {
			valid = false
			break
		}
		available[language] = true
	}
	for _, language := range i18n.RequiredLanguages() {
		if !available[language] {
			valid = false
		}
	}
	defaultLanguage, ok := i18n.CanonicalLanguage(cfg.DefaultLanguage)
	if !ok || defaultLanguage != cfg.DefaultLanguage || !available[defaultLanguage] {
		valid = false
	}
	if !valid {
		log.Printf("invalid configuration server.i18n; using default languages")
		cfg.DefaultLanguage = "en"
		cfg.AvailableLanguages = defaults
	}
}

// generateConfigYAML generates YAML content with comments
func generateConfigYAML(cfg *Config) string {
	return fmt.Sprintf(`# This node only: listen, identity, database, cache, ssl. The rest is in yaml.d/ and credentials are in secrets.yml.

server:
  port: "%s"
  fqdn: "%s"
  address: "%s"
  database:
    driver: "%s"
    url: "%s"
    pool:
      max_open: %d
      max_idle: %d
      max_lifetime: "%s"
      max_idle_time: "%s"
  baseurl: "%s"
  mode: "%s"
  update:
    branch: "%s"
    auto_install: %t
    defer_days: %d
    cli_min_version: "%s"

  schedule:
    enabled: %t
    notifications: "%s"

  metrics:
    enabled: %t
    endpoint: "%s"
    include_system: %t
    include_app: %t

  logging:
    access_format: "%s"
    level: "%s"

  privacy:
    data:
      sold: %t

  contact:
    admin:
      email: "%s"
    security:
      email: "%s"
    abuse:
      email: "%s"
    dpo:
      name: "%s"
      email: "%s"
    general:
      email: "%s"
  security:
    reports:
      retention_days: %d
  notifications:
    email:
      smtp:
        host: "%s"
        port: %d
        username: "%s"
        tls: "%s"
      from:
        name: "%s"
        email: "%s"
  pages:
    contact:
      enabled: %t
      captcha: "%s"
      captcha_site_key: "%s"
      success_message: "%s"
      antispam:
        honeypot: %t
        min_fill_seconds: %d
        max_age_seconds: %d
        per_ip: %d
        per_email: %d
        global: %d
        window: "%s"
        max_links: %d
        reject_markup: %t
        blocked_words: %s
        blocked_email_domains: %s
        strikes: %d
        ban_hours: %d
        review_min_seconds: %d
        review_max_seconds: %d
        duplicate_window_hours: %d

web-ui:
  theme: "%s"
  logo: "%s"
  favicon: "%s"
  notifications:
    enabled: %t
    announcements: %s

web-robots:
  allow: %s
  deny: %s

web-security:
  admin: "%s"
  cors: "%s"
  report_url: "%s"
  contact: "%s"
  policy: "%s"
  keyservers: %s
  publish_pgp_key: %t
`,
		cfg.Server.Port,
		cfg.Server.FQDN,
		cfg.Server.Address,
		cfg.Server.Database.Driver,
		cfg.Server.Database.URL,
		cfg.Server.Database.Pool.MaxOpen,
		cfg.Server.Database.Pool.MaxIdle,
		cfg.Server.Database.Pool.MaxLifetime,
		cfg.Server.Database.Pool.MaxIdleTime,
		cfg.Server.BaseURL,
		cfg.Server.Mode,
		cfg.Server.Update.Branch,
		cfg.Server.Update.AutoInstall,
		cfg.Server.Update.DeferDays,
		cfg.Server.Update.CLIMinVersion,
		cfg.Server.Schedule.Enabled,
		cfg.Server.Schedule.Notifications,
		cfg.Server.Metrics.Enabled,
		cfg.Server.Metrics.Endpoint,
		cfg.Server.Metrics.IncludeSystem,
		cfg.Server.Metrics.IncludeApp,
		cfg.Server.Logging.AccessFormat,
		cfg.Server.Logging.Level,
		cfg.Server.Privacy.Data.Sold,
		cfg.Server.Contact.Admin.Email,
		cfg.Server.Contact.Security.Email,
		cfg.Server.Contact.Abuse.Email,
		cfg.Server.Contact.DPO.Name,
		cfg.Server.Contact.DPO.Email,
		cfg.Server.Contact.General.Email,
		cfg.Server.Security.Reports.RetentionDays,
		cfg.Server.Notifications.Email.SMTP.Host,
		cfg.Server.Notifications.Email.SMTP.Port,
		cfg.Server.Notifications.Email.SMTP.Username,
		cfg.Server.Notifications.Email.SMTP.TLS,
		cfg.Server.Notifications.Email.From.Name,
		cfg.Server.Notifications.Email.From.Email,
		cfg.Server.Pages.Contact.Enabled,
		cfg.Server.Pages.Contact.Captcha,
		cfg.Server.Pages.Contact.CaptchaSiteKey,
		cfg.Server.Pages.Contact.SuccessMessage,
		cfg.Server.Pages.Contact.Antispam.Honeypot,
		cfg.Server.Pages.Contact.Antispam.MinFillSeconds,
		cfg.Server.Pages.Contact.Antispam.MaxAgeSeconds,
		cfg.Server.Pages.Contact.Antispam.PerIP,
		cfg.Server.Pages.Contact.Antispam.PerEmail,
		cfg.Server.Pages.Contact.Antispam.Global,
		cfg.Server.Pages.Contact.Antispam.Window,
		cfg.Server.Pages.Contact.Antispam.MaxLinks,
		cfg.Server.Pages.Contact.Antispam.RejectMarkup,
		formatStringSlice(cfg.Server.Pages.Contact.Antispam.BlockedWords),
		formatStringSlice(cfg.Server.Pages.Contact.Antispam.BlockedEmailDomains),
		cfg.Server.Pages.Contact.Antispam.Strikes,
		cfg.Server.Pages.Contact.Antispam.BanHours,
		cfg.Server.Pages.Contact.Antispam.ReviewMinSeconds,
		cfg.Server.Pages.Contact.Antispam.ReviewMaxSeconds,
		cfg.Server.Pages.Contact.Antispam.DuplicateWindowHours,
		cfg.WebUI.Theme,
		cfg.WebUI.Logo,
		cfg.WebUI.Favicon,
		cfg.WebUI.Notifications.Enabled,
		formatStringSlice(cfg.WebUI.Notifications.Announcements),
		formatStringSlice(cfg.WebRobots.Allow),
		formatStringSlice(cfg.WebRobots.Deny),
		cfg.WebSecurity.Admin,
		cfg.WebSecurity.CORS,
		cfg.WebSecurity.ReportURL,
		cfg.WebSecurity.Contact,
		cfg.WebSecurity.Policy,
		formatStringSlice(cfg.WebSecurity.Keyservers),
		cfg.WebSecurity.PublishPGPKey,
	)
}

func formatStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	result := "["
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("\"%s\"", v)
	}
	result += "]"
	return result
}

// GetTheme returns the current theme
func GetTheme() string {
	cfg := Get()
	return cfg.WebUI.Theme
}

// GetCORS returns the CORS setting
func GetCORS() string {
	cfg := Get()
	if cfg.WebSecurity.CORS == "" {
		return "*"
	}
	return cfg.WebSecurity.CORS
}
