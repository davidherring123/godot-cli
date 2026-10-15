// Package core provides shared Godot project functionality and schemas.
package core

import (
	"reflect"

	"github.com/davidherring123/godot-cli/internal/core/input"
	"github.com/davidherring123/godot-cli/internal/core/node"
	"github.com/davidherring123/godot-cli/internal/core/project/settings"
	"github.com/davidherring123/godot-cli/internal/core/resource"
	"github.com/davidherring123/godot-cli/internal/core/scene"
	"github.com/invopop/jsonschema"
)

type ActionDefinition struct {
	Name        string
	Description string
	Input       *jsonschema.Schema
	Output      *jsonschema.Schema
}

func define[P, R any](name, description string) ActionDefinition {
	reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}
	return ActionDefinition{
		Name:        name,
		Description: description,
		Input:       reflector.ReflectFromType(reflect.TypeFor[P]()),
		Output:      reflector.ReflectFromType(reflect.TypeFor[R]()),
	}
}

var (
	SceneList       = define[scene.ListParams, scene.ListResult](scene.ListName, "List scenes in the project")
	SceneCurrent    = define[scene.CurrentParams, scene.CurrentResult](scene.CurrentName, "Show the active editor scene")
	SceneOpen       = define[scene.OpenParams, scene.OpenResult](scene.OpenName, "Open or activate an editor scene")
	SceneCreate     = define[scene.CreateParams, scene.CreateResult](scene.CreateName, "Create a new scene file and open it in the editor")
	SceneSave       = define[scene.SaveParams, scene.SaveResult](scene.SaveName, "Save the active editor scene")
	SceneTree       = define[scene.TreeParams, scene.TreeResult](scene.TreeName, "Show the active editor scene's node tree")
	NodeInspect     = define[node.InspectParams, node.InspectResult](node.InspectName, "Inspect a node in the active editor scene")
	NodeSet         = define[node.SetParams, node.SetResult](node.SetName, "Set node properties as one undoable editor action")
	NodeAdd         = define[node.AddParams, node.AddResult](node.AddName, "Add a node to the active editor scene as one undoable editor action")
	NodeInstantiate = define[node.InstantiateParams, node.InstantiateResult](
		node.InstantiateName,
		"Add a linked instance of a PackedScene to the active editor scene as one undoable editor action",
	)
	NodeRename     = define[node.RenameParams, node.RenameResult](node.RenameName, "Rename a node as one undoable editor action")
	NodeDuplicate  = define[node.DuplicateParams, node.DuplicateResult](node.DuplicateName, "Duplicate a node and its children as one undoable editor action")
	NodeMove       = define[node.MoveParams, node.MoveResult](node.MoveName, "Move a node to a new parent or position as one undoable editor action")
	NodeDelete     = define[node.DeleteParams, node.DeleteResult](node.DeleteName, "Delete a node from the active editor scene as one undoable editor action")
	ResourceCreate = define[resource.CreateParams, resource.CreateResult](
		resource.CreateName,
		"Create a resource file and write it to disk immediately",
	)
	ResourceInspect = define[resource.InspectParams, resource.InspectResult](
		resource.InspectName,
		"Inspect a resource file's editor-visible properties",
	)
	ResourceSet = define[resource.SetParams, resource.SetResult](
		resource.SetName,
		"Set resource file properties and save the file immediately",
	)
	ResourceDelete = define[resource.DeleteParams, resource.DeleteResult](
		resource.DeleteName,
		"Delete a resource file that nothing else references",
	)
	ProjectSettingsList = define[settings.ListParams, settings.ListResult](
		settings.ListName,
		"List editor-visible project settings",
	)
	ProjectSettingsGet = define[settings.GetParams, settings.GetResult](
		settings.GetName,
		"Show one project setting with its default and override state",
	)
	ProjectSettingsSet = define[settings.SetParams, settings.SetResult](
		settings.SetName,
		"Set a project setting and save project.godot immediately",
	)
	ProjectSettingsUnset = define[settings.UnsetParams, settings.UnsetResult](
		settings.UnsetName,
		"Remove a project setting override and save project.godot immediately",
	)
	InputList        = define[input.ListParams, input.ListResult](input.ListName, "List Input Map actions")
	InputInspect     = define[input.InspectParams, input.InspectResult](input.InspectName, "Show one Input Map action and its bindings")
	InputAdd         = define[input.AddParams, input.AddResult](input.AddName, "Add an Input Map action and save project.godot immediately")
	InputSet         = define[input.SetParams, input.SetResult](input.SetName, "Set an Input Map action's deadzone and save project.godot immediately")
	InputRename      = define[input.RenameParams, input.RenameResult](input.RenameName, "Rename an Input Map action and save project.godot immediately")
	InputDelete      = define[input.DeleteParams, input.DeleteResult](input.DeleteName, "Delete an Input Map action and save project.godot immediately")
	InputEventAdd    = define[input.EventAddParams, input.EventAddResult](input.EventAddName, "Add a binding to an Input Map action and save project.godot immediately")
	InputEventDelete = define[input.EventDeleteParams, input.EventDeleteResult](input.EventDeleteName, "Remove a binding from an Input Map action and save project.godot immediately")
)

func All() []ActionDefinition {
	return []ActionDefinition{
		SceneList,
		SceneCurrent,
		SceneOpen,
		SceneCreate,
		SceneSave,
		SceneTree,
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
