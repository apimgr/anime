//go:build windows

package updater

import "golang.org/x/sys/windows/svc"

func IsServiceProcess() bool {
	isService, err := svc.IsWindowsService()
	return err == nil && isService
}
