package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadMergedValues(mainPath string) map[string]interface{} {
	merged := make(map[string]interface{})
	if err := secureConfigPath(mainPath); err != nil {
		log.Printf("configuration file permission warning: %s: %v", filepath.Base(mainPath), err)
	}
	if values, err := readYAMLMap(mainPath); err == nil {
		mergeConfigMaps(merged, values)
	} else if !os.IsNotExist(err) {
		log.Printf("configuration file ignored: %s: %v", filepath.Base(mainPath), err)
	}

	dir := filepath.Join(filepath.Dir(mainPath), "yaml.d")
	if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
		if chmodErr := os.Chmod(dir, 0700); chmodErr != nil {
			log.Printf("configuration drop-in directory permission warning: %s: %v", filepath.Base(dir), chmodErr)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("configuration drop-in directory unavailable: %s: %v", filepath.Base(dir), err)
	}
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".orig") {
				continue
			}
			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".yml" && ext != ".yaml" {
				continue
			}
			path := filepath.Join(dir, name)
			info, statErr := os.Stat(path)
			if statErr != nil || !info.Mode().IsRegular() || !dropinPathAllowed(dir, path) {
				continue
			}
			if chmodErr := secureConfigPath(path); chmodErr != nil {
				log.Printf("configuration drop-in ignored: %s: cannot secure permissions: %v", name, chmodErr)
				continue
			}
			values, readErr := readYAMLMap(path)
			if readErr != nil {
				log.Printf("configuration drop-in ignored: %s: %v", name, readErr)
				continue
			}
			log.Printf("configuration drop-in loaded: %s", name)
			mergeConfigMaps(merged, values)
		}
	}

	secretsPath := filepath.Join(filepath.Dir(mainPath), "secrets.yml")
	if err := secureConfigPath(secretsPath); err != nil {
		log.Printf("secrets file permission warning: %s: %v", filepath.Base(secretsPath), err)
	}
	if values, err := readYAMLMap(secretsPath); err == nil {
		mergeConfigMaps(merged, values)
	} else if !os.IsNotExist(err) {
		log.Printf("secrets file ignored: %s: %v", filepath.Base(secretsPath), err)
	}
	return merged
}

func readYAMLMap(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if values == nil {
		values = make(map[string]interface{})
	}
	return values, nil
}

func dropinPathAllowed(dir, path string) bool {
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(resolvedDir, resolvedPath)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func mergeConfigMaps(target, source map[string]interface{}) {
	for key, incoming := range source {
		if incoming == nil {
			continue
		}
		incomingMap, isMap := incoming.(map[string]interface{})
		if !isMap {
			if genericMap, ok := incoming.(map[interface{}]interface{}); ok {
				incomingMap = make(map[string]interface{}, len(genericMap))
				for nestedKey, nestedValue := range genericMap {
					if textKey, ok := nestedKey.(string); ok {
						incomingMap[textKey] = nestedValue
					}
				}
				isMap = true
			}
		}
		if isMap {
			current, ok := target[key].(map[string]interface{})
			if !ok {
				current = make(map[string]interface{})
				target[key] = current
			}
			mergeConfigMaps(current, incomingMap)
			continue
		}
		target[key] = incoming
	}
}

func unmarshalMergedConfig(values map[string]interface{}, cfg *Config) error {
	server, _ := values["server"].(map[string]interface{})
	if server != nil {
		if _, exists := server["logging"]; !exists {
			if logs, ok := server["logs"]; ok {
				server["logging"] = logs
			}
		}
		if _, exists := server["schedule"]; !exists {
			if scheduler, ok := server["scheduler"]; ok {
				server["schedule"] = scheduler
			}
		}
		if cors, ok := server["cors"]; ok {
			security, _ := values["web-security"].(map[string]interface{})
			if security == nil {
				security = make(map[string]interface{})
				values["web-security"] = security
			}
			security["cors"] = cors
		}
	}
	web, _ := values["web"].(map[string]interface{})
	if web != nil {
		if llms, ok := web["llms"]; ok {
			values["web-llms"] = llms
		}
		if ui, ok := web["ui"]; ok {
			values["web-ui"] = ui
		}
		if robots, ok := web["robots"]; ok {
			values["web-robots"] = robots
		}
		if security, ok := web["security"].(map[string]interface{}); ok {
			values["web-security"] = security
		}
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode merged configuration: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("decode merged configuration: %w", err)
	}
	return nil
}

func secureConfigPath(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if err := os.Chmod(path, 0600); err != nil {
		return err
	}
	return nil
}
