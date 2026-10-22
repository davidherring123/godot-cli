package settings

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const GetName = "project.settings.get"

func Get(projectRoot string, params GetParams) (GetResult, error) {
	if params.Setting == "" {
		return GetResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A project setting name is required",
		}
	}

	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return GetResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[GetParams, GetResult](client, GetName, params)
}
