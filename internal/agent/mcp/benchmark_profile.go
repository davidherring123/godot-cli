package mcp

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/core"
)

const (
	BenchmarkProfileLegacy   = "legacy-mcp"
	BenchmarkProfileImproved = "improved-mcp"
)

var legacyExcludedActions = map[string]bool{
	"scene.apply":            true,
	"resource.apply":         true,
	"project.settings.apply": true,
	"input.map_apply":        true,
	"runtime.start":          true,
	"runtime.status":         true,
	"runtime.capture":        true,
	"runtime.stop":           true,
}

// BenchmarkProfile describes an explicitly selected benchmark-only MCP surface.
// Production callers that do not select a profile continue to receive core.All
// and the current default instructions.
type BenchmarkProfile struct {
	Name            string
	Actions         []core.ActionDefinition
	Instructions    string
	ExcludedActions []string
}

// ResolveBenchmarkProfile returns the controlled MCP surface used by the A/B
// benchmark harness. It is exported so the harness can persist an exact
// manifest alongside each result.
func ResolveBenchmarkProfile(name, projectRoot string) (BenchmarkProfile, error) {
	profile := BenchmarkProfile{Name: name}

	switch name {
	case BenchmarkProfileLegacy:
		for _, action := range core.All() {
			if legacyExcludedActions[action.Name] {
				profile.ExcludedActions = append(profile.ExcludedActions, action.Name)
				continue
			}
			profile.Actions = append(profile.Actions, action)
		}
		profile.Instructions = legacyInstructions(projectRoot)
	case BenchmarkProfileImproved:
		profile.Actions = core.All()
		profile.Instructions = defaultInstructions(projectRoot)
	default:
		return BenchmarkProfile{}, fmt.Errorf(
			"unknown MCP benchmark profile %q (supported: %s, %s)",
			name,
			BenchmarkProfileLegacy,
			BenchmarkProfileImproved,
		)
	}

	return profile, nil
}

// legacyInstructions is the pre-improvement guidance from commit 8ae1d8f. It
// intentionally does not mention batch or runtime tools excluded by the
// legacy-mcp profile.
func legacyInstructions(projectRoot string) string {
	projectDescription := "the current Godot project"
	if projectRoot != "" {
		projectDescription = "the Godot project at " + projectRoot
	}

	return "This server inspects and edits " + projectDescription + ". Most tools use a running Godot 4 editor; " +
		"scene_list reads saved files and validate_scene uses a separate headless Godot process.\n\n" +
		"Prefer these tools for supported semantic inspection and mutation of scenes, nodes, resources, " +
		"project settings, and the Input Map. Do not parse or rewrite .tscn, .tres, or project.godot files " +
		"by hand when a tool covers the change. Use file tools for GDScript and shader source, " +
		"repository-wide searches, line-numbered reads, and raw serialization details the tools do not expose. " +
		"Use Git for history, diffs, and blame.\n\n" +
		"Active-scene tools read the live editor's current scene, including unsaved changes; file reads see " +
		"only saved contents. scene_current reports unsaved state, and scene_save or save=true writes the scene. " +
		"Node mutations are undoable as one step per call; set save=true once after a batch rather than after " +
		"every edit. scene_create writes a new scene file immediately.\n\n" +
		"Resource, project-setting, and Input Map mutations write their files immediately and are not editor undo " +
		"steps. A mutation response reports the values that were applied and, " +
		"for active-scene edits, whether the scene was saved; it does not confirm gameplay or runtime behavior. " +
		"Use validation for load, script, and serialization checks and run the project for gameplay. Do not " +
		"inspect again after a successful mutation unless the response omitted values you need or you are " +
		"answering a different question.\n\n" +
		"Use the narrowest tool directly when the target is known; skip redundant status and discovery calls. " +
		"Live tools require a running Godot 4 editor with the godot-cli addon. If no editor is running, " +
		"editor_open launches or reuses an editor; retry the tool afterward. If the addon is missing, run `godot-cli init` once in the " +
		"project and restart Godot. Tool results are JSON."
}
