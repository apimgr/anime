package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/apimgr/anime/src/config"
	"github.com/gorilla/mux"
)

type contactField struct {
	Name          string                 `json:"name"`
	Label         string                 `json:"label"`
	Type          string                 `json:"type"`
	Required      bool                   `json:"required"`
	MaxLength     *int                   `json:"max_length"`
	RequiredWhen  *contactFieldCondition `json:"required_when,omitempty"`
	MaxLengthWhen []contactLengthRule    `json:"max_length_when,omitempty"`
	Options       []string               `json:"options,omitempty"`
}

type contactFieldCondition struct {
	Field  string   `json:"field"`
	Equals string   `json:"equals,omitempty"`
	In     []string `json:"in,omitempty"`
}

type contactLengthRule struct {
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"max_length"`
}

type contactAntispamState struct {
	mu         sync.Mutex
	counts     map[string][]time.Time
	strikes    map[string][]time.Time
	bans       map[string]time.Time
	duplicates map[string]time.Time
}

var contactAntispamStates sync.Map

func (s *Server) antispamState() *contactAntispamState {
	state, _ := contactAntispamStates.LoadOrStore(s, &contactAntispamState{
		counts: make(map[string][]time.Time), strikes: make(map[string][]time.Time),
		bans: make(map[string]time.Time), duplicates: make(map[string]time.Time),
	})
	return state.(*contactAntispamState)
}

var contactTypeLabels = map[string]string{
	"abuse":    "Report abuse",
	"dmca":     "Send a DMCA notice",
	"privacy":  "Make a privacy request",
	"security": "Report a security vulnerability",
}

func contactTextField(name, label string, max int, required bool) contactField {
	return contactField{Name: name, Label: label, Type: "text", Required: required, MaxLength: &max}
}

func contactFields(kind string) []contactField {
	fields := []contactField{
		contactTextField("name", "Name", 100, true),
		contactField{Name: "email", Label: "Email", Type: "email", Required: true, MaxLength: intPointer(254)},
	}
	switch kind {
	case "abuse":
		fields = append(fields,
			contactField{Name: "reported_url", Label: "Reported URL", Type: "url", Required: true, MaxLength: intPointer(2048)},
			contactField{Name: "reason", Label: "Reason", Type: "select", Required: true, Options: []string{"illegal_content", "terms_violation", "harassment", "malware_phishing", "other"}},
			contactTextField("subject", "Subject", 200, true), contactField{Name: "message", Label: "Message", Type: "textarea", Required: true, MaxLength: intPointer(5000)})
	case "dmca":
		fields = append(fields,
			contactField{Name: "reported_url", Label: "Reported URL", Type: "url", Required: true, MaxLength: intPointer(2048)},
			contactField{Name: "copyrighted_work", Label: "Copyrighted work", Type: "textarea", Required: true, MaxLength: intPointer(2000)},
			contactField{Name: "good_faith", Label: "Good-faith statement", Type: "checkbox", Required: true},
			contactField{Name: "accuracy", Label: "Accuracy statement", Type: "checkbox", Required: true},
			contactTextField("signature", "Signature", 100, true),
			contactTextField("subject", "Subject", 200, true), contactField{Name: "message", Label: "Message", Type: "textarea", Required: true, MaxLength: intPointer(5000)})
	case "privacy":
		fields = append(fields,
			contactField{Name: "request_kind", Label: "Request kind", Type: "select", Required: true, Options: []string{"access", "export", "deletion", "correction", "opt_out", "other"}},
			contactTextField("subject", "Subject", 200, true), contactField{Name: "message", Label: "Message", Type: "textarea", Required: true, MaxLength: intPointer(5000)})
	case "security":
		fields = append(fields,
			contactTextField("researcher_gpg", "Researcher GPG", 16384, false),
			contactField{Name: "component", Label: "Affected component", Type: "text", Required: true, MaxLength: intPointer(255), Options: []string{"api", "frontend", "cli", "other"}},
			contactTextField("affected_url", "Affected endpoint / URL", 2048, false),
			contactField{Name: "severity", Label: "Severity (researcher's self-assessment)", Type: "select", Required: true, Options: []string{"critical", "high", "medium", "low", "informational"}},
			contactTextField("summary", "Vulnerability summary", 200, true),
			contactField{Name: "steps", Label: "Steps to reproduce", Type: "textarea", Required: true, MaxLength: intPointer(10000)},
			contactField{Name: "impact", Label: "Impact", Type: "textarea", Required: true, MaxLength: intPointer(5000)},
			contactField{Name: "suggested_fix", Label: "Suggested fix", Type: "textarea", Required: false, MaxLength: intPointer(5000)},
			contactField{Name: "cve_requested", Label: "CVE requested?", Type: "checkbox"},
			contactField{Name: "disclosure_days", Label: "Disclosure timeline preference", Type: "number"},
			contactField{Name: "credit_preference", Label: "Credit preference", Type: "select", Required: true, Options: []string{"name", "handle", "no", "anonymous"}},
			contactField{Name: "credit_handle", Label: "Credit handle", Type: "text", MaxLength: intPointer(100), RequiredWhen: &contactFieldCondition{Field: "credit_preference", Equals: "handle"}},
			contactField{Name: "disclosure_agreement", Label: "Coordinated disclosure agreement", Type: "checkbox", Required: true})
	}
	return fields
}

func contactAPIFields(securityMode bool) []contactField {
	fields := []contactField{
		contactTextField("name", "Name", 100, true),
		contactField{Name: "email", Label: "Email", Type: "email", Required: true, MaxLength: intPointer(254)},
		contactField{Name: "type", Label: "Type", Type: "select", Required: true, Options: []string{"abuse", "dmca", "privacy"}},
		contactTextField("subject", "Subject", 200, true),
		contactField{Name: "message", Label: "Message", Type: "textarea", Required: true, MaxLength: intPointer(5000)},
		contactField{Name: "reported_url", Label: "Reported URL", Type: "url", MaxLength: intPointer(2048), RequiredWhen: &contactFieldCondition{Field: "type", In: []string{"abuse", "dmca"}}},
		contactField{Name: "reason", Label: "Reason", Type: "select", Options: []string{"illegal_content", "terms_violation", "harassment", "malware_phishing", "other"}, RequiredWhen: &contactFieldCondition{Field: "type", Equals: "abuse"}},
		contactField{Name: "copyrighted_work", Label: "Copyrighted work", Type: "textarea", MaxLength: intPointer(2000), RequiredWhen: &contactFieldCondition{Field: "type", Equals: "dmca"}},
		contactField{Name: "good_faith", Label: "Good-faith statement", Type: "checkbox", RequiredWhen: &contactFieldCondition{Field: "type", Equals: "dmca"}},
		contactField{Name: "accuracy", Label: "Accuracy statement", Type: "checkbox", RequiredWhen: &contactFieldCondition{Field: "type", Equals: "dmca"}},
		contactField{Name: "signature", Label: "Signature", Type: "text", MaxLength: intPointer(100), RequiredWhen: &contactFieldCondition{Field: "type", Equals: "dmca"}},
		contactField{Name: "request_kind", Label: "Request kind", Type: "select", Options: []string{"access", "export", "deletion", "correction", "opt_out", "other"}, RequiredWhen: &contactFieldCondition{Field: "type", Equals: "privacy"}},
	}
	if securityMode {
		fields[2].Options = append(fields[2].Options, "security")
		for _, field := range contactFields("security")[2:] {
			if field.Required {
				field.Required = false
				field.RequiredWhen = &contactFieldCondition{Field: "type", Equals: "security"}
			}
			fields = append(fields, field)
		}
		for index := range fields {
			if fields[index].Name == "researcher_gpg" {
				fields[index].MaxLengthWhen = []contactLengthRule{{Prefix: "https://", MaxLength: 2048}}
			}
		}
	}
	return fields
}

func intPointer(value int) *int { return &value }

func (s *Server) newContactFormToken(now time.Time) string {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	issued := fmt.Sprintf("%d", now.Unix())
	encodedNonce := hex.EncodeToString(nonce)
	message := "contact-form|" + issued + "|" + encodedNonce
	mac := hmac.New(sha256.New, s.installationSecret)
	_, _ = mac.Write([]byte(message))
	return issued + "." + encodedNonce + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) consumeContactNonce(nonce string, expires time.Time, now time.Time) bool {
	s.contactMu.Lock()
	defer s.contactMu.Unlock()
	for key, deadline := range s.contactNonces {
		if !now.Before(deadline) {
			delete(s.contactNonces, key)
		}
	}
	if _, exists := s.contactNonces[nonce]; exists || len(s.contactNonces) >= 50000 {
		return false
	}
	s.contactNonces[nonce] = expires
	return true
}

func (s *Server) contactNonceConsumed(token string, noncePart int, partCount int) bool {
	parts := strings.Split(token, ".")
	if len(parts) != partCount || noncePart >= len(parts) {
		return false
	}
	s.contactMu.Lock()
	defer s.contactMu.Unlock()
	_, exists := s.contactNonces[parts[noncePart]]
	return exists
}

func (s *Server) validContactFormToken(token string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(parts[1]) != 32 {
		return false
	}
	issuedAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	issued := time.Unix(issuedAt, 0)
	age := now.Sub(issued)
	policy := s.cfg.Server.Pages.Contact.Antispam
	if policy.MinFillSeconds > 0 && age < time.Duration(policy.MinFillSeconds)*time.Second || policy.MaxAgeSeconds > 0 && age > time.Duration(policy.MaxAgeSeconds)*time.Second {
		return false
	}
	mac := hmac.New(sha256.New, s.installationSecret)
	_, _ = mac.Write([]byte("contact-form|" + parts[0] + "|" + parts[1]))
	signature, err := hex.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	return s.consumeContactNonce(parts[1], issued.Add(24*time.Hour), now)
}

func contactFieldsHash(values url.Values) string {
	canonical := make(url.Values)
	for name, entries := range values {
		if name == "step" || name == "review_token" || name == "confirmed" || name == "captcha_answer" || name == "captcha_id" || name == "captcha_response" || name == "g-recaptcha-response" || name == "h-captcha-response" || name == "website" {
			continue
		}
		for _, value := range entries {
			canonical.Add(name, strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
		}
	}
	hash := sha256.Sum256([]byte(canonical.Encode()))
	return hex.EncodeToString(hash[:])
}

func (s *Server) newContactReviewToken(values url.Values, honeypot bool, now time.Time) (string, string) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", ""
	}
	issued := fmt.Sprint(now.Unix())
	encodedNonce := hex.EncodeToString(nonce)
	flags := "-"
	if honeypot {
		flags = "d"
	}
	fieldsHash := contactFieldsHash(values)
	message := "contact-review|" + issued + "|" + encodedNonce + "|" + flags + "|" + fieldsHash
	mac := hmac.New(sha256.New, s.installationSecret)
	_, _ = mac.Write([]byte(message))
	token := issued + "." + encodedNonce + "." + flags + "." + fieldsHash + "." + hex.EncodeToString(mac.Sum(nil))
	return token, fieldsHash
}

func (s *Server) validContactReviewToken(token string, values url.Values, confirmed bool, now time.Time) (bool, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 5 || (parts[2] != "-" && parts[2] != "d") || !confirmed {
		return false, false
	}
	issuedAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false, false
	}
	issued := time.Unix(issuedAt, 0)
	age := now.Sub(issued)
	policy := s.cfg.Server.Pages.Contact.Antispam
	if policy.ReviewMinSeconds > 0 && age < time.Duration(policy.ReviewMinSeconds)*time.Second || policy.ReviewMaxSeconds > 0 && age > time.Duration(policy.ReviewMaxSeconds)*time.Second {
		return false, false
	}
	if parts[3] != contactFieldsHash(values) {
		return false, false
	}
	message := "contact-review|" + parts[0] + "|" + parts[1] + "|" + parts[2] + "|" + parts[3]
	mac := hmac.New(sha256.New, s.installationSecret)
	_, _ = mac.Write([]byte(message))
	signature, err := hex.DecodeString(parts[4])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return false, false
	}
	if !s.consumeContactNonce(parts[1], issued.Add(10*time.Minute), now) {
		return false, false
	}
	return true, parts[2] == "d"
}

type simpleCaptcha struct {
	ID       string `json:"captcha_id"`
	Question string `json:"question"`
}

func (s *Server) issueContactCaptcha(r *http.Request) (simpleCaptcha, error) {
	left, err := rand.Int(rand.Reader, big.NewInt(20))
	if err != nil {
		return simpleCaptcha{}, err
	}
	right, err := rand.Int(rand.Reader, big.NewInt(20))
	if err != nil {
		return simpleCaptcha{}, err
	}
	a, b, operator := left.Int64()+1, right.Int64()+1, "+"
	if a < b {
		a, b, operator = b, a, "-"
	}
	answer := a + b
	if operator == "-" {
		answer = a - b
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return simpleCaptcha{}, err
	}
	id := hex.EncodeToString(idBytes)
	hash := sha256.Sum256([]byte(fmt.Sprint(answer)))
	if s.database == nil || s.database.SQL == nil {
		return simpleCaptcha{}, fmt.Errorf("captcha storage is unavailable")
	}
	_, err = s.database.SQL.ExecContext(r.Context(), `INSERT INTO captcha_challenges (id, answer_hash, expires_at) VALUES (?, ?, ?)`, id, hex.EncodeToString(hash[:]), time.Now().UTC().Add(10*time.Minute).Format(time.RFC3339))
	if err != nil {
		return simpleCaptcha{}, err
	}
	_, _ = s.database.SQL.ExecContext(r.Context(), `DELETE FROM captcha_challenges WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return simpleCaptcha{ID: id, Question: fmt.Sprintf("%d %s %d =", a, operator, b)}, nil
}

