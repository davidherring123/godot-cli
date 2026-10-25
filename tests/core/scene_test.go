package core_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/adapter/bridge"
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestSceneListSucceeds(t *testing.T) {
	session := requireSuite(t)

	result, err := scene.List(session.Root(), scene.ListParams{})
	if err != nil {
		t.Fatalf("expected scene.list to succeed: %v", err)
	}

	if len(result.Scenes) == 0 {
		t.Fatal("expected scene.list to find scenes")
	}

	if err := session.LogJSON(scene.ListName, result); err != nil {
		t.Fatal(err)
	}
}

func TestSceneOpenSucceeds(t *testing.T) {
	session := requireSuite(t)

	openFixtureScene(
		t,
		session,
		"res://cases/scene_open_succeeds/main.tscn",
	)

	t.Log("scene.open completed successfully")
}

func TestSceneOpenMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[scene.OpenParams, scene.OpenResult](
		session.Client,
		scene.OpenName,
		scene.OpenParams{Scene: "res://cases/missing/main.tscn"},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestSceneCurrentSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_current_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[scene.CurrentParams, scene.CurrentResult](
		session.Client,
		scene.CurrentName,
		scene.CurrentParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected scene.current to succeed: %v", err)
	}

	if err := session.LogJSON(scene.CurrentName, result); err != nil {
		t.Fatal(err)
	}
}

func TestSceneCurrentConflict(t *testing.T) {
	session := requireSuite(t)

	openFixtureScene(
		t,
		session,
		"res://cases/scene_current_conflict/main.tscn",
	)

	_, err := bridge.Call[scene.CurrentParams, scene.CurrentResult](
		session.Client,
		scene.CurrentName,
		scene.CurrentParams{
			ExpectedScene: "res://cases/different/main.tscn",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)
}

func TestSceneTreeSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_tree_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[scene.TreeParams, scene.TreeResult](
		session.Client,
		scene.TreeName,
		scene.TreeParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected scene.tree to succeed: %v", err)
	}

	if err := session.LogJSON(scene.TreeName, result); err != nil {
		t.Fatal(err)
	}
}

func TestSceneSaveSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_save_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[scene.SaveParams, scene.SaveResult](
		session.Client,
		scene.SaveName,
		scene.SaveParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected scene.save to succeed: %v", err)
	}

	if err := session.LogJSON(scene.SaveName, result); err != nil {
		t.Fatal(err)
	}
}

func TestSceneCreateSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_create_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	created := "res://cases/scene_create_succeeds/generated/arena.tscn"

	result, err := bridge.Call[scene.CreateParams, scene.CreateResult](
		session.Client,
		scene.CreateName,
		scene.CreateParams{
			Scene:    created,
			RootType: "Node2D",
			Name:     "Arena",
			Properties: map[string]json.RawMessage{
				"position": json.RawMessage(`{"x":128,"y":256}`),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected scene.create to succeed: %v", err)
	}

	if result.Scene != created {
		t.Fatalf("expected scene %s, got %s", created, result.Scene)
	}

	if result.Name != "Arena" {
		t.Fatalf("expected root name Arena, got %s", result.Name)
	}

	if result.Type != "Node2D" {
		t.Fatalf("expected root type Node2D, got %s", result.Type)
	}

	if result.Unsaved {
		t.Fatal("expected the created scene to be saved")
	}

	var position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}

	if err := json.Unmarshal(result.Properties["position"], &position); err != nil {
		t.Fatalf("decoding position property: %v", err)
	}

	if position.X != 128 || position.Y != 256 {
		t.Fatalf("expected position (128, 256), got %+v", position)
	}

	savedText := readSavedScene(t, session, created)

	if !strings.Contains(savedText, `[node name="Arena" type="Node2D"`) {
		t.Fatalf("expected the created root in the saved scene:\n%s", savedText)
	}

	if _, err := bridge.Call[scene.CurrentParams, scene.CurrentResult](
		session.Client,
		scene.CurrentName,
		scene.CurrentParams{ExpectedScene: created},
	); err != nil {
		t.Fatalf("expected the created scene to be active: %v", err)
	}

	if err := session.LogJSON(scene.CreateName, result); err != nil {
		t.Fatal(err)
	}
}

func TestSceneCreateWithScriptSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_create_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	created := "res://cases/scene_create_succeeds/generated/player.tscn"

	result, err := bridge.Call[scene.CreateParams, scene.CreateResult](
		session.Client,
		scene.CreateName,
		scene.CreateParams{
			Scene:    created,
			RootType: "CharacterBody2D",
			Name:     "Player",
			Properties: map[string]json.RawMessage{
				"script": json.RawMessage(`{"resource":"res://scripts/player.gd"}`),
				"speed":  json.RawMessage(`250`),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected scene.create to succeed: %v", err)
	}

	if result.Type != "CharacterBody2D" {
		t.Fatalf("expected root type CharacterBody2D, got %s", result.Type)
	}

	assertResourceProperty(
		t,
		result.Properties,
		"script",
		"res://scripts/player.gd",
		"GDScript",
	)

	var speed float64

	if err := json.Unmarshal(result.Properties["speed"], &speed); err != nil {
		t.Fatalf("decoding speed property: %v", err)
	}

	if speed != 250 {
		t.Fatalf("expected speed 250, got %v", speed)
	}
}

func TestSceneCreateExistingFileConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_create_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[scene.CreateParams, scene.CreateResult](
		session.Client,
		scene.CreateName,
		scene.CreateParams{
			Scene:    path,
			RootType: "Node2D",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)
}

func TestSceneCreateUnknownTypeRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_create_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[scene.CreateParams, scene.CreateResult](
		session.Client,
		scene.CreateName,
		scene.CreateParams{
			Scene:    "res://cases/scene_create_succeeds/generated/unknown.tscn",
			RootType: "NotANodeType",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)
}

func TestSceneCreateInvalidExtensionRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_create_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[scene.CreateParams, scene.CreateResult](
		session.Client,
		scene.CreateName,
		scene.CreateParams{
			Scene:    "res://cases/scene_create_succeeds/generated/notes.txt",
			RootType: "Node2D",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)
}

func TestSceneListEmptyReturnsEmptyArray(t *testing.T) {
	root := t.TempDir()

	result, err := scene.List(root, scene.ListParams{})
	if err != nil {
		t.Fatalf("expected scene.list to succeed: %v", err)
	}

	if result.Scenes == nil {
		t.Fatal("expected an empty slice, not nil")
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != `{"scenes":[]}` {
		t.Fatalf("expected scenes to marshal as [], got %s", data)
	}
}

func TestSceneCurrentWithNoSceneSucceeds(t *testing.T) {
	session, err := harness.Start(nil)
	if errors.Is(err, harness.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("starting an isolated editor session: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := bridge.Call[scene.CurrentParams, scene.CurrentResult](
		session.Client,
		scene.CurrentName,
		scene.CurrentParams{},
	)
	if err != nil {
		t.Fatalf("expected scene.current to succeed with no scene: %v", err)
	}

	if result.Active {
		t.Fatal("expected active to be false when no scene is being edited")
	}

	if result.Scene != "" {
		t.Fatalf("expected an empty scene path, got %s", result.Scene)
	}

	_, err = bridge.Call[scene.CurrentParams, scene.CurrentResult](
		session.Client,
		scene.CurrentName,
		scene.CurrentParams{ExpectedScene: "res://main.tscn"},
	)

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)

	if err := session.LogJSON(scene.CurrentName, result); err != nil {
		t.Fatal(err)
	}
}

func openFixtureScene(
	t *testing.T,
	session *harness.Session,
	path string,
) scene.OpenResult {
	t.Helper()

	result, err := bridge.Call[scene.OpenParams, scene.OpenResult](
		session.Client,
		scene.OpenName,
		scene.OpenParams{Scene: path},
	)
	if err != nil {
		t.Fatalf("opening fixture scene %s: %v", path, err)
	}

	if err := session.LogJSON(scene.OpenName, result); err != nil {
		t.Fatal(err)
	}

	return result
}

func assertAPIErrorCode(
	t *testing.T,
	err error,
	expected apierr.Code,
) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %s error", expected)
	}

	var apiError *apierr.Error

	if !errors.As(err, &apiError) {
		t.Fatalf("expected API error, got %v", err)
	}

	if apiError.Code != expected {
		t.Fatalf("expected %s error, got %s", expected, apiError.Code)
	}

	t.Logf("received expected error: %v", apiError)
}
