package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/common/i18n"
)

type sitemapDocument struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapIndexDocument struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	XMLNS    string              `xml:"xmlns,attr"`
	Sitemaps []sitemapIndexEntry `xml:"sitemap"`
}

type sitemapIndexEntry struct {
	Location string `xml:"loc"`
	LastMod  string `xml:"lastmod"`
}

type sitemapURL struct {
	Location   string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

type publicSitemapPage struct {
	Path      string
	Frequency string
	Priority  string
}

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry,omitempty"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type atomEntry struct {
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Link      atomLink `xml:"link"`
	Published string   `xml:"published"`
	Updated   string   `xml:"updated"`
	Summary   string   `xml:"summary"`
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language"`
	LastBuild   string    `xml:"lastBuildDate"`
	Items       []rssItem `xml:"item,omitempty"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func BuildURL(r *http.Request, path string) string {
	origin := strings.TrimSuffix(requestOrigin(r), "/")
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return origin + path
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	entries := s.sitemapEntries(r)
	lastmod := discoveryModifiedTime().Format("2006-01-02")
	maxURLs := s.cfg.Server.SEO.Sitemap.MaxURLs
	if maxURLs < 1 || maxURLs > 50000 {
		maxURLs = 50000
	}
	if len(entries) > maxURLs {
		count := (len(entries) + maxURLs - 1) / maxURLs
		index := sitemapIndexDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", Sitemaps: make([]sitemapIndexEntry, 0, count)}
		for page := 1; page <= count; page++ {
			index.Sitemaps = append(index.Sitemaps, sitemapIndexEntry{Location: BuildURL(r, "/sitemap-"+strconv.Itoa(page)+".xml"), LastMod: lastmod})
		}
		body, err := xml.MarshalIndent(index, "", "  ")
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "sitemap unavailable")
			return
		}
		writeDiscoveryDocument(w, r, "application/xml; charset=utf-8", append([]byte(xml.Header), body...), discoveryModifiedTime())
		return
	}
	document := sitemapDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: entries}
	body, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "sitemap unavailable")
		return
	}
	writeDiscoveryDocument(w, r, "application/xml; charset=utf-8", append([]byte(xml.Header), body...), discoveryModifiedTime())
}

func (s *Server) handleSitemapPart(w http.ResponseWriter, r *http.Request) {
	pageText := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/sitemap-"), ".xml")
	page, err := strconv.Atoi(pageText)
	if err != nil || page < 1 {
		writeProblem(w, http.StatusNotFound, "not found")
		return
	}
	entries := s.sitemapEntries(r)
	maxURLs := s.cfg.Server.SEO.Sitemap.MaxURLs
	if maxURLs < 1 || maxURLs > 50000 {
		maxURLs = 50000
	}
	start := (page - 1) * maxURLs
	if start >= len(entries) {
		writeProblem(w, http.StatusNotFound, "not found")
		return
	}
	end := min(start+maxURLs, len(entries))
	document := sitemapDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: entries[start:end]}
	body, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "sitemap unavailable")
		return
	}
	writeDiscoveryDocument(w, r, "application/xml; charset=utf-8", append([]byte(xml.Header), body...), discoveryModifiedTime())
}

func (s *Server) sitemapEntries(r *http.Request) []sitemapURL {
	pages := []publicSitemapPage{
		{Path: "/", Frequency: "daily", Priority: "1.0"},
		{Path: "/quotes", Frequency: "daily", Priority: "0.9"},
		{Path: "/server/about", Frequency: "weekly", Priority: "0.8"},
		{Path: "/server/help", Frequency: "weekly", Priority: "0.8"},
		{Path: "/server/privacy", Frequency: "weekly", Priority: "0.8"},
		{Path: "/server/terms", Frequency: "weekly", Priority: "0.8"},
		{Path: "/server/dpo", Frequency: "weekly", Priority: "0.7"},
		{Path: "/server/security", Frequency: "weekly", Priority: "0.8"},
		{Path: "/server/security/policy", Frequency: "weekly", Priority: "0.7"},
		{Path: "/server/security/thanks", Frequency: "weekly", Priority: "0.6"},
		{Path: "/server/docs/swagger", Frequency: "weekly", Priority: "0.7"},
		{Path: "/server/docs/graphql", Frequency: "weekly", Priority: "0.7"},
	}
	for _, page := range registeredSitemapPages() {
		pages = append(pages, publicSitemapPage{Path: page.Path, Frequency: "weekly", Priority: "0.7"})
	}
	lastmod := discoveryModifiedTime().Format("2006-01-02")
	entries := make([]sitemapURL, 0, len(pages))
	for _, page := range pages {
		if sitemapAllowed(page.Path, s.cfg.WebRobots.Allow, s.cfg.WebRobots.Deny) {
			entries = append(entries, sitemapURL{Location: BuildURL(r, page.Path), LastMod: lastmod, ChangeFreq: page.Frequency, Priority: page.Priority})
		}
	}
	return entries
}

func sitemapAllowed(path string, allow, deny []string) bool {
	if path == "/server/contact" || strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/debug/") || strings.HasPrefix(path, "/server/preferences/") || strings.HasPrefix(path, "/server/security/report/") {
		return false
	}
	if !isPublicIndexRoute(path) {
		return false
	}
	allowed := len(allow) == 0
	for _, rule := range allow {
		if strings.HasPrefix(path, rule) {
			allowed = true
		}
	}
	for _, rule := range deny {
		if rule != "" && strings.HasPrefix(path, rule) {
			allowed = false
		}
	}
	return allowed
}

func (s *Server) handleLLMSTxt(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WebLLMS.Enabled {
		writeProblem(w, http.StatusNotFound, "not found")
		return
	}
	var output strings.Builder
	output.WriteString("# Anime Quotes API\n\n")
	output.WriteString("> A public, read-only browser and API for an embedded collection of anime quotes.\n\n")
	output.WriteString("## API\n")
	output.WriteString("Base URL: " + BuildURL(r, "/api/v1") + "\n")
	output.WriteString("Authentication: REST reads are public; a Bearer token is required on every MCP request. Obtain one from POST " + BuildURL(r, "/api/v1/server/preferences/tokens/primary") + " or the token page, then create one linked token per MCP client.\n")
	readLimit := s.cfg.Server.RateLimit.Read
	if readLimit.Requests <= 0 || readLimit.Window <= 0 {
		readLimit.Requests = 120
		readLimit.Window = 60
	}
	perMinute := readLimit.Requests * 60 / readLimit.Window
	output.WriteString("Rate limit: " + strconv.Itoa(perMinute) + " requests per minute for read routes.\n")
	output.WriteString("MCP: " + BuildURL(r, "/mcp") + " (Streamable HTTP; Bearer token required)\n\n")
	if s.cfg.WebLLMS.IncludeEndpoints {
		output.WriteString("## Endpoints\n")
		paths, _ := openAPIDocument()["paths"].(map[string]interface{})
		routes := make([]string, 0, len(paths))
		for path := range paths {
			if strings.HasPrefix(path, "/api/v1/") {
				routes = append(routes, path)
			}
		}
		sort.Strings(routes)
		for _, path := range routes {
			operations, _ := paths[path].(map[string]interface{})
			methods := make([]string, 0, len(operations))
			for method := range operations {
				methods = append(methods, method)
			}
			sort.Strings(methods)
			for _, method := range methods {
				operation, _ := operations[method].(map[string]interface{})
				summary, _ := operation["summary"].(string)
				auth := ""
				if _, required := operation["security"]; required {
					auth = " (Bearer token required)"
				}
				output.WriteString("- " + strings.ToUpper(method) + " " + path + " - " + summary + auth + "\n")
				if s.cfg.WebLLMS.IncludeSchemas {
					for _, field := range []string{"parameters", "requestBody", "responses"} {
						if value, exists := operation[field]; exists {
							if schema, marshalErr := json.Marshal(value); marshalErr == nil {
								output.WriteString("  " + field + ": " + string(schema) + "\n")
							}
						}
					}
				}
			}
		}
		output.WriteString("\n")
	}
	output.WriteString("## Capabilities\n- Get a random anime quote.\n- Search quotes by text, anime, and character.\n- Read service health and collection statistics.\n\n")
	securityEmail := s.securityContactEmail(r)
	output.WriteString("## Contact\nAPI issues: " + strings.TrimSpace(s.cfg.Server.Contact.General.Email) + "\nSecurity: " + securityEmail + "\n")
	for _, section := range s.cfg.WebLLMS.CustomSections {
		output.WriteString("\n" + section + "\n")
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	payload := []byte(output.String())
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(payload)
	}
}

func (s *Server) handleAtomFeed(w http.ResponseWriter, r *http.Request) {
	modified := discoveryModifiedTime()
	originURL := BuildURL(r, "/")
	title := s.feedTitle()
	feed := atomFeed{ID: originURL, Title: title, Updated: modified.Format(time.RFC3339), Links: []atomLink{{Href: BuildURL(r, "/feed.xml"), Rel: "self", Type: "application/atom+xml"}, {Href: originURL}}}
	for _, quote := range s.recentPublicQuotes() {
		permalink := BuildURL(r, "/quotes") + "#quote-" + stableQuoteID(quote.Anime, quote.Character, quote.Quote)
		titleText := truncateXMLText(quote.Character+" — "+quote.Anime, 140)
		feed.Entries = append(feed.Entries, atomEntry{ID: permalink, Title: titleText, Link: atomLink{Href: permalink}, Published: modified.Format(time.RFC3339), Updated: modified.Format(time.RFC3339), Summary: truncateXMLText(quote.Quote, 280)})
	}
	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "feed unavailable")
		return
	}
	writeDiscoveryDocument(w, r, "application/atom+xml; charset=utf-8", append([]byte(xml.Header), body...), modified)
}

func (s *Server) handleRSSFeed(w http.ResponseWriter, r *http.Request) {
	modified := discoveryModifiedTime()
	originURL := BuildURL(r, "/")
	feed := rssFeed{Version: "2.0", Channel: rssChannel{Title: s.feedTitle(), Link: originURL, Description: i18n.Translate(s.resolvedLanguage(r), "feeds.description"), Language: s.resolvedLanguage(r), LastBuild: modified.Format(time.RFC1123Z)}}
	for _, quote := range s.recentPublicQuotes() {
		permalink := BuildURL(r, "/quotes") + "#quote-" + stableQuoteID(quote.Anime, quote.Character, quote.Quote)
		feed.Channel.Items = append(feed.Channel.Items, rssItem{Title: truncateXMLText(quote.Character+" — "+quote.Anime, 140), Link: permalink, GUID: permalink, Description: truncateXMLText(quote.Quote, 280), PubDate: modified.Format(time.RFC1123Z)})
	}
	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "feed unavailable")
		return
	}
	writeDiscoveryDocument(w, r, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), body...), modified)
}

func (s *Server) feedTitle() string {
	if title := strings.TrimSpace(s.cfg.Server.SEO.Feed.Title); title != "" {
		return title
	}
	return s.cfg.Server.Branding.Title
}

func (s *Server) recentPublicQuotes() []anime.Quote {
	quotes := s.animeService.SearchQuotes("", "", "")
	maxItems := s.cfg.Server.SEO.Feed.MaxItems
	if maxItems < 1 || maxItems > 200 {
		maxItems = 50
	}
	if len(quotes) > maxItems {
		quotes = quotes[len(quotes)-maxItems:]
	}
	return quotes
}

func stableQuoteID(animeName, character, quote string) string {
	value := animeName + "\x00" + character + "\x00" + quote
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func discoveryModifiedTime() time.Time {
	if parsed, err := time.Parse(time.RFC3339, BuildDate); err == nil {
		return parsed.UTC().Truncate(time.Second)
	}
	return time.Unix(0, 0).UTC()
}

func writeDiscoveryDocument(w http.ResponseWriter, r *http.Request, contentType string, payload []byte, modified time.Time) {
	digest := sha256.Sum256(payload)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", modified.UTC().Format(http.TimeFormat))
	if matchesETag(r.Header.Get("If-None-Match"), etag) || ifModifiedSince(r, modified) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(payload)
	}
}

func matchesETag(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

func ifModifiedSince(r *http.Request, modified time.Time) bool {
	if r.Header.Get("If-None-Match") != "" {
		return false
	}
	date, err := http.ParseTime(r.Header.Get("If-Modified-Since"))
	return err == nil && !modified.UTC().Truncate(time.Second).After(date)
}

func truncateXMLText(value string, limit int) string {
	var cleaned strings.Builder
	for len(value) > 0 {
		runeValue, size := utf8.DecodeRuneInString(value)
		if runeValue == utf8.RuneError && size == 1 {
			value = value[1:]
			continue
		}
		value = value[size:]
		if runeValue == '\t' || runeValue == '\n' || runeValue == '\r' || runeValue >= 0x20 && runeValue <= 0xD7FF || runeValue >= 0xE000 && runeValue <= 0xFFFD || runeValue >= 0x10000 && runeValue <= 0x10FFFF {
			cleaned.WriteRune(runeValue)
		}
	}
	var result strings.Builder
	count := 0
	for _, runeValue := range cleaned.String() {
		if count >= limit {
			break
		}
		result.WriteRune(runeValue)
		count++
	}
	return result.String()
}
