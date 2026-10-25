package resource

import (
	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

const InspectName = "resource.inspect"

func Inspect(projectRoot string, params InspectParams) (InspectResult, error) {
	if params.Resource == "" {
		return InspectResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A resource path is required",
		}
	}
	for _, property := range params.Properties {
		if property == "" {
			return InspectResult{}, &apierr.Error{
				Code:    apierr.CodeInvalidArgument,
				Message: "Property filters must be nonempty",
			}
		}
	}
	return invocation.Call[InspectParams, InspectResult](projectRoot, InspectName, params)
}
