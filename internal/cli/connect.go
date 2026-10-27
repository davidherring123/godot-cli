package cli

import (
	"github.com/davidherring123/godot-cli/internal/agent/mcp"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/spf13/cobra"
)

func NewConnectCommand(version string) *cobra.Command {
	var projectPath string

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Serve the MCP server over stdio for one Godot project",
		Long: "Serve the existing godot-cli MCP tools over stdio for one Godot project. " +
			"This is the stable entrypoint written into AI client configuration. " +
			"Pass --project, or start it from anywhere inside the project. " +
			"Live-editor tools require a running Godot 4 editor with the godot-cli addon installed. " +
			"stdout is reserved for MCP messages; diagnostics are written to stderr.",
		Example: "godot-cli connect --project C:/Projects/MyGame",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := project.Resolve(projectPath)
			if err != nil {
				return err
			}

			return mcp.Run(cmd.Context(), mcp.Options{
				Version:     version,
				ProjectRoot: resolved.Root,
				Transport:   mcp.TransportStdio,
			})
		},
	}

	cmd.Flags().StringVar(
		&projectPath,
		"project",
		"",
		"Godot project directory or any path inside it; defaults to the current directory",
	)

	return cmd
}
