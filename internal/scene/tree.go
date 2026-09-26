package scene

import (
	"github.com/davidherring123/godot-cli/internal/bridge"
)

type TreeNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Path     string     `json:"path"`
	Children []TreeNode `json:"children"`
}

type TreeResult struct {
	Scene string   `json:"scene"`
	Root  TreeNode `json:"root"`
}

type TreeParams struct {
	Scene string `json:"scene"`
}

func Tree(
	client *bridge.Client,
	scenePath string,
) (TreeResult, error) {
	return bridge.Call[TreeParams, TreeResult](
		client,
		"scene.tree",
		TreeParams{
			Scene: scenePath,
		},
	)
}
