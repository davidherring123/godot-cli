package node

import (
	"github.com/davidherring123/godot-mcp/internal/port/invocation"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const InspectName = "node.inspect"

func Inspect(projectRoot string, params InspectParams) (InspectResult, error) {
	if params.Node == "" {
		return InspectResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A node path is required",
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
