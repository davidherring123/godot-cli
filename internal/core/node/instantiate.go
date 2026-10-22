package node

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const InstantiateName = "node.instantiate"

func Instantiate(projectRoot string, params InstantiateParams) (InstantiateResult, error) {
	if params.Scene == "" {
		return InstantiateResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A scene resource path is required",
		}
	}
	if params.Parent == "" {
		return InstantiateResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A parent node path is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return InstantiateResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[InstantiateParams, InstantiateResult](client, InstantiateName, params)
}
