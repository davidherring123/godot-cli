//go:build windows

package bridge

import (
	"errors"
	"syscall"
)

const stillActive = 259

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, err := syscall.OpenProcess(
		syscall.PROCESS_QUERY_INFORMATION,
		false,
		uint32(pid),
	)
	if err != nil {
		// Access denied still means the process exists.
		return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
	}

	defer func() { _ = syscall.CloseHandle(handle) }()

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		// Do not treat an inconclusive probe as a dead editor.
		return true
	}

	return exitCode == stillActive
}
