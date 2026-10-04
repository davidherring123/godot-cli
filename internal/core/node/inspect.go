package node

import (
	"errors"

	"github.com/davidherring123/godot-cli/internal/bridge"
)

const InspectName = "node.inspect"

func Inspect(projectRoot string, params InspectParams) (InspectResult, error) {
	if params.Scene == "" {
		return InspectResult{}, errors.New("invalid_scene_path: A scene path is required")
	}
	if params.Node == "" {
		return InspectResult{}, errors.New("invalid_node_path: A node path is required")
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return InspectResult{}, err
	}
	defer client.Close()

	return bridge.Call[InspectParams, InspectResult](client, InspectName, params)
}
