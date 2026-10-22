package resource

import "github.com/davidherring123/godot-cli/internal/core/bridge"

const InspectName = "resource.inspect"

func Inspect(projectRoot string, params InspectParams) (InspectResult, error) {
	if params.Resource == "" {
		return InspectResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "A resource path is required",
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
	defer func() { _ = client.Close() }()
	return bridge.Call[InspectParams, InspectResult](client, InspectName, params)
}
