package scene

import "github.com/davidherring123/godot-cli/internal/port/invocation"

const TreeName = "scene.tree"

func Tree(projectRoot string, params TreeParams) (TreeResult, error) {
	return invocation.Call[TreeParams, TreeResult](projectRoot, TreeName, params)
}