func contactCaptchaDescriptor(provider string, challenge simpleCaptcha, siteKey string) map[string]interface{} {
	if provider == "simple" {
		return map[string]interface{}{"provider": "simple", "captcha_id": challenge.ID, "question": challenge.Question}
	}
	return map[string]interface{}{"provider": provider, "site_key": siteKey, "fallback": map[string]string{"provider": "simple", "captcha_id": challenge.ID, "question": challenge.Question}}
}

func (s *Server) contactCaptchaProvider(r *http.Request) string {
	host := normalizeContactHost(strings.ToLower(resolvedRequestHost(r)))
	if strings.HasSuffix(host, ".onion") || strings.HasSuffix(host, ".i2p") {
		return "simple"
	}
	provider := strings.ToLower(strings.TrimSpace(s.cfg.Server.Pages.Contact.Captcha))
	if provider == "recaptcha" || provider == "hcaptcha" {
		return provider
	}
	return "simple"
}

func (s *Server) consumeContactCaptcha(r *http.Request, values url.Values) bool {
	provider := s.contactCaptchaProvider(r)
	providerResponse := values.Get("captcha_response")
	if provider == "recaptcha" && providerResponse == "" {
		providerResponse = values.Get("g-recaptcha-response")
	}
	if provider == "hcaptcha" && providerResponse == "" {
		providerResponse = values.Get("h-captcha-response")
	}
	if providerResponse != "" {
		return s.verifyExternalCaptcha(r, provider, providerResponse)
	}
	id, answer := values.Get("captcha_id"), values.Get("captcha_answer")
	if s.database == nil || s.database.SQL == nil || id == "" || answer == "" {
		return false
	}
	var expected string
	var expires string
	err := s.database.SQL.QueryRowContext(r.Context(), `SELECT answer_hash, expires_at FROM captcha_challenges WHERE id = ?`, id).Scan(&expected, &expires)
	if err != nil {
		return false
	}
	_, _ = s.database.SQL.ExecContext(r.Context(), `DELETE FROM captcha_challenges WHERE id = ?`, id)
	deadline, err := time.Parse(time.RFC3339, expires)
	if err != nil || !time.Now().UTC().Before(deadline) {
		return false
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(answer)))
	provided := hex.EncodeToString(hash[:])
	return hmac.Equal([]byte(expected), []byte(provided))
}

