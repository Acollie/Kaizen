package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

// ValidateInput checks raw tool input against the subset of JSON Schema that
// SchemaFor produces: object properties, required keys, additionalProperties
// false, primitive types, enums and array items. Tool input can arrive
// truncated or malformed (eager input streaming does not validate it), so this
// runs before every tool.
func ValidateInput(schema json.RawMessage, raw json.RawMessage) error {
	var schemaMap map[string]any
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return fmt.Errorf("invalid tool schema: %w", err)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		trimmed = []byte("{}")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("invalid tool input: not valid JSON: %w", err)
	}
	return validateValue(schemaMap, value, "input")
}

func validateValue(schema map[string]any, value any, path string) error {
	if enum, ok := schema["enum"].([]any); ok {
		if err := validateEnum(enum, value, path); err != nil {
			return err
		}
	}
	switch schema["type"] {
	case "object":
		return validateObject(schema, value, path)
	case "array":
		return validateArray(schema, value, path)
	case "string":
		return expectType[string](value, path, "a string")
	case "boolean":
		return expectType[bool](value, path, "a boolean")
	case "integer":
		return validateNumber(value, path, true)
	case "number":
		return validateNumber(value, path, false)
	default:
		return nil
	}
}

func expectType[T any](value any, path, description string) error {
	if _, ok := value.(T); !ok {
		return fmt.Errorf("invalid tool input: %s must be %s", path, description)
	}
	return nil
}

func validateEnum(enum []any, value any, path string) error {
	for _, allowed := range enum {
		if allowed == value {
			return nil
		}
	}
	return fmt.Errorf("invalid tool input: %s must be one of %v, got %v", path, enum, value)
}

func validateNumber(value any, path string, integer bool) error {
	number, ok := value.(json.Number)
	if !ok {
		return fmt.Errorf("invalid tool input: %s must be a number", path)
	}
	parsed, err := number.Float64()
	if err != nil {
		return fmt.Errorf("invalid tool input: %s is not a valid number", path)
	}
	if integer && parsed != math.Trunc(parsed) {
		return fmt.Errorf("invalid tool input: %s must be an integer", path)
	}
	return nil
}

func validateObject(schema map[string]any, value any, path string) error {
	object, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid tool input: %s must be an object", path)
	}
	properties, _ := schema["properties"].(map[string]any)
	if required, ok := schema["required"].([]any); ok {
		for _, key := range required {
			name, _ := key.(string)
			if _, present := object[name]; !present {
				return fmt.Errorf("invalid tool input: %s is missing required field %q", path, name)
			}
		}
	}
	for key, fieldValue := range object {
		fieldSchema, known := properties[key].(map[string]any)
		if !known {
			if schema["additionalProperties"] == false {
				return fmt.Errorf("invalid tool input: %s has unknown field %q", path, key)
			}
			continue
		}
		if err := validateValue(fieldSchema, fieldValue, path+"."+key); err != nil {
			return err
		}
	}
	return nil
}

func validateArray(schema map[string]any, value any, path string) error {
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("invalid tool input: %s must be an array", path)
	}
	itemSchema, _ := schema["items"].(map[string]any)
	if itemSchema == nil {
		return nil
	}
	for index, item := range items {
		if err := validateValue(itemSchema, item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
	}
	return nil
}
