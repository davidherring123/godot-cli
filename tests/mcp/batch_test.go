package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerExposesDomainBatchTools(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	session := connectMCP(t, root, []core.ActionDefinition{
		core.ResourceApply,
		core.ProjectSettingsApply,
		core.InputMapApply,
	})

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}

	found := make(map[string]bool, len(tools.Tools))

	for _, tool := range tools.Tools {
		found[tool.Name] = true
	}

	for _, name := range []string{
		"resource_apply",
		"project_settings_apply",
		"input_map_apply",
	} {
		if !found[name] {
			t.Errorf("tool %q is not exposed", name)
		}
	}
}

func TestMCPLiveDomainBatchDryRunAndErrors(t *testing.T) {
	session := startLiveSession(t)
	name := "mcp_test_batch"

	mcpSession := connectMCP(t, session.Root(), []core.ActionDefinition{
		core.InputMapApply,
		core.InputInspect,
		core.ProjectSettingsApply,
	})

	result, err := mcpSession.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name: "input_map_apply",
		Arguments: map[string]any{
			"dry_run": true,
			"operations": []any{
				map[string]any{
					"operation": "add",
					"action":    name,
					"deadzone":  0.2,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("calling input_map_apply: %v", err)
	}

	if result.IsError {
		t.Fatalf("input_map_apply returned an error: %s", mcpText(result))
	}

	for _, expected := range []string{`"applied": false`, `"operation_count": 1`, `"status": "added"`} {
		if !strings.Contains(mcpText(result), expected) {
			t.Fatalf("expected %q in %s", expected, mcpText(result))
		}
	}

	result, err = mcpSession.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name:      "input_inspect",
		Arguments: map[string]any{"action": name},
	})
	if err != nil {
		t.Fatalf("calling input_inspect: %v", err)
	}

	if !result.IsError {
		t.Fatal("dry run created the action")
	}

	result, err = mcpSession.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name: "project_settings_apply",
		Arguments: map[string]any{
			"operations": []any{
				map[string]any{
					"operation": "set",
					"setting":   "godot_cli/mcp_missing",
					"value":     1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("calling project_settings_apply: %v", err)
	}

	if !result.IsError {
		t.Fatal("project_settings_apply returned success for an unknown setting")
	}

	text := mcpText(result)

	for _, expected := range []string{`"code":"not_found"`, "operation 0"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in %s", expected, text)
		}
	}
}
