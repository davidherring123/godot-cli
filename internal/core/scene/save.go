package scene

import "github.com/davidherring123/godot-cli/internal/core/invocation"

const SaveName = "scene.save"

func Save(projectRoot string, params SaveParams) (SaveResult, error) {
	return invocation.Call[SaveParams, SaveResult](projectRoot, SaveName, params)
}
