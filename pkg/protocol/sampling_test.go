package protocol

import (
	"encoding/json"
	"testing"
)

func TestMessage(t *testing.T) {
	// Test basic message creation and fields
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
		Name:    "test-user",
	}

	if msg.Role != "user" {
		t.Errorf("Expected Role to be 'user', got %q", msg.Role)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("Expected Content to be 'Hello, world!', got %q", msg.Content)
	}

	if msg.Name != "test-user" {
		t.Errorf("Expected Name to be 'test-user', got %q", msg.Name)
	}

	// Test JSON serialization
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Message: %v", err)
	}

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal Message: %v", err)
	}

	if decoded.Role != msg.Role {
		t.Errorf("Expected decoded Role to be %q, got %q", msg.Role, decoded.Role)
	}

	if decoded.Content != msg.Content {
		t.Errorf("Expected decoded Content to be %q, got %q", msg.Content, decoded.Content)
	}

	if decoded.Name != msg.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", msg.Name, decoded.Name)
	}
}

func TestSampleParams(t *testing.T) {
	// Test basic SampleParams creation and fields
	params := SampleParams{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		},
		SystemPrompt:   "You are a helpful assistant",
		IncludeContext: true,
		ResourceRefs:   []string{"resource1", "resource2"},
		RequestID:      "sample-123",
		MaxTokens:      100,
		Stream:         true,
		Options: map[string]interface{}{
			"temperature": 0.7,
			"topP":        0.9,
		},
	}

	if len(params.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(params.Messages))
	}

	if params.Messages[0].Role != "user" {
		t.Errorf("Expected first message role to be 'user', got %q", params.Messages[0].Role)
	}

	if params.SystemPrompt != "You are a helpful assistant" {
		t.Errorf("Expected SystemPrompt to be 'You are a helpful assistant', got %q", params.SystemPrompt)
	}

	if !params.IncludeContext {
		t.Error("Expected IncludeContext to be true")
	}

	if len(params.ResourceRefs) != 2 {
		t.Errorf("Expected 2 resource refs, got %d", len(params.ResourceRefs))
	}

	if params.RequestID != "sample-123" {
		t.Errorf("Expected RequestID to be 'sample-123', got %q", params.RequestID)
	}

	if params.MaxTokens != 100 {
		t.Errorf("Expected MaxTokens to be 100, got %d", params.MaxTokens)
	}

	if !params.Stream {
		t.Error("Expected Stream to be true")
	}

	if params.Options["temperature"] != 0.7 {
		t.Errorf("Expected Options['temperature'] to be 0.7, got %v", params.Options["temperature"])
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal SampleParams: %v", err)
	}

	var decoded SampleParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SampleParams: %v", err)
	}

	if len(decoded.Messages) != len(params.Messages) {
		t.Errorf("Expected %d messages after decoding, got %d", len(params.Messages), len(decoded.Messages))
	}

	if decoded.SystemPrompt != params.SystemPrompt {
		t.Errorf("Expected decoded SystemPrompt to be %q, got %q", params.SystemPrompt, decoded.SystemPrompt)
	}
}

func TestSampleResult(t *testing.T) {
	// Test basic SampleResult creation and fields
	result := SampleResult{
		Content:      "This is a sample response",
		Model:        "gpt-4",
		FinishReason: "stop",
		Usage: &TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
		Metadata: json.RawMessage(`{"custom":"value"}`),
	}

	if result.Content != "This is a sample response" {
		t.Errorf("Expected Content to be 'This is a sample response', got %q", result.Content)
	}

	if result.Model != "gpt-4" {
		t.Errorf("Expected Model to be 'gpt-4', got %q", result.Model)
	}

	if result.FinishReason != "stop" {
		t.Errorf("Expected FinishReason to be 'stop', got %q", result.FinishReason)
	}

	if result.Usage == nil {
		t.Fatal("Expected Usage to not be nil")
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("Expected Usage.PromptTokens to be 10, got %d", result.Usage.PromptTokens)
	}

	if result.Usage.CompletionTokens != 5 {
		t.Errorf("Expected Usage.CompletionTokens to be 5, got %d", result.Usage.CompletionTokens)
	}

	if result.Usage.TotalTokens != 15 {
		t.Errorf("Expected Usage.TotalTokens to be 15, got %d", result.Usage.TotalTokens)
	}

	// Test JSON serialization
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal SampleResult: %v", err)
	}

	var decoded SampleResult
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SampleResult: %v", err)
	}

	if decoded.Content != result.Content {
		t.Errorf("Expected decoded Content to be %q, got %q", result.Content, decoded.Content)
	}

	if decoded.Model != result.Model {
		t.Errorf("Expected decoded Model to be %q, got %q", result.Model, decoded.Model)
	}
}

