package server

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	openpgp "github.com/ProtonMail/go-crypto/openpgp/v2"
	"github.com/apimgr/anime/src/config"
)

const pgpPrivateKeyFile = "pgp.priv.asc.enc"

var securityPGPUpdateMu sync.Mutex

func securityKeyDirectory(cfgDir string) string {
	return filepath.Join(cfgDir, "security")
}

func (s *Server) securityPublicKey() ([]byte, error) {
	return os.ReadFile(filepath.Join(securityKeyDirectory(s.cfg.ConfigDir), "pgp.pub.asc"))
}

func (s *Server) securityKeyFingerprint() string {
	data, err := s.securityPublicKey()
	if err != nil {
		return ""
	}
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	if err != nil || len(entities) != 1 {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(entities[0].PrimaryKey.Fingerprint))
}

func (s *Server) encryptSecurityReport(body []byte) ([]byte, string, error) {
	publicKey, err := s.securityPublicKey()
	if err == nil {
		entities, parseErr := openpgp.ReadArmoredKeyRing(bytes.NewReader(publicKey))
		if parseErr != nil || len(entities) != 1 {
			return nil, "", fmt.Errorf("configured security public key is invalid")
		}
		if _, privateErr := currentSecuritySignerFromServer(s); privateErr != nil {
			return nil, "", fmt.Errorf("configured security private key is unavailable")
		}
		armored, encryptErr := encryptOpenPGP(entities, body)
		return armored, "pgp", encryptErr
	}
	if !os.IsNotExist(err) {
		return nil, "", err
	}
	encrypted, err := s.encryptReportValue(body)
	return encrypted, "aes-256-gcm", err
}

func GenerateSecurityPGPKey(configDir string, installationSecret []byte, name, email string, now time.Time) (string, error) {
	if len(installationSecret) < 32 {
		return "", fmt.Errorf("installation secret is unavailable")
	}
	directory := securityKeyDirectory(configDir)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(directory, "pgp.pub.asc")); err == nil {
		return "", fmt.Errorf("a security PGP key already exists")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	seconds := uint32((2 * 365 * 24 * time.Hour) / time.Second)
	entity, err := openpgp.NewEntity(name, "", email, &packet.Config{Algorithm: packet.PubKeyAlgoEd25519, KeyLifetimeSecs: seconds, Time: func() time.Time { return now }})
	if err != nil {
		return "", err
	}
	if err := entity.AddEncryptionSubkey(&packet.Config{Algorithm: packet.PubKeyAlgoX25519, KeyLifetimeSecs: seconds, Time: func() time.Time { return now }}); err != nil {
		return "", err
	}
	var public bytes.Buffer
	publicArmor, err := armor.Encode(&public, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", err
	}
	if err := entity.Serialize(publicArmor); err != nil {
		return "", err
	}
	if err := publicArmor.Close(); err != nil {
		return "", err
	}
	var private bytes.Buffer
	privateArmor, err := armor.Encode(&private, openpgp.PrivateKeyType, nil)
	if err != nil {
		return "", err
	}
	if err := entity.SerializePrivate(privateArmor, nil); err != nil {
		return "", err
	}
	if err := privateArmor.Close(); err != nil {
		return "", err
	}
	key, err := derivePGPStorageKey(installationSecret)
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
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := append(nonce, gcm.Seal(nil, nonce, private.Bytes(), []byte("anime-security-pgp-v1"))...)
	if err := writePrivateAtomic(filepath.Join(directory, pgpPrivateKeyFile), sealed); err != nil {
		return "", err
	}
	if err := writePublicAtomic(filepath.Join(directory, "pgp.pub.asc"), public.Bytes()); err != nil {
		_ = os.Remove(filepath.Join(directory, pgpPrivateKeyFile))
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint)), nil
}

func derivePGPStorageKey(secret []byte) ([]byte, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("installation secret is too short")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("anime-security-pgp-private-key-v1"))
	return mac.Sum(nil), nil
}

func DecryptSecurityPGPPrivateKey(path string, installationSecret []byte) ([]byte, error) {
	sealed, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := derivePGPStorageKey(installationSecret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, fmt.Errorf("encrypted private key is truncated")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte("anime-security-pgp-v1"))
}

