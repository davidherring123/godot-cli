package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPLiveSceneApply(t *testing.T) {
	session := startLiveSession(t)
	path := "res://cases/scene_apply_batch/main.tscn"

	if _, err := scene.Open(session.Root(), scene.OpenParams{Scene: path}); err != nil {
		t.Fatalf("opening fixture scene: %v", err)
	}

	mcpSession := connectMCP(t, session.Root(), []core.ActionDefinition{core.SceneApply})

	result, err := mcpSession.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name: "scene_apply",
		Arguments: map[string]any{
			"expected_scene": path,
			"save":           true,
			"operations": []map[string]any{
				{
					"op":     "node.add",
					"id":     "marker",
					"type":   "Marker2D",
					"parent": "World",
					"name":   "BatchMarker",
					"properties": map[string]any{
						"position": map[string]any{"x": 12, "y": 34},
					},
				},
				{
					"op":   "node.set",
					"node": "@marker",
					"properties": map[string]any{
						"position": map[string]any{"x": 56, "y": 78},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("calling scene_apply: %v", err)
	}

	if result.IsError {
		t.Fatalf("scene_apply returned an error: %s", mcpText(result))
	}

	var applied scene.ApplyResult

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshaling structured content: %v", err)
	}

	if err := json.Unmarshal(data, &applied); err != nil {
		t.Fatalf("decoding structured content: %v", err)
	}

	if !applied.Applied || applied.DryRun {
		t.Fatalf("unexpected applied state: %+v", applied)
	}

	if !applied.Saved {
		t.Fatal("expected scene_apply to report a save")
	}

	if len(applied.Operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(applied.Operations))
	}

	if applied.Operations[0].Path != "World/BatchMarker" ||
		applied.Operations[1].Path != "World/BatchMarker" {
		t.Fatalf("unexpected operation paths: %+v", applied.Operations)
	}

	if applied.Summary.Total != 2 || applied.Summary.Changed != 2 || applied.Summary.Added != 1 {
		t.Fatalf("unexpected summary: %+v", applied.Summary)
	}
}

func TestMCPLiveSceneApplyReportsFailedOperation(t *testing.T) {
	session := startLiveSession(t)
	path := "res://cases/scene_apply_rollback/main.tscn"

	if _, err := scene.Open(session.Root(), scene.OpenParams{Scene: path}); err != nil {
		t.Fatalf("opening fixture scene: %v", err)
	}

	mcpSession := connectMCP(t, session.Root(), []core.ActionDefinition{core.SceneApply})

	result, err := mcpSession.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name: "scene_apply",
		Arguments: map[string]any{
			"expected_scene": path,
			"operations": []map[string]any{
				{
					"op":     "node.add",
					"id":     "temp",
					"type":   "Marker2D",
					"parent": "World",
					"name":   "Temp",
				},
				{
					"op":   "node.set",
					"node": "@temp",
					"properties": map[string]any{
						"definitely_missing": true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("calling scene_apply: %v", err)
	}

	if !result.IsError {
		t.Fatal("scene_apply returned success; want a failed operation")
	}

	text := mcpText(result)

	for _, fragment := range []string{
		`"code":"not_found"`,
		"operation[1]",
		"node.set",
		"definitely_missing",
		"Prevalidation failed",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("error result does not contain %q: %s", fragment, text)
		}
	}
}
