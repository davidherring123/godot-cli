package cli

import (
	"encoding/json"
	"fmt"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/project/settings"
	"github.com/spf13/cobra"
)

func NewProjectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Inspect and edit project configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newProjectSettingsCommand())

	return cmd
}

func newProjectSettingsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Inspect and edit project settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newProjectSettingsListCommand(),
		newProjectSettingsGetCommand(),
		newProjectSettingsSetCommand(),
		newProjectSettingsUnsetCommand(),
		newProjectSettingsApplyCommand(),
	)

	return cmd
}

func newProjectSettingsListCommand() *cobra.Command {
	var prefix string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List project settings",
		Long:    "List editor-visible project settings with their type, current value, engine default, and whether project.godot overrides them. Use --prefix to narrow the list to a settings group such as display/window. Requires a running editor with the addon enabled.",
		Example: "godot-cli project settings list --prefix display/window",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runProjectSettings(func(root string) (any, error) {
				return settings.List(root, settings.ListParams{Prefix: prefix})
			})
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "Only include settings whose name starts with this prefix")
	return cmd
}

func newProjectSettingsGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "get <setting>",
		Short:   "Show a project setting",
		Long:    "Show one project setting's type, current value, engine default, and whether project.godot overrides it. Fails for unknown settings. Requires a running editor with the addon enabled.",
		Example: "godot-cli project settings get display/window/size/viewport_width",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runProjectSettings(func(root string) (any, error) {
				return settings.Get(root, settings.GetParams{Setting: args[0]})
			})
		},
	}
}

func newProjectSettingsSetCommand() *cobra.Command {
	var value string
	cmd := &cobra.Command{
		Use:     "set <setting>",
		Short:   "Set a project setting",
		Long:    "Set one project setting to a JSON --value and save project.godot immediately. The value must match the setting's Godot type; scalar values, vectors, colors, rectangles, arrays, dictionaries, and packed arrays are supported. Setting a value equal to the current one reports changed:false and does not rewrite the file. This is a direct project-file change, not an editor undo step. Requires a running editor with the addon enabled.",
		Example: "godot-cli project settings set display/window/size/viewport_width --value 1920",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			parsed, err := parseJSONValue(value)
			if err != nil {
				return err
			}

			return runProjectSettings(func(root string) (any, error) {
				return settings.Set(root, settings.SetParams{
					Setting: args[0],
					Value:   parsed,
				})
			})
		},
	}
	cmd.Flags().StringVar(&value, "value", "", "JSON value matching the setting's Godot type")

	_ = cmd.MarkFlagRequired("value")

	return cmd
}

func newProjectSettingsUnsetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "unset <setting>",
		Short:   "Remove a project setting override",
		Long:    "Remove a setting's override from project.godot, restoring the engine default, and save the file immediately. Reporting changed:false means the project did not override the setting. This is a direct project-file change, not an editor undo step. Requires a running editor with the addon enabled.",
		Example: "godot-cli project settings unset display/window/size/viewport_width",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runProjectSettings(func(root string) (any, error) {
				return settings.Unset(root, settings.UnsetParams{Setting: args[0]})
			})
		},
	}
}

func newProjectSettingsApplyCommand() *cobra.Command {
	var rawOperations string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: core.ProjectSettingsApply.Description,
		Long: "Apply an ordered batch of project-setting set and unset operations. " +
			"Every operation is validated against the editor's setting definitions and applied in order, with set/unset override state simulated in memory, then saved to project.godot once. " +
			"dry_run runs the same validation without saving. An all-no-op batch does not rewrite project.godot. " +
			"If saving fails, in-memory state and project.godot are restored on a best-effort basis; a rollback failure returns internal_error. " +
			"Accepts 1-100 operations. This batch is domain-local and makes no promise that settings and resource or Input Map changes happen together. " +
			"This is a direct project-file change, not an editor undo step.",
		Example:     `godot-cli project settings apply --operations '[{"operation":"set","setting":"display/window/size/viewport_width","value":1920},{"operation":"unset","setting":"application/config/name"}]'`,
		Annotations: map[string]string{skill.ActionKey: core.ProjectSettingsApply.Name},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			operations, err := parseOperations[settings.ApplyOperation](rawOperations)
			if err != nil {
				return err
			}

			return runProjectSettings(func(root string) (any, error) {
				return settings.Apply(root, settings.ApplyParams{
					Operations: operations,
					DryRun:     dryRun,
				})
			})
		},
	}
	cmd.Flags().StringVar(&rawOperations, "operations", "", "JSON array of ordered project-setting operations")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the full batch without saving project.godot")

	_ = cmd.MarkFlagRequired("operations")

	return cmd
}

func runProjectSettings(call func(root string) (any, error)) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}

	result, err := call(project.Root)
	if err != nil {
		return err
	}

	return PrintJSON(result)
}

func parseJSONValue(raw string) (json.RawMessage, error) {
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("value must be valid JSON: %s", raw)
	}

	return json.RawMessage(raw), nil
}