func (s *Server) verifyExternalCaptcha(r *http.Request, provider, response string) bool {
	endpoint := ""
	switch provider {
	case "recaptcha":
		endpoint = "https://www.google.com/recaptcha/api/siteverify"
	case "hcaptcha":
		endpoint = "https://api.hcaptcha.com/siteverify"
	default:
		return false
	}
	form := url.Values{"secret": {s.cfg.Server.Pages.Contact.CaptchaSecretKey}, "response": {response}}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if peer != "" {
		form.Set("remoteip", peer)
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 5 * time.Second}
	responseBody, err := client.Do(request)
	if err != nil {
		return false
	}
	defer func() {
		if err := responseBody.Body.Close(); err != nil {
			log.Printf("CAPTCHA response close failed: %v", err)
		}
	}()
	if responseBody.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(responseBody.Body, 16<<10)).Decode(&result); err != nil {
		return false
	}
	return result.Success
}

func sameOriginContactRequest(r *http.Request) bool {
	if site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); site != "" && site != "same-origin" {
		return false
	}
	value := strings.TrimSpace(r.Header.Get("Origin"))
	if value == "" {
		value = strings.TrimSpace(r.Referer())
	}
	if value == "" {
		return false
	}
	provided, err := url.Parse(value)
	expected, expectedErr := url.Parse(requestOrigin(r))
	return err == nil && expectedErr == nil && provided.Host != "" && strings.EqualFold(provided.Host, expected.Host) && strings.EqualFold(provided.Scheme, expected.Scheme) && provided.User == nil
}

func (s *Server) contactRateWindow() time.Duration {
	window, err := time.ParseDuration(strings.TrimSpace(s.cfg.Server.Pages.Contact.Antispam.Window))
	if err != nil || window <= 0 {
		return time.Hour
	}
	return window
}

func contactClientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func contactPrivateNetwork(r *http.Request) bool {
	host := strings.ToLower(normalizeContactHost(resolvedRequestHost(r)))
	return strings.HasSuffix(host, ".onion") || strings.HasSuffix(host, ".i2p")
}

func (s *Server) contactRateLimit(r *http.Request, email string, now time.Time) time.Duration {
	policy := s.cfg.Server.Pages.Contact.Antispam
	window := s.contactRateWindow()
	cutoff := now.Add(-window)
	state := s.antispamState()
	state.mu.Lock()
	defer state.mu.Unlock()
	for key, entries := range state.counts {
		if entries = pruneContactTimes(entries, cutoff); len(entries) == 0 {
			delete(state.counts, key)
		} else {
			state.counts[key] = entries
		}
	}
	for key, entries := range state.strikes {
		if entries = pruneContactTimes(entries, cutoff); len(entries) == 0 {
			delete(state.strikes, key)
		} else {
			state.strikes[key] = entries
		}
	}
	for key, until := range state.bans {
		if !now.Before(until) {
			delete(state.bans, key)
		}
	}
	ip := contactClientIP(r)
	if bannedUntil, exists := state.bans[ip]; exists {
		if now.Before(bannedUntil) {
			return bannedUntil.Sub(now)
		}
		delete(state.bans, ip)
	}
	keys := []struct {
		key   string
		limit int
	}{
		{key: "global", limit: policy.Global},
	}
	if !contactPrivateNetwork(r) {
		keys = append(keys, struct {
			key   string
			limit int
		}{key: "ip:" + ip, limit: policy.PerIP})
	}
	if normalizedEmail := strings.ToLower(strings.TrimSpace(email)); normalizedEmail != "" {
		emailHash := sha256.Sum256([]byte(normalizedEmail))
		keys = append(keys, struct {
			key   string
			limit int
		}{key: "email:" + hex.EncodeToString(emailHash[:]), limit: policy.PerEmail})
	}
	for _, item := range keys {
		if item.limit <= 0 {
			continue
		}
		entries := pruneContactTimes(state.counts[item.key], cutoff)
		if len(entries) > 0 {
			state.counts[item.key] = entries
		}
		if len(entries) >= item.limit {
			return entries[0].Add(window).Sub(now)
		}
	}
	return 0
}

