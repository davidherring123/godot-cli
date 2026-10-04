package scene

import (
	"errors"

	"github.com/davidherring123/godot-cli/internal/bridge"
)

const TreeName = "scene.tree"

func Tree(projectRoot string, params TreeParams) (TreeResult, error) {
	if params.Scene == "" {
		return TreeResult{}, errors.New("invalid_scene_path: A scene path is required")
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return TreeResult{}, err
	}
	defer client.Close()

	return bridge.Call[TreeParams, TreeResult](client, TreeName, params)
}
