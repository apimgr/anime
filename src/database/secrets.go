package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"
)

var generatedSecretNames = []string{
	"installation_secret",
	"cookie_signing_key",
	"csrf_token_secret",
}

// EnsureAppSecrets initializes generated secrets without replacing stored values.
func EnsureAppSecrets(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin secret initialization: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, name := range generatedSecretNames {
		value := make([]byte, 32)
		if _, err := rand.Read(value); err != nil {
			return fmt.Errorf("generate %s: %w", name, err)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO app_secrets (name, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(name) DO NOTHING`, name, base64.StdEncoding.EncodeToString(value), time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("initialize %s: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit secret initialization: %w", err)
	}
	return nil
}

// AppSecret returns a generated secret decoded from its base64 database value.
func AppSecret(ctx context.Context, db *sql.DB, name string) ([]byte, error) {
	var encoded string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE name = ?`, name).Scan(&encoded); err != nil {
		return nil, fmt.Errorf("read app secret %q: %w", name, err)
	}
	value, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode app secret %q: %w", name, err)
	}
	if len(value) != 32 {
		return nil, fmt.Errorf("app secret %q has invalid length", name)
	}
	return value, nil
}
