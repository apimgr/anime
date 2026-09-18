package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCertPEM creates a self-signed certificate/key pair for tests.
func generateTestCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.com"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM
}

func okHandlerSSL(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestGetTLSConfigDisabled(t *testing.T) {
	m := NewManager(Config{Enabled: false})
	cfg, err := m.GetTLSConfig([]string{"example.com"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil TLS config when disabled, got %v", cfg)
	}
}

func TestGetTLSConfigNoCertsAvailable(t *testing.T) {
	m := NewManager(Config{Enabled: true})
	_, err := m.GetTLSConfig([]string{"example.com"})
	if err == nil {
		t.Fatal("expected error when no certificates available and Let's Encrypt disabled")
	}
}

func TestGetTLSConfigManualCerts(t *testing.T) {
	dir := t.TempDir()
	domain := "example.com"
	certPEM, keyPEM := generateTestCertPEM(t)

	if err := os.WriteFile(filepath.Join(dir, domain+".crt"), certPEM, 0644); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, domain+".key"), keyPEM, 0644); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	m := NewManager(Config{Enabled: true, CertPath: dir})
	cfg, err := m.GetTLSConfig([]string{domain})
	if err != nil {
		t.Fatalf("expected no error loading manual cert, got %v", err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate loaded, got %+v", cfg)
	}
}

func TestGetTLSConfigManualCertsCorrupt(t *testing.T) {
	dir := t.TempDir()
	domain := "broken.example.com"

	if err := os.WriteFile(filepath.Join(dir, domain+".crt"), []byte("not a cert"), 0644); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, domain+".key"), []byte("not a key"), 0644); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	m := NewManager(Config{Enabled: true, CertPath: dir})
	if _, err := m.GetTLSConfig([]string{domain}); err == nil {
		t.Fatal("expected error loading corrupt manual certificate")
	}
}

func TestFindManualCertsFullchainFormat(t *testing.T) {
	dir := t.TempDir()
	domain := "example.org"
	domainDir := filepath.Join(dir, domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		t.Fatalf("failed to create domain dir: %v", err)
	}
	certPEM, keyPEM := generateTestCertPEM(t)
	if err := os.WriteFile(filepath.Join(domainDir, "fullchain.pem"), certPEM, 0644); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "privkey.pem"), keyPEM, 0644); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	m := &Manager{config: Config{CertPath: dir}}
	cert, key := m.findManualCerts([]string{domain})
	if cert == "" || key == "" {
		t.Fatal("expected fullchain-format certificate to be found")
	}
}

func TestFindManualCertsMissing(t *testing.T) {
	m := &Manager{config: Config{CertPath: t.TempDir()}}
	cert, key := m.findManualCerts([]string{"missing.example.com"})
	if cert != "" || key != "" {
		t.Errorf("expected no certificate found, got cert=%q key=%q", cert, key)
	}
}

func TestGetTLSConfigLetsEncrypt(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Config{
		Enabled:     true,
		CertPath:    dir,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Email: "admin@example.com"},
	})

	cfg, err := m.GetTLSConfig([]string{"example.com"})
	if err != nil {
		t.Fatalf("expected no error configuring Let's Encrypt, got %v", err)
	}
	if cfg == nil || cfg.GetCertificate == nil {
		t.Fatal("expected autocert TLS config with GetCertificate callback")
	}

	if _, err := os.Stat(filepath.Join(dir, "autocert")); err != nil {
		t.Errorf("expected autocert cache dir to be created: %v", err)
	}
}

func TestGetTLSConfigLetsEncryptCacheDirError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}

	m := NewManager(Config{
		Enabled:     true,
		CertPath:    blocker,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Email: "admin@example.com"},
	})

	if _, err := m.GetTLSConfig([]string{"example.com"}); err == nil {
		t.Fatal("expected error when autocert cache dir cannot be created")
	}
}

func TestGetHTTPHandlerLetsEncrypt(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Config{
		Enabled:     true,
		CertPath:    dir,
		LetsEncrypt: LetsEncryptConfig{Enabled: true, Email: "admin@example.com"},
	})
	if _, err := m.GetTLSConfig([]string{"example.com"}); err != nil {
		t.Fatalf("failed to configure Let's Encrypt: %v", err)
	}

	called := false
	fallback := okHandlerSSL(&called)
	handler := m.GetHTTPHandler(fallback)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected fallback handler to be invoked for a non-ACME-challenge request")
	}
}

func TestFindExistingCertsMissing(t *testing.T) {
	m := &Manager{}
	cert, key := m.findExistingCerts([]string{"missing.example.com"})
	if cert != "" || key != "" {
		t.Errorf("expected no certificate found, got cert=%q key=%q", cert, key)
	}
}

func TestGetHTTPHandlerFallback(t *testing.T) {
	m := NewManager(Config{})
	called := false
	fallback := okHandlerSSL(&called)

	handler := m.GetHTTPHandler(fallback)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected fallback handler to be called when no cert manager configured")
	}
}

func TestChallengeServerSetClearServe(t *testing.T) {
	cs := NewChallengeServer()
	cs.SetToken("token123", "auth-value")

	req := httptest.NewRequest("GET", "/.well-known/acme-challenge/token123", nil)
	rec := httptest.NewRecorder()
	handled := cs.ServeHTTP(rec, req)

	if !handled {
		t.Fatal("expected challenge request to be handled")
	}
	if rec.Body.String() != "auth-value" {
		t.Errorf("expected body auth-value, got %q", rec.Body.String())
	}

	cs.ClearToken("token123")
	rec = httptest.NewRecorder()
	handled = cs.ServeHTTP(rec, req)
	if !handled {
		t.Fatal("expected challenge request to still be handled after clearing")
	}
	if rec.Code != 404 {
		t.Errorf("expected 404 after token cleared, got %d", rec.Code)
	}
}

func TestChallengeServerIgnoresOtherPaths(t *testing.T) {
	cs := NewChallengeServer()
	req := httptest.NewRequest("GET", "/other/path", nil)
	rec := httptest.NewRecorder()

	if cs.ServeHTTP(rec, req) {
		t.Error("expected non-challenge path to be ignored")
	}
}

func TestParseChallenge(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http-01", "http-01"},
		{"http01", "http-01"},
		{"HTTP", "http-01"},
		{"tls-alpn-01", "tls-alpn-01"},
		{"tlsalpn01", "tls-alpn-01"},
		{"tls", "tls-alpn-01"},
		{"dns-01", "dns-01"},
		{"dns01", "dns-01"},
		{"dns", "dns-01"},
		{"unknown", "http-01"},
		{"  DNS  ", "dns-01"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseChallenge(tt.input); got != tt.want {
				t.Errorf("ParseChallenge(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(existing, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if !fileExists(existing) {
		t.Error("expected fileExists to return true for existing file")
	}
	if fileExists(filepath.Join(dir, "missing.txt")) {
		t.Error("expected fileExists to return false for missing file")
	}
}
