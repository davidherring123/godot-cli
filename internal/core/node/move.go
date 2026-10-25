package node

import (
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/invocation"
)

const MoveName = "node.move"

func Move(projectRoot string, params MoveParams) (MoveResult, error) {
	if params.Node == "" {
		return MoveResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	if params.Parent == "" {
		return MoveResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A parent node path is required",
		}
	}
	return invocation.Call[MoveParams, MoveResult](projectRoot, MoveName, params)
}
