package node

import (
	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
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
