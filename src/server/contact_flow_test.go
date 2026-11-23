package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestContactAPISubmissionReturnsReviewWithoutDelivery(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Server.Pages.Contact.Antispam.MinFillSeconds = 0
	s.cfg.Server.Pages.Contact.Antispam.ReviewMinSeconds = 0
	s.cfg.Server.Contact.General.Email = "reports@example.test"
	s.smtpProbed = true
	s.smtpAvailable = false
	get := httptest.NewRecorder()
	s.router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/server/contact", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("contact descriptor status=%d body=%s", get.Code, get.Body.String())
	}
	var descriptor struct {
		Data struct {
			FormToken string `json:"form_token"`
			Captcha   struct {
				ID string `json:"captcha_id"`
			} `json:"captcha"`
		} `json:"data"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &descriptor); err != nil {
		t.Fatal(err)
	}
	var expected string
	if err := s.database.SQL.QueryRow(`SELECT answer_hash FROM captcha_challenges WHERE id = ?`, descriptor.Data.Captcha.ID).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	answer := ""
	for candidate := 0; candidate <= 40; candidate++ {
		value := strconv.Itoa(candidate)
		digest := sha256.Sum256([]byte(value))
		if hex.EncodeToString(digest[:]) == expected {
			answer = value
			break
		}
	}
	if answer == "" {
		t.Fatalf("could not resolve challenge answer hash %q", expected)
	}
	form := map[string]string{
		"type": "privacy", "name": "Visitor", "email": "visitor@example.test",
		"request_kind": "access", "subject": "Privacy request", "message": "Please provide my data.",
		"form_token": descriptor.Data.FormToken, "captcha_id": descriptor.Data.Captcha.ID,
		"captcha_answer": answer,
	}
	body, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"review_token"`) {
		t.Fatalf("contact submission status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var review struct {
		Data struct {
			ReviewToken string `json:"review_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &review); err != nil || review.Data.ReviewToken == "" {
		t.Fatalf("review response token=%q err=%v", review.Data.ReviewToken, err)
	}
	var challengeCount int
	if err := s.database.SQL.QueryRow(`SELECT COUNT(*) FROM captcha_challenges WHERE id = ?`, descriptor.Data.Captcha.ID).Scan(&challengeCount); err != nil {
		t.Fatal(err)
	}
	if challengeCount != 0 {
		t.Fatal("accepted contact challenge was not consumed")
	}
	form["step"] = "confirm"
	form["review_token"] = review.Data.ReviewToken
	form["confirmed"] = "true"
	body, err = json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("confirm with unavailable mail transport status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestContactAPIRejectsMalformedAndUnknownFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "unknown field", body: `{"type":"privacy","surprise":"value"}`},
		{name: "non-scalar field", body: `{"type":"privacy","message":["bad"]}`},
		{name: "multiple JSON values", body: `{} {}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			s := newTestServer(t)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			s.router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
