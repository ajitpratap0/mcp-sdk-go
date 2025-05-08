package protocol

import (
	"encoding/json"
	"testing"
)

func TestPrompt(t *testing.T) {
	// Test basic Prompt struct with fields
	prompt := Prompt{
		ID:          "test-prompt-1",
		Name:        "Test Prompt",
		Description: "A test prompt",
		Messages: []PromptMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello world"},
		},
		Parameters: []PromptParameter{
			{Name: "param1", Description: "Test param", Type: "string", Required: true},
		},
		Tags: []string{"test", "example"},
	}

	// Verify fields
	if prompt.ID != "test-prompt-1" {
		t.Errorf("Expected ID to be 'test-prompt-1', got %q", prompt.ID)
	}

	if prompt.Name != "Test Prompt" {
		t.Errorf("Expected Name to be 'Test Prompt', got %q", prompt.Name)
	}

	if len(prompt.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(prompt.Messages))
	}

	if len(prompt.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(prompt.Tags))
	}

	// Test JSON serialization
	data, err := json.Marshal(prompt)
	if err != nil {
		t.Fatalf("Failed to marshal Prompt: %v", err)
	}

	var decoded Prompt
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal Prompt: %v", err)
	}

	// Verify decoded data
	if decoded.ID != prompt.ID {
		t.Errorf("Expected decoded ID to be %q, got %q", prompt.ID, decoded.ID)
	}

	if decoded.Name != prompt.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", prompt.Name, decoded.Name)
	}

	if len(decoded.Messages) != len(prompt.Messages) {
		t.Errorf("Expected %d messages after decoding, got %d", len(prompt.Messages), len(decoded.Messages))
	}

	if decoded.Messages[0].Role != "system" {
		t.Errorf("Expected first message role to be 'system', got %q", decoded.Messages[0].Role)
	}

	if decoded.Messages[1].Content != "Hello world" {
		t.Errorf("Expected second message content to be 'Hello world', got %q", decoded.Messages[1].Content)
	}
}

func TestPromptMessage(t *testing.T) {
	// Test basic PromptMessage creation
	msg := PromptMessage{
		Role:        "user",
		Content:     "Test content",
		Name:        "user1",
		ResourceRefs: []string{"resource1", "resource2"},
		Parameters:   []string{"param1", "param2"},
	}

	// Verify fields
	if msg.Role != "user" {
		t.Errorf("Expected Role to be 'user', got %q", msg.Role)
	}

	if msg.Content != "Test content" {
		t.Errorf("Expected Content to be 'Test content', got %q", msg.Content)
	}

	if msg.Name != "user1" {
		t.Errorf("Expected Name to be 'user1', got %q", msg.Name)
	}

	if len(msg.ResourceRefs) != 2 {
		t.Errorf("Expected 2 resource refs, got %d", len(msg.ResourceRefs))
	}

	if len(msg.Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(msg.Parameters))
	}

	// Test JSON serialization
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal PromptMessage: %v", err)
	}

	var decoded PromptMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal PromptMessage: %v", err)
	}

	// Verify decoded data
	if decoded.Role != msg.Role {
		t.Errorf("Expected decoded Role to be %q, got %q", msg.Role, decoded.Role)
	}

	if decoded.Content != msg.Content {
		t.Errorf("Expected decoded Content to be %q, got %q", msg.Content, decoded.Content)
	}
}

func TestPromptParameter(t *testing.T) {
	// Test basic PromptParameter creation
	param := PromptParameter{
		Name:        "test-param",
		Description: "A test parameter",
		Type:        "string",
		Required:    true,
		Default:     "default-value",
	}

	// Verify fields
	if param.Name != "test-param" {
		t.Errorf("Expected Name to be 'test-param', got %q", param.Name)
	}

	if param.Description != "A test parameter" {
		t.Errorf("Expected Description to be 'A test parameter', got %q", param.Description)
	}

	if param.Type != "string" {
		t.Errorf("Expected Type to be 'string', got %q", param.Type)
	}

	if !param.Required {
		t.Error("Expected Required to be true")
	}

	// Test JSON serialization
	data, err := json.Marshal(param)
	if err != nil {
		t.Fatalf("Failed to marshal PromptParameter: %v", err)
	}

	var decoded PromptParameter
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal PromptParameter: %v", err)
	}

	// Verify decoded data
	if decoded.Name != param.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", param.Name, decoded.Name)
	}

	if decoded.Description != param.Description {
		t.Errorf("Expected decoded Description to be %q, got %q", param.Description, decoded.Description)
	}

	if decoded.Required != param.Required {
		t.Errorf("Expected decoded Required to be %v, got %v", param.Required, decoded.Required)
	}
}

