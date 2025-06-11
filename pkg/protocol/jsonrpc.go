package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	// JSONRPCVersion is the supported JSON-RPC version
	JSONRPCVersion = "2.0"
)

// Standard error codes as per JSON-RPC 2.0 specification
const (
	ParseError     int = -32700
	InvalidRequest int = -32600
	MethodNotFound int = -32601
	InvalidParams  int = -32602
	InternalError  int = -32603
)

// MCP-specific error codes
const (
	// ServerInitError indicates an error during server initialization
	ServerInitError int = -32000
	// UnauthorizedError indicates the client is not authorized to make a request
	UnauthorizedError int = -32001
	// ResourceNotFound indicates a requested resource was not found
	ResourceNotFound int = -32002
	// OperationCancelled indicates an operation was cancelled
	OperationCancelled int = -32003
	// InvalidCapability indicates a requested capability is not supported
	InvalidCapability int = -32004
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
		ID:             id,
		Method:         method,
		Params:         paramsJSON,
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
		ID:             id,
		Result:         resultJSON,
	}, nil
}

// NewErrorResponse creates a new JSON-RPC 2.0 error response
func NewErrorResponse(id interface{}, code int, message string, data interface{}) (*Response, error) {
	var dataJSON json.RawMessage
	var err error
	if data != nil {
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal error data: %w", err)
		}
	}

	err2 := &Error{
		Code:    code,
		Message: message,
	}

	// Only set Data field if we actually have data
	if data != nil {
		err2.Data = dataJSON
	}

	return &Response{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		ID:             id,
		Error:          err2,
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
		Method:         method,
		Params:         paramsJSON,
	}, nil
}

// Error represents a JSON-RPC error object.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error returns the error message string, satisfying the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("jsonrpc: code %d, message: %s", e.Code, e.Message)
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

// JSONRPCBatchRequest represents a JSON-RPC 2.0 batch request
// It's an array of Request and/or Notification objects (JSON-RPC 2.0 compliant)
type JSONRPCBatchRequest []interface{}

// JSONRPCBatchResponse represents a JSON-RPC 2.0 batch response
// It's an array of Response objects corresponding to the requests in the batch
type JSONRPCBatchResponse []*Response

// NewJSONRPCBatchRequest creates a new JSON-RPC 2.0 batch request from a slice of requests and notifications
func NewJSONRPCBatchRequest(items ...interface{}) (*JSONRPCBatchRequest, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("batch request cannot be empty")
	}

	batch := make(JSONRPCBatchRequest, 0, len(items))
	for i, item := range items {
		switch v := item.(type) {
		case *Request:
			if v == nil {
				return nil, fmt.Errorf("request at index %d is nil", i)
			}
			batch = append(batch, v)
		case *Notification:
			if v == nil {
				return nil, fmt.Errorf("notification at index %d is nil", i)
			}
			batch = append(batch, v)
		default:
			return nil, fmt.Errorf("invalid item type at index %d: expected *Request or *Notification, got %T", i, item)
		}
	}

	return &batch, nil
}

// NewJSONRPCBatchResponse creates a new JSON-RPC 2.0 batch response from a slice of responses
func NewJSONRPCBatchResponse(responses ...*Response) *JSONRPCBatchResponse {
	// Filter out nil responses
	filteredResponses := make([]*Response, 0, len(responses))
	for _, resp := range responses {
		if resp != nil {
			filteredResponses = append(filteredResponses, resp)
		}
	}

	batch := JSONRPCBatchResponse(filteredResponses)
	return &batch
}

// IsEmpty returns true if the batch response is empty
func (br *JSONRPCBatchResponse) IsEmpty() bool {
	return br == nil || len(*br) == 0
}

// Add appends a response to the batch response
func (br *JSONRPCBatchResponse) Add(response *Response) {
	if br != nil && response != nil {
		*br = append(*br, response)
	}
}

// IsBatch checks if a raw JSON message is a batch request/response
func IsBatch(data []byte) bool {
	// Try to unmarshal as an array
	var arr []interface{}
	err := json.Unmarshal(data, &arr)
	// Check if unmarshal succeeded and array is not nil
	return err == nil && arr != nil
}

// ParseJSONRPCBatchRequest parses raw JSON data as a JSON-RPC 2.0 batch request
func ParseJSONRPCBatchRequest(data []byte) (*JSONRPCBatchRequest, error) {
	if !IsBatch(data) {
		return nil, fmt.Errorf("data is not a valid batch request")
	}

	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch request: %w", err)
	}

	if len(rawItems) == 0 {
		return nil, fmt.Errorf("batch request cannot be empty")
	}

	batch := make(JSONRPCBatchRequest, 0, len(rawItems))
	for i, rawItem := range rawItems {
		// Check if it's a request or notification
		if IsRequest(rawItem) {
			var req Request
			if err := json.Unmarshal(rawItem, &req); err != nil {
				return nil, fmt.Errorf("failed to unmarshal request at index %d: %w", i, err)
			}
			batch = append(batch, &req)
		} else if IsNotification(rawItem) {
			var notif Notification
			if err := json.Unmarshal(rawItem, &notif); err != nil {
				return nil, fmt.Errorf("failed to unmarshal notification at index %d: %w", i, err)
			}
			batch = append(batch, &notif)
		} else {
			return nil, fmt.Errorf("invalid JSON-RPC object at index %d", i)
		}
	}

	return &batch, nil
}

// ParseJSONRPCBatchResponse parses raw JSON data as a JSON-RPC 2.0 batch response
func ParseJSONRPCBatchResponse(data []byte) (*JSONRPCBatchResponse, error) {
	if !IsBatch(data) {
		return nil, fmt.Errorf("data is not a valid batch response")
	}

	var responses []*Response
	if err := json.Unmarshal(data, &responses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch response: %w", err)
	}

	batch := JSONRPCBatchResponse(responses)
	return &batch, nil
}

// ToJSON serializes the batch request to JSON
func (br *JSONRPCBatchRequest) ToJSON() ([]byte, error) {
	if br == nil {
		return nil, fmt.Errorf("batch request is nil")
	}
	return json.Marshal(*br)
}

// ToJSON serializes the batch response to JSON
func (br *JSONRPCBatchResponse) ToJSON() ([]byte, error) {
	if br == nil {
		return nil, fmt.Errorf("batch response is nil")
	}
	return json.Marshal(*br)
}

// GetRequests returns all Request objects from the batch
func (br *JSONRPCBatchRequest) GetRequests() []*Request {
	if br == nil {
		return nil
	}

	var requests []*Request
	for _, item := range *br {
		if req, ok := item.(*Request); ok {
			requests = append(requests, req)
		}
	}
	return requests
}

// GetNotifications returns all Notification objects from the batch
func (br *JSONRPCBatchRequest) GetNotifications() []*Notification {
	if br == nil {
		return nil
	}

	var notifications []*Notification
	for _, item := range *br {
		if notif, ok := item.(*Notification); ok {
			notifications = append(notifications, notif)
		}
	}
	return notifications
}

// Len returns the number of items in the batch request
func (br *JSONRPCBatchRequest) Len() int {
	if br == nil {
		return 0
	}
	return len(*br)
}

// Len returns the number of responses in the batch response
func (br *JSONRPCBatchResponse) Len() int {
	if br == nil {
		return 0
	}
	return len(*br)
}
