package core_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPLiveSceneCurrent(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_current_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	mcpSession := connectMCP(t, session.Root(), []core.ActionDefinition{core.SceneCurrent})

	result, err := mcpSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "scene_current",
		Arguments: map[string]any{"expected_scene": path},
	})
	if err != nil {
		t.Fatalf("calling scene_current: %v", err)
	}

	if result.IsError {
		t.Fatalf("scene_current returned an error: %s", mcpText(result))
	}

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshaling structured content: %v", err)
	}

	if !strings.Contains(string(data), path) {
		t.Fatalf("scene_current result does not contain %s: %s", path, data)
	}
}

func TestMCPLiveErrorPreservesCode(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_current_conflict/main.tscn"

	openFixtureScene(t, session, path)

	mcpSession := connectMCP(t, session.Root(), []core.ActionDefinition{core.SceneCurrent})

	result, err := mcpSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "scene_current",
		Arguments: map[string]any{
			"expected_scene": "res://cases/different/main.tscn",
		},
	})
	if err != nil {
		t.Fatalf("calling scene_current: %v", err)
	}

	if !result.IsError {
		t.Fatal("scene_current returned success; want a conflict error")
	}

	if text := mcpText(result); !strings.Contains(text, `"code":"conflict"`) {
		t.Fatalf("error result does not contain the conflict code: %s", text)
	}
}

func connectMCP(
	t *testing.T,
	projectRoot string,
	actions []core.ActionDefinition,
) *mcp.ClientSession {
	t.Helper()

	server, err := mcpserver.NewServer(mcpserver.Options{
		Version:     "test",
		ProjectRoot: projectRoot,
		Actions:     actions,
	})
	if err != nil {
		t.Fatalf("creating MCP server: %v", err)
	}

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting MCP server: %v", err)
	}

	client := mcp.NewClient(
		&mcp.Implementation{Name: "godot-cli-test", Version: "test"},
		nil,
	)

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("connecting MCP client: %v", err)
	}

	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})

	return clientSession
}

func mcpText(result *mcp.CallToolResult) string {
	var text strings.Builder

	for _, content := range result.Content {
		if value, ok := content.(*mcp.TextContent); ok {
			text.WriteString(value.Text)
		}
	}

	return text.String()
}
