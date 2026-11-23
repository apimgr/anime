package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/anime/src/common/i18n"
	"github.com/apimgr/anime/src/mode"
)

const defaultTermsContent = `## Acceptance

Using Anime Quotes API means accepting these terms; stop using the service if you do not agree.

## Acceptable use

No unlawful use, abuse of other people, or attempt to bypass rate limits or access controls. Do not test beyond the security policy at /server/security/policy.

## Liability

The service is provided as is, without warranty of any kind. To the extent the law allows, the operator is not liable for loss arising from its use.

## Changes

The terms may change; the current text on this page applies from the moment it is published.

## Governing law

The law of the place where the operator is established applies, unless the operator's own terms text names another.`

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
	databaseStatus := s.checkDatabase(r.Context())
	status, httpStatus := "unhealthy", http.StatusServiceUnavailable
	if databaseStatus == "ok" {
		status, httpStatus = "healthy", http.StatusOK
	}
	health := struct {
		Project struct {
			Name        string `json:"name"`
			Tagline     string `json:"tagline"`
			Description string `json:"description"`
		} `json:"project"`
		Status    string `json:"status"`
		Version   string `json:"version"`
		GoVersion string `json:"go_version"`
		Build     struct {
			Commit string `json:"commit"`
			Date   string `json:"date"`
		} `json:"build"`
		Uptime    string    `json:"uptime"`
		Mode      string    `json:"mode"`
		Timestamp time.Time `json:"timestamp"`
		Features  struct {
			Tor struct {
				Enabled  bool   `json:"enabled"`
				Running  bool   `json:"running"`
				Status   string `json:"status"`
				Hostname string `json:"hostname"`
			} `json:"tor"`
			I2P struct {
				Enabled  bool   `json:"enabled"`
				Running  bool   `json:"running"`
				Status   string `json:"status"`
				Hostname string `json:"hostname"`
				Provider string `json:"provider"`
			} `json:"i2p"`
			GeoIP bool `json:"geoip"`
		} `json:"features"`
		Checks struct {
			Database  string `json:"database"`
			Cache     string `json:"cache"`
			Disk      string `json:"disk"`
			Scheduler string `json:"scheduler"`
		} `json:"checks"`
		Stats struct {
			RequestsTotal int64 `json:"requests_total"`
			Requests24h   int64 `json:"requests_24h"`
			ActiveConns   int64 `json:"active_connections"`
			QuotesTotal   int   `json:"quotes_total"`
		} `json:"stats"`
	}{Status: status, Version: Version, GoVersion: runtime.Version(), Uptime: uptime.String(), Mode: string(mode.GetCurrentAppMode()), Timestamp: time.Now().UTC()}
	health.Project.Name = "Anime Quotes API"
	health.Project.Tagline = "A public, read-only collection of anime quotes."
	health.Project.Description = "A full-stack application serving anime quotes from a curated collection through a public read-only API and browser interface."
	health.Build.Commit = CommitID
	health.Build.Date = BuildDate
	health.Features.Tor.Enabled, health.Features.Tor.Running, health.Features.Tor.Status, health.Features.Tor.Hostname = torHealth()
	health.Features.I2P.Status = "disabled"
	health.Features.I2P.Provider = "none"
	health.Checks.Database = databaseStatus
	health.Checks.Cache = "ok"
	health.Checks.Disk = "ok"
	health.Checks.Scheduler = "ok"
	health.Stats.QuotesTotal = s.animeService.GetTotalQuotes()
	health.Stats.ActiveConns = s.activeConns.Load()
	if s.requestStats != nil {
		health.Stats.RequestsTotal, health.Stats.Requests24h = s.requestStats.snapshot(time.Now())
	}
	writeHealthJSON(w, httpStatus, health)
}

func (s *Server) checkDatabase(parent context.Context) string {
	if s.database == nil || s.database.SQL == nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	if err := s.database.SQL.PingContext(ctx); err != nil {
		return "error"
	}
	return "ok"
}

