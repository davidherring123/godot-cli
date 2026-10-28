package resource

import (
	"github.com/davidherring123/godot-mcp/internal/port/invocation"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const DeleteName = "resource.delete"

func Delete(projectRoot string, params DeleteParams) (DeleteResult, error) {
	if params.Resource == "" {
		return DeleteResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A resource path is required",
		}
	}
	return invocation.Call[DeleteParams, DeleteResult](projectRoot, DeleteName, params)
}
