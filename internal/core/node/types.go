package node

import "encoding/json"

type InspectParams struct {
	Scene string `json:"scene" jsonschema:"minLength=1,description=Scene resource path such as res://main.tscn."`
	Node  string `json:"node" jsonschema:"minLength=1,description=Node path relative to the scene root; use a dot for the root."`
}

type InspectResult struct {
	Scene      string     `json:"scene"`
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Properties []Property `json:"properties"`
}

type Property struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Hint  *PropertyHint   `json:"hint,omitempty"`
	Value json.RawMessage `json:"value" jsonschema:"description=Serialized Godot Variant; its JSON shape depends on the property type."`
}

type PropertyHint struct {
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
	Types    []string `json:"types,omitempty"`
	Value    string   `json:"value,omitempty"`
	Selected []Layer  `json:"selected,omitempty"`
}

type Layer struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}
