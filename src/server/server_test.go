package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/config"
	"github.com/apimgr/anime/src/mode"
	"github.com/apimgr/anime/src/updater"
)

const testQuotes = `[
	{"anime":"Naruto","character":"Naruto Uzumaki","quote":"Believe it!"}
]`

func newTestServer(t *testing.T) *Server {
	t.Helper()
	svc, err := anime.NewService([]byte(testQuotes))
	if err != nil {
		t.Fatalf("anime.NewService returned error: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Server.Token = "tok_0123456789abcdefghijklmnopqrstuv"
	s, err := NewServer(svc, cfg, "8080", "0.0.0.0")
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return s
}

func TestCLIAutodiscoverAndBinaryRoute(t *testing.T) {
	binary := []byte("cli binary")
	digest := sha256.Sum256(binary)
	checksum := hex.EncodeToString(digest[:]) + "  anime-cli-linux-amd64\n"
	var releaseServer *httptest.Server
	releaseServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/apimgr/anime/releases/latest":
			_, _ = fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[{"name":"anime-cli-linux-amd64","browser_download_url":%q},{"name":"sha256.txt","browser_download_url":%q}]}`, releaseServer.URL+"/binary", releaseServer.URL+"/checksums")
		case "/checksums":
			_, _ = fmt.Fprint(w, checksum)
		case "/binary":
			_, _ = w.Write(binary)
		default:
			http.NotFound(w, r)
		}
	}))
	defer releaseServer.Close()

	s := newTestServer(t)
	s.cfg.Server.Update.CLIMinVersion = "1.0.0"
	s.updateClient = &updater.Client{
		APIBase:   releaseServer.URL + "/repos/apimgr/anime",
		HTTP:      releaseServer.Client(),
		AllowHTTP: true,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/autodiscover", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("autodiscover status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"cli_min_version":"1.0.0"`) || !strings.Contains(rec.Body.String(), `"linux-amd64":{"version":"1.2.3","sha256":"`+hex.EncodeToString(digest[:])+`"}`) {
		t.Fatalf("autodiscover response missing update metadata: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/cli/binaries/anime-cli-linux-amd64", nil)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != releaseServer.URL+"/binary" {
		t.Fatalf("binary redirect status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/cli/binaries/not-an-anime-binary", nil)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unsupported binary status=%d", rec.Code)
	}
}

func TestNewServerRoutes(t *testing.T) {
	s := newTestServer(t)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/server/healthz"},
		{"GET", "/robots.txt"},
		{"GET", "/security.txt"},
		{"GET", "/.well-known/security.txt"},
		{"GET", "/manifest.json"},
		{"GET", "/sw.js"},
		{"GET", "/api/v1"},
		{"GET", "/api"},
		{"GET", "/api/v1/random"},
		{"GET", "/api/v1/quotes"},
		{"GET", "/api/v1/server/healthz"},
		{"GET", "/api/v1/stats"},
		{"GET", "/api/v1/random.txt"},
		{"GET", "/api/v1/quotes.txt"},
		{"GET", "/api/v1/server/healthz.txt"},
		{"GET", "/api/healthz"},
		{"GET", "/server/about"},
		{"GET", "/server/help"},
		{"GET", "/server/privacy"},
		{"GET", "/server/terms"},
		{"GET", "/server/docs/swagger"},
		{"GET", "/api/v1/server/swagger"},
		{"GET", "/server/docs/graphql"},
		{"GET", "/api/v1/server/graphql?query=%7BrandomQuote%7Banime%7D%7D"},
		{"GET", "/api/v1/stats.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			s.router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d for %s %s", rec.Code, tt.method, tt.path)
			}
		})
	}
}

