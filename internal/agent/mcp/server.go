// Package mcp exposes godot-cli's core actions as a Model Context
// Protocol server.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/davidherring123/godot-cli/internal/core"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// TransportStdio serves newline-delimited JSON-RPC over stdin/stdout.
	TransportStdio = "stdio"
	// TransportHTTP serves the MCP Streamable HTTP transport on loopback.
	TransportHTTP = "http"
)

// Options configures an MCP server instance.
type Options struct {
	Version      string
	ProjectRoot  string
	Actions      []core.ActionDefinition
	Instructions string
	// BenchmarkProfile selects a controlled benchmark-only tool surface. An
	// empty value preserves the production defaults.
	BenchmarkProfile string

	// Transport selects the serving transport. An empty value preserves the
	// original stdio behavior.
	Transport string
	// Host and Port configure the HTTP transport. Host must be a loopback
	// address; Port 0 asks the operating system for an available port.
	Host string
	Port int
	// StateFile, when set for HTTP, receives an atomically written
	// ServerState discovery document.
	StateFile string

	// TraceWriter, when non-nil, receives one newline-delimited JSON object
	// per completed tools/call. Tracing is disabled by default; it never
	// changes a tool result.
	TraceWriter io.Writer

	// TraceErrorWriter receives trace persistence diagnostics. When nil,
	// diagnostics go to stderr. It is only used when TraceWriter is set.
	TraceErrorWriter io.Writer
}

// NewServer creates an MCP server that exposes actions. When Actions is nil,
// all core actions are exposed.
func NewServer(options Options) (*mcpgo.Server, error) {
	if options.BenchmarkProfile != "" {
		profile, err := ResolveBenchmarkProfile(options.BenchmarkProfile, options.ProjectRoot)
		if err != nil {
			return nil, err
		}
		options.Actions = profile.Actions
		options.Instructions = profile.Instructions
	}

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
	tracer := newCallTracer(options.TraceWriter, options.TraceErrorWriter)

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

		if err := addAction(server, serializer, tracer, options.ProjectRoot, action); err != nil {
			return nil, err
		}

		seen[toolName] = action.Name
	}

	return server, nil
}

// Run serves the MCP protocol over the transport selected by options.Transport
// until the context is cancelled or the client disconnects. The empty transport
// preserves the original stdio behavior.
func Run(ctx context.Context, options Options) error {
	switch options.Transport {
	case "", TransportStdio:
		return RunStdio(ctx, options)
	case TransportHTTP:
		return RunHTTP(ctx, options)
	default:
		return fmt.Errorf(
			"unsupported MCP transport %q; use %q or %q",
			options.Transport,
			TransportStdio,
			TransportHTTP,
		)
	}
}

// RunStdio serves the MCP protocol over stdin/stdout until the client
// disconnects. Diagnostics are written to stderr so stdout stays reserved for
// protocol messages.
func RunStdio(ctx context.Context, options Options) error {
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

	return "This server inspects and edits " + projectDescription + ". It is managed by a running Godot 4 editor; " +
		"validate_scene checks the active editor scene, including unsaved changes, without saving or switching scenes.\n\n" +
		"Prefer these tools for supported semantic inspection and mutation of scenes, nodes, resources, " +
		"project settings, and the Input Map. Do not parse or rewrite .tscn, .tres, or project.godot files " +
		"by hand when a tool covers the change. Use file tools for GDScript and shader source, " +
		"repository-wide searches, line-numbered reads, and raw serialization details the tools do not expose. " +
		"Use Git for history, diffs, and blame.\n\n" +
		"Active-scene tools read the live editor's current scene, including unsaved changes; file reads see " +
		"only saved contents. scene_current reports unsaved state, and scene_save or save=true writes the scene. " +
		"Node mutations are undoable; set save=true once after related edits. Prefer scene_apply for multiple " +
		"ordered edits: it accepts up to 100 operations, supports best-effort dry_run and @id references, and " +
		"commits as one undo step. scene_create writes a new scene file immediately.\n\n" +
		"Resource, project-setting, and Input Map mutations write their files immediately and are not editor undo " +
		"steps. Prefer resource_apply, project_settings_apply, and input_map_apply for ordered domain-local batches " +
		"with dry-run and best-effort rollback. A mutation response reports the values that were applied and, " +
		"for active-scene edits, whether the scene was saved; it does not confirm gameplay or runtime behavior. " +
		"Use validate_scene for script and serialization checks and runtime tools for gameplay. Do not " +
		"inspect again after a successful mutation unless the response omitted values you need or you are " +
		"answering a different question.\n\n" +
		"Use the narrowest tool directly when the target is known; skip redundant status and discovery calls. " +
		"Runtime tools start a separate game process: use runtime_start, runtime_status, runtime_capture, and " +
		"runtime_stop for frames, errors, logs, and windowed PNG captures. Live tools require a running Godot 4 " +
		"editor with the godot-cli addon. If no editor is running, open this project in Godot and retry. " +
		"Tool results are JSON unless image content is explicitly available."
}
