package protocol

import "context"

// ReceiveHandler is called when raw message data is received.
// It processes the raw byte data of an incoming message.
type ReceiveHandler func(data []byte)

// RequestHandler handles incoming requests
type RequestHandler func(ctx context.Context, params interface{}) (interface{}, error)

// NotificationHandler handles incoming notifications
type NotificationHandler func(ctx context.Context, params interface{}) error

// ProgressHandler handles progress notifications for streaming operations
type ProgressHandler func(params interface{}) error

// PaginationRequest defines request parameters for paginated results.
type PaginationRequest struct {
	// Limit specifies the maximum number of items to return.
	Limit int `json:"limit,omitempty"`

	// Offset specifies the number of items to skip.
	Offset int `json:"offset,omitempty"`

	// After specifies a cursor for continuing from a previous result.
	After string `json:"after,omitempty"`
}

// CompletionRequest defines parameters for a completion request.
type CompletionRequest struct {
	// Prompt is the input text to complete.
	Prompt string `json:"prompt"`

	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness in generation.
	Temperature float64 `json:"temperature,omitempty"`

	// Options contains additional completion options.
	Options map[string]interface{} `json:"options,omitempty"`
}

// CompletionResponse contains the result of a completion operation.
type CompletionResponse struct {
	// Text is the generated completion text.
	Text string `json:"text"`

	// FinishReason indicates why completion stopped.
	FinishReason string `json:"finish_reason,omitempty"`

	// Usage contains token usage information.
	Usage *CompletionUsage `json:"usage,omitempty"`
}

// CompletionUsage contains token usage information for a completion.
type CompletionUsage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the total number of tokens.
	TotalTokens int `json:"total_tokens"`
}

// SamplingEvent represents an event during sampling.
type SamplingEvent struct {
	// Type is the type of sampling event.
	Type string `json:"type"`

	// Data contains event-specific data.
	Data interface{} `json:"data"`

	// TokenID is the ID of the token this event relates to.
	TokenID string `json:"token_id,omitempty"`
}
