package server

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/gorilla/mux"
)

type securityReportStatus struct {
	TrackingID        string
	Status            string
	ResearcherComment string
	DisclosureDate    string
	Severity          string
}

func (s *Server) handlePGPKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.securityPublicKey()
	if err != nil {
		writeProblem(w, http.StatusNotFound, "No public encryption key is configured")
		return
	}
	block, err := armor.Decode(bytes.NewReader(key))
	if err != nil || block.Type != "PGP PUBLIC KEY BLOCK" {
		writeProblem(w, http.StatusNotFound, "No public encryption key is configured")
		return
	}
	w.Header().Set("Content-Type", "application/pgp-keys")
	w.Header().Set("Content-Disposition", `attachment; filename="pgp-key.asc"`)
	if r.Method == http.MethodGet {
		_, _ = w.Write(key)
	}
}

func (s *Server) encryptReportValue(value []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s.cfg.Server.Security.EncryptionKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("security encryption key is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, value, nil)...), nil
}

func (s *Server) decryptReportValue(value []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s.cfg.Server.Security.EncryptionKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("security encryption key is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(value) < gcm.NonceSize() {
		return nil, fmt.Errorf("encrypted report value is truncated")
	}
	nonce := value[:gcm.NonceSize()]
	return gcm.Open(nil, nonce, value[gcm.NonceSize():], nil)
}

func randomReportToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func (s *Server) saveSecurityReport(r *http.Request, values map[string]string) (string, string, []byte, error) {
	securityPGPUpdateMu.Lock()
	defer securityPGPUpdateMu.Unlock()
	tx, err := s.database.SQL.BeginTx(r.Context(), nil)
	if err != nil {
		return "", "", nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("security report transaction rollback failed: %v", err)
		}
	}()
	if _, err := tx.ExecContext(r.Context(), `UPDATE security_pgp_lock SET id = id WHERE id = 1`); err != nil {
		return "", "", nil, err
	}
	trackingBytes := make([]byte, 8)
	if _, err := rand.Read(trackingBytes); err != nil {
		return "", "", nil, err
	}
	trackingID := "sec_" + hex.EncodeToString(trackingBytes)
	token, err := randomReportToken()
	if err != nil {
		return "", "", nil, err
	}
	body, err := json.Marshal(values)
	if err != nil {
		return "", "", nil, err
	}
	bodyEncrypted, bodyAlgorithm, err := s.encryptSecurityReport(body)
	if err != nil {
		return "", "", nil, err
	}
	contact, err := json.Marshal(map[string]string{"email": values["email"], "researcher_gpg": values["researcher_gpg"]})
	if err != nil {
		return "", "", nil, err
	}
	contactEncrypted, err := s.encryptReportValue(contact)
	if err != nil {
		return "", "", nil, err
	}
	tokenHash := sha256.Sum256([]byte(token))
	creditPreference := map[string]string{"name": "name", "handle": "handle", "no": "no", "anonymous": "anon"}[values["credit_preference"]]
	if creditPreference == "" {
		creditPreference = "no"
	}
	creditName := ""
	switch creditPreference {
	case "name":
		creditName = values["name"]
	case "handle":
		creditName = values["credit_handle"]
	}
	cveRequested := values["cve_requested"] == "true"
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO security_reports (tracking_id, security_id, severity, component, status, body_enc, body_alg, contact_enc, credit_pref, credit_name, cve_requested, token_hash, created_at, updated_at) VALUES (?, ?, ?, ?, 'received', ?, ?, ?, ?, ?, ?, ?, ?, ?)`, trackingID, values["security_id"], values["severity"], values["component"], bodyEncrypted, bodyAlgorithm, contactEncrypted, creditPreference, nullableStringValue(creditName), cveRequested, hex.EncodeToString(tokenHash[:]), now, now)
	if err != nil {
		return "", "", nil, err
	}
	if err := tx.Commit(); err != nil {
		return "", "", nil, err
	}
	return trackingID, token, bodyEncrypted, nil
}

func nullableStringValue(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func (s *Server) sendSecurityReportMessages(r *http.Request, trackingID, token string, values map[string]string, encryptedBody []byte) {
	statusURL := requestOrigin(r) + "/server/security/report/" + trackingID + "?token=" + token
	securityRecipient := s.contactRecipient("security")
	maintainerBody := "This vulnerability report is encrypted. The attached text block contains the encrypted report body.\n\n" + string(encryptedBody)
	if string(encryptedBody) == "" {
		maintainerBody = "The encrypted vulnerability report is attached below.\n\n"
	}
	publicKey, publicErr := s.securityPublicKey()
	if publicErr == nil {
		signer, signerErr := currentSecuritySignerFromServer(s)
		if signerErr != nil {
			log.Printf("security.report_notification_failed tracking_id=%s reason=signing_key_unavailable", trackingID)
			return
		}
		signedEncrypted, err := EncryptToSecurityPGP(publicKey, []byte(maintainerBody), signer)
		if err != nil {
			log.Printf("security.report_notification_failed tracking_id=%s reason=encryption_failed", trackingID)
			return
		}
		maintainerBody = string(signedEncrypted)
	} else {
		maintainerBody = "No project PGP key is configured. The report body is encrypted with AES-256-GCM at rest; the following block is not an email attachment and requires the operator encryption key to decrypt.\n\n" + maintainerBody
		if signature, err := s.signSecurityNotice([]byte(maintainerBody)); err == nil {
			maintainerBody += "\n\nOpenPGP signature:\n" + string(signature)
		}
	}
	if err := s.sendSMTPMessage(strings.TrimSpace(s.cfg.Server.Notifications.Email.SMTP.Host), s.cfg.Server.Notifications.Email.SMTP.Port, s.cfg.Server.Notifications.Email, securityRecipient, resolvedRequestHost(r), "", values["email"], "Encrypted vulnerability report received", maintainerBody); err != nil {
		log.Printf("security.report_notification_failed tracking_id=%s severity=%s component=%q", trackingID, values["severity"], values["component"])
	}
	ackBody := "Your security report was received. Tracking ID: " + trackingID + "\nStatus: " + statusURL
	if researcherKey := strings.TrimSpace(values["researcher_gpg"]); researcherKey != "" {
		publicKey, err := FetchResearcherPGPKey(researcherKey)
		if err != nil {
			log.Printf("security.report_acknowledgment_skipped tracking_id=%s", trackingID)
			return
		}
		signer, signerErr := currentSecuritySignerFromServer(s)
		if signerErr != nil {
			log.Printf("security.report_acknowledgment_skipped tracking_id=%s", trackingID)
			return
		}
		ackBodyBytes, err := EncryptToSecurityPGP(publicKey, []byte(ackBody), signer)
		if err != nil {
			log.Printf("security.report_acknowledgment_skipped tracking_id=%s", trackingID)
			return
		}
		ackBody = string(ackBodyBytes)
	} else if signature, err := s.signSecurityNotice([]byte(ackBody)); err == nil {
		ackBody += "\n\nOpenPGP signature:\n" + string(signature)
	}
	if err := s.sendSMTPMessage(strings.TrimSpace(s.cfg.Server.Notifications.Email.SMTP.Host), s.cfg.Server.Notifications.Email.SMTP.Port, s.cfg.Server.Notifications.Email, values["email"], resolvedRequestHost(r), "", "", "Security report received", ackBody); err != nil {
		log.Printf("security.report_acknowledgment_skipped tracking_id=%s", trackingID)
	}
}

func (s *Server) handleSecurityReportPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if s.database == nil || s.database.SQL == nil {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	id := mux.Vars(r)["tracking_id"]
	token := r.URL.Query().Get("token")
	var report securityReportStatus
	var tokenHash, lastUsed string
	var closedAt sql.NullString
	err := s.database.SQL.QueryRowContext(r.Context(), `SELECT tracking_id, status, researcher_comment, COALESCE(disclosure_date, ''), severity, token_hash, COALESCE(token_last_used, ''), closed_at FROM security_reports WHERE tracking_id = ?`, id).Scan(&report.TrackingID, &report.Status, &report.ResearcherComment, &report.DisclosureDate, &report.Severity, &tokenHash, &lastUsed, &closedAt)
	if err != nil || !validReportStatusToken(token, tokenHash) {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	if lastUsed == today {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	if closedAt.Valid {
		closed, parseErr := time.Parse(time.RFC3339, closedAt.String)
		if parseErr == nil && time.Since(closed) > 30*24*time.Hour {
			s.renderStatusError(w, r, http.StatusNotFound)
			return
		}
	}
	result, err := s.database.SQL.ExecContext(r.Context(), `UPDATE security_reports SET token_last_used = ? WHERE tracking_id = ? AND (token_last_used IS NULL OR token_last_used <> ?)`, today, id, today)
	if err != nil {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	data := map[string]interface{}{"Title": "Security report status", "Description": "Private status information for your security report.", "Page": "security-report", "ContentTemplate": "security-report-content", "Report": report}
	if err := s.renderPage(w, r, data); err != nil {
		return
	}
}

func validReportStatusToken(token, expectedHash string) bool {
	if token == "" {
		return false
	}
	providedHash := sha256.Sum256([]byte(token))
	expected, err := hex.DecodeString(expectedHash)
	if err != nil || len(expected) != len(providedHash) {
		return false
	}
	return subtle.ConstantTimeCompare(providedHash[:], expected) == 1
}

func (s *Server) listSecurityThanks(ctx context.Context) ([]map[string]string, error) {
	rows, err := s.database.SQL.QueryContext(ctx, `SELECT credit, disclosed_on, severity FROM security_thanks ORDER BY disclosed_on DESC, tracking_id DESC`)
	if err != nil {
		return nil, err
	}
	defer closeAndLog(rows, "security thanks rows")
	entries := make([]map[string]string, 0)
	for rows.Next() {
		var credit, date, severity string
		if err := rows.Scan(&credit, &date, &severity); err != nil {
			return nil, err
		}
		entries = append(entries, map[string]string{"credit": credit, "date": date, "severity": severity})
	}
	return entries, rows.Err()
}
