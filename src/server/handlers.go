package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/mode"
)

// handleRandomQuote returns a random anime quote
func (s *Server) handleRandomQuote(w http.ResponseWriter, r *http.Request) {
	quote := s.animeService.GetRandomQuote()
	respondJSON(w, http.StatusOK, quote)
}

// handleAllQuotes returns all anime quotes
func (s *Server) handleAllQuotes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for _, key := range []string{"q", "anime", "character"} {
		if len(query.Get(key)) > 256 {
			writeProblem(w, http.StatusBadRequest, "Query parameter is too long")
			return
		}
	}
	quotes := s.animeService.SearchQuotes(query.Get("q"), query.Get("anime"), query.Get("character"))
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
		"mode":        mode.GetCurrentAppMode(),
		"version":     Version,
		"commit":      CommitID,
		"built":       BuildDate,
	}
	if s.database != nil {
		health["db_type"] = s.database.Driver
		health["db_locality"] = s.database.Locality
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
	quote, hasQuote := s.animeService.GetRandomQuoteRecord()
	var displayedQuote interface{}
	if hasQuote {
		displayedQuote = quote
	}
	data := map[string]interface{}{
		"Title":       "Home",
		"Page":        "home",
		"Quote":       displayedQuote,
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
	sb.WriteString(fmt.Sprintf("Version: %s\n", Version))
	sb.WriteString(fmt.Sprintf("Built: %s\n", BuildDate))
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "data": data}); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf(
			"%s %s %s",
			r.Method,
			sanitizeRequestURI(r.RequestURI),
			time.Since(start),
		)
	})
}

func sanitizeRequestURI(requestURI string) string {
	parsed, err := url.ParseRequestURI(requestURI)
	if err != nil {
		return "[invalid request URI]"
	}
	query := parsed.Query()
	for key := range query {
		switch strings.ToLower(key) {
		case "token", "session", "code", "key", "password", "secret", "auth", "pwd", "api_key", "apikey", "access_token", "refresh_token":
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.RequestURI()
}

// handleQuotesPage renders the searchable quote collection without requiring JavaScript.
func (s *Server) handleQuotesPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for _, key := range []string{"q", "anime", "character"} {
		if len(query.Get(key)) > 256 {
			http.Error(w, "query parameter is too long", http.StatusBadRequest)
			return
		}
	}
	quotes := s.animeService.SearchQuotes(query.Get("q"), query.Get("anime"), query.Get("character"))
	data := struct {
		Query, Anime, Character string
		Quotes                  []anime.Quote
	}{query.Get("q"), query.Get("anime"), query.Get("character"), quotes}
	const page = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Anime quotes</title><main><p><a href="/">Home</a></p><h1>Anime quotes</h1><form method="get"><label>Search <input name="q" maxlength="256" value="{{.Query}}"></label><label>Anime <input name="anime" maxlength="256" value="{{.Anime}}"></label><label>Character <input name="character" maxlength="256" value="{{.Character}}"></label><button>Search</button></form><p>{{len .Quotes}} matching quotes</p>{{range .Quotes}}<article><blockquote>{{.Quote}}</blockquote><p>{{.Character}} · {{.Anime}}</p></article>{{else}}<p>No matching quotes.</p>{{end}}</main></html>`
	if err := template.Must(template.New("quotes").Parse(page)).Execute(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleInfoPage renders the public informational pages specified in IDEA.md.
func (s *Server) handleInfoPage(w http.ResponseWriter, r *http.Request) {
	titles := map[string]string{"/server/about": "About Anime Quotes", "/server/help": "Help", "/server/privacy": "Privacy", "/server/terms": "Terms"}
	title := titles[r.URL.Path]
	if title == "" {
		http.NotFound(w, r)
		return
	}
	content := map[string]string{
		"/server/about":   "A public, read-only API and browser interface for a curated collection of anime quotes.",
		"/server/help":    "Browse quotes or call GET /api/v1/quotes. Search with q, anime, and character query parameters. GET /api/v1/random returns a random quote.",
		"/server/privacy": "This service does not create accounts or store personal data. Standard request logs may include connection metadata managed by the operator.",
		"/server/terms":   "Quote text belongs to its respective rights holders. This service is provided as a public reference collection.",
	}[r.URL.Path]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>%s</title><main><p><a href=\"/\">Home</a></p><h1>%s</h1><p>%s</p></main></html>", template.HTMLEscapeString(title), template.HTMLEscapeString(title), template.HTMLEscapeString(content))
}

// writeProblem returns a compact RFC 7807 error response for invalid API input.
func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": detail})
}
