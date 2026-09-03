package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// handleRandomQuote returns a random anime quote
func (s *Server) handleRandomQuote(w http.ResponseWriter, r *http.Request) {
	quote := s.animeService.GetRandomQuote()
	respondJSON(w, http.StatusOK, quote)
}

// handleAllQuotes returns all anime quotes
func (s *Server) handleAllQuotes(w http.ResponseWriter, r *http.Request) {
	quotes := s.animeService.GetAllQuotes()
	respondJSON(w, http.StatusOK, quotes)
}

// handleHealth returns the health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	health := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"totalQuotes": s.animeService.GetTotalQuotes(),
		"uptime":      uptime.String(),
		"version":     "0.0.1",
	}
	respondJSON(w, http.StatusOK, health)
}

// handleStats returns statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	stats := map[string]interface{}{
		"totalQuotes": s.animeService.GetTotalQuotes(),
		"uptime":      uptime.String(),
		"goVersion":   runtime.Version(),
		"platform":    fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		"theme":       s.cfg.WebUI.Theme,
	}
	respondJSON(w, http.StatusOK, stats)
}

// handleHome renders the homepage with a random quote
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	quote := s.animeService.GetRandomQuote()
	data := map[string]interface{}{
		"Title":       "Home",
		"Page":        "home",
		"Quote":       quote,
		"TotalQuotes": s.animeService.GetTotalQuotes(),
		"ServerURL":   s.getServerURL(r),
		"Theme":       s.cfg.WebUI.Theme,
	}

	if err := templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Text format handlers (.txt extension)

// handleRandomQuoteText returns a random quote as plain text
func (s *Server) handleRandomQuoteText(w http.ResponseWriter, r *http.Request) {
	quote := s.animeService.GetRandomQuote()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	if q, ok := quote.(map[string]interface{}); ok {
		sb.WriteString(fmt.Sprintf("Quote: %v\n", q["quote"]))
		sb.WriteString(fmt.Sprintf("Character: %v\n", q["character"]))
		sb.WriteString(fmt.Sprintf("Anime: %v\n", q["anime"]))
	} else {
		sb.WriteString(fmt.Sprintf("%v", quote))
	}
	w.Write([]byte(sb.String()))
}

// handleAllQuotesText returns all quotes as plain text
func (s *Server) handleAllQuotesText(w http.ResponseWriter, r *http.Request) {
	quotes := s.animeService.GetAllQuotes()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total Quotes: %d\n", s.animeService.GetTotalQuotes()))
	sb.WriteString("---\n")

	if quotesSlice, ok := quotes.([]interface{}); ok {
		for i, q := range quotesSlice {
			if quote, ok := q.(map[string]interface{}); ok {
				sb.WriteString(fmt.Sprintf("\n[%d]\n", i+1))
				sb.WriteString(fmt.Sprintf("Quote: %v\n", quote["quote"]))
				sb.WriteString(fmt.Sprintf("Character: %v\n", quote["character"]))
				sb.WriteString(fmt.Sprintf("Anime: %v\n", quote["anime"]))
			}
		}
	}
	w.Write([]byte(sb.String()))
}

// handleHealthText returns health status as plain text
func (s *Server) handleHealthText(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	sb.WriteString("Status: healthy\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Total Quotes: %d\n", s.animeService.GetTotalQuotes()))
	sb.WriteString(fmt.Sprintf("Uptime: %s\n", uptime.String()))
	sb.WriteString("Version: 0.0.1\n")
	w.Write([]byte(sb.String()))
}

// handleStatsText returns statistics as plain text
func (s *Server) handleStatsText(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total Quotes: %d\n", s.animeService.GetTotalQuotes()))
	sb.WriteString(fmt.Sprintf("Uptime: %s\n", uptime.String()))
	sb.WriteString(fmt.Sprintf("Go Version: %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("Theme: %s\n", s.cfg.WebUI.Theme))
	w.Write([]byte(sb.String()))
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// parseJSON parses JSON request body
func parseJSON(r *http.Request, target interface{}) error {
	return json.NewDecoder(r.Body).Decode(target)
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf(
			"%s %s %s",
			r.Method,
			r.RequestURI,
			time.Since(start),
		)
	})
}
