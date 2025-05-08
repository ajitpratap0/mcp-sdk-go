package utils

import (
	"encoding/json"
	"fmt"
)

// GenerateJSONSchema generates a JSON schema from a Go struct
// This is useful for creating tool input/output schemas
func GenerateJSONSchema(v interface{}) (json.RawMessage, error) {
	// A very simple implementation - in a real-world scenario, you'd want to use
	// a proper JSON Schema generator library
	
	// For now, we'll just marshal the example object and return it as a schema
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal example: %w", err)
	}
	
	// Note: This is a placeholder. A real implementation would analyze the structure
	// and generate a proper JSON Schema.
	schema := fmt.Sprintf(`{
		"type": "object",
		"example": %s
	}`, string(data))
	
	return json.RawMessage(schema), nil
}

// ValidateAgainstSchema validates data against a JSON schema
func ValidateAgainstSchema(data json.RawMessage, schema json.RawMessage) error {
	// This is a placeholder. In a real-world scenario, you'd want to use
	// a proper JSON Schema validation library
	
	// For now, just ensure the data is valid JSON
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	
	return nil
}

// MergeJSONObjects merges multiple JSON objects, with later objects taking precedence
func MergeJSONObjects(objects ...json.RawMessage) (json.RawMessage, error) {
	if len(objects) == 0 {
		return json.RawMessage("{}"), nil
	}
	
	if len(objects) == 1 {
		return objects[0], nil
	}
	
	// Unmarshal all objects
	var result map[string]interface{}
	for _, obj := range objects {
		var current map[string]interface{}
		if err := json.Unmarshal(obj, &current); err != nil {
			return nil, fmt.Errorf("failed to unmarshal object: %w", err)
		}
		
		// Initialize result if needed
		if result == nil {
			result = make(map[string]interface{})
		}
		
		// Merge current into result
		for k, v := range current {
			result[k] = v
		}
	}
	
	// Marshal the result
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged object: %w", err)
	}
	
	return data, nil
}

// JSONToStruct unmarshals JSON into a struct with better error messages
func JSONToStruct(data json.RawMessage, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w (data: %s)", err, string(data))
	}
	
	return nil
}
