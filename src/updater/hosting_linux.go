//go:build linux

package updater

import "os"

func IsServiceProcess() bool {
	return os.Getenv("INVOCATION_ID") != "" || os.Getenv("NOTIFY_SOCKET") != "" || os.Getenv("SUPERVISED") == "1"
}
