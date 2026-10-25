package core_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/davidherring123/godot-cli/internal/adapter/bridge"
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/node"
	"github.com/davidherring123/godot-cli/internal/core/validate"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestValidateLiveSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/validate_live_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := validate.Live(session.Root(), validate.LiveParams{})
	if err != nil {
		t.Fatalf("expected validate.live to succeed: %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected the active scene to be valid: %v", result.Errors)
	}

	if result.Scene != path {
		t.Fatalf("expected scene %s, got %s", path, result.Scene)
	}

	if result.Unsaved {
		t.Fatal("expected the freshly opened scene to have no unsaved changes")
	}

	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}

	if err := session.LogJSON(validate.LiveName, result); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLiveReportsUnsavedChanges(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/validate_live_unsaved/main.tscn"

	openFixtureScene(t, session, path)

	result, err := validate.Live(session.Root(), validate.LiveParams{})
	if err != nil {
		t.Fatalf("expected validate.live to succeed: %v", err)
	}

	if result.Unsaved {
		t.Fatal("expected the freshly opened scene to have no unsaved changes")
	}

	_, err = bridge.Call[node.SetParams, node.SetResult](
		session.Client,
		node.SetName,
		node.SetParams{
			Node: "Player",
			Properties: map[string]json.RawMessage{
				"position": json.RawMessage(`{"x":64,"y":96}`),
			},
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected node.set to succeed: %v", err)
	}

	result, err = validate.Live(
		session.Root(),
		validate.LiveParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected validate.live to succeed: %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected the edited scene to be valid: %v", result.Errors)
	}

	if !result.Unsaved {
		t.Fatal("expected unsaved to be true after an unsaved edit")
	}

	if result.Scene != path {
		t.Fatalf("expected scene %s, got %s", path, result.Scene)
	}

	if err := session.LogJSON(validate.LiveName, result); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLiveBrokenScriptFails(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/validate_live_broken/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.SetParams, node.SetResult](
		session.Client,
		node.SetName,
		node.SetParams{
			Node: "Player",
			Properties: map[string]json.RawMessage{
				"script": json.RawMessage(
					`{"resource":"res://cases/validate_live_broken/broken.gd"}`,
				),
			},
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected node.set to succeed: %v", err)
	}

	result, err := validate.Live(session.Root(), validate.LiveParams{})
	if err != nil {
		t.Fatalf("expected validate.live to succeed: %v", err)
	}

	if result.Valid {
		t.Fatal("expected a scene with a broken script to be invalid")
	}

	requireValidateErrorContains(t, result.Errors, "does not parse")
}

func TestValidateLiveExpectedSceneConflict(t *testing.T) {
	session := requireSuite(t)

	openFixtureScene(t, session, "res://cases/validate_live_succeeds/main.tscn")

	_, err := validate.Live(
		session.Root(),
		validate.LiveParams{
			ExpectedScene: "res://cases/validate_live_unsaved/main.tscn",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)
}

func TestValidateLiveWithoutSceneFails(t *testing.T) {
	session, err := harness.Start(nil)
	if errors.Is(err, harness.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("starting an isolated editor session: %v", err)
	}
	defer func() { _ = session.Close() }()

	_, err = validate.Live(session.Root(), validate.LiveParams{})

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)
}

const validateProject = `; Validate fixture project.
config_version=5

[application]

config/name="Validate Fixture"
run/main_scene="res://main.tscn"
`

const validateProjectWithoutMainScene = `; Validate fixture project.
config_version=5

[application]

config/name="Validate Fixture"
`

const validateMainScene = `[gd_scene load_steps=2 format=3]

[ext_resource type="Script" path="res://main.gd" id="1_main"]

[node name="Main" type="Node2D"]
script = ExtResource("1_main")
`

const validateMainScript = `extends Node2D

func _ready() -> void:
    print("MAIN READY")
`

func TestValidateSavedProjectSucceeds(t *testing.T) {
	root := writeValidateProject(t, map[string]string{
		"project.godot": validateProject,
		"main.tscn":     validateMainScene,
		"main.gd":       validateMainScript,
	})

	before := snapshotFiles(t, root)

	result, err := validate.Run(root, validate.RunParams{})
	if errors.Is(err, validate.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("expected validate to run: %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected a valid project, got errors: %v", result.Errors)
	}

	if result.Scene != "res://main.tscn" {
		t.Fatalf("expected the configured main scene, got %s", result.Scene)
	}

	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), `"errors":[]`) {
		t.Fatalf("expected errors to marshal as [], got %s", data)
	}

	after := snapshotFiles(t, root)

	if !reflect.DeepEqual(before, after) {
		t.Fatalf(
			"expected validate to leave the project unchanged:\n%s",
			diffSnapshots(before, after),
		)
	}

	t.Logf("validate.Run reported %+v", result)
}

