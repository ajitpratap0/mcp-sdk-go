package protocol

import (
	"encoding/json"
	"testing"
)

func TestCompleteParams(t *testing.T) {
	// Test basic CompleteParams creation
	params := CompleteParams{
		SystemPrompt: "You are a helpful assistant.",
		Messages: []Message{
			{Role: "user", Content: "func main() {"},
		},
		ModelPreferences: &ModelPreferences{
			Model:     "gpt-4",
			MaxTokens: 50,
			Temperature: 0.7,
			TopP: 1.0,
		},
	}

	// Verify fields
	if len(params.Messages) != 1 || params.Messages[0].Content != "func main() {" {
		t.Errorf("Expected Message content to be 'func main() {', got %q", params.Messages[0].Content)
	}

	if params.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("Expected SystemPrompt to be 'You are a helpful assistant.', got %q", params.SystemPrompt)
	}

	if params.ModelPreferences == nil {
		t.Fatal("Expected ModelPreferences to not be nil")
	}

	if params.ModelPreferences.MaxTokens != 50 {
		t.Errorf("Expected ModelPreferences.MaxTokens to be 50, got %d", params.ModelPreferences.MaxTokens)
	}

	if params.ModelPreferences.Temperature != 0.7 {
		t.Errorf("Expected ModelPreferences.Temperature to be 0.7, got %v", params.ModelPreferences.Temperature)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal CompletionParams: %v", err)
	}

	var decoded CompleteParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal CompleteParams: %v", err)
	}

	// Verify decoded data
	if len(decoded.Messages) != len(params.Messages) {
		t.Errorf("Expected decoded Messages length to be %d, got %d", len(params.Messages), len(decoded.Messages))
	}

	if decoded.SystemPrompt != params.SystemPrompt {
		t.Errorf("Expected decoded SystemPrompt to be %q, got %q", params.SystemPrompt, decoded.SystemPrompt)
	}

	if decoded.ModelPreferences.MaxTokens != params.ModelPreferences.MaxTokens {
		t.Errorf("Expected decoded ModelPreferences.MaxTokens to be %d, got %d", params.ModelPreferences.MaxTokens, decoded.ModelPreferences.MaxTokens)
	}

	if decoded.ModelPreferences.Temperature != params.ModelPreferences.Temperature {
		t.Errorf("Expected decoded ModelPreferences.Temperature to be %v, got %v", params.ModelPreferences.Temperature, decoded.ModelPreferences.Temperature)
	}
}

func TestCompleteResult(t *testing.T) {
	// Test CompleteResult creation
	result := CompleteResult{
		Content: "    fmt.Println(\"Hello, World!\")\n}",
		Model: "gpt-4",
		FinishReason: "stop",
		Metadata: json.RawMessage(`{"usage":{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18}}`),
	}

	// Verify fields
	if result.Content != "    fmt.Println(\"Hello, World!\")\n}" {
		t.Errorf("Expected Content to be '    fmt.Println(\"Hello, World!\")\n}', got %q", result.Content)
	}

	if result.Model != "gpt-4" {
		t.Errorf("Expected Model to be 'gpt-4', got %q", result.Model)
	}

	if result.FinishReason != "stop" {
		t.Errorf("Expected FinishReason to be 'stop', got %q", result.FinishReason)
	}

	if result.Metadata == nil {
		t.Fatal("Expected Metadata to not be nil")
	}

	// Test JSON serialization
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal CompletionResult: %v", err)
	}

	var decoded CompleteResult
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal CompleteResult: %v", err)
	}

	// Verify decoded data
	if decoded.Content != result.Content {
		t.Errorf("Expected decoded Content to be %q, got %q", result.Content, decoded.Content)
	}

	if decoded.Model != result.Model {
		t.Errorf("Expected decoded Model to be %q, got %q", result.Model, decoded.Model)
	}

	if decoded.FinishReason != result.FinishReason {
		t.Errorf("Expected decoded FinishReason to be %q, got %q", result.FinishReason, decoded.FinishReason)
	}
}

// This test has been removed as there's no equivalent struct in the protocol package
// The CompleteResult is used directly and there's no streaming version in the current implementation
