package database

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB represents the database connection
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection
func NewDB(dataDir string) (*DB, error) {
	dbPath := filepath.Join(dataDir, "anime.db")

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// initSchema creates the database tables
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		type TEXT DEFAULT 'string',
		category TEXT DEFAULT '',
		description TEXT DEFAULT '',
		requires_reload INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS admin_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		token TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login DATETIME
	);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Migrate existing settings table if needed
	_, err = db.conn.Exec(`
		ALTER TABLE settings ADD COLUMN type TEXT DEFAULT 'string';
		ALTER TABLE settings ADD COLUMN category TEXT DEFAULT '';
		ALTER TABLE settings ADD COLUMN description TEXT DEFAULT '';
		ALTER TABLE settings ADD COLUMN requires_reload INTEGER DEFAULT 0;
	`)
	// Ignore errors if columns already exist

	// Initialize default settings
	return db.InitializeDefaultSettings()
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}
