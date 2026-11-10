//go:build windows

package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func replaceBinary(currentPath, verifiedPath string) error {
	info, err := os.Stat(currentPath)
	if err != nil {
		return fmt.Errorf("inspect current executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("current executable is not a regular file")
	}
	oldPath := currentPath + ".old"
	if _, err := os.Lstat(oldPath); err == nil {
		return fmt.Errorf("previous update backup already exists at %s", oldPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect previous update backup: %w", err)
	}
	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("rename current executable: %w", err)
	}
	if err := os.Rename(verifiedPath, currentPath); err != nil {
		if rollbackErr := os.Rename(oldPath, currentPath); rollbackErr != nil {
			return fmt.Errorf("install update failed: %w; restore current executable failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("install update (check write permission for %s): %w", currentPath, err)
	}
	oldPath16, err := windows.UTF16PtrFromString(filepath.Clean(oldPath))
	if err != nil {
		return fmt.Errorf("prepare previous executable cleanup: %w", err)
	}
	if err := windows.MoveFileEx(oldPath16, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT); err != nil {
		return fmt.Errorf("schedule previous executable cleanup: %w", err)
	}
	return nil
}
