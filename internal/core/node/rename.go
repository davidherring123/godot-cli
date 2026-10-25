package node

import (
	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

const RenameName = "node.rename"

func Rename(projectRoot string, params RenameParams) (RenameResult, error) {
	if params.Node == "" {
		return RenameResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	if params.Name == "" {
		return RenameResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A new name is required",
		}
	}
	return invocation.Call[RenameParams, RenameResult](projectRoot, RenameName, params)
}
