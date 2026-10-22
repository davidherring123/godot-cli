package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/agent/mcp"
	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/davidherring123/godot-cli/tests/harness"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPLiveSceneCurrent(t *testing.T) {
	session := startLiveSession(t)
	path := "res://cases/scene_current_succeeds/main.tscn"

	if _, err := scene.Open(session.Root(), scene.OpenParams{Scene: path}); err != nil {
		t.Fatalf("opening fixture scene: %v", err)
	}

	mcpSession := connectMCP(t, session.Root(), []core.ActionDefinition{core.SceneCurrent})

	result, err := mcpSession.CallTool(context.Background(), &mcpgo.CallToolParams{
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
	session := startLiveSession(t)
	path := "res://cases/scene_current_conflict/main.tscn"

	if _, err := scene.Open(session.Root(), scene.OpenParams{Scene: path}); err != nil {
		t.Fatalf("opening fixture scene: %v", err)
	}

	mcpSession := connectMCP(t, session.Root(), []core.ActionDefinition{core.SceneCurrent})

	result, err := mcpSession.CallTool(context.Background(), &mcpgo.CallToolParams{
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

func startLiveSession(t *testing.T) *harness.Session {
	t.Helper()

	session, err := harness.Start(t.Logf)
	if errors.Is(err, harness.ErrGodotNotFound) {
		t.Skip(err)
	}

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("closing MCP test session: %v", err)
		}
	})

	return session
}

func connectMCP(
	t *testing.T,
	projectRoot string,
	actions []core.ActionDefinition,
) *mcpgo.ClientSession {
	t.Helper()

	server, err := mcp.NewServer(mcp.Options{
		Version:     "test",
		ProjectRoot: projectRoot,
		Actions:     actions,
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
		&mcpgo.Implementation{Name: "godot-cli-test", Version: "test"},
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

func mcpText(result *mcpgo.CallToolResult) string {
	var text strings.Builder

	for _, content := range result.Content {
		if value, ok := content.(*mcpgo.TextContent); ok {
			text.WriteString(value.Text)
		}
	}

	return text.String()
}
