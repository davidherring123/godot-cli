package cli

import (
	"encoding/json"
	"fmt"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/input"
	"github.com/spf13/cobra"
)

func NewInputCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Inspect and edit the Input Map",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newInputListCommand(),
		newInputInspectCommand(),
		newInputAddCommand(),
		newInputSetCommand(),
		newInputRenameCommand(),
		newInputDeleteCommand(),
		newInputEventCommand(),
		newInputApplyCommand(),
	)

	return cmd
}

func newInputListCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List Input Map actions",
		Long:        "List every Input Map action with its deadzone and bindings in stored order. Requires a running editor with the addon enabled.",
		Example:     "godot-cli input list",
		Annotations: map[string]string{skill.ActionKey: "input.list"},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runInput(func(root string) (any, error) {
				return input.List(root, input.ListParams{})
			})
		},
	}
}

func newInputInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "inspect <action>",
		Short:       "Show an Input Map action",
		Long:        "Show one Input Map action's deadzone and bindings in stored order. Fails for unknown actions. Requires a running editor with the addon enabled.",
		Example:     "godot-cli input inspect jump",
		Annotations: map[string]string{skill.ActionKey: "input.inspect"},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runInput(func(root string) (any, error) {
				return input.Inspect(root, input.InspectParams{Action: args[0]})
			})
		},
	}
}

func newInputAddCommand() *cobra.Command {
	var deadzone float64
	cmd := &cobra.Command{
		Use:         "add <action>",
		Short:       "Add an Input Map action",
		Long:        "Add an empty Input Map action and save project.godot immediately. Fails if the action already exists. --deadzone defaults to Godot's 0.5 and must be between 0 and 1. Add bindings with input event add. This is a direct project-file change, not an editor undo step. Requires a running editor with the addon enabled.",
		Example:     "godot-cli input add jump --deadzone 0.2",
		Annotations: map[string]string{skill.ActionKey: "input.add"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var requested *float64

			if cmd.Flags().Changed("deadzone") {
				value := deadzone
				requested = &value
			}

			return runInput(func(root string) (any, error) {
				return input.Add(root, input.AddParams{
					Action:   args[0],
					Deadzone: requested,
				})
			})
		},
	}
	cmd.Flags().Float64Var(&deadzone, "deadzone", 0.5, "Analog deadzone between 0 and 1")
	return cmd
}

func newInputSetCommand() *cobra.Command {
	var deadzone float64
	cmd := &cobra.Command{
		Use:         "set <action>",
		Short:       "Set an Input Map action's deadzone",
		Long:        "Set an existing Input Map action's analog deadzone and save project.godot immediately. Fails for unknown actions. Bindings are unchanged. This is a direct project-file change, not an editor undo step. Requires a running editor with the addon enabled.",
		Example:     "godot-cli input set jump --deadzone 0.3",
		Annotations: map[string]string{skill.ActionKey: "input.set"},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runInput(func(root string) (any, error) {
				return input.Set(root, input.SetParams{
					Action:   args[0],
					Deadzone: deadzone,
				})
			})
		},
	}
	cmd.Flags().Float64Var(&deadzone, "deadzone", 0.5, "Analog deadzone between 0 and 1")

	_ = cmd.MarkFlagRequired("deadzone")

	return cmd
}

func newInputRenameCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "rename <action> <new-name>",
		Short:       "Rename an Input Map action",
		Long:        "Rename an Input Map action, keeping its deadzone and bindings, and save project.godot immediately. Fails if the action is missing or the new name already exists. This is a direct project-file change, not an editor undo step. Requires a running editor with the addon enabled.",
		Example:     "godot-cli input rename jump leap",
		Annotations: map[string]string{skill.ActionKey: "input.rename"},
		Args:        cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runInput(func(root string) (any, error) {
				return input.Rename(root, input.RenameParams{
					Action:  args[0],
					NewName: args[1],
				})
			})
		},
	}
}

func newInputDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "delete <action>",
		Short:       "Delete an Input Map action",
		Long:        "Delete an Input Map action and its bindings, and save project.godot immediately. Fails for unknown actions. This is a direct project-file change and cannot be undone. Requires a running editor with the addon enabled.",
		Example:     "godot-cli input delete jump",
		Annotations: map[string]string{skill.ActionKey: "input.delete"},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runInput(func(root string) (any, error) {
				return input.Delete(root, input.DeleteParams{Action: args[0]})
			})
		},
	}
}

func newInputEventCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Manage Input Map bindings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newInputEventAddCommand(),
		newInputEventDeleteCommand(),
	)

	return cmd
}

func newInputEventAddCommand() *cobra.Command {
	var rawEvent string
	cmd := &cobra.Command{
		Use:         "add <action>",
		Short:       "Add a binding to an Input Map action",
		Long:        "Add a binding to an existing Input Map action and save project.godot immediately. --event is a JSON object; the type field selects key (keycode, physical_keycode, shift, ctrl, alt, meta, echo), mouse_button (button_index), joypad_button (button_index), or joypad_motion (axis, axis_value). An optional device index defaults to -1 for all devices. This is a direct project-file change, not an editor undo step. Requires a running editor with the addon enabled.",
		Example:     `godot-cli input event add jump --event '{"type":"key","physical_keycode":32}'`,
		Annotations: map[string]string{skill.ActionKey: "input.event_add"},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var event input.Event

			if err := json.Unmarshal([]byte(rawEvent), &event); err != nil {
				return fmt.Errorf("event must be a JSON object: %w", err)
			}

			return runInput(func(root string) (any, error) {
				return input.EventAdd(root, input.EventAddParams{
					Action: args[0],
					Event:  event,
				})
			})
		},
	}
	cmd.Flags().StringVar(&rawEvent, "event", "", "JSON object describing the binding")

	_ = cmd.MarkFlagRequired("event")

	return cmd
}

func newInputEventDeleteCommand() *cobra.Command {
	var index int
	cmd := &cobra.Command{
		Use:         "delete <action>",
		Short:       "Remove a binding from an Input Map action",
		Long:        "Remove the binding at a zero-based --index from an existing Input Map action and save project.godot immediately. Use input inspect to see binding indexes. This is a direct project-file change, not an editor undo step. Requires a running editor with the addon enabled.",
		Example:     "godot-cli input event delete jump --index 0",
		Annotations: map[string]string{skill.ActionKey: "input.event_delete"},
		Args:        cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runInput(func(root string) (any, error) {
				return input.EventDelete(root, input.EventDeleteParams{
					Action: args[0],
					Index:  index,
				})
			})
		},
	}
	cmd.Flags().IntVar(&index, "index", -1, "Zero-based position of the binding to remove")

	_ = cmd.MarkFlagRequired("index")

	return cmd
}

func newInputApplyCommand() *cobra.Command {
	var rawOperations string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: core.InputMapApply.Description,
		Long: "Apply an ordered batch of Input Map action and binding operations: add, set, rename, delete, event_add, and event_delete. " +
			"Every operation is validated in order against the editor's Input Map, then applied in memory and saved to project.godot once. " +
			"dry_run runs the same validation without saving and without reloading the Input Map. An all-no-op batch does not rewrite project.godot. " +
			"If saving fails, in-memory state and project.godot are restored on a best-effort basis; a rollback failure returns internal_error. " +
			"Accepts 1-100 operations. Binding indexes refer to the state after earlier operations in the same batch. " +
			"This batch is domain-local and makes no promise that Input Map, resource, or project-setting changes happen together. " +
			"This is a direct project-file change, not an editor undo step.",
		Example:     `godot-cli input apply --operations '[{"operation":"add","action":"jump","deadzone":0.2},{"operation":"event_add","action":"jump","event":{"type":"key","physical_keycode":32}},{"operation":"set","action":"jump","deadzone":0.3}]'`,
		Annotations: map[string]string{skill.ActionKey: core.InputMapApply.Name},
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			operations, err := parseOperations[input.ApplyOperation](rawOperations)
			if err != nil {
				return err
			}

			return runInput(func(root string) (any, error) {
				return input.Apply(root, input.ApplyParams{
					Operations: operations,
					DryRun:     dryRun,
				})
			})
		},
	}
	cmd.Flags().StringVar(&rawOperations, "operations", "", "JSON array of ordered Input Map operations")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the full batch without saving project.godot")

	_ = cmd.MarkFlagRequired("operations")

	return cmd
}

func runInput(call func(root string) (any, error)) error {
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
