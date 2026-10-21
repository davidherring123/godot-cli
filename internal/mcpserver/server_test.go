package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerExposesAllCoreActions(t *testing.T) {
	root := testProject(t)
	session := newTestSession(t, root, nil)

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}

	if len(result.Tools) != len(core.All()) {
		t.Fatalf("tool count = %d; want %d", len(result.Tools), len(core.All()))
	}

	tools := make(map[string]*mcp.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}

	for _, action := range core.All() {
		name := core.ToolName(action.Name)
		tool, ok := tools[name]
		if !ok {
			t.Errorf("tool %q is missing", name)
			continue
		}

		if tool.Description != action.Description {
			t.Errorf("tool %q description = %q; want %q", name, tool.Description, action.Description)
		}

		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Errorf("tool %q input schema has type %T; want object schema", name, tool.InputSchema)
			continue
		}

		if schema["type"] != "object" {
			t.Errorf("tool %q input schema type = %v; want object", name, schema["type"])
		}
	}

	if _, ok := tools["validate_scene"]; !ok {
		t.Error("validate_scene is not exposed")
	}
}

func TestServerCallsSceneList(t *testing.T) {
	root := testProject(t)
	session := newTestSession(t, root, []core.ActionDefinition{core.SceneList})

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "scene_list",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("calling scene_list: %v", err)
	}

	if result.IsError {
		t.Fatalf("scene_list returned an error: %s", toolText(result))
	}

	if result.StructuredContent == nil {
		t.Fatal("scene_list returned no structured content")
	}

	if !strings.Contains(toolText(result), "res://main.tscn") {
		t.Fatalf("scene_list result does not contain main.tscn: %s", toolText(result))
	}
}

func TestServerMapsAPIErrors(t *testing.T) {
	root := testProject(t)
	session := newTestSession(t, root, []core.ActionDefinition{core.NodeInspect})

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "node_inspect",
		Arguments: map[string]any{"node": ""},
	})
	if err != nil {
		t.Fatalf("calling node_inspect: %v", err)
	}

	if !result.IsError {
		t.Fatal("node_inspect returned success; want a tool error")
	}

	text := toolText(result)

	if !strings.Contains(text, `"code":"invalid_argument"`) {
		t.Errorf("error result does not contain invalid_argument: %s", text)
	}

	if !strings.Contains(text, "A node path is required") {
		t.Errorf("error result does not contain the bridge message: %s", text)
	}
}

func TestServerMapsArgumentErrors(t *testing.T) {
	root := testProject(t)
	session := newTestSession(t, root, []core.ActionDefinition{core.NodeInspect})

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "node_inspect",
		Arguments: map[string]any{"node": 12},
	})
	if err != nil {
		t.Fatalf("calling node_inspect: %v", err)
	}

	if !result.IsError {
		t.Fatal("node_inspect returned success; want a tool error")
	}

	text := toolText(result)

	if !strings.Contains(text, `"code":"invalid_argument"`) {
		t.Errorf("error result does not contain invalid_argument: %s", text)
	}

	if !strings.Contains(text, "invalid arguments") {
		t.Errorf("error result does not describe invalid arguments: %s", text)
	}
}

func TestServerRejectsDuplicateToolNames(t *testing.T) {
	_, err := NewServer(Options{
		ProjectRoot: testProject(t),
		Actions:     []core.ActionDefinition{core.SceneList, core.SceneList},
	})
	if err == nil {
		t.Fatal("NewServer succeeded with duplicate tools; want an error")
	}
}

func TestResolveProjectRoot(t *testing.T) {
	root := testProject(t)
	nested := filepath.Join(root, "levels", "forest")

	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
	}{
		{name: "project directory", path: root},
		{name: "nested directory", path: nested},
		{name: "project file", path: filepath.Join(root, "project.godot")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := ResolveProjectRoot(test.path)
			if err != nil {
				t.Fatalf("resolving project root: %v", err)
			}

			if actual != root {
				t.Fatalf("project root = %q; want %q", actual, root)
			}
		})
	}
}

func TestResolveProjectRootFromWorkingDirectory(t *testing.T) {
	root := testProject(t)
	nested := filepath.Join(root, "levels")

	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(nested)

	actual, err := ResolveProjectRoot("")
	if err != nil {
		t.Fatalf("resolving project root: %v", err)
	}

	if actual != root {
		t.Fatalf("project root = %q; want %q", actual, root)
	}
}

func TestResolveProjectRootMissingPath(t *testing.T) {
	_, err := ResolveProjectRoot(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("ResolveProjectRoot succeeded for a missing path; want an error")
	}
}

func testProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "main.tscn"),
		[]byte("[gd_scene format=3]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	return root
}

func newTestSession(t *testing.T, root string, actions []core.ActionDefinition) *mcp.ClientSession {
	t.Helper()

	server, err := NewServer(Options{
		Version:     "test",
		ProjectRoot: root,
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

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("connecting MCP client: %v", err)
	}

	t.Cleanup(func() {
		_ = session.Close()
		_ = serverSession.Close()
	})

	return session
}

func toolText(result *mcp.CallToolResult) string {
	var text strings.Builder

	for _, content := range result.Content {
		if value, ok := content.(*mcp.TextContent); ok {
			text.WriteString(value.Text)
		}
	}

	return text.String()
}
