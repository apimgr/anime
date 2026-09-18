package main

import (
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/apimgr/anime/src/config"
)

// runExitingHelper re-invokes this test binary as a subprocess with the given
// helper env var set, so os.Exit/log.Fatalf branches can be exercised safely
// without terminating the actual test process.
func runExitingHelper(t *testing.T, helperEnv, testName string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected subprocess to exit with a non-zero status")
	} else if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("expected an *exec.ExitError, got %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = original

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	return string(out)
}

func TestPrintHelp(t *testing.T) {
	output := captureStdout(t, printHelp)
	if !strings.Contains(output, "Anime Quotes API Server") {
		t.Errorf("expected help banner in output, got %q", output)
	}
	if !strings.Contains(output, "--service start") {
		t.Errorf("expected service usage in output, got %q", output)
	}
}

func TestCheckHealthSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port
	if err := checkHealth(strconv.Itoa(port)); err != nil {
		t.Fatalf("expected healthy response, got error: %v", err)
	}
}

func TestCheckHealthNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port
	if err := checkHealth(strconv.Itoa(port)); err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestCheckHealthConnectionRefused(t *testing.T) {
	if err := checkHealth("1"); err == nil {
		t.Fatal("expected error for unreachable port")
	}
}

func TestSetApplicationMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")

	output := captureStdout(t, func() {
		setApplicationMode("development", configPath)
	})
	if !strings.Contains(output, "Application mode set to: development") {
		t.Errorf("expected mode confirmation, got %q", output)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if cfg.Server.Mode != "development" {
		t.Errorf("expected saved mode development, got %q", cfg.Server.Mode)
	}
}

func TestShowCurrentMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")

	output := captureStdout(t, func() {
		showCurrentMode(configPath)
	})
	if !strings.Contains(output, "Current mode: production") {
		t.Errorf("expected default mode production, got %q", output)
	}
}

func TestRunInitialSetup(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")

	output := captureStdout(t, func() {
		runInitialSetup(configPath)
	})
	if !strings.Contains(output, "Setup complete") {
		t.Errorf("expected setup completion message, got %q", output)
	}
	if !strings.Contains(output, configPath) {
		t.Errorf("expected config path in output, got %q", output)
	}
}

func TestHandleUpdateCommandCheck(t *testing.T) {
	cfg := config.DefaultConfig()
	output := captureStdout(t, func() {
		handleUpdateCommand("check", cfg)
	})
	if !strings.Contains(output, "Checking for updates") {
		t.Errorf("expected update check output, got %q", output)
	}
}

func TestHandleUpdateCommandYes(t *testing.T) {
	cfg := config.DefaultConfig()
	output := captureStdout(t, func() {
		handleUpdateCommand("yes", cfg)
	})
	if !strings.Contains(output, "Installing updates") {
		t.Errorf("expected install updates output, got %q", output)
	}
}

func TestHandleUpdateCommandBranchShowCurrent(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"anime"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.Parse()

	cfg := config.DefaultConfig()
	output := captureStdout(t, func() {
		handleUpdateCommand("branch", cfg)
	})
	if !strings.Contains(output, "Current update branch") {
		t.Errorf("expected current branch output, got %q", output)
	}
}

func TestHandleServiceCommandHelp(t *testing.T) {
	output := captureStdout(t, func() {
		handleServiceCommand("--help", "")
	})
	if !strings.Contains(output, "Service commands:") {
		t.Errorf("expected service commands help output, got %q", output)
	}
}

func TestHandleMaintenanceCommandMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")
	output := captureStdout(t, func() {
		handleMaintenanceCommand("mode", "", "", "", configPath)
	})
	if !strings.Contains(output, "Current mode: production") {
		t.Errorf("expected current mode output, got %q", output)
	}
}

func TestHandleMaintenanceCommandSetup(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")
	output := captureStdout(t, func() {
		handleMaintenanceCommand("setup", "", "", "", configPath)
	})
	if !strings.Contains(output, "Setup complete") {
		t.Errorf("expected setup completion output, got %q", output)
	}
}

func TestHandleMaintenanceCommandUpdate(t *testing.T) {
	output := captureStdout(t, func() {
		handleMaintenanceCommand("update", "", "", "", "")
	})
	if !strings.Contains(output, "Checking for updates") {
		t.Errorf("expected update check output, got %q", output)
	}
}

func TestMaintenanceUpdate(t *testing.T) {
	output := captureStdout(t, maintenanceUpdate)
	if !strings.Contains(output, "Update feature not yet implemented") {
		t.Errorf("expected update-not-implemented output, got %q", output)
	}
}

