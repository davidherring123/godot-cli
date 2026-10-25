//go:build linux

package runtime

import (
	"fmt"
	"os"
	"strings"
)

// processIdentity returns the process start time, which is stable for the
// lifetime of a process and differs when the OS reuses a PID.
func processIdentity(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", false
	}

	// The command name is parenthesized and may contain spaces or closing
	// parentheses, so split after the final ')'.
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 || end+2 >= len(data) {
		return "", false
	}

	fields := strings.Fields(string(data[end+2:]))

	// starttime is field 22 of /proc/<pid>/stat; fields here start at field 3.
	const startTimeIndex = 19

	if len(fields) <= startTimeIndex {
		return "", false
	}

	return "linux:" + fields[startTimeIndex], true
}
