package input

const InspectName = "input.inspect"

func Inspect(projectRoot string, params InspectParams) (InspectResult, error) {
	if err := validateAction(params.Action); err != nil {
		return InspectResult{}, err
	}

	return call[InspectParams, InspectResult](projectRoot, InspectName, params)
}
