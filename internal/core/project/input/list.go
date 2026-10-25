package input

const ListName = "input.list"

func List(projectRoot string, params ListParams) (ListResult, error) {
	return call[ListParams, ListResult](projectRoot, ListName, params)
}
