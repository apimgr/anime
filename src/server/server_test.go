package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/config"
	"github.com/apimgr/anime/src/database"
	"github.com/apimgr/anime/src/mode"
	"github.com/apimgr/anime/src/updater"
)

const testQuotes = `[
	{"anime":"Naruto","character":"Naruto Uzumaki","quote":"Believe it!"}
]`

func newTestServer(t *testing.T) *Server {
	t.Helper()
	svc, err := anime.NewService([]byte(testQuotes))
	if err != nil {
		t.Fatalf("anime.NewService returned error: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Server.Token = "adm_0123456789abcdefghijklmnopqrstuv"
	cfg.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	db, err := database.Open(config.DatabaseConfig{Driver: "sqlite", URL: ":memory:", Pool: config.DatabasePool{MaxOpen: 1, MaxIdle: 1}}, t.TempDir())
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.SQL.Close() })
	s, err := NewServer(svc, cfg, "8080", "0.0.0.0", db)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return s
}

func TestCLIAutodiscoverAndBinaryRoute(t *testing.T) {
	binary := []byte("cli binary")
	digest := sha256.Sum256(binary)
	checksum := hex.EncodeToString(digest[:]) + "  anime-cli-linux-amd64\n"
	var releaseServer *httptest.Server
	releaseServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/apimgr/anime/releases/latest":
			_, _ = fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[{"name":"anime-cli-linux-amd64","browser_download_url":%q},{"name":"sha256.txt","browser_download_url":%q}]}`, releaseServer.URL+"/binary", releaseServer.URL+"/checksums")
		case "/checksums":
			_, _ = fmt.Fprint(w, checksum)
		case "/binary":
			_, _ = w.Write(binary)
		default:
			http.NotFound(w, r)
		}
	}))
	defer releaseServer.Close()

	s := newTestServer(t)
	s.cfg.Server.Update.CLIMinVersion = "1.0.0"
	s.updateClient = &updater.Client{
		APIBase:   releaseServer.URL + "/repos/apimgr/anime",
		HTTP:      releaseServer.Client(),
		AllowHTTP: true,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/autodiscover", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("autodiscover status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"cli_min_version":"1.0.0"`) || !strings.Contains(rec.Body.String(), `"linux-amd64":{"version":"1.2.3","sha256":"`+hex.EncodeToString(digest[:])+`"}`) {
		t.Fatalf("autodiscover response missing update metadata: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/cli/binaries/anime-cli-linux-amd64", nil)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != releaseServer.URL+"/binary" {
		t.Fatalf("binary redirect status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/cli/binaries/not-an-anime-binary", nil)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unsupported binary status=%d", rec.Code)
	}
}

func TestNewServerRoutes(t *testing.T) {
	s := newTestServer(t)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/server/healthz"},
		{"GET", "/server/preferences"},
		{"GET", "/server/preferences/sync"},
		{"GET", "/server/security"},
		{"GET", "/server/security/policy"},
		{"GET", "/server/security/thanks"},
		{"GET", "/server/dpo"},
		{"GET", "/robots.txt"},
		{"GET", "/.well-known/security.txt"},
		{"GET", "/sitemap.xml"},
		{"GET", "/feed.xml"},
		{"GET", "/rss.xml"},
		{"GET", "/.well-known/llms.txt"},
		{"GET", "/llms.txt"},
		{"GET", "/manifest.json"},
		{"GET", "/sw.js"},
		{"GET", "/api/v1"},
		{"GET", "/api"},
		{"GET", "/api/v1/random"},
		{"GET", "/api/v1/quotes"},
		{"GET", "/api/v1/server/healthz"},
		{"GET", "/api/v1/server/about"},
		{"GET", "/api/v1/server/preferences"},
		{"GET", "/api/v1/server/preferences/sync"},
		{"GET", "/api/v1/server/preferences/sync/import?theme=light"},
		{"GET", "/api/v1/server/help"},
		{"GET", "/api/v1/server/privacy"},
		{"GET", "/api/v1/server/terms"},
		{"GET", "/api/v1/server/dpo"},
		{"GET", "/api/v1/server/security"},
		{"GET", "/api/v1/server/security/policy"},
		{"GET", "/api/v1/server/security/thanks"},
		{"GET", "/api/v1/stats"},
		{"GET", "/api/v1/random.txt"},
		{"GET", "/api/v1/quotes.txt"},
		{"GET", "/api/v1/server/healthz.txt"},
		{"GET", "/api/healthz"},
		{"GET", "/server/about"},
		{"GET", "/server/help"},
		{"GET", "/server/privacy"},
		{"GET", "/server/terms"},
		{"GET", "/server/security"},
		{"GET", "/server/security/policy"},
		{"GET", "/server/security/thanks"},
		{"GET", "/server/docs/swagger"},
		{"GET", "/api/v1/server/swagger"},
		{"GET", "/server/docs/graphql"},
		{"GET", "/quotes"},
		{"GET", "/api/v1/server/graphql?query=%7BrandomQuote%7Banime%7D%7D"},
		{"GET", "/api/v1/stats.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			s.router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d for %s %s", rec.Code, tt.method, tt.path)
			}
		})
	}
}

