package server

import (
	"log"
	"sync"
	"time"

	"github.com/apimgr/anime/src/database"
)

// SettingsManager manages server settings with live reload capability
type SettingsManager struct {
	db    *database.DB
	mu    sync.RWMutex
	cache map[string]string
}

// CORSSettings holds CORS configuration
type CORSSettings struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// RateLimitSettings holds rate limiting configuration
type RateLimitSettings struct {
	Enabled     bool
	GlobalRPS   int
	GlobalBurst int
	APIRPS      int
	APIBurst    int
	AdminRPS    int
	AdminBurst  int
}

// RequestLimitSettings holds request limit configuration
type RequestLimitSettings struct {
	Timeout       int // seconds
	MaxBodySize   int // bytes
	MaxHeaderSize int // bytes
}

// ConnectionLimitSettings holds connection limit configuration
type ConnectionLimitSettings struct {
	MaxConcurrent int
	IdleTimeout   int // seconds
	ReadTimeout   int // seconds
	WriteTimeout  int // seconds
}

// SecurityHeaderSettings holds security header configuration
type SecurityHeaderSettings struct {
	FrameOptions           string
	ContentTypeOptions     string
	XSSProtection          string
	ReferrerPolicy         string
	PermissionsPolicy      string
	CSPEnabled             bool
	CSP                    string
	HSTSEnabled            bool
	HSTSMaxAge             int
	HSTSIncludeSubdomains  bool
}

// NewSettingsManager creates a new settings manager
func NewSettingsManager(db *database.DB) *SettingsManager {
	sm := &SettingsManager{
		db:    db,
		cache: make(map[string]string),
	}

	// Load settings into cache
	if err := sm.LoadSettings(); err != nil {
		log.Printf("Warning: Failed to load settings: %v", err)
	}

	return sm
}

// LoadSettings loads all settings from database into cache
func (sm *SettingsManager) LoadSettings() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	settings, err := sm.db.GetAllSettings()
	if err != nil {
		return err
	}

	sm.cache = settings
	log.Printf("Loaded %d settings into cache", len(settings))
	return nil
}

// ReloadSettings reloads settings from database (for live updates)
func (sm *SettingsManager) ReloadSettings() error {
	return sm.LoadSettings()
}

// GetSetting retrieves a setting from cache
func (sm *SettingsManager) GetSetting(key string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.cache[key]
}

// GetCORS returns CORS settings
func (sm *SettingsManager) GetCORS() CORSSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settings := CORSSettings{
		Enabled:          sm.db.GetBoolSetting("cors_enabled", true),
		AllowedOrigins:   sm.db.GetStringArraySetting("cors_allowed_origins", []string{"*"}),
		AllowedMethods:   sm.db.GetStringArraySetting("cors_allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		AllowedHeaders:   sm.db.GetStringArraySetting("cors_allowed_headers", []string{"Content-Type", "Authorization"}),
		AllowCredentials: sm.db.GetBoolSetting("cors_allow_credentials", false),
		MaxAge:           sm.db.GetIntSetting("cors_max_age", 3600),
	}

	return settings
}

// GetRateLimits returns rate limiting settings
func (sm *SettingsManager) GetRateLimits() RateLimitSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settings := RateLimitSettings{
		Enabled:     sm.db.GetBoolSetting("rate_limit_enabled", true),
		GlobalRPS:   sm.db.GetIntSetting("rate_limit_global_rps", 100),
		GlobalBurst: sm.db.GetIntSetting("rate_limit_global_burst", 200),
		APIRPS:      sm.db.GetIntSetting("rate_limit_api_rps", 50),
		APIBurst:    sm.db.GetIntSetting("rate_limit_api_burst", 100),
		AdminRPS:    sm.db.GetIntSetting("rate_limit_admin_rps", 10),
		AdminBurst:  sm.db.GetIntSetting("rate_limit_admin_burst", 20),
	}

	return settings
}

// GetRequestLimits returns request limit settings
func (sm *SettingsManager) GetRequestLimits() RequestLimitSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settings := RequestLimitSettings{
		Timeout:       sm.db.GetIntSetting("request_timeout", 60),
		MaxBodySize:   sm.db.GetIntSetting("request_max_body_size", 10485760),
		MaxHeaderSize: sm.db.GetIntSetting("request_max_header_size", 1048576),
	}

	return settings
}

