package server

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
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
	router       *mux.Router
	animeService *anime.Service
	cfg          *config.Config
	port         string
	address      string
	startTime    time.Time
	updateClient *updater.Client
	cliUpdates   *cliUpdateCache
	debugEnabled bool
	database     *database.Database
}

// NewServer creates a new HTTP server
func NewServer(animeService *anime.Service, cfg *config.Config, port, address string, databases ...*database.Database) (*Server, error) {
	// Initialize templates
	if err := initTemplates(); err != nil {
		return nil, fmt.Errorf("failed to initialize templates: %w", err)
	}

	s := &Server{
		router:       mux.NewRouter(),
		animeService: animeService,
		cfg:          cfg,
		port:         port,
		address:      address,
		startTime:    time.Now(),
		updateClient: updater.NewClient(),
		cliUpdates:   &cliUpdateCache{},
		debugEnabled: mode.ShouldShowDebugEndpoints(),
	}
	if len(databases) > 0 {
		s.database = databases[0]
	}

	s.setupRoutes()
	return s, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Apply global middleware (in order of execution)
	// Convert explicit plain-text Accept requests after route handlers produce their normal response.
	s.router.Use(contentNegotiationMiddleware)
	s.router.Use(recoverMiddleware)           // Panic recovery
	s.router.Use(s.securityHeadersMiddleware) // Security headers
	s.router.Use(s.corsMiddleware)            // CORS
	s.router.Use(requestSizeLimitMiddleware)  // Request size limits
	s.router.Use(throttleMiddleware(maxConcurrentRequests))
	s.router.Use(globalRateLimitMiddleware()) // Global rate limiting
	s.router.Use(loggingMiddleware)           // Request logging

	// Static files (CSS, JS, images)
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/", s.serveStatic()))

	// Special files (PWA, robots, security)
	s.router.HandleFunc("/robots.txt", s.handleRobotsTxt).Methods("GET")
	s.router.HandleFunc("/security.txt", s.handleSecurityTxt).Methods("GET")
	s.router.HandleFunc("/.well-known/security.txt", s.handleSecurityTxt).Methods("GET")
	s.router.HandleFunc("/manifest.json", s.handleManifest).Methods("GET")
	s.router.HandleFunc("/sw.js", s.handleServiceWorker).Methods("GET")

	// Web UI routes
	s.router.HandleFunc("/", s.handleHome).Methods("GET")
	s.router.HandleFunc("/quotes", s.handleQuotesPage).Methods("GET")
	s.router.HandleFunc("/server/about", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/help", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/healthz", s.handleHealthz).Methods("GET")
	s.router.HandleFunc("/server/privacy", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/terms", s.handleInfoPage).Methods("GET")
	s.router.HandleFunc("/server/docs/swagger", s.handleSwaggerUI).Methods("GET")
	s.router.HandleFunc("/server/docs/graphql", s.handleGraphQLUI).Methods("GET")
	s.router.Handle("/graphql", apiRateLimitMiddleware()(http.HandlerFunc(s.handleGraphQL))).Methods("GET", "POST")

	// API roots and aliases are read-only and use the same API rate limit as versioned routes.
	s.router.HandleFunc("/api/autodiscover", s.handleAutodiscover).Methods("GET")
	s.router.PathPrefix("/cli/binaries/").HandlerFunc(s.handleCLIBinary).Methods("GET")
	s.router.Handle("/api/v1", apiRateLimitMiddleware()(http.HandlerFunc(s.handleAPIRoot))).Methods("GET")
	s.router.Handle("/api", apiRateLimitMiddleware()(http.HandlerFunc(s.handleAPIRoot))).Methods("GET")

	// API v1 routes (public read-only API) with API-specific rate limiting.
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(apiRateLimitMiddleware())
	api.HandleFunc("/random", s.handleRandomQuote).Methods("GET")
	api.HandleFunc("/quotes", s.handleAllQuotes).Methods("GET")
	api.HandleFunc("/server/healthz", s.handleHealth).Methods("GET")
	api.HandleFunc("/server/swagger", s.handleOpenAPI).Methods("GET")
	api.HandleFunc("/server/graphql", s.handleGraphQL).Methods("GET", "POST")
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
	s.router.Handle("/api/healthz", apiRateLimitMiddleware()(http.HandlerFunc(s.handleHealth))).Methods("GET")
	s.router.Handle("/api/swagger", apiRateLimitMiddleware()(http.HandlerFunc(s.handleOpenAPI))).Methods("GET")
	s.router.Handle("/api/graphql", apiRateLimitMiddleware()(http.HandlerFunc(s.handleGraphQL))).Methods("GET", "POST")
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
	}

	return server.ListenAndServe()
}

// handleRobotsTxt generates robots.txt from config
func (s *Server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	sb.WriteString("User-agent: *\n")

	for _, path := range s.cfg.WebRobots.Allow {
		sb.WriteString(fmt.Sprintf("Allow: %s\n", path))
	}
	for _, path := range s.cfg.WebRobots.Deny {
		sb.WriteString(fmt.Sprintf("Disallow: %s\n", path))
	}

	w.Write([]byte(sb.String()))
}

// handleSecurityTxt generates security.txt
func (s *Server) handleSecurityTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	admin := s.cfg.WebSecurity.Admin
	if admin == "" {
		admin = "security@apimgr.us"
	}

	content := fmt.Sprintf(`Contact: mailto:%s
Expires: 2026-12-31T23:59:59.000Z
Preferred-Languages: en
Canonical: https://anime.apimgr.us/.well-known/security.txt
`, admin)

	w.Write([]byte(content))
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

	w.Write([]byte(manifest))
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
	w.Write([]byte(sw))
}

// handleHealthz returns a simple health check response
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>Anime Quotes health</title><main><p><a href=\"/\">Home</a></p><h1>Service health</h1><p>Status: healthy</p><p>Quotes loaded: %d</p><p><a href=\"/api/v1/server/healthz\">JSON health API</a></p></main></html>", s.animeService.GetTotalQuotes())
}

func (s *Server) handleAPIRoot(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{"name": "anime", "version": Version, "routes": []string{"/api/v1/random", "/api/v1/quotes", "/api/v1/server/healthz", "/api/v1/server/swagger", "/api/v1/server/graphql"}})
}
