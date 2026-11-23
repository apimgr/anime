package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) handleConsent(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil || !sameOriginPreferenceRequest(r) || !s.validCSRFToken(r.PostFormValue("csrf_token"), time.Now()) {
		s.renderStatusError(w, r, http.StatusForbidden)
		return
	}
	consent := preferenceConsentState{Essential: true, Timestamp: time.Now().Unix()}
	switch r.PostFormValue("choice") {
	case "accept":
		consent.Preferences = true
		consent.Analytics = true
	case "save":
		consent.Preferences = r.PostFormValue("consent_preferences") == "on"
		consent.Analytics = r.PostFormValue("consent_analytics") == "on"
	case "decline":
	default:
		consent.Timestamp = 0
	}
	if consent.Timestamp != 0 {
		if value, err := json.Marshal(consent); err == nil {
			http.SetCookie(w, preferenceCookie(r, "cookie_consent", url.QueryEscape(string(value))))
		}
	}
	target, ok := validatedConsentRedirect(r.PostFormValue("redirect"))
	if !ok {
		target, ok = consentRefererRedirect(r)
	}
	if !ok {
		target = "/"
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func validatedConsentRedirect(target string) (string, bool) {
	if target == "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") || strings.HasPrefix(target, "/\\") || strings.ContainsAny(target, "\r\n") {
		return "", false
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Path == "/server/consent" {
		return "", false
	}
	return parsed.RequestURI(), true
}

func consentRefererRedirect(r *http.Request) (string, bool) {
	referer, err := url.Parse(r.Referer())
	expected, expectedErr := url.Parse(requestOrigin(r))
	if err != nil || expectedErr != nil || referer.Host == "" || !strings.EqualFold(referer.Host, expected.Host) || referer.Scheme != expected.Scheme || referer.User != nil {
		return "", false
	}
	return validatedConsentRedirect(referer.RequestURI())
}
