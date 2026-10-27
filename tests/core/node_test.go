package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-mcp/internal/adapter/bridge"
	"github.com/davidherring123/godot-mcp/internal/core/node"
	"github.com/davidherring123/godot-mcp/internal/core/scene"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
	"github.com/davidherring123/godot-mcp/tests/harness"
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

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestNodeSetSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_set_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result := setNodeProperties(
		t,
		session,
		path,
		"Player",
		map[string]json.RawMessage{
			"position": json.RawMessage(`{"x":64,"y":96}`),
		},
	)

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

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestNodeSetResourceSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_set_resource/main.tscn"

	openFixtureScene(t, session, path)

	result := setNodeProperties(
		t,
		session,
		path,
		"Sprite",
		map[string]json.RawMessage{
			"texture": json.RawMessage(`{"resource":"res://assets/placeholder_texture.tres"}`),
		},
	)

	assertResourceProperty(
		t,
		result.Properties,
		"texture",
		"res://assets/placeholder_texture.tres",
		"PlaceholderTexture2D",
	)

	if err := session.LogJSON(node.SetName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeSetScriptSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_set_script/main.tscn"

	openFixtureScene(t, session, path)

	result := setNodeProperties(
		t,
		session,
		path,
		"Player",
		map[string]json.RawMessage{
			"script": json.RawMessage(`{"resource":"res://scripts/player.gd"}`),
		},
	)

	assertResourceProperty(
		t,
		result.Properties,
		"script",
		"res://scripts/player.gd",
		"GDScript",
	)

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
			Save:          true,
			Properties: map[string]json.RawMessage{
				"position": json.RawMessage(`{"x":32,"y":48}`),
			},
		},
	)
	if err != nil {
		t.Fatalf("expected node.add to succeed: %v", err)
	}

	if !result.Saved {
		t.Fatal("expected node.add to save the scene")
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

	assertAPIErrorCode(t, err, apierr.CodeConflict)
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

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestNodeAddUnknownTypeRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_add_unknown_type/main.tscn"

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

	assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)
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

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
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

	assertAPIErrorCode(t, err, apierr.CodeNotFound)

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

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)
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

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestNodeInstantiateSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_instantiate_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result := instantiateNode(t, session, node.InstantiateParams{
		Scene:         "res://scenes/player.tscn",
		Parent:        "World",
		Name:          "Player",
		ExpectedScene: path,
		Save:          true,
		Properties: map[string]json.RawMessage{
			"position": json.RawMessage(`{"x":64,"y":96}`),
			"speed":    json.RawMessage(`400`),
		},
	})

	if !result.Saved {
		t.Fatal("expected node.instantiate to save the scene")
	}

	if result.Source != "res://scenes/player.tscn" {
		t.Fatalf("expected source res://scenes/player.tscn, got %s", result.Source)
	}

	if result.Path != "World/Player" {
		t.Fatalf("expected path World/Player, got %s", result.Path)
	}

	if result.Name != "Player" {
		t.Fatalf("expected name Player, got %s", result.Name)
	}

	if result.Type != "CharacterBody2D" {
		t.Fatalf("expected type CharacterBody2D, got %s", result.Type)
	}

	var position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}

	if err := json.Unmarshal(result.Properties["position"], &position); err != nil {
		t.Fatalf("decoding position property: %v", err)
	}

	if position.X != 64 || position.Y != 96 {
		t.Fatalf("expected position (64, 96), got %+v", position)
	}

	var speed float64

	if err := json.Unmarshal(result.Properties["speed"], &speed); err != nil {
		t.Fatalf("decoding speed property: %v", err)
	}

	if speed != 400 {
		t.Fatalf("expected speed 400, got %v", speed)
	}

	savedText := readSavedScene(t, session, path)

	if !strings.Contains(savedText, "res://scenes/player.tscn") {
		t.Fatalf(
			"expected saved scene to reference res://scenes/player.tscn:\n%s",
			savedText,
		)
	}

	if !strings.Contains(savedText, "instance=") {
		t.Fatalf(
			"expected saved scene to contain a linked scene instance:\n%s",
			savedText,
		)
	}

	if !strings.Contains(savedText, "position = Vector2(64, 96)") {
		t.Fatalf(
			"expected the instance override to persist in the saved scene:\n%s",
			savedText,
		)
	}

	if err := session.LogJSON(node.InstantiateName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeInstantiateMissingSceneReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_instantiate_missing_scene/main.tscn"

	openFixtureScene(t, session, path)

	assertInstantiateErrorCode(t, session, node.InstantiateParams{
		Scene:         "res://scenes/missing.tscn",
		Parent:        ".",
		ExpectedScene: path,
	}, apierr.CodeNotFound)
}

func TestNodeInstantiateSelfReferenceRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_instantiate_self_reference/main.tscn"

	openFixtureScene(t, session, path)

	assertInstantiateErrorCode(t, session, node.InstantiateParams{
		Scene:         path,
		Parent:        ".",
		ExpectedScene: path,
	}, apierr.CodeFailedPrecondition)
}

func TestNodeInstantiateUnsavedSourceRejected(t *testing.T) {
	session := requireSuite(t)
	destination := "res://cases/node_instantiate_unsaved_source/main.tscn"
	source := "res://scenes/player.tscn"

	openFixtureScene(t, session, source)

	setNodeProperties(
		t,
		session,
		source,
		".",
		map[string]json.RawMessage{
			"position": json.RawMessage(`{"x":8,"y":8}`),
		},
	)

	openFixtureScene(t, session, destination)

	assertInstantiateErrorCode(t, session, node.InstantiateParams{
		Scene:         source,
		Parent:        ".",
		ExpectedScene: destination,
	}, apierr.CodeFailedPrecondition)

	openFixtureScene(t, session, source)

	if _, err := bridge.Call[scene.SaveParams, scene.SaveResult](
		session.Client,
		scene.SaveName,
		scene.SaveParams{ExpectedScene: source},
	); err != nil {
		t.Fatalf("saving source scene after test: %v", err)
	}
}

func TestNodeRenameSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_rename_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.RenameParams, node.RenameResult](
		session.Client,
		node.RenameName,
		node.RenameParams{
			Node:          "Player",
			Name:          "Hero",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.rename to succeed: %v", err)
	}

	if result.Path != "Player" || result.Name != "Player" {
		t.Fatalf(
			"expected previous path/name Player, got %s/%s",
			result.Path,
			result.Name,
		)
	}

	if result.NewPath != "Hero" || result.NewName != "Hero" {
		t.Fatalf(
			"expected new path/name Hero, got %s/%s",
			result.NewPath,
			result.NewName,
		)
	}

	if !result.Changed {
		t.Fatal("expected node.rename to report a change")
	}

	if !result.Saved {
		t.Fatal("expected node.rename to save the scene")
	}

	savedText := readSavedScene(t, session, path)

	for _, expected := range []string{
		`[node name="Hero" type="Node2D" parent="."`,
		`remote_path = NodePath("../Hero")`,
		`tracks/0/path = NodePath("Hero:position")`,
	} {
		if !strings.Contains(savedText, expected) {
			t.Fatalf(
				"expected saved scene to contain %q:\n%s",
				expected,
				savedText,
			)
		}
	}

	if strings.Contains(savedText, `name="Player"`) {
		t.Fatalf("expected the old node name to be gone:\n%s", savedText)
	}

	_, err = bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          "Player",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)

	if err := session.LogJSON(node.RenameName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeRenameConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_rename_conflict/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.RenameParams, node.RenameResult](
		session.Client,
		node.RenameName,
		node.RenameParams{
			Node:          "Player",
			Name:          "World",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)
}

func TestNodeRenameSameNameIsNoOp(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_rename_same_name/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.RenameParams, node.RenameResult](
		session.Client,
		node.RenameName,
		node.RenameParams{
			Node:          "Player",
			Name:          "Player",
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected node.rename to succeed: %v", err)
	}

	if result.Changed {
		t.Fatal("expected renaming to the same name to report no change")
	}

	if result.NewPath != "Player" || result.NewName != "Player" {
		t.Fatalf(
			"expected path/name to stay Player, got %s/%s",
			result.NewPath,
			result.NewName,
		)
	}
}

func TestNodeRenameMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_rename_missing/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.RenameParams, node.RenameResult](
		session.Client,
		node.RenameName,
		node.RenameParams{
			Node:          "Missing",
			Name:          "Other",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestNodeRenameRootSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_rename_root/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.RenameParams, node.RenameResult](
		session.Client,
		node.RenameName,
		node.RenameParams{
			Node:          ".",
			Name:          "Arena",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.rename to succeed: %v", err)
	}

	if result.Name != "Main" || result.NewName != "Arena" {
		t.Fatalf(
			"expected Main to become Arena, got %s to %s",
			result.Name,
			result.NewName,
		)
	}

	if result.Path != "." || result.NewPath != "." {
		t.Fatalf(
			"expected the root path to stay ., got %s and %s",
			result.Path,
			result.NewPath,
		)
	}

	savedText := readSavedScene(t, session, path)

	if !strings.Contains(savedText, `[node name="Arena" type="Node2D"`) {
		t.Fatalf("expected the saved root to be renamed:\n%s", savedText)
	}
}

func TestNodeRenameInstanceChildRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_rename_instance_child/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.RenameParams, node.RenameResult](
		session.Client,
		node.RenameName,
		node.RenameParams{
			Node:          "Player/Sprite",
			Name:          "Icon",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)
}

func TestNodeDuplicateSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_duplicate_succeeds/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.DuplicateParams, node.DuplicateResult](
		session.Client,
		node.DuplicateName,
		node.DuplicateParams{
			Node:          "World/Player",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.duplicate to succeed: %v", err)
	}

	if result.Path != "World/Player" || result.Name != "Player" {
		t.Fatalf(
			"expected the original World/Player, got %s/%s",
			result.Path,
			result.Name,
		)
	}

	if result.NewPath != "World/Player2" || result.NewName != "Player2" {
		t.Fatalf(
			"expected the copy World/Player2, got %s/%s",
			result.NewPath,
			result.NewName,
		)
	}

	if result.Type != "Node2D" {
		t.Fatalf("expected type Node2D, got %s", result.Type)
	}

	if !result.Saved {
		t.Fatal("expected node.duplicate to save the scene")
	}

	child, err := bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          "World/Player2/Spawn",
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected the duplicated child to exist: %v", err)
	}

	if child.Type != "Marker2D" {
		t.Fatalf("expected duplicated child type Marker2D, got %s", child.Type)
	}

	copied, err := bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          "World/Player2",
			Properties:    []string{"position"},
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected the duplicated node to exist: %v", err)
	}

	if len(copied.Properties) != 1 {
		t.Fatalf("expected one position property, got %d", len(copied.Properties))
	}

	var position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}

	if err := json.Unmarshal(copied.Properties[0].Value, &position); err != nil {
		t.Fatalf("decoding copied position: %v", err)
	}

	if position.X != 32 || position.Y != 48 {
		t.Fatalf("expected copied position (32, 48), got %+v", position)
	}

	savedText := readSavedScene(t, session, path)

	if !strings.Contains(savedText, `name="Player2"`) {
		t.Fatalf("expected the copy in the saved scene:\n%s", savedText)
	}

	if err := session.LogJSON(node.DuplicateName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeDuplicateInstanceSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_duplicate_instance/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.DuplicateParams, node.DuplicateResult](
		session.Client,
		node.DuplicateName,
		node.DuplicateParams{
			Node:          "Player",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.duplicate to succeed: %v", err)
	}

	if result.NewName != "Player2" || result.Type != "CharacterBody2D" {
		t.Fatalf(
			"expected a CharacterBody2D copy named Player2, got %s/%s",
			result.NewName,
			result.Type,
		)
	}

	savedText := readSavedScene(t, session, path)

	if count := strings.Count(savedText, "instance=ExtResource"); count != 2 {
		t.Fatalf(
			"expected two scene instances in the saved scene, got %d:\n%s",
			count,
			savedText,
		)
	}
}

func TestNodeDuplicateConnectionSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_duplicate_connection/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.DuplicateParams, node.DuplicateResult](
		session.Client,
		node.DuplicateName,
		node.DuplicateParams{
			Node:          "Timer",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.duplicate to succeed: %v", err)
	}

	if result.NewName != "Timer2" {
		t.Fatalf("expected copy name Timer2, got %s", result.NewName)
	}

	savedText := readSavedScene(t, session, path)

	if count := strings.Count(savedText, "[connection "); count != 2 {
		t.Fatalf(
			"expected two connections in the saved scene, got %d:\n%s",
			count,
			savedText,
		)
	}

	if !strings.Contains(savedText, `from="Timer2"`) {
		t.Fatalf(
			"expected the copy's connection in the saved scene:\n%s",
			savedText,
		)
	}
}

func TestNodeDuplicateRootRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_duplicate_root/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.DuplicateParams, node.DuplicateResult](
		session.Client,
		node.DuplicateName,
		node.DuplicateParams{
			Node:          ".",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)
}

func TestNodeDuplicateNameConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_duplicate_name_conflict/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.DuplicateParams, node.DuplicateResult](
		session.Client,
		node.DuplicateName,
		node.DuplicateParams{
			Node:          "Player",
			Name:          "World",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)
}

func TestNodeDuplicateMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_duplicate_missing/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.DuplicateParams, node.DuplicateResult](
		session.Client,
		node.DuplicateName,
		node.DuplicateParams{
			Node:          "Missing",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestNodeMoveReparentsWithReferences(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_reparent/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          "Player",
			Parent:        "World",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.move to succeed: %v", err)
	}

	if result.Path != "Player" || result.Name != "Player" {
		t.Fatalf(
			"expected previous path/name Player, got %s/%s",
			result.Path,
			result.Name,
		)
	}

	if result.NewPath != "World/Player" || result.NewName != "Player" {
		t.Fatalf(
			"expected new path World/Player, got %s/%s",
			result.NewPath,
			result.NewName,
		)
	}

	if !result.Changed {
		t.Fatal("expected node.move to report a change")
	}

	if !result.Saved {
		t.Fatal("expected node.move to save the scene")
	}

	savedText := readSavedScene(t, session, path)

	for _, expected := range []string{
		`[node name="Player" type="Node2D" parent="World"`,
		`remote_path = NodePath("../World/Player")`,
		`tracks/0/path = NodePath("World/Player:position")`,
		`position = Vector2(-90, -80)`,
	} {
		if !strings.Contains(savedText, expected) {
			t.Fatalf(
				"expected saved scene to contain %q:\n%s",
				expected,
				savedText,
			)
		}
	}

	_, err = bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          "Player",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)

	if err := session.LogJSON(node.MoveName, result); err != nil {
		t.Fatal(err)
	}
}

func TestNodeMoveReorders(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_reorder/main.tscn"

	openFixtureScene(t, session, path)

	index := 2

	result, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          "A",
			Parent:        ".",
			Index:         &index,
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.move to succeed: %v", err)
	}

	if result.NewPath != "A" || result.NewName != "A" {
		t.Fatalf(
			"expected the reordered node to stay at A, got %s/%s",
			result.NewPath,
			result.NewName,
		)
	}

	if !result.Changed {
		t.Fatal("expected node.move to report a change")
	}

	tree, err := bridge.Call[scene.TreeParams, scene.TreeResult](
		session.Client,
		scene.TreeName,
		scene.TreeParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected scene.tree to succeed: %v", err)
	}

	names := make([]string, 0, len(tree.Root.Children))

	for _, child := range tree.Root.Children {
		names = append(names, child.Name)
	}

	if strings.Join(names, ",") != "B,C,A" {
		t.Fatalf("expected child order B,C,A, got %s", strings.Join(names, ","))
	}
}

func TestNodeMoveSamePositionIsNoOp(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_same_position/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          "B",
			Parent:        ".",
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected node.move to succeed: %v", err)
	}

	if result.Changed {
		t.Fatal("expected moving to the same position to report no change")
	}

	if result.NewPath != "B" {
		t.Fatalf("expected path B, got %s", result.NewPath)
	}
}

func TestNodeMoveUnderSelfRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_under_self/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          "Parent",
			Parent:        "Parent/Child",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)
}

func TestNodeMoveRootRejected(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_root/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          ".",
			Parent:        ".",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeFailedPrecondition)
}

func TestNodeMoveGeneratesUniqueName(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_name_conflict/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          "Player",
			Parent:        "World",
			ExpectedScene: path,
			Save:          true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.move to succeed: %v", err)
	}

	if result.NewName != "Player2" || result.NewPath != "World/Player2" {
		t.Fatalf(
			"expected the move to uniquify to World/Player2, got %s/%s",
			result.NewPath,
			result.NewName,
		)
	}

	savedText := readSavedScene(t, session, path)

	if !strings.Contains(savedText, `[node name="Player2" type="Node2D" parent="World"`) {
		t.Fatalf("expected the uniquified node in the saved scene:\n%s", savedText)
	}
}

func TestNodeMoveMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_missing/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          "Missing",
			Parent:        ".",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)

	_, err = bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:          ".",
			Parent:        "Missing",
			ExpectedScene: path,
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestNodeMoveKeepsLocalTransform(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_move_keep_local/main.tscn"

	openFixtureScene(t, session, path)

	keep := false

	_, err := bridge.Call[node.MoveParams, node.MoveResult](
		session.Client,
		node.MoveName,
		node.MoveParams{
			Node:                "Player",
			Parent:              "World",
			KeepGlobalTransform: &keep,
			ExpectedScene:       path,
			Save:                true,
		},
	)
	if err != nil {
		t.Fatalf("expected node.move to succeed: %v", err)
	}

	savedText := readSavedScene(t, session, path)

	if !strings.Contains(savedText, "position = Vector2(10, 20)") {
		t.Fatalf(
			"expected the local transform to be kept in the saved scene:\n%s",
			savedText,
		)
	}
}

func TestNodeSetThemeOverrideSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/node_set_theme_override/main.tscn"

	openFixtureScene(t, session, path)

	result := setNodeProperties(
		t,
		session,
		path,
		"Label",
		map[string]json.RawMessage{
			"theme_override_font_sizes/font_size": json.RawMessage(`24`),
			"theme_override_colors/font_color":    json.RawMessage(`{"r":1,"g":0,"b":0,"a":1}`),
		},
	)

	var fontSize float64

	if err := json.Unmarshal(
		result.Properties["theme_override_font_sizes/font_size"],
		&fontSize,
	); err != nil {
		t.Fatalf("decoding theme override font size: %v", err)
	}

	if fontSize != 24 {
		t.Fatalf("expected font size 24, got %v", fontSize)
	}

	if _, err := bridge.Call[scene.SaveParams, scene.SaveResult](
		session.Client,
		scene.SaveName,
		scene.SaveParams{ExpectedScene: path},
	); err != nil {
		t.Fatalf("saving the theme override scene: %v", err)
	}

	savedText := readSavedScene(t, session, path)

	for _, expected := range []string{
		"theme_override_font_sizes/font_size = 24",
		"theme_override_colors/font_color = Color(1, 0, 0, 1)",
	} {
		if !strings.Contains(savedText, expected) {
			t.Fatalf(
				"expected saved scene to contain %q:\n%s",
				expected,
				savedText,
			)
		}
	}

	if err := session.LogJSON(node.SetName, result); err != nil {
		t.Fatal(err)
	}
}

func instantiateNode(
	t *testing.T,
	session *harness.Session,
	params node.InstantiateParams,
) node.InstantiateResult {
	t.Helper()

	result, err := bridge.Call[node.InstantiateParams, node.InstantiateResult](
		session.Client,
		node.InstantiateName,
		params,
	)
	if err != nil {
		t.Fatalf("expected node.instantiate to succeed: %v", err)
	}

	return result
}

func assertInstantiateErrorCode(
	t *testing.T,
	session *harness.Session,
	params node.InstantiateParams,
	expected apierr.Code,
) {
	t.Helper()

	_, err := bridge.Call[node.InstantiateParams, node.InstantiateResult](
		session.Client,
		node.InstantiateName,
		params,
	)

	assertAPIErrorCode(t, err, expected)
}

func readSavedScene(
	t *testing.T,
	session *harness.Session,
	scenePath string,
) string {
	t.Helper()

	relative := strings.TrimPrefix(scenePath, "res://")
	saved, err := os.ReadFile(filepath.Join(session.Root(), filepath.FromSlash(relative)))

	if err != nil {
		t.Fatalf("reading saved scene %s: %v", scenePath, err)
	}

	return string(saved)
}

func setNodeProperties(
	t *testing.T,
	session *harness.Session,
	path string,
	nodePath string,
	properties map[string]json.RawMessage,
) node.SetResult {
	t.Helper()

	result, err := bridge.Call[node.SetParams, node.SetResult](
		session.Client,
		node.SetName,
		node.SetParams{
			Node:          nodePath,
			ExpectedScene: path,
			Properties:    properties,
		},
	)
	if err != nil {
		t.Fatalf("expected node.set to succeed: %v", err)
	}

	if !result.Changed {
		t.Fatal("expected node.set to report a change")
	}

	return result
}

func assertResourceProperty(
	t *testing.T,
	properties map[string]json.RawMessage,
	name string,
	resource string,
	resourceType string,
) {
	t.Helper()

	var property map[string]any

	if err := json.Unmarshal(properties[name], &property); err != nil {
		t.Fatalf("decoding %s property: %v", name, err)
	}

	if property["resource"] != resource {
		t.Fatalf(
			"expected %s resource %s, got %v",
			name,
			resource,
			property["resource"],
		)
	}

	if property["type"] != resourceType {
		t.Fatalf(
			"expected %s type %s, got %v",
			name,
			resourceType,
			property["type"],
		)
	}
}
