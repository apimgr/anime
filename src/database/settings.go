package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Setting represents a key-value configuration setting
type Setting struct {
	Key           string
	Value         string
	Type          string // "string", "int", "bool", "json"
	Category      string // "cors", "rate_limiting", "request_limits", "connection_limits", "security"
	Description   string
	RequiresReload bool
	UpdatedAt     time.Time
}

// GetSetting retrieves a setting value by key
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting stores or updates a setting value
func (db *DB) SetSetting(key, value string) error {
	_, err := db.conn.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP
	`, key, value, value)
	return err
}

// GetSettingWithTimestamp retrieves a setting with its metadata
func (db *DB) GetSettingWithTimestamp(key string) (*Setting, error) {
	var setting Setting
	err := db.conn.QueryRow(`
		SELECT key, value, COALESCE(type, 'string'), COALESCE(category, ''),
		       COALESCE(description, ''), COALESCE(requires_reload, 0), updated_at
		FROM settings
		WHERE key = ?
	`, key).Scan(&setting.Key, &setting.Value, &setting.Type, &setting.Category,
		&setting.Description, &setting.RequiresReload, &setting.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query setting: %w", err)
	}

	return &setting, nil
}

// GetAllSettings retrieves all settings
func (db *DB) GetAllSettings() (map[string]string, error) {
	rows, err := db.conn.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("failed to query settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating settings: %w", err)
	}

	return settings, nil
}

// DeleteSetting removes a setting by key
func (db *DB) DeleteSetting(key string) error {
	result, err := db.conn.Exec("DELETE FROM settings WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete setting: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("setting not found")
	}

	return nil
}

// SetSettingJSON stores a JSON-serializable value as a setting
func (db *DB) SetSettingJSON(key string, value interface{}) error {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return db.SetSetting(key, string(jsonBytes))
}

// GetSettingJSON retrieves and unmarshals a JSON setting
func (db *DB) GetSettingJSON(key string, target interface{}) error {
	value, err := db.GetSetting(key)
	if err != nil {
		return err
	}
	if value == "" {
		return nil
	}

	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// GetAllSettingsDetailed retrieves all settings with full metadata
func (db *DB) GetAllSettingsDetailed() ([]*Setting, error) {
	rows, err := db.conn.Query(`
		SELECT key, value, COALESCE(type, 'string'), COALESCE(category, ''),
		       COALESCE(description, ''), COALESCE(requires_reload, 0), updated_at
		FROM settings
		ORDER BY category, key
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query settings: %w", err)
	}
	defer rows.Close()

	var settings []*Setting
	for rows.Next() {
		var setting Setting
		if err := rows.Scan(&setting.Key, &setting.Value, &setting.Type, &setting.Category,
			&setting.Description, &setting.RequiresReload, &setting.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings = append(settings, &setting)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating settings: %w", err)
	}

	return settings, nil
}

