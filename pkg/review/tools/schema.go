package tools

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// SchemaFor reflects a JSON Schema from a struct's json and jsonschema tags.
// Every object gets additionalProperties false and no $ref indirection, which
// is what strict tool use on the Messages API requires. The $schema and $id
// keywords are dropped because tool input schemas do not need them.
func SchemaFor(value any) json.RawMessage {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties:  false,
		RequiredFromJSONSchemaTags: true,
		DoNotReference:             true,
	}
	schema := reflector.Reflect(value)
	schema.Version = ""
	schema.ID = ""
	encoded, err := json.Marshal(schema)
	if err != nil {
		// Reflection over our own static structs cannot fail at runtime
		panic(fmt.Sprintf("tools: marshal schema: %v", err))
	}
	return ensureProperties(encoded)
}

// ensureProperties adds an empty properties object to schemas reflected from
// empty structs, since some clients expect the key to be present
func ensureProperties(encoded []byte) json.RawMessage {
	var schemaMap map[string]any
	if err := json.Unmarshal(encoded, &schemaMap); err != nil {
		return encoded
	}
	if _, ok := schemaMap["properties"]; ok {
		return encoded
	}
	schemaMap["properties"] = map[string]any{}
	withProperties, err := json.Marshal(schemaMap)
	if err != nil {
		return encoded
	}
	return withProperties
}