func pruneContactTimes(entries []time.Time, cutoff time.Time) []time.Time {
	first := 0
	for first < len(entries) && !entries[first].After(cutoff) {
		first++
	}
	return append([]time.Time(nil), entries[first:]...)
}

func (s *Server) countContactAccepted(r *http.Request, email string, now time.Time) {
	state := s.antispamState()
	state.mu.Lock()
	defer state.mu.Unlock()
	window := s.contactRateWindow()
	cutoff := now.Add(-window)
	keys := []string{"global"}
	if !contactPrivateNetwork(r) {
		keys = append(keys, "ip:"+contactClientIP(r))
	}
	if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
		emailHash := sha256.Sum256([]byte(email))
		keys = append(keys, "email:"+hex.EncodeToString(emailHash[:]))
	}
	for _, key := range keys {
		state.counts[key] = append(pruneContactTimes(state.counts[key], cutoff), now)
	}
	boundContactTimesMap(state.counts)
}

func (s *Server) recordContactStrike(r *http.Request, reason string, now time.Time) bool {
	policy := s.cfg.Server.Pages.Contact.Antispam
	ip := contactClientIP(r)
	log.Printf("contact.spam_blocked reason=%s ip=%q", reason, ip)
	if policy.Strikes <= 0 || contactPrivateNetwork(r) || ip == "" {
		return false
	}
	state := s.antispamState()
	state.mu.Lock()
	defer state.mu.Unlock()
	window := s.contactRateWindow()
	strikes := append(pruneContactTimes(state.strikes[ip], now.Add(-window)), now)
	state.strikes[ip] = strikes
	boundContactTimesMap(state.strikes)
	if len(strikes) < policy.Strikes {
		return false
	}
	banDuration := time.Duration(policy.BanHours) * time.Hour
	if banDuration <= 0 {
		return false
	}
	state.bans[ip] = now.Add(banDuration)
	boundContactBans(state.bans)
	return true
}

func boundContactBans(values map[string]time.Time) {
	for len(values) > 50000 {
		oldestKey := ""
		var earliest time.Time
		for key, until := range values {
			if oldestKey == "" || until.Before(earliest) {
				oldestKey, earliest = key, until
			}
		}
		delete(values, oldestKey)
	}
}

func (s *Server) contactDuplicate(values url.Values, now time.Time) bool {
	policy := s.cfg.Server.Pages.Contact.Antispam
	if policy.DuplicateWindowHours <= 0 {
		return false
	}
	canonical := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(values.Get("email"))), " ")) + "\x00" + strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(values.Get("subject"))), " ")) + "\x00" + strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(values.Get("message"))), " "))
	digest := sha256.Sum256([]byte(canonical))
	key := hex.EncodeToString(digest[:])
	state := s.antispamState()
	state.mu.Lock()
	defer state.mu.Unlock()
	window := time.Duration(policy.DuplicateWindowHours) * time.Hour
	for candidate, created := range state.duplicates {
		if now.Sub(created) >= window {
			delete(state.duplicates, candidate)
		}
	}
	if created, exists := state.duplicates[key]; exists && now.Sub(created) < window {
		return true
	}
	if len(state.duplicates) >= 10000 {
		oldestKey := ""
		var oldest time.Time
		for candidate, created := range state.duplicates {
			if oldestKey == "" || created.Before(oldest) {
				oldestKey, oldest = candidate, created
			}
		}
		delete(state.duplicates, oldestKey)
	}
	state.duplicates[key] = now
	return false
}

func boundContactTimesMap(values map[string][]time.Time) {
	for len(values) > 50000 {
		oldestKey := ""
		var oldest time.Time
		for key, entries := range values {
			if len(entries) == 0 {
				oldestKey = key
				break
			}
			if oldestKey == "" || entries[0].Before(oldest) {
				oldestKey, oldest = key, entries[0]
			}
		}
		delete(values, oldestKey)
	}
}

func (s *Server) writeContactRateLimit(w http.ResponseWriter, r *http.Request, retry time.Duration) {
	seconds := int(retry.Seconds()) + 1
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeProblem(w, http.StatusTooManyRequests, "The report form is temporarily rate limited")
		return
	}
	s.renderStatusError(w, r, http.StatusTooManyRequests)
}

func (s *Server) parseContactAPIBody(w http.ResponseWriter, r *http.Request) (url.Values, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("contact body must contain one JSON value")
	}
	values := make(url.Values, len(fields))
	for name, raw := range fields {
		var textValue string
		if err := json.Unmarshal(raw, &textValue); err == nil {
			values.Set(name, textValue)
			continue
		}
		var boolValue bool
		if err := json.Unmarshal(raw, &boolValue); err == nil {
			if boolValue {
				values.Set(name, "true")
			}
			continue
		}
		var number json.Number
		if err := json.Unmarshal(raw, &number); err == nil {
			values.Set(name, number.String())
			continue
		}
		return nil, fmt.Errorf("contact fields must be strings or booleans")
	}
	return values, nil
}

func (s *Server) handleContactFormPost(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 256<<10 {
		writeProblem(w, http.StatusRequestEntityTooLarge, "The report is too large")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	if err := r.ParseForm(); err != nil {
		writeProblem(w, http.StatusBadRequest, "Please submit the form from this site")
		return
	}
	if !sameOriginContactRequest(r) {
		s.recordContactStrike(r, "origin", time.Now())
		writeProblem(w, http.StatusBadRequest, "Please submit the form from this site")
		return
	}
	s.processContactSubmission(w, r, r.PostForm, false)
}

func (s *Server) handleContactAPIPost(w http.ResponseWriter, r *http.Request) {
	values, err := s.parseContactAPIBody(w, r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "A valid contact submission is required")
		return
	}
	s.processContactSubmission(w, r, values, true)
}

