package runtime

// sameProcess reports whether pid still refers to the process identified by
// identity. identity is recorded when a session starts, so a reused PID is not
// mistaken for the original runtime. Empty identity means the session predates
// identity tracking or the platform cannot provide one; liveness alone is then
// the best available check.
func sameProcess(pid int, identity string) bool {
	if !processAlive(pid) {
		return false
	}

	if identity == "" {
		return true
	}

	current, ok := processIdentity(pid)
	if !ok {
		// The process exists but its identity could not be verified. Treat it
		// as gone rather than risk signaling an unrelated reused PID.
		return false
	}

	return current == identity
}
