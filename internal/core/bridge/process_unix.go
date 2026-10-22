//go:build unix

package bridge

import (
	"errors"
	"syscall"
)

// processAlive reports whether a process exists without sending it a signal.
// Signal 0 only performs permission and existence checks. EPERM means the
// process exists but belongs to another user; any unexpected error is treated
// as alive so a temporary probe failure never hides a valid editor.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)

	return !errors.Is(err, syscall.ESRCH)
}
