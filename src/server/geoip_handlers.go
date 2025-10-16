package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// handleGeoIPRequest handles GeoIP lookup for the request IP
func (s *Server) handleGeoIPRequest(w http.ResponseWriter, r *http.Request) {
	// Extract IP from request
	ip := getRequestIP(r)

	// Parse IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		s.jsonError(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	// Lookup
	location, err := s.geoipService.Lookup(parsedIP)
	if err != nil {
		s.jsonError(w, "GeoIP lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    location,
	}, http.StatusOK)
}

// handleGeoIPLookup handles GeoIP lookup for a specific IP
func (s *Server) handleGeoIPLookup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ipStr := vars["ip"]

	// Remove brackets if present (from URL encoding)
	ipStr = strings.Trim(ipStr, "[]")

	// Parse IP
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		s.jsonError(w, "Invalid IP address: "+ipStr, http.StatusBadRequest)
		return
	}

	// Lookup
	location, err := s.geoipService.Lookup(parsedIP)
	if err != nil {
		s.jsonError(w, "GeoIP lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    location,
	}, http.StatusOK)
}

// handleGeoIPUpdate handles manual GeoIP database update (admin only)
func (s *Server) handleGeoIPUpdate(w http.ResponseWriter, r *http.Request) {
	if s.geoipService == nil {
		s.jsonError(w, "GeoIP service not available", http.StatusServiceUnavailable)
		return
	}

	// Update databases
	err := s.geoipService.UpdateDatabases()
	if err != nil {
		s.jsonError(w, "Failed to update databases: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"success": true,
		"message": "GeoIP databases updated successfully",
	}, http.StatusOK)
}

// getRequestIP extracts the real IP address from the request
func getRequestIP(r *http.Request) string {
	// Check X-Forwarded-For header (proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// jsonResponse sends a JSON response
func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// jsonError sends a JSON error response
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	s.jsonResponse(w, map[string]interface{}{
		"success": false,
		"error": map[string]string{
			"message": message,
		},
	}, status)
}
