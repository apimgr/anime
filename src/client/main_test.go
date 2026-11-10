package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunEarlyCommandsAndValidation(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"--help"}, &out, &errOut); err != nil || !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help: output=%q err=%v", out.String(), err)
	}
	out.Reset()
	if err := run([]string{"--version"}, &out, &errOut); err != nil || !strings.Contains(out.String(), "dev (unknown)") {
		t.Fatalf("version: output=%q err=%v", out.String(), err)
	}
	for _, args := range [][]string{{"--output", "xml"}, {"--color", "maybe"}, {"--shell", "invalid"}} {
		if err := run(args, &out, &errOut); err == nil {
			t.Errorf("run(%v) expected validation error", args)
		}
	}
	if err := run([]string{"--server", "https://example.invalid", "unknown"}, &out, &errOut); err == nil {
		t.Fatal("unknown command succeeded")
	}
}

func TestRenderAndBuildDate(t *testing.T) {
	var out bytes.Buffer
	quote := map[string]interface{}{"quote": "I am here", "character": "A", "anime": "B"}
	if err := render(&out, map[string]interface{}{"data": []interface{}{quote}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "I am here") || !strings.Contains(out.String(), "Character: A\nAnime: B") {
		t.Fatalf("unexpected rendering: %s", out.String())
	}
	if err := render(&out, "unexpected"); err == nil {
		t.Fatal("render accepted an unsupported value")
	}
	for _, format := range []string{"table", "json", "yaml", "plain", "csv"} {
		out.Reset()
		if err := renderFormat(&out, map[string]interface{}{"data": []interface{}{quote}}, format, false); err != nil || out.Len() == 0 {
			t.Errorf("render format %q output=%q err=%v", format, out.String(), err)
		}
	}
	old := BuildEpoch
	defer func() { BuildEpoch = old }()
	BuildEpoch = "invalid"
	if got := clientBuildDate(); got != "N/A" {
		t.Fatalf("invalid build epoch = %q", got)
	}
	BuildEpoch = "0"
	if got := clientBuildDate(); got != "N/A" {
		t.Fatalf("zero build epoch = %q", got)
	}
	BuildEpoch = "1"
	if got := clientBuildDate(); got != "1970-01-01T00:00:01Z" {
		t.Fatalf("valid build epoch = %q", got)
	}
}

func TestClientCacheHelpers(t *testing.T) {
	cacheDir := t.TempDir()
	requestURL := "https://quotes.example/api/v1/quotes?q=one"
	body := []byte(`{"data":[]}`)
	if err := storeCachedResponse(cacheDir, requestURL, body, 1024); err != nil {
		t.Fatal(err)
	}
	got, err := readCachedResponse(cacheDir, requestURL, time.Minute)
	if err != nil || string(got) != string(body) {
		t.Fatalf("cached response=%q err=%v", got, err)
	}
	if _, err := parseByteSize("100MB"); err != nil {
		t.Fatalf("parse cache size: %v", err)
	}
	if got, err := parseByteSize("1KB"); err != nil || got != 1024 {
		t.Fatalf("parse 1KB=%d err=%v", got, err)
	}
	if _, err := parseByteSize("invalid"); err == nil {
		t.Fatal("invalid cache size succeeded")
	}
	if validAPIVersion("v1") == false || validAPIVersion("v1/../x") {
		t.Fatal("API version validation failed")
	}
	if err := storeCachedResponse(cacheDir, requestURL, body, 1); err != nil {
		t.Fatal(err)
	}
	if got, err := readCachedResponse(cacheDir, requestURL, time.Minute); err != nil || string(got) != string(body) {
		t.Fatalf("replacement cached response=%q err=%v", got, err)
	}
	if got, err := readCachedResponse(cacheDir, requestURL, 0); err != nil || got != nil {
		t.Fatalf("disabled cache=%q err=%v", got, err)
	}
}

func TestClientLoggingAndRequestRetries(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "cli.log")
	settings := defaultCLISettings()
	settings.Logging.File = logPath
	settings.Logging.MaxSize = "1B"
	settings.Logging.MaxFiles = 1
	if err := os.WriteFile(logPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := initClientLog(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Fatalf("rotated log missing: %v", err)
	}
	info, err := os.Stat(logPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("log mode=%v err=%v", info, err)
	}

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	body, err := fetchResponse(server.URL, time.Second, 2, 0)
	if err != nil || string(body) != "ok" || attempts != 3 {
		t.Fatalf("response=%q attempts=%d err=%v", body, attempts, err)
	}
}

func TestClientConfiguration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", "")
	path, dirs, err := cliDirectories()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	settings.Server.Primary = "https://quotes.example"
	settings.Output.Format = "yaml"
	settings.TUI.Theme = "light"
	if err := saveCLIConfig(settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server.Primary != settings.Server.Primary || loaded.Output.Format != settings.Output.Format || loaded.TUI.Theme != settings.TUI.Theme {
		t.Fatalf("configuration round trip = %#v", loaded)
	}
	profilePath := filepath.Join(filepath.Dir(path), "staging.yml")
	if err := os.WriteFile(profilePath, []byte("server:\n  primary: https://staging.example\noutput:\n  format: csv\n"), 0600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadCLIConfigFile("staging")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Server.Primary != "https://staging.example" || profile.Output.Format != "csv" || profile.TUI.Theme != "dark" {
		t.Fatalf("profile configuration = %#v", profile)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions = %o", info.Mode().Perm())
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Errorf("directory %s permissions=%v err=%v", dir, info, err)
		}
	}
	if err := os.WriteFile(path, []byte("server: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCLIConfig(); err == nil {
		t.Fatal("malformed configuration loaded")
	}
	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCLIConfig(); err != nil {
		t.Fatalf("missing configuration was not recreated: %v", err)
	}
}

func TestRunHTTPActionsAndFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept header = %q", r.Header.Get("Accept"))
		}
		switch r.URL.Path {
		case "/api/v1/random":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]string{"quote": "One", "character": "A", "anime": "B"}})
		case "/api/v1/quotes":
			if r.URL.Query().Get("q") != "" && r.URL.Query().Get("q") != "two words" {
				t.Errorf("search query = %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]string{{"quote": "Two", "character": "C", "anime": "D"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANIME_SERVER_PRIMARY", server.URL)
	for _, args := range [][]string{{"random"}, {"list"}, {"search", "two", "words"}, {"--output", "json", "random"}, {"--debug", "random"}} {
		var out, errOut bytes.Buffer
		if err := run(args, &out, &errOut); err != nil {
			t.Errorf("run(%v): %v", args, err)
		}
		if out.Len() == 0 {
			t.Errorf("run(%v) returned no output", args)
		}
	}
	for _, args := range [][]string{{"--server", "http://user:pass@example.com", "random"}, {"--server", "http://example.com/#fragment", "random"}, {"--server", server.URL}, {"--server", server.URL, "search"}} {
		var out, errOut bytes.Buffer
		if err := run(args, &out, &errOut); err == nil {
			t.Errorf("run(%v) expected an error", args)
		}
	}
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failure", http.StatusServiceUnavailable) }))
	defer badServer.Close()
	var out, errOut bytes.Buffer
	if err := run([]string{"--server", badServer.URL, "random"}, &out, &errOut); err == nil {
		t.Fatal("non-2xx API response succeeded")
	}
	if err := run([]string{"--server", "http://127.0.0.1:1", "random"}, &out, &errOut); err == nil {
		t.Fatal("unavailable server succeeded")
	}
	if err := run([]string{"--lang", "fr", "--server", server.URL, "random"}, &out, &errOut); err != nil {
		t.Fatalf("unsupported language did not fall back to English: %v", err)
	}
	if err := run([]string{"--server", server.URL + "/", "random"}, &out, &errOut); err != nil {
		t.Fatalf("trailing slash server URL: %v", err)
	}
}

