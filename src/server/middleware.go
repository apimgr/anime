package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/httprate"
)

type requestBucket struct {
	hour  int64
	count int64
}

type requestStats struct {
	mu      sync.Mutex
	total   int64
	buckets [24]requestBucket
}

func (s *requestStats) begin(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total++
	hour := now.Unix() / 3600
	index := hour % int64(len(s.buckets))
	if s.buckets[index].hour != hour {
		s.buckets[index] = requestBucket{hour: hour}
	}
	s.buckets[index].count++
}

func (s *requestStats) snapshot(now time.Time) (int64, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hour := now.Unix() / 3600
	var last24h int64
	for _, bucket := range s.buckets {
		if bucket.hour >= hour-23 && bucket.hour <= hour {
			last24h += bucket.count
		}
	}
	return s.total, last24h
}

func (s *Server) requestStatsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requestStats.begin(time.Now())
		next.ServeHTTP(w, r)
	})
}

func urlNormalizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || !strings.HasSuffix(path, "/") || strings.Contains(path, "/debug/") || strings.Contains(path[strings.LastIndex(path, "/"):], ".") {
			next.ServeHTTP(w, r)
			return
		}
		canonical := strings.TrimSuffix(path, "/")
		if r.URL.RawQuery != "" {
			canonical += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, canonical, http.StatusMovedPermanently)
	})
}

// Rate limiting configuration
const (
	// Global rate limit (all endpoints)
	globalRPS   = 100 // 100 requests per second
	globalBurst = 200

	// API rate limit (stricter for API)
	apiRPS   = 50
	apiBurst = 100

	// Request size limits
	maxBodySize   = 10 << 20 // 10MB
	maxHeaderSize = 1 << 20  // 1MB

	// Connection limits
	maxConcurrentRequests = 1000

	// Timeouts
	maxRequestTimeout = 60 * time.Second
)

// securityHeadersMiddleware adds security headers to all responses
func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Content Security Policy
		policy := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'"
		if strings.HasPrefix(r.URL.Path, "/server/contact") || strings.HasPrefix(r.URL.Path, "/api/v1/server/contact") {
			switch s.contactCaptchaProvider(r) {
			case "recaptcha":
				policy = "default-src 'self'; script-src 'self' https://www.google.com/recaptcha/ https://www.gstatic.com/recaptcha/; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' https://www.google.com/recaptcha/; frame-src https://www.google.com/recaptcha/"
			case "hcaptcha":
				policy = "default-src 'self'; script-src 'self' https://js.hcaptcha.com https://hcaptcha.com https://*.hcaptcha.com; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' https://hcaptcha.com https://*.hcaptcha.com; frame-src https://hcaptcha.com https://*.hcaptcha.com"
			}
		}
		w.Header().Set("Content-Security-Policy", policy)

		// HSTS (if using HTTPS)
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// requestSizeLimitMiddleware limits the size of incoming requests
func requestSizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		next.ServeHTTP(w, r)
	})
}

// throttleMiddleware limits concurrent requests
func throttleMiddleware(maxConcurrent int, onReject ...func(http.ResponseWriter, *http.Request, int)) func(http.Handler) http.Handler {
	semaphore := make(chan struct{}, maxConcurrent)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				next.ServeHTTP(w, r)
			default:
				log.Printf("Too many concurrent requests, rejecting: %s %s", r.Method, sanitizeRequestURI(r.RequestURI))
				if len(onReject) > 0 {
					onReject[0](w, r, http.StatusServiceUnavailable)
				} else {
					writeFallbackError(w, r, http.StatusServiceUnavailable)
				}
			}
		})
	}
}

// corsMiddleware handles CORS based on config
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get CORS setting from config (default: *)
		corsOrigin := s.cfg.WebSecurity.CORS
		if corsOrigin == "" {
			corsOrigin = "*"
		}

		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		if corsOrigin == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			// Check if origin matches configured origins
			allowedOrigins := strings.Split(corsOrigin, ",")
			for _, allowed := range allowedOrigins {
				if strings.TrimSpace(allowed) == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		// Set standard CORS headers
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, Authorization, X-CSRF-Token, X-XSRF-Token, MCP-Protocol-Version, Mcp-Method, Mcp-Name")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// globalRateLimitMiddleware applies global rate limiting by IP
func globalRateLimitMiddleware(onLimit ...http.HandlerFunc) func(http.Handler) http.Handler {
	return rateLimitByIP(globalRPS, onLimit...)
}

// apiRateLimitMiddleware applies API-specific rate limiting by IP
func apiRateLimitMiddleware(onLimit ...http.HandlerFunc) func(http.Handler) http.Handler {
	return rateLimitByIP(apiRPS, onLimit...)
}

func rateLimitByIP(requestLimit int, onLimit ...http.HandlerFunc) func(http.Handler) http.Handler {
	options := []httprate.Option{httprate.WithKeyFuncs(httprate.KeyByIP)}
	if len(onLimit) > 0 {
		options = append(options, httprate.WithLimitHandler(onLimit[0]))
	}
	return httprate.Limit(requestLimit, time.Second, options...)
}

// recoverMiddleware recovers from panics and logs them
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				writeFallbackError(w, r, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Unused but kept for reference
var _ = fmt.Sprintf
