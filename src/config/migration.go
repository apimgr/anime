package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type areaMove struct {
	RootSection string
	File        string
	Area        string
	Section     string
}

var configAreaMoves = []areaMove{
	{RootSection: "branding", File: "branding.yml", Area: "server", Section: "branding"},
	{RootSection: "seo", File: "branding.yml", Area: "server", Section: "seo"},
	{RootSection: "web-ui", File: "branding.yml", Area: "web", Section: "ui"},
	{RootSection: "i18n", File: "web.yml", Area: "server", Section: "i18n"},
	{RootSection: "logging", File: "logs.yml", Area: "server", Section: "logs"},
	{RootSection: "metrics", File: "metrics.yml", Area: "server", Section: "metrics"},
	{RootSection: "contact", File: "contact.yml", Area: "server", Section: "contact"},
	{RootSection: "privacy", File: "privacy.yml", Area: "server", Section: "privacy"},
	{RootSection: "compliance", File: "privacy.yml", Area: "server", Section: "compliance"},
	{RootSection: "tracking", File: "privacy.yml", Area: "server", Section: "tracking"},
	{RootSection: "security", File: "security.yml", Area: "server", Section: "security"},
	{RootSection: "rate_limit", File: "security.yml", Area: "server", Section: "rate_limit"},
	{RootSection: "tokens", File: "security.yml", Area: "server", Section: "tokens"},
	{RootSection: "pages", File: "pages.yml", Area: "server", Section: "pages"},
	{RootSection: "notifications", File: "notifications.yml", Area: "server", Section: "notifications"},
	{RootSection: "scheduler", File: "maintenance.yml", Area: "server", Section: "scheduler"},
	{RootSection: "schedule", File: "maintenance.yml", Area: "server", Section: "scheduler"},
	{RootSection: "update", File: "maintenance.yml", Area: "server", Section: "update"},
	{RootSection: "cors", File: "web.yml", Area: "server", Section: "cors"},
	{RootSection: "web-robots", File: "web.yml", Area: "web", Section: "robots"},
	{RootSection: "web-security", File: "web.yml", Area: "web", Section: "security"},
	{RootSection: "web-llms", File: "web.yml", Area: "web", Section: "llms"},
}

func migrateAreaFiles(mainPath string) error {
	root, err := readYAMLMap(mainPath)
	if err != nil {
		return err
	}
	server, _ := root["server"].(map[string]interface{})
	if server == nil {
		server = make(map[string]interface{})
		root["server"] = server
	}
	moves := make(map[string]map[string]interface{})
	changed := false
	for _, move := range configAreaMoves {
		var value interface{}
		if move.Area == "server" {
			value, changed = takeMapValue(server, move.RootSection, changed)
		} else {
			value, changed = takeMapValue(root, move.RootSection, changed)
		}
		if value == nil {
			continue
		}
		area := moves[move.File]
		if area == nil {
			area = make(map[string]interface{})
			moves[move.File] = area
		}
		top, _ := area[move.Area].(map[string]interface{})
		if top == nil {
			top = make(map[string]interface{})
			area[move.Area] = top
		}
		current, _ := top[move.Section].(map[string]interface{})
		if current == nil {
			current = make(map[string]interface{})
			top[move.Section] = current
		}
		if source, ok := value.(map[string]interface{}); ok {
			mergeMissingConfigMaps(current, source)
		} else {
			top[move.Section] = value
		}
	}
	if !changed {
		return nil
	}

	dir := filepath.Join(filepath.Dir(mainPath), "yaml.d")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create configuration area directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("secure configuration area directory: %w", err)
	}
	backup := mainPath + ".bak"
	if err := copyPrivateFile(mainPath, backup); err != nil {
		return fmt.Errorf("backup server config before migration: %w", err)
	}
	for name, incoming := range moves {
		path := filepath.Join(dir, name)
		existing, readErr := readYAMLMap(path)
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("read existing area file %s: %w", name, readErr)
		}
		if existing == nil {
			existing = make(map[string]interface{})
		}
		mergeMissingConfigMaps(existing, incoming)
		content, marshalErr := yaml.Marshal(existing)
		if marshalErr != nil {
			return marshalErr
		}
		if writeErr := writePrivateAtomic(path, content); writeErr != nil {
			return fmt.Errorf("write area file %s: %w", name, writeErr)
		}
	}
	content, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	content = append([]byte("# This node only: listen, identity, database, cache, ssl. The rest is in yaml.d/ and credentials are in secrets.yml.\n\n"), content...)
	if err := writePrivateAtomic(mainPath, content); err != nil {
		return fmt.Errorf("rewrite server config after migration: %w", err)
	}
	return nil
}

func takeMapValue(values map[string]interface{}, key string, changed bool) (interface{}, bool) {
	value, exists := values[key]
	if exists {
		delete(values, key)
		changed = true
	}
	return value, changed
}

func mergeMissingConfigMaps(target, source map[string]interface{}) {
	for key, value := range source {
		current, exists := target[key]
		if !exists {
			target[key] = value
			continue
		}
		targetMap, targetIsMap := current.(map[string]interface{})
		sourceMap, sourceIsMap := value.(map[string]interface{})
		if targetIsMap && sourceIsMap {
			mergeMissingConfigMaps(targetMap, sourceMap)
		}
	}
}

func copyPrivateFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writePrivateAtomic(target, data)
}
