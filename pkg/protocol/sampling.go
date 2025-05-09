package protocol

import "encoding/json"

// Message represents a chat message in the MCP protocol
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// SampleParams defines parameters for the sample request
type SampleParams struct {
	Messages          []Message              `json:"messages"`
	ModelPreferences  *ModelPreferences      `json:"modelPreferences,omitempty"`
	SystemPrompt      string                 `json:"systemPrompt,omitempty"`
	IncludeContext    bool                   `json:"includeContext,omitempty"`
	ResourceRefs      []string               `json:"resourceRefs,omitempty"`
	AdditionalContext json.RawMessage        `json:"additionalContext,omitempty"`
	RequestID         string                 `json:"requestId,omitempty"`
	MaxTokens         int                    `json:"maxTokens,omitempty"`
	Stream            bool                   `json:"stream,omitempty"`
	Options           map[string]interface{} `json:"options,omitempty"`
}

// SampleResult defines the response for the sample request
type SampleResult struct {
	Content      string          `json:"content"`
	Model        string          `json:"model,omitempty"`
	FinishReason string          `json:"finishReason,omitempty"`
	Usage        *TokenUsage     `json:"usage,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// TokenUsage provides information about token usage in a completion
type TokenUsage struct {
	PromptTokens     int `json:"promptTokens,omitempty"`
	CompletionTokens int `json:"completionTokens,omitempty"`
	TotalTokens      int `json:"totalTokens,omitempty"`
}

// SampleStreamParams defines parameters for streamed sample results
type SampleStreamParams struct {
	RequestID    string `json:"requestId"`
	Content      string `json:"content"`
	IsFinal      bool   `json:"isFinal,omitempty"`
	FinishReason string `json:"finishReason,omitempty"`
}

// Roots components for context management

// Root represents a collection of resources
type Root struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	ResourceURIs []string `json:"resourceUris"`
	Tags         []string `json:"tags,omitempty"`
}

// ListRootsParams defines parameters for listing roots
type ListRootsParams struct {
	Tag string `json:"tag,omitempty"`
	PaginationParams
}

// ListRootsResult defines the response for listing roots
type ListRootsResult struct {
	Roots []Root `json:"roots"`
	PaginationResult
}

// RootsChangedParams defines parameters for the rootsChanged notification
type RootsChangedParams struct {
	Added    []Root   `json:"added,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	Modified []Root   `json:"modified,omitempty"`
}
