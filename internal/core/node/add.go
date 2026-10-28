// Package node inspects and modifies nodes in the active editor scene.
package node

import (
	"github.com/davidherring123/godot-mcp/internal/port/invocation"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const AddName = "node.add"

func Add(projectRoot string, params AddParams) (AddResult, error) {
	if params.Type == "" {
		return AddResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A node type is required",
		}
	}
	if params.Parent == "" {
		return AddResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A parent node path is required",
		}
	}
	return invocation.Call[AddParams, AddResult](projectRoot, AddName, params)
}
