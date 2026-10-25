package settings

import "github.com/davidherring123/godot-cli/internal/port/invocation"

const ListName = "project.settings.list"

func List(projectRoot string, params ListParams) (ListResult, error) {
	return invocation.Call[ListParams, ListResult](projectRoot, ListName, params)
}
