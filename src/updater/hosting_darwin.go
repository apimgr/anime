//go:build darwin

package updater

import "os"

func IsServiceProcess() bool { return os.Getenv("LAUNCH_JOBKEY_LABEL") != "" }
