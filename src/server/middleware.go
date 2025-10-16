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

	// Admin rate limit (most restrictive)
	adminRPS   = 10
	adminBurst = 20

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
		// Get security header settings
		var headers SecurityHeaderSettings
		if s.settingsManager != nil {
			headers = s.settingsManager.GetSecurityHeaders()
		} else {
			// Fallback to defaults if settings manager not available
			headers = SecurityHeaderSettings{
				FrameOptions:          "DENY",
				ContentTypeOptions:    "nosniff",
				XSSProtection:         "1; mode=block",
				ReferrerPolicy:        "strict-origin-when-cross-origin",
				PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
				CSPEnabled:            true,
				CSP:                   "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'",
				HSTSEnabled:           true,
				HSTSMaxAge:            31536000,
				HSTSIncludeSubdomains: true,
			}
		}

		// Apply security headers
		w.Header().Set("X-Frame-Options", headers.FrameOptions)
		w.Header().Set("X-Content-Type-Options", headers.ContentTypeOptions)
		w.Header().Set("X-XSS-Protection", headers.XSSProtection)
		w.Header().Set("Referrer-Policy", headers.ReferrerPolicy)
		w.Header().Set("Permissions-Policy", headers.PermissionsPolicy)

		// Content Security Policy
		if headers.CSPEnabled && headers.CSP != "" {
			w.Header().Set("Content-Security-Policy", headers.CSP)
		}

		// HSTS (if using HTTPS)
		if r.TLS != nil && headers.HSTSEnabled {
			hsts := fmt.Sprintf("max-age=%d", headers.HSTSMaxAge)
			if headers.HSTSIncludeSubdomains {
				hsts += "; includeSubDomains"
			}
			w.Header().Set("Strict-Transport-Security", hsts)
		}

		next.ServeHTTP(w, r)
	})
}

// requestSizeLimitMiddleware limits the size of incoming requests
func requestSizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Limit body size
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		next.ServeHTTP(w, r)
	})
}

// timeoutMiddleware adds a timeout to requests
func timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a channel to signal completion
		done := make(chan bool)

		// Create a timeout timer
		timer := time.NewTimer(maxRequestTimeout)
		defer timer.Stop()

		// Run the handler in a goroutine
		go func() {
			next.ServeHTTP(w, r)
			done <- true
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			// Request completed successfully
			return
		case <-timer.C:
			// Request timed out
			log.Printf("Request timeout: %s %s", r.Method, r.RequestURI)
			http.Error(w, "Request Timeout", http.StatusRequestTimeout)
			return
		}
	})
}

// throttleMiddleware limits concurrent requests
func throttleMiddleware(maxConcurrent int) func(http.Handler) http.Handler {
	// Create a semaphore using a buffered channel
	semaphore := make(chan struct{}, maxConcurrent)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to acquire semaphore
			select {
			case semaphore <- struct{}{}:
				// Acquired, process request
				defer func() { <-semaphore }() // Release when done
				next.ServeHTTP(w, r)
			default:
				// Too many concurrent requests
				log.Printf("Too many concurrent requests, rejecting: %s %s", r.Method, r.RequestURI)
				http.Error(w, "Service Temporarily Unavailable", http.StatusServiceUnavailable)
			}
		})
	}
}

// corsMiddleware handles CORS based on settings manager
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Get CORS settings from settings manager
		var corsSettings CORSSettings
		if s.settingsManager != nil {
			corsSettings = s.settingsManager.GetCORS()
		} else {
			// Fallback to defaults
			corsSettings = CORSSettings{
				Enabled:          true,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "Authorization"},
				AllowCredentials: false,
				MaxAge:           3600,
			}
		}

		// Skip if CORS is disabled
		if !corsSettings.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check if origin is allowed
		originAllowed := false
		for _, allowed := range corsSettings.AllowedOrigins {
			if allowed == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				originAllowed = true
				break
			} else if origin == allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				originAllowed = true
				break
			}
		}

		if originAllowed {
			// Set allowed methods
			methods := strings.Join(corsSettings.AllowedMethods, ", ")
			w.Header().Set("Access-Control-Allow-Methods", methods)

			// Set allowed headers
			headers := strings.Join(corsSettings.AllowedHeaders, ", ")
			w.Header().Set("Access-Control-Allow-Headers", headers)

			// Set credentials
			if corsSettings.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Set max age
			w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", corsSettings.MaxAge))
		}

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

// adminRateLimitMiddleware applies admin-specific rate limiting by IP
func adminRateLimitMiddleware() func(http.Handler) http.Handler {
	return httprate.LimitByIP(adminRPS, 1*time.Second)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (when behind proxy)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
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
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// Remove IPv6 brackets if present
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
