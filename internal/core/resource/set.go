package resource

import "github.com/davidherring123/godot-cli/internal/bridge"

const SetName = "resource.set"

func Set(projectRoot string, params SetParams) (SetResult, error) {
	if params.Resource == "" {
		return SetResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A resource path is required",
		}
	}
	if len(params.Properties) == 0 {
		return SetResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "At least one property is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return SetResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[SetParams, SetResult](client, SetName, params)
}
