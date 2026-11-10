package config

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
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
	Port               string         `yaml:"port"`
	Token              string         `yaml:"token"`
	FQDN               string         `yaml:"fqdn"`
	Address            string         `yaml:"address"`
	BaseURL            string         `yaml:"baseurl"`
	Mode               string         `yaml:"mode"`
	Update             UpdateConfig   `yaml:"update"`
	LegacyUpdateBranch string         `yaml:"update_branch,omitempty"`
	Schedule           ScheduleConfig `yaml:"schedule"`
	Metrics            MetricsConfig  `yaml:"metrics"`
	Logging            LoggingConfig  `yaml:"logging"`
	Database           DatabaseConfig `yaml:"database"`
}

type DatabaseConfig struct {
	Driver string       `yaml:"driver"`
	URL    string       `yaml:"url"`
	Pool   DatabasePool `yaml:"pool"`
}

type DatabasePool struct {
	MaxOpen     int    `yaml:"max_open"`
	MaxIdle     int    `yaml:"max_idle"`
	MaxLifetime string `yaml:"max_lifetime"`
	MaxIdleTime string `yaml:"max_idle_time"`
}

type UpdateConfig struct {
	Branch        string  `yaml:"branch"`
	AutoInstall   Boolean `yaml:"auto_install"`
	DeferDays     int     `yaml:"defer_days"`
	CLIMinVersion string  `yaml:"cli_min_version"`
}

// ScheduleConfig contains scheduler settings
type ScheduleConfig struct {
	Enabled       Boolean `yaml:"enabled"`
	Notifications string  `yaml:"notifications"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled       Boolean `yaml:"enabled"`
	Endpoint      string  `yaml:"endpoint"`
	IncludeSystem Boolean `yaml:"include_system"`
	IncludeApp    Boolean `yaml:"include_app"`
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
	Enabled       Boolean  `yaml:"enabled"`
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
			Port:    "",
			FQDN:    "",
			Address: "[::]",
			BaseURL: "/",
			Mode:    "production",
			Database: DatabaseConfig{
				Driver: "sqlite",
				Pool: DatabasePool{
					MaxOpen: 25, MaxIdle: 5, MaxLifetime: "5m", MaxIdleTime: "1m",
				},
			},
			Update: UpdateConfig{Branch: "stable"},
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
		port, err := SelectAvailablePort()
		if err != nil {
			return nil, fmt.Errorf("failed to select a default server port: %w", err)
		}
		cfg.Server.Port = port
		token, err := GenerateOperatorToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate operator token: %w", err)
		}
		cfg.Server.Token = token
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
	cfg.Server.Update.Branch = ""
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = cfg.Server.LegacyUpdateBranch
	}
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = "stable"
	}
	needsSave := false
	if strings.TrimSpace(cfg.Server.Port) == "" {
		port, err := SelectAvailablePort()
		if err != nil {
			return nil, fmt.Errorf("failed to select a default server port: %w", err)
		}
		cfg.Server.Port = port
		needsSave = true
	}
	if strings.TrimSpace(cfg.Server.Token) == "" {
		token, err := GenerateOperatorToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate operator token: %w", err)
		}
		cfg.Server.Token = token
		needsSave = true
	}
	if needsSave {
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to save generated configuration defaults: %w", err)
		}
	}
	cfg.Server.LegacyUpdateBranch = ""

	current = cfg
	return cfg, nil
}

// GenerateOperatorToken returns a fresh server-wide token using the required base62 alphabet.
func GenerateOperatorToken() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const tokenLength = 32
	token := make([]byte, tokenLength)
	limit := big.NewInt(int64(len(alphabet)))
	for i := range token {
		value, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		token[i] = alphabet[value.Int64()]
	}
	return "tok_" + string(token), nil
}

// SelectAvailablePort returns an unused port in the default 64000–64999 range.
func SelectAvailablePort() (string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		var randomValue [4]byte
		if _, err := rand.Read(randomValue[:]); err != nil {
			return "", fmt.Errorf("generate random port candidate: %w", err)
		}
		randomNumber := int(randomValue[0])*256 + int(randomValue[1])
		portNumber := 64000 + randomNumber%1000
		listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(portNumber)))
		if err != nil {
			continue
		}
		if err := listener.Close(); err != nil {
			return "", fmt.Errorf("release default port candidate: %w", err)
		}
		return strconv.Itoa(portNumber), nil
	}
	return "", fmt.Errorf("no unused port found in default range 64000–64999")
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
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = cfg.Server.LegacyUpdateBranch
	}
	if cfg.Server.Update.Branch == "" {
		cfg.Server.Update.Branch = "stable"
	}
	cfg.Server.LegacyUpdateBranch = ""
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
	dirMode := os.FileMode(0755)
	fileMode := os.FileMode(0644)
	if os.Geteuid() != 0 {
		dirMode = 0700
		fileMode = 0600
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	content := generateConfigYAML(cfg)
	if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	if err := os.Chmod(path, fileMode); err != nil {
		return fmt.Errorf("failed to set configuration file permissions: %w", err)
	}

	return nil
}

// generateConfigYAML generates YAML content with comments
func generateConfigYAML(cfg *Config) string {
	return fmt.Sprintf(`# Anime Quotes API Server Configuration
# Documentation: docs/configuration.md

server:
  port: "%s"
  token: "%s"
  fqdn: "%s"
  address: "%s"
  database:
    driver: "%s"
    url: "%s"
    pool:
      max_open: %d
      max_idle: %d
      max_lifetime: "%s"
      max_idle_time: "%s"
  baseurl: "%s"
  mode: "%s"
  update:
    branch: "%s"
    auto_install: %t
    defer_days: %d
    cli_min_version: "%s"

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
		cfg.Server.Token,
		cfg.Server.FQDN,
		cfg.Server.Address,
		cfg.Server.Database.Driver,
		cfg.Server.Database.URL,
		cfg.Server.Database.Pool.MaxOpen,
		cfg.Server.Database.Pool.MaxIdle,
		cfg.Server.Database.Pool.MaxLifetime,
		cfg.Server.Database.Pool.MaxIdleTime,
		cfg.Server.BaseURL,
		cfg.Server.Mode,
		cfg.Server.Update.Branch,
		cfg.Server.Update.AutoInstall,
		cfg.Server.Update.DeferDays,
		cfg.Server.Update.CLIMinVersion,
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