func ExportSecurityPGPPrivateKey(configDir string, installationSecret []byte, outputPath string) error {
	private, err := DecryptSecurityPGPPrivateKey(filepath.Join(securityKeyDirectory(configDir), pgpPrivateKeyFile), installationSecret)
	if err != nil {
		return err
	}
	return writePrivateAtomic(outputPath, private)
}

func ImportSecurityPGPPrivateKey(configDir string, installationSecret, private []byte) (string, string, error) {
	if len(private) == 0 || len(private) > 1<<20 {
		return "", "", fmt.Errorf("private key file is empty or too large")
	}
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(private))
	if err != nil || len(entities) != 1 {
		return "", "", fmt.Errorf("private key file is invalid")
	}
	entity := entities[0]
	var identity string
	for _, candidate := range entity.Identities {
		if candidate != nil && candidate.UserId != nil {
			identity = candidate.UserId.Email
			if identity != "" {
				break
			}
		}
	}
	if identity == "" {
		return "", "", fmt.Errorf("private key has no email identity")
	}
	var public bytes.Buffer
	writer, err := armor.Encode(&public, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", "", err
	}
	if err := entity.Serialize(writer); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}
	key, err := derivePGPStorageKey(installationSecret)
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	sealed := append(nonce, gcm.Seal(nil, nonce, private, []byte("anime-security-pgp-v1"))...)
	directory := securityKeyDirectory(configDir)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", "", err
	}
	if err := writePrivateAtomic(filepath.Join(directory, pgpPrivateKeyFile), sealed); err != nil {
		return "", "", err
	}
	if err := writePublicAtomic(filepath.Join(directory, "pgp.pub.asc"), public.Bytes()); err != nil {
		_ = os.Remove(filepath.Join(directory, pgpPrivateKeyFile))
		return "", "", err
	}
	return strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint)), identity, nil
}

func InspectSecurityPGPPrivateKey(private []byte) (string, string, error) {
	if len(private) == 0 || len(private) > 1<<20 {
		return "", "", fmt.Errorf("private key file is empty or too large")
	}
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(private))
	if err != nil || len(entities) != 1 {
		return "", "", fmt.Errorf("private key file is invalid")
	}
	identity := ""
	for _, candidate := range entities[0].Identities {
		if candidate != nil && candidate.UserId != nil && candidate.UserId.Email != "" {
			identity = candidate.UserId.Email
			break
		}
	}
	if identity == "" {
		return "", "", fmt.Errorf("private key has no email identity")
	}
	return strings.ToUpper(hex.EncodeToString(entities[0].PrimaryKey.Fingerprint)), identity, nil
}

func InspectSecurityPGPPublicKey(public []byte) (string, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(public))
	if err != nil || len(entities) != 1 {
		return "", fmt.Errorf("public key file is invalid")
	}
	return strings.ToUpper(hex.EncodeToString(entities[0].PrimaryKey.Fingerprint)), nil
}

