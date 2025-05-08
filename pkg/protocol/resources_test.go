package protocol

import (
	"encoding/json"
	"testing"
)

func TestResource(t *testing.T) {
	// Test basic Resource struct creation and fields
	resource := Resource{
		URI:         "test://resource/1",
		Name:        "Test Resource",
		Description: "A test resource",
		Type:        "text/plain",
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "42",
		},
	}

	// Verify fields
	if resource.URI != "test://resource/1" {
		t.Errorf("Expected URI to be 'test://resource/1', got %q", resource.URI)
	}

	if resource.Name != "Test Resource" {
		t.Errorf("Expected Name to be 'Test Resource', got %q", resource.Name)
	}

	if resource.Description != "A test resource" {
		t.Errorf("Expected Description to be 'A test resource', got %q", resource.Description)
	}

	if resource.Type != "text/plain" {
		t.Errorf("Expected Type to be 'text/plain', got %q", resource.Type)
	}

	// Resource does not have Tags field in the actual implementation

	if len(resource.Metadata) != 2 {
		t.Errorf("Expected 2 metadata entries, got %d", len(resource.Metadata))
	}

	// Test JSON serialization
	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("Failed to marshal Resource: %v", err)
	}

	var decoded Resource
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal Resource: %v", err)
	}

	// Verify decoded data
	if decoded.URI != resource.URI {
		t.Errorf("Expected decoded URI to be %q, got %q", resource.URI, decoded.URI)
	}

	if decoded.Name != resource.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", resource.Name, decoded.Name)
	}

	if decoded.Type != resource.Type {
		t.Errorf("Expected decoded Type to be %q, got %q", resource.Type, decoded.Type)
	}

	// Resource does not have Tags field in the actual implementation
}

func TestResourceContents(t *testing.T) {
	// Test ResourceContents struct
	contents := ResourceContents{
		Type:     "text/plain",
		Content:  json.RawMessage(`"This is the content"`),
		Metadata: map[string]string{"key": "value"},
	}

	// Verify fields
	if contents.Type != "text/plain" {
		t.Errorf("Expected Type to be 'text/plain', got %q", contents.Type)
	}

	// Test JSON serialization
	data, err := json.Marshal(contents)
	if err != nil {
		t.Fatalf("Failed to marshal ResourceContents: %v", err)
	}

	var decoded ResourceContents
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ResourceContents: %v", err)
	}

	// Verify decoded data
	if decoded.Type != contents.Type {
		t.Errorf("Expected decoded Type to be %q, got %q", contents.Type, decoded.Type)
	}
}

func TestResourceTemplate(t *testing.T) {
	// Test ResourceTemplate struct
	template := ResourceTemplate{
		URI:         "template://resource/1",
		Name:        "Test Template",
		Description: "A test template",
		Type:        "text/template",
	}

	// Verify fields
	if template.URI != "template://resource/1" {
		t.Errorf("Expected URI to be 'template://resource/1', got %q", template.URI)
	}

	if template.Name != "Test Template" {
		t.Errorf("Expected Name to be 'Test Template', got %q", template.Name)
	}

	if template.Type != "text/template" {
		t.Errorf("Expected Type to be 'text/template', got %q", template.Type)
	}

	// Test JSON serialization
	data, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("Failed to marshal ResourceTemplate: %v", err)
	}

	var decoded ResourceTemplate
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ResourceTemplate: %v", err)
	}

	// Verify decoded data
	if decoded.URI != template.URI {
		t.Errorf("Expected decoded URI to be %q, got %q", template.URI, decoded.URI)
	}

	if decoded.Name != template.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", template.Name, decoded.Name)
	}
}

