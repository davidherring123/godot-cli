package input

const RenameName = "input.rename"

func Rename(projectRoot string, params RenameParams) (RenameResult, error) {
	if err := validateAction(params.Action); err != nil {
		return RenameResult{}, err
	}

	if params.NewName == "" {
		return RenameResult{}, validationError("A new action name is required")
	}

	return call[RenameParams, RenameResult](projectRoot, RenameName, params)
}
