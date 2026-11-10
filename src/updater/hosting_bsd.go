//go:build freebsd || openbsd || netbsd

package updater

import "os"

func IsServiceProcess() bool { return os.Getenv("RC_PID") != "" || os.Getenv("daemon_pidfile") != "" }