func TestServerRootRedirectsToAbout(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/server/about" {
		t.Fatalf("server root redirect status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestErrorResponsesUseThemedHTMLAndNegotiatedJSON(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/missing-page", nil)
	request.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("HTML error status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	body := recorder.Body.String()
	if strings.Count(body, "<main") != 1 || strings.Count(body, "<h1") != 1 || !strings.Contains(body, "data-theme=\"dark\"") || !strings.Contains(body, s.cfg.Server.Branding.Title+" · MIT License") {
		t.Fatalf("HTML error did not use the shared themed shell: %s", body)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Header().Get("Content-Type"), "application/problem+json") || !strings.Contains(recorder.Body.String(), `"status":404`) {
		t.Fatalf("API error status=%d content-type=%q body=%s", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
}

func TestRegisteredPagesDrivePreferenceHubAndHelpAPI(t *testing.T) {
	s := newTestServer(t)
	preferences := registeredServerPages("preferences")
	if len(preferences) != 2 || preferences[0].Path != "/server/preferences/tokens" || preferences[1].Path != "/server/preferences/sync" {
		t.Fatalf("registered preferences pages=%+v", preferences)
	}
	help := registeredServerPages("help")
	if len(help) != 1 || help[0].Path != "/server/help/mcp" {
		t.Fatalf("unexpected enabled help pages=%+v", help)
	}
	request := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if strings.Count(recorder.Body.String(), `href="/server/preferences/sync"`) != 1 {
		t.Fatalf("preference hub does not render the registry exactly once: %s", recorder.Body.String())
	}
	if strings.Count(recorder.Body.String(), `href="/server/preferences/tokens"`) != 1 {
		t.Fatalf("preference hub does not render the token page exactly once: %s", recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `name="token"`) {
		t.Fatalf("tokens page status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/server/help", nil)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), `"path":"/server/help/mcp"`) {
		t.Fatalf("help API does not reflect the enabled page registry: %s", recorder.Body.String())
	}
}

func TestSecurityAPIMirrorsPublicSecurityPages(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/server/security", nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	body := recorder.Body.String()
	for _, expected := range []string{"github.com/apimgr/anime/security/advisories/new", "server/security/policy", "server/security/thanks", `"available":false`} {
		if !strings.Contains(body, expected) {
			t.Errorf("security JSON missing %q: %s", expected, body)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/server/security/policy", nil)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	for _, expected := range []string{"Scope", "Out of scope", "allow 90 days", "Safe harbor", "Reporting"} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Errorf("policy API missing %q: %s", expected, recorder.Body.String())
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/server/security/thanks", nil)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), `"entries":[]`) {
		t.Errorf("empty acknowledgment list does not match its page state: %s", recorder.Body.String())
	}
}

func TestServerPagesRenderSharedLayoutAndAccurateContent(t *testing.T) {
	s := newTestServer(t)
	tests := []struct {
		path string
		want string
	}{
		{"/server/about", "A full-stack application"},
		{"/server/help", "Search quotes"},
		{"/server/privacy", "search terms may therefore appear"},
		{"/server/terms", "Governing law"},
		{"/server/dpo", "Data Protection Officer"},
		{"/server/preferences", "Appearance"},
		{"/server/preferences/sync", "Sync preferences"},
		{"/server/security", "GitHub private vulnerability reporting"},
		{"/server/security/policy", "allow 90 days"},
		{"/server/security/thanks", "No researchers have been listed"},
		{"/server/healthz", "Status: healthy"},
		{"/server/docs/swagger", "OpenAPI 3.1 specification"},
		{"/server/docs/graphql", "GraphQL explorer"},
		{"/quotes", "matching quotes"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		rec := httptest.NewRecorder()
		s.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status=%d, want %d", test.path, rec.Code, http.StatusOK)
			continue
		}
		body := rec.Body.String()
		if !strings.Contains(body, test.want) || !strings.Contains(body, s.cfg.Server.Branding.Title+" · MIT License") || !strings.Contains(body, "name=\"description\"") {
			t.Errorf("%s did not render current content and shared layout", test.path)
		}
		if !strings.Contains(body, `<html lang="en" dir="ltr" data-theme="dark">`) ||
			!strings.Contains(body, `class="skip-link" href="#main-content"`) ||
			!strings.Contains(body, `class="skip-link" href="#main-nav"`) ||
			!strings.Contains(body, `class="nav-links"`) ||
			!strings.Contains(body, `id="mobile-menu-state" type="checkbox"`) {
			t.Errorf("%s did not render accessible document metadata and navigation", test.path)
		}
	}
}

func TestOperatorBrandingContentIsLocalizedAndSanitized(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Server.Branding.Content = map[string]config.BrandingPageContent{
		"header": {Format: "html", Body: config.LocalizedContentBody{"default": `<p>Header <script>alert(1)</script><a href="/server/contact?type=privacy">report</a></p>`}},
		"footer": {Format: "html", Body: config.LocalizedContentBody{"default": `<p>Footer custom</p>`}},
		"about":  {Format: "md", Body: config.LocalizedContentBody{"default": "Operator **about**", "fr": "Bonjour"}},
		"terms":  {Format: "md", Body: config.LocalizedContentBody{"default": "Custom **terms**"}},
	}
	request := httptest.NewRequest(http.MethodGet, "/server/about?lang=fr", nil)
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	for _, expected := range []string{"Bonjour", "Header", "Footer custom"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("localized branding content missing %q: %s", expected, response.Body.String())
		}
	}
	for _, forbidden := range []string{"<script>alert(1)", "alert(1)", `href="/server/contact`} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Errorf("unsafe or forbidden branding content remained (%q): %s", forbidden, response.Body.String())
		}
	}

	for _, path := range []string{"/server/terms", "/api/v1/server/terms"} {
		request = httptest.NewRequest(http.MethodGet, path, nil)
		response = httptest.NewRecorder()
		s.router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Custom") || strings.Contains(response.Body.String(), "Governing law") {
			t.Errorf("%s did not use the configured terms content: %d %s", path, response.Code, response.Body.String())
		}
	}
}

