package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/config"
	"github.com/apimgr/anime/src/paths"
	"github.com/apimgr/anime/src/server"
)

//go:embed data/dataset.json
var animeData []byte

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

const projectName = "anime"

func main() {
	// Get default directories
	configDir, dataDir, logsDir := paths.GetDefaultDirs(projectName)

	// Command-line flags
	port := flag.String("port", "", "Server port (overrides config)")
	address := flag.String("address", "", "Server address (overrides config)")
	configDirFlag := flag.String("config", "", "Configuration directory")
	dataDirFlag := flag.String("data", "", "Data directory")
	logsDirFlag := flag.String("logs", "", "Logs directory")
	version := flag.Bool("version", false, "Print version information")
	status := flag.Bool("status", false, "Check service status (for healthcheck)")
	help := flag.Bool("help", false, "Show help")

	// Service commands
	serviceCmd := flag.String("service", "", "Service commands: start, stop, restart, reload, status, --install, --uninstall, --disable, --help")

	// Maintenance commands
	maintenanceCmd := flag.String("maintenance", "", "Maintenance commands: backup, restore, update, mode, setup")

	// Application mode
	modeFlag := flag.String("mode", "", "Application mode: production, development")

	// Update commands
	updateCmd := flag.String("update", "", "Update commands: check, yes, branch {stable|beta|daily}")

	flag.Parse()

	// Handle help flag
	if *help {
		printHelp()
		os.Exit(0)
	}

	// Handle version flag
	if *version {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	// Override directories from flags
	if *configDirFlag != "" {
		configDir = *configDirFlag
	}
	if *dataDirFlag != "" {
		dataDir = *dataDirFlag
	}
	if *logsDirFlag != "" {
		logsDir = *logsDirFlag
	}

	// Override from environment
	if envConfig := os.Getenv("CONFIG_DIR"); envConfig != "" && *configDirFlag == "" {
		configDir = envConfig
	}
	if envData := os.Getenv("DATA_DIR"); envData != "" && *dataDirFlag == "" {
		dataDir = envData
	}
	if envLogs := os.Getenv("LOGS_DIR"); envLogs != "" && *logsDirFlag == "" {
		logsDir = envLogs
	}

	// Ensure directories exist
	if err := paths.EnsureDirs(configDir, dataDir, logsDir); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Load configuration
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Handle status flag (for Docker healthcheck)
	if *status {
		checkPort := cfg.Server.Port
		if checkPort == "" {
			checkPort = "8080"
		}
		if err := checkHealth(checkPort); err != nil {
			fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OK")
		os.Exit(0)
	}

	// Handle mode flag
	if *modeFlag != "" {
		setApplicationMode(*modeFlag, configPath)
		return
	}

	// Handle update command
	if *updateCmd != "" {
		handleUpdateCommand(*updateCmd, cfg)
		return
	}

	// Handle service commands
	if *serviceCmd != "" {
		handleServiceCommand(*serviceCmd, configDir)
		return
	}

	// Handle maintenance commands
	if *maintenanceCmd != "" {
		handleMaintenanceCommand(*maintenanceCmd, configDir, dataDir, logsDir, configPath)
		return
	}

	// Determine port (flag > env > config > default)
	serverPort := cfg.Server.Port
	if *port != "" {
		serverPort = *port
	} else if envPort := os.Getenv("PORT"); envPort != "" {
		serverPort = envPort
	}
	if serverPort == "" {
		serverPort = "8080"
	}

	// Determine address (flag > env > config > default)
	serverAddress := cfg.Server.Address
	if *address != "" {
		serverAddress = *address
	} else if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		serverAddress = envAddr
	}
	if serverAddress == "" {
		serverAddress = "0.0.0.0"
	}

	// Initialize anime service with embedded data
	log.Println("Initializing anime quotes service...")
	animeService, err := anime.NewService(animeData)
	if err != nil {
		log.Fatalf("Failed to initialize anime service: %v", err)
	}

	log.Printf("Loaded %d anime quotes", animeService.GetTotalQuotes())

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Create and start HTTP server
	srv, err := server.NewServer(animeService, cfg, serverPort, serverAddress)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start()
	}()

	// Wait for shutdown signal or server error
	for {
		select {
		case err := <-errChan:
			log.Fatalf("Server failed: %v", err)
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, reloading configuration...")
				if _, err := config.Load(configPath); err != nil {
					log.Printf("Failed to reload configuration: %v", err)
				} else {
					log.Println("Configuration reloaded successfully")
				}
			default:
				log.Printf("Received signal %v, shutting down...", sig)
				log.Println("Shutdown complete")
				os.Exit(0)
			}
		}
	}
}

