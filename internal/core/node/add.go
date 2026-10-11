package node

import (
	"github.com/davidherring123/godot-cli/internal/bridge"
)

const AddName = "node.add"

func Add(projectRoot string, params AddParams) (AddResult, error) {
	if params.Type == "" {
		return AddResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A node type is required",
		}
	}
	if params.Parent == "" {
		return AddResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A parent node path is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return AddResult{}, err
	}
	defer client.Close()
	return bridge.Call[AddParams, AddResult](client, AddName, params)
}
