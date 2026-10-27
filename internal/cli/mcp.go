package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidherring123/godot-cli/internal/agent/mcp"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/spf13/cobra"
)

func NewMCPCommand(version string) *cobra.Command {
	var projectPath string
	var tracePath string
	var benchmarkProfile string
	var transport string
	var host string
	var port int
	var stateFile string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run a Model Context Protocol server over stdio or Streamable HTTP",
		Long: "Expose the project's godot-cli actions as MCP tools. " +
			"The default transport is stdio over stdin/stdout; use --transport http for the MCP Streamable HTTP transport at /mcp. " +
			"HTTP only binds a loopback address, writes an optional discovery state file after binding, " +
			"and removes that file on shutdown when this process still owns it. " +
			"The server targets one Godot project: pass --project, or start it from anywhere inside the project. " +
			"Live-editor tools require a running Godot 4 editor with the godot-cli addon installed. " +
			"For stdio, stdout is reserved for MCP messages; diagnostics are written to stderr.",
		Example: "godot-cli mcp\n" +
			"godot-cli mcp --project C:/Projects/MyGame\n" +
			"godot-cli mcp --transport http --port 8765 --state-file .godot/godot-cli/mcp.json\n" +
			"godot-cli mcp --trace-file /tmp/godot-cli-trace.jsonl",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if transport != mcp.TransportStdio && transport != mcp.TransportHTTP {
				return fmt.Errorf(
					"invalid MCP transport %q; use %q or %q",
					transport,
					mcp.TransportStdio,
					mcp.TransportHTTP,
				)
			}

			resolved, err := project.Resolve(projectPath)
			if err != nil {
				return err
			}

			traceFile, err := openTraceFile(tracePath)
			if err != nil {
				return err
			}

			options := mcp.Options{
				Version:          version,
				ProjectRoot:      resolved.Root,
				BenchmarkProfile: benchmarkProfile,
				Transport:        transport,
				Host:             host,
				Port:             port,
				StateFile:        stateFile,
			}

			if traceFile != nil {
				options.TraceWriter = traceFile
			}

			runErr := mcp.Run(cmd.Context(), options)

			if traceFile == nil {
				return runErr
			}

			// Report delayed write errors instead of losing them with the file
			// handle. They do not change the result of any tool call.
			if err := traceFile.Sync(); err != nil {
				runErr = errors.Join(runErr, fmt.Errorf("syncing trace file: %w", err))
			}

			if err := traceFile.Close(); err != nil {
				runErr = errors.Join(runErr, fmt.Errorf("closing trace file: %w", err))
			}

			return runErr
		},
	}

	cmd.Flags().StringVar(
		&projectPath,
		"project",
		"",
		"Godot project directory or any path inside it; defaults to the current directory",
	)

	cmd.Flags().StringVar(
		&tracePath,
		"trace-file",
		"",
		"Append one JSON object per completed MCP tool call to this JSONL file; disabled by default",
	)
	cmd.Flags().StringVar(
		&transport,
		"transport",
		mcp.TransportStdio,
		"MCP transport: stdio or http",
	)
	cmd.Flags().StringVar(
		&host,
		"host",
		"127.0.0.1",
		"Loopback host for --transport http",
	)
	cmd.Flags().IntVar(
		&port,
		"port",
		0,
		"Port for --transport http; 0 chooses an available port",
	)
	cmd.Flags().StringVar(
		&stateFile,
		"state-file",
		"",
		"Write an atomic MCP server discovery JSON file for --transport http; disabled by default",
	)
	cmd.Flags().StringVar(
		&benchmarkProfile,
		"benchmark-profile",
		"",
		"Select a controlled MCP benchmark surface",
	)
	_ = cmd.Flags().MarkHidden("benchmark-profile")

	return cmd
}

func openTraceFile(path string) (*os.File, error) {
	if path == "" {
		return nil, nil
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving trace file %q: %w", path, err)
	}

	file, err := os.OpenFile(absolute, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening trace file %q: %w", absolute, err)
	}

	return file, nil
}
