package scene

import "github.com/davidherring123/godot-cli/internal/bridge"

const CurrentName = "scene.current"

func Current(projectRoot string, params CurrentParams) (CurrentResult, error) {
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return CurrentResult{}, err
	}
	defer client.Close()
	return bridge.Call[CurrentParams, CurrentResult](client, CurrentName, params)
}
