package protocol

import "encoding/json"

// Prompt represents a prompt in the MCP protocol
type Prompt struct {
	ID          string             `json:"id"`
	Name        string             `json:"name,omitempty"`
	Description string             `json:"description,omitempty"`
	Messages    []PromptMessage    `json:"messages"`
	Parameters  []PromptParameter  `json:"parameters,omitempty"`
	Schema      json.RawMessage    `json:"schema,omitempty"`
	Examples    []PromptExample    `json:"examples,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
}

// PromptMessage defines a message in a prompt
type PromptMessage struct {
	Role        string          `json:"role"`
	Content     string          `json:"content"`
	Name        string          `json:"name,omitempty"`
	ResourceRefs []string       `json:"resourceRefs,omitempty"`
	Parameters   []string       `json:"parameters,omitempty"`
}

// PromptParameter defines a parameter for a prompt
type PromptParameter struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// PromptExample provides an example of prompt usage
type PromptExample struct {
	Name       string                 `json:"name"`
	Description string                `json:"description,omitempty"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     string                 `json:"result,omitempty"`
}

// ListPromptsParams defines parameters for listing prompts
type ListPromptsParams struct {
	Tag string `json:"tag,omitempty"`
	PaginationParams
}

// ListPromptsResult defines the response for listing prompts
type ListPromptsResult struct {
	Prompts []Prompt `json:"prompts"`
	PaginationResult
}

// GetPromptParams defines parameters for getting a prompt
type GetPromptParams struct {
	ID string `json:"id"`
}

// GetPromptResult defines the response for getting a prompt
type GetPromptResult struct {
	Prompt Prompt `json:"prompt"`
}

// PromptsChangedParams defines parameters for the promptsChanged notification
type PromptsChangedParams struct {
	Added    []Prompt `json:"added,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	Modified []Prompt `json:"modified,omitempty"`
}
