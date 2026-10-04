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
	SceneList   = define[scene.ListParams, scene.ListResult](scene.ListName, "List scenes in the project")
	SceneTree   = define[scene.TreeParams, scene.TreeResult](scene.TreeName, "Show a scene's node tree")
	NodeInspect = define[node.InspectParams, node.InspectResult](node.InspectName, "Inspect a node and its properties")
)

func All() []ActionDefinition {
	return []ActionDefinition{SceneList, SceneTree, NodeInspect}
}

func Lookup(name string) (ActionDefinition, bool) {
	for _, definition := range All() {
		if definition.Name == name {
			return definition, true
		}
	}
	return ActionDefinition{}, false
}
