package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/database"
	"github.com/gorilla/mux"
)

// Server represents the HTTP server
type Server struct {
	router          *mux.Router
	animeService    *anime.Service
	db              *database.DB
	settingsManager *SettingsManager
	port            string
	address         string
	startTime       time.Time
}

// NewServer creates a new HTTP server
func NewServer(animeService *anime.Service, db *database.DB, port, address string) (*Server, error) {
	// Initialize templates
	if err := initTemplates(); err != nil {
		return nil, fmt.Errorf("failed to initialize templates: %w", err)
	}

	// Initialize settings manager
	settingsManager := NewSettingsManager(db)

	s := &Server{
		router:          mux.NewRouter(),
		animeService:    animeService,
		db:              db,
		settingsManager: settingsManager,
		port:            port,
		address:         address,
		startTime:       time.Now(),
	}

	// Start brute force protection cleanup routine
	startBruteForceCleanup()

	s.setupRoutes()
	return s, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Apply global middleware (in order of execution)
	s.router.Use(recoverMiddleware)                          // Panic recovery
	s.router.Use(s.securityHeadersMiddleware)                // Security headers (dynamic from settings)
	s.router.Use(s.corsMiddleware)                           // CORS (dynamic from settings)
	s.router.Use(requestSizeLimitMiddleware)                 // Request size limits
	s.router.Use(throttleMiddleware(maxConcurrentRequests))  // Connection limits
	s.router.Use(globalRateLimitMiddleware())                // Global rate limiting
	s.router.Use(loggingMiddleware)                          // Request logging

	// Static files (CSS, JS, images)
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/", s.serveStatic()))

	// Web UI routes
	s.router.HandleFunc("/", s.handleHome).Methods("GET")
	s.router.HandleFunc("/admin", s.handleAdmin).Methods("GET")

	// API v1 routes (public) with API-specific rate limiting
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(apiRateLimitMiddleware()) // API-specific rate limit
	api.HandleFunc("/random", s.handleRandomQuote).Methods("GET")
	api.HandleFunc("/quotes", s.handleAllQuotes).Methods("GET")
	api.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Admin API routes (protected) with strictest rate limiting
	adminAPI := s.router.PathPrefix("/api/v1/admin").Subrouter()
	adminAPI.Use(adminRateLimitMiddleware()) // Admin-specific rate limit (most restrictive)
	adminAPI.Use(s.authMiddleware)           // Authentication required
	adminAPI.HandleFunc("/settings", s.handleAdminSettings).Methods("GET", "PUT")
	adminAPI.HandleFunc("/settings/reset", s.handleAdminSettingsReset).Methods("POST")
	adminAPI.HandleFunc("/settings/export", s.handleAdminSettingsExport).Methods("GET")
	adminAPI.HandleFunc("/settings/import", s.handleAdminSettingsImport).Methods("POST")
	adminAPI.HandleFunc("/quotes", s.handleAdminQuotes).Methods("GET", "POST", "PUT", "DELETE")
	adminAPI.HandleFunc("/stats", s.handleAdminStats).Methods("GET")

	// Admin Web UI routes
	s.router.HandleFunc("/admin/settings", s.handleAdminSettingsPage).Methods("GET")
}

// getServerURL returns the server URL for display
// IPv6 addresses are wrapped in brackets: [2001:db8::1]:8080
func (s *Server) getServerURL(r *http.Request) string {
	// Try to get hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "localhost" {
		return fmt.Sprintf("%s:%s", hostname, s.port)
	}

	// Use host from request
	host := r.Host
	if host != "" && !strings.Contains(host, "localhost") && !strings.Contains(host, "127.0.0.1") {
		// Host from request should already have proper IPv6 formatting
		return host
	}

	// Fallback
	return fmt.Sprintf("<your-host>:%s", s.port)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Build listen address
	var addr string
	if s.address == "::" {
		// Dual-stack - just use port
		addr = fmt.Sprintf(":%s", s.port)
	} else {
		addr = fmt.Sprintf("%s:%s", s.address, s.port)
	}

	// Format display URL with IPv6 brackets if needed
	displayURL := addr
	if strings.Contains(s.address, ":") && s.address != "::" && s.address != "" {
		// Specific IPv6 address (not dual-stack ::)
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
	log.Printf("  Admin Rate Limit:   %d req/s (burst: %d)", adminRPS, adminBurst)
	log.Printf("  Max Request Size:   %d MB", maxBodySize>>20)
	log.Printf("  Max Header Size:    %d MB", maxHeaderSize>>20)
	log.Printf("  Max Concurrent:     %d requests", maxConcurrentRequests)
	log.Printf("  Request Timeout:    %v", maxRequestTimeout)
	log.Printf("  Brute Force:        %d attempts / %v", maxFailedAttempts, failedAttemptTimeout)
	log.Printf("")
	log.Printf("Web UI:")
	log.Printf("  GET /              - Homepage with random quote")
	log.Printf("  GET /admin         - Admin dashboard")
	log.Printf("")
	log.Printf("API Endpoints:")
	log.Printf("  GET /api/v1/random       - Get a random quote")
	log.Printf("  GET /api/v1/quotes       - Get all quotes")
	log.Printf("  GET /api/v1/health       - Health check")
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
