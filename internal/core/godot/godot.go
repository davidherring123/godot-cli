// Package godot locates the Godot executable used to run and open projects.
package godot

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ErrNotFound reports that no Godot executable could be located.
var ErrNotFound = errors.New("godot executable not found")

// FindExecutable locates Godot using GODOT_BIN, then the godot and godot4
// executables on PATH.
func FindExecutable() (string, error) {
	candidates := []string{
		os.Getenv("GODOT_BIN"),
		"godot",
		"godot4",
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("%w; set GODOT_BIN to its path", ErrNotFound)
}
