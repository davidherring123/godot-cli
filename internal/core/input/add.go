package input

const AddName = "input.add"

func Add(projectRoot string, params AddParams) (AddResult, error) {
	if err := validateAction(params.Action); err != nil {
		return AddResult{}, err
	}

	if params.Deadzone != nil {
		if err := validateDeadzone(*params.Deadzone); err != nil {
			return AddResult{}, err
		}
	}

	return call[AddParams, AddResult](projectRoot, AddName, params)
}
