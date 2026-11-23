package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func resetConfigState(t *testing.T) {
	t.Helper()
	mu.Lock()
	current = nil
	configPath = ""
	mu.Unlock()
}

func TestBrandingContentAcceptsPlainAndLocalizedBodies(t *testing.T) {
	var branding struct {
		Content map[string]BrandingPageContent `yaml:"content"`
	}
	input := `content:
  about:
    format: md
    body: Plain content
  terms:
    format: md
    body:
      default: Default terms
      fr: Conditions françaises
`
	if err := yaml.Unmarshal([]byte(input), &branding); err != nil {
		t.Fatalf("unmarshal branding content: %v", err)
	}
	if got := branding.Content["about"].Body.Resolve("fr"); got != "Plain content" {
		t.Fatalf("plain body fallback = %q", got)
	}
	if got := branding.Content["terms"].Body.Resolve("fr"); got != "Conditions françaises" {
		t.Fatalf("localized body = %q", got)
	}
	if got := branding.Content["terms"].Body.Resolve("de"); got != "Default terms" {
		t.Fatalf("default body fallback = %q", got)
	}
}

func TestPrivacyConfigValidationUsesSafeDefaults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Privacy.Consent.Position = "middle"
	cfg.Server.Privacy.Consent.Policy.URL = "javascript:alert(1)"
	cfg.Server.Privacy.Cookies.Essential.Description = ""
	if err := validateServerConfig(cfg); err != nil {
		t.Fatalf("validateServerConfig returned error: %v", err)
	}
	privacy := cfg.Server.Privacy
	if privacy.Consent.Position != "bottom" || privacy.Consent.Policy.URL != "/server/privacy" || privacy.Cookies.Essential.Description == "" || !privacy.Cookies.Essential.Enabled {
		t.Fatalf("unsafe privacy values were not replaced with defaults: %+v", privacy)
	}
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

func TestDefaultContactAntispamPolicy(t *testing.T) {
	policy := DefaultConfig().Server.Pages.Contact.Antispam
	if !bool(policy.Honeypot) || policy.MinFillSeconds != 3 || policy.MaxAgeSeconds != 43200 || policy.PerIP != 5 || policy.PerEmail != 3 || policy.Global != 60 || policy.Window != "1h" || policy.MaxLinks != 10 || !bool(policy.RejectMarkup) || policy.Strikes != 3 || policy.BanHours != 24 || policy.ReviewMinSeconds != 2 || policy.ReviewMaxSeconds != 900 || policy.DuplicateWindowHours != 24 {
		t.Fatalf("unexpected contact antispam defaults: %#v", policy)
	}
}

func TestSEOVerificationConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.SEO.Verification = VerificationSEOConfig{
		Google:    strings.Repeat("g", 43),
		Bing:      "ABCDEF0123456789",
		Yandex:    "abcdef0123456789",
		Baidu:     "Baidu123",
		Pinterest: "abcdef",
		Facebook:  "facebook123",
		Custom: []CustomVerificationMeta{
			{Name: "valid-tag_1", Content: "value <&"},
			{Name: "invalid:tag", Content: "ignored"},
			{Name: "missing-content"},
			{Name: "a", Property: "b", Content: "ignored"},
		},
	}
	cfg.Server.SEO.MetaTags = map[string]string{
		"description":  "ignored",
		"valid-name":   "value",
		"invalid name": "ignored",
	}
	if err := validateServerConfig(cfg); err != nil {
		t.Fatalf("validateServerConfig returned error: %v", err)
	}
	if cfg.Server.SEO.Verification.Google == "" || cfg.Server.SEO.Verification.Bing == "" || cfg.Server.SEO.Verification.Yandex == "" || cfg.Server.SEO.Verification.Baidu == "" || cfg.Server.SEO.Verification.Pinterest == "" || cfg.Server.SEO.Verification.Facebook == "" {
		t.Fatal("valid provider verification codes were rejected")
	}
	if len(cfg.Server.SEO.Verification.Custom) != 1 || cfg.Server.SEO.Verification.Custom[0].Name != "valid-tag_1" {
		t.Fatalf("custom verification validation produced %#v", cfg.Server.SEO.Verification.Custom)
	}
	if len(cfg.Server.SEO.MetaTags) != 1 || cfg.Server.SEO.MetaTags["valid-name"] != "value" {
		t.Fatalf("custom meta tag validation produced %#v", cfg.Server.SEO.MetaTags)
	}
	cfg.Server.SEO.Verification.Google = strings.Repeat("g", 44)
	if err := validateServerConfig(cfg); err != nil {
		t.Fatalf("validateServerConfig returned error for invalid provider tag: %v", err)
	}
	if cfg.Server.SEO.Verification.Google != "" {
		t.Fatal("invalid provider verification code was retained")
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
	if cfg.Server.I18n.DefaultLanguage != "en" || len(cfg.Server.I18n.AvailableLanguages) != 6 {
		t.Errorf("expected the default i18n configuration to enable the six required languages, got %#v", cfg.Server.I18n)
	}
	if cfg.Server.Branding.Title == "" || cfg.Server.Branding.Description == "" || cfg.Server.SEO.TitleSeparator != " - " {
		t.Errorf("expected branding and SEO defaults, got branding=%#v seo=%#v", cfg.Server.Branding, cfg.Server.SEO)
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
	if !strings.HasPrefix(cfg.Server.Token, "adm_") || len(cfg.Server.Token) != 36 {
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
  contact:
    dpo:
      name: "Privacy Officer"
      email: "dpo@example.test"
    general:
      email: "privacy@example.test"
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
	if cfg.Server.Contact.DPO.Name != "Privacy Officer" || cfg.Server.Contact.DPO.Email != "dpo@example.test" || cfg.Server.Contact.General.Email != "privacy@example.test" {
		t.Errorf("DPO contact configuration was not loaded: %+v", cfg.Server.Contact)
	}
	if !strings.HasPrefix(cfg.Server.Token, "adm_") || len(cfg.Server.Token) != 36 {
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

func TestLoadIgnoresUnknownFieldsAndKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  unknown_setting: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error for an unknown setting: %v", err)
	}
	port, err := strconv.Atoi(cfg.Server.Port)
	if err != nil || port < 64000 || port > 64999 {
		t.Fatalf("unknown setting produced invalid default port %q", cfg.Server.Port)
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

func TestLoadMergesOrderedYAMLDropins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  mode: production\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dropDir := filepath.Join(dir, "yaml.d")
	if err := os.MkdirAll(dropDir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"20-theme.yml": "web-ui:\n  theme: light\n",
		"10-theme.yml": "web-ui:\n  theme: dark\n",
	} {
		if err := os.WriteFile(filepath.Join(dropDir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Mode != "production" || cfg.WebUI.Theme != "light" {
		t.Fatalf("merged values: mode=%q theme=%q", cfg.Server.Mode, cfg.WebUI.Theme)
	}
}

func TestMigrateAreaSectionsAndKeepExistingDropinValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  metrics:\n    enabled: true\n  i18n:\n    default_language: fr\n    available_languages: [en, es, fr, de, it, ar]\n  mode: production\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dropDir := filepath.Join(dir, "yaml.d")
	if err := os.MkdirAll(dropDir, 0700); err != nil {
		t.Fatal(err)
	}
	areaPath := filepath.Join(dropDir, "metrics.yml")
	if err := os.WriteFile(areaPath, []byte("server:\n  metrics:\n    endpoint: /custom-metrics\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := migrateAreaFiles(path); err != nil {
		t.Fatalf("migrateAreaFiles returned error: %v", err)
	}
	root, err := readYAMLMap(path)
	if err != nil {
		t.Fatal(err)
	}
	server := root["server"].(map[string]interface{})
	if _, exists := server["metrics"]; exists {
		t.Fatal("metrics section remained in server.yml")
	}
	area, err := readYAMLMap(areaPath)
	if err != nil {
		t.Fatal(err)
	}
	metrics := area["server"].(map[string]interface{})["metrics"].(map[string]interface{})
	if metrics["enabled"] != true || metrics["endpoint"] != "/custom-metrics" {
		t.Fatalf("merged area metrics=%v", metrics)
	}
	webConfig, err := readYAMLMap(filepath.Join(dropDir, "web.yml"))
	if err != nil {
		t.Fatal(err)
	}
	i18nValues := webConfig["server"].(map[string]interface{})["i18n"].(map[string]interface{})
	if i18nValues["default_language"] != "fr" || len(i18nValues["available_languages"].([]interface{})) != 6 {
		t.Fatalf("migrated i18n values=%v", i18nValues)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected pre-migration backup: %v", err)
	}
}

func TestGeneratedOperatorTokenIsStoredOnlyInSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !strings.HasPrefix(cfg.Server.Token, "adm_") {
		t.Fatalf("generated operator token has invalid prefix: %q", cfg.Server.Token)
	}
	mainConfig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	secrets, err := os.ReadFile(filepath.Join(dir, "secrets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mainConfig), cfg.Server.Token) || !strings.Contains(string(secrets), cfg.Server.Token) {
		t.Fatal("generated operator token must appear only in secrets.yml")
	}
	info, err := os.Stat(filepath.Join(dir, "secrets.yml"))
	if err != nil {
		t.Fatalf("stat secrets.yml: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secrets permissions=%v", info.Mode().Perm())
	}
}