func TestSEOOutputAndRobotsPolicy(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Server.Branding.Title = "Quote Library"
	s.cfg.Server.SEO.Canonical = "https://quotes.example"
	s.cfg.Server.SEO.Verification.Google = "verify123"
	s.cfg.Server.SEO.Verification.Custom = []config.CustomVerificationMeta{{Property: "fb_app_id", Content: "12345"}}

	request := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{
		`<link rel="canonical" href="https://quotes.example/server/about">`,
		`<meta name="google-site-verification" content="verify123">`,
		`<meta property="fb_app_id" content="12345">`,
		`<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite"`,
		`<meta name="robots" content="index,follow">`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("public SEO output missing %q: %s", expected, body)
		}
	}
	if response.Header().Get("X-Robots-Tag") != "" {
		t.Errorf("public route unexpectedly has X-Robots-Tag %q", response.Header().Get("X-Robots-Tag"))
	}

	for _, path := range []string{"/server/preferences", "/server/healthz", "/api/v1/quotes", "/not-found"} {
		request = httptest.NewRequest(http.MethodGet, path, nil)
		response = httptest.NewRecorder()
		s.router.ServeHTTP(response, request)
		if response.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Errorf("private route %s X-Robots-Tag=%q", path, response.Header().Get("X-Robots-Tag"))
		}
		if strings.Contains(response.Header().Get("Content-Type"), "text/html") && !strings.Contains(response.Body.String(), `<meta name="robots" content="noindex,nofollow">`) {
			t.Errorf("private HTML route %s lacks noindex meta tag", path)
		}
	}

	s.cfg.Server.SEO.Noindex = true
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Header().Get("X-Robots-Tag") != "noindex, nofollow" || !strings.Contains(response.Body.String(), `<meta name="robots" content="noindex,nofollow">`) {
		t.Fatalf("private instance SEO policy header=%q body=%s", response.Header().Get("X-Robots-Tag"), response.Body.String())
	}
}

func TestPageThemeUsesValidCookieAndConfiguredFallback(t *testing.T) {
	s := newTestServer(t)
	s.cfg.WebUI.Theme = "light"
	request := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	request.AddCookie(&http.Cookie{Name: "theme", Value: "auto"})
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), `<html lang="en" dir="ltr" data-theme="auto">`) {
		t.Fatalf("valid theme cookie was not used in server-rendered page")
	}

	request = httptest.NewRequest(http.MethodGet, "/server/about", nil)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), `<html lang="en" dir="ltr" data-theme="light">`) {
		t.Fatalf("configured theme was not used when no preference cookie was sent")
	}

	request = httptest.NewRequest(http.MethodGet, "/server/about", nil)
	request.AddCookie(&http.Cookie{Name: "theme", Value: "invalid"})
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), `<html lang="en" dir="ltr" data-theme="light">`) {
		t.Fatalf("invalid theme cookie was not ignored")
	}
}

