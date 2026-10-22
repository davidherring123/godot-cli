package editor

import (
	"errors"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
)

// Status reports whether a Godot editor with the addon is registered for the
// project.
func Status(projectRoot string, _ StatusParams) (StatusResult, error) {
	instance, err := bridge.FindInstance(projectRoot)
	if errors.Is(err, bridge.ErrNoEditor) {
		return StatusResult{}, nil
	}

	if err != nil {
		return StatusResult{}, err
	}

	return StatusResult{
		Running: true,
		PID:     instance.PID,
		Port:    instance.Port,
		Version: instance.Version,
	}, nil
}