func RotateSecurityPGPKey(configDir string, installationSecret []byte, db *sql.DB, name, email string, now time.Time) (string, error) {
	if db == nil {
		return "", fmt.Errorf("security report database is unavailable")
	}
	securityPGPUpdateMu.Lock()
	defer securityPGPUpdateMu.Unlock()
	directory := securityKeyDirectory(configDir)
	oldPublic, err := os.ReadFile(filepath.Join(directory, "pgp.pub.asc"))
	if err != nil {
		return "", err
	}
	oldPrivateFile := filepath.Join(directory, pgpPrivateKeyFile)
	oldKeyPath := filepath.Join(directory, "pgp.priv.old.asc.enc")
	if _, err := os.Stat(oldKeyPath); err == nil {
		return "", fmt.Errorf("the previous PGP private key is still within its 30-day retention period")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	oldPrivateSealed, err := os.ReadFile(oldPrivateFile)
	if err != nil {
		return "", err
	}
	oldPrivate, err := DecryptSecurityPGPPrivateKey(oldPrivateFile, installationSecret)
	if err != nil {
		return "", err
	}
	oldEntities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(oldPrivate))
	if err != nil || len(oldEntities) != 1 {
		return "", fmt.Errorf("current security private key is invalid")
	}
	temporary, err := os.MkdirTemp(directory, ".rotation-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(temporary); err != nil {
			log.Printf("temporary PGP rotation directory cleanup failed: %v", err)
		}
	}()
	fingerprint, err := GenerateSecurityPGPKey(temporary, installationSecret, name, email, now)
	if err != nil {
		return "", err
	}
	newDirectory := securityKeyDirectory(temporary)
	newPublic, err := os.ReadFile(filepath.Join(newDirectory, "pgp.pub.asc"))
	if err != nil {
		return "", err
	}
	newPrivate, err := DecryptSecurityPGPPrivateKey(filepath.Join(newDirectory, pgpPrivateKeyFile), installationSecret)
	if err != nil {
		return "", err
	}
	newEntities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(newPublic))
	if err != nil || len(newEntities) != 1 {
		return "", fmt.Errorf("new security public key is invalid")
	}
	newSigner, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(newPrivate))
	if err != nil || len(newSigner) != 1 {
		return "", fmt.Errorf("new security private key is invalid")
	}
	rotationSignature, err := signArmored(oldEntities[0], newPublic)
	if err != nil {
		return "", err
	}
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("PGP rotation transaction rollback failed: %v", err)
		}
	}()
	if _, err := tx.Exec(`UPDATE security_pgp_lock SET id = id WHERE id = 1`); err != nil {
		return "", err
	}
	rows, err := tx.Query(`SELECT tracking_id, body_enc FROM security_reports WHERE body_alg = 'pgp'`)
	if err != nil {
		return "", err
	}
	type reportCipher struct {
		id   string
		body []byte
	}
	var reports []reportCipher
	for rows.Next() {
		var report reportCipher
		if err := rows.Scan(&report.id, &report.body); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Printf("PGP rotation report rows close failed: %v", closeErr)
			}
			return "", err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("PGP rotation report rows close failed: %v", closeErr)
		}
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	for _, report := range reports {
		clear, err := decryptOpenPGP(report.body, oldEntities[0])
		if err != nil {
			return "", fmt.Errorf("decrypt report %s: %w", report.id, err)
		}
		reencrypted, err := encryptOpenPGP(newEntities, clear)
		if err != nil {
			return "", err
		}
		if _, err := tx.Exec(`UPDATE security_reports SET body_enc = ? WHERE tracking_id = ?`, reencrypted, report.id); err != nil {
			return "", err
		}
	}
	created := now.UTC()
	metadataResult, err := tx.Exec(`UPDATE security_pgp_metadata SET fingerprint = ?, created_at = ?, expires_at = ?, last_rotated_at = ?, revoked = 0 WHERE id = 1`, fingerprint, created.Format(time.RFC3339), created.AddDate(2, 0, 0).Format(time.RFC3339), created.Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	if count, err := metadataResult.RowsAffected(); err != nil || count != 1 {
		return "", fmt.Errorf("security PGP metadata is missing")
	}
	oldPath := oldKeyPath
	oldPublicPath := filepath.Join(directory, "pgp.pub.old.asc")
	if err := writePublicAtomic(oldPublicPath, oldPublic); err != nil {
		return "", err
	}
	newPrivateSealed, err := os.ReadFile(filepath.Join(newDirectory, pgpPrivateKeyFile))
	if err != nil {
		_ = os.Remove(oldPublicPath)
		return "", err
	}
	if err := writePrivateAtomic(oldPath, oldPrivateSealed); err != nil {
		_ = os.Remove(oldPublicPath)
		return "", err
	}
	if err := writePrivateAtomic(oldPrivateFile, newPrivateSealed); err != nil {
		_ = os.Remove(oldPath)
		_ = os.Remove(oldPublicPath)
		return "", err
	}
	if err := writePublicAtomic(filepath.Join(directory, "pgp.pub.asc"), newPublic); err != nil {
		_ = writePrivateAtomic(oldPrivateFile, oldPrivateSealed)
		_ = os.Remove(oldPath)
		_ = os.Remove(oldPublicPath)
		return "", err
	}
	if err := writePublicAtomic(filepath.Join(directory, "pgp.pub.asc.sig"), rotationSignature); err != nil {
		_ = writePrivateAtomic(oldPrivateFile, oldPrivateSealed)
		_ = writePublicAtomic(filepath.Join(directory, "pgp.pub.asc"), oldPublic)
		_ = os.Remove(oldPath)
		_ = os.Remove(oldPublicPath)
		return "", err
	}
	if err := tx.Commit(); err != nil {
		_ = writePrivateAtomic(oldPrivateFile, oldPrivateSealed)
		_ = writePublicAtomic(filepath.Join(directory, "pgp.pub.asc"), oldPublic)
		_ = os.Remove(filepath.Join(directory, "pgp.pub.asc.sig"))
		_ = os.Remove(oldPath)
		_ = os.Remove(oldPublicPath)
		return "", err
	}
	return fingerprint, nil
}

