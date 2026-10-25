package scene

import (
	"bytes"
	"encoding/json"
)

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

type ApplyParams struct {
	Operations    []ApplyOperation `json:"operations" jsonschema:"minItems=1,maxItems=100,description=Ordered operations to apply as one atomic editor undo step. Every operation is validated on a temporary scene copy before the live scene is edited. A batch may contain at most 100 operations."`
	DryRun        bool             `json:"dry_run,omitempty" jsonschema:"description=Validate and plan the batch on a temporary scene copy without modifying the live editor scene. The preview never writes remapped paths into resources or animation tracks shared with the live scene. This is still best-effort topology validation: a packed-scene copy omits editor-only and unowned runtime nodes, so the live pass can still fail and roll back. Cannot be combined with save."`
	ExpectedScene string           `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
	Save          bool             `json:"save,omitempty" jsonschema:"description=Save the active scene once after every operation succeeds. Not valid with dry_run. If the save fails, the batch remains applied and undoable."`
}

type ApplyOperation struct {
	Operation           string                     `json:"operation" jsonschema:"enum=node.add,enum=node.set,enum=node.rename,enum=node.duplicate,enum=node.move,enum=node.delete,enum=node.instantiate,description=Operation to run."`
	ID                  string                     `json:"id,omitempty" jsonschema:"minLength=1,description=Optional unique id for this operation. Later operations can reference the resulting node as @id. An id cannot be referenced after its node is deleted."`
	Node                string                     `json:"node,omitempty" jsonschema:"minLength=1,description=Target node path for node.set / node.rename / node.duplicate / node.move / node.delete. Use a dot for the scene root or @id for a node produced earlier in the batch."`
	Parent              string                     `json:"parent,omitempty" jsonschema:"minLength=1,description=Parent node path for node.add / node.instantiate or the destination parent for node.move. Use a dot for the scene root or @id for a node produced earlier in the batch."`
	Type                string                     `json:"type,omitempty" jsonschema:"minLength=1,description=Node class for node.add such as Sprite2D; global script classes are supported."`
	Name                string                     `json:"name,omitempty" jsonschema:"minLength=1,description=Requested or new node name for node.add / node.instantiate / node.rename / node.duplicate."`
	Scene               string                     `json:"scene,omitempty" jsonschema:"minLength=1,description=Source PackedScene path for node.instantiate such as res://player.tscn."`
	Properties          map[string]json.RawMessage `json:"properties,omitempty" jsonschema:"description=Property values for node.set / node.add / node.instantiate using the same JSON shapes as the matching granular action."`
	Index               *int                       `json:"index,omitempty" jsonschema:"description=Zero-based destination position among the new parent's children for node.move; omit to move the node to the end."`
	KeepGlobalTransform *bool                      `json:"keep_global_transform,omitempty" jsonschema:"description=For node.move keep the node's global transform when its parent changes; defaults to true."`

	// present records which JSON fields the caller supplied. It lets
	// validation reject an explicit empty id or an empty properties object
	// for an operation that does not accept properties.
	present map[string]bool
}

// UnmarshalJSON records which fields were present and keeps unknown-field
// rejection working for both CLI and MCP callers.
func (operation *ApplyOperation) UnmarshalJSON(data []byte) error {
	type operationAlias ApplyOperation

	var decoded operationAlias

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&decoded); err != nil {
		return err
	}

	fields := make(map[string]json.RawMessage)

	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	*operation = ApplyOperation(decoded)
	operation.present = make(map[string]bool, len(fields))

	for field := range fields {
		operation.present[field] = true
	}

	return nil
}

// HasField reports whether the caller supplied a JSON field. Values built in Go
// code fall back to non-zero checks because they have no wire presence data.
func (operation ApplyOperation) HasField(field string) bool {
	if operation.present != nil {
		return operation.present[field]
	}

	switch field {
	case "operation":
		return operation.Operation != ""
	case "id":
		return operation.ID != ""
	case "node":
		return operation.Node != ""
	case "parent":
		return operation.Parent != ""
	case "type":
		return operation.Type != ""
	case "name":
		return operation.Name != ""
	case "scene":
		return operation.Scene != ""
	case "properties":
		return operation.Properties != nil
	case "index":
		return operation.Index != nil
	case "keep_global_transform":
		return operation.KeepGlobalTransform != nil
	default:
		return false
	}
}

type ApplyOperationResult struct {
	Index     int    `json:"index" jsonschema:"description=Zero-based position of the operation in the request."`
	ID        string `json:"id,omitempty" jsonschema:"description=Operation id when one was supplied."`
	Operation string `json:"operation"`
	Path      string `json:"path,omitempty" jsonschema:"description=Path snapshot at the moment this operation completed. Later operations may rename, move, or delete the node, so this is not necessarily the final path."`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
	Changed   bool   `json:"changed" jsonschema:"description=Whether this operation changed the scene."`
	Summary   string `json:"summary" jsonschema:"description=Compact one-line description of the result."`
}

type ApplySummary struct {
	Total   int `json:"total"`
	Changed int `json:"changed"`
	Added   int `json:"added" jsonschema:"description=Operations that added or duplicated a node."`
	Deleted int `json:"deleted"`
}

type ApplyResult struct {
	Scene      string                 `json:"scene"`
	Applied    bool                   `json:"applied" jsonschema:"description=Whether the batch was committed to the editor; false for dry_run."`
	DryRun     bool                   `json:"dry_run"`
	Operations []ApplyOperationResult `json:"operations"`
	Summary    ApplySummary           `json:"summary"`
	Saved      bool                   `json:"saved" jsonschema:"description=Whether the scene was saved as part of this action. A save failure is reported as an error and leaves the applied batch undoable; retry with scene.save instead of re-running the batch."`
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
