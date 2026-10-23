package core_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/node"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestSceneApplyBatchAppliesOperations(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_apply_batch/main.tscn"

	openFixtureScene(t, session, path)

	result := applyScene(t, session, scene.ApplyParams{
		ExpectedScene: path,
		Save:          true,
		Operations: []scene.ApplyOperation{
			{
				Op:     "node.add",
				ID:     "spawn",
				Type:   "Marker2D",
				Parent: "World",
				Name:   "Spawn",
				Properties: map[string]json.RawMessage{
					"position": json.RawMessage(`{"x":32,"y":48}`),
				},
			},
			{
				Op:   "node.set",
				Node: "@spawn",
				Properties: map[string]json.RawMessage{
					"position": json.RawMessage(`{"x":64,"y":96}`),
				},
			},
			{
				Op:     "node.add",
				ID:     "icon",
				Type:   "Sprite2D",
				Parent: "@spawn",
				Name:   "Icon",
			},
			{
				Op:   "node.rename",
				Node: "@spawn",
				Name: "SpawnPoint",
			},
			{
				Op:   "node.set",
				Node: "@icon",
				Properties: map[string]json.RawMessage{
					"position": json.RawMessage(`{"x":1,"y":2}`),
				},
			},
			{
				Op:   "node.duplicate",
				ID:   "icon_copy",
				Node: "@icon",
				Name: "IconCopy",
			},
			{
				Op:     "node.move",
				Node:   "@icon_copy",
				Parent: "World",
				Index:  intPointer(0),
			},
			{
				Op:     "node.instantiate",
				ID:     "batch_player",
				Scene:  "res://scenes/player.tscn",
				Parent: "World",
				Name:   "BatchPlayer",
				Properties: map[string]json.RawMessage{
					"position": json.RawMessage(`{"x":8,"y":9}`),
				},
			},
			{
				Op:   "node.set",
				Node: "@batch_player",
				Properties: map[string]json.RawMessage{
					"speed": json.RawMessage(`250`),
				},
			},
		},
	})

	if !result.Applied {
		t.Fatal("expected scene.apply to commit the batch")
	}

	if result.DryRun {
		t.Fatal("expected dry_run to be false")
	}

	if !result.Saved {
		t.Fatal("expected scene.apply to save the scene")
	}

	if result.Scene != path {
		t.Fatalf("expected scene %s, got %s", path, result.Scene)
	}

	if len(result.Operations) != 9 {
		t.Fatalf("expected 9 operation results, got %d", len(result.Operations))
	}

	expectedPaths := []string{
		"World/Spawn",
		"World/Spawn",
		"World/Spawn/Icon",
		"World/SpawnPoint",
		"World/SpawnPoint/Icon",
		"World/SpawnPoint/IconCopy",
		"World/IconCopy",
		"World/BatchPlayer",
		"World/BatchPlayer",
	}

	for index, expected := range expectedPaths {
		if result.Operations[index].Path != expected {
			t.Fatalf(
				"operation[%d] path = %s; want %s",
				index,
				result.Operations[index].Path,
				expected,
			)
		}
	}

	if result.Operations[0].ID != "spawn" || !result.Operations[0].Changed {
		t.Fatalf("unexpected first operation result: %+v", result.Operations[0])
	}

	if result.Operations[3].Name != "SpawnPoint" {
		t.Fatalf("expected renamed node SpawnPoint, got %q", result.Operations[3].Name)
	}

	if result.Summary.Total != 9 ||
		result.Summary.Changed != 9 ||
		result.Summary.Added != 4 ||
		result.Summary.Deleted != 0 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}

	tree := sceneTree(t, session, path)
	world := childNamed(t, tree.Root, "World")
	names := childNames(world)

	if got := strings.Join(names, ","); got != "IconCopy,Player,Sprite,SpawnPoint,BatchPlayer" {
		t.Fatalf("unexpected World child order: %s", got)
	}

	savedText := readSavedScene(t, session, path)

	for _, expected := range []string{
		`[node name="SpawnPoint" type="Marker2D" parent="World"`,
		"position = Vector2(64, 96)",
		`[node name="Icon" type="Sprite2D" parent="World/SpawnPoint"`,
		`[node name="IconCopy" type="Sprite2D" parent="World"`,
		"position = Vector2(1, 2)",
		`[node name="BatchPlayer" parent="World"`,
		`instance=ExtResource(`,
		"speed = 250.0",
	} {
		if !strings.Contains(savedText, expected) {
			t.Fatalf(
				"expected saved scene to contain %q:\n%s",
				expected,
				savedText,
			)
		}
	}

	if err := session.LogJSON(scene.ApplyName, result); err != nil {
		t.Fatal(err)
	}
}

