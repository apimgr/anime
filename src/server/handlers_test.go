package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRandomQuote(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/random", nil)
	rec := httptest.NewRecorder()
	s.handleRandomQuote(rec, req)

	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type, got %q", rec.Header().Get("Content-Type"))
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok || data["anime"] != "Naruto" {
		t.Errorf("expected wrapped Naruto quote, got %#v", body)
	}
}

func TestHandleAllQuotes(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/quotes", nil)
	rec := httptest.NewRecorder()
	s.handleAllQuotes(rec, req)

	var body struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected 1 quote, got %d", len(body.Data))
	}
}

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/server/healthz", nil)
	rec := httptest.NewRecorder()
	s.handleHealth(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected wrapped data, got %#v", body)
	}
	if data["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", data["status"])
	}
	if data["totalQuotes"].(float64) != 1 {
		t.Errorf("expected totalQuotes 1, got %v", data["totalQuotes"])
	}
}

func TestHandleStats(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	s.handleStats(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok || data["theme"] != "dark" {
		t.Errorf("expected wrapped dark theme data, got %#v", body)
	}
}

func TestHandleHome(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	s.handleHome(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRandomQuoteText(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/random.txt", nil)
	rec := httptest.NewRecorder()
	s.handleRandomQuoteText(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Anime: Naruto") {
		t.Errorf("expected anime name in text response, got %q", body)
	}
}

func TestHandleAllQuotesText(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/quotes.txt", nil)
	rec := httptest.NewRecorder()
	s.handleAllQuotesText(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Total Quotes: 1") {
		t.Errorf("expected total quotes count, got %q", body)
	}
	if !strings.Contains(body, "Naruto") {
		t.Errorf("expected quote content, got %q", body)
	}
}

func TestHandleHealthText(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/server/healthz.txt", nil)
	rec := httptest.NewRecorder()
	s.handleHealthText(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Status: healthy") {
		t.Errorf("expected status line, got %q", body)
	}
}

func TestHandleStatsText(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/stats.txt", nil)
	rec := httptest.NewRecorder()
	s.handleStatsText(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Theme: dark") {
		t.Errorf("expected theme line, got %q", body)
	}
}

func TestRespondJSONEncodeError(t *testing.T) {
	rec := httptest.NewRecorder()
	// A channel value cannot be marshaled to JSON, forcing the encode-error branch.
	respondJSON(rec, 200, map[string]interface{}{"bad": make(chan int)})

	if rec.Code != 200 {
		t.Errorf("expected status header to still be written, got %d", rec.Code)
	}
}

func TestHandleAllQuotesSearchAndValidation(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/quotes?q=believe&character=naruto", nil)
	rec := httptest.NewRecorder()
	s.handleAllQuotes(rec, req)
	var body struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected one match, got %d", len(body.Data))
	}
	req = httptest.NewRequest("GET", "/api/v1/quotes?q="+strings.Repeat("x", 257), nil)
	rec = httptest.NewRecorder()
	s.handleAllQuotes(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 for oversized query, got %d", rec.Code)
	}
}

func TestSanitizeRequestURI(t *testing.T) {
	got := sanitizeRequestURI("/api/v1/items?Q=hello&token=secret&API_KEY=another-secret&limit=10")
	if strings.Contains(got, "secret") || strings.Contains(got, "another-secret") {
		t.Fatalf("sanitized request URI leaked a credential: %q", got)
	}
	if !strings.Contains(got, "Q=hello") || !strings.Contains(got, "limit=10") {
		t.Fatalf("sanitized request URI dropped safe query values: %q", got)
	}
}
