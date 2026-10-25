package scene

import (
	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

const OpenName = "scene.open"

func Open(projectRoot string, params OpenParams) (OpenResult, error) {
	if params.Scene == "" {
		return OpenResult{}, &apierr.Error{
			Code:    apierr.CodeInvalidArgument,
			Message: "A scene path is required",
		}
	}
	return invocation.Call[OpenParams, OpenResult](projectRoot, OpenName, params)
}
