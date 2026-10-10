package node

import (
	"encoding/json"
	"fmt"

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
	for name, value := range params.Properties {
		if name == "" || !json.Valid(value) {
			return AddResult{}, &bridge.APIError{
				Code: bridge.ErrorCodeInvalidArgument,
				Message: fmt.Sprintf(
					"%q requires a name and valid JSON value",
					name,
				),
			}
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return AddResult{}, err
	}
	defer client.Close()
	return bridge.Call[AddParams, AddResult](client, AddName, params)
}
