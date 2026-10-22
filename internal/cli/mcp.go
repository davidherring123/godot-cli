package cli

import (
	"github.com/davidherring123/godot-cli/internal/agent/mcp"
	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/spf13/cobra"
)

func NewMCPCommand(version string) *cobra.Command {
	var projectPath string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run a Model Context Protocol server over stdio",
		Long: "Expose the project's godot-cli actions as MCP tools over stdin/stdout. " +
			"The server targets one Godot project: pass --project, or start it from anywhere inside the project. " +
			"Live-editor tools require a running Godot 4 editor with the godot-cli addon installed. " +
			"stdout is reserved for MCP messages; diagnostics are written to stderr.",
		Example: "godot-cli mcp\ngodot-cli mcp --project C:/Projects/MyGame",
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			skill.IgnoreKey: "true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := project.Resolve(projectPath)
			if err != nil {
				return err
			}

			return mcp.Run(cmd.Context(), mcp.Options{
				Version:     version,
				ProjectRoot: resolved.Root,
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
