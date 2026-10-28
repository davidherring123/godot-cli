package node

import (
	"github.com/davidherring123/godot-mcp/internal/port/invocation"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const InstantiateName = "node.instantiate"

func Instantiate(projectRoot string, params InstantiateParams) (InstantiateResult, error) {
	if params.Scene == "" {
		return InstantiateResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A scene resource path is required",
		}
	}
	if params.Parent == "" {
		return InstantiateResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A parent node path is required",
		}
	}
	return invocation.Call[InstantiateParams, InstantiateResult](projectRoot, InstantiateName, params)
}
