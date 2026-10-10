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
		newNodeAddCommand(),
		newNodeDeleteCommand(),
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
		Long:        "Set multiple properties on one node in the active editor scene as a single undo step. Supply a JSON object of property names to values. Supports bool, integer, float, string, StringName, NodePath, Vector2/3/4 (including integer vectors), Color, and Rect2/Rect2i. Vectors use x/y/z/w objects; colors use r/g/b/a; rectangles use position and size vectors. Resource-valued properties such as textures and materials are assigned with {\"resource\": \"res://...\"} references or null. All properties are validated before editing. Returns actual resulting values. Does not switch scenes; saves only when --save is passed.",
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

func newNodeAddCommand() *cobra.Command {
	var flags mutationFlags
	var nodeName string
	var rawProperties string
	cmd := &cobra.Command{
		Use:         "add <type> <parent>",
		Short:       core.NodeAdd.Description,
		Long:        "Add a new node under an existing parent in the active editor scene as a single undo step. The type is a Godot class such as Sprite2D; global script classes are accepted. Omit --name to generate a unique name based on the type; an explicit name fails if a sibling already uses it. Resources such as textures, materials, and scripts are assigned with {\"resource\": \"res://...\"} references. All values are validated before editing. Returns the resulting node path and property values. Does not switch scenes; saves only when --save is passed.",
		Example:     `godot-cli node add Sprite2D World --name Sprite --properties '{"position":{"x":10,"y":20},"texture":{"resource":"res://icon.svg"}}' --save --expect-scene res://main.tscn`,
		Annotations: map[string]string{skill.ActionKey: core.NodeAdd.Name},
		Args:        cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var properties map[string]json.RawMessage

			if rawProperties != "" {
				if err := json.Unmarshal([]byte(rawProperties), &properties); err != nil {
					return fmt.Errorf("properties must be a JSON object: %w", err)
				}
			}

			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := node.Add(project.Root, node.AddParams{
				Type:          args[0],
				Parent:        args[1],
				Name:          nodeName,
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
	cmd.Flags().StringVar(&nodeName, "name", "", "Name for the new node; defaults to a unique name based on the type")
	cmd.Flags().StringVar(&rawProperties, "properties", "", "JSON object of properties to apply when the node is added")
	return cmd
}

func newNodeDeleteCommand() *cobra.Command {
	var flags mutationFlags
	cmd := &cobra.Command{
		Use:         "delete <node>",
		Short:       core.NodeDelete.Description,
		Long:        "Delete a node from the active editor scene as a single undo step. The node path is relative to the scene root (use . for the root); the scene root itself cannot be deleted. Does not switch scenes; saves only when --save is passed.",
		Example:     `godot-cli node delete Player --save --expect-scene res://main.tscn`,
		Annotations: map[string]string{skill.ActionKey: core.NodeDelete.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := node.Delete(project.Root, node.DeleteParams{
				Node:          args[0],
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
