package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/anime/src/config"
	"github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

const (
	queryTimeout = 5 * time.Second
	openTimeout  = 5 * time.Second
)

type Database struct {
	SQL      *sql.DB
	Driver   string
	Locality string
}

func Open(cfg config.DatabaseConfig, databaseDir string) (*Database, error) {
	driver := normalizeDriver(cfg.Driver)
	dsn := strings.TrimSpace(os.ExpandEnv(cfg.URL))
	if dsn == "" {
		driver = "sqlite"
		dsn = filepath.Join(databaseDir, "server.db")
	} else if parsed, err := url.Parse(dsn); err == nil && parsed.Scheme == "libsql" {
		driver = "libsql"
	}

	var db *sql.DB
	var err error
	locality := "local"
	switch driver {
	case "sqlite":
		if err := ensureDatabaseDirectory(dsn); err != nil {
			return nil, err
		}
		db, err = openSQLite(dsn)
	case "libsql":
		locality = "remote"
		db, err = openLibSQL(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	maxOpen := cfg.Pool.MaxOpen
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := cfg.Pool.MaxIdle
	if maxIdle < 0 {
		maxIdle = 0
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	maxLifetime := parseDuration(cfg.Pool.MaxLifetime, 5*time.Minute)
	maxIdleTime := parseDuration(cfg.Pool.MaxIdleTime, time.Minute)
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
	db.SetConnMaxIdleTime(maxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database connection check failed: %w", err)
	}
	if err := EnsureSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if driver == "sqlite" {
		if err := setSQLiteFilePermissions(dsn); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return &Database{SQL: db, Driver: driver, Locality: locality}, nil
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite", "sqlite2", "sqlite3":
		return "sqlite"
	case "libsql", "turso":
		return "libsql"
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func ensureDatabaseDirectory(dsn string) error {
	path := dsn
	if strings.HasPrefix(path, "file:") {
		parsed, err := url.Parse(path)
		if err != nil {
			return fmt.Errorf("invalid SQLite database URL")
		}
		path = parsed.Path
	}
	if path == ":memory:" || strings.HasPrefix(path, "file::memory:") {
		return nil
	}
	mode := os.FileMode(0755)
	if os.Geteuid() != 0 {
		mode = 0700
	}
	if err := os.MkdirAll(filepath.Dir(path), mode); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	return nil
}

func openSQLite(path string) (*sql.DB, error) {
	if !strings.HasPrefix(path, "file:") && path != ":memory:" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve SQLite path: %w", err)
		}
		path = "file:" + filepath.ToSlash(absolute)
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	path += separator + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	return sql.Open("sqlite", path)
}

func setSQLiteFilePermissions(dsn string) error {
	if dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory:") {
		return nil
	}
	path := dsn
	if strings.HasPrefix(path, "file:") {
		parsed, err := url.Parse(path)
		if err != nil {
			return fmt.Errorf("invalid SQLite database URL")
		}
		path = parsed.Path
	}
	permissions := os.FileMode(0644)
	if os.Geteuid() != 0 {
		permissions = 0600
	}
	if err := os.Chmod(path, permissions); err != nil {
		return fmt.Errorf("set database file permissions: %w", err)
	}
	return nil
}

func openLibSQL(dsn string) (*sql.DB, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid libsql database URL")
	}
	query := parsed.Query()
	authToken := query.Get("authToken")
	if authToken == "" {
		authToken = query.Get("auth_token")
	}
	query.Del("authToken")
	query.Del("auth_token")
	parsed.RawQuery = query.Encode()
	if authToken == "" {
		return nil, fmt.Errorf("libsql database URL must provide an authentication token")
	}
	connector, err := libsql.NewConnector(parsed.String(), libsql.WithAuthToken(authToken))
	if err != nil {
		return nil, fmt.Errorf("configure libsql connector: %w", err)
	}
	return sql.OpenDB(connector), nil
}

func EnsureSchema(parent context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	for _, statement := range schemaStatements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize database schema: %w", err)
		}
	}
	return nil
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL, type TEXT NOT NULL DEFAULT 'string', updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')))`,
	`CREATE TABLE IF NOT EXISTS config_meta (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL DEFAULT 1, updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')))`,
	`INSERT OR IGNORE INTO config_meta (id, version) VALUES (1, 1)`,
	`CREATE TRIGGER IF NOT EXISTS config_version_bump_insert AFTER INSERT ON config BEGIN UPDATE config_meta SET version = version + 1, updated_at = strftime('%s', 'now') WHERE id = 1; END`,
	`CREATE TRIGGER IF NOT EXISTS config_version_bump_update AFTER UPDATE ON config BEGIN UPDATE config_meta SET version = version + 1, updated_at = strftime('%s', 'now') WHERE id = 1; END`,
	`CREATE TRIGGER IF NOT EXISTS config_version_bump_delete AFTER DELETE ON config BEGIN UPDATE config_meta SET version = version + 1, updated_at = strftime('%s', 'now') WHERE id = 1; END`,
	`CREATE INDEX IF NOT EXISTS idx_config_key_prefix ON config(key)`,
	`CREATE TABLE IF NOT EXISTS rate_limits (key TEXT PRIMARY KEY, count INTEGER NOT NULL DEFAULT 1, window_start INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')))`,
	`CREATE INDEX IF NOT EXISTS idx_rate_limits_window ON rate_limits(window_start)`,
	`CREATE TABLE IF NOT EXISTS audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), level TEXT NOT NULL DEFAULT 'info', category TEXT NOT NULL, action TEXT NOT NULL, actor_ip TEXT, target_type TEXT, target_id TEXT, details TEXT, success INTEGER NOT NULL DEFAULT 1)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_log(category)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_actor_ip ON audit_log(actor_ip)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_target ON audit_log(target_type, target_id)`,
	`CREATE TABLE IF NOT EXISTS scheduler_tasks (id TEXT PRIMARY KEY, name TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, schedule TEXT NOT NULL, last_run INTEGER, next_run INTEGER, last_status TEXT, last_error TEXT, run_count INTEGER NOT NULL DEFAULT 0, fail_count INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE IF NOT EXISTS scheduler_history (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, started_at INTEGER NOT NULL, finished_at INTEGER, status TEXT NOT NULL, error TEXT, duration_ms INTEGER)`,
	`CREATE INDEX IF NOT EXISTS idx_scheduler_history_task ON scheduler_history(task_id)`,
	`CREATE INDEX IF NOT EXISTS idx_scheduler_history_started ON scheduler_history(started_at)`,
	`CREATE TABLE IF NOT EXISTS backups (id INTEGER PRIMARY KEY AUTOINCREMENT, filename TEXT NOT NULL UNIQUE, filepath TEXT NOT NULL, size_bytes INTEGER NOT NULL, type TEXT NOT NULL DEFAULT 'auto', created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), checksum TEXT, notes TEXT)`,
	`CREATE INDEX IF NOT EXISTS idx_backups_created ON backups(created_at)`,
	`CREATE TABLE IF NOT EXISTS owners (id TEXT PRIMARY KEY, created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), last_seen_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')))`,
	`CREATE TABLE IF NOT EXISTS api_tokens (id INTEGER PRIMARY KEY AUTOINCREMENT, owner_id TEXT NOT NULL REFERENCES owners(id) ON DELETE CASCADE, kind TEXT NOT NULL CHECK (kind IN ('primary', 'linked')), name TEXT, token_hash TEXT NOT NULL UNIQUE, token_prefix TEXT NOT NULL, token_enc TEXT NOT NULL, created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), rotated_at INTEGER, expires_at INTEGER, last_used_at INTEGER, revoked_at INTEGER, revoked_reason TEXT)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_primary ON api_tokens(owner_id) WHERE kind = 'primary' AND revoked_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(token_hash)`,
	`CREATE INDEX IF NOT EXISTS idx_api_tokens_prefix ON api_tokens(token_prefix)`,
	`CREATE INDEX IF NOT EXISTS idx_api_tokens_owner ON api_tokens(owner_id)`,
	`CREATE INDEX IF NOT EXISTS idx_api_tokens_active ON api_tokens(revoked_at) WHERE revoked_at IS NULL`,
}

func QueryContext(ctx context.Context, db *sql.DB, query string, args ...any) (*sql.Rows, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return db.QueryContext(queryCtx, query, args...)
}
