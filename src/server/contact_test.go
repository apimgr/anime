package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestContactChooserIsNoIndexAndContainsNoFormFields(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/contact", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || rec.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("contact chooser status=%d robots=%q", rec.Code, rec.Header().Get("X-Robots-Tag"))
	}
	if strings.Contains(body, `action="/server/contact"`) || strings.Contains(body, `name="name"`) || strings.Contains(body, `name="type"`) {
		t.Fatalf("contact chooser rendered report form fields: %s", body)
	}
	var challenges int
	if err := s.database.SQL.QueryRow(`SELECT COUNT(*) FROM captcha_challenges`).Scan(&challenges); err != nil {
		t.Fatal(err)
	}
	if challenges != 0 {
		t.Fatalf("contact chooser issued %d CAPTCHA challenges without rendering a form", challenges)
	}
	for _, required := range []string{"security policy", "Abuse reports", "Send a DMCA notice", `name="robots"`} {
		if !strings.Contains(body, required) {
			t.Errorf("contact chooser missing %q", required)
		}
	}
}

func TestSecurityReportStatusLinkIsSingleUsePerDayAndNoReferrer(t *testing.T) {
	s := newTestServer(t)
	token := "private-status-token"
	hash := sha256.Sum256([]byte(token))
	_, err := s.database.SQL.Exec(`INSERT INTO security_reports (tracking_id, security_id, severity, component, status, body_enc, body_alg, contact_enc, token_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sec_0123456789abcdef", "id", "high", "api", "received", []byte("encrypted"), "aes-256-gcm", []byte("encrypted"), hex.EncodeToString(hash[:]), time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	path := "/server/security/report/sec_0123456789abcdef?token=" + url.QueryEscape(token)
	first := httptest.NewRecorder()
	s.router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, path, nil))
	if first.Code != http.StatusOK || first.Header().Get("Referrer-Policy") != "no-referrer" || first.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status page status=%d referrer=%q cache=%q", first.Code, first.Header().Get("Referrer-Policy"), first.Header().Get("Cache-Control"))
	}
	second := httptest.NewRecorder()
	s.router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, path, nil))
	if second.Code != http.StatusNotFound {
		t.Fatalf("status token replay status=%d, want %d", second.Code, http.StatusNotFound)
	}
}

func TestSecurityReportMaintenanceTransitionsAndAcknowledgments(t *testing.T) {
	s := newTestServer(t)
	_, err := s.database.SQL.Exec(`INSERT INTO security_reports (tracking_id, security_id, severity, component, status, body_enc, body_alg, contact_enc, credit_pref, credit_name, token_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sec_abcdef0123456789", "id", "medium", "api", "received", []byte("encrypted"), "aes-256-gcm", []byte("encrypted"), "anon", nil, strings.Repeat("0", 64), time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := MaintainSecurityReports(context.Background(), s.database, s.cfg, []string{"update", "sec_abcdef0123456789", "--status", "disclosed", "--comment", "A fix is available"}, &output); err != nil {
		t.Fatal(err)
	}
	var status, closedAt string
	if err := s.database.SQL.QueryRow(`SELECT status, closed_at FROM security_reports WHERE tracking_id = ?`, "sec_abcdef0123456789").Scan(&status, &closedAt); err != nil {
		t.Fatal(err)
	}
	if status != "disclosed" || closedAt == "" {
		t.Fatalf("status=%q closed_at=%q", status, closedAt)
	}
	var thanks int
	if err := s.database.SQL.QueryRow(`SELECT COUNT(*) FROM security_thanks WHERE tracking_id = ?`, "sec_abcdef0123456789").Scan(&thanks); err != nil {
		t.Fatal(err)
	}
	if thanks != 1 {
		t.Fatalf("thanks row count=%d, want 1", thanks)
	}
	if err := MaintainSecurityReports(context.Background(), s.database, s.cfg, []string{"update", "sec_abcdef0123456789", "--status", "triaged"}, &output); err == nil {
		t.Fatal("backward status transition was accepted")
	}
}

func TestContactFormAndReviewTokensAreTimeGatedAndSingleUse(t *testing.T) {
	s := newTestServer(t)
	now := time.Now().UTC()
	formToken := s.newContactFormToken(now)
	if s.validContactFormToken(formToken, now.Add(time.Second)) {
		t.Fatal("contact form token was accepted before the minimum fill time")
	}
	if !s.validContactFormToken(formToken, now.Add(3*time.Second)) {
		t.Fatal("valid contact form token was rejected")
	}
	if s.validContactFormToken(formToken, now.Add(4*time.Second)) {
		t.Fatal("contact form token replay was accepted")
	}

	values := url.Values{"type": {"privacy"}, "email": {"person@example.test"}, "subject": {"Privacy request"}}
	reviewToken, _ := s.newContactReviewToken(values, false, now)
	if ok, _ := s.validContactReviewToken(reviewToken, values, true, now.Add(time.Second)); ok {
		t.Fatal("review token was accepted before the minimum review time")
	}
	if ok, _ := s.validContactReviewToken(reviewToken, values, true, now.Add(3*time.Second)); !ok {
		t.Fatal("valid review token was rejected")
	}
	if ok, _ := s.validContactReviewToken(reviewToken, values, true, now.Add(4*time.Second)); ok {
		t.Fatal("review token replay was accepted")
	}
}

func TestContactAntispamConfiguredBlocksDuplicatesAndBans(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Server.Pages.Contact.Antispam.Strikes = 3
	request := httptest.NewRequest(http.MethodPost, "/server/contact", nil)
	request.RemoteAddr = "203.0.113.9:1234"
	request.Host = "anime.example.test"
	values := url.Values{"type": {"abuse"}, "name": {"Sender"}, "email": {"person@example.test"}, "subject": {"Same"}, "message": {"Same body"}}
	if s.contactDuplicate(values, time.Now()) {
		t.Fatal("first contact submission was treated as a duplicate")
	}
	if !s.contactDuplicate(values, time.Now().Add(time.Second)) {
		t.Fatal("repeated contact submission was not detected")
	}
	for range 5 {
		s.countContactAccepted(request, "", time.Now())
	}
	if retry := s.contactRateLimit(request, "", time.Now()); retry <= 0 {
		t.Fatal("sixth submission from one IP was not rate limited")
	}
	if s.recordContactStrike(request, "duplicate", time.Now()) || s.recordContactStrike(request, "origin", time.Now()) || !s.recordContactStrike(request, "captcha", time.Now()) {
		t.Fatal("strike threshold did not create a ban on the third strike")
	}
	if retry := s.contactRateLimit(request, "", time.Now()); retry <= 0 {
		t.Fatal("banned IP was not rate limited")
	}
}

func TestContactAntispamRejectsConfiguredMarkupAndEmailDomain(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Server.Pages.Contact.Antispam.RejectMarkup = true
	s.cfg.Server.Pages.Contact.Antispam.BlockedEmailDomains = []string{"blocked.example"}
	values := url.Values{"type": {"privacy"}, "name": {"Sender"}, "email": {"sender@blocked.example"}, "request_kind": {"access"}, "subject": {"hello"}, "message": {"<a href=\"https://anime.example.test\">link</a>"}}
	validation := validateContactValues(values, "privacy", "anime.example.test", s.cfg)
	if validation["message"] != "invalid content" || validation["email"] != "invalid content" {
		t.Fatalf("configured policy validation errors=%v", validation)
	}
}

func TestContactValidationRejectsHeaderInjectionAndInvalidReportedHost(t *testing.T) {
	values := url.Values{"name": {"Visitor"}, "email": {"visitor@example.test"}, "subject": {"Hello\r\nBcc: attacker@example.test"}, "message": {"Please review"}, "type": {"abuse"}, "reported_url": {"https://attacker.example/item"}, "reason": {"terms_violation"}}
	errors := validateContactValues(values, "abuse", "example.test")
	if errors["subject"] == "" || errors["reported_url"] == "" {
		t.Fatalf("invalid contact fields were accepted: %#v", errors)
	}
}

func TestContactTypePagesRenderOnlyTheirFields(t *testing.T) {
	s := newTestServer(t)
	cases := []struct {
		kind     string
		contains []string
		omits    []string
	}{
		{kind: "abuse", contains: []string{`name="reported_url"`, `name="reason"`}, omits: []string{`name="copyrighted_work"`, `name="request_kind"`}},
		{kind: "dmca", contains: []string{`name="reported_url"`, `name="copyrighted_work"`, `name="good_faith"`, `name="accuracy"`, `name="signature"`}, omits: []string{`name="request_kind"`}},
		{kind: "privacy", contains: []string{`name="request_kind"`}, omits: []string{`name="reported_url"`, `name="copyrighted_work"`}},
	}
	for _, test := range cases {
		t.Run(test.kind, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/server/contact?type="+test.kind, nil)
			rec := httptest.NewRecorder()
			s.router.ServeHTTP(rec, req)
			body := rec.Body.String()
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, body)
			}
			for _, name := range test.contains {
				if !strings.Contains(body, name) {
					t.Errorf("missing %s", name)
				}
			}
			for _, name := range test.omits {
				if strings.Contains(body, name) {
					t.Errorf("unexpected field %s", name)
				}
			}
		})
	}
}

func TestContactAPIFormDescriptor(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/contact?type=privacy", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, expected := range []string{`"form_token"`, `"captcha_id"`, `"request_kind"`, `"required_when"`, `"security_url"`, `"types":["abuse","dmca","privacy"]`} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Errorf("response missing %s: %s", expected, rec.Body.String())
		}
	}
}
