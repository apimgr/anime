package mode

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/apimgr/anime/src/config"
)

type AppMode string

const (
	AppModeProduction  AppMode = "production"
	AppModeDevelopment AppMode = "development"
	AppModeDebug       AppMode = "debug"
)

var (
	stateMu      sync.RWMutex
	currentMode  = AppModeProduction
	debugEnabled bool
)

// ParseAppMode normalizes a supported application mode name.
func ParseAppMode(value string) (AppMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production":
		return AppModeProduction, nil
	case "dev", "devel", "development":
		return AppModeDevelopment, nil
	case "debug":
		return AppModeDebug, nil
	default:
		return "", fmt.Errorf("invalid application mode %q; expected production, development, or debug", value)
	}
}

// ConfigureAppMode applies CLI, environment, and default mode/debug precedence.
func ConfigureAppMode(modeFlag string, modeFlagSet bool, debugFlag bool, debugFlagSet bool) error {
	return ConfigureAppModeWithDefault(string(AppModeProduction), modeFlag, modeFlagSet, debugFlag, debugFlagSet)
}

// ConfigureAppModeWithDefault applies CLI, environment, file, and default precedence.
func ConfigureAppModeWithDefault(modeDefault string, modeFlag string, modeFlagSet bool, debugFlag bool, debugFlagSet bool) error {
	modeValue := modeDefault
	if modeValue == "" {
		modeValue = string(AppModeProduction)
	}
	if environmentMode := os.Getenv("MODE"); environmentMode != "" {
		modeValue = environmentMode
	}
	if modeFlagSet {
		modeValue = modeFlag
	}
	selectedMode := AppModeProduction
	if modeValue != "" {
		parsedMode, err := ParseAppMode(modeValue)
		if err != nil {
			return err
		}
		selectedMode = parsedMode
	}

	selectedDebug := selectedMode == AppModeDebug
	if debugValue, isSet := os.LookupEnv("DEBUG"); isSet {
		parsedDebug, err := config.ParseBool(debugValue, selectedDebug)
		if err != nil {
			return fmt.Errorf("invalid DEBUG environment value: %w", err)
		}
		selectedDebug = parsedDebug
	}
	if debugFlagSet {
		selectedDebug = debugFlag
	}

	stateMu.Lock()
	currentMode = selectedMode
	debugEnabled = selectedDebug
	stateMu.Unlock()
	updateProfilingSettings(selectedDebug)
	return nil
}

// SetAppMode sets an application mode and applies its mode-specific defaults.
func SetAppMode(value string) error {
	selectedMode, err := ParseAppMode(value)
	if err != nil {
		return err
	}
	stateMu.Lock()
	currentMode = selectedMode
	debugEnabled = selectedMode == AppModeDebug
	selectedDebug := debugEnabled
	stateMu.Unlock()
	updateProfilingSettings(selectedDebug)
	return nil
}

// SetDebugEnabled explicitly controls diagnostics independently of app mode.
func SetDebugEnabled(enabled bool) {
	stateMu.Lock()
	debugEnabled = enabled
	stateMu.Unlock()
	updateProfilingSettings(enabled)
}

// GetCurrentAppMode returns the selected runtime mode.
func GetCurrentAppMode() AppMode {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return currentMode
}

// IsAppModeDev reports whether the selected runtime mode is development.
func IsAppModeDev() bool {
	return GetCurrentAppMode() == AppModeDevelopment
}

// IsAppModeProd reports whether the selected runtime mode is production.
func IsAppModeProd() bool {
	return GetCurrentAppMode() == AppModeProduction
}

// IsAppModeDebug reports whether the selected runtime mode is debug.
func IsAppModeDebug() bool {
	return GetCurrentAppMode() == AppModeDebug
}

// IsDebugEnabled reports whether explicit debug diagnostics are enabled.
func IsDebugEnabled() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return debugEnabled
}

// GetAppModeString returns the mode with its diagnostic state for startup output.
func GetAppModeString() string {
	appMode := GetCurrentAppMode()
	if IsDebugEnabled() {
		return string(appMode) + " [debugging]"
	}
	return string(appMode)
}

// ShouldShowDebugEndpoints reports whether the debug flag enables diagnostics.
func ShouldShowDebugEndpoints() bool {
	return IsDebugEnabled()
}

// GetClientErrorDetail selects detailed errors outside production mode.
func GetClientErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	if !IsAppModeProd() {
		return err.Error()
	}
	return "An internal error occurred"
}

// GetHTTPStaticCacheHeaders returns cache headers appropriate to the app mode.
func GetHTTPStaticCacheHeaders() map[string]string {
	if IsAppModeDev() || IsAppModeDebug() {
		return map[string]string{
			"Cache-Control": "no-cache, no-store, must-revalidate",
			"Pragma":        "no-cache",
			"Expires":       "0",
		}
	}
	return map[string]string{"Cache-Control": "public, max-age=31536000, immutable"}
}

// ApplyHTTPStaticCacheHeaders applies mode-specific static cache headers.
func ApplyHTTPStaticCacheHeaders(response http.ResponseWriter) {
	for name, value := range GetHTTPStaticCacheHeaders() {
		response.Header().Set(name, value)
	}
}

// GetAppModeLogLevel returns the default log level for the selected mode.
func GetAppModeLogLevel() string {
	if IsAppModeDev() || IsAppModeDebug() {
		return "debug"
	}
	return "info"
}

// ShouldAppModeCacheTemplates reports whether templates use the production cache.
func ShouldAppModeCacheTemplates() bool {
	return IsAppModeProd()
}

// ShouldAppModeCacheStaticFiles reports whether static files use the production cache.
func ShouldAppModeCacheStaticFiles() bool {
	return IsAppModeProd()
}

// ShouldAppModeAutoReload reports whether development template reloading is enabled.
func ShouldAppModeAutoReload() bool {
	return IsAppModeDev() || IsAppModeDebug()
}

// updateProfilingSettings enables profiling only when debug diagnostics are enabled.
func updateProfilingSettings(enabled bool) {
	if enabled {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		return
	}
	runtime.SetBlockProfileRate(0)
	runtime.SetMutexProfileFraction(0)
}
