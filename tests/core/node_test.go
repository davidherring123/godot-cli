package core_test

import (
	"encoding/json"
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
