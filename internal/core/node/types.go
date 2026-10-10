package node

import "encoding/json"

type InspectParams struct {
	Node          string   `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Properties    []string `json:"properties,omitempty" jsonschema:"description=Property names to include; all editor-visible properties are returned when omitted."`
	ExpectedScene string   `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type SetParams struct {
	Node          string                     `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Properties    map[string]json.RawMessage `json:"properties" jsonschema:"description=Nonempty object of property names to JSON values. Supports booleans numbers strings NodePaths vectors colors and rectangles, plus resource references like {\"resource\": \"res://...\"} and null for object properties; arrays and dictionaries are not supported yet."`
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

type AddParams struct {
	Type          string                     `json:"type" jsonschema:"minLength=1,description=Node class to instantiate, such as Sprite2D. Global script classes are supported."`
	Parent        string                     `json:"parent" jsonschema:"minLength=1,description=Node path of the parent in the active editor scene; use a dot for the scene root."`
	Name          string                     `json:"name,omitempty" jsonschema:"minLength=1,description=Name for the new node. When omitted a unique name is generated from the type. Fails if a sibling already uses the name."`
	Properties    map[string]json.RawMessage `json:"properties,omitempty" jsonschema:"description=Optional property values to apply when the node is added. Supports the same values as node.set, including resource references; the script property may reference a res:// script."`
	ExpectedScene string                     `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool                       `json:"save,omitempty" jsonschema:"description=Save the active scene after adding the node."`
}

type AddResult struct {
	Scene      string                     `json:"scene"`
	Path       string                     `json:"path" jsonschema:"description=Node path of the new node relative to the scene root."`
	Name       string                     `json:"name" jsonschema:"description=Actual node name; may be uniquified when no name was requested."`
	Type       string                     `json:"type" jsonschema:"description=Native Godot class of the new node."`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Actual values after applying the requested properties."`
	Saved      bool                       `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
}

type DeleteParams struct {
	Node          string `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool   `json:"save,omitempty" jsonschema:"description=Save the active scene after deleting the node."`
}

type DeleteResult struct {
	Scene   string `json:"scene"`
	Path    string `json:"path"`
	Deleted bool   `json:"deleted" jsonschema:"description=Whether the node was removed."`
	Saved   bool   `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
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