func TestPrintClientShell(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "sh", "dash", "ksh", "powershell", "pwsh"} {
		var out bytes.Buffer
		if err := printClientShell(&out, "completions", []string{shell}); err != nil || out.Len() == 0 {
			t.Errorf("completion shell %s: output=%q err=%v", shell, out.String(), err)
		}
	}
	for _, shell := range []string{"bash", "zsh", "fish", "sh", "dash", "ksh", "powershell", "pwsh"} {
		var out bytes.Buffer
		if err := printClientShell(&out, "init", []string{shell}); err != nil || out.Len() == 0 {
			t.Errorf("init shell %s: output=%q err=%v", shell, out.String(), err)
		}
	}
	if err := printClientShell(&bytes.Buffer{}, "completions", []string{"unknown"}); err == nil {
		t.Fatal("unsupported shell succeeded")
	}
}

func TestTUIModelAndLoader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/random" {
			_, _ = w.Write([]byte(`{"data":{"anime":"B","character":"A","quote":"One"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"anime":"D","character":"C","quote":"Two"}]}`))
	}))
	defer server.Close()
	model := tuiModel{serverURL: server.URL, width: 80, height: 24}
	for _, action := range []string{"random", "list", "search"} {
		msg := model.load(action, "query")()
		loaded, ok := msg.(quotesLoaded)
		if !ok || loaded.err != nil || len(loaded.quotes) != 1 {
			t.Errorf("load(%s) = %#v", action, msg)
		}
	}
	model.loading = true
	var current tea.Model = model
	current, _ = current.(tuiModel).Update(quotesLoaded{quotes: []quoteRecord{{Quote: "Two"}}})
	if current.(tuiModel).loading || current.(tuiModel).message != "1 quotes" {
		t.Fatalf("loaded state = %#v", current)
	}
	current, _ = current.(tuiModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !current.(tuiModel).searching {
		t.Fatal("slash did not open search")
	}
	current, _ = current.(tuiModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if current.(tuiModel).query != "x" {
		t.Fatal("search input was not recorded")
	}
	view := current.(tuiModel).View()
	if !strings.Contains(view, "Search: x") {
		t.Fatalf("search view = %q", view)
	}
	current, _ = current.(tuiModel).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if current.(tuiModel).searching {
		t.Fatal("escape did not close search")
	}
	current, _ = current.(tuiModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !strings.Contains(current.(tuiModel).message, "help") {
		t.Fatal("help key did not update the message")
	}
	current, command := current.(tuiModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if command == nil || !current.(tuiModel).loading {
		t.Fatal("random key did not start loading")
	}
	_, command = current.(tuiModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if command == nil {
		t.Fatal("quit key did not return a command")
	}
	if got := (tuiModel{serverURL: "bad-url"}).load("random", "")().(quotesLoaded); got.err == nil {
		t.Fatal("invalid TUI URL succeeded")
	}
}