// GetConnectionLimits returns connection limit settings
func (sm *SettingsManager) GetConnectionLimits() ConnectionLimitSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settings := ConnectionLimitSettings{
		MaxConcurrent: sm.db.GetIntSetting("connection_max_concurrent", 1000),
		IdleTimeout:   sm.db.GetIntSetting("connection_idle_timeout", 120),
		ReadTimeout:   sm.db.GetIntSetting("connection_read_timeout", 10),
		WriteTimeout:  sm.db.GetIntSetting("connection_write_timeout", 10),
	}

	return settings
}

// GetSecurityHeaders returns security header settings
func (sm *SettingsManager) GetSecurityHeaders() SecurityHeaderSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	frameOptions, _ := sm.db.GetSetting("security_frame_options")
	if frameOptions == "" {
		frameOptions = "DENY"
	}

	contentTypeOptions, _ := sm.db.GetSetting("security_content_type_options")
	if contentTypeOptions == "" {
		contentTypeOptions = "nosniff"
	}

	xssProtection, _ := sm.db.GetSetting("security_xss_protection")
	if xssProtection == "" {
		xssProtection = "1; mode=block"
	}

	referrerPolicy, _ := sm.db.GetSetting("security_referrer_policy")
	if referrerPolicy == "" {
		referrerPolicy = "strict-origin-when-cross-origin"
	}

	permissionsPolicy, _ := sm.db.GetSetting("security_permissions_policy")
	if permissionsPolicy == "" {
		permissionsPolicy = "geolocation=(), microphone=(), camera=()"
	}

	csp, _ := sm.db.GetSetting("security_csp")
	if csp == "" {
		csp = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'"
	}

	settings := SecurityHeaderSettings{
		FrameOptions:          frameOptions,
		ContentTypeOptions:    contentTypeOptions,
		XSSProtection:         xssProtection,
		ReferrerPolicy:        referrerPolicy,
		PermissionsPolicy:     permissionsPolicy,
		CSPEnabled:            sm.db.GetBoolSetting("security_csp_enabled", true),
		CSP:                   csp,
		HSTSEnabled:           sm.db.GetBoolSetting("security_hsts_enabled", true),
		HSTSMaxAge:            sm.db.GetIntSetting("security_hsts_max_age", 31536000),
		HSTSIncludeSubdomains: sm.db.GetBoolSetting("security_hsts_include_subdomains", true),
	}

	return settings
}

// UpdateSetting updates a single setting and reloads cache
func (sm *SettingsManager) UpdateSetting(key, value string) error {
	if err := sm.db.SetSetting(key, value); err != nil {
		return err
	}

	// Reload cache
	return sm.ReloadSettings()
}

// ExportSettings exports all settings as a map
func (sm *SettingsManager) ExportSettings() (map[string]interface{}, error) {
	settings, err := sm.db.GetAllSettingsDetailed()
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for _, setting := range settings {
		result[setting.Key] = map[string]interface{}{
			"value":          setting.Value,
			"type":           setting.Type,
			"category":       setting.Category,
			"description":    setting.Description,
			"requires_reload": setting.RequiresReload,
			"updated_at":     setting.UpdatedAt.Format(time.RFC3339),
		}
	}

	return result, nil
}

// ImportSettings imports settings from a map
func (sm *SettingsManager) ImportSettings(settings map[string]string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for key, value := range settings {
		if err := sm.db.SetSetting(key, value); err != nil {
			log.Printf("Warning: Failed to import setting %s: %v", key, err)
		}
	}

	// Reload cache
	return sm.LoadSettings()
}

// ResetToDefaults resets all settings to their default values
func (sm *SettingsManager) ResetToDefaults() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Delete all existing settings
	allSettings, err := sm.db.GetAllSettings()
	if err != nil {
		return err
	}

	for key := range allSettings {
		if err := sm.db.DeleteSetting(key); err != nil {
			log.Printf("Warning: Failed to delete setting %s: %v", key, err)
		}
	}

	// Reinitialize defaults
	if err := sm.db.InitializeDefaultSettings(); err != nil {
		return err
	}

	// Reload cache
	return sm.LoadSettings()
}
