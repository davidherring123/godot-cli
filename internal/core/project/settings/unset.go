package settings

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const UnsetName = "project.settings.unset"

func Unset(projectRoot string, params UnsetParams) (UnsetResult, error) {
	if params.Setting == "" {
		return UnsetResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A project setting name is required",
		}
	}

	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return UnsetResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[UnsetParams, UnsetResult](client, UnsetName, params)
}
