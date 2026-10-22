package validate

import "github.com/davidherring123/godot-cli/internal/core/bridge"

func Live(projectRoot string, params LiveParams) (LiveResult, error) {
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return LiveResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[LiveParams, LiveResult](client, LiveName, params)
}