func TestSceneApplyMarksSceneUnsaved(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_apply_unsaved/main.tscn"

	openFixtureScene(t, session, path)

	result := applyScene(t, session, scene.ApplyParams{
		ExpectedScene: path,
		Operations: []scene.ApplyOperation{
			{
				Op:     "node.add",
				Type:   "Marker2D",
				Parent: "World",
				Name:   "UnsavedMarker",
			},
		},
	})

	if result.Saved {
		t.Fatal("expected scene.apply without save not to save the scene")
	}

	current, err := bridge.Call[scene.CurrentParams, scene.CurrentResult](
		session.Client,
		scene.CurrentName,
		scene.CurrentParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected scene.current to succeed: %v", err)
	}

	if !current.Unsaved {
		t.Fatal("expected the batch to mark the scene unsaved")
	}
}

func TestSceneApplyFailedPrevalidationLeavesSceneUntouched(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_apply_rollback/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[scene.ApplyParams, scene.ApplyResult](
		session.Client,
		scene.ApplyName,
		scene.ApplyParams{
			ExpectedScene: path,
			Operations: []scene.ApplyOperation{
				{
					Op:     "node.add",
					ID:     "temp",
					Type:   "Marker2D",
					Parent: "World",
					Name:   "Temp",
				},
				{
					Op:   "node.rename",
					Node: "@temp",
					Name: "TempRenamed",
				},
				{
					Op:   "node.set",
					Node: "@temp",
					Properties: map[string]json.RawMessage{
						"definitely_missing": json.RawMessage(`true`),
					},
				},
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)
	assertApplyErrorMessage(t, err,
		"Prevalidation failed",
		"operation[2]",
		"node.set",
		"definitely_missing",
	)

	tree := sceneTree(t, session, path)
	world := childNamed(t, tree.Root, "World")

	if got := strings.Join(childNames(world), ","); got != "Existing" {
		t.Fatalf("failed batch left nodes behind: %s", got)
	}

	// The editor must still accept a normal batch after a failed batch.
	result := applyScene(t, session, scene.ApplyParams{
		ExpectedScene: path,
		Operations: []scene.ApplyOperation{
			{
				Op:     "node.add",
				Type:   "Marker2D",
				Parent: "World/Existing",
				Name:   "Recovered",
			},
		},
	})

	if len(result.Operations) != 1 || result.Operations[0].Path != "World/Existing/Recovered" {
		t.Fatalf("unexpected recovery result: %+v", result.Operations)
	}
}

func TestSceneApplyRollsBackWhenLiveStateDrifts(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_apply_live_rollback/main.tscn"

	openFixtureScene(t, session, path)

	// The tool script creates an unowned editor-only node that
	// PackedScene.pack omits from the prevalidation copy.
	before := sceneTree(t, session, path)
	worldBefore := childNamed(t, before.Root, "World")

	if got := strings.Join(childNames(worldBefore), ","); got != "Existing,RuntimeOnly" {
		t.Fatalf("expected editor-only node before the batch, got %s", got)
	}

	_, err := bridge.Call[scene.ApplyParams, scene.ApplyResult](
		session.Client,
		scene.ApplyName,
		scene.ApplyParams{
			ExpectedScene: path,
			Operations: []scene.ApplyOperation{
				{
					Op:   "node.set",
					ID:   "existing",
					Node: "World/Existing",
					Properties: map[string]json.RawMessage{
						"position": json.RawMessage(`{"x":50,"y":60}`),
					},
				},
				{
					Op:   "node.rename",
					Node: "@existing",
					Name: "Renamed",
				},
				{
					Op:     "node.add",
					ID:     "temp",
					Type:   "Marker2D",
					Parent: "World",
					Name:   "Temp",
				},
				{
					Op:     "node.move",
					Node:   "@existing",
					Parent: "@temp",
				},
				{
					Op:     "node.add",
					ID:     "victim",
					Type:   "Marker2D",
					Parent: "World",
					Name:   "Victim",
				},
				{
					Op:   "node.delete",
					Node: "@victim",
				},
				{
					Op:     "node.add",
					Type:   "Marker2D",
					Parent: "World",
					Name:   "RuntimeOnly",
				},
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)
	assertApplyErrorMessage(t, err,
		"Batch failed and was rolled back",
		"operation[6]",
		"node.add",
		"RuntimeOnly",
	)

	tree := sceneTree(t, session, path)
	world := childNamed(t, tree.Root, "World")

	if got := strings.Join(childNames(world), ","); got != "Existing,RuntimeOnly" {
		t.Fatalf("rollback did not restore the original tree: %s", got)
	}

	existing := childNamed(t, world, "Existing")
	position := inspectorProperty(t, session, path, existing.Path, "position")

	var vector struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}

	if err := json.Unmarshal(position.Value, &vector); err != nil {
		t.Fatalf("decoding restored position: %v", err)
	}

	if vector.X != 5 || vector.Y != 6 {
		t.Fatalf("rollback did not restore position, got %+v", vector)
	}

	for _, removed := range []string{"Temp", "Renamed", "Victim"} {
		for _, child := range world.Children {
			if child.Name == removed {
				t.Fatalf("rollback left %s behind", removed)
			}
		}
	}
}

func TestSceneApplyDryRunDoesNotMutateScene(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_apply_dry_run/main.tscn"

	openFixtureScene(t, session, path)

	result, err := bridge.Call[scene.ApplyParams, scene.ApplyResult](
		session.Client,
		scene.ApplyName,
		scene.ApplyParams{
			ExpectedScene: path,
			DryRun:        true,
			Operations: []scene.ApplyOperation{
				{
					Op:     "node.add",
					ID:     "temp",
					Type:   "Marker2D",
					Parent: "World",
					Name:   "Temp",
				},
				{
					Op:   "node.rename",
					Node: "@temp",
					Name: "DryRunTemp",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("expected scene.apply dry_run to succeed: %v", err)
	}

	if result.Applied {
		t.Fatal("expected dry_run not to commit the batch")
	}

	if !result.DryRun {
		t.Fatal("expected dry_run to be true")
	}

	if result.Saved {
		t.Fatal("expected dry_run not to save the scene")
	}

	if len(result.Operations) != 2 {
		t.Fatalf("expected 2 planned operations, got %d", len(result.Operations))
	}

	if result.Operations[0].Path != "World/Temp" ||
		result.Operations[1].Path != "World/DryRunTemp" {
		t.Fatalf("unexpected planned paths: %+v", result.Operations)
	}

	if result.Summary.Total != 2 || result.Summary.Changed != 2 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}

	tree := sceneTree(t, session, path)
	world := childNamed(t, tree.Root, "World")

	if got := strings.Join(childNames(world), ","); got != "Existing" {
		t.Fatalf("dry_run mutated the live scene: %s", got)
	}

	if err := session.LogJSON(scene.ApplyName, result); err != nil {
		t.Fatal(err)
	}
}

func TestSceneApplyPrevalidationRejectsConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_apply_prevalidation/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[scene.ApplyParams, scene.ApplyResult](
		session.Client,
		scene.ApplyName,
		scene.ApplyParams{
			ExpectedScene: path,
			Operations: []scene.ApplyOperation{
				{
					Op:     "node.add",
					Type:   "Marker2D",
					Parent: "World",
					Name:   "Existing",
				},
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)
	assertApplyErrorMessage(t, err, "Prevalidation failed", "operation[0]", "node.add")

	tree := sceneTree(t, session, path)
	world := childNamed(t, tree.Root, "World")

	if got := strings.Join(childNames(world), ","); got != "Existing" {
		t.Fatalf("prevalidation failure mutated the live scene: %s", got)
	}
}

func TestSceneApplyExpectedSceneConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/scene_apply_expected_scene/main.tscn"

	openFixtureScene(t, session, path)

	_, err := bridge.Call[scene.ApplyParams, scene.ApplyResult](
		session.Client,
		scene.ApplyName,
		scene.ApplyParams{
			ExpectedScene: "res://cases/scene_apply_batch/main.tscn",
			Operations: []scene.ApplyOperation{
				{
					Op:     "node.add",
					Type:   "Marker2D",
					Parent: "World",
					Name:   "WrongScene",
				},
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)
}

func TestSceneApplyRejectsInvalidBatchesBeforeConnecting(t *testing.T) {
	cases := []struct {
		name     string
		params   scene.ApplyParams
		contains string
	}{
		{
			name:     "empty",
			params:   scene.ApplyParams{},
			contains: "nonempty operations array",
		},
		{
			name: "unknown operation",
			params: scene.ApplyParams{Operations: []scene.ApplyOperation{
				{Op: "node.explode"},
			}},
			contains: "unknown operation",
		},
		{
			name: "duplicate id",
			params: scene.ApplyParams{Operations: []scene.ApplyOperation{
				{Op: "node.delete", ID: "x", Node: "."},
				{Op: "node.delete", ID: "x", Node: "."},
			}},
			contains: "duplicate id",
		},
		{
			name: "unknown id reference",
			params: scene.ApplyParams{Operations: []scene.ApplyOperation{
				{Op: "node.set", Node: "@missing", Properties: map[string]json.RawMessage{
					"visible": json.RawMessage(`true`),
				}},
			}},
			contains: "not-yet-defined operation id",
		},
		{
			name: "field not valid for operation",
			params: scene.ApplyParams{Operations: []scene.ApplyOperation{
				{Op: "node.delete", Node: ".", Type: "Marker2D"},
			}},
			contains: "does not accept field type",
		},
		{
			name: "dry run with save",
			params: scene.ApplyParams{
				DryRun: true,
				Save:   true,
				Operations: []scene.ApplyOperation{
					{Op: "node.delete", Node: "."},
				},
			},
			contains: "save cannot be combined with dry_run",
		},
		{
			name: "set without properties",
			params: scene.ApplyParams{Operations: []scene.ApplyOperation{
				{Op: "node.set", Node: "."},
			}},
			contains: "nonempty properties object",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := scene.Apply("", testCase.params)

			assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)
			assertApplyErrorMessage(t, err, testCase.contains)
		})
	}
}

func applyScene(
	t *testing.T,
	session *harness.Session,
	params scene.ApplyParams,
) scene.ApplyResult {
	t.Helper()

	result, err := bridge.Call[scene.ApplyParams, scene.ApplyResult](
		session.Client,
		scene.ApplyName,
		params,
	)
	if err != nil {
		t.Fatalf("expected scene.apply to succeed: %v", err)
	}

	return result
}

func sceneTree(
	t *testing.T,
	session *harness.Session,
	path string,
) scene.TreeResult {
	t.Helper()

	tree, err := bridge.Call[scene.TreeParams, scene.TreeResult](
		session.Client,
		scene.TreeName,
		scene.TreeParams{ExpectedScene: path},
	)
	if err != nil {
		t.Fatalf("expected scene.tree to succeed: %v", err)
	}

	return tree
}

func childNamed(t *testing.T, node scene.TreeNode, name string) scene.TreeNode {
	t.Helper()

	for _, child := range node.Children {
		if child.Name == name {
			return child
		}
	}

	t.Fatalf("node %s has no child named %s", node.Path, name)

	return scene.TreeNode{}
}

func inspectorProperty(
	t *testing.T,
	session *harness.Session,
	path string,
	nodePath string,
	name string,
) node.Property {
	t.Helper()

	result, err := bridge.Call[node.InspectParams, node.InspectResult](
		session.Client,
		node.InspectName,
		node.InspectParams{
			Node:          nodePath,
			Properties:    []string{name},
			ExpectedScene: path,
		},
	)
	if err != nil {
		t.Fatalf("expected node.inspect to succeed: %v", err)
	}

	for _, property := range result.Properties {
		if property.Name == name {
			return property
		}
	}

	t.Fatalf("node %s has no property %s", nodePath, name)

	return node.Property{}
}

func childNames(node scene.TreeNode) []string {
	names := make([]string, 0, len(node.Children))

	for _, child := range node.Children {
		names = append(names, child.Name)
	}

	return names
}

func intPointer(value int) *int {
	return &value
}

func assertApplyErrorMessage(t *testing.T, err error, fragments ...string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected an apply error")
	}

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) {
		t.Fatalf("expected an API error, got %v", err)
	}

	for _, fragment := range fragments {
		if !strings.Contains(apiError.Message, fragment) {
			t.Fatalf(
				"expected error message to contain %q, got %q",
				fragment,
				apiError.Message,
			)
		}
	}
}
