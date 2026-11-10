package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	OrgName      = "apimgr"
	InternalName = "anime"
)

type Directories struct {
	Config string
	Data   string
	Cache  string
	Logs   string
	Backup string
	PID    string
	DB     string
	System bool
}

// ResolveDefaultDirectories returns the platform paths captured for this process.
func ResolveDefaultDirectories(projectName string) Directories {
	container := IsRunningInContainer()
	system := isElevated()
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	directories := resolveDirectories(runtime.GOOS, system, container, home, os.Getenv("ProgramData"), os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), projectName)
	if directories.System && !container && !isDirectoryWritable(directories.Backup) {
		directories.Backup = filepath.Join(directories.Data, "backup")
	}
	return directories
}

func resolveDirectories(goos string, system, container bool, home, programData, appData, localAppData, projectName string) Directories {
	if container {
		return Directories{
			Config: "/config/" + projectName,
			Data:   "/data/" + projectName,
			Cache:  filepath.Join("/data", projectName, "cache"),
			Logs:   filepath.Join("/data/log", projectName),
			Backup: filepath.Join("/data/backups", projectName),
			DB:     "/data/db/sqlite",
			System: system,
		}
	}

	if goos == "windows" {
		if programData == "" {
			programData = `C:\ProgramData`
		}
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if system {
			root := filepath.Join(programData, OrgName, projectName)
			return Directories{
				Config: root,
				Data:   filepath.Join(root, "data"),
				Cache:  filepath.Join(root, "cache"),
				Logs:   filepath.Join(root, "logs"),
				Backup: filepath.Join(programData, "Backups", OrgName, projectName),
				DB:     filepath.Join(root, "db"),
				System: true,
			}
		}
		root := filepath.Join(localAppData, OrgName, projectName)
		return Directories{
			Config: filepath.Join(appData, OrgName, projectName),
			Data:   root,
			Cache:  filepath.Join(root, "cache"),
			Logs:   filepath.Join(root, "logs"),
			Backup: filepath.Join(localAppData, "Backups", OrgName, projectName),
			DB:     filepath.Join(root, "db"),
		}
	}

	if goos == "darwin" {
		if system {
			root := filepath.Join("/Library/Application Support", OrgName, projectName)
			return Directories{
				Config: root,
				Data:   filepath.Join(root, "data"),
				Cache:  filepath.Join("/Library/Caches", OrgName, projectName),
				Logs:   filepath.Join("/Library/Logs", OrgName, projectName),
				Backup: filepath.Join("/Library/Backups", OrgName, projectName),
				PID:    filepath.Join("/var/run", OrgName, projectName+".pid"),
				DB:     filepath.Join(root, "db"),
				System: true,
			}
		}
		root := filepath.Join(home, "Library", "Application Support", OrgName, projectName)
		return Directories{
			Config: root,
			Data:   root,
			Cache:  filepath.Join(home, "Library", "Caches", OrgName, projectName),
			Logs:   filepath.Join(home, "Library", "Logs", OrgName, projectName),
			Backup: filepath.Join(home, "Library", "Backups", OrgName, projectName),
			PID:    filepath.Join(root, projectName+".pid"),
			DB:     filepath.Join(root, "db"),
		}
	}

	if system {
		configRoot := filepath.Join("/etc", OrgName, projectName)
		dataBase := "/var/lib"
		backupBase := "/mnt/Backups"
		if goos == "freebsd" || goos == "openbsd" || goos == "netbsd" {
			configRoot = filepath.Join("/usr/local/etc", OrgName, projectName)
			dataBase = "/var/db"
			backupBase = "/var/backups"
		}
		dataRoot := filepath.Join(dataBase, OrgName, projectName)
		return Directories{
			Config: configRoot,
			Data:   dataRoot,
			Cache:  filepath.Join("/var/cache", OrgName, projectName),
			Logs:   filepath.Join("/var/log", OrgName, projectName),
			Backup: filepath.Join(backupBase, OrgName, projectName),
			PID:    filepath.Join("/var/run", OrgName, projectName+".pid"),
			DB:     filepath.Join(dataRoot, "db"),
			System: true,
		}
	}

	if home == "" {
		home = "."
	}
	configRoot := os.Getenv("XDG_CONFIG_HOME")
	if configRoot == "" {
		configRoot = filepath.Join(home, ".config")
	}
	dataRoot := os.Getenv("XDG_DATA_HOME")
	if dataRoot == "" {
		dataRoot = filepath.Join(home, ".local", "share")
	}
	cacheRoot := os.Getenv("XDG_CACHE_HOME")
	if cacheRoot == "" {
		cacheRoot = filepath.Join(home, ".cache")
	}
	dataDir := filepath.Join(dataRoot, OrgName, projectName)
	return Directories{
		Config: filepath.Join(configRoot, OrgName, projectName),
		Data:   dataDir,
		Cache:  filepath.Join(cacheRoot, OrgName, projectName),
		Logs:   filepath.Join(home, ".local", "log", OrgName, projectName),
		Backup: filepath.Join(dataRoot, "Backups", OrgName, projectName),
		PID:    filepath.Join(dataDir, projectName+".pid"),
		DB:     filepath.Join(dataDir, "db"),
	}
}

