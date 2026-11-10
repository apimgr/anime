package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrefersPlainText(t *testing.T) {
	tests := []struct {
		accept string
		want   bool
	}{
		{"text/plain", true},
		{"text/plain;q=0.9, text/html;q=0.2", true},
		{"text/html,application/xhtml+xml,*/*;q=0.8", false},
		{"text/plain;q=0, text/html", false},
		{"*/*", false},
	}
	for _, test := range tests {
		if got := prefersPlainText(test.accept); got != test.want {
			t.Errorf("prefersPlainText(%q) = %t, want %t", test.accept, got, test.want)
		}
	}
}

func TestContentNegotiationConvertsJSONAndHTML(t *testing.T) {
	for _, test := range []struct {
		name        string
		contentType string
		body        string
		want        string
	}{
		{"API JSON", "application/json", `{"ok":true,"data":{"anime":"Naruto","quote":"Believe it"}}`, "anime: Naruto\nquote: Believe it"},
		{"SSR HTML", "text/html; charset=utf-8", "<!doctype html><html><body><h1>Anime quotes</h1><script>ignored()</script></body></html>", "Anime quotes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := contentNegotiationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				_, _ = w.Write([]byte(test.body))
			}))
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Accept", "text/plain")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
				t.Fatalf("Content-Type = %q", got)
			}
			if !strings.Contains(response.Body.String(), test.want) {
				t.Fatalf("body %q does not contain %q", response.Body.String(), test.want)
			}
		})
	}
}
