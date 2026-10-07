package core_test

import (
	"errors"
	"testing"

	"github.com/davidherring123/godot-cli/internal/bridge"
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

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
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

	assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)
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
	expected bridge.ErrorCode,
) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %s error", expected)
	}

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) {
		t.Fatalf("expected API error, got %v", err)
	}

	if apiError.Code != expected {
		t.Fatalf("expected %s error, got %s", expected, apiError.Code)
	}

	t.Logf("received expected error: %v", apiError)
}
