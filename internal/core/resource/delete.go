package resource

import "github.com/davidherring123/godot-cli/internal/bridge"

const DeleteName = "resource.delete"

func Delete(projectRoot string, params DeleteParams) (DeleteResult, error) {
	if params.Resource == "" {
		return DeleteResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A resource path is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return DeleteResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[DeleteParams, DeleteResult](client, DeleteName, params)
}
