// Package godot locates the Godot executable used to run and open projects.
package godot

import (
	"fmt"
	"os"
	"os/exec"

	coregodot "github.com/davidherring123/godot-cli/internal/port/godot"
)

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

	return "", fmt.Errorf("%w; set GODOT_BIN to its path", coregodot.ErrNotFound)
}
