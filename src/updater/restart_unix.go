//go:build !windows

package updater

import (
	"os"
	"path/filepath"
	"syscall"
)

func RestartSelf(args []string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	return syscall.Exec(executable, append([]string{executable}, args...), os.Environ())
}