func (s *Server) processContactSubmission(w http.ResponseWriter, r *http.Request, values url.Values, api bool) {
	now := time.Now()
	if retry := s.contactRateLimit(r, values.Get("email"), now); retry > 0 {
		s.writeContactRateLimit(w, r, retry)
		return
	}
	if api {
		if _, supplied := values["website"]; supplied {
			s.contactFailure(w, r, true, values.Get("type"), false, values, map[string]string{"website": "unknown field"}, "A valid contact submission is required")
			return
		}
	}
	kind := values.Get("type")
	securityMode := kind == "security" && s.validSecurityID(values.Get("security_id"), time.Now())
	if !securityMode && kind != "abuse" && kind != "dmca" && kind != "privacy" {
		s.contactFailure(w, r, api, kind, securityMode, values, map[string]string{"type": "A valid report type is required"}, "Please choose a valid report type")
		return
	}
	step := values.Get("step")
	if step == "" {
		step = "submit"
	}
	if step == "edit" {
		s.renderContactForm(w, r, kind, securityMode, values, "")
		return
	}
	shapeErrors := validateContactShape(values, kind)
	if len(shapeErrors) != 0 {
		s.contactFailure(w, r, api, kind, securityMode, values, shapeErrors, "Please check the report fields")
		return
	}
	if step == "submit" {
		if !s.validContactFormToken(values.Get("form_token"), now) {
			if s.contactNonceConsumed(values.Get("form_token"), 1, 3) {
				s.recordContactStrike(r, "replay", now)
			}
			s.contactFailure(w, r, api, kind, securityMode, values, map[string]string{"form_token": "Please wait a moment and submit again"}, "Please wait a moment and submit again")
			return
		}
		honeypot := strings.TrimSpace(values.Get("website")) != ""
		if !s.consumeContactCaptcha(r, values) {
			s.recordContactStrike(r, "captcha", now)
			s.contactFailure(w, r, api, kind, securityMode, values, map[string]string{"captcha": "The verification answer is invalid"}, "The verification answer is invalid")
			return
		}
		if !honeypot {
			validation := validateContactValues(values, kind, resolvedRequestHost(r), s.cfg)
			if kind == "abuse" || kind == "dmca" {
				if err := s.matchReportedRoute(values.Get("reported_url")); err != nil {
					validation["reported_url"] = "invalid URL"
				}
			}
			if len(validation) != 0 {
				if validation["subject"] == "invalid content" || validation["message"] == "invalid content" || validation["email"] == "invalid content" {
					s.recordContactStrike(r, "content", now)
				}
				s.contactFailure(w, r, api, kind, securityMode, values, validation, "Please check the report fields")
				return
			}
		}
		token, _ := s.newContactReviewToken(values, honeypot, time.Now())
		if api {
			respondJSON(w, http.StatusOK, map[string]interface{}{"review": map[string]string{"type": kind, "email": values.Get("email"), "subject": values.Get("subject")}, "review_token": token, "confirm_after_seconds": max(0, s.cfg.Server.Pages.Contact.Antispam.ReviewMinSeconds)})
			return
		}
		s.renderContactReview(w, r, kind, securityMode, values, token, "")
		return
	}
	if step != "confirm" {
		s.contactFailure(w, r, api, kind, securityMode, values, map[string]string{"step": "The report step is invalid"}, "The report step is invalid")
		return
	}
	valid, dropped := s.validContactReviewToken(values.Get("review_token"), values, values.Get("confirmed") == "true", time.Now())
	if !valid {
		if s.contactNonceConsumed(values.Get("review_token"), 1, 5) {
			s.recordContactStrike(r, "replay", now)
		}
		s.contactFailure(w, r, api, kind, securityMode, values, map[string]string{"review_token": "Please read the details and confirm"}, "Please read the details and confirm")
		return
	}
	if dropped {
		s.recordContactStrike(r, "honeypot", now)
		s.contactSuccess(w, api, "")
		return
	}
	if kind != "security" && s.contactDuplicate(values, now) {
		s.recordContactStrike(r, "duplicate", now)
		s.contactSuccess(w, api, "")
		return
	}
	if retry := s.contactRateLimit(r, values.Get("email"), now); retry > 0 {
		s.writeContactRateLimit(w, r, retry)
		return
	}
	s.countContactAccepted(r, values.Get("email"), now)
	if kind == "security" {
		reportValues := make(map[string]string)
		for name, entries := range values {
			if name == "form_token" || name == "captcha_id" || name == "captcha_answer" || name == "website" || name == "step" || name == "review_token" || name == "confirmed" {
				continue
			}
			if len(entries) > 0 {
				reportValues[name] = entries[0]
			}
		}
		trackingID, statusToken, encrypted, err := s.saveSecurityReport(r, reportValues)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "The security report could not be stored")
			return
		}
		component := strings.Map(func(char rune) rune {
			if unicode.IsControl(char) {
				return ' '
			}
			return char
		}, reportValues["component"])
		log.Printf("security.report_received tracking_id=%s severity=%s component=%q", trackingID, reportValues["severity"], strings.TrimSpace(component))
		s.sendSecurityReportMessages(r, trackingID, statusToken, reportValues, encrypted)
		if api {
			respondJSON(w, http.StatusOK, map[string]string{"tracking_id": trackingID})
			return
		}
		s.contactSuccess(w, false, "We received your report. Your tracking ID is "+trackingID+".")
		return
	}
	body := contactPlainBody(values)
	if err := s.sendContactNotification(resolvedRequestHost(r), kind, values.Get("name"), values.Get("email"), values.Get("subject"), body); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "The report could not be delivered. Please try again later")
		return
	}
	s.contactSuccess(w, api, "")
}

func (s *Server) matchReportedRoute(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	request := &http.Request{Method: http.MethodGet, URL: parsed, Host: parsed.Host}
	match := &mux.RouteMatch{}
	if !s.router.Match(request, match) || match.Handler == nil {
		return fmt.Errorf("reported URL does not match an application route")
	}
	return nil
}

func (s *Server) contactFailure(w http.ResponseWriter, r *http.Request, api bool, kind string, securityMode bool, values url.Values, errors map[string]string, message string) {
	formToken := s.newContactFormToken(time.Now())
	captcha, err := s.issueContactCaptcha(r)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "The report form is temporarily unavailable")
		return
	}
	if api {
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "about:blank", "title": http.StatusText(http.StatusBadRequest), "status": http.StatusBadRequest, "detail": message, "errors": errors, "form_token": formToken, "captcha": contactCaptchaDescriptor(s.contactCaptchaProvider(r), captcha, s.cfg.Server.Pages.Contact.CaptchaSiteKey)})
		return
	}
	s.renderContactFormWithStatus(w, r, kind, securityMode, values, errors, formToken, captcha, http.StatusBadRequest)
}

func contactPlainBody(values url.Values) string {
	var body strings.Builder
	for _, name := range []string{"type", "reported_url", "reason", "copyrighted_work", "good_faith", "accuracy", "signature", "request_kind", "name", "email", "subject", "message"} {
		if value := strings.TrimSpace(values.Get(name)); value != "" {
			fmt.Fprintf(&body, "%s: %s\n", strings.ReplaceAll(name, "_", " "), value)
		}
	}
	return body.String()
}

