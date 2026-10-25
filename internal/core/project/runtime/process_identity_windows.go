//go:build windows

package runtime

import (
	"fmt"
	"syscall"
)

// processIdentity returns the process creation time, which is stable for the
// lifetime of a process and differs when the OS reuses a PID.
func processIdentity(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}

	handle, err := syscall.OpenProcess(
		syscall.PROCESS_QUERY_INFORMATION,
		false,
		uint32(pid),
	)
	if err != nil {
		return "", false
	}

	defer func() { _ = syscall.CloseHandle(handle) }()

	var created, exited, kernel, user syscall.Filetime

	if err := syscall.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return "", false
	}

	return fmt.Sprintf("windows:%08x%08x", created.HighDateTime, created.LowDateTime), true
}
