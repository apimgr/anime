// Package mode implements application mode detection and configuration
// according to the BASE.md specification.
//
// Mode Detection Priority (highest to lowest):
//  1. --mode CLI flag
//  2. MODE environment variable
//  3. Default: production
//
// Supported modes:
//   - production: Optimized for production use with security and performance
//   - development: Optimized for development with debugging and verbose output
package mode

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

const (
	appName = "anime"
)

// Mode represents an application mode
type Mode string

const (
	// Production mode: optimized for production use
	Production Mode = "production"

	// Development mode: optimized for development with debugging enabled
	Development Mode = "development"
)

var (
	// currentMode holds the current application mode
	currentMode Mode = Production

	// mu protects currentMode for thread-safe access
	mu sync.RWMutex
)

// init initializes the mode from environment variable if set
func init() {
	if envMode := os.Getenv("MODE"); envMode != "" {
		if parsed, err := ParseMode(envMode); err == nil {
			currentMode = parsed
		}
	}
}

// ParseMode parses a mode string and returns the corresponding Mode constant.
// It accepts: "dev", "development", "prod", "production" (case-insensitive).
// Returns an error if the mode string is invalid.
func ParseMode(s string) (Mode, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))

	switch normalized {
	case "dev", "development":
		return Development, nil
	case "prod", "production":
		return Production, nil
	default:
		return "", fmt.Errorf("invalid mode: %q (expected: dev, development, prod, or production)", s)
	}
}

// Get returns the current application mode.
// This function is thread-safe.
func Get() Mode {
	mu.RLock()
	defer mu.RUnlock()
	return currentMode
}

// Set sets the application mode.
// The mode string is parsed using ParseMode and must be valid.
// Returns an error if the mode string is invalid.
// This function is thread-safe.
func Set(modeStr string) error {
	parsed, err := ParseMode(modeStr)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	currentMode = parsed

	return nil
}

// IsDevelopment returns true if the current mode is development.
// This function is thread-safe.
func IsDevelopment() bool {
	return Get() == Development
}

// IsProduction returns true if the current mode is production.
// This function is thread-safe.
func IsProduction() bool {
	return Get() == Production
}

// ShouldShowDebugEndpoints returns true if debug endpoints should be enabled.
// Debug endpoints (/debug/pprof/*, /debug/vars) are only available in development mode.
// This function is thread-safe.
func ShouldShowDebugEndpoints() bool {
	return IsDevelopment()
}

// GetErrorDetail returns error details based on the current mode.
// In development mode, returns the full error message with details.
// In production mode, returns a generic error message without internal details.
// This function is thread-safe.
func GetErrorDetail(err error) string {
	if err == nil {
		return ""
	}

	if IsDevelopment() {
		// Development mode: return full error details
		return err.Error()
	}

	// Production mode: return generic error message
	return "An internal error occurred"
}

// GetCacheHeaders returns appropriate cache headers based on the current mode.
// In development mode: no-cache headers to ensure fresh content
// In production mode: aggressive caching headers for static files
// This function is thread-safe.
func GetCacheHeaders() map[string]string {
	if IsDevelopment() {
		// Development mode: disable caching
		return map[string]string{
			"Cache-Control": "no-cache, no-store, must-revalidate",
			"Pragma":        "no-cache",
			"Expires":       "0",
		}
	}

	// Production mode: enable caching (1 year for static files)
	return map[string]string{
		"Cache-Control": "public, max-age=31536000, immutable",
	}
}

// ApplyCacheHeaders applies the appropriate cache headers to an HTTP response
// based on the current mode.
// This function is thread-safe.
func ApplyCacheHeaders(w http.ResponseWriter) {
	headers := GetCacheHeaders()
	for key, value := range headers {
		w.Header().Set(key, value)
	}
}

// GetLogLevel returns the appropriate log level for the current mode.
// Development mode: "debug"
// Production mode: "info"
// This function is thread-safe.
func GetLogLevel() string {
	if IsDevelopment() {
		return "debug"
	}
	return "info"
}

// ShouldCacheTemplates returns true if templates should be cached.
// Templates are cached in production mode but not in development mode
// to allow for live editing.
// This function is thread-safe.
func ShouldCacheTemplates() bool {
	return IsProduction()
}

// ShouldCacheStaticFiles returns true if static files should be cached.
// Static files are cached in production mode but not in development mode
// to allow for live editing.
// This function is thread-safe.
func ShouldCacheStaticFiles() bool {
	return IsProduction()
}

// ShouldAutoReload returns true if auto-reload should be enabled.
// Auto-reload is enabled in development mode but disabled in production mode.
// This function is thread-safe.
func ShouldAutoReload() bool {
	return IsDevelopment()
}

// ShouldEnableProfiling returns true if profiling endpoints should be enabled.
// Profiling is available at /debug/pprof/* in development mode only.
// This function is thread-safe.
func ShouldEnableProfiling() bool {
	return IsDevelopment()
}

// String returns the string representation of the mode.
func (m Mode) String() string {
	return string(m)
}
