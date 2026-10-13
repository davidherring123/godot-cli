package node

import "github.com/davidherring123/godot-cli/internal/bridge"

const RenameName = "node.rename"

func Rename(projectRoot string, params RenameParams) (RenameResult, error) {
	if params.Node == "" {
		return RenameResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	if params.Name == "" {
		return RenameResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A new name is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return RenameResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[RenameParams, RenameResult](client, RenameName, params)
}
