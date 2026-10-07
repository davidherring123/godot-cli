package node

import "github.com/davidherring123/godot-cli/internal/bridge"

const InspectName = "node.inspect"

func Inspect(projectRoot string, params InspectParams) (InspectResult, error) {
	if params.Node == "" {
		return InspectResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A node path is required",
		}
	}
	for _, property := range params.Properties {
		if property == "" {
			return InspectResult{}, &bridge.APIError{
				Code:    bridge.ErrorCodeInvalidArgument,
				Message: "Property filters must be nonempty",
			}
		}
	}
	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return InspectResult{}, err
	}
	defer client.Close()

	return bridge.Call[InspectParams, InspectResult](client, InspectName, params)
}
