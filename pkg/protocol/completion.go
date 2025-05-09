package protocol

import "encoding/json"

// CompleteParams defines parameters for the complete request
type CompleteParams struct {
	PromptID         string                 `json:"promptId,omitempty"`
	Messages         []Message              `json:"messages,omitempty"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	ResourceRefs     []string               `json:"resourceRefs,omitempty"`
	ModelPreferences *ModelPreferences      `json:"modelPreferences,omitempty"`
	SystemPrompt     string                 `json:"systemPrompt,omitempty"`
	Context          json.RawMessage        `json:"context,omitempty"`
}

// CompleteResult defines the response for the complete request
type CompleteResult struct {
	Content      string          `json:"content"`
	Model        string          `json:"model,omitempty"`
	FinishReason string          `json:"finishReason,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// ModelPreferences defines preferences for AI model selection and parameters
type ModelPreferences struct {
	Model             string                 `json:"model,omitempty"`
	ProviderID        string                 `json:"providerId,omitempty"`
	Temperature       float64                `json:"temperature,omitempty"`
	TopP              float64                `json:"topP,omitempty"`
	MaxTokens         int                    `json:"maxTokens,omitempty"`
	StopSequences     []string               `json:"stopSequences,omitempty"`
	FrequencyPenalty  float64                `json:"frequencyPenalty,omitempty"`
	PresencePenalty   float64                `json:"presencePenalty,omitempty"`
	ResponseFormat    *ResponseFormat        `json:"responseFormat,omitempty"`
	AdditionalOptions map[string]interface{} `json:"additionalOptions,omitempty"`
}

// ResponseFormat defines the expected format of the completion response
type ResponseFormat struct {
	Type    string                 `json:"type"`
	Schema  json.RawMessage        `json:"schema,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}
