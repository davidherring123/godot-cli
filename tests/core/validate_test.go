package core_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/adapter/bridge"
	"github.com/davidherring123/godot-cli/internal/core/node"
	"github.com/davidherring123/godot-cli/internal/core/validation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestValidateSceneSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/validate_live_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := validate.Scene(session.Root(), validate.SceneParams{})
	if err != nil {
		t.Fatalf("expected validate.scene to succeed: %v", err)
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

	if err := session.LogJSON(validate.SceneName, result); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSceneReportsUnsavedChanges(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/validate_live_unsaved/main.tscn"

	openFixtureScene(t, session, path)

	result, err := validate.Scene(session.Root(), validate.SceneParams{})
	if err != nil {
		t.Fatalf("expected validate.scene to succeed: %v", err)
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

	result, err = validate.Scene(
		session.Root(),
		validate.SceneParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected validate.scene to succeed: %v", err)
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

	if err := session.LogJSON(validate.SceneName, result); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSceneBrokenScriptFails(t *testing.T) {
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

	result, err := validate.Scene(session.Root(), validate.SceneParams{})
	if err != nil {
		t.Fatalf("expected validate.scene to succeed: %v", err)
	}

	if result.Valid {
		t.Fatal("expected a scene with a broken script to be invalid")
	}

	requireValidateErrorContains(t, result.Errors, "does not parse")
}

func TestValidateSceneExpectedSceneConflict(t *testing.T) {
	session := requireSuite(t)

	openFixtureScene(t, session, "res://cases/validate_live_succeeds/main.tscn")

	_, err := validate.Scene(
		session.Root(),
		validate.SceneParams{
			ExpectedScene: "res://cases/validate_live_unsaved/main.tscn",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)
}

func TestValidateSceneWithoutSceneFails(t *testing.T) {
	session, err := harness.Start(nil)
	if errors.Is(err, harness.ErrGodotNotFound) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("starting an isolated editor session: %v", err)
	}
	defer func() { _ = session.Close() }()

	_, err = validate.Scene(session.Root(), validate.SceneParams{})

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)
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
