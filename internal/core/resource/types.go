package resource

import (
	"encoding/json"

	"github.com/davidherring123/godot-cli/internal/core/property"
)

type CreateParams struct {
	Resource   string                     `json:"resource" jsonschema:"minLength=1,description=New resource file path such as res://materials/metal.tres, ending in .tres or .res. Fails if the file already exists."`
	Type       string                     `json:"type" jsonschema:"minLength=1,description=Resource class to instantiate, such as StandardMaterial3D. Global script classes are supported."`
	Properties map[string]json.RawMessage `json:"properties,omitempty" jsonschema:"description=Optional property values to apply when the resource is created. Supports the same values as node.set, including resource references."`
}

type CreateResult struct {
	Resource   string                     `json:"resource" jsonschema:"description=Resource path that was written."`
	Type       string                     `json:"type" jsonschema:"description=Actual resource type; the global script class name when a script is attached."`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Actual property values after applying the requested properties."`
}

type InspectParams struct {
	Resource   string   `json:"resource" jsonschema:"minLength=1,description=Existing resource file path such as res://materials/metal.tres."`
	Properties []string `json:"properties,omitempty" jsonschema:"description=Property names to include; all editor-visible properties are returned when omitted."`
}

type InspectResult struct {
	Resource   string              `json:"resource"`
	Type       string              `json:"type" jsonschema:"description=Actual resource type; the global script class name when a script is attached."`
	Properties []property.Property `json:"properties"`
}

type SetParams struct {
	Resource   string                     `json:"resource" jsonschema:"minLength=1,description=Existing resource file path such as res://materials/metal.tres."`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Nonempty object of property names to JSON values. Supports the same values as node.set, including resource references; the file is saved immediately."`
}

type SetResult struct {
	Resource   string                     `json:"resource"`
	Type       string                     `json:"type" jsonschema:"description=Actual resource type; the global script class name when a script is attached."`
	Properties map[string]json.RawMessage `json:"properties" jsonschema:"description=Actual values after applying the requested properties."`
	Changed    bool                       `json:"changed" jsonschema:"description=Whether the resource file changed; false when the values already matched."`
}

type DeleteParams struct {
	Resource string `json:"resource" jsonschema:"minLength=1,description=Existing resource file path such as res://materials/metal.tres. Fails while other project files still reference the resource."`
}

type DeleteResult struct {
	Resource string `json:"resource"`
	Deleted  bool   `json:"deleted" jsonschema:"description=Whether the resource file was removed."`
}

// ApplyOperation is one ordered resource mutation in a resource.apply batch.
// Operation selects which fields apply:
//
//   - create: resource, type, and optional properties
//   - set: resource and a nonempty properties object
//   - delete: resource
type ApplyOperation struct {
	Operation  string                     `json:"operation" jsonschema:"enum=create,enum=set,enum=delete,description=Operation kind."`
	Resource   string                     `json:"resource" jsonschema:"minLength=1,description=Resource file path such as res://materials/metal.tres."`
	Type       string                     `json:"type,omitempty" jsonschema:"description=Resource class for create, such as StandardMaterial3D."`
	Properties map[string]json.RawMessage `json:"properties,omitempty" jsonschema:"description=Property values for create or set; uses the same JSON values as resource.create and resource.set."`
}

type ApplyParams struct {
	Operations []ApplyOperation `json:"operations" jsonschema:"minItems=1,maxItems=100,description=Ordered resource operations to validate and apply."`
	DryRun     bool             `json:"dry_run,omitempty" jsonschema:"description=Validate the full batch without writing any file."`
}

type ApplyOperationSummary struct {
	Index     int    `json:"index" jsonschema:"description=Zero-based position of the operation in the batch."`
	Operation string `json:"operation"`
	Resource  string `json:"resource"`
	Status    string `json:"status" jsonschema:"description=Outcome such as created, updated, unchanged, or deleted."`
	Changed   bool   `json:"changed"`
	Type      string `json:"type,omitempty" jsonschema:"description=Actual resource type for create and set."`
}

type ApplyResult struct {
	DryRun         bool                    `json:"dry_run"`
	Applied        bool                    `json:"applied" jsonschema:"description=Whether the batch committed; false for a dry run."`
	OperationCount int                     `json:"operation_count"`
	Changed        int                     `json:"changed" jsonschema:"description=Number of operations that changed the resource state."`
	Unchanged      int                     `json:"unchanged"`
	Operations     []ApplyOperationSummary `json:"operations"`
}
