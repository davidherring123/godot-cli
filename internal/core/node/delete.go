package node

import "github.com/davidherring123/godot-cli/internal/bridge"

const DeleteName = "node.delete"

func Delete(projectRoot string, params DeleteParams) (DeleteResult, error) {
	if params.Node == "" {
		return DeleteResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return DeleteResult{}, err
	}
	defer client.Close()
	return bridge.Call[DeleteParams, DeleteResult](client, DeleteName, params)
}