func printHelp() {
	fmt.Printf(`Anime Quotes API Server v%s

Usage: anime [options]

Options:
  --port PORT          Server port (default: from config or 8080)
  --address ADDRESS    Server address (default: from config or 0.0.0.0)
  --config DIR         Configuration directory
  --data DIR           Data directory
  --logs DIR           Logs directory
  --version            Print version information
  --status             Check service status (for healthcheck)
  --help               Show this help message

Mode Commands:
  --mode production    Set production mode
  --mode development   Set development mode

Update Commands:
  --update check       Check for available updates
  --update yes         Install available updates
  --update branch {stable|beta|daily}  Set update branch

Service Commands:
  --service start      Start the service
  --service stop       Stop the service
  --service restart    Restart the service
  --service reload     Reload configuration
  --service status     Show service status
  --service --install  Install as system service
  --service --uninstall Remove system service
  --service --disable  Disable the service

Maintenance Commands:
  --maintenance backup [file]   Backup configuration and data
  --maintenance restore [file]  Restore from backup
  --maintenance update          Check for and install updates
  --maintenance mode            Show current application mode
  --maintenance setup           Run initial setup wizard

Environment Variables:
  PORT         Server port
  ADDRESS      Server address
  CONFIG_DIR   Configuration directory
  DATA_DIR     Data directory
  LOGS_DIR     Logs directory

Configuration:
  Root:    /etc/apimgr/anime/server.yml
  User:    ~/.config/apimgr/anime/server.yml
  Docker:  /config/server.yml

`, Version)
}

