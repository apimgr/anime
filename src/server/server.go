package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/config"
	"github.com/apimgr/anime/src/database"
	"github.com/apimgr/anime/src/mode"
	"github.com/apimgr/anime/src/updater"
	"github.com/gorilla/mux"
)

// Build info (set from main)
var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "N/A"
	OfficialSite = ""
)

// Server represents the HTTP server
type Server struct {
	router             *mux.Router
	animeService       *anime.Service
	cfg                *config.Config
	port               string
	address            string
	startTime          time.Time
	updateClient       *updater.Client
	cliUpdates         *cliUpdateCache
	debugEnabled       bool
	database           *database.Database
	requestStats       *requestStats
	activeConns        atomic.Int64
	csrfKey            []byte
	installationSecret []byte
	contactMu          sync.Mutex
	contactNonces      map[string]time.Time
	smtpProbed         bool
	smtpAvailable      bool
}

// NewServer creates a new HTTP server
func NewServer(animeService *anime.Service, cfg *config.Config, port, address string, databases ...*database.Database) (*Server, error) {
	// Initialize templates
	if err := initTemplates(); err != nil {
		return nil, fmt.Errorf("failed to initialize templates: %w", err)
	}

	csrfKey := make([]byte, 32)
	installationSecret := make([]byte, 32)
	if _, err := rand.Read(csrfKey); err != nil {
		return nil, fmt.Errorf("failed to initialize CSRF signing key: %w", err)
	}
	if _, err := rand.Read(installationSecret); err != nil {
		return nil, fmt.Errorf("failed to initialize installation secret: %w", err)
	}
	s := &Server{
		router:        mux.NewRouter(),
		animeService:  animeService,
		cfg:           cfg,
		port:          port,
		address:       address,
		startTime:     time.Now(),
		updateClient:  updater.NewClient(),
		cliUpdates:    &cliUpdateCache{},
		debugEnabled:  mode.ShouldShowDebugEndpoints(),
		requestStats:  &requestStats{},
		csrfKey:       csrfKey,
		contactNonces: make(map[string]time.Time),
	}
	if len(databases) > 0 {
		s.database = databases[0]
		if s.database != nil && s.database.SQL != nil {
			secret, err := database.AppSecret(context.Background(), s.database.SQL, "installation_secret")
			if err != nil {
				return nil, fmt.Errorf("load installation secret: %w", err)
			}
			installationSecret = secret
			csrfSecret, err := database.AppSecret(context.Background(), s.database.SQL, "csrf_token_secret")
			if err != nil {
				return nil, fmt.Errorf("load CSRF token secret: %w", err)
			}
			csrfKey = csrfSecret
		}
	}
	s.installationSecret = installationSecret
	s.csrfKey = csrfKey

	s.setupRoutes()
	return s, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Apply global middleware (in order of execution)
	// Convert explicit plain-text Accept requests after route handlers produce their normal response.
	s.router.Use(urlNormalizeMiddleware)
	s.router.Use(s.requestStatsMiddleware)
	s.router.Use(contentNegotiationMiddleware)
	s.router.Use(s.seoMiddleware)
	s.router.Use(s.recoverRequests)
	s.router.Use(s.securityHeadersMiddleware) // Security headers
	s.router.Use(s.corsMiddleware)            // CORS
	s.router.Use(requestSizeLimitMiddleware)  // Request size limits
	s.router.Use(throttleMiddleware(maxConcurrentRequests, s.renderStatusError))
	s.router.Use(globalRateLimitMiddleware(s.rateLimitError))
	s.router.Use(loggingMiddleware) // Request logging

	// Static files (CSS, JS, images)
	s.router.PathPrefix("/locales/").HandlerFunc(s.handleLocaleJSON).Methods("GET")
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/", s.serveStatic()))

	// Special files (PWA, robots, security)
	s.router.HandleFunc("/robots.txt", s.handleRobotsTxt).Methods("GET")
	s.router.HandleFunc("/.well-known/security.txt", s.handleSecurityTxt).Methods("GET", "HEAD")
	s.router.HandleFunc("/sitemap.xml", s.handleSitemap).Methods("GET", "HEAD")
	s.router.PathPrefix("/sitemap-").HandlerFunc(s.handleSitemapPart).Methods("GET", "HEAD")
	s.router.HandleFunc("/feed.xml", s.handleAtomFeed).Methods("GET", "HEAD")
	s.router.HandleFunc("/rss.xml", s.handleRSSFeed).Methods("GET", "HEAD")
	s.router.HandleFunc("/.well-known/llms.txt", s.handleLLMSTxt).Methods("GET", "HEAD")
	s.router.HandleFunc("/llms.txt", s.handleLLMSTxt).Methods("GET", "HEAD")
	s.router.HandleFunc("/.well-known/pgp-key.asc", s.handlePGPKey).Methods("GET", "HEAD")
	s.router.HandleFunc("/manifest.json", s.handleManifest).Methods("GET")
	s.router.HandleFunc("/sw.js", s.handleServiceWorker).Methods("GET")

	// Web UI routes
	s.router.HandleFunc("/", s.handleHome).Methods("GET")
	s.router.HandleFunc("/quotes", s.handleQuotesPage).Methods("GET")
	s.router.HandleFunc("/server/about", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/help", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/help/mcp", s.handleMCPHelpPage).Methods("GET")
	s.router.HandleFunc("/server/healthz", s.handleHealthz).Methods("GET")
	s.router.HandleFunc("/server/preferences", s.handlePreferences).Methods("GET", "POST")
	s.router.HandleFunc("/server/preferences/tokens", s.handleTokensPage).Methods("GET", "POST")
	s.router.PathPrefix("/server/preferences/token/").HandlerFunc(s.handleTokenWebAction).Methods("POST")
	s.router.PathPrefix("/server/preferences/tokens/").HandlerFunc(s.handleTokenWebAction).Methods("POST")
	s.router.HandleFunc("/server/consent", s.handleConsent).Methods("POST")
	s.router.HandleFunc("/server/preferences/sync", s.handlePreferencesSync).Methods("GET", "POST")
	s.router.HandleFunc("/server/preferences/sync/import", s.importPreferences).Methods("GET")
	s.router.HandleFunc("/server/security", s.handleSecurityPage).Methods("GET")
	s.router.HandleFunc("/server/security/policy", s.handleSecurityPage).Methods("GET")
	s.router.HandleFunc("/server/security/thanks", s.handleSecurityPage).Methods("GET")
	s.router.HandleFunc("/server/security/report/{tracking_id}", s.handleSecurityReportPage).Methods("GET")
	s.router.HandleFunc("/server/contact", s.handleContactPage).Methods("GET", "POST")
	s.router.HandleFunc("/server/privacy", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/terms", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/dpo", s.handleDPOPage).Methods("GET")
	s.router.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/server/about", http.StatusMovedPermanently)
	}).Methods("GET")
	s.router.HandleFunc("/server/docs/swagger", s.handleSwaggerUI).Methods("GET")
	s.router.HandleFunc("/server/docs/graphql", s.handleGraphQLUI).Methods("GET")
	s.router.Handle("/graphql", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleGraphQL))).Methods("GET", "POST")

	// API roots and aliases are read-only and use the same API rate limit as versioned routes.
	s.router.HandleFunc("/api/autodiscover", s.handleAutodiscover).Methods("GET")
	s.router.PathPrefix("/cli/binaries/").HandlerFunc(s.handleCLIBinary).Methods("GET")
	s.router.Handle("/api/v1", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleAPIRoot))).Methods("GET")
	s.router.Handle("/api", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleAPIRoot))).Methods("GET")

	// API v1 routes (public read-only API) with API-specific rate limiting.
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(apiRateLimitMiddleware(s.rateLimitError))
	api.HandleFunc("/random", s.handleRandomQuote).Methods("GET")
	api.HandleFunc("/quotes", s.handleAllQuotes).Methods("GET")
	api.HandleFunc("/server/healthz", s.handleHealth).Methods("GET")
	api.HandleFunc("/server/preferences", s.handlePreferencesAPI).Methods("GET", "POST")
	api.HandleFunc("/server/preferences/tokens", s.handleTokensAPI)
	api.PathPrefix("/server/preferences/tokens/").HandlerFunc(s.handleTokensAPI)
	api.HandleFunc("/server/preferences/sync", s.handlePreferencesSyncAPI).Methods("GET")
	api.HandleFunc("/server/preferences/sync/import", s.importPreferencesAPI).Methods("GET")
	api.HandleFunc("/server/about", s.handleInfoJSON).Methods("GET")
	api.HandleFunc("/server/help", s.handleInfoJSON).Methods("GET")
	api.HandleFunc("/server/help/mcp", s.handleMCPHelpJSON).Methods("GET")
	api.HandleFunc("/server/privacy", s.handleInfoJSON).Methods("GET")
	api.HandleFunc("/server/terms", s.handleInfoJSON).Methods("GET")
	api.HandleFunc("/server/dpo", s.handleDPOJSON).Methods("GET")
	api.HandleFunc("/server/security", s.handleSecurityJSON).Methods("GET")
	api.HandleFunc("/server/security/policy", s.handleSecurityJSON).Methods("GET")
	api.HandleFunc("/server/security/thanks", s.handleSecurityJSON).Methods("GET")
	api.HandleFunc("/server/contact", s.handleContactAPI).Methods("GET", "POST")
	api.HandleFunc("/server/swagger", s.handleOpenAPI).Methods("GET")
	api.HandleFunc("/server/graphql", s.handleGraphQL).Methods("GET", "POST")
	api.HandleFunc("/server/mcp", s.handleMCP)
	api.HandleFunc("/stats", s.handleStats).Methods("GET")
	if s.debugEnabled {
		debugHandler := s.operatorAuthMiddleware(s.debugMux())
		s.router.PathPrefix("/debug/").Handler(debugHandler)
		s.router.PathPrefix("/api/debug/").Handler(http.StripPrefix("/api", debugHandler))
		s.router.PathPrefix("/api/v1/debug/").Handler(http.StripPrefix("/api/v1", debugHandler))
	}

	// Text format endpoints (.txt extension)
	api.HandleFunc("/random.txt", s.handleRandomQuoteText).Methods("GET")
	api.HandleFunc("/quotes.txt", s.handleAllQuotesText).Methods("GET")
	api.HandleFunc("/server/healthz.txt", s.handleHealthText).Methods("GET")
	api.HandleFunc("/stats.txt", s.handleStatsText).Methods("GET")
	s.router.Handle("/api/healthz", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleHealth))).Methods("GET")
	s.router.Handle("/api/swagger", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleOpenAPI))).Methods("GET")
	s.router.Handle("/api/graphql", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleGraphQL))).Methods("GET", "POST")
	s.router.Handle("/api/mcp", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleMCP)))
	s.router.Handle("/mcp", apiRateLimitMiddleware(s.rateLimitError)(http.HandlerFunc(s.handleMCP)))
	s.router.NotFoundHandler = http.HandlerFunc(s.notFound)
	s.router.MethodNotAllowedHandler = http.HandlerFunc(s.methodNotAllowed)
}

