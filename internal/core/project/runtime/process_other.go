//go:build !unix && !windows

package runtime

// processAlive is conservative on platforms without a portable liveness
// check: assume the process exists so a stale registration is never used to
// start a second runtime.
func processAlive(pid int) bool {
	return pid > 0
}