func (s *Server) contactSuccess(w http.ResponseWriter, api bool, message string) {
	if api {
		respondJSON(w, http.StatusOK, map[string]interface{}{"submitted": true})
		return
	}
	if message == "" {
		message = s.cfg.Server.Pages.Contact.SuccessMessage
	}
	data := map[string]interface{}{"Title": "Report received", "Description": "Your report has been received.", "Page": "contact", "ContentTemplate": "contact-success-content", "SuccessMessage": message}
	if err := s.renderPage(w, &http.Request{URL: &url.URL{Path: "/server/contact"}}, data); err != nil {
		return
	}
}

func (s *Server) renderContactForm(w http.ResponseWriter, r *http.Request, kind string, securityMode bool, values url.Values, errorMessage string) {
	captcha, err := s.issueContactCaptcha(r)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "The report form is temporarily unavailable")
		return
	}
	s.renderContactFormWithStatus(w, r, kind, securityMode, values, map[string]string{}, s.newContactFormToken(time.Now()), captcha, http.StatusOK)
}

func (s *Server) renderContactFormWithStatus(w http.ResponseWriter, r *http.Request, kind string, securityMode bool, values url.Values, errors map[string]string, formToken string, captcha simpleCaptcha, status int) {
	fields := make(map[string]string, len(values))
	for name := range values {
		fields[name] = values.Get(name)
	}
	data := map[string]interface{}{"Title": contactTypeLabels[kind], "Description": "Send a report to the service operator.", "Page": "contact", "ContentTemplate": "contact-content", "ContactType": kind, "ContactTypeTitle": contactTypeLabels[kind], "ContactChooser": false, "SecurityMode": securityMode, "SecurityID": values.Get("security_id"), "FormToken": formToken, "ContactFields": contactFields(kind), "CaptchaID": captcha.ID, "CaptchaQuestion": captcha.Question, "CaptchaProvider": s.contactCaptchaProvider(r), "CaptchaSiteKey": s.cfg.Server.Pages.Contact.CaptchaSiteKey, "ContactValues": fields, "ContactErrors": errors}
	if err := s.renderPageWithStatus(w, r, data, status); err != nil {
		return
	}
}

func (s *Server) renderContactReview(w http.ResponseWriter, r *http.Request, kind string, securityMode bool, values url.Values, token, errorMessage string) {
	data := map[string]interface{}{"Title": "Review report", "Description": "Review the report before sending.", "Page": "contact", "ContentTemplate": "contact-review-content", "ContactType": kind, "ContactValues": values, "ReviewToken": token, "SecurityMode": securityMode, "SecurityID": values.Get("security_id"), "ReviewError": errorMessage}
	if err := s.renderPage(w, r, data); err != nil {
		return
	}
}

func (s *Server) handleContactPage(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Server.Pages.Contact.Enabled {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost {
		s.handleContactFormPost(w, r)
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("type"))
	securityMode := s.validSecurityID(r.URL.Query().Get("security_id"), time.Now())
	if supplied := r.URL.Query().Get("security_id"); supplied != "" && !securityMode {
		s.logInvalidSecurityID(r, supplied)
	}
	if !securityMode && kind != "abuse" && kind != "dmca" && kind != "privacy" {
		kind = ""
	}
	if securityMode {
		kind = "security"
	}
	data := map[string]interface{}{
		"Title": "Report", "Description": "Submit a security, abuse, copyright, or privacy report.",
		"Page": "contact", "ContentTemplate": "contact-content", "ContactType": kind,
		"ContactTypeTitle": contactTypeLabels[kind], "ContactChooser": kind == "",
		"FormToken": s.newContactFormToken(time.Now()), "ContactFields": contactFields(kind),
		"SecurityMode": securityMode, "SecurityID": "",
		"ContactTypes": []map[string]string{{"Type": "abuse", "Label": contactTypeLabels["abuse"]}, {"Type": "dmca", "Label": contactTypeLabels["dmca"]}, {"Type": "privacy", "Label": contactTypeLabels["privacy"]}},
		"AbuseEmail":   s.publicAbuseEmail(), "FQDN": resolvedRequestHost(r),
		"ContactValues": map[string]string{},
	}
	if securityMode {
		data["SecurityID"] = r.URL.Query().Get("security_id")
	}
	if kind == "" {
		if err := s.renderPage(w, r, data); err != nil {
			return
		}
		return
	}
	captcha, err := s.issueContactCaptcha(r)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "The report form is temporarily unavailable")
		return
	}
	data["CaptchaID"], data["CaptchaQuestion"] = captcha.ID, captcha.Question
	data["CaptchaProvider"] = s.contactCaptchaProvider(r)
	data["CaptchaSiteKey"] = s.cfg.Server.Pages.Contact.CaptchaSiteKey
	if err := s.renderPage(w, r, data); err != nil {
		return
	}
}

func (s *Server) publicAbuseEmail() string {
	if value := strings.TrimSpace(s.cfg.Server.Contact.Abuse.Email); value != "" {
		return value
	}
	return strings.TrimSpace(s.cfg.Server.Contact.General.Email)
}

func (s *Server) logInvalidSecurityID(r *http.Request, value string) {
	hash := sha256.Sum256([]byte(value))
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	log.Printf("security.security_id_invalid ip=%q user_agent=%q id_hash=%s", peer, r.UserAgent(), hex.EncodeToString(hash[:])[:8])
}

