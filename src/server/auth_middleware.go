package server

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// failedLoginAttempt tracks failed login attempts per IP
type failedLoginAttempt struct {
	count      int
	lastAttempt time.Time
}

// failedAttempts tracks failed login attempts by IP address
var (
	failedAttempts   = make(map[string]*failedLoginAttempt)
	failedAttemptsMu sync.RWMutex
)

const (
	maxFailedAttempts    = 5
	failedAttemptTimeout = 15 * time.Minute
)

// checkBruteForce checks if an IP is blocked due to too many failed login attempts
func checkBruteForce(ip string) bool {
	failedAttemptsMu.RLock()
	defer failedAttemptsMu.RUnlock()

	attempt, exists := failedAttempts[ip]
	if !exists {
		return false
	}

	// Check if timeout has expired
	if time.Since(attempt.lastAttempt) > failedAttemptTimeout {
		return false
	}

	return attempt.count >= maxFailedAttempts
}

// recordFailedAttempt records a failed login attempt for an IP
func recordFailedAttempt(ip string) {
	failedAttemptsMu.Lock()
	defer failedAttemptsMu.Unlock()

	attempt, exists := failedAttempts[ip]
	if !exists {
		failedAttempts[ip] = &failedLoginAttempt{
			count:      1,
			lastAttempt: time.Now(),
		}
		return
	}

	// Reset count if timeout has expired
	if time.Since(attempt.lastAttempt) > failedAttemptTimeout {
		attempt.count = 1
		attempt.lastAttempt = time.Now()
		return
	}

	// Increment count
	attempt.count++
	attempt.lastAttempt = time.Now()
}

// resetFailedAttempts clears failed attempts for an IP (on successful login)
func resetFailedAttempts(ip string) {
	failedAttemptsMu.Lock()
	defer failedAttemptsMu.Unlock()

	delete(failedAttempts, ip)
}

// cleanupExpiredAttempts removes expired failed attempt records
func cleanupExpiredAttempts() {
	failedAttemptsMu.Lock()
	defer failedAttemptsMu.Unlock()

	now := time.Now()
	for ip, attempt := range failedAttempts {
		if now.Sub(attempt.lastAttempt) > failedAttemptTimeout {
			delete(failedAttempts, ip)
		}
	}
}

// startBruteForceCleanup starts a background goroutine to clean up expired attempts
func startBruteForceCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			cleanupExpiredAttempts()
		}
	}()
}

// authMiddleware validates admin authentication token
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP for brute force protection
		clientIP := getClientIP(r)

		// Check for brute force attack
		if checkBruteForce(clientIP) {
			log.Printf("Brute force protection triggered for IP: %s", clientIP)
			http.Error(w, "Too Many Failed Attempts - Please Try Again Later", http.StatusTooManyRequests)
			return
		}

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Check for token in cookie
			cookie, err := r.Cookie("admin_token")
			if err != nil {
				log.Printf("No authentication token provided from IP: %s", clientIP)
				recordFailedAttempt(clientIP)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			authHeader = "Bearer " + cookie.Value
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Printf("Invalid authorization header format from IP: %s", clientIP)
			recordFailedAttempt(clientIP)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Validate token against database
		if s.db != nil {
			valid, err := s.db.ValidateToken(token)
			if err != nil {
				log.Printf("Error validating token from IP %s: %v", clientIP, err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if !valid {
				log.Printf("Invalid or expired token from IP: %s", clientIP)
				recordFailedAttempt(clientIP)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Token is valid, reset failed attempts for this IP
			resetFailedAttempts(clientIP)
		} else {
			// Fallback: if no database is configured, allow access (development mode)
			log.Printf("Warning: No database configured, skipping auth check")
		}

		// Token is valid, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// adminAuthMiddleware is a convenience wrapper for admin route authentication
func (s *Server) adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.authMiddleware(http.HandlerFunc(next)).ServeHTTP(w, r)
	}
}
