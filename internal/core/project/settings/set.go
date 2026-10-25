package settings

import (
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/invocation"
)

const SetName = "project.settings.set"

func Set(projectRoot string, params SetParams) (SetResult, error) {
	if params.Setting == "" {
		return SetResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A project setting name is required",
		}
	}

	if len(params.Value) == 0 {
		return SetResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A value is required",
		}
	}

	return invocation.Call[SetParams, SetResult](projectRoot, SetName, params)
}
