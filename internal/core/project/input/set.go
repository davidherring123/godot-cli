package input

const SetName = "input.set"

func Set(projectRoot string, params SetParams) (SetResult, error) {
	if err := validateAction(params.Action); err != nil {
		return SetResult{}, err
	}

	if err := validateDeadzone(params.Deadzone); err != nil {
		return SetResult{}, err
	}

	return call[SetParams, SetResult](projectRoot, SetName, params)
}
