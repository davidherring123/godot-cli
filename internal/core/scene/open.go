package scene

import "github.com/davidherring123/godot-cli/internal/bridge"

const OpenName = "scene.open"

func Open(projectRoot string, params OpenParams) (OpenResult, error) {
	if params.Scene == "" {
		return OpenResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A scene path is required",
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return OpenResult{}, err
	}
	defer client.Close()
	return bridge.Call[OpenParams, OpenResult](client, OpenName, params)
}
