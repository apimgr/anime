package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetDefaultDirs(t *testing.T) {
	configDir, dataDir, logsDir := GetDefaultDirs("anime")

	if configDir == "" || dataDir == "" || logsDir == "" {
		t.Fatalf("expected non-empty dirs, got config=%q data=%q logs=%q", configDir, dataDir, logsDir)
	}

	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		if !filepath.IsAbs(configDir) {
			t.Errorf("expected absolute config dir, got %q", configDir)
		}
	}

	for _, dir := range []string{configDir, dataDir, logsDir} {
		if !strings.Contains(dir, "anime") {
			t.Errorf("expected dir %q to contain project %q", dir, "anime")
		}
		if !IsRunningInContainer() && !strings.Contains(dir, OrgName) {
			t.Errorf("expected native dir %q to contain org %q", dir, OrgName)
		}
	}
}

func TestEnsureDirAndEnsureDirs(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	logsDir := filepath.Join(base, "logs")

	if err := EnsureDir(configDir, false); err != nil {
		t.Fatalf("EnsureDir returned error: %v", err)
	}
	if info, err := os.Stat(configDir); err != nil || !info.IsDir() {
		t.Fatalf("expected directory to exist: %v", err)
	}

	if err := EnsureDirs(configDir, dataDir, logsDir); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}
	for _, dir := range []string{configDir, dataDir, logsDir} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("expected directory %q to exist: %v", dir, err)
		}
	}
}

func TestEnsureDirsFailsOnFirstError(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}
	// blocker is a file, so MkdirAll underneath it must fail.
	configDir := filepath.Join(blocker, "config")
	dataDir := filepath.Join(base, "data")
	logsDir := filepath.Join(base, "logs")

	if err := EnsureDirs(configDir, dataDir, logsDir); err == nil {
		t.Fatal("expected error when configDir cannot be created")
	}
}

func TestEnsureDirsFailsOnDataDirError(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(blocker, "data")
	logsDir := filepath.Join(base, "logs")

	if err := EnsureDirs(configDir, dataDir, logsDir); err == nil {
		t.Fatal("expected error when dataDir cannot be created")
	}
}

func TestEnsureDirsFailsOnLogsDirError(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	logsDir := filepath.Join(blocker, "logs")

	if err := EnsureDirs(configDir, dataDir, logsDir); err == nil {
		t.Fatal("expected error when logsDir cannot be created")
	}
}

func TestIsRunningInContainer(t *testing.T) {
	got := IsRunningInContainer()

	want := false
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv", "/dev/lxc"} {
		if _, err := os.Stat(marker); err == nil {
			want = true
		}
	}
	if os.Getenv("container") != "" || os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		want = true
	}
	if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		switch strings.TrimSpace(string(data)) {
		case "tini", "dumb-init", "s6-svscan", "runsv", "runsvdir", "catatonit":
			want = true
		}
	}
	if got != want {
		t.Errorf("IsRunningInContainer() = %v, want %v", got, want)
	}
}

func TestGetBackupDir(t *testing.T) {
	got := GetBackupDir("anime")
	want := ResolveDefaultDirectories("anime").Backup
	if got != want {
		t.Errorf("GetBackupDir() = %q, want %q", got, want)
	}
}

func TestResolveDefaultDirectoriesByPlatform(t *testing.T) {
	linux := resolveDirectories("linux", true, false, "/home/test", "", "", "", "anime")
	if linux.Config != "/etc/apimgr/anime" || linux.Data != "/var/lib/apimgr/anime" || linux.Cache != "/var/cache/apimgr/anime" || linux.Logs != "/var/log/apimgr/anime" {
		t.Fatalf("unexpected Linux system directories: %#v", linux)
	}
	bsd := resolveDirectories("freebsd", true, false, "/home/test", "", "", "", "anime")
	if bsd.Config != "/usr/local/etc/apimgr/anime" || bsd.Data != "/var/db/apimgr/anime" {
		t.Fatalf("unexpected BSD system directories: %#v", bsd)
	}
	macos := resolveDirectories("darwin", false, false, "/Users/test", "", "", "", "anime")
	if macos.Config != "/Users/test/Library/Application Support/apimgr/anime" || macos.Logs != "/Users/test/Library/Logs/apimgr/anime" {
		t.Fatalf("unexpected macOS user directories: %#v", macos)
	}
	container := resolveDirectories("linux", true, true, "/root", "", "", "", "anime")
	if container.Config != "/config/anime" || container.DB != "/data/db/sqlite" || container.Logs != "/data/log/anime" {
		t.Fatalf("unexpected container directories: %#v", container)
	}
}
