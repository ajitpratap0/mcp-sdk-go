package utils

import (
	"encoding/json"
	"testing"
)

type TestStruct struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	IsValid bool   `json:"isValid"`
}

func TestGenerateJSONSchema(t *testing.T) {
	// Test struct for schema generation
	testObj := TestStruct{
		Name:    "Test",
		Age:     30,
		IsValid: true,
	}

	// Generate schema
	schema, err := GenerateJSONSchema(testObj)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	// Validate schema format
	var schemaObj map[string]interface{}
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		t.Fatalf("Generated schema is not valid JSON: %v", err)
	}

	// Check type field
	if schemaObj["type"] != "object" {
		t.Errorf("Expected schema type to be 'object', got %v", schemaObj["type"])
	}

	// Check example field
	example, ok := schemaObj["example"]
	if !ok {
		t.Fatal("Expected schema to have 'example' field")
	}

	// Check example content
	exampleMap, ok := example.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected example to be a map, got %T", example)
	}

	if exampleMap["name"] != "Test" {
		t.Errorf("Expected example.name to be 'Test', got %v", exampleMap["name"])
	}

	if int(exampleMap["age"].(float64)) != 30 {
		t.Errorf("Expected example.age to be 30, got %v", exampleMap["age"])
	}

	if exampleMap["isValid"] != true {
		t.Errorf("Expected example.isValid to be true, got %v", exampleMap["isValid"])
	}
}

func TestValidateAgainstSchema(t *testing.T) {
	// Valid JSON data
	validData := json.RawMessage(`{"name": "Test", "age": 30, "isValid": true}`)

	// Invalid JSON data
	invalidData := json.RawMessage(`{"name": "Test", "age": 30, "isValid": true`)

	// Simple schema
	schema := json.RawMessage(`{"type": "object"}`)

	// Test validation of valid data
	err := ValidateAgainstSchema(validData, schema)
	if err != nil {
		t.Errorf("Expected valid data to pass validation, got error: %v", err)
	}

	// Test validation of invalid data
	err = ValidateAgainstSchema(invalidData, schema)
	if err == nil {
		t.Error("Expected invalid data to fail validation, but got no error")
	}
}

func TestMergeJSONObjects(t *testing.T) {
	// Test merging empty list
	result, err := MergeJSONObjects()
	if err != nil {
		t.Errorf("Expected MergeJSONObjects() to succeed, got error: %v", err)
	}
	if string(result) != "{}" {
		t.Errorf("Expected empty merge result to be {}, got %s", string(result))
	}

	// Test merging single object
	obj1 := json.RawMessage(`{"a": 1, "b": 2}`)
	result, err = MergeJSONObjects(obj1)
	if err != nil {
		t.Errorf("Expected MergeJSONObjects(obj1) to succeed, got error: %v", err)
	}
	if string(result) != string(obj1) {
		t.Errorf("Expected single object merge to return same object, got %s", string(result))
	}

	// Test merging multiple objects
	obj2 := json.RawMessage(`{"b": 3, "c": 4}`)
	expected := `{"a":1,"b":3,"c":4}`

	result, err = MergeJSONObjects(obj1, obj2)
	if err != nil {
		t.Errorf("Expected MergeJSONObjects(obj1, obj2) to succeed, got error: %v", err)
	}

	// Normalize for comparison (marshal/unmarshal to handle whitespace differences)
	var expectedObj, resultObj interface{}
	if err := json.Unmarshal([]byte(expected), &expectedObj); err != nil {
		t.Fatalf("Failed to unmarshal expected JSON: %v", err)
	}
	if err := json.Unmarshal(result, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result JSON: %v", err)
	}

	expectedJSON, _ := json.Marshal(expectedObj)
	resultJSON, _ := json.Marshal(resultObj)

	if string(expectedJSON) != string(resultJSON) {
		t.Errorf("Expected merged result to be %s, got %s", expected, string(result))
	}

	// Test with invalid JSON
	invalidObj := json.RawMessage(`{"invalid": true`)
	_, err = MergeJSONObjects(obj1, invalidObj)
	if err == nil {
		t.Error("Expected MergeJSONObjects with invalid object to fail, but got no error")
	}
}

func TestJSONToStruct(t *testing.T) {
	// Valid JSON matching struct
	validJSON := json.RawMessage(`{"name": "Test", "age": 30, "isValid": true}`)

	// Test unmarshaling to struct
	var testStruct TestStruct
	err := JSONToStruct(validJSON, &testStruct)
	if err != nil {
		t.Errorf("Expected JSONToStruct to succeed, got error: %v", err)
	}

	// Verify struct fields
	if testStruct.Name != "Test" {
		t.Errorf("Expected Name to be 'Test', got %q", testStruct.Name)
	}

	if testStruct.Age != 30 {
		t.Errorf("Expected Age to be 30, got %d", testStruct.Age)
	}

	if !testStruct.IsValid {
		t.Error("Expected IsValid to be true")
	}

	// Test with invalid JSON
	invalidJSON := json.RawMessage(`{"name": "Test"`)
	err = JSONToStruct(invalidJSON, &testStruct)
	if err == nil {
		t.Error("Expected JSONToStruct with invalid JSON to fail, but got no error")
	}
}
