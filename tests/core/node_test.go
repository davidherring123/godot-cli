package core_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/core/node"
)

func TestNodeInspectSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_inspect_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          "Player",
			Properties:    []string{"position"},
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected node.inspect to succeed: %v", err)
	}

	if err := session.LogJSON(node.InspectName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeInspectMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_inspect_missing/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          "Missing",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func TestNodeSetSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_set_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.SetParams, node.SetResult](
		session.Client,
		node.SetName,
		node.SetParams{
			Node:          "Player",
			ExpectedScene: path,
			Properties: map[string]json.RawMessage{
				"position": json.RawMessage(`{"x":64,"y":96}`),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected node.set to succeed: %v", err)
	}

	if !result.Changed {
		t.Fatal("expected node.set to report a change")
	}

	if err := session.LogJSON(node.SetName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeSetMissingPropertyReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_set_missing_property/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.SetParams, node.SetResult](
		session.Client,
		node.SetName,
		node.SetParams{
			Node:          "Player",
			ExpectedScene: path,
			Properties: map[string]json.RawMessage{
				"missing_property": json.RawMessage(`true`),
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func TestNodeSetResourceSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_set_resource/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.SetParams, node.SetResult](
		session.Client,
		node.SetName,
		node.SetParams{
			Node:          "Sprite",
			ExpectedScene: path,
			Properties: map[string]json.RawMessage{
				"texture": json.RawMessage(`{"resource":"res://assets/placeholder_texture.tres"}`),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected node.set to succeed: %v", err)
	}

	if !result.Changed {
		t.Fatal("expected node.set to report a change")
	}

	var texture map[string]any

	if err := json.Unmarshal(result.Properties["texture"], &texture); err != nil {
		t.Fatalf("decoding texture property: %v", err)
	}

	if texture["resource"] != "res://assets/placeholder_texture.tres" {
		t.Fatalf("expected texture resource path, got %v", texture["resource"])
	}

	if texture["type"] != "PlaceholderTexture2D" {
		t.Fatalf("expected PlaceholderTexture2D, got %v", texture["type"])
	}

	if err := session.LogJSON(node.SetName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeAddSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:          "Marker2D",
			Parent:        "World",
			Name:          "Spawn",
			ExpectedScene: path,
			Properties: map[string]json.RawMessage{
				"position": json.RawMessage(`{"x":32,"y":48}`),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected node.add to succeed: %v", err)
	}

	if result.Path != "World/Spawn" {
		t.Fatalf("expected path World/Spawn, got %s", result.Path)
	}

	if result.Name != "Spawn" {
		t.Fatalf("expected name Spawn, got %s", result.Name)
	}

	if result.Type != "Marker2D" {
		t.Fatalf("expected type Marker2D, got %s", result.Type)
	}

	var position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}

	if err := json.Unmarshal(result.Properties["position"], &position); err != nil {
		t.Fatalf("decoding position property: %v", err)
	}

	if position.X != 32 || position.Y != 48 {
		t.Fatalf("expected position (32, 48), got %+v", position)
	}

	if err := session.LogJSON(node.AddName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeAddGeneratedNameIsUnique(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_unique_name/main.tscn"

	openFixtureScene(t, session, path)

	first, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:          "Marker2D",
			Parent:        ".",
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected first node.add to succeed: %v", err)
	}

	second, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:          "Marker2D",
			Parent:        ".",
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected second node.add to succeed: %v", err)
	}

	if first.Name != "Marker2D" {
		t.Fatalf("expected generated name Marker2D, got %s", first.Name)
	}

	if second.Name == first.Name || !strings.HasPrefix(second.Name, "Marker2D") {
		t.Fatalf(
			"expected a unique name based on Marker2D, got %q",
			second.Name,
		)
	}
}

func TestNodeAddDuplicateNameConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_duplicate_name/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:          "Marker2D",
			Parent:        ".",
			Name:          "World",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)
}

func TestNodeAddMissingParentReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_missing_parent/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:          "Marker2D",
			Parent:        "Missing",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func TestNodeAddUnknownTypeRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_missing_parent/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:          "NotANodeType",
			Parent:        ".",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)
}

func TestNodeAddWithScriptResource(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_resource_script/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:   "CharacterBody2D",
			Parent: ".",
			Name:   "Player",
			Properties: map[string]json.RawMessage{
				"script": json.RawMessage(`{"resource":"res://scripts/player.gd"}`),
				"speed":  json.RawMessage(`250`),
			},
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected node.add to succeed: %v", err)
	}

	if result.Type != "CharacterBody2D" {
		t.Fatalf("expected type CharacterBody2D, got %s", result.Type)
	}

	var script map[string]any

	if err := json.Unmarshal(result.Properties["script"], &script); err != nil {
		t.Fatalf("decoding script property: %v", err)
	}

	if script["resource"] != "res://scripts/player.gd" {
		t.Fatalf("expected script resource res://scripts/player.gd, got %v", script["resource"])
	}

	if script["type"] != "GDScript" {
		t.Fatalf("expected script type GDScript, got %v", script["type"])
	}

	var speed float64

	if err := json.Unmarshal(result.Properties["speed"], &speed); err != nil {
		t.Fatalf("decoding speed property: %v", err)
	}

	if speed != 250 {
		t.Fatalf("expected speed 250, got %v", speed)
	}

	if err := session.LogJSON(node.AddName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeAddMissingResourceReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_missing_resource/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.AddParams, node.AddResult](
		session.Client,
		node.AddName,
		node.AddParams{
			Type:   "Sprite2D",
			Parent: ".",
			Properties: map[string]json.RawMessage{
				"texture": json.RawMessage(`{"resource":"res://cases/node_add_missing_resource/missing.png"}`),
			},
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}

func TestNodeDeleteSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_delete_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.DeleteParams, node.DeleteResult](
		session.Client,
		node.DeleteName,
		node.DeleteParams{
			Node:          "World/Player",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.delete to succeed: %v", err)
	}

	if !result.Deleted {
		t.Fatal("expected node.delete to report a deletion")
	}

	if !result.Saved {
		t.Fatal("expected node.delete to save the scene")
	}

	if result.Path != "World/Player" {
		t.Fatalf("expected path World/Player, got %s", result.Path)
	}

	_, err = bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          "World/Player",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)

	if err := session.LogJSON(node.DeleteName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeDeleteRootRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_delete_root/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.DeleteParams, node.DeleteResult](
		session.Client,
		node.DeleteName,
		node.DeleteParams{
			Node:          ".",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeFailedPrecondition)
}

func TestNodeDeleteMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_delete_missing/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.DeleteParams, node.DeleteResult](
		session.Client,
		node.DeleteName,
		node.DeleteParams{
			Node:          "Missing",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
}
