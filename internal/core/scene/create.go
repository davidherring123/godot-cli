package scene

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const CreateName = "scene.create"

func Create(projectRoot string, params CreateParams) (CreateResult, error) {
	if params.Scene == "" {
		return CreateResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A scene resource path is required",
		}
	}
	if params.RootType == "" {
		return CreateResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A root node type is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return CreateResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[CreateParams, CreateResult](client, CreateName, params)
}
