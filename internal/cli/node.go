package cli

import (
	"encoding/json"
	"fmt"
	"strings"

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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newNodeInspectCommand(),
		newNodeSetCommand(),
		newNodeAddCommand(),
		newNodeInstantiateCommand(),
		newNodeRenameCommand(),
		newNodeDuplicateCommand(),
		newNodeMoveCommand(),
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
		RunE: func(_ *cobra.Command, args []string) error {
			return runNodeParams("", func(
				project *core.Project,
				_ map[string]json.RawMessage,
			) (any, error) {
				return node.Inspect(project.Root, node.InspectParams{
					Node:          args[0],
					Properties:    properties,
					ExpectedScene: sceneFlags.expectedScene,
				})
			})
		},
	}
	sceneFlags.bind(cmd)
	cmd.Flags().StringArrayVarP(&properties, "property", "p", nil, "Only include this property (repeatable)")
	return cmd
}

func newNodeSetCommand() *cobra.Command {
	cmd := newNodeCommand(nodeCommandSpec{
		use:        "set <node>",
		short:      core.NodeSet.Description,
		long:       "Set multiple properties on one node in the active editor scene as a single undo step. Supply a JSON object of property names to values with --properties. Supports bool, integer, float, string, StringName, NodePath, Vector2/3/4 (including integer vectors), Color, and Rect2/Rect2i. Vectors use x/y/z/w objects; colors use r/g/b/a; rectangles use position and size vectors. Resource-valued properties such as textures, materials, and scripts are assigned with {\"resource\": \"res://...\"} references or null. All properties are validated before editing. Returns actual resulting values. Does not switch scenes; saves only when --save is passed.",
		example:    `godot-cli node set Player --properties '{"position":{"x":100,"y":50},"visible":true}' --save --expect-scene res://main.tscn`,
		action:     core.NodeSet.Name,
		properties: "JSON object of properties to apply",
		parameters: func(
			_ *cobra.Command,
			args []string,
			_ string,
			flags mutationFlags,
		) nodeParamsFunc {
			return func(
				project *core.Project,
				properties map[string]json.RawMessage,
			) (any, error) {
				return node.Set(project.Root, node.SetParams{
					Node:          args[0],
					Properties:    properties,
					ExpectedScene: flags.expectedScene,
					Save:          flags.save,
				})
			}
		},
	})

	_ = cmd.MarkFlagRequired("properties")

	return cmd
}

type nodeParamsFunc func(
	project *core.Project,
	properties map[string]json.RawMessage,
) (any, error)

type nodeCommandSpec struct {
	use        string
	short      string
	long       string
	example    string
	action     string
	nameUsage  string
	properties string
	parameters func(
		cmd *cobra.Command,
		args []string,
		name string,
		flags mutationFlags,
	) nodeParamsFunc
}

func newNodeCommand(spec nodeCommandSpec) *cobra.Command {
	var flags mutationFlags
	var name string
	var rawProperties string
	cmd := &cobra.Command{
		Use:         spec.use,
		Short:       spec.short,
		Long:        spec.long,
		Example:     spec.example,
		Annotations: map[string]string{skill.ActionKey: spec.action},
		Args:        cobra.ExactArgs(strings.Count(spec.use, " ")),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodeParams(rawProperties, spec.parameters(cmd, args, name, flags))
		},
	}
	flags.bind(cmd)

	if spec.nameUsage != "" {
		cmd.Flags().StringVar(&name, "name", "", spec.nameUsage)
	}

	if spec.properties != "" {
		cmd.Flags().StringVar(&rawProperties, "properties", "", spec.properties)
	}

	return cmd
}

