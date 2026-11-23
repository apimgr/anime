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
	defer func() {
		if err := db.SQL.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}()

	if db.Driver != "sqlite" || db.Locality != "local" {
		t.Fatalf("database metadata = %q/%q", db.Driver, db.Locality)
	}
	if err := EnsureSchema(context.Background(), db.SQL); err != nil {
		t.Fatalf("second schema initialization failed: %v", err)
	}
	for _, table := range []string{"config", "config_meta", "rate_limits", "audit_log", "scheduler_tasks", "scheduler_history", "backups", "owners", "api_tokens", "security_pgp_lock"} {
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
	defer func() {
		if err := db.SQL.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}()
	if _, err := os.Stat(filepath.Join(databaseDir, "server.db")); err != nil {
		t.Fatalf("database was not created in the resolved database directory: %v", err)
	}
}

func TestOpenReadOnlyRequiresExistingDatabaseAndCannotWrite(t *testing.T) {
	databaseDir := t.TempDir()
	cfg := config.DefaultConfig().Server.Database
	cfg.URL = filepath.Join(databaseDir, "server.db")
	if _, err := OpenReadOnly(cfg, databaseDir); err == nil {
		t.Fatal("OpenReadOnly created a missing database")
	}
	if _, err := os.Stat(cfg.URL); !os.IsNotExist(err) {
		t.Fatalf("missing database was created: stat error=%v", err)
	}
	db, err := Open(cfg, databaseDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	readOnly, err := OpenReadOnly(cfg, databaseDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := readOnly.SQL.Close(); err != nil {
			t.Errorf("close read-only database: %v", err)
		}
	}()
	if _, err := readOnly.SQL.Exec(`CREATE TABLE forbidden_write (id INTEGER)`); err == nil {
		t.Fatal("read-only database accepted a schema write")
	}
	var found string
	if err := readOnly.SQL.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'owners'`).Scan(&found); err != nil {
		t.Fatalf("read-only database could not query existing schema: %v", err)
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
