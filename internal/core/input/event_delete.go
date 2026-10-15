package input

import "github.com/davidherring123/godot-cli/internal/bridge"

const EventDeleteName = "input.event_delete"

func EventDelete(projectRoot string, params EventDeleteParams) (EventDeleteResult, error) {
	if err := validateAction(params.Action); err != nil {
		return EventDeleteResult{}, err
	}

	if params.Index < 0 {
		return EventDeleteResult{}, &bridge.APIError{
			Code:    bridge.ErrorCodeInvalidArgument,
			Message: "An event index is required",
		}
	}

	return call[EventDeleteParams, EventDeleteResult](
		projectRoot,
		EventDeleteName,
		params,
	)
}
