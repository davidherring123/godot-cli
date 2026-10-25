package node

import (
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/invocation"
)

const DuplicateName = "node.duplicate"

func Duplicate(projectRoot string, params DuplicateParams) (DuplicateResult, error) {
	if params.Node == "" {
		return DuplicateResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	return invocation.Call[DuplicateParams, DuplicateResult](projectRoot, DuplicateName, params)
}
