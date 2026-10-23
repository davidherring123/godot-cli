package cli

import (
	"encoding/json"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/davidherring123/godot-cli/internal/core/resource"
	"github.com/spf13/cobra"
)

func NewResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Inspect and manage resource files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newResourceCreateCommand(),
		newResourceInspectCommand(),
		newResourceSetCommand(),
		newResourceDeleteCommand(),
		newResourceApplyCommand(),
	)

	return cmd
}

func newResourceCreateCommand() *cobra.Command {
	var resourceType string
	var rawProperties string
	cmd := &cobra.Command{
		Use:         "create <resource>",
		Short:       core.ResourceCreate.Description,
		Long:        "Create a .tres or .res resource file and write it to disk immediately. The type accepts Godot classes such as StandardMaterial3D and global script classes extending Resource; unknown, abstract, and non-resource types are rejected. Missing directories are created and an existing file is rejected. Resources such as textures and materials are assigned with {\"resource\": \"res://...\"} references. All values are validated before the file is written. Does not require an active scene and is not an editor undo step.",
		Example:     `godot-cli resource create res://materials/metal.tres --type StandardMaterial3D --properties '{"albedo_color":{"r":0.8,"g":0.8,"b":0.9,"a":1}}'`,
		Annotations: map[string]string{skill.ActionKey: core.ResourceCreate.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runProperties(rawProperties, func(
				project *project.Project,
				properties map[string]json.RawMessage,
			) (any, error) {
				return resource.Create(project.Root, resource.CreateParams{
					Resource:   args[0],
					Type:       resourceType,
					Properties: properties,
				})
			})
		},
	}
	cmd.Flags().StringVar(&resourceType, "type", "", "Resource class such as StandardMaterial3D; global script classes are supported")
	cmd.Flags().StringVar(&rawProperties, "properties", "", "JSON object of properties to apply when the resource is created")

	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func newResourceInspectCommand() *cobra.Command {
	var properties []string
	cmd := &cobra.Command{
		Use:         "inspect <resource>",
		Short:       core.ResourceInspect.Description,
		Long:        "Inspect a resource file's editor-visible properties. Pass --property to include only specific properties; an unknown or non-editor-visible filter fails the command. Resources are returned with their type, and resource-valued properties are returned as {\"resource\": \"res://...\"} references. Does not require an active scene.",
		Example:     "godot-cli resource inspect res://materials/metal.tres --property albedo_color",
		Annotations: map[string]string{skill.ActionKey: core.ResourceInspect.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}
			result, err := resource.Inspect(project.Root, resource.InspectParams{
				Resource:   args[0],
				Properties: properties,
			})
			if err != nil {
				return err
			}
			return PrintJSON(result)
		},
	}
	cmd.Flags().StringArrayVarP(&properties, "property", "p", nil, "Only include this property (repeatable)")
	return cmd
}

func newResourceSetCommand() *cobra.Command {
	var rawProperties string
	cmd := &cobra.Command{
		Use:         "set <resource>",
		Short:       core.ResourceSet.Description,
		Long:        "Set properties on a .tres or .res resource file and save it immediately. All properties are validated before anything is written; if saving fails the previous values are restored. Setting a property to its current value reports changed:false and does not rewrite the file. Supports the same values as node.set, including resource references like {\"resource\": \"res://...\"}. Does not require an active scene and is not an editor undo step.",
		Example:     `godot-cli resource set res://materials/metal.tres --properties '{"roughness":0.4,"albedo_color":{"r":1,"g":0,"b":0,"a":1}}'`,
		Annotations: map[string]string{skill.ActionKey: core.ResourceSet.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runProperties(rawProperties, func(
				project *project.Project,
				properties map[string]json.RawMessage,
			) (any, error) {
				return resource.Set(project.Root, resource.SetParams{
					Resource:   args[0],
					Properties: properties,
				})
			})
		},
	}
	cmd.Flags().StringVar(&rawProperties, "properties", "", "JSON object of properties to apply and save")

	_ = cmd.MarkFlagRequired("properties")

	return cmd
}

func newResourceDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "delete <resource>",
		Short:       core.ResourceDelete.Description,
		Long:        "Delete a .tres or .res resource file from disk permanently. Fails with the referencing paths when another project file, such as a scene or resource, still depends on it. Does not require an active scene and cannot be undone.",
		Example:     "godot-cli resource delete res://materials/metal.tres",
		Annotations: map[string]string{skill.ActionKey: core.ResourceDelete.Name},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}
			result, err := resource.Delete(project.Root, resource.DeleteParams{
				Resource: args[0],
			})
			if err != nil {
				return err
			}
			return PrintJSON(result)
		},
	}
}

func newResourceApplyCommand() *cobra.Command {
	var rawOperations string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: core.ResourceApply.Description,
		Long: "Apply an ordered batch of resource create, set, and delete operations. " +
			"Every operation is validated and applied in order; dry_run validates the full batch without writing. " +
			"Resources created earlier in a batch can be referenced by later create and set operations in both preview and apply mode. " +
			"Deletes check references introduced by earlier create/set operations as well as references already on disk; this tracking is conservative, so removing a reference earlier in the same batch can still block a later delete. " +
			"If a write fails after earlier operations, all affected files are restored on a best-effort basis and the error names the failed operation; a rollback failure returns internal_error. " +
			"Accepts 1-100 operations. dry_run may still run custom Resource scripts and property setters in memory, so scripts with external side effects are not side-effect free. " +
			"This batch is domain-local: it makes no promise that resource and project-setting or Input Map changes happen together. " +
			"Does not require an active scene and is not an editor undo step.",
		Example:     `godot-cli resource apply --operations '[{"operation":"create","resource":"res://materials/metal.tres","type":"StandardMaterial3D"},{"operation":"set","resource":"res://materials/metal.tres","properties":{"roughness":0.4}}]'`,
		Annotations: map[string]string{skill.ActionKey: core.ResourceApply.Name},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			operations, err := parseOperations[resource.ApplyOperation](rawOperations)
			if err != nil {
				return err
			}

			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := resource.Apply(project.Root, resource.ApplyParams{
				Operations: operations,
				DryRun:     dryRun,
			})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}
	cmd.Flags().StringVar(&rawOperations, "operations", "", "JSON array of ordered resource operations")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the full batch without writing any file")

	_ = cmd.MarkFlagRequired("operations")

	return cmd
}