// GetDefaultDirs returns the legacy config, data, and log path tuple.
func GetDefaultDirs(projectName string) (configDir, dataDir, logsDir string) {
	dirs := ResolveDefaultDirectories(projectName)
	return dirs.Config, dirs.Data, dirs.Logs
}

// EnsureDir creates a directory using system or user-only permissions and verifies write access.
func EnsureDir(path string, system bool) error {
	permission := os.FileMode(0700)
	if system {
		permission = 0755
	}
	if err := os.MkdirAll(path, permission); err != nil {
		return fmt.Errorf("create directory %q: %w", path, err)
	}
	probe, err := os.CreateTemp(path, ".write-test-")
	if err != nil {
		return fmt.Errorf("directory %q is not writable: %w", path, err)
	}
	probeName := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probeName)
		return fmt.Errorf("close directory write check in %q: %w", path, err)
	}
	if err := os.Remove(probeName); err != nil {
		return fmt.Errorf("remove directory write check in %q: %w", path, err)
	}
	return nil
}

// EnsureRuntimeDirectories creates all runtime directories with mode-aware permissions.
func EnsureRuntimeDirectories(dirs Directories) error {
	for _, directory := range []string{dirs.Config, dirs.Data, dirs.Cache, dirs.Logs, dirs.Backup, dirs.DB} {
		if directory == "" {
			continue
		}
		if err := EnsureDir(directory, dirs.System); err != nil {
			return err
		}
	}
	if dirs.PID != "" {
		if err := EnsureDir(filepath.Dir(dirs.PID), dirs.System); err != nil {
			return err
		}
	}
	return nil
}

// EnsureDirs creates the legacy config, data, and log directories.
func EnsureDirs(configDir, dataDir, logsDir string) error {
	for _, directory := range []string{configDir, dataDir, logsDir} {
		if err := EnsureDir(directory, os.Geteuid() == 0); err != nil {
			return err
		}
	}
	return nil
}

// IsRunningInContainer checks common container markers and init process names.
func IsRunningInContainer() bool {
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv", "/dev/lxc"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	if os.Getenv("container") != "" || os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		process := strings.TrimSpace(string(data))
		switch process {
		case "tini", "dumb-init", "s6-svscan", "runsv", "runsvdir", "catatonit":
			return true
		}
	}
	return false
}

// GetBackupDir returns a mode-aware default backup location.
func GetBackupDir(projectName string) string {
	return ResolveDefaultDirectories(projectName).Backup
}

func isElevated() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return os.Geteuid() == 0
}

func IsElevated() bool {
	return isElevated()
}

func isDirectoryWritable(directory string) bool {
	testDirectory := directory
	if _, err := os.Stat(testDirectory); os.IsNotExist(err) {
		testDirectory = filepath.Dir(directory)
	}
	info, err := os.Stat(testDirectory)
	if err != nil || !info.IsDir() {
		return false
	}
	probe, err := os.CreateTemp(testDirectory, ".write-test-")
	if err != nil {
		return false
	}
	probePath := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return false
	}
	return os.Remove(probePath) == nil
}