func torHealth() (enabled, running bool, status, hostname string) {
	enabled = strings.EqualFold(os.Getenv("ENABLE_TOR"), "true")
	hostname = strings.TrimSpace(os.Getenv("TOR_HOSTNAME"))
	if enabled && strings.HasSuffix(hostname, ".onion") && runtime.GOOS == "linux" {
		if pid, err := strconv.Atoi(os.Getenv("TOR_PID")); err == nil && pid > 0 {
			if process, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
				closingParen := strings.LastIndexByte(string(process), ')')
				if closingParen >= 0 && closingParen+2 < len(process) {
					state := process[closingParen+2]
					running = state != 'Z' && state != 'X'
				}
			}
		}
	}
	status = "disabled"
	if enabled {
		status = "starting"
		if running {
			status = "running"
		}
	}
	return enabled, running, status, hostname
}

func writeHealthJSON(w http.ResponseWriter, status int, value interface{}) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "health response unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

// handleStats returns statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	requestsTotal, requests24h := s.requestStats.snapshot(time.Now())
	stats := map[string]interface{}{
		"totalQuotes":        s.animeService.GetTotalQuotes(),
		"requests_total":     requestsTotal,
		"requests_24h":       requests24h,
		"active_connections": s.activeConns.Load(),
		"uptime":             uptime.String(),
		"goVersion":          runtime.Version(),
		"platform":           fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		"theme":              s.cfg.WebUI.Theme,
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
		"Title":           "Home",
		"ContentTemplate": "home-content",
		"Description":     "Browse and search a public collection of anime quotes, or use the read-only API.",
		"Page":            "home",
		"Quote":           displayedQuote,
		"TotalQuotes":     s.animeService.GetTotalQuotes(),
		"ServerURL":       s.getServerURL(r),
		"Theme":           s.cfg.WebUI.Theme,
		"Version":         Version,
	}

	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering template: %v", err)
	}
}

// Text format handlers (.txt extension)

// handleRandomQuoteText returns a random quote as plain text
func (s *Server) handleRandomQuoteText(w http.ResponseWriter, r *http.Request) {
	quote := s.animeService.GetRandomQuote()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	if q, ok := quote.(map[string]interface{}); ok {
		sb.WriteString("Quote: " + fmt.Sprint(q["quote"]) + "\n")
		sb.WriteString("Character: " + fmt.Sprint(q["character"]) + "\n")
		sb.WriteString("Anime: " + fmt.Sprint(q["anime"]) + "\n")
	} else {
		_, _ = fmt.Fprint(&sb, quote)
	}
	_, _ = w.Write([]byte(sb.String()))
}

// handleAllQuotesText returns all quotes as plain text
func (s *Server) handleAllQuotesText(w http.ResponseWriter, r *http.Request) {
	quotes := s.animeService.GetAllQuotes()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	sb.WriteString("Total Quotes: " + strconv.Itoa(s.animeService.GetTotalQuotes()) + "\n")
	sb.WriteString("---\n")

	if quotesSlice, ok := quotes.([]interface{}); ok {
		for i, q := range quotesSlice {
			if quote, ok := q.(map[string]interface{}); ok {
				sb.WriteString("\n[" + strconv.Itoa(i+1) + "]\n")
				sb.WriteString("Quote: " + fmt.Sprint(quote["quote"]) + "\n")
				sb.WriteString("Character: " + fmt.Sprint(quote["character"]) + "\n")
				sb.WriteString("Anime: " + fmt.Sprint(quote["anime"]) + "\n")
			}
		}
	}
	_, _ = w.Write([]byte(sb.String()))
}

