package server

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxPreferenceSyncParameters = 32

func (s *Server) handlePreferencesSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost {
		s.importPreferencesForm(w, r)
		return
	}
	values := s.currentPreferenceValues(r)
	query := canonicalPreferenceQuery(values)
	code := base64.RawURLEncoding.EncodeToString([]byte(query))
	origin := requestOrigin(r)
	fullURL := origin + "/server/preferences/sync/import?" + query
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Add("Vary", "Accept")
		respondJSON(w, http.StatusOK, map[string]interface{}{"theme": values.Get("theme"), "lang": values.Get("lang"), "prefs": preferenceMap(values), "url": fullURL, "code": code})
		return
	}
	if prefersPlainText(r.Header.Get("Accept")) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Add("Vary", "Accept")
		_, _ = fmt.Fprintf(w, "URL: %s\nCode: %s\n", fullURL, code)
		return
	}
	data := map[string]interface{}{
		"Title": "Sync preferences", "Description": "Copy these browser preferences to another device.",
		"ContentTemplate": "preferences-sync-content", "Page": "preferences-sync", "Version": Version,
		"Theme": resolvedTheme(r, s.cfg.WebUI.Theme), "CSRFToken": s.csrfToken(time.Now()),
		"CurrentPath": r.URL.RequestURI(), "PreferenceURL": fullURL,
		"PreferenceCode": code,
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering preference sync page: %v", err)
	}
}

func (s *Server) importPreferencesForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !sameOriginPreferenceRequest(r) || !s.validCSRFToken(r.PostFormValue("csrf_token"), time.Now()) {
		s.renderStatusError(w, r, http.StatusForbidden)
		return
	}
	direct := valuesFromURLValues(r.PostForm)
	delete(direct, "csrf_token")
	delete(direct, "code")
	delete(direct, "redirect")
	values, _, _ := resolvePreferenceInput(direct, r.PostFormValue("code"))
	applyPreferenceCookies(w, r, values)
	target, ok := validatedPreferencesRedirect(r.PostFormValue("redirect"))
	if !ok {
		target, ok = preferencesRefererRedirect(r)
	}
	if !ok {
		target = "/server/preferences/sync"
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) importPreferences(w http.ResponseWriter, r *http.Request) {
	values, _, _ := resolvePreferenceInput(r.URL.Query(), r.URL.Query().Get("code"))
	applyPreferenceCookies(w, r, values)
	target, ok := validatedPreferencesRedirect(r.URL.Query().Get("redirect"))
	if !ok {
		target, ok = preferencesRefererRedirect(r)
	}
	if !ok {
		target = "/"
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) handlePreferencesSyncAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	values := s.currentPreferenceValues(r)
	query := canonicalPreferenceQuery(values)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"theme": values.Get("theme"), "lang": values.Get("lang"), "prefs": preferenceMap(values),
		"url":  requestOrigin(r) + "/server/preferences/sync/import?" + query,
		"code": base64.RawURLEncoding.EncodeToString([]byte(query)),
	})
}

func (s *Server) importPreferencesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	values, applied, ignored := resolvePreferenceInput(r.URL.Query(), r.URL.Query().Get("code"))
	applyPreferenceCookies(w, r, values)
	effective := s.currentPreferenceValues(r)
	for key, entries := range values {
		effective[key] = append([]string(nil), entries...)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"theme": effective.Get("theme"), "lang": effective.Get("lang"), "prefs": preferenceMap(effective),
		"applied": applied, "ignored": ignored,
	})
}

func preferenceMap(values url.Values) map[string]string {
	prefs := make(map[string]string)
	for name, entries := range values {
		if len(entries) > 0 && validAppPreference(name, entries[len(entries)-1]) {
			prefs[strings.TrimPrefix(name, "anime_pref_")] = entries[len(entries)-1]
		}
	}
	return prefs
}

func (s *Server) currentPreferenceValues(r *http.Request) url.Values {
	values := url.Values{"theme": {resolvedTheme(r, s.cfg.WebUI.Theme)}, "lang": {s.resolvedLanguage(r)}}
	for _, cookie := range r.Cookies() {
		if validAppPreference(cookie.Name, cookie.Value) {
			values.Set(cookie.Name, cookie.Value)
		}
	}
	return values
}

func canonicalPreferenceQuery(values url.Values) string {
	canonical := url.Values{"theme": {values.Get("theme")}, "lang": {values.Get("lang")}}
	for key := range values {
		if validAppPreference(key, values.Get(key)) {
			canonical.Set(key, values.Get(key))
		}
	}
	return canonical.Encode()
}

