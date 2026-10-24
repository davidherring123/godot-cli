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

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run a Model Context Protocol server over stdio",
		Long: "Expose the project's godot-cli actions as MCP tools over stdin/stdout. " +
			"The server targets one Godot project: pass --project, or start it from anywhere inside the project. " +
			"Live-editor tools require a running Godot 4 editor with the godot-cli addon installed. " +
			"stdout is reserved for MCP messages; diagnostics are written to stderr.",
		Example: "godot-cli mcp\n" +
			"godot-cli mcp --project C:/Projects/MyGame\n" +
			"godot-cli mcp --trace-file /tmp/godot-cli-trace.jsonl",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := project.Resolve(projectPath)
			if err != nil {
				return err
			}

			traceFile, err := openTraceFile(tracePath)
			if err != nil {
				return err
			}

			options := mcp.Options{
				Version:     version,
				ProjectRoot: resolved.Root,
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
