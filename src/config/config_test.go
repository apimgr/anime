package config

import (
	"os"
	"path/filepath"
	"strconv"
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

	if cfg.Server.Address != "[::]" {
		t.Errorf("expected default address [::], got %q", cfg.Server.Address)
	}
	if cfg.Server.Mode != "production" {
		t.Errorf("expected default mode production, got %q", cfg.Server.Mode)
	}
	if cfg.Server.Update.Branch != "stable" {
		t.Errorf("expected stable update channel, got %q", cfg.Server.Update.Branch)
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

func TestLoadLegacyUpdateChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  update_branch: daily\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Update.Branch != "daily" {
		t.Fatalf("legacy update channel = %q", cfg.Server.Update.Branch)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "update_branch:") || !strings.Contains(string(content), "branch: \"daily\"") {
		t.Fatalf("canonical update channel not written: %s", content)
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
	if !strings.HasPrefix(cfg.Server.Token, "tok_") || len(cfg.Server.Token) != 36 {
		t.Errorf("expected a generated 32-character operator token, got %q", cfg.Server.Token)
	}
	port, err := strconv.Atoi(cfg.Server.Port)
	if err != nil || port < 64000 || port > 64999 {
		t.Errorf("expected default port in 64000-64999, got %q", cfg.Server.Port)
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
	if !strings.HasPrefix(cfg.Server.Token, "tok_") || len(cfg.Server.Token) != 36 {
		t.Errorf("expected missing operator token to be generated, got %q", cfg.Server.Token)
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

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  unknown_setting: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "not found in type") {
		t.Fatalf("Load error = %v, want unknown field error", err)
	}
}

func TestLoadUsesFlexibleBooleanValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.yml")
	content := "server:\n  port: \"64580\"\n  schedule:\n    enabled: aye\n  metrics:\n    enabled: nope\n    include_system: on\n    include_app: deny\nweb-ui:\n  notifications:\n    enabled: yup\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Server.Schedule.Enabled || cfg.Server.Metrics.Enabled || !cfg.Server.Metrics.IncludeSystem || cfg.Server.Metrics.IncludeApp || !cfg.WebUI.Notifications.Enabled {
		t.Fatalf("flexible boolean settings parsed unexpectedly: %#v", cfg)
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

func TestParseBool(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "YES", want: true},
		{value: "enabled", want: true},
		{value: "oui", want: true},
		{value: "0", want: false},
		{value: "No", want: false},
		{value: "disabled", want: false},
		{value: "niet", want: false},
	} {
		got, err := ParseBool(test.value, !test.want)
		if err != nil {
			t.Fatalf("ParseBool(%q) returned error: %v", test.value, err)
		}
		if got != test.want {
			t.Errorf("ParseBool(%q) = %v, want %v", test.value, got, test.want)
		}
	}

	if got, err := ParseBool("", true); err != nil || !got {
		t.Errorf("ParseBool empty default = %v, %v; want true, nil", got, err)
	}
	if _, err := ParseBool("maybe", false); err == nil {
		t.Error("ParseBool accepted an invalid value")
	}
}