func TestValidateSavedExplicitSceneSucceeds(t *testing.T) {
	root := writeValidateProject(t, map[string]string{
		"project.godot": validateProject,
		"main.tscn":     validateMainScene,
		"main.gd":       validateMainScript,
		"scenes/level.tscn": `[gd_scene format=3]

[node name="Level" type="Node2D"]
`,
	})

	for _, requested := range []string{
		"scenes/level.tscn",
		"res://scenes/level.tscn",
	} {
		t.Run(requested, func(t *testing.T) {
			result, err := validate.Run(root, validate.RunParams{Scene: requested})
			if errors.Is(err, validate.ErrGodotNotFound) {
				t.Skip(err)
			}
			if err != nil {
				t.Fatalf("expected validate to run: %v", err)
			}

			if !result.Valid {
				t.Fatalf("expected a valid scene, got errors: %v", result.Errors)
			}

			if result.Scene != "res://scenes/level.tscn" {
				t.Fatalf("expected res://scenes/level.tscn, got %s", result.Scene)
			}

			t.Logf("validate.Run reported %+v", result)
		})
	}
}

func TestValidateSavedMissingSceneFails(t *testing.T) {
	root := writeValidateProject(t, map[string]string{
		"project.godot": validateProject,
		"main.tscn":     validateMainScene,
		"main.gd":       validateMainScript,
	})

	result, err := validate.Run(root, validate.RunParams{Scene: "res://missing.tscn"})
	if errors.Is(err, validate.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("expected validate to run: %v", err)
	}

	if result.Valid {
		t.Fatal("expected a missing scene to be invalid")
	}

	requireValidateErrorContains(t, result.Errors, "missing.tscn")
}

func TestValidateSavedBrokenScriptFails(t *testing.T) {
	root := writeValidateProject(t, map[string]string{
		"project.godot": validateProject,
		"broken.gd": `extends Node2D

func _ready() -> void:
    this is not valid gdscript
`,
		"broken.tscn": `[gd_scene load_steps=2 format=3]

[ext_resource type="Script" path="res://broken.gd" id="1_broken"]

[node name="Broken" type="Node2D"]
script = ExtResource("1_broken")
`,
	})

	result, err := validate.Run(root, validate.RunParams{Scene: "res://broken.tscn"})
	if errors.Is(err, validate.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("expected validate to run: %v", err)
	}

	if result.Valid {
		t.Fatal("expected a scene with a broken script to be invalid")
	}

	requireValidateErrorContains(t, result.Errors, "Parse Error")
}

func TestValidateSavedRuntimeErrorFails(t *testing.T) {
	root := writeValidateProject(t, map[string]string{
		"project.godot": validateProject,
		"runtime.gd": `extends Node2D

func _ready() -> void:
    var node: Node = null
    node.get_child(0)
`,
		"runtime.tscn": `[gd_scene load_steps=2 format=3]

[ext_resource type="Script" path="res://runtime.gd" id="1_runtime"]

[node name="Runtime" type="Node2D"]
script = ExtResource("1_runtime")
`,
	})

	result, err := validate.Run(root, validate.RunParams{Scene: "res://runtime.tscn"})
	if errors.Is(err, validate.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("expected validate to run: %v", err)
	}

	if result.Valid {
		t.Fatal("expected a runtime error to be invalid")
	}

	requireValidateErrorContains(t, result.Errors, "SCRIPT ERROR")
}

func TestValidateSavedWithoutMainSceneFails(t *testing.T) {
	root := writeValidateProject(t, map[string]string{
		"project.godot": validateProjectWithoutMainScene,
	})

	_, err := validate.Run(root, validate.RunParams{})
	if errors.Is(err, validate.ErrGodotNotFound) {
		t.Skip(err)
	}

	if err == nil {
		t.Fatal("expected an error when no main scene is configured")
	}

	if !strings.Contains(err.Error(), "main scene") {
		t.Fatalf("expected a main scene error, got %v", err)
	}
}

func TestValidateSavedTimeoutFails(t *testing.T) {
	root := writeValidateProject(t, map[string]string{
		"project.godot": validateProject,
		"hang.gd": `extends Node2D

func _ready() -> void:
    while true:
        pass
`,
		"hang.tscn": `[gd_scene load_steps=2 format=3]

[ext_resource type="Script" path="res://hang.gd" id="1_hang"]

[node name="Hang" type="Node2D"]
script = ExtResource("1_hang")
`,
	})

	result, err := validate.Run(root, validate.RunParams{
		Scene:   "res://hang.tscn",
		Timeout: time.Second,
	})
	if errors.Is(err, validate.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("expected validate to run: %v", err)
	}

	if result.Valid {
		t.Fatal("expected a hanging scene to be invalid")
	}

	requireValidateErrorContains(t, result.Errors, "did not exit")
}

func writeValidateProject(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()

	for name, content := range files {
		path := filepath.Join(root, name)

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()

	snapshot := map[string]string{}

	err := filepath.WalkDir(
		root,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}

			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			snapshot[relative] = string(data)

			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	return snapshot
}

func diffSnapshots(before, after map[string]string) string {
	var report strings.Builder

	for name, content := range before {
		updated, found := after[name]

		if !found {
			report.WriteString("removed: " + name + "\n")
			continue
		}

		if updated != content {
			report.WriteString("modified: " + name + "\n")
		}
	}

	for name := range after {
		if _, found := before[name]; !found {
			report.WriteString("added: " + name + "\n")
		}
	}

	return report.String()
}

func requireValidateErrorContains(
	t *testing.T,
	errors []string,
	substring string,
) {
	t.Helper()

	for _, message := range errors {
		if strings.Contains(message, substring) {
			t.Logf("received expected validation error: %s", message)

			return
		}
	}

	t.Fatalf(
		"expected a validation error containing %q, got %v",
		substring,
		errors,
	)
}
