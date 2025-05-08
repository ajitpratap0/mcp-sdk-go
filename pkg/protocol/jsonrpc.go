package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	// JSONRPCVersion is the supported JSON-RPC version
	JSONRPCVersion = "2.0"
)

// ErrorCode represents standard JSON-RPC 2.0 error codes
type ErrorCode int

// Standard error codes as per JSON-RPC 2.0 specification
const (
	ParseError     ErrorCode = -32700
	InvalidRequest ErrorCode = -32600
	MethodNotFound ErrorCode = -32601
	InvalidParams  ErrorCode = -32602
	InternalError  ErrorCode = -32603
)

// MCP-specific error codes
const (
	// ServerInitError indicates an error during server initialization
	ServerInitError ErrorCode = -32000
	// UnauthorizedError indicates the client is not authorized to make a request
	UnauthorizedError ErrorCode = -32001
	// ResourceNotFound indicates a requested resource was not found
	ResourceNotFound ErrorCode = -32002
	// OperationCancelled indicates an operation was cancelled
	OperationCancelled ErrorCode = -32003
	// InvalidCapability indicates a requested capability is not supported
	InvalidCapability ErrorCode = -32004
)

// JSONRPCMessage represents a JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string `json:"jsonrpc"`
}

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPCMessage
	ID     interface{}     `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// NewRequest creates a new JSON-RPC 2.0 request
func NewRequest(id interface{}, method string, params interface{}) (*Request, error) {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	return &Request{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPCMessage
	ID     interface{}     `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// NewResponse creates a new JSON-RPC 2.0 success response
func NewResponse(id interface{}, result interface{}) (*Response, error) {
	var resultJSON json.RawMessage
	if result != nil {
		var err error
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
	}

	return &Response{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		ID:      id,
		Result:  resultJSON,
	}, nil
}

// NewErrorResponse creates a new JSON-RPC 2.0 error response
func NewErrorResponse(id interface{}, code ErrorCode, message string, data interface{}) (*Response, error) {
	var dataJSON interface{}
	if data != nil {
		var err error
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal error data: %w", err)
		}
		dataJSON = json.RawMessage(dataBytes)
	}

	return &Response{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    dataJSON,
		},
	}, nil
}

// Notification represents a JSON-RPC 2.0 notification
type Notification struct {
	JSONRPCMessage
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// NewNotification creates a new JSON-RPC 2.0 notification
func NewNotification(method string, params interface{}) (*Notification, error) {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	return &Notification{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

// Error represents a JSON-RPC 2.0 error object
type Error struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// IsRequest checks if a raw JSON message is a JSON-RPC 2.0 request
func IsRequest(data []byte) bool {
	var msg struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Method  string      `json:"method"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return msg.JSONRPC == JSONRPCVersion && msg.ID != nil && msg.Method != ""
}

// IsResponse checks if a raw JSON message is a JSON-RPC 2.0 response
func IsResponse(data []byte) bool {
	var msg struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   *Error      `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return msg.JSONRPC == JSONRPCVersion && msg.ID != nil && (msg.Result != nil || msg.Error != nil)
}

// IsNotification checks if a raw JSON message is a JSON-RPC 2.0 notification
func IsNotification(data []byte) bool {
	var msg struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Method  string      `json:"method"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return msg.JSONRPC == JSONRPCVersion && msg.ID == nil && msg.Method != ""
}
