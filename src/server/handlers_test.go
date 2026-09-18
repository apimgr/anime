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

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected JSON content type, got %q", rec.Header().Get("Content-Type"))
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["anime"] != "Naruto" {
		t.Errorf("expected anime Naruto, got %v", body["anime"])
	}
}

func TestHandleAllQuotes(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/quotes", nil)
	rec := httptest.NewRecorder()
	s.handleAllQuotes(rec, req)

	var body []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 quote, got %d", len(body))
	}
}

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	s.handleHealth(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", body["status"])
	}
	if body["totalQuotes"].(float64) != 1 {
		t.Errorf("expected totalQuotes 1, got %v", body["totalQuotes"])
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
	if body["theme"] != "dark" {
		t.Errorf("expected theme dark, got %v", body["theme"])
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
	req := httptest.NewRequest("GET", "/api/v1/health.txt", nil)
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
