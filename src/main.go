package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apimgr/anime/src/anime"
	"github.com/apimgr/anime/src/config"
	"github.com/apimgr/anime/src/database"
	"github.com/apimgr/anime/src/mode"
	"github.com/apimgr/anime/src/paths"
	"github.com/apimgr/anime/src/server"
	"github.com/apimgr/anime/src/service"
	"github.com/apimgr/anime/src/updater"
	"golang.org/x/term"
)

//go:embed data/dataset.json
var animeData []byte

var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildEpoch   = "0"
	OfficialSite = ""
)

const projectName = "anime"

var updaterClientFactory = updater.NewClient

func main() {
	os.Args = append([]string{os.Args[0]}, normalizeUpdateArgs(os.Args[1:])...)
	// Resolve directory mode before startup initialization and keep it fixed.
	directories := paths.ResolveDefaultDirectories(projectName)
	configDir, dataDir, logsDir := directories.Config, directories.Data, directories.Logs
	cacheDir, backupDir, pidFile, databaseDir := directories.Cache, directories.Backup, directories.PID, directories.DB

	// Command-line flags
	port := flag.String("port", "", "Server port (overrides config)")
	address := flag.String("address", "", "Server address (overrides config)")
	configDirFlag := flag.String("config", "", "Configuration directory")
	dataDirFlag := flag.String("data", "", "Data directory")
	cacheDirFlag := flag.String("cache", "", "Cache directory")
	logsDirFlag := flag.String("log", "", "Log directory")
	backupDirFlag := flag.String("backup", "", "Backup directory")
	pidFileFlag := flag.String("pid", "", "PID file path")
	baseURLFlag := flag.String("baseurl", "", "URL path prefix")
	version := flag.Bool("version", false, "Print version information")
	flag.BoolVar(version, "v", false, "Print version information")
	status := flag.Bool("status", false, "Check service status (for healthcheck)")
	help := flag.Bool("help", false, "Show help")
	flag.BoolVar(help, "h", false, "Show help")
	debugFlag := flag.Bool("debug", false, "Enable debug logging and debug endpoints")
	colorFlag := flag.String("color", "auto", "Color output: auto, yes, no")
	_ = flag.String("lang", "en", "Display language")
	shell := flag.String("shell", "", "Print shell completions: bash, zsh, fish")

	// Service commands
	serviceCmd := flag.String("service", "", "Service commands: start, stop, restart, reload, status, --install, --uninstall, --disable, --help")

	// Maintenance commands
	maintenanceCmd := flag.String("maintenance", "", "Maintenance commands: backup, restore, update, mode, setup, pgp, security")

	// Application mode
	modeFlag := flag.String("mode", "", "Application mode: production, development, debug")

	// Update commands
	updateCmd := flag.String("update", "", "Update commands: check, yes, branch {stable|beta|daily}")

	flag.Parse()

	// --color=no (or TERM=dumb under auto) disables colored output for every component via NO_COLOR; --color=yes overrides an inherited NO_COLOR
	switch *colorFlag {
	case "no":
		_ = os.Setenv("NO_COLOR", "1")
	case "yes":
		_ = os.Unsetenv("NO_COLOR")
	}

	// Handle help flag
	if *help {
		printHelp()
		os.Exit(0)
	}

	// Handle version flag
	if *version {
		printVersion()
		os.Exit(0)
	}

	if *shell != "" {
		printShellCommand(*shell, flag.Args())
		return
	}

	server.Version = Version
	server.CommitID = CommitID
	server.BuildDate = buildDate()

	// Override directories from flags
	if *configDirFlag != "" {
		configDir = *configDirFlag
	}
	if *dataDirFlag != "" {
		dataDir = *dataDirFlag
		databaseDir = filepath.Join(dataDir, "db")
	}
	if *cacheDirFlag != "" {
		cacheDir = *cacheDirFlag
	}
	if *logsDirFlag != "" {
		logsDir = *logsDirFlag
	}
	if *backupDirFlag != "" {
		backupDir = *backupDirFlag
	}
	if *pidFileFlag != "" {
		pidFile = *pidFileFlag
	}

	// Override from environment
	if envConfig := os.Getenv("CONFIG_DIR"); envConfig != "" && *configDirFlag == "" {
		configDir = envConfig
	}
	if envData := os.Getenv("DATA_DIR"); envData != "" && *dataDirFlag == "" {
		dataDir = envData
		databaseDir = filepath.Join(dataDir, "db")
	}
	if envCache := os.Getenv("CACHE_DIR"); envCache != "" && *cacheDirFlag == "" {
		cacheDir = envCache
	}
	if envLogs := os.Getenv("LOG_DIR"); envLogs != "" && *logsDirFlag == "" {
		logsDir = envLogs
	}
	if envBackup := os.Getenv("BACKUP_DIR"); envBackup != "" && *backupDirFlag == "" {
		backupDir = envBackup
	}
	if envPID := os.Getenv("PID_FILE"); envPID != "" && *pidFileFlag == "" {
		pidFile = envPID
	}
	if envDatabase := os.Getenv("DATABASE_DIR"); envDatabase != "" {
		databaseDir = envDatabase
	}

	directories.Config = configDir
	directories.Data = dataDir
	directories.Cache = cacheDir
	directories.Logs = logsDir
	directories.Backup = backupDir
	directories.PID = pidFile
	directories.DB = databaseDir

	// Ensure directories exist
	if err := paths.EnsureRuntimeDirectories(directories); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Load configuration
	configPath := filepath.Join(configDir, "server.yml")
	_, configStatErr := os.Stat(configPath)
	firstRun := os.IsNotExist(configStatErr)
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	if firstRun {
		if *port != "" {
			cfg.Server.Port = *port
		} else if environmentPort := os.Getenv("PORT"); environmentPort != "" {
			cfg.Server.Port = environmentPort
		} else if paths.IsRunningInContainer() {
			cfg.Server.Port = "80"
		}
		if err := config.Save(configPath, cfg); err != nil {
			log.Fatalf("Failed to save first-run configuration: %v", err)
		}
	}
	if *baseURLFlag != "" {
		cfg.Server.BaseURL = *baseURLFlag
	}

	// Handle status flag (for Docker healthcheck)
	if *status {
		checkPort := cfg.Server.Port
		if envPort := os.Getenv("PORT"); envPort != "" {
			checkPort = envPort
		}
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

	// Handle update command
	if *updateCmd != "" {
		if err := handleUpdateCommand(*updateCmd, cfg, configPath); err != nil {
			log.Fatalf("Update failed: %v", err)
		}
		return
	}

	// Handle service commands
	if *serviceCmd != "" {
		handleServiceCommand(*serviceCmd, configDir)
		return
	}

	// Handle maintenance commands
	if *maintenanceCmd != "" {
		handleMaintenanceCommand(*maintenanceCmd, configDir, dataDir, logsDir, backupDir, configPath)
		return
	}

	modeFlagSet := false
	debugFlagSet := false
	flag.Visit(func(parsedFlag *flag.Flag) {
		switch parsedFlag.Name {
		case "mode":
			modeFlagSet = true
		case "debug":
			debugFlagSet = true
		}
	})
	if err := mode.ConfigureAppModeWithDefault(cfg.Server.Mode, *modeFlag, modeFlagSet, *debugFlag, debugFlagSet); err != nil {
		log.Fatalf("Failed to configure application mode: %v", err)
	}
	cfg.Server.Mode = string(mode.GetCurrentAppMode())

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
	} else if envAddr := os.Getenv("LISTEN"); envAddr != "" {
		serverAddress = envAddr
	}
	if serverAddress == "" {
		serverAddress = "0.0.0.0"
	}

	db, err := database.Open(cfg.Server.Database, databaseDir)
	if err != nil {
		log.Fatalf("Failed to open application database: %v", err)
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
	srv, err := server.NewServer(animeService, cfg, serverPort, serverAddress, db)
	if err != nil {
		_ = db.SQL.Close()
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
				if err := db.SQL.Close(); err != nil {
					log.Printf("Failed to close database: %v", err)
				}
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
  --port PORT          Server port (default: random 64xxx, 80 in container)
  --address ADDRESS    Server address (default: from config or [::])
  --config DIR         Configuration directory
  --data DIR           Data directory
  --cache DIR          Cache directory
  --log DIR            Logs directory
  --backup DIR         Backup directory
  --pid FILE           PID file path
  --baseurl PATH       URL path prefix (default: /)
  -v, --version        Print version information
  --debug              Enable debug logging and debug endpoints
  --color MODE         Color output: auto, yes, no
  --lang LANG          Display language
  --shell SHELL        Print shell completions: bash, zsh, fish
  --status             Check service status (for healthcheck)
  -h, --help           Show this help message

Mode Commands:
  --mode production    Run in production mode
  --mode development   Run in development mode
  --mode debug         Run in debug mode

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
  --maintenance backup test     Verify the backed-up PGP key can be decrypted
  --maintenance restore [file]  Restore from backup
  --maintenance update          Check for and install updates
  --maintenance mode            Show current application mode
  --maintenance setup           Run initial setup wizard
  --maintenance pgp generate    Generate the security reporting key
  --maintenance pgp export public [path]  Export its public key

Environment Variables:
  PORT         Server port
  ADDRESS      Server address
  CONFIG_DIR   Configuration directory
  DATA_DIR     Data directory
  CACHE_DIR    Cache directory
  LOG_DIR      Log directory
  BACKUP_DIR   Backup directory
  PID_FILE     PID file path
  LISTEN       Listen address

Configuration:
  Root:    /etc/apimgr/anime/server.yml
  User:    ~/.config/apimgr/anime/server.yml
  Docker:  /config/anime/server.yml

`, Version)
}

func checkHealth(port string) error {
	url := fmt.Sprintf("http://127.0.0.1:%s/api/v1/server/healthz", port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
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

func handleMaintenanceCommand(cmd, configDir, dataDir, logsDir, backupDir, configPath string) {
	args := flag.Args()

	switch cmd {
	case "backup":
		if len(args) > 0 && args[0] == "test" {
			if len(args) != 1 {
				log.Fatal("usage: anime --maintenance backup test")
			}
			if err := maintenanceBackupTest(configDir, dataDir, configPath); err != nil {
				log.Fatalf("Backup test failed: %v", err)
			}
			return
		}
		backupFile := ""
		if len(args) > 0 {
			backupFile = args[0]
		} else {
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
		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		updateCommand := "yes"
		if len(args) > 0 {
			updateCommand = args[0]
		}
		if err := handleUpdateCommand(updateCommand, cfg, configPath); err != nil {
			log.Fatalf("Update failed: %v", err)
		}
	case "mode":
		if len(args) == 0 {
			showCurrentMode(configPath)
			return
		}
		setApplicationMode(args[0], configPath)
	case "setup":
		runInitialSetup(configPath)
	case "pgp":
		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		if err := handlePGPMaintenance(args, cfg, dataDir); err != nil {
			log.Fatalf("PGP maintenance failed: %v", err)
		}
	case "security":
		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		if err := handleSecurityMaintenance(args, cfg, dataDir); err != nil {
			log.Fatalf("Security maintenance failed: %v", err)
		}
	default:
		fmt.Printf("Unknown maintenance command: %s\n", cmd)
		fmt.Println("Available commands: backup, restore, update, mode, setup, pgp, security")
		os.Exit(1)
	}
}

func handlePGPMaintenance(args []string, cfg *config.Config, dataDir string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: anime --maintenance pgp {generate|rotate|publish|import <file>|export public [path]|export private <path>|delete}")
	}
	if err := authorizeMaintenance(cfg); err != nil {
		return err
	}
	db, err := database.Open(cfg.Server.Database, filepath.Join(dataDir, "db"))
	if err != nil {
		return err
	}
	defer func() {
		if err := db.SQL.Close(); err != nil {
			log.Printf("database close failed: %v", err)
		}
	}()
	installationSecret, err := database.AppSecret(context.Background(), db.SQL, "installation_secret")
	if err != nil {
		return err
	}
	securityDir := filepath.Join(cfg.ConfigDir, "security")
	switch args[0] {
	case "generate":
		contact := strings.TrimSpace(cfg.Server.Contact.Security.Email)
		if contact == "" {
			contact = strings.TrimSpace(cfg.Server.Contact.Admin.Email)
		}
		if contact == "" && cfg.Server.FQDN != "" {
			contact = "security@" + strings.TrimSpace(cfg.Server.FQDN)
		}
		if contact == "" {
			return fmt.Errorf("configure server.contact.security.email or server.fqdn before generating the security key")
		}
		fingerprint, err := server.GenerateSecurityPGPKey(cfg.ConfigDir, installationSecret, "Anime Security", contact, time.Now().UTC())
		if err != nil {
			return err
		}
		created := time.Now().UTC()
		_, err = db.SQL.Exec(`INSERT INTO security_pgp_metadata (id, fingerprint, created_at, expires_at, revoked) VALUES (1, ?, ?, ?, 0) ON CONFLICT(id) DO UPDATE SET fingerprint=excluded.fingerprint, created_at=excluded.created_at, expires_at=excluded.expires_at, last_rotated_at=NULL, revoked=0`, fingerprint, created.Format(time.RFC3339), created.AddDate(2, 0, 0).Format(time.RFC3339))
		if err != nil {
			_ = os.Remove(filepath.Join(securityDir, "pgp.pub.asc"))
			_ = os.Remove(filepath.Join(securityDir, "pgp.priv.asc.enc"))
			return err
		}
		cfg.WebSecurity.PublishPGPKey = true
		if err := config.Save(filepath.Join(cfg.ConfigDir, "server.yml"), cfg); err != nil {
			return err
		}
		fmt.Printf("Generated security PGP key %s\n", fingerprint)
		if err := server.PublishSecurityPGPKey(context.Background(), cfg, db.SQL); err != nil {
			log.Printf("security.pgp_publish_pending reason=%q", err.Error())
		}
		return nil
	case "import":
		if len(args) != 2 {
			return fmt.Errorf("usage: anime --maintenance pgp import <file>")
		}
		if err := authorizeMaintenance(cfg); err != nil {
			return err
		}
		private, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		fingerprint, identityEmail, err := server.InspectSecurityPGPPrivateKey(private)
		if err != nil {
			return err
		}
		if existingPublic, readErr := os.ReadFile(filepath.Join(securityDir, "pgp.pub.asc")); readErr == nil {
			existingFingerprint, parseErr := server.InspectSecurityPGPPublicKey(existingPublic)
			if parseErr != nil {
				return parseErr
			}
			if existingFingerprint != fingerprint {
				return fmt.Errorf("a different security key is active; rotate it rather than importing over retained reports")
			}
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
		var storedFingerprint string
		metadataErr := db.SQL.QueryRow(`SELECT fingerprint FROM security_pgp_metadata WHERE id = 1`).Scan(&storedFingerprint)
		if metadataErr == nil && storedFingerprint != fingerprint {
			return fmt.Errorf("imported key does not match the recorded security key; retained reports may require the original key")
		}
		if metadataErr == sql.ErrNoRows {
			var encryptedReports int
			if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM security_reports WHERE body_alg = 'pgp'`).Scan(&encryptedReports); err != nil {
				return err
			}
			if encryptedReports > 0 {
				return fmt.Errorf("cannot import an unknown key while PGP-encrypted reports exist")
			}
		} else if metadataErr != nil {
			return metadataErr
		}
		expectedEmail := strings.TrimSpace(cfg.Server.Contact.Security.Email)
		if expectedEmail == "" {
			expectedEmail = strings.TrimSpace(cfg.Server.Contact.Admin.Email)
		}
		if expectedEmail == "" && strings.TrimSpace(cfg.Server.FQDN) != "" {
			expectedEmail = "security@" + strings.TrimSpace(cfg.Server.FQDN)
		}
		if expectedEmail != "" && !strings.EqualFold(expectedEmail, identityEmail) {
			fmt.Printf("Warning: imported key identity %s does not match configured security contact %s. Type IMPORT KEY to override: ", identityEmail, expectedEmail)
			confirmation, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
			if readErr != nil || strings.TrimSpace(confirmation) != "IMPORT KEY" {
				return fmt.Errorf("key identity mismatch; import cancelled")
			}
		}
		fingerprint, _, err = server.ImportSecurityPGPPrivateKey(cfg.ConfigDir, installationSecret, private)
		if err != nil {
			return err
		}
		created := time.Now().UTC()
		_, err = db.SQL.Exec(`INSERT INTO security_pgp_metadata (id, fingerprint, created_at, expires_at, revoked) VALUES (1, ?, ?, ?, 0) ON CONFLICT(id) DO UPDATE SET fingerprint=excluded.fingerprint, created_at=excluded.created_at, expires_at=excluded.expires_at, last_rotated_at=NULL, revoked=0`, fingerprint, created.Format(time.RFC3339), created.AddDate(2, 0, 0).Format(time.RFC3339))
		if err != nil {
			return err
		}
		cfg.WebSecurity.PublishPGPKey = true
		if err := config.Save(filepath.Join(cfg.ConfigDir, "server.yml"), cfg); err != nil {
			return err
		}
		if err := server.PublishSecurityPGPKey(context.Background(), cfg, db.SQL); err != nil {
			log.Printf("security.pgp_publish_pending reason=%q", err.Error())
		}
		fmt.Printf("Imported security PGP key %s\n", fingerprint)
		return nil
	case "publish":
		if len(args) != 1 {
			return fmt.Errorf("usage: anime --maintenance pgp publish")
		}
		return server.PublishSecurityPGPKey(context.Background(), cfg, db.SQL)
	case "rotate":
		if len(args) != 1 {
			return fmt.Errorf("usage: anime --maintenance pgp rotate")
		}
		contact := strings.TrimSpace(cfg.Server.Contact.Security.Email)
		if contact == "" {
			contact = strings.TrimSpace(cfg.Server.Contact.Admin.Email)
		}
		if contact == "" && cfg.Server.FQDN != "" {
			contact = "security@" + strings.TrimSpace(cfg.Server.FQDN)
		}
		if contact == "" {
			return fmt.Errorf("configure server.contact.security.email or server.fqdn before rotating the security key")
		}
		fingerprint, err := server.RotateSecurityPGPKey(cfg.ConfigDir, installationSecret, db.SQL, "Anime Security", contact, time.Now().UTC())
		if err != nil {
			return err
		}
		cfg.WebSecurity.PublishPGPKey = true
		if err := config.Save(filepath.Join(cfg.ConfigDir, "server.yml"), cfg); err != nil {
			return err
		}
		if err := server.PublishSecurityPGPKey(context.Background(), cfg, db.SQL); err != nil {
			log.Printf("security.pgp_publish_pending reason=%q", err.Error())
		}
		fmt.Printf("Rotated security PGP key %s\n", fingerprint)
		return nil
	case "export":
		if len(args) < 2 {
			return fmt.Errorf("usage: anime --maintenance pgp export {public [path]|private <path>}")
		}
		switch args[1] {
		case "public":
			key, err := os.ReadFile(filepath.Join(securityDir, "pgp.pub.asc"))
			if err != nil {
				return err
			}
			if len(args) < 3 {
				_, err = os.Stdout.Write(key)
				return err
			}
			return os.WriteFile(args[2], key, 0644)
		case "private":
			if len(args) != 3 {
				return fmt.Errorf("usage: anime --maintenance pgp export private <path>")
			}
			if err := authorizeMaintenance(cfg); err != nil {
				return err
			}
			var previousExport string
			err := db.SQL.QueryRow(`SELECT exported_at FROM security_pgp_exports WHERE id = 1`).Scan(&previousExport)
			if err == nil {
				last, parseErr := time.Parse(time.RFC3339, previousExport)
				if parseErr == nil && time.Since(last) < time.Hour {
					return fmt.Errorf("private-key export is limited to once per hour")
				}
			} else if err != sql.ErrNoRows {
				return err
			}
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Reason for exporting the private key: ")
			reason, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(reason) == "" {
				return fmt.Errorf("an export reason is required")
			}
			fmt.Print("Type EXPORT PRIVATE SECURITY KEY to continue: ")
			confirmation, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(confirmation) != "EXPORT PRIVATE SECURITY KEY" {
				return fmt.Errorf("confirmation did not match")
			}
			if err := server.ExportSecurityPGPPrivateKey(cfg.ConfigDir, installationSecret, args[2]); err != nil {
				return err
			}
			exportedAt := time.Now().UTC().Format(time.RFC3339)
			_, err = db.SQL.Exec(`INSERT INTO security_pgp_exports (id, exported_at) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET exported_at=excluded.exported_at`, exportedAt)
			if err != nil {
				return err
			}
			operatorIP := "local"
			if remote := strings.Fields(os.Getenv("SSH_CLIENT")); len(remote) > 0 {
				operatorIP = remote[0]
			}
			_, _ = db.SQL.Exec(`INSERT INTO audit_log (timestamp, level, category, action, actor_ip, target_type, target_id, details, success) VALUES (?, 'warning', 'security', 'private_key_exported', ?, 'pgp_key', 'project', ?, 1)`, time.Now().UTC().Unix(), operatorIP, strings.TrimSpace(reason))
			log.Printf("security.private_key_exported operator_ip=%q path=%q reason=%q", operatorIP, args[2], strings.TrimSpace(reason))
			return nil
		default:
			return fmt.Errorf("unknown PGP export type %q", args[1])
		}
	case "delete":
		if err := authorizeMaintenance(cfg); err != nil {
			return err
		}
		fmt.Println("Warning: retained PGP-encrypted reports will no longer be decryptable.")
		fmt.Print("Type DELETE SECURITY PGP KEY to continue: ")
		confirmation, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil || strings.TrimSpace(confirmation) != "DELETE SECURITY PGP KEY" {
			return fmt.Errorf("confirmation did not match")
		}
		for _, filename := range []string{"pgp.pub.asc", "pgp.pub.asc.sig", "pgp.pub.old.asc", "pgp.priv.asc.enc", "pgp.priv.old.asc.enc"} {
			if err := os.Remove(filepath.Join(securityDir, filename)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		_, err = db.SQL.Exec(`UPDATE security_pgp_metadata SET revoked=1 WHERE id=1`)
		if err != nil {
			return err
		}
		cfg.WebSecurity.PublishPGPKey = false
		err = config.Save(filepath.Join(cfg.ConfigDir, "server.yml"), cfg)
		return err
	default:
		return fmt.Errorf("unknown PGP action %q; available: generate, rotate, publish, import, export, delete", args[0])
	}
}

func handleSecurityMaintenance(args []string, cfg *config.Config, dataDir string) error {
	if err := authorizeMaintenance(cfg); err != nil {
		return err
	}
	db, err := database.Open(cfg.Server.Database, filepath.Join(dataDir, "db"))
	if err != nil {
		return err
	}
	defer func() {
		if err := db.SQL.Close(); err != nil {
			log.Printf("database close failed: %v", err)
		}
	}()
	return server.MaintainSecurityReports(context.Background(), db, cfg, args, os.Stdout)
}

func authorizeMaintenance(cfg *config.Config) error {
	if paths.IsElevated() {
		return nil
	}
	fmt.Print("Operator token: ")
	provided, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("read operator token: %w", err)
	}
	providedHash := sha256.Sum256(provided)
	configuredHash := sha256.Sum256([]byte(cfg.Server.Token))
	valid := subtle.ConstantTimeCompare(providedHash[:], configuredHash[:])
	valid &= subtle.ConstantTimeEq(int32(len(provided)), int32(len(cfg.Server.Token)))
	if cfg.Server.Token == "" || valid != 1 {
		return fmt.Errorf("invalid credentials")
	}
	return nil
}

