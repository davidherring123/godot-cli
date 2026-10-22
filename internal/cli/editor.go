package cli

import (
	"time"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/editor"
	"github.com/spf13/cobra"
)

func NewEditorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "editor",
		Short: "Open and inspect the Godot editor",
	}

	cmd.AddCommand(
		newEditorOpenCommand(),
		newEditorStatusCommand(),
	)

	return cmd
}

func newEditorOpenCommand() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the Godot editor and wait for the godot-cli addon",
		Long: "Start the Godot editor for the current project and wait for the godot-cli addon bridge to register. " +
			"Prints JSON with opened, already_running, pid, port, and version. " +
			"Returns the existing instance when an editor is already running. " +
			"Requires the addon to be installed and enabled; run `godot-cli init` first.",
		Example: "godot-cli editor open\ngodot-cli editor open --timeout 60s",
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			skill.ActionKey: core.EditorOpen.Name,
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := editor.Open(project.Root, editor.OpenParams{
				TimeoutSeconds: timeout.Seconds(),
			})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "How long to wait for the editor bridge to start")

	return cmd
}

func newEditorStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether a Godot editor is running for this project",
		Long: "Report whether a Godot editor with the godot-cli addon is registered for the current project. " +
			"Prints JSON with running, pid, port, and version. Does not start the editor.",
		Example: "godot-cli editor status",
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			skill.ActionKey: core.EditorStatus.Name,
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := editor.Status(project.Root, editor.StatusParams{})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}
}
