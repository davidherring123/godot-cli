package cli

import (
	"errors"
	"time"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/validate"
	"github.com/spf13/cobra"
)

var errValidationFailed = errors.New("validation failed")

func NewValidateCommand() *cobra.Command {
	var (
		live    bool
		timeout time.Duration
		flags   editorSceneFlags
	)

	cmd := &cobra.Command{
		Use:         "validate [scene]",
		Short:       "Validate a scene by loading and running it in Godot",
		Long:        "Load a scene in a fresh headless Godot process, run it briefly, and report load, script, resource, and early runtime errors as JSON. Uses the configured main scene unless a scene path is supplied. Godot is asked to quit gracefully; the project is never opened in the editor, saved, or modified. The project must already be imported: resources without import data fail, as they would when running the game. Prints JSON with valid, scene, and errors, and exits nonzero when validation fails. With --live, validate the active editor scene instead, including unsaved state, through the running editor addon; nothing is saved.",
		Example:     "godot-cli validate\ngodot-cli validate res://levels/arena.tscn\ngodot-cli validate --live --expect-scene res://main.tscn",
		Annotations: map[string]string{skill.ActionKey: core.ValidateLive.Name},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if live && len(args) > 0 {
				return errors.New("a scene argument cannot be combined with --live")
			}

			if !live && flags.expectedScene != "" {
				return errors.New("--expect-scene requires --live")
			}

			return runValidate(args, live, timeout, flags.expectedScene)
		},
	}

	cmd.Flags().BoolVar(&live, "live", false, "Validate the active editor scene instead of the saved project")
	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Second, "How long the headless validation run may take before Godot is stopped")
	flags.bind(cmd)

	return cmd
}

func runValidate(
	args []string,
	live bool,
	timeout time.Duration,
	expectedScene string,
) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}

	if live {
		result, err := validate.Live(
			project.Root,
			validate.LiveParams{ExpectedScene: expectedScene},
		)
		if err != nil {
			return err
		}

		if err := PrintJSON(result); err != nil {
			return err
		}

		if !result.Valid {
			return errValidationFailed
		}

		return nil
	}

	scene := ""

	if len(args) > 0 {
		scene = args[0]
	}

	result, err := validate.Scene(project.Root, validate.SceneParams{
		Scene:          scene,
		TimeoutSeconds: timeout.Seconds(),
	})
	if err != nil {
		return err
	}

	if err := PrintJSON(result); err != nil {
		return err
	}

	if !result.Valid {
		return errValidationFailed
	}

	return nil
}
