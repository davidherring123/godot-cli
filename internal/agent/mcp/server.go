// Package mcp exposes godot-cli's core actions as a Model Context
// Protocol server.
package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/davidherring123/godot-cli/internal/core"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options configures an MCP server instance.
type Options struct {
	Version      string
	ProjectRoot  string
	Actions      []core.ActionDefinition
	Instructions string
}

// NewServer creates an MCP server that exposes actions. When Actions is nil,
// all core actions are exposed.
func NewServer(options Options) (*mcpgo.Server, error) {
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

	server := mcpgo.NewServer(
		&mcpgo.Implementation{
			Name:        "godot-cli",
			Title:       "godot-cli",
			Version:     version,
			Description: "Inspect and edit a local Godot project through the godot-cli editor bridge.",
		},
		&mcpgo.ServerOptions{Instructions: instructions},
	)

	seen := make(map[string]string, len(actions))
	serializer := newCallSerializer()

	for _, action := range actions {
		toolName := ToolName(action.Name)

		if previous, ok := seen[toolName]; ok {
			return nil, fmt.Errorf(
				"actions %q and %q map to the same MCP tool %q",
				previous,
				action.Name,
				toolName,
			)
		}

		if err := addAction(server, serializer, options.ProjectRoot, action); err != nil {
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

	server, err := NewServer(options)
	if err != nil {
		return err
	}

	return server.Run(ctx, &mcpgo.StdioTransport{})
}

func defaultInstructions(projectRoot string) string {
	projectDescription := "the current Godot project"

	if projectRoot != "" {
		projectDescription = "the Godot project at " + projectRoot
	}

	return "This server inspects and edits " + projectDescription + ". " +
		"Live-editor tools require Godot 4 to be running with the godot-cli addon installed; " +
		"start it with the editor_open tool, and run `godot-cli init` once in the project to install and enable the addon. " +
		"Runtime tools start the game itself and need no editor: runtime_start waits until the scene is ready, " +
		"runtime_status reports state, frames, errors, logs, and the last capture, runtime_capture returns a PNG " +
		"artifact and MCP image content, and runtime_stop ends the session. Capture requires a windowed runtime; " +
		"headless runtime sessions report errors and frames but return unsupported for capture. " +
		"Scene and node edits are undoable: set save=true once after a batch of edits, not after every change. " +
		"Resource, project-setting, and Input Map tools save immediately and have no save argument. " +
		"Use resource_apply, project_settings_apply, and input_map_apply for ordered domain batches of up to 100 operations; " +
		"each supports dry_run, reports per-operation diagnostics, and attempts best-effort rollback within its own domain on failure. " +
		"Rollback failures return internal_error because the editor may be partially changed. " +
		"Batches are never atomic across domains: apply resource, project-setting, and Input Map batches separately. " +
		"Tool results are JSON unless an image content block is explicitly available."
}