func TestDebugRoutesRequireDebugMode(t *testing.T) {
	if err := mode.SetAppMode("production"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mode.SetAppMode("production") })
	production := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/debug/pprof/", nil)
	response := httptest.NewRecorder()
	production.router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("production pprof status=%d, want %d", response.Code, http.StatusNotFound)
	}

	if err := mode.SetAppMode("debug"); err != nil {
		t.Fatal(err)
	}
	debug := newTestServer(t)
	response = httptest.NewRecorder()
	debug.router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated debug pprof status=%d, want %d", response.Code, http.StatusUnauthorized)
	}
	request.Header.Set("Authorization", "Bearer "+debug.cfg.Server.Token)
	response = httptest.NewRecorder()
	debug.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("debug pprof status=%d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	for _, path := range []string{
		"/debug/vars", "/debug/config", "/debug/routes", "/debug/cache",
		"/debug/db", "/debug/scheduler", "/debug/memory", "/debug/goroutines",
		"/api/debug/memory", "/api/v1/debug/memory",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+debug.cfg.Server.Token)
		rec := httptest.NewRecorder()
		debug.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("debug route %s status=%d, want %d: %s", path, rec.Code, http.StatusOK, rec.Body.String())
		}
		if path == "/debug/config" && strings.Contains(rec.Body.String(), debug.cfg.Server.Token) {
			t.Error("debug config response leaked the operator token")
		}
	}
}

func TestHandleHealthz(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/server/healthz", nil)
	rec := httptest.NewRecorder()
	s.handleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Status: healthy") {
		t.Errorf("expected health page, got %q", rec.Body.String())
	}
}

func TestHandleRobotsTxt(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	rec := httptest.NewRecorder()
	s.handleRobotsTxt(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Allow: /") {
		t.Errorf("expected robots.txt to contain allow rules, got %q", body)
	}
	if !strings.Contains(body, "Disallow: /debug") {
		t.Errorf("expected robots.txt to contain deny rules, got %q", body)
	}
}

func TestHandleSecurityTxt(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/security.txt", nil)
	rec := httptest.NewRecorder()
	s.handleSecurityTxt(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Contact: mailto:security@apimgr.us") {
		t.Errorf("expected default security contact, got %q", body)
	}
}

func TestHandleSecurityTxtCustomAdmin(t *testing.T) {
	s := newTestServer(t)
	s.cfg.WebSecurity.Admin = "admin@example.com"

	req := httptest.NewRequest("GET", "/security.txt", nil)
	rec := httptest.NewRecorder()
	s.handleSecurityTxt(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Contact: mailto:admin@example.com") {
		t.Errorf("expected custom security contact, got %q", body)
	}
}

func TestHandleManifest(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest("GET", "/manifest.json", nil)
	rec := httptest.NewRecorder()
	s.handleManifest(rec, req)
	if !strings.Contains(rec.Body.String(), `"background_color": "#1a1a1a"`) {
		t.Errorf("expected dark background color for dark theme, got %q", rec.Body.String())
	}

	s.cfg.WebUI.Theme = "light"
	rec = httptest.NewRecorder()
	s.handleManifest(rec, req)
	if !strings.Contains(rec.Body.String(), `"background_color": "#ffffff"`) {
		t.Errorf("expected light background color for light theme, got %q", rec.Body.String())
	}
}

func TestHandleServiceWorker(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/sw.js", nil)
	rec := httptest.NewRecorder()
	s.handleServiceWorker(rec, req)

	if rec.Header().Get("Content-Type") != "application/javascript" {
		t.Errorf("expected javascript content type, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "CACHE_NAME") {
		t.Error("expected service worker script body")
	}
}

func TestGetServerURL(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"

	url := s.getServerURL(req)
	if url == "" {
		t.Error("expected non-empty server URL")
	}
}

func TestStartTimeIsSet(t *testing.T) {
	s := newTestServer(t)
	if time.Since(s.startTime) < 0 {
		t.Error("expected startTime to be in the past")
	}
}

func TestServerStart(t *testing.T) {
	svc, err := anime.NewService([]byte(testQuotes))
	if err != nil {
		t.Fatalf("anime.NewService returned error: %v", err)
	}
	cfg := config.DefaultConfig()
	s, err := NewServer(svc, cfg, "0", "127.0.0.1")
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	select {
	case err := <-errCh:
		t.Fatalf("Start returned early: %v", err)
	case <-time.After(200 * time.Millisecond):
		// Server is listening; leave it running for the test process lifetime.
	}
}

func TestGraphQLSearchAndOpenAPISpec(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/graphql?query=%7Bquotes%28q%3A%22believe%22%29%7Banime+quote%7D%7D", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Believe it!") {
		t.Fatalf("unexpected GraphQL response: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/server/swagger", nil)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"openapi":"3.1.0"`) {
		t.Fatalf("unexpected OpenAPI response: %d %s", rec.Code, rec.Body.String())
	}
}
