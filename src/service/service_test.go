package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectServiceManager(t *testing.T) {
	got := DetectServiceManager()

	switch runtime.GOOS {
	case "darwin":
		if got != ServiceLaunchd {
			t.Errorf("expected ServiceLaunchd on darwin, got %v", got)
		}
	case "windows":
		if got != ServiceWindows {
			t.Errorf("expected ServiceWindows on windows, got %v", got)
		}
	case "freebsd", "openbsd", "netbsd":
		if got != ServiceBSDRC {
			t.Errorf("expected ServiceBSDRC on %s, got %v", runtime.GOOS, got)
		}
	case "linux":
		if got != ServiceSystemd && got != ServiceRunit && got != ServiceUnknown {
			t.Errorf("unexpected service manager on linux: %v", got)
		}
	default:
		if got != ServiceUnknown {
			t.Errorf("expected ServiceUnknown on %s, got %v", runtime.GOOS, got)
		}
	}
}

func TestGetBinaryPath(t *testing.T) {
	got := GetBinaryPath()
	if got == "" {
		t.Fatal("expected non-empty binary path")
	}

	if runtime.GOOS == "windows" {
		if !strings.Contains(got, "anime.exe") {
			t.Errorf("expected windows path to contain anime.exe, got %q", got)
		}
	} else {
		if got != "/usr/local/bin/anime" {
			t.Errorf("expected /usr/local/bin/anime, got %q", got)
		}
	}
}

func TestCopyBinary(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-binary")
	if err := os.WriteFile(src, []byte("binary-content"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	dst := filepath.Join(dir, "nested", "dst-binary")

	if err := copyBinary(src, dst); err != nil {
		t.Fatalf("copyBinary returned error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read copied binary: %v", err)
	}
	if string(data) != "binary-content" {
		t.Errorf("expected copied content to match, got %q", string(data))
	}
}

func TestCopyBinaryMissingSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "missing-src")
	dst := filepath.Join(dir, "dst-binary")

	if err := copyBinary(src, dst); err == nil {
		t.Fatal("expected error when source file does not exist")
	}
}

func TestCopyBinaryDestDirBlocked(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-binary")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}
	dst := filepath.Join(blocker, "sub", "dst-binary")

	if err := copyBinary(src, dst); err == nil {
		t.Fatal("expected error when destination directory cannot be created")
	}
}

// TestStartStopRestartReloadUnsupportedManager only asserts the error branch:
// it is safe to run only because this container has no systemd/runit/launchd
// present, so DetectServiceManager() returns ServiceUnknown and none of
// Start/Stop/Restart/Reload issue any real exec.Command call.
func TestStartStopRestartReloadUnsupportedManager(t *testing.T) {
	if DetectServiceManager() != ServiceUnknown {
		t.Skip("service manager detected on this host; skipping to avoid invoking real service commands")
	}

	if err := Start(); err == nil {
		t.Error("expected error for unsupported service manager")
	}
	if err := Stop(); err == nil {
		t.Error("expected error for unsupported service manager")
	}
	if err := Restart(); err == nil {
		t.Error("expected error for unsupported service manager")
	}
	if err := Reload(); err == nil {
		t.Error("expected error for unsupported service manager")
	}
}

func TestInstallUninstallUnsupportedManager(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" &&
		runtime.GOOS != "freebsd" && runtime.GOOS != "openbsd" && runtime.GOOS != "netbsd" {
		if err := Install(); err == nil {
			t.Error("expected error for unsupported OS")
		}
		if err := Uninstall(); err == nil {
			t.Error("expected error for unsupported OS")
		}
	}
}