// getServerURL returns the server URL for display
func (s *Server) getServerURL(r *http.Request) string {
	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "localhost" {
		return fmt.Sprintf("%s:%s", hostname, s.port)
	}

	host := r.Host
	if host != "" && !strings.Contains(host, "localhost") && !strings.Contains(host, "127.0.0.1") {
		return host
	}

	return fmt.Sprintf("<your-host>:%s", s.port)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.initializeSMTP()
	cleanupStop := make(chan struct{})
	go s.securityCleanupLoop(cleanupStop)
	defer close(cleanupStop)
	// Build listen address
	var addr string
	if s.address == "::" {
		addr = fmt.Sprintf(":%s", s.port)
	} else {
		addr = fmt.Sprintf("%s:%s", s.address, s.port)
	}

	// Format display URL with IPv6 brackets if needed
	displayURL := addr
	if strings.Contains(s.address, ":") && s.address != "::" && s.address != "" {
		displayURL = fmt.Sprintf("[%s]:%s", s.address, s.port)
	} else if s.address == "::" {
		displayURL = fmt.Sprintf("<your-host>:%s", s.port)
	}

	log.Printf("Starting Anime Quotes API server on %s", addr)
	log.Printf("Total quotes loaded: %d", s.animeService.GetTotalQuotes())
	log.Printf("")
	log.Printf("Security Configuration:")
	log.Printf("  Global Rate Limit:  %d req/s (burst: %d)", globalRPS, globalBurst)
	log.Printf("  API Rate Limit:     %d req/s (burst: %d)", apiRPS, apiBurst)
	log.Printf("  Max Request Size:   %d MB", maxBodySize>>20)
	log.Printf("  Max Header Size:    %d MB", maxHeaderSize>>20)
	log.Printf("  Max Concurrent:     %d requests", maxConcurrentRequests)
	log.Printf("  Request Timeout:    %v", maxRequestTimeout)
	log.Printf("")
	log.Printf("Web UI:")
	log.Printf("  GET /                    - Homepage with random quote")
	log.Printf("  GET /server/healthz      - Health check")
	log.Printf("")
	log.Printf("API Endpoints:")
	log.Printf("  GET /api/v1/random       - Get a random quote")
	log.Printf("  GET /api/v1/quotes       - Get all quotes")
	log.Printf("  GET /api/v1/server/healthz - Health check")
	log.Printf("  GET /api/v1/stats        - Statistics")
	log.Printf("")
	log.Printf("Text Format (add .txt extension):")
	log.Printf("  GET /api/v1/random.txt   - Random quote as text")
	log.Printf("  GET /api/v1/quotes.txt   - All quotes as text")
	log.Printf("  GET /api/v1/server/healthz.txt - Health as text")
	log.Printf("  GET /api/v1/stats.txt    - Statistics as text")
	log.Printf("")
	log.Printf("Special Files:")
	log.Printf("  GET /robots.txt          - Robots file")
	log.Printf("  GET /security.txt        - Security contact")
	log.Printf("  GET /manifest.json       - PWA manifest")
	log.Printf("  GET /sw.js               - Service worker")
	log.Printf("")
	log.Printf("Access the web UI at: http://%s", displayURL)

	// Create HTTP server with security timeouts
	server := &http.Server{
		Addr:           addr,
		Handler:        s.router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: maxHeaderSize,
		ConnState:      s.trackConnection,
	}

	return server.ListenAndServe()
}

