package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/apimgr/anime/src/common/i18n"
)

const csrfTokenLifetime = 2 * time.Hour

var supportedAppPreferenceKeys = map[string]map[string]struct{}{}

func (s *Server) csrfToken(now time.Time) string {
	timestamp := now.UTC().Unix()
	message := "preferences:" + time.Unix(timestamp, 0).UTC().Format(time.RFC3339)
	mac := hmac.New(sha256.New, s.csrfKey)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString([]byte(message)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) validCSRFToken(token string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	message, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	parsed, err := time.Parse("preferences:2006-01-02T15:04:05Z", string(message))
	if err != nil || now.Before(parsed) || now.Sub(parsed) > csrfTokenLifetime {
		return false
	}
	mac := hmac.New(sha256.New, s.csrfKey)
	_, _ = mac.Write(message)
	return hmac.Equal(signature, mac.Sum(nil))
}

func nextTheme(current string) string {
	switch current {
	case "dark":
		return "light"
	case "light":
		return "auto"
	default:
		return "dark"
	}
}

func (s *Server) handlePreferences(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost {
		s.savePreferences(w, r)
		return
	}
	theme := resolvedTheme(r, s.cfg.WebUI.Theme)
	consent := preferenceConsent(r)
	ccpaOptedOut := false
	if cookie, err := r.Cookie("ccpa_opt_out"); err == nil && cookie.Value == "true" {
		ccpaOptedOut = true
	}
	languages := make([]map[string]string, 0)
	for _, code := range s.cfg.Server.I18n.AvailableLanguages {
		languages = append(languages, map[string]string{"Code": code, "Name": i18n.NativeName(code)})
	}
	languageSelection := "automatic"
	if requested := r.URL.Query().Get("lang"); requested != "" {
		if language, ok := canonicalLanguage(requested); ok {
			languageSelection = language
		}
	} else if cookie, err := r.Cookie("lang"); err == nil {
		if language, ok := canonicalLanguage(cookie.Value); ok {
			languageSelection = language
		}
	}
	data := map[string]interface{}{
		"Title": "Preferences", "Description": "Choose the appearance used by this browser.",
		"ContentTemplate": "preferences-content", "Page": "preferences", "Theme": theme,
		"CurrentPath": r.URL.RequestURI(), "NextTheme": nextTheme(theme), "CSRFToken": s.csrfToken(time.Now()),
		"Version":            Version,
		"Pages":              registeredServerPages("preferences", s.resolvedLanguage(r)),
		"ConsentPreferences": consent.Preferences,
		"ConsentAnalytics":   consent.Analytics,
		"ConsentSet":         consent.Timestamp != 0,
		"DataSold":           s.cfg.Server.Privacy.Data.Sold,
		"CCPAOptedOut":       ccpaOptedOut,
		"AutomaticLanguage":  i18n.NativeName(s.resolvedLanguage(r)),
		"LanguageSelection":  languageSelection,
		"Languages":          languages,
		"CancelURL":          preferencesCancelTarget(r),
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering preferences page: %v", err)
	}
}

func preferencesCancelTarget(r *http.Request) string {
	if target, ok := preferencesRefererRedirect(r); ok {
		return target
	}
	return "/"
}

func (s *Server) savePreferences(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !sameOriginPreferenceRequest(r) || !s.validCSRFToken(r.PostFormValue("csrf_token"), time.Now()) {
		s.renderStatusError(w, r, http.StatusForbidden)
		return
	}
	if r.PostFormValue("action") == "reset" {
		clearPreferenceCookies(w, r)
		w.Header().Set("Clear-Site-Data", `"storage"`)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if theme := r.PostFormValue("theme"); theme == "dark" || theme == "light" || theme == "auto" {
		http.SetCookie(w, preferenceCookie(r, "theme", theme))
	}
	if _, submitted := r.PostForm["lang"]; submitted {
		if lang := r.PostFormValue("lang"); lang == "automatic" || lang == "" {
			http.SetCookie(w, expiredPreferenceCookie(r, "lang"))
		} else if language, ok := canonicalLanguage(lang); ok {
			http.SetCookie(w, preferenceCookie(r, "lang", language))
		}
	}
	if r.PostFormValue("consent_form") == "1" {
		consent := preferenceConsentState{
			Essential: true, Preferences: r.PostFormValue("consent_preferences") == "on",
			Analytics: r.PostFormValue("consent_analytics") == "on", Timestamp: time.Now().Unix(),
		}
		if payload, err := json.Marshal(consent); err == nil {
			http.SetCookie(w, preferenceCookie(r, "cookie_consent", url.QueryEscape(string(payload))))
		}
		if s.cfg.Server.Privacy.Data.Sold {
			if r.PostFormValue("ccpa_opt_out") == "on" {
				http.SetCookie(w, preferenceCookie(r, "ccpa_opt_out", "true"))
			} else {
				http.SetCookie(w, expiredPreferenceCookie(r, "ccpa_opt_out"))
			}
		}
	}
	target, ok := validatedPreferencesRedirect(r.PostFormValue("redirect"))
	if !ok {
		target, ok = preferencesRefererRedirect(r)
	}
	if !ok {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func sameOriginPreferenceRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Referer()
	}
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	expected, expectedErr := url.Parse(requestOrigin(r))
	if err != nil || expectedErr != nil || parsed.Host == "" || !strings.EqualFold(parsed.Host, expected.Host) || parsed.User != nil || parsed.Scheme != expected.Scheme {
		return false
	}
	return true
}

func (s *Server) handlePreferencesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		respondJSON(w, http.StatusOK, map[string]interface{}{"theme": resolvedTheme(r, s.cfg.WebUI.Theme), "lang": s.resolvedLanguage(r), "prefs": appPreferences(r)})
		return
	}
	values, err := preferenceValues(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "a valid preference object is required")
		return
	}
	applied := make([]string, 0, 2)
	ignored := make([]string, 0, len(values))
	effectivePrefs := appPreferences(r)
	effectiveTheme := resolvedTheme(r, s.cfg.WebUI.Theme)
	effectiveLanguage := s.resolvedLanguage(r)
	for name, value := range values {
		if preferenceControlField(name) {
			continue
		}
		switch name {
		case "theme":
			if value == "dark" || value == "light" || value == "auto" {
				http.SetCookie(w, preferenceCookie(r, "theme", value))
				applied = append(applied, name)
				effectiveTheme = value
			} else {
				ignored = append(ignored, name)
			}
		case "lang":
			if language, ok := canonicalLanguage(value); ok {
				http.SetCookie(w, preferenceCookie(r, "lang", language))
				applied = append(applied, name)
				effectiveLanguage = language
			} else {
				ignored = append(ignored, name)
			}
		default:
			if validAppPreference(name, value) {
				http.SetCookie(w, preferenceCookie(r, name, value))
				applied = append(applied, name)
				effectivePrefs[strings.TrimPrefix(name, "anime_pref_")] = value
			} else {
				ignored = append(ignored, name)
			}
		}
	}
	sort.Strings(applied)
	sort.Strings(ignored)
	if len(applied) == 0 {
		writeProblem(w, http.StatusBadRequest, "no valid preferences were supplied")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"theme": effectiveTheme, "lang": effectiveLanguage, "prefs": effectivePrefs, "applied": applied, "ignored": ignored})
}