func setApplicationMode(value, configPath string) {
	selectedMode, err := mode.ParseAppMode(value)
	if err != nil {
		log.Fatalf("Invalid application mode: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if !paths.IsElevated() {
		fmt.Print("Operator token: ")
		provided, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			log.Fatalf("Failed to read operator token: %v", err)
		}
		providedHash := sha256.Sum256(provided)
		configuredHash := sha256.Sum256([]byte(cfg.Server.Token))
		valid := subtle.ConstantTimeCompare(providedHash[:], configuredHash[:])
		valid &= subtle.ConstantTimeEq(int32(len(provided)), int32(len(cfg.Server.Token)))
		if cfg.Server.Token == "" || valid != 1 {
			log.Fatal("Invalid credentials")
		}
	}
	cfg.Server.Mode = string(selectedMode)
	if err := config.Save(configPath, cfg); err != nil {
		log.Fatalf("Failed to save application mode: %v", err)
	}
	fmt.Printf("Application mode set to: %s\n", selectedMode)
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
		if err := os.Remove("/etc/systemd/system/anime.service"); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to remove service file: %v", err)
		}
		runCommand("systemctl", "daemon-reload")
	case "darwin":
		runCommand("launchctl", "unload", "/Library/LaunchDaemons/us.apimgr.anime.plist")
		if err := os.Remove("/Library/LaunchDaemons/us.apimgr.anime.plist"); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to remove service plist: %v", err)
		}
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

