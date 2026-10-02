package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Godot project status",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
}

func runStatus(
	cmd *cobra.Command,
	args []string,
) error {
	context, err := FetchProjectContext()
	if err != nil {
		return err
	}

	fmt.Println("Godot project:", context.Root)

	return nil
}