func (s *Server) trackConnection(_ net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		s.activeConns.Add(1)
	case http.StateClosed, http.StateHijacked:
		s.activeConns.Add(-1)
	}
}

// handleRobotsTxt generates robots.txt from config
func (s *Server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	sb.WriteString("User-agent: *\n")

	for _, path := range s.cfg.WebRobots.Allow {
		sb.WriteString("Allow: " + path + "\n")
	}
	for _, path := range s.cfg.WebRobots.Deny {
		sb.WriteString("Disallow: " + path + "\n")
	}
	sb.WriteString("Sitemap: " + BuildURL(r, "/sitemap.xml") + "\n")

	_, _ = w.Write([]byte(sb.String()))
}

// handleSecurityTxt generates security.txt
func (s *Server) handleSecurityTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	origin := strings.TrimSuffix(BuildURL(r, "/"), "/")
	var content strings.Builder
	fmt.Fprintf(&content, "Contact: %s\n", s.securityReportURL())
	if s.cfg.Server.Pages.Contact.Enabled {
		fmt.Fprintf(&content, "Contact: %s/server/contact?security_id=%s\n", origin, s.securityID(time.Now()))
	}
	securityEmail := s.securityContactEmail(r)
	if securityEmail != "" {
		fmt.Fprintf(&content, "Contact: mailto:%s\n", securityEmail)
	}
	if s.securityKeyFingerprint() != "" {
		fmt.Fprintf(&content, "Encryption: %s/.well-known/pgp-key.asc\n", origin)
	}
	fmt.Fprintf(&content, "Policy: %s/server/security\nExpires: %s\nPreferred-Languages: en, es, fr, de, it, ar\nCanonical: %s/.well-known/security.txt\n", origin, time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339), origin)

	body := []byte(content.String())
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