func newNodeAddCommand() *cobra.Command {
	return newNodeCommand(nodeCommandSpec{
		use:        "add <type> <parent>",
		short:      core.NodeAdd.Description,
		long:       "Add a new node under an existing parent in the active editor scene as a single undo step. The type is a Godot class such as Sprite2D; global script classes are accepted. Omit --name to generate a unique name based on the type; an explicit name fails if a sibling already uses it. Resources such as textures, materials, and scripts are assigned with {\"resource\": \"res://...\"} references. All values are validated before editing. Returns the resulting node path and property values. Does not switch scenes; saves only when --save is passed.",
		example:    `godot-cli node add Sprite2D World --name Sprite --properties '{"position":{"x":10,"y":20},"texture":{"resource":"res://icon.svg"}}' --save --expect-scene res://main.tscn`,
		action:     core.NodeAdd.Name,
		nameUsage:  "Name for the new node; defaults to a unique name based on the type",
		properties: "JSON object of properties to apply when the node is added",
		parameters: func(
			_ *cobra.Command,
			args []string,
			name string,
			flags mutationFlags,
		) nodeParamsFunc {
			return func(
				project *core.Project,
				properties map[string]json.RawMessage,
			) (any, error) {
				return node.Add(project.Root, node.AddParams{
					Type:          args[0],
					Parent:        args[1],
					Name:          name,
					Properties:    properties,
					ExpectedScene: flags.expectedScene,
					Save:          flags.save,
				})
			}
		},
	})
}

func newNodeInstantiateCommand() *cobra.Command {
	return newNodeCommand(nodeCommandSpec{
		use:        "instantiate <scene> <parent>",
		short:      core.NodeInstantiate.Description,
		long:       "Add a linked instance of an existing PackedScene under a parent in the active editor scene as a single undo step. The instance stays connected to its source scene, so changes in the source propagate to every instance; only the instance root can be overridden here. The source scene must be saved first, because instancing reads it from disk and an unsaved source is rejected. Omit --name to use the source scene root's name, uniquified; an explicit name fails if a sibling already uses it. Resources such as textures and materials are assigned with {\"resource\": \"res://...\"} references. All values are validated before editing. Returns the resulting node path and property values. Does not switch scenes; saves only when --save is passed.",
		example:    `godot-cli node instantiate res://player.tscn World --name Player --properties '{"position":{"x":64,"y":96}}' --save --expect-scene res://main.tscn`,
		action:     core.NodeInstantiate.Name,
		nameUsage:  "Name for the instance root; defaults to the source scene root's name",
		properties: "JSON object of root property overrides to apply when the instance is added",
		parameters: func(
			_ *cobra.Command,
			args []string,
			name string,
			flags mutationFlags,
		) nodeParamsFunc {
			return func(
				project *core.Project,
				properties map[string]json.RawMessage,
			) (any, error) {
				return node.Instantiate(project.Root, node.InstantiateParams{
					Scene:         args[0],
					Parent:        args[1],
					Name:          name,
					Properties:    properties,
					ExpectedScene: flags.expectedScene,
					Save:          flags.save,
				})
			}
		},
	})
}

func newNodeRenameCommand() *cobra.Command {
	return newNodeCommand(nodeCommandSpec{
		use:     "rename <node> <name>",
		short:   core.NodeRename.Description,
		long:    "Rename a node in the active editor scene as a single undo step. NodePath references to the node and its children are updated across the scene, including animation tracks, matching the editor's rename behavior. Fails if a sibling already uses the name; renaming a node to its current name is a successful no-op. The node path is relative to the scene root (use . for the root). Returns the path and name before and after the rename. Does not switch scenes; saves only when --save is passed.",
		example: `godot-cli node rename Player Hero --save --expect-scene res://main.tscn`,
		action:  core.NodeRename.Name,
		parameters: func(
			_ *cobra.Command,
			args []string,
			_ string,
			flags mutationFlags,
		) nodeParamsFunc {
			return func(
				project *core.Project,
				_ map[string]json.RawMessage,
			) (any, error) {
				return node.Rename(project.Root, node.RenameParams{
					Node:          args[0],
					Name:          args[1],
					ExpectedScene: flags.expectedScene,
					Save:          flags.save,
				})
			}
		},
	})
}

