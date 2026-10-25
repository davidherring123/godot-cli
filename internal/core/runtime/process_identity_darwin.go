//go:build darwin

package runtime

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// processIdentity returns the process start time, which is stable for the
// lifetime of a process and differs when the OS reuses a PID.
func processIdentity(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}

	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || info == nil {
		return "", false
	}

	return fmt.Sprintf(
		"darwin:%d.%d",
		info.Proc.P_starttime.Sec,
		info.Proc.P_starttime.Usec,
	), true
}