func TestPreferencesThemeFormSavesWithoutJavaScript(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `name="theme" value="light"`) {
		t.Fatalf("preferences page status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	for _, required := range []string{`name="lang"`, `name="consent_form" value="1"`, `name="consent_preferences"`, `name="consent_analytics"`, `popovertarget="preferences-reset"`, `name="action" value="reset"`, `>Save</button>`} {
		if !strings.Contains(recorder.Body.String(), required) {
			t.Errorf("preferences form missing %q", required)
		}
	}
	token := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(recorder.Body.String())
	if len(token) != 2 {
		t.Fatal("preferences form did not render a CSRF token")
	}
	form := url.Values{"csrf_token": {token[1]}, "theme": {"light"}, "redirect": {"/server/preferences"}}
	request = httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", "http://example.com/server/about?q=anime")
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/server/about?q=anime" {
		t.Fatalf("preference save status=%d location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
	if cookie := recorder.Result().Cookies(); len(cookie) != 1 || cookie[0].Name != "theme" || cookie[0].Value != "light" || cookie[0].Path != "/" || cookie[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("preference save cookie=%v", cookie)
	}
}

func TestPreferencesRejectInvalidCSRFTokensAndRedirects(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader("theme=light"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF token status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
	form := url.Values{"csrf_token": {s.csrfToken(time.Now())}, "theme": {"light"}}
	request = httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://attacker.example")
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin preference save status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
	for _, target := range []string{"https://example.invalid/", "//example.invalid/", "/\\example.invalid", "/server/preferences"} {
		if got := safePreferencesRedirect(target); got != "/" {
			t.Errorf("safePreferencesRedirect(%q)=%q, want /", target, got)
		}
	}
}

func TestPreferencesConsentAndReset(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"csrf_token": {s.csrfToken(time.Now())}, "consent_form": {"1"}, "consent_preferences": {"on"}, "theme": {"dark"}}
	request := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("consent save status=%d", recorder.Code)
	}
	var foundConsent bool
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name != "cookie_consent" {
			continue
		}
		foundConsent = true
		value, decodeErr := url.QueryUnescape(cookie.Value)
		var consent preferenceConsentState
		jsonErr := json.Unmarshal([]byte(value), &consent)
		if decodeErr != nil || jsonErr != nil || !consent.Essential || !consent.Preferences || consent.Analytics || consent.Timestamp == 0 {
			t.Fatalf("unexpected consent cookie %q: %+v (%v, %v)", cookie.Value, consent, decodeErr, jsonErr)
		}
	}
	if !foundConsent {
		t.Fatal("consent cookie was not written")
	}
	form = url.Values{"csrf_token": {s.csrfToken(time.Now())}, "action": {"reset"}}
	request = httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	request.AddCookie(&http.Cookie{Name: "cookie_consent", Value: `{"essential":true}`})
	request.AddCookie(&http.Cookie{Name: "anime_pref_sort", Value: "name"})
	request.AddCookie(&http.Cookie{Name: "owner_token", Value: "credential"})
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/" || recorder.Header().Get("Clear-Site-Data") != `"storage"` {
		t.Fatalf("reset response status=%d location=%q clear-site-data=%q", recorder.Code, recorder.Header().Get("Location"), recorder.Header().Get("Clear-Site-Data"))
	}
	cleared := map[string]bool{}
	for _, cookie := range recorder.Result().Cookies() {
		cleared[cookie.Name] = cookie.MaxAge < 0
	}
	if !cleared["theme"] || !cleared["cookie_consent"] || !cleared["anime_pref_sort"] || cleared["owner_token"] {
		t.Fatalf("reset did not selectively clear cookies: %#v", cleared)
	}
}

func TestConsentBannerAndNoJavaScriptActions(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), `class="consent-banner consent-banner-bottom"`) || !strings.Contains(recorder.Body.String(), `action="/server/consent"`) || !strings.Contains(recorder.Body.String(), "I Agree") || !strings.Contains(recorder.Body.String(), "Decline") {
		t.Fatal("first visit did not render the no-JavaScript consent banner")
	}
	s.cfg.Server.Privacy.Data.Sold = true
	soldRequest := httptest.NewRequest(http.MethodGet, "/server/about?lang=es", nil)
	soldResponse := httptest.NewRecorder()
	s.router.ServeHTTP(soldResponse, soldRequest)
	if !strings.Contains(soldResponse.Body.String(), "Sus datos pueden compartirse o venderse") {
		t.Errorf("sold-data consent banner was not translated and updated: %s", soldResponse.Body.String())
	}
	s.cfg.Server.Privacy.Data.Sold = false
	form := url.Values{"csrf_token": {s.csrfToken(time.Now())}, "choice": {"decline"}, "redirect": {"/server/about"}}
	request = httptest.NewRequest(http.MethodPost, "/server/consent", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/server/about" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("consent response status=%d location=%q cache=%q", recorder.Code, recorder.Header().Get("Location"), recorder.Header().Get("Cache-Control"))
	}
	var consentCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "cookie_consent" {
			consentCookie = cookie
		}
	}
	if consentCookie == nil {
		t.Fatal("decline did not persist essential-only consent")
	}
	value, decodeErr := url.QueryUnescape(consentCookie.Value)
	var consent preferenceConsentState
	jsonErr := json.Unmarshal([]byte(value), &consent)
	if decodeErr != nil || jsonErr != nil || !consent.Essential || consent.Preferences || consent.Analytics {
		t.Fatalf("decline persisted invalid consent: %+v (%v, %v)", consent, decodeErr, jsonErr)
	}
	request = httptest.NewRequest(http.MethodGet, "/server/about", nil)
	request.AddCookie(consentCookie)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if strings.Contains(recorder.Body.String(), `class="consent-banner"`) {
		t.Fatal("consent banner was rendered after a valid choice")
	}
}

func TestPreferencesAPIStoresOnlySupportedValues(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences", strings.NewReader(`{"theme":"light","lang":"fr"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"theme":"light"`) || !strings.Contains(recorder.Body.String(), `"lang":"fr"`) {
		t.Fatalf("preference API status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	values := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		values[cookie.Name] = cookie.Value
	}
	if len(values) != 2 || values["theme"] != "light" || values["lang"] != "fr" {
		t.Fatalf("preference API cookie=%v", cookies)
	}
}

