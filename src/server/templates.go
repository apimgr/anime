package server

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/apimgr/anime/src/common/i18n"
	"github.com/apimgr/anime/src/config"
)

// Embed static files and templates
//
//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templateFS embed.FS

var templates *template.Template

// initTemplates initializes the HTML templates
func initTemplates() error {
	var err error
	funcs := template.FuncMap{
		"asset": func(path string) string {
			separator := "?"
			if strings.Contains(path, "?") {
				separator = "&"
			}
			return path + separator + "v=" + assetStamp()
		},
		"t": func(language, key string) string {
			return i18n.Translate(language, key)
		},
		"tf": func(language, key string, values ...interface{}) string {
			arguments := make(map[string]string, len(values)/2)
			for index := 0; index+1 < len(values); index += 2 {
				arguments[fmt.Sprint(values[index])] = fmt.Sprint(values[index+1])
			}
			return i18n.TranslateFormat(language, key, arguments)
		},
		"tp": func(language, key string, count int) string {
			return i18n.TranslatePlural(language, key, count)
		},
		"join": func(values []string, separator string) string {
			return strings.Join(values, separator)
		},
		"jsonLD": func(value interface{}) template.JS {
			encoded, err := json.Marshal(value)
			if err != nil {
				return template.JS("{}")
			}
			return template.JS(string(encoded))
		},
	}
	templates, err = template.New("pages").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	return nil
}

func resolvedTheme(r *http.Request, configured string) string {
	theme := configured
	if cookie, err := r.Cookie("theme"); err == nil {
		switch cookie.Value {
		case "dark", "light", "auto":
			theme = cookie.Value
		}
	}
	if theme != "dark" && theme != "light" && theme != "auto" {
		theme = "dark"
	}
	return theme
}

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, data map[string]interface{}) error {
	return s.renderPageWithStatus(w, r, data, http.StatusOK)
}

