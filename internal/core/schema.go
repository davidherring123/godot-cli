// Package core provides shared Godot project functionality and schemas.
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/davidherring123/godot-cli/internal/core/node"
	"github.com/davidherring123/godot-cli/internal/core/project/editor"
	"github.com/davidherring123/godot-cli/internal/core/project/input"
	"github.com/davidherring123/godot-cli/internal/core/project/runtime"
	"github.com/davidherring123/godot-cli/internal/core/project/settings"
	"github.com/davidherring123/godot-cli/internal/core/resource"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/davidherring123/godot-cli/internal/core/validation"
	"github.com/invopop/jsonschema"
)

// ActionDefinition describes one editor action that can be exposed through the
// CLI, the agent skill, and the MCP server.
type ActionDefinition struct {
	Name        string
	Description string
	Input       *jsonschema.Schema
	Output      *jsonschema.Schema
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
	Invoke      InvokeFunc
}

// InvokeFunc runs an action with raw JSON arguments. The arguments are decoded
// into the action's typed Params before the action function is called.
type InvokeFunc func(projectRoot string, input json.RawMessage) (any, error)

// ArgumentError reports that raw arguments could not be decoded into an
// action's typed parameter struct.
type ArgumentError struct {
	Action string
	Err    error
}

func (err *ArgumentError) Error() string {
	return fmt.Sprintf("%s: invalid arguments: %v", err.Action, err.Err)
}

func (err *ArgumentError) Unwrap() error {
	return err.Err
}

type actionHints struct {
	readOnly    bool
	destructive bool
	idempotent  bool
}

// actionOption adjusts the behavior hints attached to an action.
type actionOption func(*actionHints)

// withReadOnly marks an action as not modifying the project or editor state.
func withReadOnly() actionOption {
	return func(hints *actionHints) {
		hints.readOnly = true
		hints.destructive = false
	}
}

// withAdditive marks a mutating action as only adding to the project.
// Actions are destructive by default.
func withAdditive() actionOption {
	return func(hints *actionHints) {
		hints.destructive = false
	}
}

// withIdempotent marks an action as safe to retry without changing the result
// beyond its first successful call.
func withIdempotent() actionOption {
	return func(hints *actionHints) {
		hints.idempotent = true
	}
}

func define[P, R any](
	name string,
	description string,
	run func(string, P) (R, error),
	options ...actionOption,
) ActionDefinition {
	reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}

	// Mutations are destructive unless the action explicitly opts into
	// additive behavior.
	hints := actionHints{destructive: true}

	for _, option := range options {
		option(&hints)
	}

	return ActionDefinition{
		Name:        name,
		Description: description,
		Input:       reflector.ReflectFromType(reflect.TypeFor[P]()),
		Output:      reflector.ReflectFromType(reflect.TypeFor[R]()),
		ReadOnly:    hints.readOnly,
		Destructive: hints.destructive,
		Idempotent:  hints.readOnly || hints.idempotent,
		Invoke: func(projectRoot string, input json.RawMessage) (any, error) {
			var params P

			if arguments := bytes.TrimSpace(input); len(arguments) > 0 &&
				!bytes.Equal(arguments, []byte("null")) {
				decoder := json.NewDecoder(bytes.NewReader(arguments))
				decoder.DisallowUnknownFields()

				if err := decoder.Decode(&params); err != nil {
					return nil, &ArgumentError{Action: name, Err: err}
				}
			}

			return run(projectRoot, params)
		},
	}
}