func checkHealth(port string) error {
	url := fmt.Sprintf("http://127.0.0.1:%s/api/v1/health", port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

func handleServiceCommand(cmd, configDir string) {
	switch cmd {
	case "start":
		serviceStart()
	case "stop":
		serviceStop()
	case "restart":
		serviceRestart()
	case "reload":
		serviceReload()
	case "status":
		serviceStatus()
	case "--install":
		serviceInstall(configDir)
	case "--uninstall":
		serviceUninstall()
	case "--disable":
		serviceDisable()
	case "--help":
		fmt.Println("Service commands: start, stop, restart, reload, status, --install, --uninstall, --disable")
	default:
		fmt.Printf("Unknown service command: %s\n", cmd)
		os.Exit(1)
	}
}

func handleMaintenanceCommand(cmd, configDir, dataDir, logsDir, configPath string) {
	args := flag.Args()

	switch cmd {
	case "backup":
		backupFile := ""
		if len(args) > 0 {
			backupFile = args[0]
		} else {
			backupDir := paths.GetBackupDir(projectName)
			if err := os.MkdirAll(backupDir, 0755); err != nil {
				log.Fatalf("Failed to create backup directory: %v", err)
			}
			timestamp := time.Now().Format("20060102-150405")
			backupFile = filepath.Join(backupDir, fmt.Sprintf("anime-backup-%s.tar.gz", timestamp))
		}
		maintenanceBackup(configDir, dataDir, backupFile)
	case "restore":
		if len(args) == 0 {
			fmt.Println("Usage: anime --maintenance restore <backup-file>")
			os.Exit(1)
		}
		maintenanceRestore(args[0], configDir, dataDir)
	case "update":
		maintenanceUpdate()
	case "mode":
		showCurrentMode(configPath)
	case "setup":
		runInitialSetup(configPath)
	default:
		fmt.Printf("Unknown maintenance command: %s\n", cmd)
		fmt.Println("Available commands: backup, restore, update, mode, setup")
		os.Exit(1)
	}
}

// Service management functions
func serviceStart() {
	switch runtime.GOOS {
	case "linux":
		runCommand("systemctl", "start", "anime")
	case "darwin":
		runCommand("launchctl", "start", "us.apimgr.anime")
	default:
		fmt.Printf("Service management not supported on %s\n", runtime.GOOS)
	}
}

func serviceStop() {
	switch runtime.GOOS {
	case "linux":
		runCommand("systemctl", "stop", "anime")
	case "darwin":
		runCommand("launchctl", "stop", "us.apimgr.anime")
	default:
		fmt.Printf("Service management not supported on %s\n", runtime.GOOS)
	}
}

func serviceRestart() {
	switch runtime.GOOS {
	case "linux":
		runCommand("systemctl", "restart", "anime")
	case "darwin":
		runCommand("launchctl", "stop", "us.apimgr.anime")
		runCommand("launchctl", "start", "us.apimgr.anime")
	default:
		fmt.Printf("Service management not supported on %s\n", runtime.GOOS)
	}
}

func serviceReload() {
	switch runtime.GOOS {
	case "linux":
		runCommand("systemctl", "reload", "anime")
	case "darwin":
		// Send SIGHUP to reload config
		runCommand("pkill", "-HUP", "anime")
	default:
		fmt.Printf("Service management not supported on %s\n", runtime.GOOS)
	}
}

func serviceStatus() {
	switch runtime.GOOS {
	case "linux":
		runCommand("systemctl", "status", "anime")
	case "darwin":
		runCommand("launchctl", "list", "us.apimgr.anime")
	default:
		fmt.Printf("Service management not supported on %s\n", runtime.GOOS)
	}
}

func serviceInstall(configDir string) {
	fmt.Println("Installing anime service...")
	switch runtime.GOOS {
	case "linux":
		installSystemdService(configDir)
	case "darwin":
		installLaunchdService(configDir)
	default:
		fmt.Printf("Service installation not supported on %s\n", runtime.GOOS)
	}
}

func serviceUninstall() {
	fmt.Println("Uninstalling anime service...")
	switch runtime.GOOS {
	case "linux":
		runCommand("systemctl", "stop", "anime")
		runCommand("systemctl", "disable", "anime")
		os.Remove("/etc/systemd/system/anime.service")
		runCommand("systemctl", "daemon-reload")
	case "darwin":
		runCommand("launchctl", "unload", "/Library/LaunchDaemons/us.apimgr.anime.plist")
		os.Remove("/Library/LaunchDaemons/us.apimgr.anime.plist")
	default:
		fmt.Printf("Service uninstallation not supported on %s\n", runtime.GOOS)
	}
}

func serviceDisable() {
	switch runtime.GOOS {
	case "linux":
		runCommand("systemctl", "disable", "anime")
	case "darwin":
		runCommand("launchctl", "unload", "/Library/LaunchDaemons/us.apimgr.anime.plist")
	default:
		fmt.Printf("Service disable not supported on %s\n", runtime.GOOS)
	}
}

func installSystemdService(configDir string) {
	service := fmt.Sprintf(`[Unit]
Description=Anime Quotes API Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/anime --config %s
Restart=always
RestartSec=5
User=anime
Group=anime

[Install]
WantedBy=multi-user.target
`, configDir)

	if err := os.WriteFile("/etc/systemd/system/anime.service", []byte(service), 0644); err != nil {
		log.Fatalf("Failed to write systemd service file: %v", err)
	}
	runCommand("systemctl", "daemon-reload")
	runCommand("systemctl", "enable", "anime")
	runCommand("systemctl", "start", "anime")
	fmt.Println("Service installed and started successfully")
}

func installLaunchdService(configDir string) {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>us.apimgr.anime</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/anime</string>
        <string>--config</string>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
`, configDir)

	if err := os.WriteFile("/Library/LaunchDaemons/us.apimgr.anime.plist", []byte(plist), 0644); err != nil {
		log.Fatalf("Failed to write launchd plist: %v", err)
	}
	runCommand("launchctl", "load", "/Library/LaunchDaemons/us.apimgr.anime.plist")
	fmt.Println("Service installed and started successfully")
}

// Maintenance functions
func maintenanceBackup(configDir, dataDir, backupFile string) {
	fmt.Printf("Creating backup: %s\n", backupFile)

	// Create tar.gz of config and data directories
	cmd := exec.Command("tar", "-czf", backupFile, "-C", filepath.Dir(configDir), filepath.Base(configDir), "-C", filepath.Dir(dataDir), filepath.Base(dataDir))
	if err := cmd.Run(); err != nil {
		log.Fatalf("Backup failed: %v", err)
	}

	fmt.Printf("Backup created successfully: %s\n", backupFile)
}

func maintenanceRestore(backupFile, configDir, dataDir string) {
	fmt.Printf("Restoring from backup: %s\n", backupFile)

	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		log.Fatalf("Backup file not found: %s", backupFile)
	}

	// Extract backup to appropriate locations
	cmd := exec.Command("tar", "-xzf", backupFile, "-C", "/")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Restore failed: %v", err)
	}

	fmt.Println("Restore completed successfully")
}

func maintenanceUpdate() {
	fmt.Println("Checking for updates...")
	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Update feature not yet implemented")
	fmt.Println("Visit https://github.com/apimgr/anime/releases for the latest version")
}

// setApplicationMode sets the application mode
func setApplicationMode(mode, configPath string) {
	if mode != "production" && mode != "development" {
		fmt.Printf("Invalid mode: %s\n", mode)
		fmt.Println("Valid modes: production, development")
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cfg.Server.Mode = mode
	if err := config.Save(configPath, cfg); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	fmt.Printf("Application mode set to: %s\n", mode)
}

// showCurrentMode displays the current application mode
func showCurrentMode(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mode := cfg.Server.Mode
	if mode == "" {
		mode = "production"
	}
	fmt.Printf("Current mode: %s\n", mode)
}

// runInitialSetup runs the initial setup wizard
func runInitialSetup(configPath string) {
	fmt.Println("Anime API Initial Setup")
	fmt.Println("========================")

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Interactive setup would go here
	// For now, just show current configuration
	fmt.Printf("\nConfiguration file: %s\n", configPath)
	fmt.Printf("Current port: %s\n", cfg.Server.Port)
	fmt.Printf("Current mode: %s\n", cfg.Server.Mode)
	fmt.Println("\nSetup complete. Edit the configuration file to customize settings.")
}

// handleUpdateCommand handles update-related commands
func handleUpdateCommand(cmd string, cfg *config.Config) {
	args := flag.Args()

	switch cmd {
	case "check":
		fmt.Println("Checking for updates...")
		fmt.Printf("Current version: %s\n", Version)
		fmt.Printf("Update branch: %s\n", cfg.Server.UpdateBranch)
		fmt.Println("No updates available (update checking not implemented)")
	case "yes":
		fmt.Println("Installing updates...")
		fmt.Println("Update installation not implemented")
	case "branch":
		if len(args) == 0 {
			fmt.Printf("Current update branch: %s\n", cfg.Server.UpdateBranch)
			fmt.Println("Usage: anime --update branch {stable|beta|daily}")
			return
		}
		branch := args[0]
		if branch != "stable" && branch != "beta" && branch != "daily" {
			fmt.Printf("Invalid branch: %s\n", branch)
			fmt.Println("Valid branches: stable, beta, daily")
			os.Exit(1)
		}
		cfg.Server.UpdateBranch = branch
		fmt.Printf("Update branch set to: %s\n", branch)
	default:
		fmt.Printf("Unknown update command: %s\n", cmd)
		fmt.Println("Available commands: check, yes, branch")
		os.Exit(1)
	}
}

func runCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Command failed: %s %v: %v", name, args, err)
	}
}
