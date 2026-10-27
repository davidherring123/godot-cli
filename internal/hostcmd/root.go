// Package hostcmd implements the private command surface of the bundled
// godot-mcp-host process. The editor addon owns its lifecycle; users interact
// with MCP tools and AI client integrations instead of terminal editor
// commands.
package hostcmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/davidherring123/godot-cli/internal/agent/integration"
	"github.com/davidherring123/godot-cli/internal/agent/mcp"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/spf13/cobra"
)

const (
	commandName = "godot-mcp-host"
	defaultHost = "127.0.0.1"
)

// NewRootCommand creates the complete host command surface: the editor-managed
// HTTP endpoint, the stable stdio client entrypoint, integration management,
// and version reporting.
func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Private MCP host managed by the godot-cli editor addon",
		Long: "Serve the godot-cli MCP tools for a single Godot project. " +
			"This executable is bundled with the addon: the editor starts and stops the HTTP endpoint, " +
			"and AI clients launch the stable stdio entrypoint written into their project configuration.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	cmd.SetVersionTemplate(commandName + " {{.Version}}\n")
	cmd.AddCommand(
		NewServeCommand(version),
		NewConnectCommand(version),
		NewIntegrationsCommand(),
		NewVersionCommand(version),
	)

	return cmd
}

// NewServeCommand serves the editor-managed Streamable HTTP MCP endpoint. It is
// loopback-only and is not a general-purpose terminal server.
func NewServeCommand(version string) *cobra.Command {
	var projectPath string
	var host string
	var port int
	var stateFile string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the editor-managed MCP endpoint on loopback",
		Long: "Serve the godot-cli MCP tools over Streamable HTTP for the Godot editor addon. " +
			"The endpoint is always loopback-only; the addon passes a state file so it can discover " +
			"the bound URL and process ID. Use connect for a stdio client configuration.",
		Example: commandName + " serve --project C:/Projects/MyGame --state-file .godot/godot-cli/mcp/backend-1234.json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := project.Resolve(projectPath)
			if err != nil {
				return err
			}

			return mcp.RunHTTP(cmd.Context(), mcp.Options{
				Version:     version,
				ProjectRoot: resolved.Root,
				Host:        host,
				Port:        port,
				StateFile:   stateFile,
			})
		},
	}

	cmd.Flags().StringVar(
		&projectPath,
		"project",
		"",
		"Godot project directory or any path inside it; defaults to the current directory",
	)
	cmd.Flags().StringVar(
		&host,
		"host",
		defaultHost,
		"Loopback host for the MCP HTTP endpoint",
	)
	cmd.Flags().IntVar(
		&port,
		"port",
		0,
		"Port for the MCP HTTP endpoint; 0 chooses an available port",
	)
	cmd.Flags().StringVar(
		&stateFile,
		"state-file",
		"",
		"Write an atomic MCP server discovery JSON file after binding",
	)

	return cmd
}

// NewConnectCommand serves the stable stdio entrypoint written into client
// configuration.
func NewConnectCommand(version string) *cobra.Command {
	var projectPath string

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Serve the MCP server over stdio for one Godot project",
		Long: "Serve the godot-cli MCP tools over stdio for one Godot project. " +
			"This is the stable entrypoint written into AI client configuration. " +
			"Pass --project, or start it from anywhere inside the project. " +
			"Live-editor tools require a running Godot 4 editor with the godot-cli addon installed. " +
			"stdout is reserved for MCP messages; diagnostics are written to stderr.",
		Example: commandName + " connect --project C:/Projects/MyGame",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := project.Resolve(projectPath)
			if err != nil {
				return err
			}

			return mcp.RunStdio(cmd.Context(), mcp.Options{
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

// NewIntegrationsCommand exposes the project-scoped client integration status
// and install operations used by the editor panel.
func NewIntegrationsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Inspect and install AI client integrations",
		Long: "Manage project-scoped MCP configuration and portable skills for AI clients. " +
			"Both commands print one JSON object and resolve --project the same way as other host commands.",
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
		Example: commandName + " integrations status --project C:/Projects/MyGame",
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

			return writeJSON(cmd.OutOrStdout(), report)
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
		Example:   commandName + " integrations install opencode --project C:/Projects/MyGame",
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

			return writeJSON(cmd.OutOrStdout(), report)
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

// NewVersionCommand prints the host version used by the addon and clients.
func NewVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the host version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", commandName, version)
			return err
		},
	}
}

func resolveIntegrationProject(projectPath string) (string, error) {
	resolved, err := project.Resolve(projectPath)
	if err != nil {
		return "", err
	}

	return resolved.Root, nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	return encoder.Encode(value)
}
