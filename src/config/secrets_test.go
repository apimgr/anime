package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCreatesPrivatePersistentEncryptionSecret(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: \"64581\"\n  token: \"adm_testtoken\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	key, err := base64.StdEncoding.DecodeString(first.Server.Security.EncryptionKey)
	if err != nil || len(key) != 32 || first.Server.Security.EncryptionKeyVersion != 1 {
		t.Fatalf("generated encryption key invalid: length=%d version=%d err=%v", len(key), first.Server.Security.EncryptionKeyVersion, err)
	}
	info, err := os.Stat(filepath.Join(dir, "secrets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secrets file permissions=%v", info.Mode().Perm())
	}
	second, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if first.Server.Security.EncryptionKey != second.Server.Security.EncryptionKey {
		t.Fatal("encryption key changed between config loads")
	}
}

func TestLoadMovesSecretValuesOutOfServerConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "server.yml")
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	contents := fmt.Sprintf("server:\n  port: \"64582\"\n  token: \"adm_testtoken\"\n  security:\n    encryption_key: %q\n    encryption_key_version: 1\n  notifications:\n    email:\n      smtp:\n        password: \"mail-secret\"\n", key)
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(configPath); err != nil {
		t.Fatal(err)
	}
	mainConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mainConfig), key) || strings.Contains(string(mainConfig), "mail-secret") {
		t.Fatal("secret values remained in server.yml after migration")
	}
	secretConfig, err := os.ReadFile(filepath.Join(dir, "secrets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(secretConfig), key) || !strings.Contains(string(secretConfig), "mail-secret") {
		t.Fatal("migrated secret values are missing from secrets.yml")
	}
}