func maintenanceBackupTest(configDir, dataDir, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	privatePath := filepath.Join(configDir, "security", "pgp.priv.asc.enc")
	publicPath := filepath.Join(configDir, "security", "pgp.pub.asc")
	if _, err := os.Stat(privatePath); os.IsNotExist(err) {
		if _, publicErr := os.Stat(publicPath); os.IsNotExist(publicErr) {
			fmt.Println("Backup test: no PGP keypair is configured")
			return nil
		}
	}
	db, err := database.OpenReadOnly(cfg.Server.Database, filepath.Join(dataDir, "db"))
	if err != nil {
		return err
	}
	defer func() {
		if err := db.SQL.Close(); err != nil {
			log.Printf("database close failed: %v", err)
		}
	}()
	installationSecret, err := database.AppSecret(context.Background(), db.SQL, "installation_secret")
	if err != nil {
		return err
	}
	private, err := server.DecryptSecurityPGPPrivateKey(privatePath, installationSecret)
	if err != nil {
		return fmt.Errorf("encrypted private key cannot be decrypted: %w", err)
	}
	defer func() {
		for index := range private {
			private[index] = 0
		}
	}()
	privateFingerprint, _, err := server.InspectSecurityPGPPrivateKey(private)
	if err != nil {
		return fmt.Errorf("decrypted private key is invalid: %w", err)
	}
	public, err := os.ReadFile(publicPath)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	publicFingerprint, err := server.InspectSecurityPGPPublicKey(public)
	if err != nil {
		return fmt.Errorf("public key is invalid: %w", err)
	}
	if privateFingerprint != publicFingerprint {
		return fmt.Errorf("public and private key fingerprints do not match")
	}
	var recordedFingerprint string
	if err := db.SQL.QueryRow(`SELECT fingerprint FROM security_pgp_metadata WHERE id = 1`).Scan(&recordedFingerprint); err != nil {
		return fmt.Errorf("read PGP key metadata: %w", err)
	}
	if recordedFingerprint != privateFingerprint {
		return fmt.Errorf("database and keypair fingerprints do not match")
	}
	fmt.Printf("Backup test passed: encrypted PGP private key decrypts and matches fingerprint %s\n", privateFingerprint)
	return nil
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
	if cfg, err := config.Load(filepath.Join(configDir, "server.yml")); err == nil {
		if keyExists, _ := os.Stat(filepath.Join(configDir, "security", "pgp.pub.asc")); keyExists != nil {
			if db, openErr := database.Open(cfg.Server.Database, filepath.Join(dataDir, "db")); openErr == nil {
				if publishErr := server.PublishSecurityPGPKey(context.Background(), cfg, db.SQL); publishErr != nil {
					log.Printf("security.pgp_restore_publish_pending reason=%q", publishErr.Error())
				}
				_ = db.SQL.Close()
			} else {
				log.Printf("security.pgp_restore_publish_pending reason=database_unavailable")
			}
		}
	}

	fmt.Println("Restore completed successfully")
}

