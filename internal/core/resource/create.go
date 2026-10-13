// Package resource manages .tres and .res files through a running Godot editor.
package resource

import "github.com/davidherring123/godot-cli/internal/bridge"

const CreateName = "resource.create"

func Create(projectRoot string, params CreateParams) (CreateResult, error) {
	if params.Resource == "" {
		return CreateResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A resource path is required",
		}
	}
	if params.Type == "" {
		return CreateResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A resource type is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return CreateResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[CreateParams, CreateResult](client, CreateName, params)
}