func signArmored(signer *openpgp.Entity, content []byte) ([]byte, error) {
	var signature bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&signature, []*openpgp.Entity{signer}, bytes.NewReader(content), nil); err != nil {
		return nil, err
	}
	return signature.Bytes(), nil
}

func decryptOpenPGP(encrypted []byte, recipient *openpgp.Entity) ([]byte, error) {
	block, err := armor.Decode(bytes.NewReader(encrypted))
	if err != nil {
		return nil, err
	}
	details, err := openpgp.ReadMessage(block.Body, openpgp.EntityList{recipient}, nil, nil)
	if err != nil {
		return nil, err
	}
	clear, err := io.ReadAll(details.UnverifiedBody)
	if err != nil {
		return nil, err
	}
	if details.SignatureError != nil {
		return nil, details.SignatureError
	}
	return clear, nil
}

func EncryptToSecurityPGP(publicKey, message []byte, signer ...*openpgp.Entity) ([]byte, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(publicKey))
	if err != nil {
		return nil, err
	}
	return encryptOpenPGP(entities, message, signer...)
}

func FetchResearcherPGPKey(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
		if _, err := openpgp.ReadArmoredKeyRing(strings.NewReader(trimmed)); err != nil {
			return nil, fmt.Errorf("researcher public key is invalid")
		}
		return []byte(trimmed), nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || len(trimmed) > 2048 {
		return nil, fmt.Errorf("researcher public key URL must be HTTPS")
	}
	addresses, err := net.LookupIP(parsed.Hostname())
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("researcher public key host is unavailable")
	}
	for _, address := range addresses {
		if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
			return nil, fmt.Errorf("researcher public key host is not public")
		}
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{ServerName: parsed.Hostname(), MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var lastErr error
			for _, address := range addresses {
				connection, dialErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(address.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				lastErr = dialErr
			}
			return nil, lastErr
		},
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	defer transport.CloseIdleConnections()
	response, err := client.Get(parsed.String())
	if err != nil {
		return nil, err
	}
	defer closeAndLog(response.Body, "researcher key response")
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("researcher public key could not be downloaded")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil || len(data) > 16384 {
		return nil, fmt.Errorf("researcher public key is too large")
	}
	if _, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("researcher public key is invalid")
	}
	return data, nil
}

func encryptOpenPGP(entities openpgp.EntityList, message []byte, signers ...*openpgp.Entity) ([]byte, error) {
	var encrypted bytes.Buffer
	armorWriter, err := armor.Encode(&encrypted, openpgp.MessageType, nil)
	if err != nil {
		return nil, err
	}
	var signingEntities []*openpgp.Entity
	for _, signer := range signers {
		if signer != nil {
			signingEntities = append(signingEntities, signer)
		}
	}
	writer, err := openpgp.Encrypt(armorWriter, entities, nil, signingEntities, nil, &packet.Config{DefaultCipher: packet.CipherAES256})
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(writer, bytes.NewReader(message)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if err := armorWriter.Close(); err != nil {
		return nil, err
	}
	return encrypted.Bytes(), nil
}

func currentSecuritySignerFromServer(s *Server) (*openpgp.Entity, error) {
	if s == nil {
		return nil, fmt.Errorf("server is unavailable")
	}
	private, err := DecryptSecurityPGPPrivateKey(filepath.Join(securityKeyDirectory(s.cfg.ConfigDir), pgpPrivateKeyFile), s.installationSecret)
	if err != nil {
		return nil, err
	}
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(private))
	if err != nil || len(entities) != 1 {
		return nil, fmt.Errorf("security private key is invalid")
	}
	return entities[0], nil
}

