package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadOrCreateSecrets(configPath string, cfg *Config) error {
	path := filepath.Join(filepath.Dir(configPath), "secrets.yml")
	values := make(map[string]interface{})
	data, err := os.ReadFile(path)
	needsWrite := false
	if err == nil {
		if err := yaml.Unmarshal(data, &values); err != nil {
			return fmt.Errorf("parse secrets file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read secrets file: %w", err)
	} else {
		needsWrite = true
	}
	server := nestedMap(values, "server")
	token := stringValue(server["token"])
	if token == "" {
		token = cfg.Server.Token
	}
	if token == "" {
		var err error
		token, err = GenerateOperatorToken()
		if err != nil {
			return fmt.Errorf("generate operator token: %w", err)
		}
	}
	if stringValue(server["token"]) != token {
		needsWrite = true
	}
	server["token"] = token
	cfg.Server.Token = token
	security := nestedMap(server, "security")
	secret := stringValue(security["encryption_key"])
	version := intValue(security["encryption_key_version"])
	if secret == "" {
		secret = cfg.Server.Security.EncryptionKey
		if secret != "" {
			needsWrite = true
		}
	}
	if secret == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("generate encryption key: %w", err)
		}
		secret = base64.StdEncoding.EncodeToString(key)
		needsWrite = true
	}
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("server.security.encryption_key must be base64-encoded 32-byte key material")
	}
	if version == 0 {
		version = cfg.Server.Security.EncryptionKeyVersion
	}
	if version == 0 {
		version = 1
		needsWrite = true
	}
	notifications := nestedMap(server, "notifications")
	email := nestedMap(notifications, "email")
	smtp := nestedMap(email, "smtp")
	smtpPassword := stringValue(smtp["password"])
	if smtpPassword == "" {
		smtpPassword = cfg.Server.Notifications.Email.SMTP.Password
		if smtpPassword != "" {
			smtp["password"] = smtpPassword
			needsWrite = true
		}
	}
	if smtpPassword != "" {
		smtp["password"] = smtpPassword
		email["smtp"] = smtp
		notifications["email"] = email
		server["notifications"] = notifications
		cfg.Server.Notifications.Email.SMTP.Password = smtpPassword
	}
	pages := nestedMap(server, "pages")
	contactPage := nestedMap(pages, "contact")
	captchaSecret := stringValue(contactPage["captcha_secret_key"])
	if captchaSecret == "" {
		captchaSecret = cfg.Server.Pages.Contact.CaptchaSecretKey
		if captchaSecret != "" {
			needsWrite = true
		}
	}
	if captchaSecret != "" {
		contactPage["captcha_secret_key"] = captchaSecret
		pages["contact"] = contactPage
		server["pages"] = pages
		cfg.Server.Pages.Contact.CaptchaSecretKey = captchaSecret
	}
	security["encryption_key"] = secret
	security["encryption_key_version"] = version
	server["security"] = security
	values["server"] = server
	if needsWrite {
		if err := writeSecretsFile(path, values); err != nil {
			return err
		}
	} else {
		if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
			return fmt.Errorf("secure secrets directory: %w", err)
		}
		if err := os.Chmod(path, 0600); err != nil {
			return fmt.Errorf("secure secrets file: %w", err)
		}
	}
	if err := removeSecretsFromMainConfig(configPath); err != nil {
		return err
	}
	if err := removeSecretsFromDropins(filepath.Join(filepath.Dir(configPath), "yaml.d")); err != nil {
		return err
	}
	cfg.Server.Security.EncryptionKey = secret
	cfg.Server.Security.EncryptionKeyVersion = version
	return nil
}

func removeSecretsFromMainConfig(path string) error {
	return removeSecretsFromConfigFile(path)
}

func removeSecretsFromDropins(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read drop-in directory for secret migration: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || (filepath.Ext(entry.Name()) != ".yml" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		if err := removeSecretsFromConfigFile(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func removeSecretsFromConfigFile(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read main config for secret migration: %w", err)
	}
	values := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("parse main config for secret migration: %w", err)
	}
	server, ok := values["server"].(map[string]interface{})
	if !ok {
		return nil
	}
	security, ok := server["security"].(map[string]interface{})
	secretMoved := false
	if _, present := server["token"]; present {
		delete(server, "token")
		secretMoved = true
	}
	if ok {
		_, hasKey := security["encryption_key"]
		_, hasVersion := security["encryption_key_version"]
		secretMoved = hasKey || hasVersion
		delete(security, "encryption_key")
		delete(security, "encryption_key_version")
		if len(security) == 0 {
			delete(server, "security")
		}
	}
	if notifications, exists := server["notifications"].(map[string]interface{}); exists {
		if email, exists := notifications["email"].(map[string]interface{}); exists {
			if smtp, exists := email["smtp"].(map[string]interface{}); exists {
				if _, present := smtp["password"]; present {
					delete(smtp, "password")
					secretMoved = true
				}
				if len(smtp) == 0 {
					delete(email, "smtp")
				}
				if len(email) == 0 {
					delete(notifications, "email")
				}
				if len(notifications) == 0 {
					delete(server, "notifications")
				}
			}
		}
	}
	if pages, exists := server["pages"].(map[string]interface{}); exists {
		if contact, exists := pages["contact"].(map[string]interface{}); exists {
			if _, present := contact["captcha_secret_key"]; present {
				delete(contact, "captcha_secret_key")
				secretMoved = true
			}
			if len(contact) == 0 {
				delete(pages, "contact")
			}
			if len(pages) == 0 {
				delete(server, "pages")
			}
		}
	}
	if !secretMoved {
		return nil
	}
	content, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode migrated main config: %w", err)
	}
	if err := writePrivateAtomic(path, content); err != nil {
		return fmt.Errorf("write migrated main config: %w", err)
	}
	return nil
}

func nestedMap(parent map[string]interface{}, key string) map[string]interface{} {
	if child, ok := parent[key].(map[string]interface{}); ok {
		return child
	}
	child := make(map[string]interface{})
	parent[key] = child
	return child
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func intValue(value interface{}) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case uint64:
		return int(number)
	default:
		return 0
	}
}

func writeSecretsFile(path string, values map[string]interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("secure secrets directory: %w", err)
	}
	content, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode secrets file: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".secrets.yml-*")
	if err != nil {
		return fmt.Errorf("create temporary secrets file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) {
			log.Printf("temporary secrets file cleanup failed: %v", err)
		}
	}()
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary secrets file: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write secrets file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync secrets file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close secrets file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("install secrets file: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("secure secrets file: %w", err)
	}
	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("sync secrets directory: %w", err)
	}
	return nil
}

func writePrivateAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".config-migrate-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) {
			log.Printf("temporary configuration file cleanup failed: %v", err)
		}
	}()
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	if err := os.Chmod(path, 0600); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := directory.Close(); err != nil {
			log.Printf("configuration directory close failed: %v", err)
		}
	}()
	return directory.Sync()
}
