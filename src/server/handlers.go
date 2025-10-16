package server

import (
	"encoding/json"
	"log"
	"net/http"
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
	health := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"totalQuotes": s.animeService.GetTotalQuotes(),
	}
	respondJSON(w, http.StatusOK, health)
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
		"IsAdmin":     false,
	}

	if err := templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
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