func appPreferences(r *http.Request) map[string]string {
	prefs := make(map[string]string)
	for _, cookie := range r.Cookies() {
		if validAppPreference(cookie.Name, cookie.Value) {
			prefs[strings.TrimPrefix(cookie.Name, "anime_pref_")] = cookie.Value
		}
	}
	return prefs
}

func preferenceCookie(r *http.Request, name, value string) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: 31536000, Expires: time.Now().AddDate(1, 0, 0), SameSite: http.SameSiteLaxMode, Secure: resolvedRequestScheme(r) == "https"}
}

func resolvedRequestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	ip := net.ParseIP(peer)
	if ip == nil || (!ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast()) {
		return "http"
	}
	for _, value := range []string{r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Ssl"), r.Header.Get("X-Url-Scheme")} {
		switch strings.ToLower(strings.TrimSpace(strings.Split(value, ",")[0])) {
		case "https", "on":
			return "https"
		case "http", "off":
			return "http"
		}
	}
	return "http"
}

func preferenceValues(r *http.Request) (map[string]string, error) {
	values := make(map[string]string)
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		var raw map[string]json.RawMessage
		if err := decoder.Decode(&raw); err != nil || raw == nil {
			return nil, errInvalidPreferenceBody
		}
		var trailing interface{}
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, errInvalidPreferenceBody
		}
		for name, value := range raw {
			var text string
			if err := json.Unmarshal(value, &text); err != nil {
				values[name] = ""
				continue
			}
			values[name] = text
		}
		return values, nil
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	for name, items := range r.PostForm {
		if len(items) > 0 {
			values[name] = items[len(items)-1]
		}
	}
	return values, nil
}

