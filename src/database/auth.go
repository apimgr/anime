package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AdminUser represents an admin user
type AdminUser struct {
	ID           int64
	Username     string
	PasswordHash string
	Token        string
	CreatedAt    time.Time
	LastLogin    *time.Time
}

// CreateAdminUser creates a new admin user with hashed password
func (db *DB) CreateAdminUser(username, password string) (*AdminUser, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate token
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Insert into database
	result, err := db.conn.Exec(`
		INSERT INTO admin_users (username, password_hash, token, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, username, string(hashedPassword), token)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &AdminUser{
		ID:           id,
		Username:     username,
		PasswordHash: string(hashedPassword),
		Token:        token,
		CreatedAt:    time.Now(),
	}, nil
}

// AuthenticateAdmin authenticates an admin user and returns their token
func (db *DB) AuthenticateAdmin(username, password string) (string, error) {
	var user AdminUser
	err := db.conn.QueryRow(`
		SELECT id, username, password_hash, token
		FROM admin_users
		WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Token)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("invalid username or password")
	}
	if err != nil {
		return "", fmt.Errorf("failed to query admin user: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid username or password")
	}

	// Update last login
	_, err = db.conn.Exec(`
		UPDATE admin_users
		SET last_login = CURRENT_TIMESTAMP
		WHERE id = ?
	`, user.ID)
	if err != nil {
		// Log error but don't fail authentication
		fmt.Printf("Warning: failed to update last login: %v\n", err)
	}

	return user.Token, nil
}

// ValidateToken validates an admin token
func (db *DB) ValidateToken(token string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM admin_users
		WHERE token = ?
	`, token).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to validate token: %w", err)
	}

	return count > 0, nil
}

// GetAdminByToken retrieves an admin user by token
func (db *DB) GetAdminByToken(token string) (*AdminUser, error) {
	var user AdminUser
	var lastLogin sql.NullTime

	err := db.conn.QueryRow(`
		SELECT id, username, password_hash, token, created_at, last_login
		FROM admin_users
		WHERE token = ?
	`, token).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Token, &user.CreatedAt, &lastLogin)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("admin user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query admin user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// RevokeToken revokes an admin token by generating a new one
func (db *DB) RevokeToken(username string) (string, error) {
	// Generate new token
	newToken, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Update token in database
	result, err := db.conn.Exec(`
		UPDATE admin_users
		SET token = ?
		WHERE username = ?
	`, newToken, username)
	if err != nil {
		return "", fmt.Errorf("failed to update token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return "", fmt.Errorf("admin user not found")
	}

	return newToken, nil
}

// generateToken generates a secure random token
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// AdminCredentials represents admin user credentials for display
type AdminCredentials struct {
	Username string
	Password string
	Token    string
}

// SaveCredentialsToFile saves credentials to a file
func SaveCredentialsToFile(creds *AdminCredentials, configDir, port string) error {
	serverURL := getAccessibleURL(port)

	content := fmt.Sprintf(`Anime Quotes API - ADMIN CREDENTIALS
========================================
WEB UI LOGIN:
  URL:      %s/admin
  Username: %s

API ACCESS:
  URL:      %s/api/v1/admin
  Header:   Authorization: Bearer %s

CREDENTIALS:
  Username: %s
  Password: %s
  Token:    %s

Created: %s
========================================`, serverURL, creds.Username, serverURL, creds.Token,
		creds.Username, creds.Password, creds.Token, time.Now().Format("2006-01-02 15:04:05"))

	credPath := filepath.Join(configDir, "credentials.txt")
	return os.WriteFile(credPath, []byte(content), 0600)
}

// getAccessibleURL returns the most relevant URL for accessing the server
// Priority: FQDN > hostname > public IP > fallback
// NEVER shows localhost, 127.0.0.1, or 0.0.0.0
// IPv6 addresses are wrapped in brackets: http://[2001:db8::1]:8080
func getAccessibleURL(port string) string {
	// Try to get hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "localhost" {
		// Try to resolve hostname to see if it's a valid FQDN
		if addrs, err := net.LookupHost(hostname); err == nil && len(addrs) > 0 {
			return fmt.Sprintf("http://%s:%s", hostname, port)
		}
	}

	// Try to get outbound IP (most likely accessible IP)
	if ip := getOutboundIP(); ip != "" {
		// Wrap IPv6 addresses in brackets for URLs
		if strings.Contains(ip, ":") {
			return fmt.Sprintf("http://[%s]:%s", ip, port)
		}
		return fmt.Sprintf("http://%s:%s", ip, port)
	}

	// Fallback to hostname if we have one
	if hostname != "" && hostname != "localhost" {
		return fmt.Sprintf("http://%s:%s", hostname, port)
	}

	// Last resort: use a generic message
	return fmt.Sprintf("http://<your-host>:%s", port)
}

// getOutboundIP gets the preferred outbound IP of this machine
// Tries IPv4 first (8.8.8.8:80), then IPv6 ([2001:4860:4860::8888]:80)
func getOutboundIP() string {
	// Try IPv4 first
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	// Try IPv6 if IPv4 fails
	conn, err = net.Dial("udp", "[2001:4860:4860::8888]:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	return ""
}
