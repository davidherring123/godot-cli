package scene

import "encoding/json"

type ListParams struct{}

type ListResult struct {
	Scenes []string `json:"scenes" jsonschema:"description=Project scene paths with the res:// prefix; an empty array when no scenes are found."`
}

type TreeParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type CurrentParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type CurrentResult struct {
	Scene   string `json:"scene" jsonschema:"description=Active scene resource path; empty when no scene is being edited or the scene has never been saved."`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Unsaved bool   `json:"unsaved"`
	Active  bool   `json:"active" jsonschema:"description=Whether a scene is currently being edited; false when no scene is open."`
}

type OpenParams struct {
	Scene string `json:"scene" jsonschema:"minLength=1,description=Existing scene resource path such as res://main.tscn."`
}

type OpenResult = CurrentResult

type SaveParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type SaveResult struct {
	Scene string `json:"scene"`
	Saved bool   `json:"saved"`
}

type CreateParams struct {
	Scene      string                     `json:"scene" jsonschema:"minLength=1,description=New scene resource path such as res://levels/arena.tscn, ending in .tscn or .scn. Fails if the file already exists."`
	RootType   string                     `json:"root_type" jsonschema:"minLength=1,description=Root node class such as Node2D. Global script classes are supported."`
	Name       string                     `json:"name,omitempty" jsonschema:"minLength=1,description=Root node name; defaults to the file name without its extension."`
	Properties map[string]json.RawMessage `json:"properties,omitempty" jsonschema:"description=Optional root property values to apply when the scene is created. Supports the same values as node.set, including a res:// script reference."`
}

type CreateResult struct {
	Scene      string                     `json:"scene"`
	Name       string                     `json:"name"`
	Type       string                     `json:"type"`
	Unsaved    bool                       `json:"unsaved"`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Actual root property values after applying the requested properties."`
}

type TreeResult struct {
	Scene string   `json:"scene"`
	Root  TreeNode `json:"root"`
}

type TreeNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Path     string     `json:"path" jsonschema:"description=Node path relative to the scene root; the root is a dot."`
	Children []TreeNode `json:"children"`
}
