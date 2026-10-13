package resource

import "github.com/invopop/jsonschema"

func (CreateParams) JSONSchemaExtend(schema *jsonschema.Schema) {
	extendPropertyNames(schema)
}

func (SetParams) JSONSchemaExtend(schema *jsonschema.Schema) {
	properties := extendPropertyNames(schema)

	if properties == nil {
		return
	}

	minimum := uint64(1)
	properties.MinProperties = &minimum
}

func extendPropertyNames(schema *jsonschema.Schema) *jsonschema.Schema {
	properties, ok := schema.Properties.Get("properties")

	if !ok {
		return nil
	}

	minimum := uint64(1)
	properties.PropertyNames = &jsonschema.Schema{Type: "string", MinLength: &minimum}

	return properties
}
