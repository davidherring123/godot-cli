// Package validate checks the active editor scene for script and serialization errors.
package validate

const SceneName = "validate.scene"

type SceneParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type SceneResult struct {
	Valid   bool     `json:"valid" jsonschema:"description=Whether the active scene packed and instanced without script resource or serialization errors."`
	Scene   string   `json:"scene" jsonschema:"description=Active scene resource path; empty when the scene has never been saved."`
	Unsaved bool     `json:"unsaved" jsonschema:"description=Whether the active scene has unsaved changes."`
	Errors  []string `json:"errors" jsonschema:"description=Validation errors; empty when valid."`
}
