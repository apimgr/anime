package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/database"
	"github.com/apimgr/anime/src/geoip"
	"github.com/apimgr/anime/src/paths"
	"github.com/apimgr/anime/src/scheduler"
	"github.com/apimgr/anime/src/server"
)

//go:embed data/anime.json
var animeData []byte

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	// Command-line flags
	port := flag.String("port", getEnv("PORT", "8080"), "Server port")
	address := flag.String("address", getEnv("ADDRESS", "::"), "Server address (:: for dual-stack IPv4+IPv6, 0.0.0.0 for IPv4 only)")
	dataDir := flag.String("data", getEnv("DATA_DIR", "/data"), "Data directory for database")
	version := flag.Bool("version", false, "Print version information")
	status := flag.Bool("status", false, "Check service status (for healthcheck)")
	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	// Handle status flag (for Docker healthcheck)
	if *status {
		// Simple health check - try to connect to the server
		// For now, just exit successfully if the binary runs
		os.Exit(0)
	}

	// Initialize anime service with embedded data
	log.Println("Initializing anime quotes service...")
	animeService, err := anime.NewService(animeData)
	if err != nil {
		log.Fatalf("Failed to initialize anime service: %v", err)
	}

	log.Printf("Loaded %d anime quotes", animeService.GetTotalQuotes())

	// Get config directory for GeoIP databases
	configDir := paths.GetConfigDir()

	// Initialize GeoIP service
	log.Println("Initializing GeoIP service...")
	geoipService, err := geoip.NewService(configDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize GeoIP service: %v", err)
		log.Printf("GeoIP lookups will not be available")
		geoipService = nil
	}

	// Initialize database (optional - can be nil for development)
	var db *database.DB
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Printf("Warning: Failed to create data directory: %v", err)
		log.Printf("Running without database support")
	} else {
		db, err = database.NewDB(*dataDir)
		if err != nil {
			log.Printf("Warning: Failed to initialize database: %v", err)
			log.Printf("Running without database support")
		} else {
			log.Println("Database initialized successfully")
		}
	}

	// Initialize scheduler for GeoIP database updates
	sched := scheduler.New()
	if geoipService != nil {
		// Schedule weekly updates (Sunday at 3:00 AM)
		err = sched.AddTask("geoip-update", "0 3 * * 0", func() error {
			return geoipService.UpdateDatabases()
		})
		if err != nil {
			log.Printf("Warning: Failed to schedule GeoIP updates: %v", err)
		} else {
			log.Println("Scheduled weekly GeoIP database updates (Sunday 3:00 AM)")
			sched.Start()
		}
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create and start HTTP server
	srv, err := server.NewServer(animeService, geoipService, db, *port, *address)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start()
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-errChan:
		log.Fatalf("Server failed: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
		sched.Stop()
		if geoipService != nil {
			geoipService.Close()
		}
		log.Println("Shutdown complete")
	}
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
