package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetConfigState(t *testing.T) {
	t.Helper()
	mu.Lock()
	current = nil
	configPath = ""
	mu.Unlock()
}

func TestGetWithNoConfigLoaded(t *testing.T) {
	resetConfigState(t)
	defer resetConfigState(t)

	cfg := Get()
	if cfg == nil {
		t.Fatal("expected Get to return a default config, got nil")
	}
	if cfg.WebUI.Theme != "dark" {
		t.Errorf("expected default theme dark, got %q", cfg.WebUI.Theme)
	}
}

func TestSaveCurrentWithNoConfigLoaded(t *testing.T) {
	resetConfigState(t)
	defer resetConfigState(t)

	if err := SaveCurrent(); err == nil {
		t.Fatal("expected error when no configuration is loaded")
	}
}

func TestSetPortWithNoConfigLoaded(t *testing.T) {
	resetConfigState(t)
	defer resetConfigState(t)

	if err := SetPort("9999"); err != nil {
		t.Fatalf("SetPort returned error: %v", err)
	}
	if Get().Server.Port != "9999" {
		t.Errorf("expected port 9999, got %q", Get().Server.Port)
	}
}

func TestSaveConfigInvalidDirectory(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}
	// blocker is a file, not a directory, so MkdirAll underneath it must fail.
	path := filepath.Join(blocker, "sub", "config.yml")

	if err := Save(path, DefaultConfig()); err == nil {
		t.Fatal("expected error saving config under a file path")
	}
}

func TestSaveConfigWriteFileFails(t *testing.T) {
	dir := t.TempDir()
	// path is itself a directory, so os.WriteFile underneath saveConfig must fail.
	path := filepath.Join(dir, "config.yml")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory at config path: %v", err)
	}

	if err := Save(path, DefaultConfig()); err == nil {
		t.Fatal("expected error writing config to a directory path")
	}
}

func TestLoadReadFileFails(t *testing.T) {
	dir := t.TempDir()
	// path is a directory, so os.Stat succeeds but os.ReadFile must fail.
	path := filepath.Join(dir, "config.yml")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory at config path: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected error reading config from a directory path")
	}
}

func TestMigrateYmlToYamlMigratesFromSibling(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	ymlPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(yamlPath, []byte("server:\n  port: \"5555\"\n"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(ymlPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Port != "5555" {
		t.Errorf("expected port 5555, got %q", cfg.Server.Port)
	}
	if _, err := os.Stat(ymlPath); err != nil {
		t.Errorf("expected migrated .yml file to exist: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Address != "0.0.0.0" {
		t.Errorf("expected default address 0.0.0.0, got %q", cfg.Server.Address)
	}
	if cfg.Server.Mode != "production" {
		t.Errorf("expected default mode production, got %q", cfg.Server.Mode)
	}
	if !cfg.Server.Schedule.Enabled {
		t.Error("expected schedule enabled by default")
	}
	if cfg.WebUI.Theme != "dark" {
		t.Errorf("expected default theme dark, got %q", cfg.WebUI.Theme)
	}
	if cfg.WebSecurity.CORS != "*" {
		t.Errorf("expected default CORS *, got %q", cfg.WebSecurity.CORS)
	}
}

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebUI.Theme != "dark" {
		t.Errorf("expected default theme, got %q", cfg.WebUI.Theme)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
}

func TestLoadExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `server:
  port: "9999"
  address: "127.0.0.1"
web-ui:
  theme: "light"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Port != "9999" {
		t.Errorf("expected port 9999, got %q", cfg.Server.Port)
	}
	if cfg.WebUI.Theme != "light" {
		t.Errorf("expected theme light, got %q", cfg.WebUI.Theme)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	if err := os.WriteFile(path, []byte("not: [valid: yaml"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestMigrateYamlToYml(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte("server:\n  port: \"1234\"\n"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Port != "1234" {
		t.Errorf("expected port 1234, got %q", cfg.Server.Port)
	}

	ymlPath := filepath.Join(dir, "config.yml")
	if _, err := os.Stat(ymlPath); err != nil {
		t.Errorf("expected migrated .yml file to exist: %v", err)
	}
	if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
		t.Errorf("expected original .yaml file to be renamed away")
	}
}

func TestSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg := DefaultConfig()
	cfg.Server.Port = "8080"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got := Get()
	if got.Server.Port != "8080" {
		t.Errorf("expected port 8080, got %q", got.Server.Port)
	}
}

func TestSaveNilConfig(t *testing.T) {
	if err := Save("", nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestSaveCurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg := DefaultConfig()
	cfg.Server.Port = "7000"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if err := SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}
	if !strings.Contains(string(data), "7000") {
		t.Error("expected saved config file to contain updated port")
	}
}

func TestSetPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if err := SetPort("4242"); err != nil {
		t.Fatalf("SetPort returned error: %v", err)
	}
	if Get().Server.Port != "4242" {
		t.Errorf("expected port 4242, got %q", Get().Server.Port)
	}
}

func TestGetTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if _, err := Load(path); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if GetTheme() != "dark" {
		t.Errorf("expected theme dark, got %q", GetTheme())
	}
}

func TestGetCORS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg := DefaultConfig()
	cfg.WebSecurity.CORS = ""
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if GetCORS() != "*" {
		t.Errorf("expected fallback CORS *, got %q", GetCORS())
	}

	cfg.WebSecurity.CORS = "https://example.com"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if GetCORS() != "https://example.com" {
		t.Errorf("expected configured CORS, got %q", GetCORS())
	}
}

func TestFormatStringSlice(t *testing.T) {
	if got := formatStringSlice(nil); got != "[]" {
		t.Errorf("expected [], got %q", got)
	}
	if got := formatStringSlice([]string{"/a", "/b"}); got != `["/a", "/b"]` {
		t.Errorf("unexpected result: %q", got)
	}
}
