package commands

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/spf13/cobra"
)

func NewInitCommand() *cobra.Command {
	var agents []string
	
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install godot-cli into the current project",
		Args:  cobra.NoArgs,
		RunE:  runInit,
	}

	cmd.Flags().StringSliceVar(
		&agents,
		"agent",
		nil,
		"Install skills for an AI agent (codex, opencode)",
	)

	return cmd
}

func runInit(
	cmd *cobra.Command,
	agents []string,
) error {
	context, err := FetchProjectContext()
	if err != nil {
		return err
	}

	if addon.IsInstalled(context.Root) {
		fmt.Println(
			"Godot CLI is already installed in this project.",
		)
		return nil
	}

	if err := addon.Install(context.Root); err != nil {
		return fmt.Errorf(
			"installing addon: %w",
			err,
		)
	}

	fmt.Println(
		"✓ Installed addons/godot_cli\n\n" +
			"Enable the addon in Godot:\n" +
			"Project → Project Settings → Plugins → Godot CLI",
	)


	for _, agent := range agents {
		if err := agent.Install
	}

	return nil
}
