// Package property describes Godot object property metadata shared by the node,
// resource, and project settings domains.
package property

import "encoding/json"

type Property struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Hint  *Hint           `json:"hint,omitempty"`
	Value json.RawMessage `json:"value" jsonschema:"description=Serialized Godot Variant; its JSON shape depends on the property type."`
}

type Hint struct {
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
