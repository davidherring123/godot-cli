package node

import "github.com/invopop/jsonschema"

func (SetParams) JSONSchemaExtend(schema *jsonschema.Schema) {
	properties, ok := schema.Properties.Get("properties")
	if !ok {
		return
	}
	minimum := uint64(1)
	properties.MinProperties = &minimum
	properties.PropertyNames = &jsonschema.Schema{Type: "string", MinLength: &minimum}
}
