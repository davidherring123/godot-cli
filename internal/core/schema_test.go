package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var mcpToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func TestActionsAreWellFormed(t *testing.T) {
	names := make(map[string]bool)
	toolNames := make(map[string]string)

	for _, action := range All() {
		if action.Name == "" {
			t.Fatal("action has an empty name")
		}

		if names[action.Name] {
			t.Fatalf("duplicate action name %q", action.Name)
		}

		names[action.Name] = true

		toolName := ToolName(action.Name)

		if previous, ok := toolNames[toolName]; ok {
			t.Fatalf(
				"actions %q and %q map to the same MCP tool %q",
				previous,
				action.Name,
				toolName,
			)
		}

		toolNames[toolName] = action.Name

		if !mcpToolNamePattern.MatchString(toolName) {
			t.Errorf("action %q maps to invalid MCP tool name %q", action.Name, toolName)
		}

		if action.Description == "" {
			t.Errorf("action %q has no description", action.Name)
		}

		if action.Invoke == nil {
			t.Errorf("action %q has no invocation function", action.Name)
		}

		if action.ReadOnly && action.Destructive {
			t.Errorf("read-only action %q is marked destructive", action.Name)
		}

		assertObjectSchema(t, action.Name+" input", action.Input)
		assertObjectSchema(t, action.Name+" output", action.Output)
	}
}

func TestActionsIncludeSceneValidation(t *testing.T) {
	action, ok := Lookup("validate.scene")
	if !ok {
		t.Fatal("validate.scene is not registered")
	}

	if !action.ReadOnly || action.Destructive {
		t.Fatalf(
			"validate.scene hints = read-only %v, destructive %v; want read-only",
			action.ReadOnly,
			action.Destructive,
		)
	}
}

func TestActionInvokeDecodesArguments(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "levels"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "levels", "arena.tscn"),
		[]byte("[gd_scene format=3]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	result, err := SceneList.Invoke(root, nil)
	if err != nil {
		t.Fatalf("invoking scene.list: %v", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "res://levels/arena.tscn") {
		t.Fatalf("scene.list result does not contain the scene: %s", data)
	}
}

func TestActionInvokeRejectsInvalidArguments(t *testing.T) {
	for _, input := range []string{
		`{"node": 12}`,
		`{"node": ".", "unknown": true}`,
		`[]`,
	} {
		_, err := NodeInspect.Invoke(t.TempDir(), []byte(input))

		var argumentError *ArgumentError
		if !errors.As(err, &argumentError) {
			t.Fatalf("input %s returned %v; want ArgumentError", input, err)
		}

		if argumentError.Action != NodeInspect.Name {
			t.Errorf(
				"argument error action = %q; want %q",
				argumentError.Action,
				NodeInspect.Name,
			)
		}
	}
}

func TestToolName(t *testing.T) {
	for action, expected := range map[string]string{
		"scene.list":           "scene_list",
		"project.settings.get": "project_settings_get",
		"input.event_add":      "input_event_add",
		"validate.live":        "validate_live",
	} {
		if actual := ToolName(action); actual != expected {
			t.Errorf("ToolName(%q) = %q; want %q", action, actual, expected)
		}
	}
}

func assertObjectSchema(t *testing.T, label string, schema any) {
	t.Helper()

	if schema == nil {
		t.Errorf("%s schema is nil", label)
		return
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Errorf("marshaling %s schema: %v", label, err)
		return
	}

	var document struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &document); err != nil {
		t.Errorf("reading %s schema: %v", label, err)
		return
	}

	if document.Type != "object" {
		t.Errorf("%s schema type = %q; want object", label, document.Type)
	}
}
