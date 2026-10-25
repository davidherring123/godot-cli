package input

const DeleteName = "input.delete"

func Delete(projectRoot string, params DeleteParams) (DeleteResult, error) {
	if err := validateAction(params.Action); err != nil {
		return DeleteResult{}, err
	}

	return call[DeleteParams, DeleteResult](projectRoot, DeleteName, params)
}
