package protocol

import (
	"fmt"
	"time"
)

const (
	// Current protocol revision
	ProtocolRevision = "2025-03-26"

	// Methods for lifecycle management
	MethodInitialize  = "initialize"
	MethodInitialized = "initialized"

	// Methods for capability management
	MethodSetCapability = "setCapability"
	MethodCapability    = "capability"

	// Methods for server features
	MethodListTools         = "listTools"
	MethodCallTool          = "callTool"
	MethodToolsChanged      = "toolsChanged"
	MethodListResources     = "listResources"
	MethodReadResource      = "readResource"
	MethodResourcesChanged  = "resourcesChanged"
	MethodSubscribeResource = "subscribeResource"
	MethodResourceUpdated   = "resourceUpdated"
	MethodListPrompts       = "listPrompts"
	MethodGetPrompt         = "getPrompt"
	MethodPromptsChanged    = "promptsChanged"
	MethodComplete          = "complete"
	MethodListRoots         = "listRoots"
	MethodRootsChanged      = "rootsChanged"

	// Methods for client features
	MethodSample = "sample"

	// Methods for utilities
	MethodCancel      = "cancel"
	MethodPing        = "ping"
	MethodProgress    = "progress"
	MethodSetLogLevel = "setLogLevel"
	MethodLog         = "log"
)

// CapabilityType defines the types of capabilities in MCP
type CapabilityType string

const (
	// CapabilityTools indicates the server supports tools
	CapabilityTools CapabilityType = "tools"

	// CapabilityResources indicates the server supports resources
	CapabilityResources CapabilityType = "resources"

	// CapabilityResourceSubscriptions indicates the server supports resource subscriptions
	CapabilityResourceSubscriptions CapabilityType = "resourceSubscriptions"

	// CapabilityPrompts indicates the server supports prompts
	CapabilityPrompts CapabilityType = "prompts"

	// CapabilityComplete indicates the server supports completions
	CapabilityComplete CapabilityType = "complete"

	// CapabilityRoots indicates client or server supports roots
	CapabilityRoots CapabilityType = "roots"

	// CapabilitySampling indicates client supports sampling
	CapabilitySampling CapabilityType = "sampling"

	// CapabilityLogging indicates client or server supports logging
	CapabilityLogging CapabilityType = "logging"

	// CapabilityPagination indicates server supports pagination
	CapabilityPagination CapabilityType = "pagination"
)

// InitializeParams defines the parameters for the initialize request
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Name            string                 `json:"name"`
	Version         string                 `json:"version"`
	Capabilities    map[string]bool        `json:"capabilities"`
	ClientInfo      *ClientInfo            `json:"clientInfo,omitempty"`
	Trace           string                 `json:"trace,omitempty"`
	FeatureOptions  map[string]interface{} `json:"featureOptions,omitempty"`
}

// ClientInfo provides additional information about the client
type ClientInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Platform string `json:"platform,omitempty"`
}

// InitializeResult defines the response for the initialize request
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Name            string                 `json:"name"`
	Version         string                 `json:"version"`
	Capabilities    map[string]bool        `json:"capabilities"`
	ServerInfo      *ServerInfo            `json:"serverInfo,omitempty"`
	FeatureOptions  map[string]interface{} `json:"featureOptions,omitempty"`
}

// ServerInfo provides additional information about the server
type ServerInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
}

// InitializedParams is sent as a notification once the client is ready
type InitializedParams struct {
	// Intentionally empty as per specification
}

// SetCapabilityParams is used to dynamically set capabilities
type SetCapabilityParams struct {
	Capability string `json:"capability"`
	Value      bool   `json:"value"`
}

// SetCapabilityResult is the response for setCapability
type SetCapabilityResult struct {
	Success bool `json:"success"`
}

// CancelParams defines parameters for the cancel request
type CancelParams struct {
	ID interface{} `json:"id"`
}

// CancelResult is the response for cancel
type CancelResult struct {
	Cancelled bool `json:"cancelled"`
}

// PingParams defines parameters for the ping request
type PingParams struct {
	// Optional timestamp from sender
	Timestamp int64 `json:"timestamp,omitempty"`
}

// PingResult is the response for ping
type PingResult struct {
	// The original timestamp if provided, otherwise the server's current timestamp
	Timestamp int64 `json:"timestamp"`
}

// ProgressParams defines parameters for the progress notification
type ProgressParams struct {
	ID        interface{} `json:"id"`
	Message   string      `json:"message,omitempty"`
	Percent   float64     `json:"percent,omitempty"`
	Completed bool        `json:"completed,omitempty"`
}

// LogLevel specifies the severity of log messages
type LogLevel string

const (
	// LogLevelTrace for highly detailed tracing information
	LogLevelTrace LogLevel = "trace"

	// LogLevelDebug for debug information
	LogLevelDebug LogLevel = "debug"

	// LogLevelInfo for general information
	LogLevelInfo LogLevel = "info"

	// LogLevelWarn for warnings
	LogLevelWarn LogLevel = "warn"

	// LogLevelError for errors
	LogLevelError LogLevel = "error"

	// LogLevelFatal for fatal errors
	LogLevelFatal LogLevel = "fatal"

	// LogLevelOff to disable logging
	LogLevelOff LogLevel = "off"
)

// SetLogLevelParams defines parameters for the setLogLevel request
type SetLogLevelParams struct {
	Level LogLevel `json:"level"`
}

// SetLogLevelResult is the response for setLogLevel
type SetLogLevelResult struct {
	Success bool `json:"success"`
}

// LogParams defines parameters for the log notification
type LogParams struct {
	Level   LogLevel    `json:"level"`
	Message string      `json:"message"`
	Source  string      `json:"source,omitempty"`
	Time    time.Time   `json:"time,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorObject defines the structure for a JSON-RPC error.
type ErrorObject struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error returns a string representation of the ErrorObject.
func (e *ErrorObject) Error() string {
	return fmt.Sprintf("rpc error: code = %d desc = %s", e.Code, e.Message)
}

// PaginationParams for requests that support pagination
type PaginationParams struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

// PaginationResult for responses that support pagination
type PaginationResult struct {
	Items      []interface{} `json:"items"`
	TotalCount int           `json:"totalCount,omitempty"`
	NextCursor string        `json:"nextCursor,omitempty"`
	HasMore    bool          `json:"hasMore"`
}

// BatchRequest represents a batch of multiple requests
type BatchRequest struct {
	Requests []Request `json:"requests"`
}

// BatchResponse represents responses for a batch of requests
type BatchResponse struct {
	Responses []Response `json:"responses"`
}
