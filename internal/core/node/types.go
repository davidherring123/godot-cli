package node

import (
	"encoding/json"

	"github.com/davidherring123/godot-cli/internal/model/property"
)

type InspectParams struct {
	Node          string   `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Properties    []string `json:"properties,omitempty" jsonschema:"description=Property names to include; all editor-visible properties are returned when omitted."`
	ExpectedScene string   `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type SetParams struct {
	Node          string                     `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Properties    map[string]json.RawMessage `json:"properties" jsonschema:"description=Nonempty object of property names to JSON values. Supports booleans numbers strings NodePaths vectors colors and rectangles, plus resource references like {\"resource\": \"res://...\"} including scripts and null for object properties; arrays and dictionaries are not supported yet."`
	ExpectedScene string                     `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool                       `json:"save,omitempty" jsonschema:"description=Save the active scene after the change; save once when the edits to the scene are complete rather than after every change."`
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
	Save          bool                       `json:"save,omitempty" jsonschema:"description=Save the active scene after the change; save once when the edits to the scene are complete rather than after every change."`
}

type InstantiateParams struct {
	Scene         string                     `json:"scene" jsonschema:"minLength=1,description=Existing PackedScene resource path to instance, such as res://player.tscn. The source scene must be saved; instancing reads it from disk."`
	Parent        string                     `json:"parent" jsonschema:"minLength=1,description=Node path of the parent in the active editor scene; use a dot for the scene root."`
	Name          string                     `json:"name,omitempty" jsonschema:"minLength=1,description=Name for the instance root. When omitted the source scene root's name is used, uniquified. Fails if a sibling already uses the name."`
	Properties    map[string]json.RawMessage `json:"properties,omitempty" jsonschema:"description=Optional property overrides to apply to the instance root. Supports the same values as node.set. Set the script in the source scene instead of overriding it here."`
	ExpectedScene string                     `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool                       `json:"save,omitempty" jsonschema:"description=Save the active scene after the change; save once when the edits to the scene are complete rather than after every change."`
}

type InstantiateResult struct {
	Scene      string                     `json:"scene"`
	Source     string                     `json:"source" jsonschema:"description=Resource path of the instanced source scene."`
	Path       string                     `json:"path" jsonschema:"description=Node path of the instance root relative to the scene root."`
	Name       string                     `json:"name" jsonschema:"description=Actual node name; may be uniquified when no name was requested."`
	Type       string                     `json:"type" jsonschema:"description=Native Godot class of the instance root."`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Actual values after applying the requested overrides."`
	Saved      bool                       `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
}

type AddResult struct {
	Scene      string                     `json:"scene"`
	Path       string                     `json:"path" jsonschema:"description=Node path of the new node relative to the scene root."`
	Name       string                     `json:"name" jsonschema:"description=Actual node name; may be uniquified when no name was requested."`
	Type       string                     `json:"type" jsonschema:"description=Native Godot class of the new node."`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Actual values after applying the requested properties."`
	Saved      bool                       `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
}

type RenameParams struct {
	Node          string `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Name          string `json:"name" jsonschema:"minLength=1,description=New name for the node. Fails if a sibling already uses the name."`
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool   `json:"save,omitempty" jsonschema:"description=Save the active scene after the change; save once when the edits to the scene are complete rather than after every change."`
}

type RenameResult struct {
	Scene   string `json:"scene"`
	Path    string `json:"path" jsonschema:"description=Node path before the rename."`
	Name    string `json:"name" jsonschema:"description=Node name before the rename."`
	NewPath string `json:"new_path" jsonschema:"description=Node path after the rename."`
	NewName string `json:"new_name" jsonschema:"description=Node name after the rename."`
	Changed bool   `json:"changed" jsonschema:"description=Whether the node was renamed; false when the node already had the requested name."`
	Saved   bool   `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
}

type DuplicateParams struct {
	Node          string `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; the scene root cannot be duplicated."`
	Name          string `json:"name,omitempty" jsonschema:"minLength=1,description=Name for the copy. When omitted a unique name is generated from the original. Fails if a sibling already uses the name."`
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool   `json:"save,omitempty" jsonschema:"description=Save the active scene after the change; save once when the edits to the scene are complete rather than after every change."`
}

type DuplicateResult struct {
	Scene   string `json:"scene"`
	Path    string `json:"path" jsonschema:"description=Node path of the original node."`
	Name    string `json:"name" jsonschema:"description=Name of the original node."`
	NewPath string `json:"new_path" jsonschema:"description=Node path of the copy."`
	NewName string `json:"new_name" jsonschema:"description=Name of the copy; may be uniquified when no name was requested."`
	Type    string `json:"type" jsonschema:"description=Native Godot class of the copy."`
	Saved   bool   `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
}

type MoveParams struct {
	Node                string `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	Parent              string `json:"parent" jsonschema:"minLength=1,description=Node path of the new parent in the active editor scene; use a dot for the scene root."`
	Index               *int   `json:"index,omitempty" jsonschema:"description=Zero-based position among the new parent's children after the move. When omitted the node is moved to the end; use 0 for the first child."`
	KeepGlobalTransform *bool  `json:"keep_global_transform,omitempty" jsonschema:"description=Keep the node's global transform when its parent changes; defaults to true. Set false to keep its local transform instead."`
	ExpectedScene       string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save                bool   `json:"save,omitempty" jsonschema:"description=Save the active scene after the change; save once when the edits to the scene are complete rather than after every change."`
}

type MoveResult struct {
	Scene   string `json:"scene"`
	Path    string `json:"path" jsonschema:"description=Node path before the move."`
	Name    string `json:"name" jsonschema:"description=Node name before the move."`
	NewPath string `json:"new_path" jsonschema:"description=Node path after the move."`
	NewName string `json:"new_name" jsonschema:"description=Node name after the move; may be uniquified when the new parent already has a child with that name."`
	Changed bool   `json:"changed" jsonschema:"description=Whether the node was moved; false when it was already at the requested position."`
	Saved   bool   `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action."`
}

type DeleteParams struct {
	Node          string `json:"node" jsonschema:"minLength=1,description=Node path relative to the active editor scene root; use a dot for the root."`
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool   `json:"save,omitempty" jsonschema:"description=Save the active scene after the change; save once when the edits to the scene are complete rather than after every change."`
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

type Property = property.Property

type PropertyHint = property.Hint

type Layer = property.Layer
