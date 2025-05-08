package protocol

import (
	"encoding/json"
	"testing"
)

func TestTool(t *testing.T) {
	// Test Tool struct with various fields
	tool := Tool{
		Name:        "test-tool",
		Description: "A test tool",
		Categories:  []string{"utility", "test"},
		InputSchema: json.RawMessage(`{"type": "object"}`),
		Type:        "function",
		Async:       false,
	}

	// Verify fields
	if tool.Name != "test-tool" {
		t.Errorf("Expected Name to be 'test-tool', got %q", tool.Name)
	}

	if tool.Description != "A test tool" {
		t.Errorf("Expected Description to be 'A test tool', got %q", tool.Description)
	}

	if len(tool.Categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(tool.Categories))
	}

	if tool.Categories[0] != "utility" || tool.Categories[1] != "test" {
		t.Errorf("Expected categories to be ['utility', 'test'], got %v", tool.Categories)
	}

	// Test JSON serialization
	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal Tool: %v", err)
	}

	var decoded Tool
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal Tool: %v", err)
	}

	// Verify decoded data
	if decoded.Name != tool.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", tool.Name, decoded.Name)
	}

	if decoded.Description != tool.Description {
		t.Errorf("Expected decoded Description to be %q, got %q", tool.Description, decoded.Description)
	}

	if len(decoded.Categories) != len(tool.Categories) {
		t.Errorf("Expected %d categories after decoding, got %d", len(tool.Categories), len(decoded.Categories))
	}
}

func TestListToolsParams(t *testing.T) {
	// Test ListToolsParams creation
	params := ListToolsParams{
		Category: "utility",
		PaginationParams: PaginationParams{
			Limit:  10,
			Cursor: "test-cursor",
		},
	}

	// Verify fields
	if params.Category != "utility" {
		t.Errorf("Expected Category to be 'utility', got %q", params.Category)
	}

	if params.Limit != 10 {
		t.Errorf("Expected Limit to be 10, got %d", params.Limit)
	}

	if params.Cursor != "test-cursor" {
		t.Errorf("Expected Cursor to be 'test-cursor', got %q", params.Cursor)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal ListToolsParams: %v", err)
	}

	var decoded ListToolsParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ListToolsParams: %v", err)
	}

	// Verify decoded data
	if decoded.Category != params.Category {
		t.Errorf("Expected decoded Category to be %q, got %q", params.Category, decoded.Category)
	}

	if decoded.Limit != params.Limit {
		t.Errorf("Expected decoded Limit to be %d, got %d", params.Limit, decoded.Limit)
	}

	if decoded.Cursor != params.Cursor {
		t.Errorf("Expected decoded Cursor to be %q, got %q", params.Cursor, decoded.Cursor)
	}
}

func TestListToolsResult(t *testing.T) {
	// Create test tools
	tools := []Tool{
		{
			Name:        "tool1",
			Description: "Tool 1",
			Categories:  []string{"utility"},
		},
		{
			Name:        "tool2",
			Description: "Tool 2",
			Categories:  []string{"test"},
		},
	}

	// Test ListToolsResult creation
	result := ListToolsResult{
		Tools: tools,
		PaginationResult: PaginationResult{
			HasMore:    true,
			NextCursor: "next-cursor",
		},
	}

	// Verify fields
	if len(result.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(result.Tools))
	}

	// PaginationResult doesn't have a Total field

	if !result.HasMore {
		t.Error("Expected HasMore to be true")
	}

	if result.NextCursor != "next-cursor" {
		t.Errorf("Expected NextCursor to be 'next-cursor', got %q", result.NextCursor)
	}

	// Test JSON serialization
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal ListToolsResult: %v", err)
	}

	var decoded ListToolsResult
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ListToolsResult: %v", err)
	}

	// Verify decoded data
	if len(decoded.Tools) != len(result.Tools) {
		t.Errorf("Expected %d tools after decoding, got %d", len(result.Tools), len(decoded.Tools))
	}

	// PaginationResult doesn't have a Total field

	if decoded.HasMore != result.HasMore {
		t.Errorf("Expected decoded HasMore to be %v, got %v", result.HasMore, decoded.HasMore)
	}
}

func TestCallToolParams(t *testing.T) {
	// Test CallToolParams creation
	params := CallToolParams{
		Name:  "test-tool",
		Input: json.RawMessage(`{"param": "value"}`),
	}

	// Verify fields
	if params.Name != "test-tool" {
		t.Errorf("Expected Name to be 'test-tool', got %q", params.Name)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal CallToolParams: %v", err)
	}

	var decoded CallToolParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal CallToolParams: %v", err)
	}

	// Verify decoded data
	if decoded.Name != params.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", params.Name, decoded.Name)
	}
}

func TestCallToolResult(t *testing.T) {
	// Test CallToolResult creation
	result := CallToolResult{
		Result: json.RawMessage(`{"result": "success"}`),
		Error:  "No error",
	}

	// Test JSON serialization
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal CallToolResult: %v", err)
	}

	var decoded CallToolResult
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal CallToolResult: %v", err)
	}

	// Verify decoded data
	if decoded.Error != result.Error {
		t.Errorf("Expected decoded Error to be %q, got %q", result.Error, decoded.Error)
	}

	// CallToolResult doesn't have a Status field
}

func TestToolsChangedParams(t *testing.T) {
	// Create test tools
	added := []Tool{
		{
			Name:        "added-tool",
			Description: "Added Tool",
			Categories:  []string{"utility"},
		},
	}

	modified := []Tool{
		{
			Name:        "modified-tool",
			Description: "Modified Tool",
			Categories:  []string{"test"},
		},
	}

	// Test ToolsChangedParams creation
	params := ToolsChangedParams{
		Added:    added,
		Removed:  []string{"removed-tool"},
		Modified: modified,
	}

	// Verify fields
	if len(params.Added) != 1 {
		t.Errorf("Expected 1 added tool, got %d", len(params.Added))
	}

	if len(params.Removed) != 1 {
		t.Errorf("Expected 1 removed tool name, got %d", len(params.Removed))
	}

	if len(params.Modified) != 1 {
		t.Errorf("Expected 1 modified tool, got %d", len(params.Modified))
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal ToolsChangedParams: %v", err)
	}

	var decoded ToolsChangedParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolsChangedParams: %v", err)
	}

	// Verify decoded data
	if len(decoded.Added) != len(params.Added) {
		t.Errorf("Expected %d added tools after decoding, got %d", len(params.Added), len(decoded.Added))
	}

	if len(decoded.Removed) != len(params.Removed) {
		t.Errorf("Expected %d removed tool names after decoding, got %d", len(params.Removed), len(decoded.Removed))
	}

	if len(decoded.Modified) != len(params.Modified) {
		t.Errorf("Expected %d modified tools after decoding, got %d", len(params.Modified), len(decoded.Modified))
	}
}