func (s *Server) securityID(now time.Time) string {
	window := now.Unix() / 172800
	mac := hmac.New(sha256.New, s.installationSecret)
	_, _ = fmt.Fprintf(mac, "%d", window)
	return fmt.Sprintf("%x", mac.Sum(nil))[:16]
}

func (s *Server) validSecurityID(value string, now time.Time) bool {
	if len(value) != 16 {
		return false
	}
	for offset := int64(0); offset <= 1; offset++ {
		candidate := s.securityID(time.Unix(((now.Unix()/172800)-offset)*172800, 0))
		if hmac.Equal([]byte(value), []byte(candidate)) {
			return true
		}
	}
	return false
}

// handleManifest returns the PWA manifest
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")

	theme := s.cfg.WebUI.Theme
	bgColor := "#1a1a1a"
	if theme == "light" {
		bgColor = "#ffffff"
	}

	manifest := fmt.Sprintf(`{
  "name": "Anime Quotes API",
  "short_name": "Anime Quotes",
  "description": "Random anime quotes API",
  "start_url": "/",
  "display": "standalone",
  "background_color": "%s",
  "theme_color": "#0066cc",
  "icons": [
    {
      "src": "/static/images/icon-192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/static/images/icon-512.png",
      "sizes": "512x512",
      "type": "image/png"
    }
  ]
}`, bgColor)

	_, _ = w.Write([]byte(manifest))
}