func validateContactValues(values url.Values, kind string, requestHost string, configs ...*config.Config) map[string]string {
	errors := make(map[string]string)
	allowed := map[string]bool{"type": true, "name": true, "email": true, "subject": true, "message": true, "reported_url": true, "reason": true, "copyrighted_work": true, "good_faith": true, "accuracy": true, "signature": true, "request_kind": true, "form_token": true, "captcha_id": true, "captcha_answer": true, "captcha_response": true, "g-recaptcha-response": true, "h-captcha-response": true, "website": true, "step": true, "review_token": true, "confirmed": true, "security_id": true}
	if kind == "security" {
		for _, name := range []string{"researcher_gpg", "component", "affected_url", "severity", "summary", "steps", "impact", "suggested_fix", "cve_requested", "disclosure_days", "credit_preference", "credit_handle", "disclosure_agreement"} {
			allowed[name] = true
		}
	}
	for name := range values {
		if !allowed[name] {
			errors[name] = "unknown field"
		}
	}
	requiredFields := []string{"name", "email"}
	if kind != "security" {
		requiredFields = append(requiredFields, "subject", "message")
	}
	for _, name := range requiredFields {
		if strings.TrimSpace(values.Get(name)) == "" {
			errors[name] = "required"
		}
	}
	if address, err := mail.ParseAddress(strings.TrimSpace(values.Get("email"))); err != nil || address.Address != strings.TrimSpace(values.Get("email")) || strings.ContainsAny(values.Get("email"), ",;<>\r\n") {
		errors["email"] = "invalid email address"
	}
	for _, name := range []string{"name", "email", "subject"} {
		if strings.ContainsAny(values.Get(name), "\r\n\x00") {
			errors[name] = "invalid characters"
		}
	}
	for name, entries := range values {
		for _, value := range entries {
			for _, char := range value {
				if char == '\u200b' || char == '\u200c' || char == '\u200d' || char == '\u202a' || char == '\u202b' || char == '\u202c' || char == '\u202d' || char == '\u202e' {
					errors[name] = "invalid characters"
				}
				if !unicode.IsPrint(char) && char != '\n' && char != '\r' && char != '\t' {
					errors[name] = "invalid characters"
				}
			}
		}
	}
	limits := map[string]int{"name": 100, "email": 254, "subject": 200, "message": 5000, "reported_url": 2048, "copyrighted_work": 2000, "signature": 100, "researcher_gpg": 16384, "component": 255, "affected_url": 2048, "summary": 200, "steps": 10000, "impact": 5000, "suggested_fix": 5000, "credit_handle": 100}
	for name, limit := range limits {
		if utf8.RuneCountInString(strings.TrimSpace(values.Get(name))) > limit {
			errors[name] = "too long"
		}
	}
	if kind == "abuse" || kind == "dmca" {
		if strings.TrimSpace(values.Get("reported_url")) == "" {
			errors["reported_url"] = "required"
		} else if err := validateReportedURL(values.Get("reported_url"), requestHost); err != nil {
			errors["reported_url"] = "invalid URL"
		}
	}
	if kind == "abuse" && !oneOf(values.Get("reason"), "illegal_content", "terms_violation", "harassment", "malware_phishing", "other") {
		errors["reason"] = "required"
	}
	if kind == "dmca" {
		for name, expected := range map[string]string{"good_faith": "true", "accuracy": "true"} {
			if values.Get(name) != expected {
				errors[name] = "required"
			}
		}
		if strings.TrimSpace(values.Get("copyrighted_work")) == "" || strings.TrimSpace(values.Get("signature")) == "" {
			errors["copyrighted_work"] = "required"
		}
	}
	if kind == "privacy" && !oneOf(values.Get("request_kind"), "access", "export", "deletion", "correction", "opt_out", "other") {
		errors["request_kind"] = "required"
	}
	if kind == "security" {
		for _, name := range []string{"component", "summary", "steps", "impact"} {
			if strings.TrimSpace(values.Get(name)) == "" {
				errors[name] = "required"
			}
		}
		if !oneOf(values.Get("severity"), "critical", "high", "medium", "low", "informational") {
			errors["severity"] = "required"
		}
		if !oneOf(values.Get("credit_preference"), "name", "handle", "no", "anonymous") {
			errors["credit_preference"] = "required"
		}
		if values.Get("credit_preference") == "handle" && strings.TrimSpace(values.Get("credit_handle")) == "" {
			errors["credit_handle"] = "required"
		}
		if values.Get("disclosure_agreement") != "true" {
			errors["disclosure_agreement"] = "required"
		}
		if key := strings.TrimSpace(values.Get("researcher_gpg")); key != "" {
			if _, err := FetchResearcherPGPKey(key); err != nil {
				errors["researcher_gpg"] = "invalid public key"
			}
		}
	}
	if kind != "security" {
		var policy config.ContactAntispamConfig
		if len(configs) > 0 && configs[0] != nil {
			policy = configs[0].Server.Pages.Contact.Antispam
		} else {
			policy.RejectMarkup = true
			policy.MaxLinks = 10
		}
		linkCount := 0
		blockedWords := make([]string, 0, len(policy.BlockedWords))
		for _, word := range policy.BlockedWords {
			if word = strings.ToLower(strings.TrimSpace(word)); word != "" {
				blockedWords = append(blockedWords, word)
			}
		}
		for name, entries := range values {
			if name == "name" || name == "subject" {
				if containsContactLink(entries[0]) {
					errors[name] = "invalid content"
				}
			}
			for _, value := range entries {
				if bool(policy.RejectMarkup) && contactMarkupPattern.MatchString(decodeContactEntities(value)) {
					errors[name] = "invalid content"
				}
				if name != "email" {
					links := contactLinkCount(value)
					linkCount += links
					if links > 0 && !linksMatchHost(value, requestHost) {
						errors[name] = "invalid content"
					}
				}
				lower := strings.ToLower(value)
				for _, word := range blockedWords {
					if strings.Contains(lower, word) {
						errors[name] = "invalid content"
						break
					}
				}
			}
		}
		if policy.MaxLinks > 0 && linkCount > policy.MaxLinks {
			errors["content"] = "invalid content"
		}
		parts := strings.Split(strings.ToLower(strings.TrimSpace(values.Get("email"))), "@")
		if len(parts) == 2 {
			for _, blockedDomain := range policy.BlockedEmailDomains {
				if strings.EqualFold(parts[1], strings.TrimSpace(blockedDomain)) {
					errors["email"] = "invalid content"
					break
				}
			}
		}
	}
	return errors
}

