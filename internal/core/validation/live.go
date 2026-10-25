package validate

import "github.com/davidherring123/godot-cli/internal/port/invocation"

func Live(projectRoot string, params LiveParams) (LiveResult, error) {
	return invocation.Call[LiveParams, LiveResult](projectRoot, LiveName, params)
}