var (
	EditorStatus = define[editor.StatusParams, editor.StatusResult](
		editor.StatusName,
		"Show whether a Godot editor with the godot-mcp addon is running for this project",
		editor.Status,
		withReadOnly(),
	)
	SceneList    = define[scene.ListParams, scene.ListResult](scene.ListName, "List scenes in the project", scene.List, withReadOnly())
	SceneCurrent = define[scene.CurrentParams, scene.CurrentResult](scene.CurrentName, "Show the active editor scene", scene.Current, withReadOnly())
	SceneOpen    = define[scene.OpenParams, scene.OpenResult](scene.OpenName, "Open or activate an editor scene", scene.Open, withAdditive())
	SceneCreate  = define[scene.CreateParams, scene.CreateResult](scene.CreateName, "Create a new scene file and open it in the editor", scene.Create, withAdditive())
	SceneSave    = define[scene.SaveParams, scene.SaveResult](scene.SaveName, "Save the active editor scene", scene.Save)
	SceneTree    = define[scene.TreeParams, scene.TreeResult](scene.TreeName, "Show the active editor scene's node tree", scene.Tree, withReadOnly())
	SceneApply   = define[scene.ApplyParams, scene.ApplyResult](
		scene.ApplyName,
		"Apply 1-100 ordered node operations to the active scene as one atomic undo step, with best-effort prevalidation, optional dry_run, and deferred history registration",
		scene.Apply,
	)
	ValidateScene = define[validate.SceneParams, validate.SceneResult](
		validate.SceneName,
		"Validate the active editor scene, including unsaved changes, without saving, opening, or switching scenes",
		validate.Scene,
		withReadOnly(),
	)
	NodeInspect     = define[node.InspectParams, node.InspectResult](node.InspectName, "Inspect a node in the active editor scene", node.Inspect, withReadOnly())
	NodeSet         = define[node.SetParams, node.SetResult](node.SetName, "Set node properties as one undoable editor action", node.Set)
	NodeAdd         = define[node.AddParams, node.AddResult](node.AddName, "Add a node to the active editor scene as one undoable editor action", node.Add, withAdditive())
	NodeInstantiate = define[node.InstantiateParams, node.InstantiateResult](
		node.InstantiateName,
		"Add a linked instance of a PackedScene to the active editor scene as one undoable editor action",
		node.Instantiate,
		withAdditive(),
	)
	NodeRename     = define[node.RenameParams, node.RenameResult](node.RenameName, "Rename a node as one undoable editor action", node.Rename)
	NodeDuplicate  = define[node.DuplicateParams, node.DuplicateResult](node.DuplicateName, "Duplicate a node and its children as one undoable editor action", node.Duplicate, withAdditive())
	NodeMove       = define[node.MoveParams, node.MoveResult](node.MoveName, "Move a node to a new parent or position as one undoable editor action", node.Move)
	NodeDelete     = define[node.DeleteParams, node.DeleteResult](node.DeleteName, "Delete a node from the active editor scene as one undoable editor action", node.Delete)
	ResourceCreate = define[resource.CreateParams, resource.CreateResult](
		resource.CreateName,
		"Create a resource file and write it to disk immediately",
		resource.Create,
		withAdditive(),
	)
	ResourceInspect = define[resource.InspectParams, resource.InspectResult](
		resource.InspectName,
		"Inspect a resource file's editor-visible properties",
		resource.Inspect,
		withReadOnly(),
	)
	ResourceSet = define[resource.SetParams, resource.SetResult](
		resource.SetName,
		"Set resource file properties and save the file immediately",
		resource.Set,
	)
	ResourceDelete = define[resource.DeleteParams, resource.DeleteResult](
		resource.DeleteName,
		"Delete a resource file that nothing else references",
		resource.Delete,
	)
	ResourceApply = define[resource.ApplyParams, resource.ApplyResult](
		resource.ApplyName,
		"Apply ordered resource create, set, and delete operations with in-order validation, optional dry_run, and best-effort rollback of this domain's writes on failure",
		resource.Apply,
	)
	ProjectSettingsList = define[settings.ListParams, settings.ListResult](
		settings.ListName,
		"List editor-visible project settings",
		settings.List,
		withReadOnly(),
	)
	ProjectSettingsGet = define[settings.GetParams, settings.GetResult](
		settings.GetName,
		"Show one project setting with its default and override state",
		settings.Get,
		withReadOnly(),
	)
	ProjectSettingsSet = define[settings.SetParams, settings.SetResult](
		settings.SetName,
		"Set a project setting and save project.godot immediately",
		settings.Set,
	)
	ProjectSettingsUnset = define[settings.UnsetParams, settings.UnsetResult](
		settings.UnsetName,
		"Remove a project setting override and save project.godot immediately",
		settings.Unset,
	)
	ProjectSettingsApply = define[settings.ApplyParams, settings.ApplyResult](
		settings.ApplyName,
		"Apply ordered project-setting set and unset operations with in-order validation, optional dry_run, and a single project.godot save with best-effort rollback on save failure",
		settings.Apply,
	)
	InputList        = define[input.ListParams, input.ListResult](input.ListName, "List Input Map actions", input.List, withReadOnly())
	InputInspect     = define[input.InspectParams, input.InspectResult](input.InspectName, "Show one Input Map action and its bindings", input.Inspect, withReadOnly())
	InputAdd         = define[input.AddParams, input.AddResult](input.AddName, "Add an Input Map action and save project.godot immediately", input.Add, withAdditive())
	InputSet         = define[input.SetParams, input.SetResult](input.SetName, "Set an Input Map action's deadzone and save project.godot immediately", input.Set)
	InputRename      = define[input.RenameParams, input.RenameResult](input.RenameName, "Rename an Input Map action and save project.godot immediately", input.Rename)
	InputDelete      = define[input.DeleteParams, input.DeleteResult](input.DeleteName, "Delete an Input Map action and save project.godot immediately", input.Delete)
	InputEventAdd    = define[input.EventAddParams, input.EventAddResult](input.EventAddName, "Add a binding to an Input Map action and save project.godot immediately", input.EventAdd, withAdditive())
	InputEventDelete = define[input.EventDeleteParams, input.EventDeleteResult](input.EventDeleteName, "Remove a binding from an Input Map action and save project.godot immediately", input.EventDelete)
	InputMapApply    = define[input.ApplyParams, input.ApplyResult](
		input.ApplyName,
		"Apply ordered Input Map action and binding mutations with in-order validation, optional dry_run, and a single project.godot save with best-effort rollback on save failure",
		input.Apply,
	)
	RuntimeStart = define[runtime.StartParams, runtime.StatusResult](
		runtime.StartName,
		"Start the project's game process, wait until its scene is ready, and return the runtime session",
		runtime.Start,
		withAdditive(),
	)
	RuntimeStatus = define[runtime.StatusParams, runtime.StatusResult](
		runtime.StatusName,
		"Show the active or most recent runtime session, including errors, frames, and artifacts",
		runtime.Status,
		withReadOnly(),
	)
	RuntimeCapture = define[runtime.CaptureParams, runtime.CaptureResult](
		runtime.CaptureName,
		"Capture a PNG screenshot from the running game; the MCP result includes the image when possible",
		runtime.Capture,
		withAdditive(),
	)
	RuntimeStop = define[runtime.StopParams, runtime.StopResult](
		runtime.StopName,
		"Stop the runtime session, waiting for a graceful exit before killing the game process",
		runtime.Stop,
		withIdempotent(),
	)
)

func All() []ActionDefinition {
	return []ActionDefinition{
		EditorStatus,
		SceneList,
		SceneCurrent,
		SceneOpen,
		SceneCreate,
		SceneSave,
		SceneTree,
		SceneApply,
		ValidateScene,
		NodeInspect,
		NodeSet,
		NodeAdd,
		NodeInstantiate,
		NodeRename,
		NodeDuplicate,
		NodeMove,
		NodeDelete,
		ResourceCreate,
		ResourceInspect,
		ResourceSet,
		ResourceDelete,
		ResourceApply,
		ProjectSettingsList,
		ProjectSettingsGet,
		ProjectSettingsSet,
		ProjectSettingsUnset,
		ProjectSettingsApply,
		InputList,
		InputInspect,
		InputAdd,
		InputSet,
		InputRename,
		InputDelete,
		InputEventAdd,
		InputEventDelete,
		InputMapApply,
		RuntimeStart,
		RuntimeStatus,
		RuntimeCapture,
		RuntimeStop,
	}
}

func Lookup(name string) (ActionDefinition, bool) {
	for _, definition := range All() {
		if definition.Name == name {
			return definition, true
		}
	}
	return ActionDefinition{}, false
}
