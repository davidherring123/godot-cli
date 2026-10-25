package resource

import (
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/invocation"
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
