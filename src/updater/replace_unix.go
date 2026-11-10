//go:build !windows

package updater

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func replaceBinary(currentPath, verifiedPath string) error {
	info, err := os.Stat(currentPath)
	if err != nil {
		return fmt.Errorf("inspect current executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("current executable is not a regular file")
	}
	staged, err := os.CreateTemp(filepath.Dir(currentPath), "."+filepath.Base(currentPath)+".update-*")
	if err != nil {
		return fmt.Errorf("stage update beside executable: %w", err)
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)
	source, err := os.Open(verifiedPath)
	if err != nil {
		_ = staged.Close()
		return fmt.Errorf("open verified update: %w", err)
	}
	_, copyErr := io.Copy(staged, source)
	closeSourceErr := source.Close()
	if copyErr != nil {
		_ = staged.Close()
		return fmt.Errorf("stage verified update: %w", copyErr)
	}
	if closeSourceErr != nil {
		_ = staged.Close()
		return fmt.Errorf("close verified update: %w", closeSourceErr)
	}
	if err := staged.Chmod(info.Mode().Perm()); err != nil {
		_ = staged.Close()
		return fmt.Errorf("set update permissions: %w", err)
	}
	if err := staged.Sync(); err != nil {
		_ = staged.Close()
		return fmt.Errorf("sync staged update: %w", err)
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("close staged update: %w", err)
	}
	if err := os.Rename(stagedPath, currentPath); err != nil {
		return fmt.Errorf("replace executable (check write permission for %s): %w", currentPath, err)
	}
	return nil
}
