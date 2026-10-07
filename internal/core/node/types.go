package node

import "encoding/json"

type InspectParams struct {
	Node          string   `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Properties    []string `json:"properties,omitempty" jsonschema:"description=Property names to include; all editor-visible properties are returned when omitted."`
	ExpectedScene string   `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type SetParams struct {
	Node          string                     `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Properties    map[string]json.RawMessage `json:"properties" jsonschema:"description=Nonempty object of property names to JSON values. Supports booleans numbers strings NodePaths vectors colors and rectangles; resource and collection assignments are not supported yet."`
	ExpectedScene string                     `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool                       `json:"save,omitempty" jsonschema:"description=Save the active scene after applying the properties."`
}

type SetResult struct {
	Scene      string                     `json:"scene"`
	Path       string                     `json:"path"`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Actual values after applying the requested properties."`
	Changed    bool                       `json:"changed" jsonschema:"description=Whether an undoable edit was committed."`
	Saved      bool                       `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
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
