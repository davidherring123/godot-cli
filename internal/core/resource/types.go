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
