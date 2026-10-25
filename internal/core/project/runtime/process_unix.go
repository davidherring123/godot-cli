//go:build unix

package runtime

import (
	"errors"
	"syscall"
)

// processAlive reports whether a process exists without signaling it in a way
// that changes its state. Permission failures mean the process exists.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)

	return !errors.Is(err, syscall.ESRCH)
}
