//go:build !unix && !windows

package bridge

// processAlive is conservative on platforms without a portable existence
// check: assume the registration is valid rather than launching a duplicate
// editor. Connect will still fail promptly if the bridge is not reachable.
func processAlive(pid int) bool {
	return pid > 0
}