// handleHealthText returns health status as plain text
func (s *Server) handleHealthText(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	databaseStatus := s.checkDatabase(r.Context())
	status := "healthy"
	if databaseStatus != "ok" {
		status, databaseStatus = "unhealthy", "error"
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	requestsTotal, requests24h := int64(0), int64(0)
	if s.requestStats != nil {
		requestsTotal, requests24h = s.requestStats.snapshot(time.Now())
	}

	var sb strings.Builder
	sb.WriteString("project.name: Anime Quotes API\n")
	sb.WriteString("project.description: A public read-only anime quote collection and browser interface.\n")
	sb.WriteString("status: " + status + "\n")
	sb.WriteString("version: " + Version + "\n")
	sb.WriteString("go_version: " + runtime.Version() + "\n")
	sb.WriteString("build.commit: " + CommitID + "\n")
	sb.WriteString("build.date: " + BuildDate + "\n")
	sb.WriteString("uptime: " + uptime.String() + "\n")
	sb.WriteString("mode: " + string(mode.GetCurrentAppMode()) + "\n")
	sb.WriteString("timestamp: " + time.Now().UTC().Format(time.RFC3339) + "\n")
	torEnabled, torRunning, torStatus, torHostname := torHealth()
	sb.WriteString("features.tor.enabled: " + strconv.FormatBool(torEnabled) + "\nfeatures.tor.running: " + strconv.FormatBool(torRunning) + "\nfeatures.tor.status: " + torStatus + "\n")
	if torRunning {
		sb.WriteString("features.tor.hostname: " + torHostname + "\n")
	}
	sb.WriteString("features.i2p.status: disabled\nfeatures.geoip: disabled\n")
	sb.WriteString("checks.database: " + databaseStatus + "\nchecks.cache: ok\nchecks.disk: ok\nchecks.scheduler: ok\n")
	sb.WriteString("stats.requests_total: " + strconv.FormatInt(requestsTotal, 10) + "\nstats.requests_24h: " + strconv.FormatInt(requests24h, 10) + "\nstats.active_connections: " + strconv.FormatInt(s.activeConns.Load(), 10) + "\nstats.quotes_total: " + strconv.Itoa(s.animeService.GetTotalQuotes()) + "\n")
	_, _ = w.Write([]byte(sb.String()))
}

// handleStatsText returns statistics as plain text
func (s *Server) handleStatsText(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var sb strings.Builder
	sb.WriteString("Total Quotes: " + strconv.Itoa(s.animeService.GetTotalQuotes()) + "\n")
	sb.WriteString("Uptime: " + uptime.String() + "\n")
	sb.WriteString("Go Version: " + runtime.Version() + "\n")
	sb.WriteString("Platform: " + runtime.GOOS + "/" + runtime.GOARCH + "\n")
	sb.WriteString("Theme: " + s.cfg.WebUI.Theme + "\n")
	_, _ = w.Write([]byte(sb.String()))
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
		source := ""
		if isMCPRoute(r.URL.Path) || isMCPSource(r) {
			source = " source=mcp"
		}
		log.Printf(
			"%s %s %s%s",
			r.Method,
			sanitizeRequestURI(r.RequestURI),
			time.Since(start),
			source,
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
			s.renderStatusError(w, r, http.StatusBadRequest)
			return
		}
	}
	quotes := s.animeService.SearchQuotes(query.Get("q"), query.Get("anime"), query.Get("character"))
	quoteRows := make([]map[string]string, 0, len(quotes))
	for _, quote := range quotes {
		quoteRows = append(quoteRows, map[string]string{
			"ID":    stableQuoteID(quote.Anime, quote.Character, quote.Quote),
			"Anime": quote.Anime, "Character": quote.Character, "Quote": quote.Quote,
		})
	}
	data := map[string]interface{}{
		"Title": i18n.Translate(s.resolvedLanguage(r), "quotes_page.title"), "Description": i18n.Translate(s.resolvedLanguage(r), "quotes_page.description"),
		"ContentTemplate": "quotes-content", "Page": "quotes", "Version": Version,
		"Query": query.Get("q"), "Anime": query.Get("anime"), "Character": query.Get("character"), "Quotes": quoteRows,
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering quotes page: %v", err)
	}
}

