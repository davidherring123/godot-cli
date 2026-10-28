package node

import (
	"github.com/davidherring123/godot-mcp/internal/port/invocation"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const DeleteName = "node.delete"

func Delete(projectRoot string, params DeleteParams) (DeleteResult, error) {
	if params.Node == "" {
		return DeleteResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	return invocation.Call[DeleteParams, DeleteResult](projectRoot, DeleteName, params)
}
