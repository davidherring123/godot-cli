// Package scene inspects and manages scenes in a running Godot editor.
package scene

import "github.com/davidherring123/godot-mcp/internal/port/invocation"

const CurrentName = "scene.current"

func Current(projectRoot string, params CurrentParams) (CurrentResult, error) {
	return invocation.Call[CurrentParams, CurrentResult](projectRoot, CurrentName, params)
}
