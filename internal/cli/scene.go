package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/spf13/cobra"
)

func NewSceneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scene",
		Short: "Inspect and manage scenes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newSceneListCommand(),
		newSceneCurrentCommand(),
		newSceneOpenCommand(),
		newSceneCreateCommand(),
		newSceneTreeCommand(),
		newSceneSaveCommand(),
		newSceneApplyCommand(),
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

func runSceneList(_ *cobra.Command, _ []string) error {
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

func newSceneCreateCommand() *cobra.Command {
	var rootType string
	var sceneName string
	var rawProperties string
	cmd := &cobra.Command{
		Use:         "create <scene>",
		Short:       core.SceneCreate.Description,
		Long:        "Create a new scene file containing one root node and open it as the active editor scene. The path must end in .tscn or .scn, missing directories are created, and an existing file is rejected. The root type accepts Godot classes such as Node2D and global script classes. Omit --name to derive the root name from the file name. Resources such as scripts, textures, and materials are assigned with {\"resource\": \"res://...\"} references. All values are validated before the file is written. The file is written once and opened; it is not an editor undo step.",
		Example:     `godot-cli scene create res://levels/arena.tscn --root-type Node2D --name Arena`,
		Annotations: map[string]string{skill.ActionKey: core.SceneCreate.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runProperties(rawProperties, func(
				project *project.Project,
				properties map[string]json.RawMessage,
			) (any, error) {
				return scene.Create(project.Root, scene.CreateParams{
					Scene:      args[0],
					RootType:   rootType,
					Name:       sceneName,
					Properties: properties,
				})
			})
		},
	}
	cmd.Flags().StringVar(&rootType, "root-type", "", "Root node class such as Node2D; global script classes are supported")
	cmd.Flags().StringVar(&sceneName, "name", "", "Root node name; defaults to the file name without its extension")
	cmd.Flags().StringVar(&rawProperties, "properties", "", "JSON object of root properties to apply when the scene is created")

	_ = cmd.MarkFlagRequired("root-type")

	return cmd
}

func newSceneTreeCommand() *cobra.Command {
	var flags editorSceneFlags
	cmd := &cobra.Command{
		Use:         "tree",
		Short:       core.SceneTree.Description,
		Long:        "Read the active editor scene's node tree as JSON, including unsaved changes. Requires a running editor with the addon enabled. Does not open or switch scenes.",
		Example:     "godot-cli scene tree\ngodot-cli scene tree --expect-scene res://main.tscn",
		Annotations: map[string]string{skill.ActionKey: core.SceneTree.Name},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSceneTree(flags.expectedScene)
		},
	}
	flags.bind(cmd)
	return cmd
}

func runSceneTree(expectedScene string) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}
	result, err := scene.Tree(project.Root, scene.TreeParams{ExpectedScene: expectedScene})
	if err != nil {
		return err
	}
	return PrintJSON(result)
}

func newSceneCurrentCommand() *cobra.Command {
	var flags editorSceneFlags
	cmd := &cobra.Command{
		Use:         "current",
		Short:       core.SceneCurrent.Description,
		Long:        "Identify the active editor scene and whether it has unsaved changes. Succeeds with active:false when no scene is being edited; an empty scene path with active:true means the scene has never been saved. --expect-scene fails when a different scene is active or no scene is open.",
		Example:     "godot-cli scene current\ngodot-cli scene current --expect-scene res://main.tscn",
		Annotations: map[string]string{skill.ActionKey: core.SceneCurrent.Name},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}
			result, err := scene.Current(project.Root, scene.CurrentParams{
				ExpectedScene: flags.expectedScene,
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

func newSceneOpenCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "open <scene>",
		Short:       core.SceneOpen.Description,
		Long:        "Open a res:// scene path through Godot or activate its existing tab, preserving unsaved edits. Returns only after confirming that scene is active. Does not save or reload scenes.",
		Example:     "godot-cli scene open res://main.tscn",
		Annotations: map[string]string{skill.ActionKey: core.SceneOpen.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}
			result, err := scene.Open(project.Root, scene.OpenParams{Scene: args[0]})
			if err != nil {
				return err
			}
			return PrintJSON(result)
		},
	}
}

func newSceneApplyCommand() *cobra.Command {
	var rawOperations string
	var dryRun bool
	var flags mutationFlags
	cmd := &cobra.Command{
		Use:         "apply",
		Short:       core.SceneApply.Description,
		Long:        "Apply an ordered array of node operations to the active editor scene as one atomic undo step. Each operation names its own `op`: node.add, node.set, node.rename, node.duplicate, node.move, node.delete, or node.instantiate, and uses the same fields as the matching single-node command. Give an operation an `id` and later operations can target its resulting node with `@id` in node or parent, so a batch can build and edit new nodes in one call. The whole batch is first validated on a temporary copy of the scene: if any operation fails, the live scene is untouched, and a failure during the real apply rolls back through the editor undo stack. --dry-run returns the planned paths and summaries without editing. Use --save once to save the scene after the batch succeeds. Operations run in order and commit as one undo step.",
		Example:     `godot-cli scene apply --operations '[{"op":"node.add","id":"spawn","type":"Marker2D","parent":"World","name":"Spawn","properties":{"position":{"x":32,"y":48}}},{"op":"node.set","node":"@spawn","properties":{"position":{"x":64,"y":96}}}]' --save --expect-scene res://main.tscn`,
		Annotations: map[string]string{skill.ActionKey: core.SceneApply.Name},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			var operations []scene.ApplyOperation

			decoder := json.NewDecoder(strings.NewReader(rawOperations))
			decoder.DisallowUnknownFields()

			if err := decoder.Decode(&operations); err != nil {
				return fmt.Errorf("operations must be a JSON array of scene operations: %w", err)
			}

			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := scene.Apply(project.Root, scene.ApplyParams{
				Operations:    operations,
				DryRun:        dryRun,
				ExpectedScene: flags.expectedScene,
				Save:          flags.save,
			})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}

	cmd.Flags().StringVar(&rawOperations, "operations", "", "JSON array of scene operations to apply in order")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and plan the batch without editing the active scene")
	flags.bind(cmd)

	_ = cmd.MarkFlagRequired("operations")

	return cmd
}

func newSceneSaveCommand() *cobra.Command {
	var flags editorSceneFlags
	cmd := &cobra.Command{
		Use:         "save",
		Short:       core.SceneSave.Description,
		Long:        "Save the active scene through Godot's editor to its existing resource path. Does not switch scenes or save other tabs. A scene without a path must first be saved using the editor's Save As.",
		Example:     "godot-cli scene save --expect-scene res://main.tscn",
		Annotations: map[string]string{skill.ActionKey: core.SceneSave.Name},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}
			result, err := scene.Save(project.Root, scene.SaveParams{ExpectedScene: flags.expectedScene})
			if err != nil {
				return err
			}
			return PrintJSON(result)
		},
	}
	flags.bind(cmd)
	return cmd
}
