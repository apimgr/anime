package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaveSecurityReportEncryptsBodyAndContact(t *testing.T) {
	s := newTestServer(t)
	values := map[string]string{
		"security_id": "security-id", "severity": "high", "component": "api",
		"email": "researcher@example.test", "name": "Researcher", "summary": "Private issue",
		"steps": "Reproduce privately", "impact": "Private impact", "credit_preference": "anonymous",
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact", nil)
	trackingID, token, encrypted, err := s.saveSecurityReport(request, values)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(trackingID, "sec_") || len(trackingID) != 20 || token == "" || strings.Contains(string(encrypted), values["summary"]) {
		t.Fatalf("tracking id=%q token present=%t encrypted plaintext=%t", trackingID, token != "", strings.Contains(string(encrypted), values["summary"]))
	}
	var body []byte
	var contact []byte
	var algorithm string
	if err := s.database.SQL.QueryRow(`SELECT body_enc, body_alg, contact_enc FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&body, &algorithm, &contact); err != nil {
		t.Fatal(err)
	}
	clearBody, err := s.decryptReportValue(body)
	if err != nil || algorithm != "aes-256-gcm" || !strings.Contains(string(clearBody), values["summary"]) {
		t.Fatalf("report decryption algorithm=%q body=%q err=%v", algorithm, clearBody, err)
	}
	clearContact, err := s.decryptReportValue(contact)
	var decoded map[string]string
	if err != nil || json.Unmarshal(clearContact, &decoded) != nil || decoded["email"] != values["email"] {
		t.Fatalf("contact decryption=%q err=%v", clearContact, err)
	}
	hash := sha256.Sum256([]byte(token))
	var storedHash string
	if err := s.database.SQL.QueryRow(`SELECT token_hash FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash != hex.EncodeToString(hash[:]) {
		t.Fatal("status token was not stored as a SHA-256 hash")
	}
}

func TestSecurityThanksAndMissingPGPKeyResponses(t *testing.T) {
	s := newTestServer(t)
	_, err := s.database.SQL.Exec(`INSERT INTO security_thanks (tracking_id, credit, severity, disclosed_on) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`, "sec_0000000000000001", "Researcher", "high", "2025-01-01", "sec_0000000000000002", "Anonymous Researcher #1", "medium", "2025-01-02")
	if err != nil {
		t.Fatal(err)
	}
	thanks, err := s.listSecurityThanks(context.Background())
	if err != nil || len(thanks) != 2 || thanks[0]["credit"] != "Anonymous Researcher #1" {
		t.Fatalf("thanks=%v err=%v", thanks, err)
	}
	recorder := httptest.NewRecorder()
	s.handlePGPKey(recorder, httptest.NewRequest(http.MethodGet, "/.well-known/pgp-key.asc", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing PGP key status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSecurityReportValueEncryptionRejectsBadKeyAndCorruption(t *testing.T) {
	s := newTestServer(t)
	original := s.cfg.Server.Security.EncryptionKey
	s.cfg.Server.Security.EncryptionKey = "bad-key"
	if _, err := s.encryptReportValue([]byte("secret")); err == nil {
		t.Fatal("encryption accepted an invalid key")
	}
	s.cfg.Server.Security.EncryptionKey = original
	encrypted, err := s.encryptReportValue([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.decryptReportValue(encrypted[:4]); err == nil {
		t.Fatal("decryption accepted a truncated value")
	}
	encrypted[len(encrypted)-1] ^= 0xff
	if _, err := s.decryptReportValue(encrypted); err == nil {
		t.Fatal("decryption accepted a modified ciphertext")
	}
}
