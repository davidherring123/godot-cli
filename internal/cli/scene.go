package cli

import (
	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/spf13/cobra"
)

func NewSceneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scene",
		Short: "Inspect and manage scenes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newSceneListCommand(),
		newSceneTreeCommand(),
	)

	return cmd
}

func newSceneListCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       core.SceneList.Description,
		Long:        "List .tscn files as res:// paths in JSON, excluding the .godot directory. Does not require a running editor.",
		Example:     "godot-cli scene list",
		Annotations: map[string]string{skill.ActionKey: core.SceneList.Name},
		Args:        cobra.NoArgs,
		RunE:        runSceneList,
	}
}

func runSceneList(cmd *cobra.Command, args []string) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}
	result, err := scene.List(project.Root, scene.ListParams{})
	if err != nil {
		return err
	}
	return PrintJSON(result)
}

func newSceneTreeCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "tree <scene>",
		Short:       core.SceneTree.Description,
		Long:        "Load a scene resource and show its node tree as JSON. Requires a running Godot editor with the addon enabled. Reads a loaded resource, not the unsaved editor scene.",
		Example:     "godot-cli scene tree res://main.tscn",
		Annotations: map[string]string{skill.ActionKey: core.SceneTree.Name},
		Args:        cobra.ExactArgs(1),
		RunE:        runSceneTree,
	}
}

func runSceneTree(cmd *cobra.Command, args []string) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}
	result, err := scene.Tree(project.Root, scene.TreeParams{Scene: args[0]})
	if err != nil {
		return err
	}
	return PrintJSON(result)
}
