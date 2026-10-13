package node

import "github.com/davidherring123/godot-cli/internal/bridge"

const MoveName = "node.move"

func Move(projectRoot string, params MoveParams) (MoveResult, error) {
	if params.Node == "" {
		return MoveResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	if params.Parent == "" {
		return MoveResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A parent node path is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return MoveResult{}, err
	}
	defer func() { _ = client.Close() }()
	return bridge.Call[MoveParams, MoveResult](client, MoveName, params)
}