func TestPreferencesSyncExportAndImport(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/server/preferences/sync", nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "http://example.com/server/preferences/sync/import?lang=en&amp;theme=dark") {
		t.Fatalf("sync export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Count(recorder.Body.String(), `data-action="copy-text"`) != 2 {
		t.Fatal("preferences export is missing its two copy controls")
	}
	if !strings.Contains(recorder.Body.String(), `name="code"`) || !strings.Contains(recorder.Body.String(), `name="csrf_token"`) {
		t.Fatal("sync page did not render its import form and CSRF token")
	}
	token := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(recorder.Body.String())
	if len(token) != 2 {
		t.Fatal("sync import form did not render a CSRF token")
	}
	form := url.Values{"csrf_token": {token[1]}, "code": {"theme=light&lang=en"}, "redirect": {"/server/preferences/sync"}}
	request = httptest.NewRequest(http.MethodPost, "/server/preferences/sync", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/server/preferences/sync" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("sync form import status=%d location=%q cache=%q", recorder.Code, recorder.Header().Get("Location"), recorder.Header().Get("Cache-Control"))
	}
	cookies := recorder.Result().Cookies()
	cookiesByName := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		cookiesByName[cookie.Name] = cookie.Value
	}
	if len(cookiesByName) != 2 || cookiesByName["theme"] != "light" || cookiesByName["lang"] != "en" {
		t.Fatalf("sync form import applied unexpected cookies: %v", cookies)
	}

	request = httptest.NewRequest(http.MethodGet, "/server/preferences/sync/import?theme=light&lang=en&cookie_consent=forged", nil)
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("sync import status=%d location=%q cache=%q", recorder.Code, recorder.Header().Get("Location"), recorder.Header().Get("Cache-Control"))
	}
	cookies = recorder.Result().Cookies()
	cookiesByName = make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		cookiesByName[cookie.Name] = cookie.Value
	}
	if len(cookiesByName) != 2 || cookiesByName["theme"] != "light" || cookiesByName["lang"] != "en" {
		t.Fatalf("sync import applied unexpected cookies: %v", cookies)
	}
}

func TestPreferencesSyncContentNegotiation(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/server/preferences/sync", nil)
	request.Header.Set("Accept", "text/plain")
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/plain; charset=utf-8" || !strings.HasPrefix(recorder.Body.String(), "URL: http://example.com/server/preferences/sync/import?") || !strings.Contains(recorder.Body.String(), "\nCode: ") {
		t.Fatalf("text export status=%d type=%q body=%q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/server/preferences/sync", nil)
	request.Header.Set("Accept", "application/json")
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"url":"http://example.com/server/preferences/sync/import?`) || !strings.Contains(recorder.Body.String(), `"code":"`) {
		t.Fatalf("JSON export status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPreferencesSyncAPIImportDirectValuesOverrideCode(t *testing.T) {
	s := newTestServer(t)
	code := base64.RawURLEncoding.EncodeToString([]byte("lang=en&theme=dark"))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/sync/import?code="+url.QueryEscape(code)+"&theme=light", nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"theme":"light"`) || !strings.Contains(recorder.Body.String(), `"ignored":[]`) {
		t.Fatalf("sync API import status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestURLNormalizeMiddlewarePreservesQuery(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/server/about/?q=anime", nil)
	recorder := httptest.NewRecorder()
	urlNormalizeMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("normalized request should redirect")
	})).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMovedPermanently || recorder.Header().Get("Location") != "/server/about?q=anime" {
		t.Fatalf("redirect status=%d location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
}

func TestDebugRoutesRequireDebugMode(t *testing.T) {
	if err := mode.SetAppMode("production"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mode.SetAppMode("production") })
	production := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/debug/pprof/", nil)
	response := httptest.NewRecorder()
	production.router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("production pprof status=%d, want %d", response.Code, http.StatusNotFound)
	}

	if err := mode.SetAppMode("debug"); err != nil {
		t.Fatal(err)
	}
	debug := newTestServer(t)
	response = httptest.NewRecorder()
	debug.router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated debug pprof status=%d, want %d", response.Code, http.StatusUnauthorized)
	}
	request.Header.Set("Authorization", "Bearer "+debug.cfg.Server.Token)
	response = httptest.NewRecorder()
	debug.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("debug pprof status=%d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	for _, path := range []string{
		"/debug/vars", "/debug/config", "/debug/routes", "/debug/cache",
		"/debug/db", "/debug/scheduler", "/debug/memory", "/debug/goroutines",
		"/api/debug/memory", "/api/v1/debug/memory",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+debug.cfg.Server.Token)
		rec := httptest.NewRecorder()
		debug.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("debug route %s status=%d, want %d: %s", path, rec.Code, http.StatusOK, rec.Body.String())
		}
		if path == "/debug/config" && strings.Contains(rec.Body.String(), debug.cfg.Server.Token) {
			t.Error("debug config response leaked the operator token")
		}
	}
}

func TestHandleHealthz(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/server/healthz", nil)
	rec := httptest.NewRecorder()
	s.handleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Status: healthy") {
		t.Errorf("expected health page, got %q", rec.Body.String())
	}
}

