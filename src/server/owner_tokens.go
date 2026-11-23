package server

import (
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
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const ownerTokenCookieName = "anime_owner_token_4mxwkxfqGg"

type ownerTokenIdentity struct {
	OwnerID string
	TokenID int64
	Kind    string
	Prefix  string
}

type ownerTokenListItem struct {
	ID              int64         `json:"id"`
	Name            string        `json:"name,omitempty"`
	Prefix          string        `json:"prefix"`
	Masked          string        `json:"-"`
	CreatedAt       int64         `json:"created_at"`
	LastUsed        sql.NullInt64 `json:"-"`
	ExpiresAt       sql.NullInt64 `json:"-"`
	RevokedAt       sql.NullInt64 `json:"-"`
	LastUsedAt      *int64        `json:"last_used_at"`
	ExpiresAtValue  *int64        `json:"expires_at"`
	Status          string        `json:"status"`
	CreatedDisplay  string        `json:"-"`
	LastUsedDisplay string        `json:"-"`
	ExpiresDisplay  string        `json:"-"`
	RevealedToken   string        `json:"-"`
}

type ownerTokenRateState struct {
	Started time.Time
	Count   int
}

var ownerTokenRate = struct {
	sync.Mutex
	clients map[string]ownerTokenRateState
}{clients: make(map[string]ownerTokenRateState)}

func (s *Server) handleTokensPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost {
		s.handleTokenWebAction(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST")
		writeProblem(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity, ok := s.ownerCookieIdentity(w, r)
	data := map[string]interface{}{
		"Title": "Owner tokens", "Description": "Manage the API tokens associated with this browser.",
		"Page": "preferences-tokens", "ContentTemplate": "tokens-content", "Version": Version,
		"HasOwner": ok, "CSRFToken": s.csrfToken(time.Now()), "OwnerTokenCookie": ownerTokenCookieName,
		"Error": r.URL.Query().Get("error"),
	}
	if ok && identity.Kind == "primary" {
		items, err := s.listOwnerTokens(r.Context(), identity.OwnerID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "token data is unavailable")
			return
		}
		data["Tokens"] = items
		data["LinkedCount"] = max(0, len(items)-1)
		if len(items) > 0 {
			data["PrimaryToken"] = items[0].Masked
		}
		data["OwnerID"] = identity.OwnerID
	}
	if err := s.renderPage(w, r, data); err != nil {
		return
	}
}

func (s *Server) handleTokenWebAction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !sameOriginPreferenceRequest(r) || !s.validCSRFToken(r.PostFormValue("csrf_token"), time.Now()) {
		s.renderStatusError(w, r, http.StatusForbidden)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if ownerTokenStrictAction(path, http.MethodPost) && !s.takeOwnerTokenRateLimit(r) {
		s.renderStatusError(w, r, http.StatusTooManyRequests)
		return
	}
	switch path {
	case "/server/preferences/token/create":
		if _, ok := s.ownerRequestIdentity(r); ok {
			s.renderStatusError(w, r, http.StatusConflict)
			return
		} else if token, ownerID, prefix, err := s.createOwnerPrimary(r.Context()); err == nil {
			s.setOwnerTokenCookie(w, r, token)
			s.auditOwnerToken(r, "token.created", ownerID, prefix, "issue")
			s.renderIssuedPrimary(w, r, token, ownerID, prefix)
			return
		} else {
			s.renderStatusError(w, r, http.StatusServiceUnavailable)
			return
		}
	case "/server/preferences/token/restore":
		token := strings.TrimSpace(r.PostFormValue("token"))
		identity, ok := s.resolveOwnerToken(r.Context(), token)
		if !ok || identity.Kind != "primary" {
			http.Redirect(w, r, "/server/preferences/tokens?error=restore", http.StatusSeeOther)
			return
		}
		s.setOwnerTokenCookie(w, r, token)
		s.auditOwnerToken(r, "token.restored", identity.OwnerID, identity.Prefix, "restore")
		http.Redirect(w, r, "/server/preferences/tokens", http.StatusSeeOther)
		return
	case "/server/preferences/token/forget":
		if _, ok := s.ownerCookieIdentity(w, r); !ok {
			http.Redirect(w, r, "/server/preferences/tokens", http.StatusSeeOther)
			return
		}
		s.clearOwnerTokenCookie(w, r)
		w.Header().Set("Clear-Site-Data", `"storage"`)
		http.Redirect(w, r, "/server/preferences/tokens", http.StatusSeeOther)
		return
	case "/server/preferences/token/reveal", "/server/preferences/token/rotate",
		"/server/preferences/tokens", "/server/preferences/tokens/primary":
		s.runTokenManagementAction(w, r, nil, path)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/server/preferences/tokens/"), "/")
	if len(parts) == 2 {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id < 1 {
			s.renderStatusError(w, r, http.StatusNotFound)
			return
		}
		s.runTokenManagementAction(w, r, &id, path)
		return
	}
	s.renderStatusError(w, r, http.StatusNotFound)
}

func (s *Server) handleTokensAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	path := strings.TrimSuffix(r.URL.Path, "/")
	if ownerTokenStrictAction(path, r.Method) && !s.takeOwnerTokenRateLimit(r) {
		writeProblem(w, http.StatusTooManyRequests, "too many token requests")
		return
	}
	if path == "/api/v1/server/preferences/tokens/primary" && r.Method == http.MethodPost {
		if _, ok := s.ownerBearerIdentity(r); ok || isOperatorBearer(r, s.cfg.Server.Token) {
			writeProblem(w, http.StatusConflict, "a valid token is already present")
			return
		}
		token, ownerID, prefix, err := s.createOwnerPrimary(r.Context())
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "token issuance is unavailable")
			return
		}
		s.auditOwnerToken(r, "token.created", ownerID, prefix, "issue")
		respondJSON(w, http.StatusCreated, map[string]string{"owner_token": token})
		return
	}
	if path == "/api/v1/server/preferences/tokens" {
		if r.Method == http.MethodGet {
			identity, ok := s.ownerBearerIdentity(r)
			if !ok || identity.Kind != "primary" {
				s.writeTokenAuthError(w, r, identity, ok)
				return
			}
			items, err := s.listOwnerTokens(r.Context(), identity.OwnerID)
			if err != nil {
				writeProblem(w, http.StatusServiceUnavailable, "token data is unavailable")
				return
			}
			respondJSON(w, http.StatusOK, map[string]interface{}{"tokens": items})
			return
		}
		if r.Method == http.MethodPost {
			identity, ok := s.ownerBearerIdentity(r)
			if !ok || identity.Kind != "primary" {
				s.writeTokenAuthError(w, r, identity, ok)
				return
			}
			var input struct {
				Name   string `json:"name"`
				Expiry string `json:"expiry"`
			}
			if err := decodeOwnerTokenJSON(w, r, &input); err != nil {
				writeProblem(w, http.StatusBadRequest, "a valid token request is required")
				return
			}
			token, item, err := s.createLinkedToken(r.Context(), identity.OwnerID, strings.TrimSpace(input.Name), input.Expiry)
			if err != nil {
				writeProblem(w, ownerTokenStatus(err), ownerTokenMessage(err))
				return
			}
			s.auditOwnerToken(r, "token.created", identity.OwnerID, item.Prefix, "linked")
			respondJSON(w, http.StatusCreated, map[string]interface{}{"token": token, "metadata": item})
			return
		}
		w.Header().Set("Allow", "GET, POST")
		writeProblem(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/server/preferences/tokens/"), "/")
	if len(parts) == 2 && parts[0] == "primary" {
		s.runTokenAPIPrimaryAction(w, r, parts[1])
		return
	}
	if len(parts) == 2 {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id < 1 {
			writeProblem(w, http.StatusNotFound, "token not found")
			return
		}
		s.runTokenAPILinkedAction(w, r, id, parts[1])
		return
	}
	if len(parts) == 1 {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id < 1 {
			writeProblem(w, http.StatusNotFound, "token not found")
			return
		}
		s.runTokenAPILinkedAction(w, r, id, "")
		return
	}
	writeProblem(w, http.StatusNotFound, "token route not found")
}

func (s *Server) runTokenManagementAction(w http.ResponseWriter, r *http.Request, linkedID *int64, path string) {
	identity, ok := s.ownerCookieIdentity(w, r)
	if !ok || identity.Kind != "primary" {
		http.Redirect(w, r, "/server/preferences/tokens?error=restore", http.StatusSeeOther)
		return
	}
	if linkedID != nil {
		parts := strings.Split(strings.TrimPrefix(path, "/server/preferences/tokens/"), "/")
		if len(parts) != 2 {
			s.renderStatusError(w, r, http.StatusNotFound)
			return
		}
		s.runLinkedWebAction(w, r, identity, *linkedID, parts[1])
		return
	}
	switch path {
	case "/server/preferences/token/reveal":
		token, err := s.revealOwnerToken(r.Context(), identity.OwnerID, identity.TokenID)
		if err != nil {
			s.renderStatusError(w, r, http.StatusServiceUnavailable)
			return
		}
		s.auditOwnerToken(r, "token.revealed", identity.OwnerID, identity.Prefix, "primary")
		s.renderTokenPageWithSecret(w, r, identity, token, nil)
	case "/server/preferences/token/rotate":
		if r.PostFormValue("confirm") != "yes" {
			s.renderStatusError(w, r, http.StatusBadRequest)
			return
		}
		token, prefix, err := s.rotateOwnerToken(r.Context(), identity.OwnerID, identity.TokenID)
		if err != nil {
			s.renderStatusError(w, r, http.StatusServiceUnavailable)
			return
		}
		s.setOwnerTokenCookie(w, r, token)
		identity.Prefix = prefix
		s.auditOwnerToken(r, "token.rotated", identity.OwnerID, prefix, "primary")
		s.renderTokenPageWithSecret(w, r, identity, token, nil)
	case "/server/preferences/tokens", "/server/preferences/tokens/primary":
		expiry := r.PostFormValue("expiry")
		if custom := r.PostFormValue("custom_expiry"); custom != "" {
			expiry = custom
		}
		token, item, err := s.createLinkedToken(r.Context(), identity.OwnerID, strings.TrimSpace(r.PostFormValue("name")), expiry)
		if err != nil {
			http.Redirect(w, r, "/server/preferences/tokens?error="+url.QueryEscape(ownerTokenMessage(err)), http.StatusSeeOther)
			return
		}
		s.auditOwnerToken(r, "token.created", identity.OwnerID, item.Prefix, "linked")
		s.renderTokenPageWithSecret(w, r, identity, token, &item)
	default:
		s.renderStatusError(w, r, http.StatusNotFound)
	}
}

func (s *Server) runLinkedWebAction(w http.ResponseWriter, r *http.Request, identity ownerTokenIdentity, id int64, action string) {
	switch action {
	case "reveal":
		token, prefix, err := s.revealLinkedToken(r.Context(), identity.OwnerID, id)
		if err != nil {
			s.renderStatusError(w, r, http.StatusNotFound)
			return
		}
		s.auditOwnerToken(r, "token.revealed", identity.OwnerID, prefix, "linked")
		s.renderTokenPageWithSecret(w, r, identity, token, nil)
	case "rotate":
		if r.PostFormValue("confirm") != "yes" {
			s.renderStatusError(w, r, http.StatusBadRequest)
			return
		}
		token, prefix, err := s.rotateLinkedToken(r.Context(), identity.OwnerID, id)
		if err != nil {
			s.renderStatusError(w, r, http.StatusNotFound)
			return
		}
		s.auditOwnerToken(r, "token.rotated", identity.OwnerID, prefix, "linked")
		s.renderTokenPageWithSecret(w, r, identity, token, nil)
	case "update":
		name, expiry := strings.TrimSpace(r.PostFormValue("name")), r.PostFormValue("expiry")
		if custom := r.PostFormValue("custom_expiry"); custom != "" {
			expiry = custom
		}
		if err := s.updateLinkedToken(r.Context(), identity.OwnerID, id, name, expiry); err != nil {
			http.Redirect(w, r, "/server/preferences/tokens?error="+url.QueryEscape(ownerTokenMessage(err))+"#token-"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/server/preferences/tokens#token-"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case "revoke":
		if r.PostFormValue("confirm") != "yes" {
			s.renderStatusError(w, r, http.StatusBadRequest)
			return
		}
		prefix, err := s.revokeLinkedToken(r.Context(), identity.OwnerID, id)
		if err != nil {
			s.renderStatusError(w, r, http.StatusNotFound)
			return
		}
		s.auditOwnerToken(r, "token.revoked", identity.OwnerID, prefix, "owner_revoked")
		http.Redirect(w, r, "/server/preferences/tokens#token-"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	default:
		s.renderStatusError(w, r, http.StatusNotFound)
	}
}

func (s *Server) runTokenAPIPrimaryAction(w http.ResponseWriter, r *http.Request, action string) {
	identity, ok := s.ownerBearerIdentity(r)
	if !ok || identity.Kind != "primary" {
		s.writeTokenAuthError(w, r, identity, ok)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeProblem(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	switch action {
	case "reveal":
		token, err := s.revealOwnerToken(r.Context(), identity.OwnerID, identity.TokenID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "token data is unavailable")
			return
		}
		s.auditOwnerToken(r, "token.revealed", identity.OwnerID, identity.Prefix, "primary")
		respondJSON(w, http.StatusOK, map[string]string{"token": token})
	case "rotate":
		token, prefix, err := s.rotateOwnerToken(r.Context(), identity.OwnerID, identity.TokenID)
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "token rotation is unavailable")
			return
		}
		s.auditOwnerToken(r, "token.rotated", identity.OwnerID, prefix, "primary")
		respondJSON(w, http.StatusOK, map[string]string{"token": token})
	default:
		writeProblem(w, http.StatusNotFound, "token route not found")
	}
}

func (s *Server) runTokenAPILinkedAction(w http.ResponseWriter, r *http.Request, id int64, action string) {
	identity, ok := s.ownerBearerIdentity(r)
	if !ok || identity.Kind != "primary" {
		s.writeTokenAuthError(w, r, identity, ok)
		return
	}
	if r.Method == http.MethodPost {
		switch action {
		case "reveal":
			token, prefix, err := s.revealLinkedToken(r.Context(), identity.OwnerID, id)
			if err != nil {
				writeProblem(w, http.StatusNotFound, "token not found")
				return
			}
			s.auditOwnerToken(r, "token.revealed", identity.OwnerID, prefix, "linked")
			respondJSON(w, http.StatusOK, map[string]string{"token": token})
			return
		case "rotate":
			token, prefix, err := s.rotateLinkedToken(r.Context(), identity.OwnerID, id)
			if err != nil {
				writeProblem(w, http.StatusNotFound, "token not found")
				return
			}
			s.auditOwnerToken(r, "token.rotated", identity.OwnerID, prefix, "linked")
			respondJSON(w, http.StatusOK, map[string]string{"token": token})
			return
		}
	}
	if r.Method == http.MethodPatch && action == "" {
		var input struct {
			Name   string `json:"name"`
			Expiry string `json:"expiry"`
		}
		if err := decodeOwnerTokenJSON(w, r, &input); err != nil {
			writeProblem(w, http.StatusBadRequest, "a valid token update is required")
			return
		}
		if err := s.updateLinkedToken(r.Context(), identity.OwnerID, id, strings.TrimSpace(input.Name), input.Expiry); err != nil {
			writeProblem(w, ownerTokenStatus(err), ownerTokenMessage(err))
			return
		}
		respondJSON(w, http.StatusOK, map[string]bool{"updated": true})
		return
	}
	if r.Method == http.MethodDelete && action == "" {
		prefix, err := s.revokeLinkedToken(r.Context(), identity.OwnerID, id)
		if err != nil {
			writeProblem(w, http.StatusNotFound, "token not found")
			return
		}
		s.auditOwnerToken(r, "token.revoked", identity.OwnerID, prefix, "owner_revoked")
		respondJSON(w, http.StatusOK, map[string]bool{"revoked": true})
		return
	}
	w.Header().Set("Allow", "POST, PATCH, DELETE")
	writeProblem(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *Server) renderTokenPageWithSecret(w http.ResponseWriter, r *http.Request, identity ownerTokenIdentity, token string, created *ownerTokenListItem) {
	items, err := s.listOwnerTokens(r.Context(), identity.OwnerID)
	if err != nil {
		s.renderStatusError(w, r, http.StatusServiceUnavailable)
		return
	}
	primarySecret := strings.HasPrefix(token, "own_") && token[:12] == identity.Prefix
	for index := range items {
		if items[index].Prefix == token[:12] && items[index].Name != "" {
			items[index].RevealedToken = token
		}
	}
	data := map[string]interface{}{
		"Title": "Owner tokens", "Description": "Manage the API tokens associated with this browser.",
		"Page": "preferences-tokens", "ContentTemplate": "tokens-content", "Version": Version,
		"HasOwner": true, "CSRFToken": s.csrfToken(time.Now()), "Tokens": items, "LinkedCount": max(0, len(items)-1),
		"PrimaryToken": primaryTokenMask(items), "SecretToken": token, "PrimarySecret": primarySecret,
		"CreatedToken": created, "OwnerID": identity.OwnerID,
	}
	w.Header().Set("Cache-Control", "no-store")
	if err := s.renderPage(w, r, data); err != nil {
		return
	}
}

func (s *Server) renderIssuedPrimary(w http.ResponseWriter, r *http.Request, token, ownerID, prefix string) {
	identity := ownerTokenIdentity{OwnerID: ownerID, Prefix: prefix, Kind: "primary"}
	items, _ := s.listOwnerTokens(r.Context(), ownerID)
	data := map[string]interface{}{
		"Title": "Owner tokens", "Description": "Manage the API tokens associated with this browser.",
		"Page": "preferences-tokens", "ContentTemplate": "tokens-content", "Version": Version,
		"HasOwner": true, "CSRFToken": s.csrfToken(time.Now()), "Tokens": items, "LinkedCount": max(0, len(items)-1),
		"PrimaryToken": primaryTokenMask(items), "SecretToken": token,
		"PrimarySecret": true,
		"OwnerID":       ownerID,
	}
	w.Header().Set("Cache-Control", "no-store")
	if err := s.renderPage(w, r, data); err != nil {
		return
	}
	_ = identity
}

func (s *Server) ownerCookieIdentity(w http.ResponseWriter, r *http.Request) (ownerTokenIdentity, bool) {
	cookie, err := r.Cookie(ownerTokenCookieName)
	if err != nil || cookie.Value == "" {
		return ownerTokenIdentity{}, false
	}
	identity, ok := s.resolveOwnerToken(r.Context(), cookie.Value)
	if !ok || identity.Kind != "primary" {
		s.clearOwnerTokenCookie(w, r)
		return ownerTokenIdentity{}, false
	}
	s.setOwnerTokenCookie(w, r, cookie.Value)
	return identity, true
}

func (s *Server) ownerBearerIdentity(r *http.Request) (ownerTokenIdentity, bool) {
	return s.resolveOwnerToken(r.Context(), bearerValue(r.Header.Get("Authorization")))
}

func (s *Server) ownerRequestIdentity(r *http.Request) (ownerTokenIdentity, bool) {
	if token := bearerValue(r.Header.Get("Authorization")); token != "" {
		return s.resolveOwnerToken(r.Context(), token)
	}
	cookie, err := r.Cookie(ownerTokenCookieName)
	if err != nil {
		return ownerTokenIdentity{}, false
	}
	return s.resolveOwnerToken(r.Context(), cookie.Value)
}

func (s *Server) resolveOwnerToken(ctx context.Context, token string) (ownerTokenIdentity, bool) {
	if s.database == nil || s.database.SQL == nil || !validOwnerTokenFormat(token) {
		return ownerTokenIdentity{}, false
	}
	hash := sha256.Sum256([]byte(token))
	var identity ownerTokenIdentity
	var storedHash string
	var revoked sql.NullInt64
	var expires sql.NullInt64
	err := s.database.SQL.QueryRowContext(ctx, `SELECT owner_id, id, kind, token_prefix, token_hash, revoked_at, expires_at FROM api_tokens WHERE token_hash = ? AND revoked_at IS NULL`, hex.EncodeToString(hash[:])).Scan(&identity.OwnerID, &identity.TokenID, &identity.Kind, &identity.Prefix, &storedHash, &revoked, &expires)
	if err != nil || subtle.ConstantTimeCompare([]byte(storedHash), []byte(hex.EncodeToString(hash[:]))) != 1 || revoked.Valid || expires.Valid && expires.Int64 <= time.Now().Unix() {
		return ownerTokenIdentity{}, false
	}
	identity.Prefix = token[:12]
	_, _ = s.database.SQL.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = CASE WHEN last_used_at IS NULL OR last_used_at < ? THEN ? ELSE last_used_at END WHERE id = ?`, time.Now().Unix()-60, time.Now().Unix(), identity.TokenID)
	_, _ = s.database.SQL.ExecContext(ctx, `UPDATE owners SET last_seen_at = ? WHERE id = ?`, time.Now().Unix(), identity.OwnerID)
	return identity, true
}

func bearerValue(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func validOwnerTokenFormat(token string) bool {
	if len(token) != 36 || !strings.HasPrefix(token, "own_") {
		return false
	}
	for _, char := range token[4:] {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func generateOwnerToken() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var output strings.Builder
	output.Grow(36)
	output.WriteString("own_")
	limit := big.NewInt(int64(len(alphabet)))
	for range 32 {
		value, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		output.WriteByte(alphabet[value.Int64()])
	}
	return output.String(), nil
}

func (s *Server) createOwnerPrimary(ctx context.Context) (string, string, string, error) {
	if s.database == nil || s.database.SQL == nil {
		return "", "", "", fmt.Errorf("database unavailable")
	}
	token, err := generateOwnerToken()
	if err != nil {
		return "", "", "", err
	}
	ownerBytes := make([]byte, 16)
	if _, err := rand.Read(ownerBytes); err != nil {
		return "", "", "", err
	}
	ownerID := hex.EncodeToString(ownerBytes)
	enc, err := s.encryptOwnerToken(token)
	if err != nil {
		return "", "", "", err
	}
	hash := sha256.Sum256([]byte(token))
	prefix := token[:12]
	tx, err := s.database.SQL.BeginTx(ctx, nil)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO owners (id, created_at, last_seen_at) VALUES (?, ?, ?)`, ownerID, time.Now().Unix(), time.Now().Unix()); err != nil {
		return "", "", "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO api_tokens (owner_id, kind, name, token_hash, token_prefix, token_enc, created_at) VALUES (?, 'primary', NULL, ?, ?, ?, ?)`, ownerID, hex.EncodeToString(hash[:]), prefix, enc, time.Now().Unix()); err != nil {
		return "", "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", "", err
	}
	return token, ownerID, prefix, nil
}

func (s *Server) encryptOwnerToken(token string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(s.cfg.Server.Security.EncryptionKey)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("owner token encryption key is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(append(nonce, gcm.Seal(nil, nonce, []byte(token), nil)...)), nil
}

func (s *Server) decryptOwnerToken(value string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(s.cfg.Server.Security.EncryptionKey)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("owner token encryption key is unavailable")
	}
	sealed, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted token is truncated")
	}
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	return string(plain), err
}

func (s *Server) setOwnerTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	maxAge := ownerTokenCookieMaxAge(s.cfg.Server.Tokens.CookieMaxAge)
	http.SetCookie(w, &http.Cookie{Name: ownerTokenCookieName, Value: token, Path: "/", MaxAge: int(maxAge.Seconds()), Expires: time.Now().Add(maxAge), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func ownerTokenCookieMaxAge(value string) time.Duration {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "y") {
		years, err := strconv.Atoi(strings.TrimSuffix(value, "y"))
		if err == nil && years > 0 && years <= 20 {
			return time.Duration(years) * 365 * 24 * time.Hour
		}
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err == nil && days > 0 && days <= 7300 {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	if duration, err := time.ParseDuration(value); err == nil && duration > 0 && duration <= 20*365*24*time.Hour {
		return duration
	}
	return 365 * 24 * time.Hour
}

func (s *Server) clearOwnerTokenCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: ownerTokenCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func (s *Server) listOwnerTokens(ctx context.Context, ownerID string) ([]ownerTokenListItem, error) {
	rows, err := s.database.SQL.QueryContext(ctx, `SELECT id, COALESCE(name, ''), token_prefix, token_enc, created_at, last_used_at, expires_at, revoked_at FROM api_tokens WHERE owner_id = ? ORDER BY kind = 'linked', created_at, id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer closeAndLog(rows, "owner token rows")
	items := make([]ownerTokenListItem, 0)
	for rows.Next() {
		var item ownerTokenListItem
		var encrypted string
		if err := rows.Scan(&item.ID, &item.Name, &item.Prefix, &encrypted, &item.CreatedAt, &item.LastUsed, &item.ExpiresAt, &item.RevokedAt); err != nil {
			return nil, err
		}
		token, err := s.decryptOwnerToken(encrypted)
		if err != nil || !validOwnerTokenFormat(token) {
			return nil, fmt.Errorf("stored owner token cannot be decrypted")
		}
		item.Masked = maskOwnerToken(token)
		item.Status = "active"
		item.CreatedDisplay = time.Unix(item.CreatedAt, 0).UTC().Format("2006-01-02 15:04 UTC")
		if item.LastUsed.Valid {
			lastUsed := item.LastUsed.Int64
			item.LastUsedAt = &lastUsed
			item.LastUsedDisplay = time.Unix(lastUsed, 0).UTC().Format("2006-01-02 15:04 UTC")
		} else {
			item.LastUsedDisplay = "Never"
		}
		if item.ExpiresAt.Valid {
			expiresAt := item.ExpiresAt.Int64
			item.ExpiresAtValue = &expiresAt
			item.ExpiresDisplay = time.Unix(expiresAt, 0).UTC().Format("2006-01-02 15:04 UTC")
		} else {
			item.ExpiresDisplay = "Never"
		}
		if item.RevokedAt.Valid {
			item.Status = "revoked"
		} else if item.ExpiresAt.Valid && item.ExpiresAt.Int64 <= time.Now().Unix() {
			item.Status = "expired"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func maskOwnerToken(token string) string {
	if len(token) < 4 {
		return "own_••••••••••••••••••••••••"
	}
	return "own_••••••••••••••••••••••••" + token[len(token)-4:]
}

func primaryTokenMask(items []ownerTokenListItem) string {
	if len(items) == 0 {
		return "own_••••••••••••••••••••••••"
	}
	return items[0].Masked
}

func (s *Server) revealOwnerToken(ctx context.Context, ownerID string, tokenID int64) (string, error) {
	var encrypted string
	if err := s.database.SQL.QueryRowContext(ctx, `SELECT token_enc FROM api_tokens WHERE owner_id = ? AND id = ? AND kind = 'primary' AND revoked_at IS NULL`, ownerID, tokenID).Scan(&encrypted); err != nil {
		return "", err
	}
	return s.decryptOwnerToken(encrypted)
}

func (s *Server) rotateOwnerToken(ctx context.Context, ownerID string, tokenID int64) (string, string, error) {
	token, err := generateOwnerToken()
	if err != nil {
		return "", "", err
	}
	enc, err := s.encryptOwnerToken(token)
	if err != nil {
		return "", "", err
	}
	hash := sha256.Sum256([]byte(token))
	prefix := token[:12]
	result, err := s.database.SQL.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ?, token_prefix = ?, token_enc = ?, rotated_at = ? WHERE owner_id = ? AND id = ? AND kind = 'primary' AND revoked_at IS NULL`, hex.EncodeToString(hash[:]), prefix, enc, time.Now().Unix(), ownerID, tokenID)
	if err != nil {
		return "", "", err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return "", "", sql.ErrNoRows
	}
	return token, prefix, nil
}

func (s *Server) createLinkedToken(ctx context.Context, ownerID, name, expiry string) (string, ownerTokenListItem, error) {
	if len(name) < 1 || len(name) > 64 || strings.TrimSpace(name) != name {
		return "", ownerTokenListItem{}, errOwnerTokenName
	}
	expiresAt, err := parseOwnerTokenExpiry(expiry)
	if err != nil {
		return "", ownerTokenListItem{}, err
	}
	var count int
	if err := s.database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`, ownerID, time.Now().Unix()).Scan(&count); err != nil {
		return "", ownerTokenListItem{}, err
	}
	maxLinked := s.cfg.Server.Tokens.MaxLinked
	if maxLinked <= 0 {
		maxLinked = 25
	}
	if count >= maxLinked {
		return "", ownerTokenListItem{}, errOwnerTokenLimit
	}
	var duplicate int
	if err := s.database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND lower(name) = lower(?)`, ownerID, name).Scan(&duplicate); err != nil {
		return "", ownerTokenListItem{}, err
	}
	if duplicate > 0 {
		return "", ownerTokenListItem{}, errOwnerTokenDuplicate
	}
	token, err := generateOwnerToken()
	if err != nil {
		return "", ownerTokenListItem{}, err
	}
	enc, err := s.encryptOwnerToken(token)
	if err != nil {
		return "", ownerTokenListItem{}, err
	}
	hash := sha256.Sum256([]byte(token))
	now := time.Now().Unix()
	var expires interface{}
	if expiresAt != 0 {
		expires = expiresAt
	}
	result, err := s.database.SQL.ExecContext(ctx, `INSERT INTO api_tokens (owner_id, kind, name, token_hash, token_prefix, token_enc, created_at, expires_at) VALUES (?, 'linked', ?, ?, ?, ?, ?, ?)`, ownerID, name, hex.EncodeToString(hash[:]), token[:12], enc, now, expires)
	if err != nil {
		return "", ownerTokenListItem{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", ownerTokenListItem{}, err
	}
	item := ownerTokenListItem{ID: id, Name: name, Prefix: token[:12], Masked: maskOwnerToken(token), CreatedAt: now, CreatedDisplay: time.Unix(now, 0).UTC().Format("2006-01-02 15:04 UTC"), LastUsedDisplay: "Never", ExpiresDisplay: "Never", Status: "active"}
	if expiresAt != 0 {
		item.ExpiresAt = sql.NullInt64{Int64: expiresAt, Valid: true}
		item.ExpiresAtValue = &expiresAt
		item.ExpiresDisplay = time.Unix(expiresAt, 0).UTC().Format("2006-01-02 15:04 UTC")
	}
	return token, item, nil
}

func parseOwnerTokenExpiry(value string) (int64, error) {
	now := time.Now()
	switch value {
	case "", "never":
		return 0, nil
	case "7d":
		return now.Add(7 * 24 * time.Hour).Unix(), nil
	case "30d":
		return now.Add(30 * 24 * time.Hour).Unix(), nil
	case "90d":
		return now.Add(90 * 24 * time.Hour).Unix(), nil
	case "1y":
		return now.AddDate(1, 0, 0).Unix(), nil
	}
	if day, err := time.Parse("2006-01-02", value); err == nil {
		end := day.Add(24*time.Hour - time.Second)
		if end.After(now) {
			return end.Unix(), nil
		}
	}
	return 0, errOwnerTokenExpiry
}

func (s *Server) revealLinkedToken(ctx context.Context, ownerID string, id int64) (string, string, error) {
	var encrypted, prefix string
	if err := s.database.SQL.QueryRowContext(ctx, `SELECT token_enc, token_prefix FROM api_tokens WHERE owner_id = ? AND id = ? AND kind = 'linked' AND revoked_at IS NULL`, ownerID, id).Scan(&encrypted, &prefix); err != nil {
		return "", "", err
	}
	token, err := s.decryptOwnerToken(encrypted)
	return token, prefix, err
}

func (s *Server) rotateLinkedToken(ctx context.Context, ownerID string, id int64) (string, string, error) {
	token, err := generateOwnerToken()
	if err != nil {
		return "", "", err
	}
	enc, err := s.encryptOwnerToken(token)
	if err != nil {
		return "", "", err
	}
	hash := sha256.Sum256([]byte(token))
	result, err := s.database.SQL.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ?, token_prefix = ?, token_enc = ?, rotated_at = ? WHERE owner_id = ? AND id = ? AND kind = 'linked' AND revoked_at IS NULL`, hex.EncodeToString(hash[:]), token[:12], enc, time.Now().Unix(), ownerID, id)
	if err != nil {
		return "", "", err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return "", "", sql.ErrNoRows
	}
	return token, token[:12], nil
}

func (s *Server) updateLinkedToken(ctx context.Context, ownerID string, id int64, name, expiry string) error {
	if len(name) < 1 || len(name) > 64 || strings.TrimSpace(name) != name {
		return errOwnerTokenName
	}
	expiresAt, err := parseOwnerTokenExpiry(expiry)
	if err != nil {
		return err
	}
	var duplicate int
	if err := s.database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND lower(name) = lower(?) AND id != ?`, ownerID, name, id).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate > 0 {
		return errOwnerTokenDuplicate
	}
	var expires interface{}
	if expiresAt != 0 {
		expires = expiresAt
	}
	result, err := s.database.SQL.ExecContext(ctx, `UPDATE api_tokens SET name = ?, expires_at = ? WHERE owner_id = ? AND id = ? AND kind = 'linked' AND revoked_at IS NULL`, name, expires, ownerID, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Server) revokeLinkedToken(ctx context.Context, ownerID string, id int64) (string, error) {
	var prefix string
	if err := s.database.SQL.QueryRowContext(ctx, `SELECT token_prefix FROM api_tokens WHERE owner_id = ? AND id = ? AND kind = 'linked' AND revoked_at IS NULL`, ownerID, id).Scan(&prefix); err != nil {
		return "", err
	}
	result, err := s.database.SQL.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ?, revoked_reason = 'owner_revoked' WHERE owner_id = ? AND id = ? AND kind = 'linked' AND revoked_at IS NULL`, time.Now().Unix(), ownerID, id)
	if err != nil {
		return "", err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return "", sql.ErrNoRows
	}
	return prefix, nil
}

func (s *Server) takeOwnerTokenRateLimit(r *http.Request) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	now := time.Now()
	window := time.Duration(s.cfg.Server.RateLimit.Strict.Window) * time.Second
	limit := s.cfg.Server.RateLimit.Strict.Requests
	if window <= 0 {
		window = 900 * time.Second
	}
	if limit <= 0 {
		limit = 10
	}
	ownerTokenRate.Lock()
	defer ownerTokenRate.Unlock()
	for key, state := range ownerTokenRate.clients {
		if now.Sub(state.Started) >= window {
			delete(ownerTokenRate.clients, key)
		}
	}
	state := ownerTokenRate.clients[peer]
	if state.Started.IsZero() || now.Sub(state.Started) >= window {
		state = ownerTokenRateState{Started: now}
	}
	state.Count++
	ownerTokenRate.clients[peer] = state
	return state.Count <= limit
}

func ownerTokenStrictAction(path, method string) bool {
	if method != http.MethodPost {
		return false
	}
	if path == "/server/preferences/token/create" || path == "/server/preferences/token/restore" ||
		path == "/server/preferences/token/reveal" || path == "/server/preferences/token/rotate" ||
		path == "/api/v1/server/preferences/tokens/primary" ||
		path == "/api/v1/server/preferences/tokens/primary/reveal" ||
		path == "/api/v1/server/preferences/tokens/primary/rotate" {
		return true
	}
	for _, prefix := range []string{"/server/preferences/tokens/", "/api/v1/server/preferences/tokens/"} {
		if strings.HasSuffix(path, "/reveal") || strings.HasSuffix(path, "/rotate") {
			return strings.HasPrefix(path, prefix)
		}
	}
	return false
}

func (s *Server) auditOwnerToken(r *http.Request, action, ownerID, prefix, source string) {
	if s.database == nil || s.database.SQL == nil {
		return
	}
	details, _ := json.Marshal(map[string]string{"owner_id": ownerID, "token_prefix": prefix, "source": source})
	_, _ = s.database.SQL.ExecContext(r.Context(), `INSERT INTO audit_log (timestamp, level, category, action, actor_ip, target_type, target_id, details, success) VALUES (?, 'info', 'token', ?, ?, 'owner', ?, ?, 1)`, time.Now().Unix(), action, clientIP(r), ownerID, string(details))
}

func clientIP(r *http.Request) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return peer
	}
	return r.RemoteAddr
}

func (s *Server) writeTokenAuthError(w http.ResponseWriter, r *http.Request, identity ownerTokenIdentity, ok bool) {
	if !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="anime"`)
		writeProblem(w, http.StatusUnauthorized, "a valid primary owner token is required")
		return
	}
	if identity.Kind != "primary" {
		writeProblem(w, http.StatusForbidden, "primary token required")
		return
	}
	writeProblem(w, http.StatusUnauthorized, "a valid primary owner token is required")
}

func isOperatorBearer(r *http.Request, configured string) bool {
	token := bearerValue(r.Header.Get("Authorization"))
	if token == "" || configured == "" {
		return false
	}
	want, got := sha256.Sum256([]byte(configured)), sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func (s *Server) mcpOwnerIdentity(r *http.Request) (ownerTokenIdentity, bool) {
	if isOperatorBearer(r, s.cfg.Server.Token) {
		return ownerTokenIdentity{Kind: "operator"}, true
	}
	return s.ownerBearerIdentity(r)
}

func (s *Server) writeMCPUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="anime"`)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": nil, "error": map[string]interface{}{"code": -32001, "message": "A valid Bearer token is required"}})
}

func decodeOwnerTokenJSON(w http.ResponseWriter, r *http.Request, target interface{}) error {
	if !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return fmt.Errorf("expected application/json")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	}
	return fmt.Errorf("request must contain one JSON value")
}

var (
	errOwnerTokenName      = fmt.Errorf("token name must contain 1 to 64 characters")
	errOwnerTokenLimit     = fmt.Errorf("active linked token limit reached")
	errOwnerTokenDuplicate = fmt.Errorf("token name is already used")
	errOwnerTokenExpiry    = fmt.Errorf("expiry must be never, 7d, 30d, 90d, 1y, or a future date")
)

func ownerTokenStatus(err error) int {
	switch err {
	case errOwnerTokenName, errOwnerTokenExpiry, errOwnerTokenDuplicate:
		return http.StatusBadRequest
	case errOwnerTokenLimit:
		return http.StatusConflict
	default:
		return http.StatusServiceUnavailable
	}
}

func ownerTokenMessage(err error) string {
	if err == nil {
		return "token request failed"
	}
	if err == errOwnerTokenName || err == errOwnerTokenExpiry || err == errOwnerTokenDuplicate || err == errOwnerTokenLimit {
		return err.Error()
	}
	return "token request failed"
}
