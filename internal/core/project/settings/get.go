package settings

import (
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/invocation"
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