func TestHandleRobotsTxt(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	rec := httptest.NewRecorder()
	s.handleRobotsTxt(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Allow: /") {
		t.Errorf("expected robots.txt to contain allow rules, got %q", body)
	}
	if !strings.Contains(body, "Disallow: /debug") {
		t.Errorf("expected robots.txt to contain deny rules, got %q", body)
	}
}

func TestDiscoveryDocumentsUsePublicRoutesAndConditionalCaching(t *testing.T) {
	s := newTestServer(t)
	s.cfg.WebRobots.Deny = []string{"/server/privacy"}

	request := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	request.Host = "example.com"
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("sitemap status=%d body=%q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, required := range []string{BuildURL(request, "/"), BuildURL(request, "/server/docs/swagger"), BuildURL(request, "/server/help/mcp")} {
		if !strings.Contains(body, required) {
			t.Errorf("sitemap missing %q: %s", required, body)
		}
	}
	for _, forbidden := range []string{"/server/privacy", "/server/healthz", "/server/contact", "/api/", "/feed.xml"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("sitemap contains forbidden %q: %s", forbidden, body)
		}
	}
	if response.Header().Get("ETag") == "" || response.Header().Get("Last-Modified") == "" || response.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Errorf("sitemap cache headers incomplete: %v", response.Header())
	}

	conditional := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	conditional.Host = request.Host
	conditional.Header.Set("If-None-Match", response.Header().Get("ETag"))
	conditionalResponse := httptest.NewRecorder()
	s.router.ServeHTTP(conditionalResponse, conditional)
	if conditionalResponse.Code != http.StatusNotModified || conditionalResponse.Body.Len() != 0 {
		t.Errorf("conditional sitemap status=%d body=%q", conditionalResponse.Code, conditionalResponse.Body.String())
	}
}

func TestDiscoveryFeedsIncludeStableQuotePermalinksAndSupportHEAD(t *testing.T) {
	s := newTestServer(t)
	for _, route := range []struct {
		path        string
		contentType string
	}{{"/feed.xml", "application/atom+xml; charset=utf-8"}, {"/rss.xml", "application/rss+xml; charset=utf-8"}} {
		request := httptest.NewRequest(http.MethodGet, route.path, nil)
		request.Host = "example.com"
		response := httptest.NewRecorder()
		s.router.ServeHTTP(response, request)
		permalink := BuildURL(request, "/quotes") + "#quote-" + stableQuoteID("Naruto", "Naruto Uzumaki", "Believe it!")
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != route.contentType {
			t.Errorf("%s status=%d content-type=%q", route.path, response.Code, response.Header().Get("Content-Type"))
		}
		if response.Header().Get("ETag") == "" || response.Header().Get("Last-Modified") == "" || response.Header().Get("Cache-Control") != "public, max-age=300" {
			t.Errorf("%s cache headers incomplete: %v", route.path, response.Header())
		}
		if strings.Contains(route.path, "feed") && !strings.Contains(response.Body.String(), permalink) {
			t.Errorf("Atom feed lacks absolute quote permalink: %s", response.Body.String())
		}
		if strings.Contains(route.path, "rss") && !strings.Contains(response.Body.String(), permalink) {
			t.Errorf("RSS feed lacks absolute quote permalink: %s", response.Body.String())
		}
		head := httptest.NewRequest(http.MethodHead, route.path, nil)
		head.Host = "example.com"
		headResponse := httptest.NewRecorder()
		s.router.ServeHTTP(headResponse, head)
		if headResponse.Code != http.StatusOK || headResponse.Body.Len() != 0 {
			t.Errorf("HEAD %s status=%d body=%q", route.path, headResponse.Code, headResponse.Body.String())
		}
	}
}

func TestHandleSecurityTxt(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/.well-known/security.txt", nil)
	rec := httptest.NewRecorder()
	s.handleSecurityTxt(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Contact: https://github.com/apimgr/anime/security/advisories/new") || !strings.Contains(body, "Policy: http://example.com/server/security") {
		t.Errorf("expected current private reporting and policy URLs, got %q", body)
	}
}

func TestSecurityIDRotatesEveryTwoDaysAndAcceptsPreviousWindow(t *testing.T) {
	s := newTestServer(t)
	now := time.Date(2026, time.January, 4, 12, 0, 0, 0, time.UTC)
	current := s.securityID(now)
	previous := s.securityID(now.Add(-48 * time.Hour))
	if current == previous {
		t.Fatal("security ID did not rotate across a 48-hour boundary")
	}
	if !s.validSecurityID(current, now) || !s.validSecurityID(previous, now) {
		t.Fatal("current and previous security IDs should be valid")
	}
	if s.validSecurityID("0123456789abcdef", now) {
		t.Fatal("unissued security ID was accepted")
	}
}

func TestHandleSecurityTxtIgnoresAdminEmail(t *testing.T) {
	s := newTestServer(t)
	s.cfg.WebSecurity.Admin = "admin@example.com"

	req := httptest.NewRequest("GET", "/.well-known/security.txt", nil)
	rec := httptest.NewRecorder()
	s.handleSecurityTxt(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Contact: https://github.com/apimgr/anime/security/advisories/new") || strings.Contains(body, "admin@example.com") {
		t.Errorf("admin email must not replace the private reporting channel, got %q", body)
	}
}