func TestListResourcesParams(t *testing.T) {
	// Test ListResourcesParams struct
	params := ListResourcesParams{
		URI:       "resources://",
		Recursive: true,
		PaginationParams: PaginationParams{
			Limit:  20,
			Cursor: "test-cursor",
		},
	}

	// Verify fields
	if params.URI != "resources://" {
		t.Errorf("Expected URI to be 'resources://', got %q", params.URI)
	}

	if !params.Recursive {
		t.Error("Expected Recursive to be true")
	}

	if params.Limit != 20 {
		t.Errorf("Expected Limit to be 20, got %d", params.Limit)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal ListResourcesParams: %v", err)
	}

	var decoded ListResourcesParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ListResourcesParams: %v", err)
	}

	// Verify decoded data
	if decoded.URI != params.URI {
		t.Errorf("Expected decoded URI to be %q, got %q", params.URI, decoded.URI)
	}

	if decoded.Recursive != params.Recursive {
		t.Errorf("Expected decoded Recursive to be %v, got %v", params.Recursive, decoded.Recursive)
	}
}

func TestGetResourceParams(t *testing.T) {
	// Test ReadResourceParams struct with URI field
	params := ReadResourceParams{
		URI: "resource://test/1",
	}

	// Verify fields
	if params.URI != "resource://test/1" {
		t.Errorf("Expected URI to be 'resource://test/1', got %q", params.URI)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal ReadResourceParams: %v", err)
	}

	var decoded ReadResourceParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ReadResourceParams: %v", err)
	}

	// Verify decoded data
	if decoded.URI != params.URI {
		t.Errorf("Expected decoded URI to be %q, got %q", params.URI, decoded.URI)
	}
}

func TestReadResourceResult(t *testing.T) {
	// Test ReadResourceResult struct
	result := ReadResourceResult{
		Contents: ResourceContents{
			URI:     "resource://test/1",
			Type:    "text/plain",
			Content: json.RawMessage(`"Content string"`),
		},
	}

	// Verify fields
	if result.Contents.URI != "resource://test/1" {
		t.Errorf("Expected Contents.URI to be 'resource://test/1', got %q", result.Contents.URI)
	}

	if result.Contents.Type != "text/plain" {
		t.Errorf("Expected Contents.Type to be 'text/plain', got %q", result.Contents.Type)
	}

	// Test JSON serialization
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal ReadResourceResult: %v", err)
	}

	var decoded ReadResourceResult
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ReadResourceResult: %v", err)
	}

	// Verify decoded data
	if decoded.Contents.URI != result.Contents.URI {
		t.Errorf("Expected decoded Contents.URI to be %q, got %q", result.Contents.URI, decoded.Contents.URI)
	}

	if decoded.Contents.Type != result.Contents.Type {
		t.Errorf("Expected decoded Contents.Type to be %q, got %q", result.Contents.Type, decoded.Contents.Type)
	}
}

func TestResourcesChangedParams(t *testing.T) {
	// Test ResourcesChangedParams struct
	params := ResourcesChangedParams{
		URI: "resources://",
		Resources: []Resource{
			{URI: "resource://1", Name: "Resource 1"},
			{URI: "resource://2", Name: "Resource 2"},
		},
		Added: []Resource{
			{URI: "resource://3", Name: "Resource 3"},
		},
		Removed: []string{"resource://4"},
		Modified: []Resource{
			{URI: "resource://5", Name: "Modified Resource"},
		},
	}

	// Verify fields
	if params.URI != "resources://" {
		t.Errorf("Expected URI to be 'resources://', got %q", params.URI)
	}

	if len(params.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(params.Resources))
	}

	if len(params.Added) != 1 {
		t.Errorf("Expected 1 added resource, got %d", len(params.Added))
	}

	if len(params.Removed) != 1 {
		t.Errorf("Expected 1 removed resource URI, got %d", len(params.Removed))
	}

	if len(params.Modified) != 1 {
		t.Errorf("Expected 1 modified resource, got %d", len(params.Modified))
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal ResourcesChangedParams: %v", err)
	}

	var decoded ResourcesChangedParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ResourcesChangedParams: %v", err)
	}

	// Verify decoded data
	if len(decoded.Resources) != len(params.Resources) {
		t.Errorf("Expected %d resources after decoding, got %d", len(params.Resources), len(decoded.Resources))
	}

	if len(decoded.Added) != len(params.Added) {
		t.Errorf("Expected %d added resources after decoding, got %d", len(params.Added), len(decoded.Added))
	}
}
