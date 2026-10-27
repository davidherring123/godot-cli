package cli

import (
	"github.com/davidherring123/godot-cli/internal/agent/integration"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/spf13/cobra"
)

func NewIntegrationsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Inspect and install AI client integrations",
		Long: "Manage project-scoped MCP configuration and portable skills for AI clients. " +
			"Both commands print JSON and resolve --project the same way as other godot-cli commands.",
	}

	cmd.AddCommand(
		newIntegrationsStatusCommand(),
		newIntegrationsInstallCommand(),
	)

	return cmd
}

func newIntegrationsStatusCommand() *cobra.Command {
	var projectPath string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show AI client integration status as JSON",
		Long: "Report each supported client's project configuration path, whether the godot MCP server is configured, " +
			"whether its command is current, and whether the project skill is installed.",
		Example: "godot-cli integrations status\ngodot-cli integrations status --project C:/Projects/MyGame",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveIntegrationProject(projectPath)
			if err != nil {
				return err
			}

			report, err := integration.Status(integration.Options{ProjectRoot: root})
			if err != nil {
				return err
			}

			return WriteJSON(cmd.OutOrStdout(), report)
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

func newIntegrationsInstallCommand() *cobra.Command {
	var projectPath string

	cmd := &cobra.Command{
		Use:       "install <opencode|claude|codex|all>",
		Short:     "Install project-scoped AI client integrations as JSON",
		Long:      "Merge the godot MCP server into the selected client's project configuration and install the portable godot-cli skill. Existing unrelated configuration is preserved.",
		Example:   "godot-cli integrations install opencode\ngodot-cli integrations install all",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"opencode", "claude", "codex", "all"},
		RunE: func(cmd *cobra.Command, args []string) error {
			clients, err := integration.ParseClients(args[0])
			if err != nil {
				return err
			}

			root, err := resolveIntegrationProject(projectPath)
			if err != nil {
				return err
			}

			report, err := integration.Install(
				integration.Options{ProjectRoot: root},
				clients,
			)
			if err != nil {
				return err
			}

			return WriteJSON(cmd.OutOrStdout(), report)
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

func resolveIntegrationProject(projectPath string) (string, error) {
	resolved, err := project.Resolve(projectPath)
	if err != nil {
		return "", err
	}

	return resolved.Root, nil
}
