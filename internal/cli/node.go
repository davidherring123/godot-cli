package cli

import (
	"encoding/json"
	"fmt"

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
		newNodeSetCommand(),
	)

	return cmd
}

func newNodeInspectCommand() *cobra.Command {
	var sceneFlags editorSceneFlags
	var properties []string
	cmd := &cobra.Command{
		Use:         "inspect <node>",
		Short:       core.NodeInspect.Description,
		Long:        "Inspect a node's editor-visible properties in the active scene, including unsaved changes. The node path is relative to the scene root (use . for the root). Does not switch scenes.",
		Example:     "godot-cli node inspect .\ngodot-cli node inspect Player --property position --property visible --expect-scene res://main.tscn",
		Annotations: map[string]string{skill.ActionKey: core.NodeInspect.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodeInspect(args[0], properties, sceneFlags.expectedScene)
		},
	}
	sceneFlags.bind(cmd)
	cmd.Flags().StringArrayVarP(&properties, "property", "p", nil, "Only include this property (repeatable)")
	return cmd
}

func runNodeInspect(nodePath string, properties []string, expectedScene string) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}
	result, err := node.Inspect(project.Root, node.InspectParams{
		Node:          nodePath,
		Properties:    properties,
		ExpectedScene: expectedScene,
	})
	if err != nil {
		return err
	}
	return PrintJSON(result)
}

func newNodeSetCommand() *cobra.Command {
	var flags mutationFlags
	cmd := &cobra.Command{
		Use:         "set <node> <properties-json>",
		Short:       core.NodeSet.Description,
		Long:        "Set multiple properties on one node in the active editor scene as a single undo step. Supply a JSON object of property names to values. Supports bool, integer, float, string, StringName, NodePath, Vector2/3/4 (including integer vectors), Color, and Rect2/Rect2i. Vectors use x/y/z/w objects; colors use r/g/b/a; rectangles use position and size vectors. All properties are validated before editing. Returns actual resulting values. Does not switch scenes; saves only when --save is passed.",
		Example:     `godot-cli node set Player '{"position":{"x":100,"y":50},"visible":true}' --save --expect-scene res://main.tscn`,
		Annotations: map[string]string{skill.ActionKey: core.NodeSet.Name},
		Args:        cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var properties map[string]json.RawMessage

			if err := json.Unmarshal([]byte(args[1]), &properties); err != nil {
				return fmt.Errorf("properties must be a JSON object: %w", err)
			}

			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := node.Set(project.Root, node.SetParams{
				Node:          args[0],
				Properties:    properties,
				ExpectedScene: flags.expectedScene,
				Save:          flags.save,
			})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}
	flags.bind(cmd)
	return cmd
}
