package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/anime/src/config"
)

func TestOpenCreatesSQLiteSchemaIdempotently(t *testing.T) {
	cfg := config.DefaultConfig().Server.Database
	cfg.URL = filepath.Join(t.TempDir(), "db", "server.db")
	db, err := Open(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.SQL.Close()

	if db.Driver != "sqlite" || db.Locality != "local" {
		t.Fatalf("database metadata = %q/%q", db.Driver, db.Locality)
	}
	if err := EnsureSchema(context.Background(), db.SQL); err != nil {
		t.Fatalf("second schema initialization failed: %v", err)
	}
	for _, table := range []string{"config", "config_meta", "rate_limits", "audit_log", "scheduler_tasks", "scheduler_history", "backups", "owners", "api_tokens"} {
		var found string
		if err := db.SQL.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&found); err != nil {
			t.Errorf("schema missing table %q: %v", table, err)
		}
	}
}

func TestOpenUsesResolvedDatabaseDirectory(t *testing.T) {
	cfg := config.DefaultConfig().Server.Database
	databaseDir := t.TempDir()
	db, err := Open(cfg, databaseDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.SQL.Close()
	if _, err := os.Stat(filepath.Join(databaseDir, "server.db")); err != nil {
		t.Fatalf("database was not created in the resolved database directory: %v", err)
	}
}

func TestNormalizeDriverAliases(t *testing.T) {
	for input, want := range map[string]string{
		"": "sqlite", "sqlite": "sqlite", "SQLITE2": "sqlite", "sqlite3": "sqlite",
		"libsql": "libsql", "turso": "libsql", "postgres": "postgres",
	} {
		if got := normalizeDriver(input); got != want {
			t.Errorf("normalizeDriver(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOpenLibSQLRequiresAuthenticationToken(t *testing.T) {
	if _, err := openLibSQL("libsql://example.invalid"); err == nil {
		t.Fatal("openLibSQL accepted a URL without an authentication token")
	}
}
