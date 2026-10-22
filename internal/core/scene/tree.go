package scene

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const TreeName = "scene.tree"

func Tree(projectRoot string, params TreeParams) (TreeResult, error) {
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return TreeResult{}, err
	}
	defer func() { _ = client.Close() }()

	return bridge.Call[TreeParams, TreeResult](client, TreeName, params)
}
