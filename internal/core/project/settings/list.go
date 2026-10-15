package settings

import "github.com/davidherring123/godot-cli/internal/bridge"

const ListName = "project.settings.list"

func List(projectRoot string, params ListParams) (ListResult, error) {
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return ListResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[ListParams, ListResult](client, ListName, params)
}
