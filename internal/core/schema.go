// Package core provides shared Godot project functionality and schemas.
package core

import (
	"reflect"

	"github.com/davidherring123/godot-cli/internal/core/node"
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
	SceneList    = define[scene.ListParams, scene.ListResult](scene.ListName, "List scenes in the project")
	SceneCurrent = define[scene.CurrentParams, scene.CurrentResult](scene.CurrentName, "Show the active editor scene")
	SceneOpen    = define[scene.OpenParams, scene.OpenResult](scene.OpenName, "Open or activate an editor scene")
	SceneSave    = define[scene.SaveParams, scene.SaveResult](scene.SaveName, "Save the active editor scene")
	SceneTree    = define[scene.TreeParams, scene.TreeResult](scene.TreeName, "Show the active editor scene's node tree")
	NodeInspect  = define[node.InspectParams, node.InspectResult](node.InspectName, "Inspect a node in the active editor scene")
	NodeSet      = define[node.SetParams, node.SetResult](node.SetName, "Set node properties as one undoable editor action")
)

func All() []ActionDefinition {
	return []ActionDefinition{SceneList, SceneCurrent, SceneOpen, SceneSave, SceneTree, NodeInspect, NodeSet}
}

func Lookup(name string) (ActionDefinition, bool) {
	for _, definition := range All() {
		if definition.Name == name {
			return definition, true
		}
	}
	return ActionDefinition{}, false
}
