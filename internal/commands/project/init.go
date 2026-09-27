package project

import (
	"fmt"
	"path/filepath"

	"github.com/davidherring123/godot-cli/internal/addon"
	"github.com/davidherring123/godot-cli/internal/commands"
	"github.com/spf13/cobra"
)

func NewInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Install godot-cli into the current project",
		Args:  cobra.NoArgs,
		RunE:  runInit,
	}
}

func runInit(
	cmd *cobra.Command,
	args []string,
) error {
	context, err := commands.FetchProjectContext()
	if err != nil {
		return err
	}

	if addon.IsInstalled(context.Root) {
		fmt.Println(
			"Godot CLI is already installed in this project.",
		)
		return nil
	}

	projectName := filepath.Base(context.Root)

	confirmed, err := commands.Confirm(
		fmt.Sprintf(
			"Install Godot CLI addon into %s (%s)?",
			projectName,
			context.Root,
		),
	)
	if err != nil {
		return err
	}

	if !confirmed {
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

	return nil
}
