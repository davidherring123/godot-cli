package core_test

import (
	"testing"

	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/core/scene"
)

func TestSceneOpenSucceeds(t *testing.T) {
	session := requireSuite(t)

	result, err := bridge.Call[scene.OpenParams, scene.OpenResult](
		session.Client,
		scene.OpenName,
		scene.OpenParams{
			Scene: "res://cases/scene_open_succeeds/main.tscn",
		},
	)
	if err != nil {
		t.Fatalf("expected scene.open to succeed: %v", err)
	}

	if err := session.LogJSON(scene.OpenName, result); err != nil {
		t.Fatal(err)
	}

	t.Log("scene.open completed successfully")
}
