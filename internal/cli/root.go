package cli

import "github.com/spf13/cobra"

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "godot-cli",
		Short:         "CLI tooling for Godot projects",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.SetVersionTemplate("godot-cli {{.Version}}\n")
	cmd.AddCommand(
		NewInitCommand(),
		NewStatusCommand(),
		NewSceneCommand(),
		NewNodeCommand(),
	)

	return cmd
}
