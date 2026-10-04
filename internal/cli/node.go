package cli

import (
	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/node"
	"github.com/spf13/cobra"
)

func NewNodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Inspect and modify nodes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newNodeInspectCommand(),
	)

	return cmd
}

func newNodeInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "inspect <scene> <node>",
		Short:       core.NodeInspect.Description,
		Long:        "Inspect a node's editor-visible properties as JSON. Requires a running Godot editor with the addon enabled. The scene is a resource path; the node is relative to its root (use . for the root). Reads a loaded resource, not the unsaved editor scene.",
		Example:     "godot-cli node inspect res://main.tscn .\ngodot-cli node inspect res://main.tscn Player",
		Annotations: map[string]string{skill.ActionKey: core.NodeInspect.Name},
		Args:        cobra.ExactArgs(2),
		RunE:        runNodeInspect,
	}
}

func runNodeInspect(cmd *cobra.Command, args []string) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}
	result, err := node.Inspect(project.Root, node.InspectParams{
		Scene: args[0],
		Node:  args[1],
	})
	if err != nil {
		return err
	}
	return PrintJSON(result)
}