// handleInfoPage renders the public informational pages specified in IDEA.md.
func (s *Server) handleInfoPage(w http.ResponseWriter, r *http.Request) {
	titles := map[string]string{"/server/about": "About", "/server/help": "Help", "/server/privacy": "Privacy", "/server/terms": "Terms of Service"}
	title, ok := titles[r.URL.Path]
	if !ok {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	data := map[string]interface{}{
		"Title": title, "Page": strings.TrimPrefix(r.URL.Path, "/server/"), "ContentTemplate": "info-content",
		"Description": "About Anime Quotes, a public read-only anime quote collection and API.",
		"Version":     Version, "CommitID": CommitID, "BuildDate": BuildDate,
		"TotalQuotes": s.animeService.GetTotalQuotes(), "ServerURL": s.getServerURL(r),
		"TorEnabled":  strings.EqualFold(os.Getenv("ENABLE_TOR"), "true") && strings.HasSuffix(os.Getenv("TOR_HOSTNAME"), ".onion"),
		"TorHostname": os.Getenv("TOR_HOSTNAME"),
		"Pages":       registeredServerPages("help", s.resolvedLanguage(r)),
	}
	if strings.TrimPrefix(r.URL.Path, "/server/") == "privacy" {
		data["Title"] = "Privacy Policy"
		privacy := s.cfg.Server.Privacy
		data["DataSold"] = privacy.Data.Sold
		data["DataStoredOnServer"] = privacy.Data.StoredOnServer
		data["PrivacyRetention"] = privacy.Retention.Period
		data["EssentialCookieDescription"] = privacy.Cookies.Essential.Description
		data["PreferenceCookieDescription"] = privacy.Cookies.Preferences.Description
		data["AnalyticsCookieDescription"] = analyticsCookieDescription(privacy.Cookies.Analytics, privacy.Data.Sold)
		data["ThirdPartyServices"] = privacy.ThirdParty.Services
		data["DPOName"], data["DPOEmail"] = s.dpoContact()
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering informational page: %v", err)
	}
}

func (s *Server) handleInfoJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeProblem(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if r.URL.Path == "/api/v1/server/privacy" {
		respondJSON(w, http.StatusOK, privacyAPIData(r, s))
		return
	}
	content := map[string]string{
		"/api/v1/server/about":   "A public, read-only browser and API for an embedded collection of anime quotes.",
		"/api/v1/server/help":    "Browse quotes at /quotes. GET /api/v1/random returns one quote. GET /api/v1/quotes supports q, anime, and character filters. REST and GraphQL documentation are available at /server/docs/swagger and /server/docs/graphql.",
		"/api/v1/server/privacy": "The service has no visitor accounts or profiles. Search terms are present in request URLs and may appear in request logs. Theme, language, and consent choices are stored in first-party cookies.",
		"/api/v1/server/terms":   defaultTermsContent,
	}
	if strings.EqualFold(os.Getenv("ENABLE_TOR"), "true") && strings.HasSuffix(os.Getenv("TOR_HOSTNAME"), ".onion") {
		content["/api/v1/server/help"] += " Tor access is available at http://" + os.Getenv("TOR_HOSTNAME") + "."
	}
	value, ok := content[r.URL.Path]
	if !ok {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	page := strings.TrimPrefix(r.URL.Path, "/api/v1/server/")
	format := "markdown"
	if page == "terms" {
		if custom := s.operatorHTML(r, "terms"); custom != "" {
			value = string(custom)
			format = "html"
		}
	}
	titles := map[string]string{"about": "About", "help": "Help", "privacy": "Privacy", "terms": "Terms of Service"}
	data := map[string]interface{}{"title": titles[page], "content": value, "format": format}
	if page == "about" {
		data["operator_content"] = string(s.operatorHTML(r, "about"))
	}
	if page == "help" {
		data["pages"] = helpPageAPIEntries(s.resolvedLanguage(r))
	}
	respondJSON(w, http.StatusOK, data)
}

func (s *Server) dpoContact() (string, string) {
	name := strings.TrimSpace(s.cfg.Server.Contact.DPO.Name)
	email := strings.TrimSpace(s.cfg.Server.Contact.DPO.Email)
	if email == "" {
		email = strings.TrimSpace(s.cfg.Server.Contact.General.Email)
	}
	return name, email
}

func (s *Server) handleDPOPage(w http.ResponseWriter, r *http.Request) {
	name, email := s.dpoContact()
	data := map[string]interface{}{
		"Title": "Data Protection Officer", "Description": "Privacy contact information for Anime Quotes.",
		"Page": "dpo", "ContentTemplate": "info-content", "Version": Version,
		"DPOName": name, "DPOEmail": email,
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering DPO page: %v", err)
	}
}

func (s *Server) handleDPOJSON(w http.ResponseWriter, r *http.Request) {
	name, email := s.dpoContact()
	respondJSON(w, http.StatusOK, map[string]string{
		"name": name, "email": email, "privacy_url": requestOrigin(r) + "/server/privacy",
	})
}

// writeProblem returns a compact RFC 7807 error response for invalid API input.
func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": detail})
}
