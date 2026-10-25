package input

const EventAddName = "input.event_add"

func EventAdd(projectRoot string, params EventAddParams) (EventAddResult, error) {
	if err := validateAction(params.Action); err != nil {
		return EventAddResult{}, err
	}

	if err := validateEvent(params.Event); err != nil {
		return EventAddResult{}, err
	}

	return call[EventAddParams, EventAddResult](projectRoot, EventAddName, params)
}
