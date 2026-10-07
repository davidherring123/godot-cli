package scene

import "github.com/davidherring123/godot-cli/internal/bridge"

const SaveName = "scene.save"

func Save(projectRoot string, params SaveParams) (SaveResult, error) {
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return SaveResult{}, err
	}
	defer client.Close()
	return bridge.Call[SaveParams, SaveResult](client, SaveName, params)
}
