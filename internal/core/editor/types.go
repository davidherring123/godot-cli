// Package editor manages the Godot editor process for a project.
package editor

const (
	OpenName   = "editor.open"
	StatusName = "editor.status"
)

type OpenParams struct {
	TimeoutSeconds float64 `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum seconds to wait for the editor bridge to start; defaults to 30."`
}

type OpenResult struct {
	Opened         bool   `json:"opened" jsonschema:"description=Whether this call started the editor; false when an editor was already running."`
	AlreadyRunning bool   `json:"already_running" jsonschema:"description=Whether an editor was already running for the project."`
	PID            int    `json:"pid" jsonschema:"description=Godot editor process ID."`
	Port           int    `json:"port" jsonschema:"description=godot-cli bridge port."`
	Version        string `json:"version" jsonschema:"description=godot-cli addon version reported by the editor."`
}

type StatusParams struct{}

type StatusResult struct {
	Running bool   `json:"running" jsonschema:"description=Whether a Godot editor with the godot-cli addon is registered for the project."`
	PID     int    `json:"pid,omitempty" jsonschema:"description=Godot editor process ID."`
	Port    int    `json:"port,omitempty" jsonschema:"description=godot-cli bridge port."`
	Version string `json:"version,omitempty" jsonschema:"description=godot-cli addon version reported by the editor."`
}
