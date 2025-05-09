package protocol

import (
	"encoding/json"
)

// Tool represents a tool in the MCP protocol
type Tool struct {
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	InputSchema   json.RawMessage  `json:"inputSchema,omitempty"`
	OutputSchema  json.RawMessage  `json:"outputSchema,omitempty"`
	Examples      []ToolExample    `json:"examples,omitempty"`
	Annotations   []ToolAnnotation `json:"annotations,omitempty"`
	Documentation string           `json:"documentation,omitempty"`
	Type          string           `json:"type,omitempty"`
	Async         bool             `json:"async,omitempty"`
	Categories    []string         `json:"categories,omitempty"`
}

// ToolExample provides an example of tool usage
type ToolExample struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Input        json.RawMessage `json:"input"`
	Output       json.RawMessage `json:"output"`
	ExtraContext string          `json:"extraContext,omitempty"`
}

// ToolAnnotation provides additional metadata for a tool
type ToolAnnotation struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// ListToolsParams defines parameters for listing tools
type ListToolsParams struct {
	Category string `json:"category,omitempty"`
	PaginationParams
}

// ListToolsResult defines the response for listing tools
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
	PaginationResult
}

// CallToolParams defines parameters for calling a tool
type CallToolParams struct {
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input,omitempty"`
	Context json.RawMessage `json:"context,omitempty"`
}

// CallToolResult defines the response for tool calls
type CallToolResult struct {
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
	Partial       bool            `json:"partial,omitempty"`
	OperationID   string          `json:"operationId,omitempty"`
	Documentation string          `json:"documentation,omitempty"`
}

// ToolsChangedParams defines parameters for the toolsChanged notification
type ToolsChangedParams struct {
	Added    []Tool   `json:"added,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	Modified []Tool   `json:"modified,omitempty"`
}
