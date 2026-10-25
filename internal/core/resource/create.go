// Package resource manages .tres and .res files through a running Godot editor.
package resource

import (
	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

const CreateName = "resource.create"

func Create(projectRoot string, params CreateParams) (CreateResult, error) {
	if params.Resource == "" {
		return CreateResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A resource path is required",
		}
	}
	if params.Type == "" {
		return CreateResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A resource type is required",
		}
	}
	return invocation.Call[CreateParams, CreateResult](projectRoot, CreateName, params)
}