func validateContactShape(values url.Values, kind string) map[string]string {
	errors := make(map[string]string)
	allowed := map[string]bool{"type": true, "name": true, "email": true, "subject": true, "message": true, "reported_url": true, "reason": true, "copyrighted_work": true, "good_faith": true, "accuracy": true, "signature": true, "request_kind": true, "form_token": true, "captcha_id": true, "captcha_answer": true, "captcha_response": true, "g-recaptcha-response": true, "h-captcha-response": true, "website": true, "step": true, "review_token": true, "confirmed": true, "security_id": true}
	if kind == "security" {
		for _, name := range []string{"researcher_gpg", "component", "affected_url", "severity", "summary", "steps", "impact", "suggested_fix", "cve_requested", "disclosure_days", "credit_preference", "credit_handle", "disclosure_agreement"} {
			allowed[name] = true
		}
	}
	for name := range values {
		if !allowed[name] {
			errors[name] = "unknown field"
		}
	}
	required := []string{"name", "email"}
	required = append(required, "type", "form_token")
	if kind != "security" {
		required = append(required, "subject", "message")
	}
	for _, name := range required {
		if strings.TrimSpace(values.Get(name)) == "" {
			errors[name] = "required"
		}
	}
	if address, err := mail.ParseAddress(strings.TrimSpace(values.Get("email"))); err != nil || address.Address != strings.TrimSpace(values.Get("email")) || strings.ContainsAny(values.Get("email"), ",;<>\r\n") {
		errors["email"] = "invalid email address"
	}
	for _, name := range []string{"name", "email", "subject"} {
		if strings.ContainsAny(values.Get(name), "\r\n\x00") {
			errors[name] = "invalid characters"
		}
	}
	for name, entries := range values {
		for _, value := range entries {
			for _, char := range value {
				if char == '\u200b' || char == '\u200c' || char == '\u200d' || char == '\u202a' || char == '\u202b' || char == '\u202c' || char == '\u202d' || char == '\u202e' || !utf8.ValidString(value) {
					errors[name] = "invalid characters"
				}
			}
		}
	}
	limits := map[string]int{"name": 100, "email": 254, "subject": 200, "message": 5000, "reported_url": 2048, "copyrighted_work": 2000, "signature": 100, "researcher_gpg": 16384, "component": 255, "affected_url": 2048, "summary": 200, "steps": 10000, "impact": 5000, "suggested_fix": 5000, "credit_handle": 100}
	for name, limit := range limits {
		if utf8.RuneCountInString(strings.TrimSpace(values.Get(name))) > limit {
			errors[name] = "too long"
		}
	}
	if kind == "abuse" || kind == "dmca" {
		if strings.TrimSpace(values.Get("reported_url")) == "" {
			errors["reported_url"] = "required"
		}
	}
	if kind == "dmca" {
		if strings.TrimSpace(values.Get("copyrighted_work")) == "" {
			errors["copyrighted_work"] = "required"
		}
		if values.Get("good_faith") != "true" {
			errors["good_faith"] = "required"
		}
		if values.Get("accuracy") != "true" {
			errors["accuracy"] = "required"
		}
		if strings.TrimSpace(values.Get("signature")) == "" {
			errors["signature"] = "required"
		}
	}
	if kind == "abuse" && !oneOf(values.Get("reason"), "illegal_content", "terms_violation", "harassment", "malware_phishing", "other") {
		errors["reason"] = "required"
	}
	if kind == "privacy" && !oneOf(values.Get("request_kind"), "access", "export", "deletion", "correction", "opt_out", "other") {
		errors["request_kind"] = "required"
	}
	if kind == "security" {
		for _, name := range []string{"component", "summary", "steps", "impact"} {
			if strings.TrimSpace(values.Get(name)) == "" {
				errors[name] = "required"
			}
		}
		if !oneOf(values.Get("severity"), "critical", "high", "medium", "low", "informational") {
			errors["severity"] = "required"
		}
		if !oneOf(values.Get("credit_preference"), "name", "handle", "no", "anonymous") {
			errors["credit_preference"] = "required"
		}
		if values.Get("credit_preference") == "handle" && strings.TrimSpace(values.Get("credit_handle")) == "" {
			errors["credit_handle"] = "required"
		}
		if values.Get("disclosure_agreement") != "true" {
			errors["disclosure_agreement"] = "required"
		}
	}
	return errors
}

var contactMarkupPattern = regexp.MustCompile(`(?i)(</?[a-z][^>]*>|<!|&(?:lt|gt|#0*60|#0*62);|\[(?:/?(?:url|img|b))\b)`)
var contactLinkPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|www\.|\b[a-z0-9.-]+\.[a-z]{2,}\b)`)

func decodeContactEntities(value string) string {
	for range 3 {
		decoded := html.UnescapeString(value)
		if decoded == value {
			break
		}
		value = decoded
	}
	return value
}

func contactLinkCount(value string) int {
	count := 0
	for _, token := range strings.Fields(value) {
		if strings.Contains(token, "@") {
			continue
		}
		count += len(contactLinkPattern.FindAllString(token, -1))
	}
	return count
}

func normalizeContactHost(value string) string {
	if parsed, err := url.Parse("//" + value); err == nil && parsed.Hostname() != "" {
		return strings.ToLower(parsed.Hostname())
	}
	return strings.ToLower(value)
}

func containsContactLink(value string) bool { return contactLinkPattern.MatchString(value) }

func linksMatchHost(value, requestHost string) bool {
	allowedHosts := []string{normalizeContactHost(requestHost)}
	for _, domain := range configuredDomains() {
		allowedHosts = append(allowedHosts, normalizeContactHost(domain))
	}
	for _, variable := range []string{"TOR_HOSTNAME", "I2P_HOSTNAME"} {
		if host := strings.TrimSpace(os.Getenv(variable)); host != "" {
			allowedHosts = append(allowedHosts, normalizeContactHost(host))
		}
	}
	for _, token := range strings.Fields(value) {
		candidate := strings.Trim(token, "\"'()[]{}.,!?;:")
		if !contactLinkPattern.MatchString(candidate) || strings.Contains(candidate, "@") {
			continue
		}
		if !strings.Contains(candidate, "://") {
			candidate = "https://" + candidate
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" {
			return false
		}
		allowed := false
		for _, host := range allowedHosts {
			if strings.EqualFold(parsed.Hostname(), host) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}

func validateReportedURL(value, requestHost string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || !linksMatchHost(parsed.Host, requestHost) || parsed.Path == "" || parsed.Path == "/" || parsed.Path == "/api" || strings.HasPrefix(parsed.Path, "/api/") || parsed.Path == "/server/contact" {
		return fmt.Errorf("invalid reported URL")
	}
	return nil
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func (s *Server) handleContactAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.cfg.Server.Pages.Contact.Enabled {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		s.handleContactAPIPost(w, r)
		return
	}
	securityMode := s.validSecurityID(r.URL.Query().Get("security_id"), time.Now())
	if supplied := r.URL.Query().Get("security_id"); supplied != "" && !securityMode {
		s.logInvalidSecurityID(r, supplied)
	}
	fields := contactAPIFields(false)
	if securityMode {
		fields = contactFields("security")
		for index := range fields {
			if fields[index].Name == "researcher_gpg" {
				fields[index].MaxLengthWhen = []contactLengthRule{{Prefix: "https://", MaxLength: 2048}}
			}
		}
	}
	formToken := s.newContactFormToken(time.Now())
	captcha, err := s.issueContactCaptcha(r)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "The report form is temporarily unavailable")
		return
	}
	var types []string
	if securityMode {
		types = []string{"security"}
	} else {
		types = []string{"abuse", "dmca", "privacy"}
	}
	data := map[string]interface{}{
		"fields": fields, "captcha": contactCaptchaDescriptor(s.contactCaptchaProvider(r), captcha, s.cfg.Server.Pages.Contact.CaptchaSiteKey),
		"form_token": formToken, "types": types, "security_url": requestOrigin(r) + "/server/security",
		"abuse_email": s.publicAbuseEmail(),
	}
	if securityMode {
		data["security_id"] = r.URL.Query().Get("security_id")
	}
	respondJSON(w, http.StatusOK, data)
}
