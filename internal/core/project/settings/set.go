package settings

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const SetName = "project.settings.set"

func Set(projectRoot string, params SetParams) (SetResult, error) {
	if params.Setting == "" {
		return SetResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A project setting name is required",
		}
	}

	if len(params.Value) == 0 {
		return SetResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A value is required",
		}
	}

	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return SetResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[SetParams, SetResult](client, SetName, params)
}
