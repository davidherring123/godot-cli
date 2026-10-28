package settings

import (
	"github.com/davidherring123/godot-mcp/internal/port/invocation"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const UnsetName = "project.settings.unset"

func Unset(projectRoot string, params UnsetParams) (UnsetResult, error) {
	if params.Setting == "" {
		return UnsetResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A project setting name is required",
		}
	}

	return invocation.Call[UnsetParams, UnsetResult](projectRoot, UnsetName, params)
}
