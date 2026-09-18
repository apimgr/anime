package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/config"
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
	s, err := NewServer(svc, cfg, "8080", "0.0.0.0")
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return s
}

func TestNewServerRoutes(t *testing.T) {
	s := newTestServer(t)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/healthz"},
		{"GET", "/robots.txt"},
		{"GET", "/security.txt"},
		{"GET", "/.well-known/security.txt"},
		{"GET", "/manifest.json"},
		{"GET", "/sw.js"},
		{"GET", "/api/v1/random"},
		{"GET", "/api/v1/quotes"},
		{"GET", "/api/v1/health"},
		{"GET", "/api/v1/stats"},
		{"GET", "/api/v1/random.txt"},
		{"GET", "/api/v1/quotes.txt"},
		{"GET", "/api/v1/health.txt"},
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

func TestHandleHealthz(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	s.handleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("expected body OK, got %q", rec.Body.String())
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