// setApplicationMode sets the application mode
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
func handleUpdateCommand(cmd string, cfg *config.Config, configPath string) error {
	args := flag.Args()

	switch cmd {
	case "check":
		release, err := checkForUpdate(cfg)
		if err != nil {
			return err
		}
		if release == nil {
			fmt.Printf("%s is up to date on the %s channel.\n", Version, cfg.Server.Update.Branch)
			return nil
		}
		fmt.Printf("Update available: %s (current: %s)\n", release.TagName, Version)
		return nil
	case "yes":
		release, err := checkForUpdate(cfg)
		if err != nil {
			return err
		}
		if release == nil {
			fmt.Printf("%s is already up to date.\n", Version)
			return nil
		}
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate current executable: %w", err)
		}
		client := updater.NewClient()
		if err := client.Install(context.Background(), release, executable); err != nil {
			return err
		}
		fmt.Printf("Installed update %s. Restarting.\n", release.TagName)
		if updater.IsServiceProcess() {
			return service.Restart()
		}
		return updater.RestartSelf(restartArguments(os.Args[1:]))
	case "branch":
		if len(args) == 0 {
			fmt.Printf("Current update branch: %s\n", cfg.Server.Update.Branch)
			fmt.Println("Usage: anime --update branch {stable|beta|devel}")
			return nil
		}
		branch, err := updater.NormalizeBranch(args[0])
		if err != nil {
			return err
		}
		cfg.Server.Update.Branch = branch
		if err := config.Save(configPath, cfg); err != nil {
			return fmt.Errorf("save update channel: %w", err)
		}
		fmt.Printf("Update branch set to: %s\n", branch)
		return nil
	default:
		return fmt.Errorf("unknown update command %q; expected check, yes, or branch", cmd)
	}
}

