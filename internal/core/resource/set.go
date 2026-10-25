package resource

import (
	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

const SetName = "resource.set"

func Set(projectRoot string, params SetParams) (SetResult, error) {
	if params.Resource == "" {
		return SetResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A resource path is required",
		}
	}
	if len(params.Properties) == 0 {
		return SetResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "At least one property is required",
		}
	}
	return invocation.Call[SetParams, SetResult](projectRoot, SetName, params)
}
