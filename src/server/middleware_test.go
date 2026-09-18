package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	s := newTestServer(t)
	handler := s.securityHeadersMiddleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	tests := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
	}
	for header, want := range tests {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("expected header %s=%q, got %q", header, want, got)
		}
	}
}

func TestSecurityHeadersMiddlewareHSTS(t *testing.T) {
	s := newTestServer(t)
	handler := s.securityHeadersMiddleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS header to be set for TLS requests")
	}
}

func TestRequestSizeLimitMiddleware(t *testing.T) {
	handler := requestSizeLimitMiddleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestThrottleMiddlewareAllowsWithinLimit(t *testing.T) {
	handler := throttleMiddleware(2)(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestThrottleMiddlewareRejectsOverLimit(t *testing.T) {
	// maxConcurrent 0 means the semaphore send always blocks, forcing the rejection branch.
	handler := throttleMiddleware(0)(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestCorsMiddlewareWildcard(t *testing.T) {
	s := newTestServer(t)
	s.cfg.WebSecurity.CORS = "*"
	handler := s.corsMiddleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected wildcard CORS origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsMiddlewareSpecificOrigin(t *testing.T) {
	s := newTestServer(t)
	s.cfg.WebSecurity.CORS = "https://allowed.com, https://also-allowed.com"
	handler := s.corsMiddleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://allowed.com" {
		t.Errorf("expected allowed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsMiddlewareDisallowedOrigin(t *testing.T) {
	s := newTestServer(t)
	s.cfg.WebSecurity.CORS = "https://allowed.com"
	handler := s.corsMiddleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://not-allowed.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsMiddlewarePreflight(t *testing.T) {
	s := newTestServer(t)
	handler := s.corsMiddleware(okHandler())

	req := httptest.NewRequest("OPTIONS", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for preflight, got %d", rec.Code)
	}
	if rec.Body.String() != "" {
		t.Errorf("expected empty body for preflight, got %q", rec.Body.String())
	}
}

func TestRecoverMiddleware(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	handler := recoverMiddleware(panicHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	handler := loggingMiddleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGlobalAndAPIRateLimitMiddleware(t *testing.T) {
	globalHandler := globalRateLimitMiddleware()(okHandler())
	apiHandler := apiRateLimitMiddleware()(okHandler())

	req := httptest.NewRequest("GET", "/", nil)

	rec := httptest.NewRecorder()
	globalHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from global rate limiter, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	apiHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from API rate limiter, got %d", rec.Code)
	}
}
