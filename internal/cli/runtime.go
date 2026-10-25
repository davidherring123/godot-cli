package cli

import (
	"time"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/project/runtime"
	"github.com/spf13/cobra"
)

func NewRuntimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Run and inspect the project's game process",
	}

	cmd.AddCommand(
		newRuntimeStartCommand(),
		newRuntimeStatusCommand(),
		newRuntimeCaptureCommand(),
		newRuntimeStopCommand(),
	)

	return cmd
}

func newRuntimeStartCommand() *cobra.Command {
	var (
		headless bool
		timeout  time.Duration
	)

	cmd := &cobra.Command{
		Use:   "start [scene]",
		Short: core.RuntimeStart.Description,
		Long: "Start the project's game process in a detached Godot runtime and wait until its scene is ready. " +
			"Uses the configured main scene unless a scene path is supplied. Runtime state, logs, and screenshots live under .godot/godot-cli/runtime. " +
			"Headless sessions report errors and frames but cannot capture screenshots. Only one runtime session may be active per project.",
		Example: "godot-cli runtime start\ngodot-cli runtime start res://levels/arena.tscn\ngodot-cli runtime start --headless",
		Args:    cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			skill.ActionKey: core.RuntimeStart.Name,
		},
		RunE: func(_ *cobra.Command, args []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			scene := ""

			if len(args) > 0 {
				scene = args[0]
			}

			result, err := runtime.Start(project.Root, runtime.StartParams{
				Scene:          scene,
				Headless:       headless,
				TimeoutSeconds: timeout.Seconds(),
			})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}

	cmd.Flags().BoolVar(&headless, "headless", false, "Run without a window; screenshots are unavailable in this mode")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "How long to wait for the runtime scene to become ready")

	return cmd
}

func newRuntimeStatusCommand() *cobra.Command {
	var session string

	cmd := &cobra.Command{
		Use:   "status",
		Short: core.RuntimeStatus.Description,
		Long: "Report the active runtime session, or the most recent one when no game process is running. " +
			"Includes lifecycle state, pid, scene, frame count, exit information, runtime error lines, log paths, and the last screenshot artifact.",
		Example: "godot-cli runtime status\ngodot-cli runtime status --session 20261023T150405Z-a1b2c3",
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			skill.ActionKey: core.RuntimeStatus.Name,
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := runtime.Status(project.Root, runtime.StatusParams{Session: session})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}

	cmd.Flags().StringVar(&session, "session", "", "Runtime session ID; defaults to the active or most recent session")

	return cmd
}

func newRuntimeCaptureCommand() *cobra.Command {
	var (
		session string
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "capture",
		Short: core.RuntimeCapture.Description,
		Long: "Ask the running game to save a PNG screenshot under .godot/godot-cli/runtime and print its project-relative res:// path, size, and frame. " +
			"Requires a windowed runtime; headless sessions return an unsupported error. MCP calls also receive the image as image content.",
		Example: "godot-cli runtime capture\ngodot-cli runtime capture --timeout 15s",
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			skill.ActionKey: core.RuntimeCapture.Name,
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := runtime.Capture(project.Root, runtime.CaptureParams{
				Session:        session,
				TimeoutSeconds: timeout.Seconds(),
			})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}

	cmd.Flags().StringVar(&session, "session", "", "Runtime session ID; defaults to the active session")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "How long to wait for the screenshot")

	return cmd
}

func newRuntimeStopCommand() *cobra.Command {
	var (
		session string
		timeout time.Duration
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "stop",
		Short: core.RuntimeStop.Description,
		Long: "Ask the runtime to quit and wait for the game process to exit, killing it if it does not stop within the timeout. " +
			"Unresponsive sessions require --force so a reused or unrelated process is never killed implicitly. Stopping when no runtime is active succeeds with already_stopped=true.",
		Example: "godot-cli runtime stop\ngodot-cli runtime stop --timeout 10s\ngodot-cli runtime stop --force",
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			skill.ActionKey: core.RuntimeStop.Name,
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			project, err := FetchProjectContext()
			if err != nil {
				return err
			}

			result, err := runtime.Stop(project.Root, runtime.StopParams{
				Session:        session,
				TimeoutSeconds: timeout.Seconds(),
				Force:          force,
			})
			if err != nil {
				return err
			}

			return PrintJSON(result)
		},
	}

	cmd.Flags().StringVar(&session, "session", "", "Runtime session ID; defaults to the active session")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "How long to wait for a graceful exit before killing the process")
	cmd.Flags().BoolVar(&force, "force", false, "Kill a session whose heartbeat has stopped responding")

	return cmd
}
