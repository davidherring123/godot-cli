//go:build !unix && !windows

package editor

import "os/exec"

// configureDetached is a best-effort fallback for platforms without a
// portable detached-process flag. The editor still works, but it shares the
// caller's process group instead of being reparented into its own session.
func configureDetached(_ *exec.Cmd) {}
