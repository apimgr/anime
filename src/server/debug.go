package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"expvar"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/apimgr/anime/src/database"
	"github.com/gorilla/mux"
)

func (s *Server) operatorAuthMiddleware(next http.Handler) http.Handler {
	configuredHash := sha256.Sum256([]byte(s.cfg.Server.Token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := extractAuthToken(r)
		providedHash := sha256.Sum256([]byte(provided))
		valid := subtle.ConstantTimeCompare(configuredHash[:], providedHash[:])
		valid &= subtle.ConstantTimeEq(int32(len(s.cfg.Server.Token)), int32(len(provided)))
		if s.cfg.Server.Token == "" || valid != 1 {
			time.Sleep(100 * time.Millisecond)
			writeProblem(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) debugMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/", http.DefaultServeMux)
	mux.Handle("/debug/pprof", http.DefaultServeMux)
	mux.Handle("/debug/vars", expvar.Handler())
	mux.HandleFunc("/debug/config", s.handleDebugConfig)
	mux.HandleFunc("/debug/routes", s.handleDebugRoutes)
	mux.HandleFunc("/debug/cache", s.handleDebugCache)
	mux.HandleFunc("/debug/db", s.handleDebugDB)
	mux.HandleFunc("/debug/scheduler", s.handleDebugScheduler)
	mux.HandleFunc("/debug/memory", s.handleDebugMemory)
	mux.HandleFunc("/debug/goroutines", s.handleDebugGoroutines)
	return mux
}

func (s *Server) handleDebugConfig(w http.ResponseWriter, _ *http.Request) {
	databaseInfo := map[string]string{"driver": "unavailable", "locality": "unavailable"}
	if s.database != nil {
		databaseInfo["driver"] = s.database.Driver
		databaseInfo["locality"] = s.database.Locality
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"server": map[string]any{
			"port": s.cfg.Server.Port, "fqdn": s.cfg.Server.FQDN, "address": s.cfg.Server.Address,
			"mode": s.cfg.Server.Mode, "database": databaseInfo, "metrics": s.cfg.Server.Metrics,
		},
		"web_ui": map[string]any{"theme": s.cfg.WebUI.Theme},
	})
}

func (s *Server) handleDebugRoutes(w http.ResponseWriter, _ *http.Request) {
	routes := make([]map[string]any, 0)
	err := s.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		methods, _ := route.GetMethods()
		routes = append(routes, map[string]any{"path": path, "methods": methods})
		return nil
	})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Unable to list routes")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"count": len(routes), "routes": routes})
}

func (s *Server) handleDebugCache(w http.ResponseWriter, _ *http.Request) {
	stats := map[string]any{"cli_update": map[string]any{"cached": false, "versions": 0}}
	if s.cliUpdates != nil {
		s.cliUpdates.mu.Lock()
		stats["cli_update"] = map[string]any{
			"cached": time.Now().Before(s.cliUpdates.expires), "expires_at": s.cliUpdates.expires,
			"versions": len(s.cliUpdates.versions),
		}
		s.cliUpdates.mu.Unlock()
	}
	respondJSON(w, http.StatusOK, stats)
}

func (s *Server) handleDebugDB(w http.ResponseWriter, _ *http.Request) {
	if s.database == nil {
		respondJSON(w, http.StatusOK, map[string]any{"status": "unavailable"})
		return
	}
	stats := s.database.SQL.Stats()
	respondJSON(w, http.StatusOK, map[string]any{
		"driver": s.database.Driver, "locality": s.database.Locality,
		"open_connections": stats.OpenConnections, "in_use": stats.InUse,
		"idle": stats.Idle, "wait_count": stats.WaitCount, "wait_duration": stats.WaitDuration.String(),
	})
}

func (s *Server) handleDebugScheduler(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		respondJSON(w, http.StatusOK, map[string]any{"status": "unavailable", "tasks": []any{}})
		return
	}
	rows, err := database.QueryContext(r.Context(), s.database.SQL, `SELECT id, name, enabled, schedule, last_run, next_run, last_status, last_error, run_count, fail_count FROM scheduler_tasks ORDER BY id`)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Unable to read scheduler status")
		return
	}
	defer rows.Close()
	tasks := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, schedule string
		var enabled, runCount, failCount int
		var lastRun, nextRun sql.NullInt64
		var lastStatus, lastError sql.NullString
		if err := rows.Scan(&id, &name, &enabled, &schedule, &lastRun, &nextRun, &lastStatus, &lastError, &runCount, &failCount); err != nil {
			writeProblem(w, http.StatusInternalServerError, "Unable to read scheduler status")
			return
		}
		tasks = append(tasks, map[string]any{
			"id": id, "name": name, "enabled": enabled != 0, "schedule": schedule,
			"last_run": nullableInt(lastRun), "next_run": nullableInt(nextRun),
			"last_status": nullableString(lastStatus), "last_error": nullableString(lastError),
			"run_count": runCount, "fail_count": failCount,
		})
	}
	if err := rows.Err(); err != nil {
		writeProblem(w, http.StatusInternalServerError, "Unable to read scheduler status")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "running", "tasks": tasks})
}

func nullableString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableInt(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func (s *Server) handleDebugMemory(w http.ResponseWriter, _ *http.Request) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	respondJSON(w, http.StatusOK, map[string]any{
		"alloc_bytes": memory.Alloc, "total_alloc_bytes": memory.TotalAlloc,
		"sys_bytes": memory.Sys, "heap_objects": memory.HeapObjects,
		"num_gc": memory.NumGC, "goroutines": runtime.NumGoroutine(),
	})
}

func (s *Server) handleDebugGoroutines(w http.ResponseWriter, _ *http.Request) {
	buffer := make([]byte, 1<<20)
	count := runtime.Stack(buffer, true)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(buffer[:count])
}

func extractAuthToken(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("Authorization")); value != "" {
		parts := strings.Fields(value)
		if len(parts) == 2 {
			switch strings.ToLower(parts[0]) {
			case "bearer":
				return parts[1]
			case "basic":
				decoded, err := base64.StdEncoding.DecodeString(parts[1])
				if err == nil {
					_, password, found := strings.Cut(string(decoded), ":")
					if found {
						return password
					}
					return string(decoded)
				}
			}
		}
		return value
	}
	for _, name := range []string{
		"X-API-Key", "API-Key", "X-Auth-Token", "X-Access-Token", "X-Token", "Token",
		"X-CSRF-Token", "X-XSRF-Token", "X-Session-ID", "X-Service-Token", "X-Internal-Token",
	} {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			return value
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}
