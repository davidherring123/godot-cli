package cli

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/spf13/cobra"
)

func NewInitCommand() *cobra.Command {
	var agents []string

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Install godot-cli into the current project",
		Long:    "Install the Godot addon. With --agent, also install or refresh the portable skill and command reference. Prints setup instructions as text.",
		Example: "godot-cli init\ngodot-cli init --agent codex,opencode",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, agents)
		},
	}

	cmd.Flags().StringSliceVar(
		&agents,
		"agent",
		nil,
		"Install or refresh skills for an AI agent (codex, opencode)",
	)

	return cmd
}

func runInit(
	cmd *cobra.Command,
	agents []string,
) error {
	if err := skill.ValidateNames(agents); err != nil {
		return err
	}

	context, err := FetchProjectContext()
	if err != nil {
		return err
	}

	if err := installAddon(cmd, context.Root); err != nil {
		return err
	}

	return installSkill(cmd, context.Root, agents)
}

func installAddon(cmd *cobra.Command, projectRoot string) error {
	if addon.IsInstalled(projectRoot) {
		cmd.Println(
			"Godot CLI is already installed in this project.",
		)
		return nil
	}

	if err := addon.Install(projectRoot); err != nil {
		return fmt.Errorf("installing addon: %w", err)
	}

	cmd.Println(
		"✓ Installed addons/godot_cli\n\n" +
			"Enable the addon in Godot:\n" +
			"Project → Project Settings → Plugins → Godot CLI",
	)
	return nil
}

func installSkill(cmd *cobra.Command, projectRoot string, agents []string) error {
	if len(agents) == 0 {
		cmd.Println(
			"\nTo install an AI agent skill later, run:\n" +
				"  godot-cli init --agent codex\n" +
				"  godot-cli init --agent opencode",
		)
		return nil
	}

	path, err := skill.Install(projectRoot, cmd.Root())
	if err != nil {
		return fmt.Errorf("installing skill: %w", err)
	}
	cmd.Println("Installed skill:", path)
	return nil
}
