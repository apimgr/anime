package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httprate"
)

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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'")

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
func throttleMiddleware(maxConcurrent int) func(http.Handler) http.Handler {
	semaphore := make(chan struct{}, maxConcurrent)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				next.ServeHTTP(w, r)
			default:
				log.Printf("Too many concurrent requests, rejecting: %s %s", r.Method, r.RequestURI)
				http.Error(w, "Service Temporarily Unavailable", http.StatusServiceUnavailable)
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
func globalRateLimitMiddleware() func(http.Handler) http.Handler {
	return httprate.LimitByIP(globalRPS, 1*time.Second)
}

// apiRateLimitMiddleware applies API-specific rate limiting by IP
func apiRateLimitMiddleware() func(http.Handler) http.Handler {
	return httprate.LimitByIP(apiRPS, 1*time.Second)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (when behind proxy)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (when behind proxy)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	ip = strings.Trim(ip, "[]")

	return ip
}

// recoverMiddleware recovers from panics and logs them
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// timeoutMiddleware adds a timeout to requests (not currently used but available)
func timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done := make(chan bool)
		timer := time.NewTimer(maxRequestTimeout)
		defer timer.Stop()

		go func() {
			next.ServeHTTP(w, r)
			done <- true
		}()

		select {
		case <-done:
			return
		case <-timer.C:
			log.Printf("Request timeout: %s %s", r.Method, r.RequestURI)
			http.Error(w, "Request Timeout", http.StatusRequestTimeout)
			return
		}
	})
}

// Unused but kept for reference
var _ = fmt.Sprintf