func TestDPOContactFallbackAndJSON(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Server.Contact.DPO.Name = "Privacy Officer"
	s.cfg.Server.Contact.General.Email = "privacy@example.test"
	s.cfg.WebSecurity.Admin = "admin@example.test"
	name, email := s.dpoContact()
	if name != "Privacy Officer" || email != "privacy@example.test" {
		t.Fatalf("DPO contact = (%q, %q), want configured name and general email", name, email)
	}
	s.cfg.Server.Contact.DPO.Email = "dpo@example.test"
	_, email = s.dpoContact()
	if email != "dpo@example.test" {
		t.Fatalf("DPO email = %q, want configured DPO email", email)
	}
	s.cfg.Server.Contact.DPO.Email = ""
	pageRequest := httptest.NewRequest(http.MethodGet, "/server/dpo", nil)
	pageResponse := httptest.NewRecorder()
	s.router.ServeHTTP(pageResponse, pageRequest)
	if pageResponse.Code != http.StatusOK || !strings.Contains(pageResponse.Body.String(), `mailto:privacy@example.test`) || strings.Contains(pageResponse.Body.String(), "admin@example.test") || strings.Contains(pageResponse.Body.String(), `href="/server/contact"`) {
		t.Fatalf("DPO page exposed an invalid contact or admin address: %d %s", pageResponse.Code, pageResponse.Body.String())
	}
	s.cfg.Server.Contact.DPO.Email = "dpo@example.test"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/dpo", nil)
	req.Host = "example.test"
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"privacy_url":"http://example.test/server/privacy"`) || !strings.Contains(rec.Body.String(), `"email":"dpo@example.test"`) {
		t.Fatalf("unexpected DPO JSON: %d %s", rec.Code, rec.Body.String())
	}
}

func TestTermsHTMLAndJSONUseTheSameBuiltInContent(t *testing.T) {
	s := newTestServer(t)
	for _, path := range []string{"/server/terms", "/api/v1/server/terms"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		s.router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d: %s", path, recorder.Code, recorder.Body.String())
		}
		for _, expected := range []string{"stop using the service if you do not agree", "bypass rate limits", "provided as is", "current text on this page", "Governing law"} {
			if !strings.Contains(recorder.Body.String(), expected) {
				t.Errorf("%s missing built-in terms text %q: %s", path, expected, recorder.Body.String())
			}
		}
	}
}

func TestPrivacyPageAndAPIReflectBrowserDataHandling(t *testing.T) {
	s := newTestServer(t)
	page := httptest.NewRecorder()
	s.router.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/server/privacy", nil))
	for _, expected := range []string{"Search terms are sent as URL query parameters", "theme and language", "cookie-consent", "not configured"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Errorf("privacy page missing %q", expected)
		}
	}
	api := httptest.NewRecorder()
	s.router.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/server/privacy", nil))
	for _, expected := range []string{`"data_stored_on_server":true`, `"data_sold":false`, `"preferences"`, `"tracking":{"enabled":false`, `"user_opted_out":false`} {
		if !strings.Contains(api.Body.String(), expected) {
			t.Errorf("privacy API missing %q: %s", expected, api.Body.String())
		}
	}
	s.cfg.Server.Privacy.Data.Sold = true
	s.cfg.Server.Privacy.Retention.Period = "Configured retention details."
	s.cfg.Server.Privacy.ThirdParty.Services = []config.PrivacyThirdPartyService{{Name: "Example Analytics", Purpose: "Aggregate page usage", DataSent: "Page views", PolicyURL: "https://analytics.example/privacy"}}
	soldPage := httptest.NewRecorder()
	s.router.ServeHTTP(soldPage, httptest.NewRequest(http.MethodGet, "/server/privacy", nil))
	if !strings.Contains(soldPage.Body.String(), "Visitor data may be shared with or sold") || !strings.Contains(soldPage.Body.String(), "id=\"ccpa-opt-out\"") || !strings.Contains(soldPage.Body.String(), "Configured retention details.") || !strings.Contains(soldPage.Body.String(), "Example Analytics") {
		t.Errorf("privacy page did not render the configured sale notice and opt-out section: %s", soldPage.Body.String())
	}
	soldAPI := httptest.NewRecorder()
	s.router.ServeHTTP(soldAPI, httptest.NewRequest(http.MethodGet, "/api/v1/server/privacy", nil))
	for _, expected := range []string{`"data_sold":true`, `"applicable":true`, `may be shared with or sold`, `Configured retention details.`, `Example Analytics`} {
		if !strings.Contains(soldAPI.Body.String(), expected) {
			t.Errorf("sold-data privacy API missing %q: %s", expected, soldAPI.Body.String())
		}
	}
}

func TestHandleManifest(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest("GET", "/manifest.json", nil)
	rec := httptest.NewRecorder()
	s.handleManifest(rec, req)
	if !strings.Contains(rec.Body.String(), `"background_color": "#1a1a1a"`) {
		t.Errorf("expected dark background color for dark theme, got %q", rec.Body.String())
	}

	s.cfg.WebUI.Theme = "light"
	rec = httptest.NewRecorder()
	s.handleManifest(rec, req)
	if !strings.Contains(rec.Body.String(), `"background_color": "#ffffff"`) {
		t.Errorf("expected light background color for light theme, got %q", rec.Body.String())
	}
}

func TestHandleServiceWorker(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/sw.js", nil)
	rec := httptest.NewRecorder()
	s.handleServiceWorker(rec, req)

	if rec.Header().Get("Content-Type") != "application/javascript" {
		t.Errorf("expected javascript content type, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "CACHE_NAME") {
		t.Error("expected service worker script body")
	}
}

func TestGetServerURL(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"

	url := s.getServerURL(req)
	if url == "" {
		t.Error("expected non-empty server URL")
	}
}

func TestStartTimeIsSet(t *testing.T) {
	s := newTestServer(t)
	if time.Since(s.startTime) < 0 {
		t.Error("expected startTime to be in the past")
	}
}

func TestServerStart(t *testing.T) {
	svc, err := anime.NewService([]byte(testQuotes))
	if err != nil {
		t.Fatalf("anime.NewService returned error: %v", err)
	}
	cfg := config.DefaultConfig()
	s, err := NewServer(svc, cfg, "0", "127.0.0.1")
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	select {
	case err := <-errCh:
		t.Fatalf("Start returned early: %v", err)
	case <-time.After(200 * time.Millisecond):
		// Server is listening; leave it running for the test process lifetime.
	}
}

func TestGraphQLSearchAndOpenAPISpec(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/graphql?query=%7Bquotes%28q%3A%22believe%22%29%7Banime+quote%7D%7D", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Believe it!") {
		t.Fatalf("unexpected GraphQL response: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/server/swagger", nil)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"openapi":"3.1.0"`) {
		t.Fatalf("unexpected OpenAPI response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLocaleRoutesAndLanguagePreference(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/locales/ar.json", nil)
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "application/json") || !strings.Contains(response.Body.String(), `"direction": "rtl"`) {
		t.Fatalf("Arabic locale response status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/server/preferences?lang=fr", nil)
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `<html lang="fr" dir="ltr"`) {
		t.Fatalf("French preference page status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Préférences") || !strings.Contains(response.Body.String(), `<option value="fr" selected lang="fr">Français</option>`) {
		t.Fatalf("French language selection was not rendered: %s", response.Body.String())
	}
	cookie := response.Result().Cookies()
	if len(cookie) == 0 || cookie[0].Name != "lang" || cookie[0].Value != "fr" {
		t.Fatalf("explicit lang query did not persist preference: %v", cookie)
	}

	request = httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	request.Header.Set("Accept-Language", "fr-CA, en;q=0.8")
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `Automatique (Français)</option>`) || !strings.Contains(response.Body.String(), `<option value="automatic" selected>`) {
		t.Fatalf("automatic language selection status=%d body=%s", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatalf("automatic language detection wrote a cookie: %v", response.Result().Cookies())
	}

	s.cfg.Server.I18n.DefaultLanguage = "fr"
	request = httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `<html lang="fr" dir="ltr"`) {
		t.Fatalf("configured default language status=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/?lang=fr", nil)
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	for _, expected := range []string{"Aller au contenu principal", "Accueil", "Toutes les citations", "Informations juridiques et sur le projet", "Politique de confidentialité", "Accepter"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("French shared shell is missing %q", expected)
		}
	}
}