func TestTokenUsage(t *testing.T) {
	// Test TokenUsage creation and fields
	usage := TokenUsage{
		PromptTokens:     25,
		CompletionTokens: 15,
		TotalTokens:      40,
	}

	if usage.PromptTokens != 25 {
		t.Errorf("Expected PromptTokens to be 25, got %d", usage.PromptTokens)
	}

	if usage.CompletionTokens != 15 {
		t.Errorf("Expected CompletionTokens to be 15, got %d", usage.CompletionTokens)
	}

	if usage.TotalTokens != 40 {
		t.Errorf("Expected TotalTokens to be 40, got %d", usage.TotalTokens)
	}

	// Test JSON serialization
	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatalf("Failed to marshal TokenUsage: %v", err)
	}

	var decoded TokenUsage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal TokenUsage: %v", err)
	}

	if decoded.PromptTokens != usage.PromptTokens {
		t.Errorf("Expected decoded PromptTokens to be %d, got %d", usage.PromptTokens, decoded.PromptTokens)
	}
}

func TestRoot(t *testing.T) {
	// Test Root creation and fields
	root := Root{
		ID:           "root-123",
		Name:         "Test Root",
		Description:  "A test root for resources",
		ResourceURIs: []string{"uri/1", "uri/2"},
		Tags:         []string{"tag1", "tag2"},
	}

	if root.ID != "root-123" {
		t.Errorf("Expected ID to be 'root-123', got %q", root.ID)
	}

	if root.Name != "Test Root" {
		t.Errorf("Expected Name to be 'Test Root', got %q", root.Name)
	}

	if root.Description != "A test root for resources" {
		t.Errorf("Expected Description to be 'A test root for resources', got %q", root.Description)
	}

	if len(root.ResourceURIs) != 2 {
		t.Errorf("Expected 2 ResourceURIs, got %d", len(root.ResourceURIs))
	}

	if len(root.Tags) != 2 {
		t.Errorf("Expected 2 Tags, got %d", len(root.Tags))
	}

	// Test JSON serialization
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("Failed to marshal Root: %v", err)
	}

	var decoded Root
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal Root: %v", err)
	}

	if decoded.ID != root.ID {
		t.Errorf("Expected decoded ID to be %q, got %q", root.ID, decoded.ID)
	}
}

func TestRootsChangedParams(t *testing.T) {
	// Test RootsChangedParams creation and fields
	params := RootsChangedParams{
		Added: []Root{
			{ID: "root-1", Name: "New Root"},
		},
		Removed: []string{"root-old"},
		Modified: []Root{
			{ID: "root-2", Name: "Updated Root"},
		},
	}

	if len(params.Added) != 1 {
		t.Errorf("Expected 1 Added root, got %d", len(params.Added))
	}

	if params.Added[0].ID != "root-1" {
		t.Errorf("Expected Added[0].ID to be 'root-1', got %q", params.Added[0].ID)
	}

	if len(params.Removed) != 1 {
		t.Errorf("Expected 1 Removed root, got %d", len(params.Removed))
	}

	if params.Removed[0] != "root-old" {
		t.Errorf("Expected Removed[0] to be 'root-old', got %q", params.Removed[0])
	}

	if len(params.Modified) != 1 {
		t.Errorf("Expected 1 Modified root, got %d", len(params.Modified))
	}

	if params.Modified[0].ID != "root-2" {
		t.Errorf("Expected Modified[0].ID to be 'root-2', got %q", params.Modified[0].ID)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal RootsChangedParams: %v", err)
	}

	var decoded RootsChangedParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal RootsChangedParams: %v", err)
	}

	if len(decoded.Added) != len(params.Added) {
		t.Errorf("Expected %d Added roots after decoding, got %d", len(params.Added), len(decoded.Added))
	}
}