// GetSettingsByCategory retrieves all settings in a specific category
func (db *DB) GetSettingsByCategory(category string) ([]*Setting, error) {
	rows, err := db.conn.Query(`
		SELECT key, value, COALESCE(type, 'string'), COALESCE(category, ''),
		       COALESCE(description, ''), COALESCE(requires_reload, 0), updated_at
		FROM settings
		WHERE category = ?
		ORDER BY key
	`, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query settings: %w", err)
	}
	defer rows.Close()

	var settings []*Setting
	for rows.Next() {
		var setting Setting
		if err := rows.Scan(&setting.Key, &setting.Value, &setting.Type, &setting.Category,
			&setting.Description, &setting.RequiresReload, &setting.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings = append(settings, &setting)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating settings: %w", err)
	}

	return settings, nil
}

// SetSettingDetailed stores or updates a setting with full metadata
func (db *DB) SetSettingDetailed(setting *Setting) error {
	_, err := db.conn.Exec(`
		INSERT INTO settings (key, value, type, category, description, requires_reload, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = ?,
			type = ?,
			category = ?,
			description = ?,
			requires_reload = ?,
			updated_at = CURRENT_TIMESTAMP
	`, setting.Key, setting.Value, setting.Type, setting.Category, setting.Description, setting.RequiresReload,
		setting.Value, setting.Type, setting.Category, setting.Description, setting.RequiresReload)
	return err
}

// GetBoolSetting retrieves a boolean setting (type-safe)
func (db *DB) GetBoolSetting(key string, defaultValue bool) bool {
	value, err := db.GetSetting(key)
	if err != nil || value == "" {
		return defaultValue
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetIntSetting retrieves an integer setting (type-safe)
func (db *DB) GetIntSetting(key string, defaultValue int) int {
	value, err := db.GetSetting(key)
	if err != nil || value == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetStringArraySetting retrieves a string array setting (type-safe)
func (db *DB) GetStringArraySetting(key string, defaultValue []string) []string {
	var result []string
	err := db.GetSettingJSON(key, &result)
	if err != nil || len(result) == 0 {
		return defaultValue
	}
	return result
}

// InitializeDefaultSettings creates default settings if they don't exist
func (db *DB) InitializeDefaultSettings() error {
	defaults := []Setting{
		// CORS Settings
		{Key: "cors_enabled", Value: "true", Type: "bool", Category: "cors", Description: "Enable CORS support", RequiresReload: false},
		{Key: "cors_allowed_origins", Value: `["*"]`, Type: "json", Category: "cors", Description: "Allowed CORS origins", RequiresReload: false},
		{Key: "cors_allowed_methods", Value: `["GET","POST","PUT","DELETE","OPTIONS"]`, Type: "json", Category: "cors", Description: "Allowed HTTP methods", RequiresReload: false},
		{Key: "cors_allowed_headers", Value: `["Content-Type","Authorization"]`, Type: "json", Category: "cors", Description: "Allowed request headers", RequiresReload: false},
		{Key: "cors_allow_credentials", Value: "false", Type: "bool", Category: "cors", Description: "Allow credentials in CORS requests", RequiresReload: false},
		{Key: "cors_max_age", Value: "3600", Type: "int", Category: "cors", Description: "Preflight cache duration (seconds)", RequiresReload: false},

		// Rate Limiting Settings
		{Key: "rate_limit_enabled", Value: "true", Type: "bool", Category: "rate_limiting", Description: "Enable rate limiting", RequiresReload: true},
		{Key: "rate_limit_global_rps", Value: "100", Type: "int", Category: "rate_limiting", Description: "Global requests per second", RequiresReload: true},
		{Key: "rate_limit_global_burst", Value: "200", Type: "int", Category: "rate_limiting", Description: "Global burst capacity", RequiresReload: true},
		{Key: "rate_limit_api_rps", Value: "50", Type: "int", Category: "rate_limiting", Description: "API requests per second", RequiresReload: true},
		{Key: "rate_limit_api_burst", Value: "100", Type: "int", Category: "rate_limiting", Description: "API burst capacity", RequiresReload: true},
		{Key: "rate_limit_admin_rps", Value: "10", Type: "int", Category: "rate_limiting", Description: "Admin requests per second", RequiresReload: true},
		{Key: "rate_limit_admin_burst", Value: "20", Type: "int", Category: "rate_limiting", Description: "Admin burst capacity", RequiresReload: true},

		// Request Limits
		{Key: "request_timeout", Value: "60", Type: "int", Category: "request_limits", Description: "Request timeout (seconds)", RequiresReload: true},
		{Key: "request_max_body_size", Value: "10485760", Type: "int", Category: "request_limits", Description: "Max request body size (bytes, 10MB)", RequiresReload: true},
		{Key: "request_max_header_size", Value: "1048576", Type: "int", Category: "request_limits", Description: "Max header size (bytes, 1MB)", RequiresReload: true},

		// Connection Limits
		{Key: "connection_max_concurrent", Value: "1000", Type: "int", Category: "connection_limits", Description: "Max concurrent connections", RequiresReload: true},
		{Key: "connection_idle_timeout", Value: "120", Type: "int", Category: "connection_limits", Description: "Idle connection timeout (seconds)", RequiresReload: true},
		{Key: "connection_read_timeout", Value: "10", Type: "int", Category: "connection_limits", Description: "Read timeout (seconds)", RequiresReload: true},
		{Key: "connection_write_timeout", Value: "10", Type: "int", Category: "connection_limits", Description: "Write timeout (seconds)", RequiresReload: true},

		// Security Headers
		{Key: "security_frame_options", Value: "DENY", Type: "string", Category: "security", Description: "X-Frame-Options header", RequiresReload: false},
		{Key: "security_content_type_options", Value: "nosniff", Type: "string", Category: "security", Description: "X-Content-Type-Options header", RequiresReload: false},
		{Key: "security_xss_protection", Value: "1; mode=block", Type: "string", Category: "security", Description: "X-XSS-Protection header", RequiresReload: false},
		{Key: "security_referrer_policy", Value: "strict-origin-when-cross-origin", Type: "string", Category: "security", Description: "Referrer-Policy header", RequiresReload: false},
		{Key: "security_permissions_policy", Value: "geolocation=(), microphone=(), camera=()", Type: "string", Category: "security", Description: "Permissions-Policy header", RequiresReload: false},
		{Key: "security_csp_enabled", Value: "true", Type: "bool", Category: "security", Description: "Enable Content Security Policy", RequiresReload: false},
		{Key: "security_csp", Value: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'", Type: "string", Category: "security", Description: "Content-Security-Policy header", RequiresReload: false},
		{Key: "security_hsts_enabled", Value: "true", Type: "bool", Category: "security", Description: "Enable HSTS (HTTPS only)", RequiresReload: false},
		{Key: "security_hsts_max_age", Value: "31536000", Type: "int", Category: "security", Description: "HSTS max-age (seconds)", RequiresReload: false},
		{Key: "security_hsts_include_subdomains", Value: "true", Type: "bool", Category: "security", Description: "HSTS includeSubDomains", RequiresReload: false},
	}

	for _, setting := range defaults {
		// Only set if key doesn't exist
		existing, err := db.GetSetting(setting.Key)
		if err != nil {
			return fmt.Errorf("failed to check setting %s: %w", setting.Key, err)
		}
		if existing == "" {
			if err := db.SetSettingDetailed(&setting); err != nil {
				return fmt.Errorf("failed to set default setting %s: %w", setting.Key, err)
			}
		}
	}

	return nil
}
