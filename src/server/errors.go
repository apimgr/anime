package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

func closeAndLog(closer io.Closer, label string) {
	if err := closer.Close(); err != nil {
		log.Printf("%s close failed: %v", label, err)
	}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.renderStatusError(w, r, http.StatusNotFound)
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET, HEAD, POST, OPTIONS")
	s.renderStatusError(w, r, http.StatusMethodNotAllowed)
}

func (s *Server) rateLimitError(w http.ResponseWriter, r *http.Request) {
	s.renderStatusError(w, r, http.StatusTooManyRequests)
}

func (s *Server) renderStatusError(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if wantsHTML(r) {
		data := map[string]interface{}{
			"Title": http.StatusText(status), "Description": http.StatusText(status),
			"ContentTemplate": "error-content", "Page": "error", "Version": Version,
			"ErrorStatus": status, "ErrorMessage": http.StatusText(status),
		}
		if err := s.renderPageWithStatus(w, r, data, status); err != nil {
			log.Printf("Error rendering %d page: %v", status, err)
		}
		return
	}
	writeFallbackError(w, r, status)
}

func (s *Server) recoverRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("Panic recovered: %v", recovered)
				s.renderStatusError(w, r, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func wantsHTML(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/graphql") {
		return false
	}
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func writeFallbackError(w http.ResponseWriter, r *http.Request, status int) {
	if strings.Contains(r.Header.Get("Accept"), "text/plain") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(http.StatusText(status) + "\n"))
		return
	}
	if !wantsHTML(r) {
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "about:blank", "title": http.StatusText(status), "status": status,
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte("<!doctype html><html><head><title>" + http.StatusText(status) + "</title></head><body><main><h1>" + http.StatusText(status) + "</h1></main></body></html>"))
}
