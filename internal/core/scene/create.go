package scene

import (
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/invocation"
)

const CreateName = "scene.create"

func Create(projectRoot string, params CreateParams) (CreateResult, error) {
	if params.Scene == "" {
		return CreateResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A scene resource path is required",
		}
	}
	if params.RootType == "" {
		return CreateResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A root node type is required",
		}
	}
	return invocation.Call[CreateParams, CreateResult](projectRoot, CreateName, params)
}
