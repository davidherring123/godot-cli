// Package core provides shared Godot project functionality and schemas.
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/davidherring123/godot-cli/internal/core/input"
	"github.com/davidherring123/godot-cli/internal/core/node"
	"github.com/davidherring123/godot-cli/internal/core/project/settings"
	"github.com/davidherring123/godot-cli/internal/core/resource"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/davidherring123/godot-cli/internal/core/validate"
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
}

// ActionOption adjusts the behavior hints attached to an action.
type ActionOption func(*actionHints)

// WithReadOnly marks an action as not modifying the project or editor state.
func WithReadOnly() ActionOption {
	return func(hints *actionHints) {
		hints.readOnly = true
		hints.destructive = false
	}
}

// WithAdditive marks a mutating action as only adding to the project.
func WithAdditive() ActionOption {
	return func(hints *actionHints) {
		hints.destructive = false
	}
}

// WithDestructive marks an action as potentially overwriting or deleting data.
func WithDestructive() ActionOption {
	return func(hints *actionHints) {
		hints.destructive = true
	}
}

func define[P, R any](
	name string,
	description string,
	run func(string, P) (R, error),
	options ...ActionOption,
) ActionDefinition {
	reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}
	hints := actionHints{destructive: true}

	for _, option := range options {
		option(&hints)
	}

	if hints.readOnly {
		hints.destructive = false
	}

	return ActionDefinition{
		Name:        name,
		Description: description,
		Input:       reflector.ReflectFromType(reflect.TypeFor[P]()),
		Output:      reflector.ReflectFromType(reflect.TypeFor[R]()),
		ReadOnly:    hints.readOnly,
		Destructive: hints.destructive,
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

// ToolName converts an action name into the underscore-separated form used by
// MCP tools.
func ToolName(action string) string {
	return strings.ReplaceAll(action, ".", "_")
}

var (
	SceneList    = define[scene.ListParams, scene.ListResult](scene.ListName, "List scenes in the project", scene.List, WithReadOnly())
	SceneCurrent = define[scene.CurrentParams, scene.CurrentResult](scene.CurrentName, "Show the active editor scene", scene.Current, WithReadOnly())
	SceneOpen    = define[scene.OpenParams, scene.OpenResult](scene.OpenName, "Open or activate an editor scene", scene.Open, WithAdditive())
	SceneCreate  = define[scene.CreateParams, scene.CreateResult](scene.CreateName, "Create a new scene file and open it in the editor", scene.Create, WithAdditive())
	SceneSave    = define[scene.SaveParams, scene.SaveResult](scene.SaveName, "Save the active editor scene", scene.Save, WithDestructive())
	SceneTree    = define[scene.TreeParams, scene.TreeResult](scene.TreeName, "Show the active editor scene's node tree", scene.Tree, WithReadOnly())
	ValidateLive = define[validate.LiveParams, validate.LiveResult](
		validate.LiveName,
		"Validate the active editor scene, including unsaved state",
		validate.Live,
		WithReadOnly(),
	)
	ValidateScene = define[validate.SceneParams, validate.RunResult](
		validate.SceneName,
		"Validate a scene by loading and running it briefly in a fresh headless Godot process",
		validate.Scene,
		WithReadOnly(),
	)
	NodeInspect     = define[node.InspectParams, node.InspectResult](node.InspectName, "Inspect a node in the active editor scene", node.Inspect, WithReadOnly())
	NodeSet         = define[node.SetParams, node.SetResult](node.SetName, "Set node properties as one undoable editor action", node.Set, WithDestructive())
	NodeAdd         = define[node.AddParams, node.AddResult](node.AddName, "Add a node to the active editor scene as one undoable editor action", node.Add, WithAdditive())
	NodeInstantiate = define[node.InstantiateParams, node.InstantiateResult](
		node.InstantiateName,
		"Add a linked instance of a PackedScene to the active editor scene as one undoable editor action",
		node.Instantiate,
		WithAdditive(),
	)
	NodeRename     = define[node.RenameParams, node.RenameResult](node.RenameName, "Rename a node as one undoable editor action", node.Rename, WithDestructive())
	NodeDuplicate  = define[node.DuplicateParams, node.DuplicateResult](node.DuplicateName, "Duplicate a node and its children as one undoable editor action", node.Duplicate, WithAdditive())
	NodeMove       = define[node.MoveParams, node.MoveResult](node.MoveName, "Move a node to a new parent or position as one undoable editor action", node.Move, WithDestructive())
	NodeDelete     = define[node.DeleteParams, node.DeleteResult](node.DeleteName, "Delete a node from the active editor scene as one undoable editor action", node.Delete, WithDestructive())
	ResourceCreate = define[resource.CreateParams, resource.CreateResult](
		resource.CreateName,
		"Create a resource file and write it to disk immediately",
		resource.Create,
		WithAdditive(),
	)
	ResourceInspect = define[resource.InspectParams, resource.InspectResult](
		resource.InspectName,
		"Inspect a resource file's editor-visible properties",
		resource.Inspect,
		WithReadOnly(),
	)
	ResourceSet = define[resource.SetParams, resource.SetResult](
		resource.SetName,
		"Set resource file properties and save the file immediately",
		resource.Set,
		WithDestructive(),
	)
	ResourceDelete = define[resource.DeleteParams, resource.DeleteResult](
		resource.DeleteName,
		"Delete a resource file that nothing else references",
		resource.Delete,
		WithDestructive(),
	)
	ProjectSettingsList = define[settings.ListParams, settings.ListResult](
		settings.ListName,
		"List editor-visible project settings",
		settings.List,
		WithReadOnly(),
	)
	ProjectSettingsGet = define[settings.GetParams, settings.GetResult](
		settings.GetName,
		"Show one project setting with its default and override state",
		settings.Get,
		WithReadOnly(),
	)
	ProjectSettingsSet = define[settings.SetParams, settings.SetResult](
		settings.SetName,
		"Set a project setting and save project.godot immediately",
		settings.Set,
		WithDestructive(),
	)
	ProjectSettingsUnset = define[settings.UnsetParams, settings.UnsetResult](
		settings.UnsetName,
		"Remove a project setting override and save project.godot immediately",
		settings.Unset,
		WithDestructive(),
	)
	InputList        = define[input.ListParams, input.ListResult](input.ListName, "List Input Map actions", input.List, WithReadOnly())
	InputInspect     = define[input.InspectParams, input.InspectResult](input.InspectName, "Show one Input Map action and its bindings", input.Inspect, WithReadOnly())
	InputAdd         = define[input.AddParams, input.AddResult](input.AddName, "Add an Input Map action and save project.godot immediately", input.Add, WithAdditive())
	InputSet         = define[input.SetParams, input.SetResult](input.SetName, "Set an Input Map action's deadzone and save project.godot immediately", input.Set, WithDestructive())
	InputRename      = define[input.RenameParams, input.RenameResult](input.RenameName, "Rename an Input Map action and save project.godot immediately", input.Rename, WithDestructive())
	InputDelete      = define[input.DeleteParams, input.DeleteResult](input.DeleteName, "Delete an Input Map action and save project.godot immediately", input.Delete, WithDestructive())
	InputEventAdd    = define[input.EventAddParams, input.EventAddResult](input.EventAddName, "Add a binding to an Input Map action and save project.godot immediately", input.EventAdd, WithAdditive())
	InputEventDelete = define[input.EventDeleteParams, input.EventDeleteResult](input.EventDeleteName, "Remove a binding from an Input Map action and save project.godot immediately", input.EventDelete, WithDestructive())
)

func All() []ActionDefinition {
	return []ActionDefinition{
		SceneList,
		SceneCurrent,
		SceneOpen,
		SceneCreate,
		SceneSave,
		SceneTree,
		ValidateLive,
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
		ProjectSettingsList,
		ProjectSettingsGet,
		ProjectSettingsSet,
		ProjectSettingsUnset,
		InputList,
		InputInspect,
		InputAdd,
		InputSet,
		InputRename,
		InputDelete,
		InputEventAdd,
		InputEventDelete,
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
