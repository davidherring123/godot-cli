package editor

import (
	"errors"

	"github.com/davidherring123/godot-mcp/internal/port/editorinstance"
)

// Status reports whether a Godot editor with the addon is registered for the
// project.
func Status(projectRoot string, _ StatusParams) (StatusResult, error) {
	instance, err := editorinstance.Find(projectRoot)
	if errors.Is(err, editorinstance.ErrNoEditor) {
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