func TestSitemapAndLLMSDiscoveryUseRequestOriginAndRouteRegistry(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	request.Host = "example.com"
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{"<loc>http://example.com/</loc>", "http://example.com/server/help/mcp", "http://example.com/server/docs/swagger"} {
		if !strings.Contains(body, expected) {
			t.Errorf("sitemap missing %q: %s", expected, body)
		}
	}
	for _, forbidden := range []string{"/server/contact", "/server/preferences/tokens", "/api/v1/"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("sitemap contains forbidden route %q", forbidden)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/.well-known/llms.txt", nil)
	request.Host = "example.com"
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "MCP: http://example.com/mcp") || !strings.Contains(response.Body.String(), "GET /api/v1/quotes") || strings.Contains(response.Body.String(), "/server/metrics") {
		t.Fatalf("llms response status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSecurityTxtLegacyAliasIsNotServed(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/security.txt", nil)
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("legacy security.txt alias status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPublicFeedsRemainFixedXMLAndCanBeEmpty(t *testing.T) {
	s := newTestServer(t)
	for _, test := range []struct {
		path        string
		contentType string
		root        string
	}{
		{path: "/feed.xml", contentType: "application/atom+xml", root: "<feed"},
		{path: "/rss.xml", contentType: "application/rss+xml", root: "<rss"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set("Accept", "text/plain")
		response := httptest.NewRecorder()
		s.router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), test.contentType) || !strings.Contains(response.Body.String(), test.root) {
			t.Errorf("%s status=%d type=%q body=%s", test.path, response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
	}
}
