// Package mcpserver exposes godot-cli's core actions as a Model Context
// Protocol server.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/project"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options configures an MCP server instance.
type Options struct {
	Version      string
	ProjectRoot  string
	Actions      []core.ActionDefinition
	Instructions string
}

// ResolveProjectRoot finds the Godot project rooted at path, or at the current
// working directory when path is empty. A path may point to a project
// directory, any directory inside a project, or a project file.
func ResolveProjectRoot(path string) (string, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
		path = cwd
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving project path: %w", err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("reading project path %s: %w", absolute, err)
	}

	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}

	root, err := project.FindProjectRoot(absolute)
	if err != nil {
		return "", fmt.Errorf("locating Godot project: %w", err)
	}

	return root, nil
}

// NewServer creates an MCP server that exposes actions. When Actions is nil,
// all core actions are exposed.
func NewServer(options Options) (*mcp.Server, error) {
	actions := options.Actions
	if actions == nil {
		actions = core.All()
	}

	version := options.Version
	if version == "" {
		version = "dev"
	}

	instructions := options.Instructions
	if instructions == "" {
		instructions = defaultInstructions(options.ProjectRoot)
	}

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:        "godot-cli",
			Title:       "godot-cli",
			Version:     version,
			Description: "Inspect and edit a local Godot project through the godot-cli editor bridge.",
		},
		&mcp.ServerOptions{Instructions: instructions},
	)

	seen := make(map[string]string, len(actions))

	for _, action := range actions {
		toolName := core.ToolName(action.Name)

		if previous, ok := seen[toolName]; ok {
			return nil, fmt.Errorf(
				"actions %q and %q map to the same MCP tool %q",
				previous,
				action.Name,
				toolName,
			)
		}

		if err := addAction(server, options.ProjectRoot, action); err != nil {
			return nil, err
		}

		seen[toolName] = action.Name
	}

	return server, nil
}

// Run serves the MCP protocol over stdio until the client disconnects.
func Run(ctx context.Context, options Options) error {
	if options.ProjectRoot == "" {
		return errors.New("project root is required")
	}

	projectFile := filepath.Join(options.ProjectRoot, "project.godot")

	info, err := os.Stat(projectFile)
	if err != nil {
		return fmt.Errorf("reading project file %s: %w", projectFile, err)
	}

	if info.IsDir() {
		return fmt.Errorf("project file is a directory: %s", projectFile)
	}

	server, err := NewServer(options)
	if err != nil {
		return err
	}

	return server.Run(ctx, &mcp.StdioTransport{})
}

func defaultInstructions(projectRoot string) string {
	projectDescription := "the current Godot project"

	if projectRoot != "" {
		projectDescription = "the Godot project at " + projectRoot
	}

	return "This server inspects and edits " + projectDescription + ". " +
		"Live-editor tools require Godot 4 to be running with the godot-cli addon installed; " +
		"run `godot-cli init` once in the project if live tools report that no editor is reachable. " +
		"Mutating tools are undoable editor actions: set save=true once after a batch of edits, " +
		"not after every change. Tool results are JSON."
}
