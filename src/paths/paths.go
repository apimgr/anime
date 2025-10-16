package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetConfigDir returns the OS-specific configuration directory
func GetConfigDir() string {
	if configDir := os.Getenv("CONFIG_DIR"); configDir != "" {
		return configDir
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "anime")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "anime")
	default: // linux, bsd, etc.
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, "anime")
		}
		return filepath.Join(os.Getenv("HOME"), ".config", "anime")
	}
}

// GetDataDir returns the OS-specific data directory
func GetDataDir() string {
	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		return dataDir
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "anime")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "anime")
	default: // linux, bsd, etc.
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			return filepath.Join(xdgData, "anime")
		}
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "anime")
	}
}

// GetLogsDir returns the OS-specific logs directory
func GetLogsDir() string {
	if logsDir := os.Getenv("LOGS_DIR"); logsDir != "" {
		return logsDir
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "anime", "logs")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Logs", "anime")
	default: // linux, bsd, etc.
		if xdgState := os.Getenv("XDG_STATE_HOME"); xdgState != "" {
			return filepath.Join(xdgState, "anime")
		}
		return filepath.Join(os.Getenv("HOME"), ".local", "state", "anime")
	}
}

// EnsureDirectories creates all required directories if they don't exist
func EnsureDirectories() error {
	dirs := []string{
		GetConfigDir(),
		GetDataDir(),
		GetLogsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
