//go:build !linux && !darwin && !windows

package runtime

// processIdentity is unavailable on this platform, so sessions fall back to
// PID liveness. A recorded identity from another platform never matches.
func processIdentity(_ int) (string, bool) {
	return "", false
}
