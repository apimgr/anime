package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	openpgp "github.com/ProtonMail/go-crypto/openpgp/v2"
	"github.com/apimgr/anime/src/config"
)

func TestSecurityPGPKeyGenerationAndReportEncryption(t *testing.T) {
	configDir := t.TempDir()
	secret := bytes.Repeat([]byte{0x42}, 32)
	fingerprint, err := GenerateSecurityPGPKey(configDir, secret, "Anime Security", "security@example.test", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(fingerprint) != 40 {
		t.Fatalf("fingerprint length=%d, want 40", len(fingerprint))
	}
	publicPath := filepath.Join(configDir, "security", "pgp.pub.asc")
	privatePath := filepath.Join(configDir, "security", pgpPrivateKeyFile)
	public, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(public))
	if err != nil || len(entities) != 1 {
		t.Fatalf("public key parse entities=%d err=%v", len(entities), err)
	}
	publicFingerprint, err := InspectSecurityPGPPublicKey(public)
	if err != nil || publicFingerprint != fingerprint {
		t.Fatalf("public fingerprint=%q err=%v", publicFingerprint, err)
	}
	imported, err := FetchResearcherPGPKey(string(public))
	if err != nil || !bytes.Equal(imported, public) {
		t.Fatalf("inline researcher key import failed: %v", err)
	}
	private, err := DecryptSecurityPGPPrivateKey(privatePath, secret)
	if err != nil {
		t.Fatal(err)
	}
	signers, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(private))
	if err != nil || len(signers) != 1 {
		t.Fatalf("private key parse entities=%d err=%v", len(signers), err)
	}
	message := []byte("encrypted security notice")
	encryptedMessage, err := EncryptToSecurityPGP(public, message, signers[0])
	if err != nil {
		t.Fatal(err)
	}
	decryptedMessage, err := decryptOpenPGP(encryptedMessage, signers[0])
	if err != nil || !bytes.Equal(decryptedMessage, message) {
		t.Fatalf("decrypted PGP message=%q err=%v", decryptedMessage, err)
	}
	info, err := os.Stat(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("private key file mode=%#o, want 0600", info.Mode().Perm())
	}
	server := &Server{cfg: &config.Config{ConfigDir: configDir}, installationSecret: secret}
	body, algorithm, err := server.encryptSecurityReport([]byte("private report text"))
	if err != nil || algorithm != "pgp" || bytes.Contains(body, []byte("private report text")) {
		t.Fatalf("report encryption algorithm=%q err=%v plaintext_present=%t", algorithm, err, bytes.Contains(body, []byte("private report text")))
	}
}

func TestSecurityPGPPrivateKeyIsEncryptedAtRest(t *testing.T) {
	configDir := t.TempDir()
	secret := bytes.Repeat([]byte{0x21}, 32)
	if _, err := GenerateSecurityPGPKey(configDir, secret, "Anime Security", "security@example.test", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "security", pgpPrivateKeyFile)
	if _, err := DecryptSecurityPGPPrivateKey(path, bytes.Repeat([]byte{0x22}, 32)); err == nil {
		t.Fatal("private key decrypted with an unrelated installation secret")
	}
	private, err := DecryptSecurityPGPPrivateKey(path, secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(private, secret) || !bytes.HasPrefix(private, []byte("-----BEGIN PGP PRIVATE KEY BLOCK-----")) {
		t.Fatal("decrypted private key did not contain armored OpenPGP key material")
	}
}

func TestSecurityPGPRotationReencryptsStoredReports(t *testing.T) {
	s := newTestServer(t)
	configDir := t.TempDir()
	s.cfg.ConfigDir = configDir
	oldFingerprint, err := GenerateSecurityPGPKey(configDir, s.installationSecret, "Anime Security", "security@example.test", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.database.SQL.Exec(`INSERT INTO security_pgp_metadata (id, fingerprint, created_at, expires_at, revoked) VALUES (1, ?, ?, ?, 0)`, oldFingerprint, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().AddDate(2, 0, 0).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, algorithm, err := s.encryptSecurityReport([]byte("retained confidential report"))
	if err != nil || algorithm != "pgp" {
		t.Fatalf("encrypt report algorithm=%q err=%v", algorithm, err)
	}
	_, err = s.database.SQL.Exec(`INSERT INTO security_reports (tracking_id, security_id, severity, component, body_enc, body_alg, token_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sec_0123456789abcdef", "id", "high", "api", encrypted, algorithm, strings.Repeat("0", 64), time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	newFingerprint, err := RotateSecurityPGPKey(configDir, s.installationSecret, s.database.SQL, "Anime Security", "security@example.test", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if newFingerprint == oldFingerprint {
		t.Fatal("rotation kept the previous fingerprint")
	}
	private, err := DecryptSecurityPGPPrivateKey(filepath.Join(configDir, "security", pgpPrivateKeyFile), s.installationSecret)
	if err != nil {
		t.Fatal(err)
	}
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(private))
	if err != nil || len(entities) != 1 {
		t.Fatalf("rotated private key parse entities=%d err=%v", len(entities), err)
	}
	var rotated []byte
	if err := s.database.SQL.QueryRow(`SELECT body_enc FROM security_reports WHERE tracking_id = ?`, "sec_0123456789abcdef").Scan(&rotated); err != nil {
		t.Fatal(err)
	}
	clear, err := decryptOpenPGP(rotated, entities[0])
	if err != nil || string(clear) != "retained confidential report" {
		t.Fatalf("rotated report cleartext=%q err=%v", clear, err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "security", "pgp.priv.old.asc.enc")); err != nil {
		t.Fatalf("old private key was not retained: %v", err)
	}
}

func TestSecurityPGPImportValidatesIdentityAndStoresEncryptedKey(t *testing.T) {
	secret := bytes.Repeat([]byte{0x63}, 32)
	sourceDir := t.TempDir()
	if _, err := GenerateSecurityPGPKey(sourceDir, secret, "Anime Security", "security@example.test", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	private, err := DecryptSecurityPGPPrivateKey(filepath.Join(sourceDir, "security", pgpPrivateKeyFile), secret)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, identity, err := InspectSecurityPGPPrivateKey(private)
	if err != nil || identity != "security@example.test" || len(fingerprint) != 40 {
		t.Fatalf("inspect identity=%q fingerprint=%q err=%v", identity, fingerprint, err)
	}
	destination := t.TempDir()
	importedFingerprint, importedIdentity, err := ImportSecurityPGPPrivateKey(destination, secret, private)
	if err != nil || importedFingerprint != fingerprint || importedIdentity != identity {
		t.Fatalf("import fingerprint=%q identity=%q err=%v", importedFingerprint, importedIdentity, err)
	}
	mode, err := os.Stat(filepath.Join(destination, "security", pgpPrivateKeyFile))
	if err != nil || mode.Mode().Perm() != 0600 {
		t.Fatalf("imported private key mode info=%v err=%v", mode, err)
	}
}
