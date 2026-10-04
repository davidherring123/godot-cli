package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show Godot project status",
		Long:    "Print the discovered Godot project root as text. Does not check whether the editor or addon is running.",
		Example: "godot-cli status",
		Args:    cobra.NoArgs,
		RunE:    runStatus,
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
