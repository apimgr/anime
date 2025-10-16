package server

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/apimgr/anime/src/database"
)

// handleAdmin renders the admin dashboard
func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)
	uptimeStr := fmt.Sprintf("%dd %dh %dm",
		int(uptime.Hours()/24),
		int(uptime.Hours())%24,
		int(uptime.Minutes())%60,
	)

	data := map[string]interface{}{
		"Title":         "Admin Dashboard",
		"Page":          "admin",
		"TotalQuotes":   s.animeService.GetTotalQuotes(),
		"ServerAddress": s.address,
		"ServerPort":    s.port,
		"ServerURL":     s.getServerURL(r),
		"DatabaseType":  "Embedded (JSON)",
		"GoVersion":     runtime.Version(),
		"Uptime":        uptimeStr,
		"IsAdmin":       true,
	}

	if err := templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminSettingsPage renders the admin settings page
func (s *Server) handleAdminSettingsPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":   "Server Settings",
		"Page":    "admin_settings",
		"IsAdmin": true,
	}

	if err := templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminSettings handles GET/PUT for admin settings
func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return all settings grouped by category
		settings, err := s.db.GetAllSettingsDetailed()
		if err != nil {
			log.Printf("Error fetching settings: %v", err)
			http.Error(w, "Failed to fetch settings", http.StatusInternalServerError)
			return
		}

		// Group settings by category
		grouped := make(map[string][]*database.Setting)
		for _, setting := range settings {
			grouped[setting.Category] = append(grouped[setting.Category], setting)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"settings": grouped,
			"categories": []string{
				"cors",
				"rate_limiting",
				"request_limits",
				"connection_limits",
				"security",
			},
		})

	case http.MethodPut:
		// Update settings
		var updates map[string]string
		if err := parseJSON(r, &updates); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		requiresReload := false
		for key, value := range updates {
			// Get setting metadata to check if reload is required
			setting, err := s.db.GetSettingWithTimestamp(key)
			if err != nil {
				log.Printf("Error checking setting %s: %v", key, err)
				continue
			}

			if setting != nil && setting.RequiresReload {
				requiresReload = true
			}

			// Update the setting
			if err := s.db.SetSetting(key, value); err != nil {
				log.Printf("Error updating setting %s: %v", key, err)
				http.Error(w, fmt.Sprintf("Failed to update setting: %s", key), http.StatusInternalServerError)
				return
			}
		}

		// Reload settings in SettingsManager
		if s.settingsManager != nil {
			if err := s.settingsManager.ReloadSettings(); err != nil {
				log.Printf("Error reloading settings: %v", err)
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":         true,
			"message":         "Settings updated successfully",
			"requires_reload": requiresReload,
			"updated_count":   len(updates),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAdminSettingsReset resets all settings to defaults
func (s *Server) handleAdminSettingsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.settingsManager != nil {
		if err := s.settingsManager.ResetToDefaults(); err != nil {
			log.Printf("Error resetting settings: %v", err)
			http.Error(w, "Failed to reset settings", http.StatusInternalServerError)
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Settings reset to defaults successfully",
	})
}

// handleAdminSettingsExport exports all settings as JSON
func (s *Server) handleAdminSettingsExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := s.db.GetAllSettingsDetailed()
	if err != nil {
		log.Printf("Error exporting settings: %v", err)
		http.Error(w, "Failed to export settings", http.StatusInternalServerError)
		return
	}

	// Convert to export format
	export := make(map[string]interface{})
	for _, setting := range settings {
		export[setting.Key] = map[string]interface{}{
			"value":          setting.Value,
			"type":           setting.Type,
			"category":       setting.Category,
			"description":    setting.Description,
			"requires_reload": setting.RequiresReload,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=anime-settings.json")
	respondJSON(w, http.StatusOK, export)
}

// handleAdminSettingsImport imports settings from JSON
func (s *Server) handleAdminSettingsImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var imports map[string]map[string]interface{}
	if err := parseJSON(r, &imports); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Convert to simple key-value map
	updates := make(map[string]string)
	for key, data := range imports {
		if value, ok := data["value"].(string); ok {
			updates[key] = value
		}
	}

	if s.settingsManager != nil {
		if err := s.settingsManager.ImportSettings(updates); err != nil {
			log.Printf("Error importing settings: %v", err)
			http.Error(w, "Failed to import settings", http.StatusInternalServerError)
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"message":        "Settings imported successfully",
		"imported_count": len(updates),
	})
}

// handleAdminQuotes handles admin quote management (add/edit/delete)
func (s *Server) handleAdminQuotes(w http.ResponseWriter, r *http.Request) {
	// Placeholder for admin quote management API
	switch r.Method {
	case http.MethodGet:
		// Return all quotes with admin metadata
		quotes := s.animeService.GetAllQuotes()
		respondJSON(w, http.StatusOK, quotes)
	case http.MethodPost:
		// Add new quote
		respondJSON(w, http.StatusCreated, map[string]interface{}{
			"message": "Quote creation endpoint - not yet implemented",
		})
	case http.MethodPut:
		// Update existing quote
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Quote update endpoint - not yet implemented",
		})
	case http.MethodDelete:
		// Delete quote
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Quote deletion endpoint - not yet implemented",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAdminStats returns admin statistics
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	// Placeholder for admin statistics API
	uptime := time.Since(s.startTime)
	stats := map[string]interface{}{
		"totalQuotes":   s.animeService.GetTotalQuotes(),
		"uptime":        uptime.String(),
		"goVersion":     runtime.Version(),
		"serverAddress": s.address,
		"serverPort":    s.port,
	}
	respondJSON(w, http.StatusOK, stats)
}
