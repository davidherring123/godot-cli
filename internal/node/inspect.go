package node

import (
	"encoding/json"

	"github.com/davidherring123/godot-cli/internal/bridge"
)

type Property struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type InspectResult struct {
	Scene      string     `json:"scene"`
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Properties []Property `json:"properties"`
}

type InspectParams struct {
	Scene string `json:"scene"`
	Node  string `json:"node"`
}

func Inspect(
	client *bridge.Client,
	scenePath string,
	nodePath string,
) (InspectResult, error) {
	return bridge.Call[InspectParams, InspectResult](
		client,
		"node.inspect",
		InspectParams{
			Scene: scenePath,
			Node:  nodePath,
		},
	)
}