func TestPromptExample(t *testing.T) {
	// Test basic PromptExample creation
	example := PromptExample{
		Name:        "test-example",
		Description: "A test example",
		Parameters: map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		},
		Result: "Example result",
	}

	// Verify fields
	if example.Name != "test-example" {
		t.Errorf("Expected Name to be 'test-example', got %q", example.Name)
	}

	if example.Description != "A test example" {
		t.Errorf("Expected Description to be 'A test example', got %q", example.Description)
	}

	if len(example.Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(example.Parameters))
	}

	if example.Result != "Example result" {
		t.Errorf("Expected Result to be 'Example result', got %q", example.Result)
	}

	// Test JSON serialization
	data, err := json.Marshal(example)
	if err != nil {
		t.Fatalf("Failed to marshal PromptExample: %v", err)
	}

	var decoded PromptExample
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal PromptExample: %v", err)
	}

	// Verify decoded data
	if decoded.Name != example.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", example.Name, decoded.Name)
	}

	if decoded.Description != example.Description {
		t.Errorf("Expected decoded Description to be %q, got %q", example.Description, decoded.Description)
	}

	if decoded.Result != example.Result {
		t.Errorf("Expected decoded Result to be %q, got %q", example.Result, decoded.Result)
	}
}

func TestListPromptsParams(t *testing.T) {
	// Test basic ListPromptsParams creation
	params := ListPromptsParams{
		Tag: "test-tag",
		PaginationParams: PaginationParams{
			Limit:  10,
			Cursor: "test-cursor",
		},
	}

	// Verify fields
	if params.Tag != "test-tag" {
		t.Errorf("Expected Tag to be 'test-tag', got %q", params.Tag)
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
		t.Fatalf("Failed to marshal ListPromptsParams: %v", err)
	}

	var decoded ListPromptsParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ListPromptsParams: %v", err)
	}

	// Verify decoded data
	if decoded.Tag != params.Tag {
		t.Errorf("Expected decoded Tag to be %q, got %q", params.Tag, decoded.Tag)
	}

	if decoded.Limit != params.Limit {
		t.Errorf("Expected decoded Limit to be %d, got %d", params.Limit, decoded.Limit)
	}

	if decoded.Cursor != params.Cursor {
		t.Errorf("Expected decoded Cursor to be %q, got %q", params.Cursor, decoded.Cursor)
	}
}

func TestPromptsChangedParams(t *testing.T) {
	// Create test data
	addedPrompt := Prompt{
		ID:   "added-prompt",
		Name: "Added Prompt",
		Messages: []PromptMessage{
			{Role: "user", Content: "Added prompt content"},
		},
	}
	
	modifiedPrompt := Prompt{
		ID:   "modified-prompt",
		Name: "Modified Prompt",
		Messages: []PromptMessage{
			{Role: "user", Content: "Modified prompt content"},
		},
	}
	
	// Test PromptsChangedParams creation
	params := PromptsChangedParams{
		Added:    []Prompt{addedPrompt},
		Removed:  []string{"removed-prompt-1", "removed-prompt-2"},
		Modified: []Prompt{modifiedPrompt},
	}
	
	// Verify fields
	if len(params.Added) != 1 {
		t.Errorf("Expected 1 added prompt, got %d", len(params.Added))
	}
	
	if len(params.Removed) != 2 {
		t.Errorf("Expected 2 removed prompt IDs, got %d", len(params.Removed))
	}
	
	if len(params.Modified) != 1 {
		t.Errorf("Expected 1 modified prompt, got %d", len(params.Modified))
	}
	
	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal PromptsChangedParams: %v", err)
	}
	
	var decoded PromptsChangedParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal PromptsChangedParams: %v", err)
	}
	
	// Verify decoded data
	if len(decoded.Added) != len(params.Added) {
		t.Errorf("Expected %d added prompts after decoding, got %d", len(params.Added), len(decoded.Added))
	}
	
	if len(decoded.Removed) != len(params.Removed) {
		t.Errorf("Expected %d removed prompt IDs after decoding, got %d", len(params.Removed), len(decoded.Removed))
	}
	
	if len(decoded.Modified) != len(params.Modified) {
		t.Errorf("Expected %d modified prompts after decoding, got %d", len(params.Modified), len(decoded.Modified))
	}
}