func (s *Server) renderPageWithStatus(w http.ResponseWriter, r *http.Request, data map[string]interface{}, status int) error {
	w.Header().Set("Cache-Control", "no-store")
	stamp := assetStamp()
	if current, err := r.Cookie("anime_build"); err == nil && current.Value != stamp && status >= 200 && status < 300 && r.Method != http.MethodHead && versionPurgeEligible(r) {
		w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
	}
	http.SetCookie(w, &http.Cookie{Name: "anime_build", Value: stamp, Path: "/", MaxAge: 31536000, Secure: true, SameSite: http.SameSiteLaxMode})
	w.Header().Set("ETag", `"`+stamp+`"`)
	if language, ok := canonicalLanguage(r.URL.Query().Get("lang")); ok {
		http.SetCookie(w, preferenceCookie(r, "lang", language))
	}
	body, err := s.renderPageBody(r, data)
	if err != nil {
		writeFallbackError(w, r, http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(body)
	return err
}

func versionPurgeEligible(r *http.Request) bool {
	dest, mode := r.Header.Get("Sec-Fetch-Dest"), r.Header.Get("Sec-Fetch-Mode")
	return (dest == "" && mode == "") || (dest == "document" && mode == "navigate")
}

func (s *Server) renderPageBody(r *http.Request, data map[string]interface{}) ([]byte, error) {
	theme := resolvedTheme(r, s.cfg.WebUI.Theme)
	language := s.resolvedLanguage(r)
	data["Lang"] = language
	data["LanguageName"] = languageNativeName(language)
	data["Dir"] = i18n.Direction(language)
	data["Theme"] = theme
	data["BrandingTitle"] = s.cfg.Server.Branding.Title
	data["SEOTitle"] = s.cfg.Server.Branding.Title
	data["BrandingTagline"] = s.cfg.Server.Branding.Tagline
	data["BrandingDescription"] = s.cfg.Server.Branding.Description
	data["BrandingLogo"] = s.cfg.Server.Branding.Logo
	publicPage := isPublicIndexRoute(r.URL.Path)
	if publicPage {
		data["OperatorHeader"] = s.operatorHTML(r, "header")
		data["OperatorFooter"] = s.operatorHTML(r, "footer")
		data["FooterContentConfigured"] = s.operatorContentConfigured(r, "footer")
	}
	data["OperatorAbout"] = s.operatorHTML(r, "about")
	data["OperatorPrivacy"] = s.operatorHTML(r, "privacy")
	data["OperatorTerms"] = s.operatorHTML(r, "terms")
	data["TitleSeparator"] = s.cfg.Server.SEO.TitleSeparator
	data["Favicon"] = s.cfg.Server.Branding.Favicon
	data["Keywords"] = s.cfg.Server.SEO.Keywords
	data["Author"] = s.cfg.Server.SEO.Author
	data["MetaTags"] = s.cfg.Server.SEO.MetaTags
	data["CanonicalURL"] = canonicalURL(r, s.cfg.Server.SEO.Canonical)
	data["Robots"] = robotsDirective(r.URL.Path, bool(s.cfg.Server.SEO.Noindex))
	data["OpenGraphEnabled"] = bool(s.cfg.Server.SEO.OpenGraph.Enabled)
	data["OpenGraphType"] = s.cfg.Server.SEO.OpenGraph.Type
	data["OpenGraphSiteName"] = s.cfg.Server.SEO.OpenGraph.SiteName
	if data["OpenGraphSiteName"] == "" {
		data["OpenGraphSiteName"] = s.cfg.Server.Branding.Title
	}
	data["OpenGraphImage"] = s.cfg.Server.SEO.OGImage
	data["TwitterEnabled"] = bool(s.cfg.Server.SEO.Twitter.Enabled)
	data["TwitterCard"] = s.cfg.Server.SEO.Twitter.Card
	data["TwitterHandle"] = s.cfg.Server.SEO.TwitterHandle
	data["TwitterCreator"] = s.cfg.Server.SEO.Twitter.Creator
	data["VerificationTags"] = verificationMetaTags(s.cfg.Server.SEO.Verification)
	data["BuildDate"] = BuildDate
	if data["Description"] == "" {
		data["Description"] = s.cfg.Server.Branding.Description
	}
	if _, isError := data["ErrorStatus"]; isError {
		data["Robots"] = "noindex,nofollow"
	}
	data["StructuredData"] = map[string]string{
		"@context":    "https://schema.org",
		"@type":       "WebSite",
		"name":        s.cfg.Server.Branding.Title,
		"description": fmt.Sprint(data["Description"]),
		"url":         canonicalURL(r, s.cfg.Server.SEO.Canonical),
	}
	data["NextTheme"] = nextTheme(theme)
	data["CSRFToken"] = s.csrfToken(time.Now())
	data["ShowConsentBanner"] = bool(s.cfg.Server.Privacy.Consent.ShowUntilAcknowledged) && preferenceConsent(r).Timestamp == 0
	consentMessageKey := "consent.message"
	configuredConsentMessage := s.cfg.Server.Privacy.Consent.Message
	if s.cfg.Server.Privacy.Data.Sold {
		consentMessageKey = "consent.message_sold"
		configuredConsentMessage = s.cfg.Server.Privacy.Consent.MessageIfSold
	}
	data["ConsentMessage"] = localizedConfigText(language, consentMessageKey, configuredConsentMessage, "")
	data["ConsentPolicyURL"] = safePolicyURL(s.cfg.Server.Privacy.Consent.Policy.URL)
	data["ConsentPolicyText"] = localizedConfigText(language, "consent.policy_link", s.cfg.Server.Privacy.Consent.Policy.Text, "Privacy policy")
	data["ConsentDecline"] = localizedConfigText(language, "consent.decline", s.cfg.Server.Privacy.Consent.Buttons.Decline, "Decline")
	data["ConsentAccept"] = localizedConfigText(language, "consent.accept", s.cfg.Server.Privacy.Consent.Buttons.Accept, "I Agree")
	data["ConsentPosition"] = consentBannerPosition(s.cfg.Server.Privacy.Consent.Position)
	data["ConsentShowPreferences"] = bool(s.cfg.Server.Privacy.Consent.ShowPreferences)
	data["ConsentPreferencesText"] = localizedConfigText(language, "consent.preferences_link", s.cfg.Server.Privacy.Consent.PreferencesText, "Manage preferences")
	data["CurrentPath"] = r.URL.RequestURI()
	var body bytes.Buffer
	if err := templates.ExecuteTemplate(&body, "base.html", data); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func localizedConfigText(language, key, configured, defaultValue string) string {
	if configured == "" || defaultValue != "" && configured == defaultValue {
		return i18n.Translate(language, key)
	}
	return configured
}

func safePolicyURL(value string) string {
	if value == "" {
		return "/server/privacy"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" {
		return "/server/privacy"
	}
	if parsed.IsAbs() && parsed.Scheme == "https" && parsed.Host != "" {
		return value
	}
	if !parsed.IsAbs() && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(parsed.Path, "//") {
		return value
	}
	return "/server/privacy"
}

func consentBannerPosition(value string) string {
	if value == "top" {
		return value
	}
	return "bottom"
}

func (s *Server) seoMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bool(s.cfg.Server.SEO.Noindex) || !isPublicIndexRoute(r.URL.Path) {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		next.ServeHTTP(w, r)
	})
}

type verificationMetaTag struct {
	Name     string
	Property string
	Content  string
}

func verificationMetaTags(config config.VerificationSEOConfig) []verificationMetaTag {
	tags := make([]verificationMetaTag, 0, 6+len(config.Custom))
	for _, provider := range []struct {
		name     string
		value    string
		property bool
	}{
		{name: "google-site-verification", value: config.Google},
		{name: "msvalidate.01", value: config.Bing},
		{name: "yandex-verification", value: config.Yandex},
		{name: "baidu-site-verification", value: config.Baidu},
		{name: "p:domain_verify", value: config.Pinterest},
		{name: "fb:domain_verification", value: config.Facebook, property: true},
	} {
		if provider.value == "" {
			continue
		}
		tag := verificationMetaTag{Content: provider.value}
		if provider.property {
			tag.Property = provider.name
		} else {
			tag.Name = provider.name
		}
		tags = append(tags, tag)
	}
	for _, custom := range config.Custom {
		tags = append(tags, verificationMetaTag{Name: custom.Name, Property: custom.Property, Content: custom.Content})
	}
	return tags
}

func robotsDirective(path string, private bool) string {
	if private || !isPublicIndexRoute(path) {
		return "noindex,nofollow"
	}
	return "index,follow"
}

func isPublicIndexRoute(path string) bool {
	switch path {
	case "/", "/quotes", "/server/about", "/server/help", "/server/help/mcp", "/server/privacy", "/server/terms", "/server/dpo", "/server/security", "/server/security/policy", "/server/security/thanks", "/server/docs/swagger", "/server/docs/graphql":
		return true
	default:
		return false
	}
}

func canonicalURL(request *http.Request, configured string) string {
	path := request.URL.Path
	if path == "" {
		path = "/"
	}
	if configured == "" {
		return BuildURL(request, path)
	}
	base, err := url.Parse(configured)
	if err != nil {
		return BuildURL(request, path)
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	base.RawQuery = ""
	base.Fragment = ""
	return base.String()
}

func languageNativeName(language string) string {
	return i18n.NativeName(language)
}

func (s *Server) handleLocaleJSON(w http.ResponseWriter, r *http.Request) {
	language := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/locales/"), ".json")
	content, ok := i18n.JSON(language)
	if !ok {
		writeProblem(w, http.StatusNotFound, "locale not found")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(content)
}

// serveStatic sets up the static file server for CSS, JS, and images
func assetStamp() string {
	version := strings.TrimSpace(Version)
	commit := strings.TrimSpace(CommitID)
	if version == "" {
		version = "dev"
	}
	if commit == "" {
		commit = "unknown"
	}
	return version + "-" + commit
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := "static/" + strings.TrimPrefix(r.URL.Path, "/static/")
	content, err := staticFS.ReadFile(name)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "asset not found")
		return
	}
	digest := sha256.Sum256(content)
	etag := fmt.Sprintf(`"%x"`, digest[:])
	w.Header().Set("ETag", etag)
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" && strings.EqualFold(path.Ext(name), ".svg") {
		contentType = "image/svg+xml"
	}
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Query().Get("v") == assetStamp() {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
}
