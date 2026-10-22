package node

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const DuplicateName = "node.duplicate"

func Duplicate(projectRoot string, params DuplicateParams) (DuplicateResult, error) {
	if params.Node == "" {
		return DuplicateResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return DuplicateResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[DuplicateParams, DuplicateResult](client, DuplicateName, params)
}
