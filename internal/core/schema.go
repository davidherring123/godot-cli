// Package core provides shared Godot project functionality and schemas.
package core

import (
	"reflect"

	"github.com/davidherring123/godot-cli/internal/core/node"
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