// handleServiceWorker returns a basic service worker
func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")

	sw := `// Anime Quotes API Service Worker
const CACHE_NAME = 'anime-quotes-v1';
const urlsToCache = [
  '/',
  '/static/css/main.css',
  '/static/js/main.js',
  '/manifest.json'
];

self.addEventListener('install', function(event) {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(function(cache) {
        return cache.addAll(urlsToCache);
      })
  );
});

self.addEventListener('fetch', function(event) {
  event.respondWith(
    caches.match(event.request)
      .then(function(response) {
        if (response) {
          return response;
        }
        return fetch(event.request);
      }
    )
  );
});
`
	_, _ = w.Write([]byte(sw))
}

// handleHealthz returns a simple health check response
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	status, httpStatus := "unhealthy", http.StatusServiceUnavailable
	databaseStatus := s.checkDatabase(r.Context())
	if databaseStatus == "ok" {
		status, httpStatus = "healthy", http.StatusOK
	}
	torEnabled, torRunning, torStatus, torHostname := torHealth()
	requestsTotal, requests24h := int64(0), int64(0)
	if s.requestStats != nil {
		requestsTotal, requests24h = s.requestStats.snapshot(time.Now())
	}
	data := map[string]interface{}{
		"Title": "Service health", "Description": "Current health status for the Anime Quotes service.",
		"ContentTemplate": "health-content", "Page": "healthz", "Version": Version,
		"TotalQuotes": s.animeService.GetTotalQuotes(), "StartedAt": s.startTime.UTC().Format(time.RFC3339), "HealthStatus": status,
		"GoVersion": runtime.Version(), "CommitID": CommitID, "BuildDate": BuildDate, "Uptime": time.Since(s.startTime).String(),
		"Mode": mode.GetCurrentAppMode(), "Timestamp": time.Now().UTC().Format(time.RFC3339), "DatabaseStatus": databaseStatus,
		"TorEnabled": torEnabled, "TorRunning": torRunning, "TorStatus": torStatus, "TorHostname": torHostname,
		"I2PStatus": "disabled", "GeoIPEnabled": false, "CacheStatus": "ok", "DiskStatus": "ok", "SchedulerStatus": "ok",
		"RequestsTotal": requestsTotal, "Requests24h": requests24h, "ActiveConnections": s.activeConns.Load(),
	}
	if err := s.renderPageWithStatus(w, r, data, httpStatus); err != nil {
		log.Printf("Error rendering health page: %v", err)
	}
}

func (s *Server) handleAPIRoot(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{"name": "anime", "version": Version, "routes": apiRouteIndex()})
}
