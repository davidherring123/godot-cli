// Package editor reports the registered Godot editor for a project.
package editor

const StatusName = "editor.status"

type StatusParams struct{}

type StatusResult struct {
	Running bool   `json:"running" jsonschema:"description=Whether a Godot editor with the godot-mcp addon is registered for the project."`
	PID     int    `json:"pid,omitempty" jsonschema:"description=Godot editor process ID."`
	Port    int    `json:"port,omitempty" jsonschema:"description=godot-mcp bridge port."`
	Version string `json:"version,omitempty" jsonschema:"description=godot-mcp addon version reported by the editor."`
}