var errInvalidPreferenceBody = errors.New("invalid preference body")

func (s *Server) resolvedLanguage(r *http.Request) string {
	return i18n.LangFromRequestWithDefault(r, s.cfg.Server.I18n.DefaultLanguage)
}

func canonicalLanguage(value string) (string, bool) {
	return i18n.CanonicalLanguage(value)
}

func preferenceControlField(name string) bool {
	switch name {
	case "csrf_token", "code", "redirect", "action", "consent_form", "consent_preferences", "consent_analytics", "ccpa_opt_out":
		return true
	default:
		return false
	}
}

func validAppPreference(name, value string) bool {
	if !strings.HasPrefix(name, "anime_pref_") || len(name) > len("anime_pref_")+64 || len(value) == 0 || len(value) > 64 {
		return false
	}
	key := strings.TrimPrefix(name, "anime_pref_")
	allowedValues, registered := supportedAppPreferenceKeys[key]
	if !registered {
		return false
	}
	if len(allowedValues) > 0 {
		_, ok := allowedValues[value]
		return ok
	}
	if key == "" || key[0] < 'a' || key[0] > 'z' {
		return false
	}
	for _, char := range key[1:] {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	for _, char := range value {
		if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') && !strings.ContainsRune("_.-", char) {
			return false
		}
	}
	return true
}

type preferenceConsentState struct {
	Essential   bool  `json:"essential"`
	Preferences bool  `json:"preferences"`
	Analytics   bool  `json:"analytics"`
	Timestamp   int64 `json:"timestamp"`
}

func preferenceConsent(r *http.Request) preferenceConsentState {
	cookie, err := r.Cookie("cookie_consent")
	if err != nil || len(cookie.Value) > 512 {
		return preferenceConsentState{Essential: true}
	}
	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return preferenceConsentState{Essential: true}
	}
	var consent preferenceConsentState
	if json.Unmarshal([]byte(value), &consent) != nil {
		return preferenceConsentState{Essential: true}
	}
	consent.Essential = true
	return consent
}

func expiredPreferenceCookie(r *http.Request, name string) *http.Cookie {
	cookie := preferenceCookie(r, name, "")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	return cookie
}

func clearPreferenceCookies(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{"theme", "lang", "cookie_consent", "ccpa_opt_out", "anime_build"} {
		http.SetCookie(w, expiredPreferenceCookie(r, name))
	}
	for _, cookie := range r.Cookies() {
		if strings.HasPrefix(cookie.Name, "anime_pref_") {
			http.SetCookie(w, expiredPreferenceCookie(r, cookie.Name))
		}
	}
}

func safePreferencesRedirect(target string) string {
	validated, ok := validatedPreferencesRedirect(target)
	if !ok {
		return "/"
	}
	return validated
}

func validatedPreferencesRedirect(target string) (string, bool) {
	if target == "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") || strings.HasPrefix(target, "/\\") || strings.ContainsAny(target, "\r\n") {
		return "", false
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || isPreferencesActionPath(parsed.Path) {
		return "", false
	}
	return parsed.RequestURI(), true
}

func preferencesRefererRedirect(r *http.Request) (string, bool) {
	referer, err := url.Parse(r.Referer())
	expected, expectedErr := url.Parse(requestOrigin(r))
	if err != nil || expectedErr != nil || referer.Host == "" || !strings.EqualFold(referer.Host, expected.Host) || referer.User != nil || referer.Scheme != expected.Scheme {
		return "", false
	}
	return validatedPreferencesRedirect(referer.RequestURI())
}

func isPreferencesActionPath(path string) bool {
	for _, action := range []string{"/server/preferences", "/server/preferences/sync", "/server/preferences/sync/import"} {
		if path == action || strings.HasPrefix(path, action+"/") {
			return true
		}
	}
	return false
}