func resolvePreferenceInput(direct url.Values, code string) (url.Values, []string, []string) {
	combined := decodePreferenceCode(code)
	for key, entries := range direct {
		if preferenceControlField(key) {
			continue
		}
		combined[key] = append([]string(nil), entries...)
	}
	resolved := make(url.Values)
	applied := make([]string, 0, 2)
	ignored := make([]string, 0)
	keys := make([]string, 0, len(combined))
	for key := range combined {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > maxPreferenceSyncParameters {
		keys = keys[:maxPreferenceSyncParameters]
	}
	for _, key := range keys {
		entries := combined[key]
		if len(entries) == 0 || len(entries[len(entries)-1]) > 64 {
			ignored = append(ignored, key)
			continue
		}
		value := entries[len(entries)-1]
		switch key {
		case "theme":
			if value == "dark" || value == "light" || value == "auto" {
				resolved.Set(key, value)
				applied = append(applied, key)
			} else {
				ignored = append(ignored, key)
			}
		case "lang":
			if language, ok := canonicalLanguage(value); ok {
				resolved.Set(key, language)
				applied = append(applied, key)
			} else {
				ignored = append(ignored, key)
			}
		default:
			if validAppPreference(key, value) {
				resolved.Set(key, value)
				applied = append(applied, key)
			} else if !preferenceControlField(key) {
				ignored = append(ignored, key)
			}
		}
	}
	return resolved, applied, ignored
}

func decodePreferenceCode(code string) url.Values {
	code = strings.TrimSpace(code)
	if len(code) > 2048 {
		return make(url.Values)
	}
	if index := strings.IndexByte(code, '?'); index >= 0 {
		code = code[index+1:]
	}
	if strings.Contains(code, "=") {
		values, err := url.ParseQuery(code)
		if err != nil {
			return make(url.Values)
		}
		return values
	}
	decoded, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(code)
	}
	if err != nil {
		return make(url.Values)
	}
	values, err := url.ParseQuery(string(decoded))
	if err != nil {
		return make(url.Values)
	}
	return values
}

func valuesFromURLValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, items := range values {
		clone[key] = append([]string(nil), items...)
	}
	return clone
}

func applyPreferenceCookies(w http.ResponseWriter, r *http.Request, values url.Values) {
	for name, entries := range values {
		if len(entries) > 0 && (name == "theme" || name == "lang" || validAppPreference(name, entries[len(entries)-1])) {
			http.SetCookie(w, preferenceCookie(r, name, entries[len(entries)-1]))
		}
	}
}

func requestOrigin(r *http.Request) string {
	host := resolvedRequestHost(r)
	if torHost := strings.TrimSpace(os.Getenv("TOR_HOSTNAME")); torHost != "" && strings.EqualFold(host, torHost) {
		return "http://" + host
	}
	scheme := resolvedRequestScheme(r)
	if parsedHost, port, err := net.SplitHostPort(host); err == nil {
		defaultPort := (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
		if defaultPort {
			host = parsedHost
		}
	}
	return scheme + "://" + host
}

func resolvedRequestHost(r *http.Request) string {
	domains := configuredDomains()
	if trustedProxyRequest(r) {
		for _, header := range []string{"X-Forwarded-Host", "X-Real-Host", "X-Original-Host"} {
			candidate := strings.TrimSpace(strings.Split(r.Header.Get(header), ",")[0])
			if validRequestHost(candidate) && (len(domains) == 0 || hostMatchesDomainList(candidate, domains)) {
				return candidate
			}
		}
	}
	if validRequestHost(r.Host) && (len(domains) == 0 || hostMatchesDomainList(r.Host, domains)) {
		return r.Host
	}
	if len(domains) > 0 {
		return strings.TrimPrefix(domains[0], "*.")
	}
	if hostname, err := os.Hostname(); err == nil && validRequestHost(hostname) {
		return hostname
	}
	return "localhost"
}

func trustedProxyRequest(r *http.Request) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	ip := net.ParseIP(peer)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func configuredDomains() []string {
	var domains []string
	for _, raw := range strings.Split(os.Getenv("DOMAIN"), ",") {
		candidate := strings.TrimSpace(raw)
		if strings.HasPrefix(candidate, "*.") && validRequestHost(strings.TrimPrefix(candidate, "*.")) || validRequestHost(candidate) {
			domains = append(domains, candidate)
		}
	}
	return domains
}

func hostMatchesDomainList(host string, domains []string) bool {
	name := strings.ToLower(host)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		name = strings.ToLower(parsed)
	}
	for _, domain := range domains {
		allowed := strings.ToLower(domain)
		if parsed, _, err := net.SplitHostPort(domain); err == nil {
			allowed = strings.ToLower(parsed)
		}
		if name == allowed || strings.HasPrefix(allowed, "*.") && strings.HasSuffix(name, allowed[1:]) {
			return true
		}
	}
	return false
}

func validRequestHost(host string) bool {
	if host == "" || strings.ContainsAny(host, " \t\r\n/@\\") {
		return false
	}
	name := host
	if parsed, port, err := net.SplitHostPort(host); err == nil {
		name = parsed
		if number, err := strconv.Atoi(port); err != nil || number < 1 || number > 65535 {
			return false
		}
	} else if strings.Count(host, ":") > 1 {
		return net.ParseIP(strings.Trim(host, "[]")) != nil
	}
	if net.ParseIP(strings.Trim(name, "[]")) != nil {
		return true
	}
	if len(name) > 253 || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}
