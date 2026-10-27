package settings

import (
	"github.com/davidherring123/godot-mcp/internal/port/invocation"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const GetName = "project.settings.get"

func Get(projectRoot string, params GetParams) (GetResult, error) {
	if params.Setting == "" {
		return GetResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A project setting name is required",
		}
	}

	return invocation.Call[GetParams, GetResult](projectRoot, GetName, params)
}
