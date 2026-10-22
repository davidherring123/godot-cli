package cli

import (
	"fmt"
	"slices"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/spf13/cobra"
)

func NewInitCommand() *cobra.Command {
	var agents []string

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Install godot-cli into the current project",
		Long:    "Install and enable the Godot addon. With --agent, also install or refresh the portable skill and command reference. For OpenCode, prints the command to register the MCP server; godot-cli does not modify OpenCode configuration.",
		Example: "godot-cli init\ngodot-cli init --agent codex\ngodot-cli init --agent opencode",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

	if err := installSkill(cmd, context.Root, agents); err != nil {
		return err
	}

	printOpenCodeMCPInstructions(cmd, agents)

	return nil
}

func installAddon(cmd *cobra.Command, projectRoot string) error {
	switch {
	case addon.IsInstalled(projectRoot) && !addon.IsManaged(projectRoot):
		cmd.Println("Godot CLI is already installed in this project.")
	case addon.IsInstalled(projectRoot):
		if err := addon.Install(projectRoot); err != nil {
			return fmt.Errorf("updating addon: %w", err)
		}

		cmd.Println("✓ Updated addons/godot_cli to match this godot-cli version.")
	default:
		if err := addon.Install(projectRoot); err != nil {
			return fmt.Errorf("installing addon: %w", err)
		}

		cmd.Println("✓ Installed addons/godot_cli")
	}

	changed, err := addon.Enable(projectRoot)
	if err != nil {
		return fmt.Errorf("enabling addon: %w", err)
	}

	if changed {
		cmd.Println("✓ Enabled the Godot CLI addon in project.godot. Restart the editor if it is already open.")
	} else {
		cmd.Println("Godot CLI addon is already enabled.")
	}

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

func printOpenCodeMCPInstructions(cmd *cobra.Command, agents []string) {
	if !slices.Contains(agents, "opencode") {
		return
	}

	cmd.Println(
		"\nTo expose this project to OpenCode, run:\n" +
			"  opencode mcp add godot -- godot-cli mcp\n\n" +
			"godot-cli does not modify OpenCode configuration.",
	)
}
