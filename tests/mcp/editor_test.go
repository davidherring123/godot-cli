package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidherring123/godot-cli/internal/agent/mcp"
	"github.com/davidherring123/godot-cli/internal/core"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerExposesEditorActions(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	server, err := mcp.NewServer(mcp.Options{
		Version:     "test",
		ProjectRoot: root,
		Actions:     []core.ActionDefinition{core.EditorStatus, core.ValidateScene},
	})
	if err != nil {
		t.Fatalf("creating MCP server: %v", err)
	}

	ctx := context.Background()
	serverTransport, clientTransport := mcpgo.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting MCP server: %v", err)
	}

	client := mcpgo.NewClient(
		&mcpgo.Implementation{Name: "godot-mcp-test", Version: "test"},
		nil,
	)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("connecting MCP client: %v", err)
	}

	t.Cleanup(func() {
		_ = session.Close()
		_ = serverSession.Close()
	})

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}

	found := make(map[string]bool, len(tools.Tools))

	for _, tool := range tools.Tools {
		found[tool.Name] = true
	}

	for _, name := range []string{"editor_status", "validate_scene"} {
		if !found[name] {
			t.Errorf("tool %q is not exposed", name)
		}
	}

	for _, name := range []string{"editor_open", "validate_live"} {
		if found[name] {
			t.Errorf("legacy tool %q is still exposed", name)
		}
	}

	result, err := session.CallTool(ctx, &mcpgo.CallToolParams{
		Name:      "editor_status",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("calling editor_status: %v", err)
	}

	if result.IsError {
		t.Fatalf("editor_status returned an error: %s", toolText(result))
	}

	if result.StructuredContent == nil {
		t.Fatal("editor_status returned no structured content")
	}
}

func toolText(result *mcpgo.CallToolResult) string {
	for _, content := range result.Content {
		if text, ok := content.(*mcpgo.TextContent); ok {
			return text.Text
		}
	}

	return ""
}
