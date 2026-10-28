package validate

import "github.com/davidherring123/godot-mcp/internal/port/invocation"

func Scene(projectRoot string, params SceneParams) (SceneResult, error) {
	return invocation.Call[SceneParams, SceneResult](projectRoot, SceneName, params)
}
