// Package validate checks that Godot scenes load and run without errors.
package validate

import "time"

const (
	LiveName  = "validate.live"
	SceneName = "validate.scene"
)

type LiveParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type LiveResult struct {
	Valid   bool     `json:"valid" jsonschema:"description=Whether the active scene packed and instanced without script resource or serialization errors."`
	Scene   string   `json:"scene" jsonschema:"description=Active scene resource path; empty when the scene has never been saved."`
	Unsaved bool     `json:"unsaved" jsonschema:"description=Whether the active scene has unsaved changes."`
	Errors  []string `json:"errors" jsonschema:"description=Validation errors; empty when valid."`
}

type SceneParams struct {
	Scene          string  `json:"scene,omitempty" jsonschema:"description=Scene resource path such as res://levels/arena.tscn. Defaults to the project's configured main scene."`
	TimeoutSeconds float64 `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum seconds the headless run may take before Godot is stopped; defaults to 15."`
}

type RunParams struct {
	Scene   string
	Timeout time.Duration
}

type RunResult struct {
	Valid  bool     `json:"valid" jsonschema:"description=Whether the scene loaded, instantiated, and ran without errors."`
	Scene  string   `json:"scene" jsonschema:"description=Scene resource path that was validated."`
	Errors []string `json:"errors" jsonschema:"description=Validation errors; empty when valid."`
}
