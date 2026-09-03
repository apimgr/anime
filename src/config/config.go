package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config represents the complete server configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	WebUI       WebUIConfig       `yaml:"web-ui"`
	WebRobots   WebRobotsConfig   `yaml:"web-robots"`
	WebSecurity WebSecurityConfig `yaml:"web-security"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	Port         string         `yaml:"port"`
	FQDN         string         `yaml:"fqdn"`
	Address      string         `yaml:"address"`
	Mode         string         `yaml:"mode"`
	UpdateBranch string         `yaml:"update_branch"`
	Admin        AdminConfig    `yaml:"admin"`
	Session      SessionConfig  `yaml:"session"`
	Schedule     ScheduleConfig `yaml:"schedule"`
	Metrics      MetricsConfig  `yaml:"metrics"`
	Logging      LoggingConfig  `yaml:"logging"`
}

// AdminConfig contains admin panel settings
type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	APIToken string `yaml:"api_token"`
}

// SessionConfig contains session settings
type SessionConfig struct {
	Timeout int `yaml:"timeout"`
}

// ScheduleConfig contains scheduler settings
type ScheduleConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Notifications string `yaml:"notifications"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Endpoint      string `yaml:"endpoint"`
	IncludeSystem bool   `yaml:"include_system"`
	IncludeApp    bool   `yaml:"include_app"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	AccessFormat string `yaml:"access_format"`
	Level        string `yaml:"level"`
}

// WebUIConfig contains web UI settings
type WebUIConfig struct {
	Theme         string              `yaml:"theme"`
	Logo          string              `yaml:"logo"`
	Favicon       string              `yaml:"favicon"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

// NotificationsConfig contains notification settings
type NotificationsConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Announcements []string `yaml:"announcements"`
}

// WebRobotsConfig contains robots.txt settings
type WebRobotsConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// WebSecurityConfig contains security settings
type WebSecurityConfig struct {
	Admin string `yaml:"admin"`
	CORS  string `yaml:"cors"`
}

var (
	current    *Config
	mu         sync.RWMutex
	configPath string
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         "",
			FQDN:         "",
			Address:      "0.0.0.0",
			Mode:         "production",
			UpdateBranch: "stable",
			Admin: AdminConfig{
				Username: "admin",
				Password: "",
				APIToken: "",
			},
			Session: SessionConfig{
				Timeout: 3600,
			},
			Schedule: ScheduleConfig{
				Enabled:       true,
				Notifications: "hourly",
			},
			Metrics: MetricsConfig{
				Enabled:       false,
				Endpoint:      "/metrics",
				IncludeSystem: true,
				IncludeApp:    true,
			},
			Logging: LoggingConfig{
				AccessFormat: "apache",
				Level:        "info",
			},
		},
		WebUI: WebUIConfig{
			Theme:   "dark",
			Logo:    "",
			Favicon: "",
			Notifications: NotificationsConfig{
				Enabled:       true,
				Announcements: []string{},
			},
		},
		WebRobots: WebRobotsConfig{
			Allow: []string{"/", "/api"},
			Deny:  []string{"/debug"},
		},
		WebSecurity: WebSecurityConfig{
			Admin: "",
			CORS:  "*",
		},
	}
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	// Migrate .yaml to .yml if needed
	path = migrateYamlToYml(path)
	configPath = path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		current = cfg
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	current = cfg
	return cfg, nil
}

// migrateYamlToYml migrates config from .yaml to .yml extension
func migrateYamlToYml(path string) string {
	// If path ends with .yaml, check if we need to migrate
	if strings.HasSuffix(path, ".yaml") {
		ymlPath := strings.TrimSuffix(path, ".yaml") + ".yml"
		// If .yml doesn't exist but .yaml does, rename
		if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
			if _, err := os.Stat(path); err == nil {
				if err := os.Rename(path, ymlPath); err == nil {
					fmt.Printf("Migrated config from %s to %s\n", path, ymlPath)
				}
			}
		}
		return ymlPath
	}
	// If path ends with .yml, check for .yaml to migrate
	if strings.HasSuffix(path, ".yml") {
		yamlPath := strings.TrimSuffix(path, ".yml") + ".yaml"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if _, err := os.Stat(yamlPath); err == nil {
				if err := os.Rename(yamlPath, path); err == nil {
					fmt.Printf("Migrated config from %s to %s\n", yamlPath, path)
				}
			}
		}
	}
	return path
}

// Get returns the current configuration
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return DefaultConfig()
	}
	return current
}

// Save saves the specified configuration to the specified path
func Save(path string, cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	current = cfg
	if path != "" {
		configPath = path
	}
	return saveConfig(cfg, path)
}

// SaveCurrent saves the current configuration to file
func SaveCurrent() error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil || configPath == "" {
		return fmt.Errorf("no configuration loaded")
	}
	return saveConfig(current, configPath)
}

// SetPort updates the port in configuration
func SetPort(port string) error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = DefaultConfig()
	}
	current.Server.Port = port
	if configPath != "" {
		return saveConfig(current, configPath)
	}
	return nil
}

// saveConfig writes configuration to a YAML file
func saveConfig(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	content := generateConfigYAML(cfg)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// generateConfigYAML generates YAML content with comments
func generateConfigYAML(cfg *Config) string {
	return fmt.Sprintf(`# Anime Quotes API Server Configuration
# Documentation: https://anime.apimgr.us/docs

server:
  port: "%s"
  fqdn: "%s"
  address: "%s"

  schedule:
    enabled: %t
    notifications: "%s"

  metrics:
    enabled: %t
    endpoint: "%s"
    include_system: %t
    include_app: %t

  logging:
    access_format: "%s"
    level: "%s"

web-ui:
  theme: "%s"
  logo: "%s"
  favicon: "%s"
  notifications:
    enabled: %t
    announcements: %s

web-robots:
  allow: %s
  deny: %s

web-security:
  admin: "%s"
  cors: "%s"
`,
		cfg.Server.Port,
		cfg.Server.FQDN,
		cfg.Server.Address,
		cfg.Server.Schedule.Enabled,
		cfg.Server.Schedule.Notifications,
		cfg.Server.Metrics.Enabled,
		cfg.Server.Metrics.Endpoint,
		cfg.Server.Metrics.IncludeSystem,
		cfg.Server.Metrics.IncludeApp,
		cfg.Server.Logging.AccessFormat,
		cfg.Server.Logging.Level,
		cfg.WebUI.Theme,
		cfg.WebUI.Logo,
		cfg.WebUI.Favicon,
		cfg.WebUI.Notifications.Enabled,
		formatStringSlice(cfg.WebUI.Notifications.Announcements),
		formatStringSlice(cfg.WebRobots.Allow),
		formatStringSlice(cfg.WebRobots.Deny),
		cfg.WebSecurity.Admin,
		cfg.WebSecurity.CORS,
	)
}

func formatStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	result := "["
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("\"%s\"", v)
	}
	result += "]"
	return result
}

// GetTheme returns the current theme
func GetTheme() string {
	cfg := Get()
	return cfg.WebUI.Theme
}

// GetCORS returns the CORS setting
func GetCORS() string {
	cfg := Get()
	if cfg.WebSecurity.CORS == "" {
		return "*"
	}
	return cfg.WebSecurity.CORS
}
