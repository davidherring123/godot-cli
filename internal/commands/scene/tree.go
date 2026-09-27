package scene

import (
	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/commands"
	"github.com/spf13/cobra"
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

func NewTreeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tree <scene>",
		Short: "Show a scene's node tree",
		Args:  cobra.ExactArgs(1),
		RunE:  runTree,
	}
}

func runTree(
	cmd *cobra.Command,
	args []string,
) error {
	context, err := commands.FetchProjectContext()
	if err != nil {
		return err
	}

	client, err := context.ConnectEditor()
	if err != nil {
		return err
	}
	defer client.Close()

	result, err := bridge.Call[
		TreeParams,
		TreeResult,
	](
		client,
		"scene.tree",
		TreeParams{
			Scene: args[0],
		},
	)
	if err != nil {
		return err
	}

	return commands.PrintJSON(result)
}