func checkForUpdate(cfg *config.Config) (*updater.Release, error) {
	buildEpoch, err := strconv.ParseInt(BuildEpoch, 10, 64)
	if err != nil {
		buildEpoch = 0
	}
	client := updaterClientFactory()
	return client.Check(context.Background(), Version, cfg.Server.Update.Branch, buildEpoch)
}

func normalizeUpdateArgs(args []string) []string {
	result := make([]string, 0, len(args))
	for index, arg := range args {
		if arg == "--update" && (index+1 == len(args) || strings.HasPrefix(args[index+1], "-")) {
			result = append(result, "--update=yes")
		} else {
			result = append(result, arg)
		}
	}
	return result
}

func restartArguments(args []string) []string {
	result := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--update" {
			if index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
				index++
			}
			continue
		}
		if strings.HasPrefix(arg, "--update=") || arg == "--maintenance=update" {
			continue
		}
		if arg == "--maintenance" {
			if index+1 < len(args) && args[index+1] == "update" {
				index++
				continue
			}
		}
		result = append(result, arg)
	}
	return result
}

func runCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Command failed: %s %v: %v", name, args, err)
	}
}

func buildDate() string {
	seconds, err := strconv.ParseInt(BuildEpoch, 10, 64)
	if err != nil || seconds <= 0 {
		return "N/A"
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func printVersion() {
	name := filepath.Base(os.Args[0])
	fmt.Printf("%s %s\nBuilt: %s\nGo: %s\nOS/Arch: %s/%s\n", name, Version, buildDate(), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func printShellCommand(command string, args []string) {
	shell := ""
	if len(args) > 0 {
		shell = args[0]
	} else {
		shell = filepath.Base(os.Getenv("SHELL"))
	}
	if shell == "" {
		shell = "bash"
	}
	name := filepath.Base(os.Args[0])
	if command == "help" {
		fmt.Printf("Shell integration for %s: --shell completions [SHELL] or --shell init [SHELL]\n", name)
		return
	}
	if command == "init" {
		switch shell {
		case "bash", "zsh":
			fmt.Printf("source <(%s --shell completions %s)\n", name, shell)
		case "fish":
			fmt.Printf("%s --shell completions fish | source\n", name)
		default:
			fmt.Fprintf(os.Stderr, "unsupported shell %q\n", shell)
			os.Exit(2)
		}
		return
	}
	if command != "completions" {
		shell = command
	}
	switch shell {
	case "bash":
		fmt.Printf("complete -W '--help --version --port --address --config --data --log --mode --service --maintenance --update' %s\n", name)
	case "zsh":
		fmt.Printf("#compdef %s\n_arguments '*:command:(--help --version --port --address --config --data --log --mode --service --maintenance --update)'\n", name)
	case "fish":
		fmt.Printf("complete -c %s -f -a '--help --version --port --address --config --data --log --mode --service --maintenance --update'\n", name)
	default:
		fmt.Fprintf(os.Stderr, "unsupported shell %q\n", shell)
		os.Exit(2)
	}
}
