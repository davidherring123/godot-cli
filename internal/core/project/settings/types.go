// Package settings inspects and edits project.godot settings.
package settings

import (
	"encoding/json"

	"github.com/davidherring123/godot-cli/internal/core/property"
)

type ListParams struct {
	Prefix string `json:"prefix,omitempty" jsonschema:"description=Only include settings whose name starts with this prefix, such as display/window."`
}

type ListResult struct {
	Settings []Setting `json:"settings"`
}

type Setting struct {
	Name       string          `json:"name"`
	Type       string          `json:"type" jsonschema:"description=Godot Variant type name of the setting."`
	Value      json.RawMessage `json:"value" jsonschema:"description=Current effective value; the engine default when the project does not override it."`
	Default    json.RawMessage `json:"default" jsonschema:"description=Engine default value."`
	Overridden bool            `json:"overridden" jsonschema:"description=Whether project.godot overrides the engine default."`
	Hint       *property.Hint  `json:"hint,omitempty"`
}

type GetParams struct {
	Setting string `json:"setting" jsonschema:"minLength=1,description=Project setting name such as display/window/size/viewport_width."`
}

type GetResult = Setting

type SetParams struct {
	Setting string          `json:"setting" jsonschema:"minLength=1,description=Project setting name such as display/window/size/viewport_width."`
	Value   json.RawMessage `json:"value" jsonschema:"description=JSON value matching the setting's Godot type."`
}

type SetResult struct {
	Setting    string          `json:"setting"`
	Type       string          `json:"type"`
	Value      json.RawMessage `json:"value" jsonschema:"description=Value after the change."`
	Default    json.RawMessage `json:"default"`
	Overridden bool            `json:"overridden"`
	Changed    bool            `json:"changed" jsonschema:"description=Whether project.godot changed; false when the value already matched."`
}

type UnsetParams struct {
	Setting string `json:"setting" jsonschema:"minLength=1,description=Project setting name to restore to its engine default."`
}

type UnsetResult struct {
	Setting    string          `json:"setting"`
	Type       string          `json:"type"`
	Value      json.RawMessage `json:"value" jsonschema:"description=Value after removing the override; the engine default."`
	Default    json.RawMessage `json:"default"`
	Overridden bool            `json:"overridden" jsonschema:"description=Always false after a successful unset."`
	Changed    bool            `json:"changed" jsonschema:"description=Whether project.godot changed; false when the setting was already using the engine default."`
}

// ApplyOperation is one ordered project-setting mutation. Operation selects
// which fields apply:
//
//   - set: setting and value
//   - unset: setting
type ApplyOperation struct {
	Operation string          `json:"operation" jsonschema:"enum=set,enum=unset,description=Operation kind."`
	Setting   string          `json:"setting" jsonschema:"minLength=1,description=Project setting name such as display/window/size/viewport_width."`
	Value     json.RawMessage `json:"value,omitempty" jsonschema:"description=JSON value for set, matching the setting's Godot type."`
}

type ApplyParams struct {
	Operations []ApplyOperation `json:"operations" jsonschema:"minItems=1,maxItems=100,description=Ordered project-setting operations to validate and apply."`
	DryRun     bool             `json:"dry_run,omitempty" jsonschema:"description=Validate the full batch without saving project.godot. Override state is simulated in memory, so later unset operations see earlier sets in the same batch."`
}

type ApplyOperationSummary struct {
	Index     int    `json:"index" jsonschema:"description=Zero-based position of the operation in the batch."`
	Operation string `json:"operation"`
	Setting   string `json:"setting"`
	Status    string `json:"status" jsonschema:"description=Outcome such as updated, unset, or unchanged."`
	Changed   bool   `json:"changed"`
	Type      string `json:"type,omitempty"`
}

type ApplyResult struct {
	DryRun         bool                    `json:"dry_run"`
	Applied        bool                    `json:"applied" jsonschema:"description=Whether the batch ran in apply mode; false for a dry run. An all-no-op batch does not rewrite project.godot."`
	OperationCount int                     `json:"operation_count"`
	Changed        int                     `json:"changed" jsonschema:"description=Number of operations that changed a setting."`
	Unchanged      int                     `json:"unchanged"`
	Operations     []ApplyOperationSummary `json:"operations"`
}