func TestMaintenanceBackup(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("server: {}\n"), 0644); err != nil {
		t.Fatalf("failed to write sample config file: %v", err)
	}
	backupFile := filepath.Join(base, "backup.tar.gz")

	output := captureStdout(t, func() {
		maintenanceBackup(configDir, dataDir, backupFile)
	})
	if !strings.Contains(output, "Backup created successfully") {
		t.Errorf("expected backup success output, got %q", output)
	}
	if _, err := os.Stat(backupFile); err != nil {
		t.Errorf("expected backup file to exist: %v", err)
	}
}

func TestHandleMaintenanceCommandBackup(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	backupFile := filepath.Join(base, "explicit-backup.tar.gz")

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"anime", backupFile}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.Parse()

	output := captureStdout(t, func() {
		handleMaintenanceCommand("backup", configDir, dataDir, "", "")
	})
	if !strings.Contains(output, "Backup created successfully") {
		t.Errorf("expected backup success output, got %q", output)
	}
	if _, err := os.Stat(backupFile); err != nil {
		t.Errorf("expected explicit backup file to exist: %v", err)
	}
}

func TestSetApplicationModeInvalidModeHelper(t *testing.T) {
	if os.Getenv("ANIME_HELPER_INVALID_MODE") != "1" {
		t.Skip("only runs as a subprocess helper")
	}
	setApplicationMode("bogus", filepath.Join(t.TempDir(), "config.yml"))
}

func TestSetApplicationModeInvalidMode(t *testing.T) {
	runExitingHelper(t, "ANIME_HELPER_INVALID_MODE", "TestSetApplicationModeInvalidModeHelper")
}

func TestHandleUpdateCommandInvalidBranchHelper(t *testing.T) {
	if os.Getenv("ANIME_HELPER_INVALID_BRANCH") != "1" {
		t.Skip("only runs as a subprocess helper")
	}
	os.Args = []string{"anime", "nightly"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.Parse()
	handleUpdateCommand("branch", config.DefaultConfig())
}

func TestHandleUpdateCommandInvalidBranch(t *testing.T) {
	runExitingHelper(t, "ANIME_HELPER_INVALID_BRANCH", "TestHandleUpdateCommandInvalidBranchHelper")
}

func TestHandleUpdateCommandUnknownHelper(t *testing.T) {
	if os.Getenv("ANIME_HELPER_UNKNOWN_UPDATE") != "1" {
		t.Skip("only runs as a subprocess helper")
	}
	handleUpdateCommand("bogus", config.DefaultConfig())
}

func TestHandleUpdateCommandUnknown(t *testing.T) {
	runExitingHelper(t, "ANIME_HELPER_UNKNOWN_UPDATE", "TestHandleUpdateCommandUnknownHelper")
}

func TestHandleServiceCommandUnknownHelper(t *testing.T) {
	if os.Getenv("ANIME_HELPER_UNKNOWN_SERVICE") != "1" {
		t.Skip("only runs as a subprocess helper")
	}
	handleServiceCommand("bogus", "")
}

func TestHandleServiceCommandUnknown(t *testing.T) {
	runExitingHelper(t, "ANIME_HELPER_UNKNOWN_SERVICE", "TestHandleServiceCommandUnknownHelper")
}

func TestHandleMaintenanceCommandUnknownHelper(t *testing.T) {
	if os.Getenv("ANIME_HELPER_UNKNOWN_MAINTENANCE") != "1" {
		t.Skip("only runs as a subprocess helper")
	}
	handleMaintenanceCommand("bogus", "", "", "", "")
}

func TestHandleMaintenanceCommandUnknown(t *testing.T) {
	runExitingHelper(t, "ANIME_HELPER_UNKNOWN_MAINTENANCE", "TestHandleMaintenanceCommandUnknownHelper")
}

func TestHandleMaintenanceCommandRestoreNoArgsHelper(t *testing.T) {
	if os.Getenv("ANIME_HELPER_RESTORE_NO_ARGS") != "1" {
		t.Skip("only runs as a subprocess helper")
	}
	os.Args = []string{"anime"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.Parse()
	handleMaintenanceCommand("restore", "", "", "", "")
}

func TestHandleMaintenanceCommandRestoreNoArgs(t *testing.T) {
	runExitingHelper(t, "ANIME_HELPER_RESTORE_NO_ARGS", "TestHandleMaintenanceCommandRestoreNoArgsHelper")
}

func TestHandleUpdateCommandBranchSet(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"anime", "beta"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.Parse()

	cfg := config.DefaultConfig()
	output := captureStdout(t, func() {
		handleUpdateCommand("branch", cfg)
	})
	if !strings.Contains(output, "Update branch set to: beta") {
		t.Errorf("expected branch set confirmation, got %q", output)
	}
	if cfg.Server.UpdateBranch != "beta" {
		t.Errorf("expected cfg.Server.UpdateBranch to be beta, got %q", cfg.Server.UpdateBranch)
	}
}