func (s *Server) signSecurityNotice(message []byte) ([]byte, error) {
	entity, err := currentSecuritySignerFromServer(s)
	if err != nil {
		return nil, err
	}
	var signature bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&signature, []*openpgp.Entity{entity}, bytes.NewReader(message), nil); err != nil {
		return nil, err
	}
	return signature.Bytes(), nil
}

func PublishSecurityPGPKey(ctx context.Context, cfg *config.Config, db *sql.DB) error {
	if cfg == nil || db == nil {
		return fmt.Errorf("security key publishing configuration is unavailable")
	}
	key, err := os.ReadFile(filepath.Join(securityKeyDirectory(cfg.ConfigDir), "pgp.pub.asc"))
	if err != nil {
		return err
	}
	if len(cfg.WebSecurity.Keyservers) == 0 {
		return fmt.Errorf("configure web.security.keyservers before publishing")
	}
	results := make(map[string]string)
	var failures []string
	for _, raw := range cfg.WebSecurity.Keyservers {
		endpoint, parseErr := keyserverSubmissionURL(raw)
		if parseErr != nil {
			failures = append(failures, parseErr.Error())
			continue
		}
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(1<<(attempt-1)) * time.Second)
			}
			if lastErr = submitSecurityPublicKey(endpoint, key); lastErr == nil {
				break
			}
		}
		if lastErr != nil {
			failures = append(failures, endpoint.Host+": "+lastErr.Error())
			log.Printf("security.pgp_publish_failed keyserver=%q", endpoint.Host)
			continue
		}
		results[endpoint.Host] = time.Now().UTC().Format(time.RFC3339)
		log.Printf("security.pgp_published keyserver=%q", endpoint.Host)
	}
	encoded, _ := json.Marshal(results)
	_, err = db.ExecContext(ctx, `UPDATE security_pgp_metadata SET keyservers_published = ? WHERE id = 1`, string(encoded))
	if err != nil {
		return err
	}
	if err := writePrivateAtomic(filepath.Join(securityKeyDirectory(cfg.ConfigDir), "keyservers.state"), encoded); err != nil {
		return err
	}
	if len(failures) > 0 {
		return fmt.Errorf("PGP key publish failed for %d keyserver(s)", len(failures))
	}
	return nil
}

func keyserverSubmissionURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("configured keyserver must be an HTTPS URL")
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "keys.openpgp.org":
		parsed.Path = "/vks/v1/upload"
	case "keyserver.ubuntu.com":
		parsed.Path = "/pks/add"
	default:
		return nil, fmt.Errorf("unsupported OpenPGP keyserver host")
	}
	return parsed, nil
}

func submitSecurityPublicKey(endpoint *url.URL, key []byte) error {
	var request *http.Request
	var err error
	if endpoint.Hostname() == "keys.openpgp.org" {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("keytext", string(key)); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		request, err = http.NewRequest(http.MethodPost, endpoint.String(), &body)
		if err == nil {
			request.Header.Set("Content-Type", writer.FormDataContentType())
		}
	} else {
		form := url.Values{"keytext": {string(key)}}
		request, err = http.NewRequest(http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
		if err == nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer closeAndLog(response.Body, "keyserver response")
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("keyserver returned HTTP %d", response.StatusCode)
	}
	return nil
}

func writePrivateAtomic(path string, contents []byte) error {
	return writeSecurityKeyAtomic(path, contents, 0600)
}

func writePublicAtomic(path string, contents []byte) error {
	return writeSecurityKeyAtomic(path, contents, 0644)
}

func writeSecurityKeyAtomic(path string, contents []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".pgp-key-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() {
		if err := os.Remove(temporary); err != nil && !os.IsNotExist(err) {
			log.Printf("temporary PGP key file cleanup failed: %v", err)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer closeAndLog(dir, "PGP key directory")
	return dir.Sync()
}