func newNodeDuplicateCommand() *cobra.Command {
	return newNodeCommand(nodeCommandSpec{
		use:       "duplicate <node>",
		short:     core.NodeDuplicate.Description,
		long:      "Duplicate a node and its children under the same parent in the active editor scene as a single undo step, placed right after the original. The copy preserves scripts, property values, scene-instance links, and persistent connections. The scene root cannot be duplicated. Omit --name to generate a unique name based on the original; an explicit name fails if a sibling already uses it. Returns the original path and name plus the copy's path, name, and type. Does not switch scenes; saves only when --save is passed.",
		example:   `godot-cli node duplicate World/Player --name Player2 --save --expect-scene res://main.tscn`,
		action:    core.NodeDuplicate.Name,
		nameUsage: "Name for the copy; defaults to a unique name based on the original",
		parameters: func(
			_ *cobra.Command,
			args []string,
			name string,
			flags mutationFlags,
		) nodeParamsFunc {
			return func(
				project *core.Project,
				_ map[string]json.RawMessage,
			) (any, error) {
				return node.Duplicate(project.Root, node.DuplicateParams{
					Node:          args[0],
					Name:          name,
					ExpectedScene: flags.expectedScene,
					Save:          flags.save,
				})
			}
		},
	})
}

func newNodeMoveCommand() *cobra.Command {
	var index int
	var keepGlobalTransform bool
	cmd := newNodeCommand(nodeCommandSpec{
		use:     "move <node> <parent>",
		short:   core.NodeMove.Description,
		long:    "Move a node under a new parent or to a new position in the active editor scene as a single undo step. Use --index for the final zero-based position among the parent's children; when omitted the node is moved to the end. The node keeps its name unless the new parent already has a child with that name, in which case a unique name is generated. NodePath references and animation tracks pointing at the node and its children are updated like the editor. A node cannot be moved under itself, and the scene root cannot be moved. Does not switch scenes; saves only when --save is passed.",
		example: `godot-cli node move Player World/Entities --index 0 --save --expect-scene res://main.tscn`,
		action:  core.NodeMove.Name,
		parameters: func(
			cmd *cobra.Command,
			args []string,
			_ string,
			flags mutationFlags,
		) nodeParamsFunc {
			return func(
				project *core.Project,
				_ map[string]json.RawMessage,
			) (any, error) {
				params := node.MoveParams{
					Node:                args[0],
					Parent:              args[1],
					KeepGlobalTransform: &keepGlobalTransform,
					ExpectedScene:       flags.expectedScene,
					Save:                flags.save,
				}

				if cmd.Flags().Changed("index") {
					value := index
					params.Index = &value
				}

				return node.Move(project.Root, params)
			}
		},
	})

	cmd.Flags().IntVar(&index, "index", -1, "Zero-based position among the new parent's children; omit to move the node to the end")
	cmd.Flags().BoolVar(
		&keepGlobalTransform,
		"keep-global-transform",
		true,
		"Keep the node's global transform when its parent changes; pass --keep-global-transform=false to keep its local transform",
	)

	return cmd
}

func runNodeParams(
	rawProperties string,
	call func(*core.Project, map[string]json.RawMessage) (any, error),
) error {
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

	result, err := call(project, properties)
	if err != nil {
		return err
	}

	return PrintJSON(result)
}

func newNodeDeleteCommand() *cobra.Command {
	return newNodeCommand(nodeCommandSpec{
		use:     "delete <node>",
		short:   core.NodeDelete.Description,
		long:    "Delete a node from the active editor scene as a single undo step. The node path is relative to the scene root (use . for the root); the scene root itself cannot be deleted. Does not switch scenes; saves only when --save is passed.",
		example: `godot-cli node delete Player --save --expect-scene res://main.tscn`,
		action:  core.NodeDelete.Name,
		parameters: func(
			_ *cobra.Command,
			args []string,
			_ string,
			flags mutationFlags,
		) nodeParamsFunc {
			return func(
				project *core.Project,
				_ map[string]json.RawMessage,
			) (any, error) {
				return node.Delete(project.Root, node.DeleteParams{
					Node:          args[0],
					ExpectedScene: flags.expectedScene,
					Save:          flags.save,
				})
			}
		},
	})
}
