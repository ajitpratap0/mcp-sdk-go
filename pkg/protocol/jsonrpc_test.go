package protocol

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONRPCMessage(t *testing.T) {
	msg := JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
	}

	if msg.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC version to be '2.0', got %q", msg.JSONRPC)
	}
}

func TestNewRequest(t *testing.T) {
	// Test with nil params
	req, err := NewRequest("req-1", "test.method", nil)
	if err != nil {
		t.Fatalf("Expected NewRequest with nil params to succeed, got error: %v", err)
	}

	if req.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version to be %q, got %q", JSONRPCVersion, req.JSONRPC)
	}

	if req.ID != "req-1" {
		t.Errorf("Expected ID to be 'req-1', got %v", req.ID)
	}

	if req.Method != "test.method" {
		t.Errorf("Expected Method to be 'test.method', got %q", req.Method)
	}

	if len(req.Params) != 0 {
		t.Errorf("Expected Params to be empty, got %s", string(req.Params))
	}

	// Test with params
	params := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	req, err = NewRequest("req-2", "test.method", params)
	if err != nil {
		t.Fatalf("Expected NewRequest with params to succeed, got error: %v", err)
	}

	// Verify params were properly encoded
	var decodedParams map[string]interface{}
	err = json.Unmarshal(req.Params, &decodedParams)
	if err != nil {
		t.Fatalf("Failed to decode params: %v", err)
	}

	if decodedParams["key"] != "value" {
		t.Errorf("Expected params['key'] to be 'value', got %v", decodedParams["key"])
	}

	if int(decodedParams["num"].(float64)) != 42 {
		t.Errorf("Expected params['num'] to be 42, got %v", decodedParams["num"])
	}
}

func TestNewResponse(t *testing.T) {
	// Test with nil result
	resp, err := NewResponse("resp-1", nil)
	if err != nil {
		t.Fatalf("Expected NewResponse with nil result to succeed, got error: %v", err)
	}

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version to be %q, got %q", JSONRPCVersion, resp.JSONRPC)
	}

	if resp.ID != "resp-1" {
		t.Errorf("Expected ID to be 'resp-1', got %v", resp.ID)
	}

	if len(resp.Result) != 0 {
		t.Errorf("Expected Result to be empty, got %s", string(resp.Result))
	}

	if resp.Error != nil {
		t.Errorf("Expected Error to be nil, got %v", resp.Error)
	}

	// Test with result
	result := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	resp, err = NewResponse("resp-2", result)
	if err != nil {
		t.Fatalf("Expected NewResponse with result to succeed, got error: %v", err)
	}

	// Verify result was properly encoded
	var decodedResult map[string]interface{}
	err = json.Unmarshal(resp.Result, &decodedResult)
	if err != nil {
		t.Fatalf("Failed to decode result: %v", err)
	}

	if decodedResult["key"] != "value" {
		t.Errorf("Expected result['key'] to be 'value', got %v", decodedResult["key"])
	}

	if int(decodedResult["num"].(float64)) != 42 {
		t.Errorf("Expected result['num'] to be 42, got %v", decodedResult["num"])
	}
}

func TestNewErrorResponse(t *testing.T) {
	// Test with nil data
	resp, err := NewErrorResponse("err-1", InvalidRequest, "Invalid request", nil)
	if err != nil {
		t.Fatalf("Expected NewErrorResponse with nil data to succeed, got error: %v", err)
	}

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version to be %q, got %q", JSONRPCVersion, resp.JSONRPC)
	}

	if resp.ID != "err-1" {
		t.Errorf("Expected ID to be 'err-1', got %v", resp.ID)
	}

	if resp.Error == nil {
		t.Fatal("Expected Error to not be nil")
	}

	if resp.Error.Code != InvalidRequest {
		t.Errorf("Expected Error.Code to be %d, got %d", InvalidRequest, resp.Error.Code)
	}

	if resp.Error.Message != "Invalid request" {
		t.Errorf("Expected Error.Message to be 'Invalid request', got %q", resp.Error.Message)
	}

	if resp.Error.Data != nil {
		t.Errorf("Expected Error.Data to be nil, got %v", resp.Error.Data)
	}

	// Test with data
	data := map[string]interface{}{
		"details": "More information",
	}

	resp, err = NewErrorResponse("err-2", MethodNotFound, "Method not found", data)
	if err != nil {
		t.Fatalf("Expected NewErrorResponse with data to succeed, got error: %v", err)
	}

	if resp.Error.Code != MethodNotFound {
		t.Errorf("Expected Error.Code to be %d, got %d", MethodNotFound, resp.Error.Code)
	}

	// Verify data was properly encoded
	if resp.Error.Data == nil {
		t.Fatal("Expected Error.Data to not be nil")
	}
}

func TestNewNotification(t *testing.T) {
	// Test with nil params
	notif, err := NewNotification("test.notification", nil)
	if err != nil {
		t.Fatalf("Expected NewNotification with nil params to succeed, got error: %v", err)
	}

	if notif.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version to be %q, got %q", JSONRPCVersion, notif.JSONRPC)
	}

	if notif.Method != "test.notification" {
		t.Errorf("Expected Method to be 'test.notification', got %q", notif.Method)
	}

	if len(notif.Params) != 0 {
		t.Errorf("Expected Params to be empty, got %s", string(notif.Params))
	}

	// Test with params
	params := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	notif, err = NewNotification("test.notification", params)
	if err != nil {
		t.Fatalf("Expected NewNotification with params to succeed, got error: %v", err)
	}

	// Verify params were properly encoded
	var decodedParams map[string]interface{}
	err = json.Unmarshal(notif.Params, &decodedParams)
	if err != nil {
		t.Fatalf("Failed to decode params: %v", err)
	}

	if decodedParams["key"] != "value" {
		t.Errorf("Expected params['key'] to be 'value', got %v", decodedParams["key"])
	}

	if int(decodedParams["num"].(float64)) != 42 {
		t.Errorf("Expected params['num'] to be 42, got %v", decodedParams["num"])
	}
}

func TestIsRequest(t *testing.T) {
	// Valid request
	req := Request{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		ID:             "req-1",
		Method:         "test.method",
		Params:         json.RawMessage(`{}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	if !IsRequest(data) {
		t.Error("Expected IsRequest to return true for valid request")
	}

	// Invalid JSON
	if IsRequest([]byte(`{"jsonrpc": "2.0", "id": 1, "method"`)) {
		t.Error("Expected IsRequest to return false for invalid JSON")
	}

	// Missing ID
	if IsRequest([]byte(`{"jsonrpc": "2.0", "method": "test"}`)) {
		t.Error("Expected IsRequest to return false for request without ID")
	}

	// Missing method
	if IsRequest([]byte(`{"jsonrpc": "2.0", "id": 1}`)) {
		t.Error("Expected IsRequest to return false for request without method")
	}

	// Wrong JSON-RPC version
	if IsRequest([]byte(`{"jsonrpc": "1.0", "id": 1, "method": "test"}`)) {
		t.Error("Expected IsRequest to return false for request with wrong JSON-RPC version")
	}
}

func TestIsResponse(t *testing.T) {
	// Valid response
	resp := Response{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		ID:             "resp-1",
		Result:         json.RawMessage(`{}`),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	if !IsResponse(data) {
		t.Error("Expected IsResponse to return true for valid response")
	}

	// Valid error response
	errorResp := Response{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		ID:             "resp-1",
		Error:          &Error{Code: InvalidRequest, Message: "Invalid request"},
	}

	data, err = json.Marshal(errorResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	if !IsResponse(data) {
		t.Error("Expected IsResponse to return true for valid error response")
	}

	// Invalid JSON
	if IsResponse([]byte(`{"jsonrpc": "2.0", "id": 1, "result":`)) {
		t.Error("Expected IsResponse to return false for invalid JSON")
	}

	// Missing ID
	if IsResponse([]byte(`{"jsonrpc": "2.0", "result": {}}`)) {
		t.Error("Expected IsResponse to return false for response without ID")
	}

	// Missing result and error
	if IsResponse([]byte(`{"jsonrpc": "2.0", "id": 1}`)) {
		t.Error("Expected IsResponse to return false for response without result or error")
	}

	// Wrong JSON-RPC version
	if IsResponse([]byte(`{"jsonrpc": "1.0", "id": 1, "result": {}}`)) {
		t.Error("Expected IsResponse to return false for response with wrong JSON-RPC version")
	}
}

func TestIsNotification(t *testing.T) {
	// Valid notification
	notif := Notification{
		JSONRPCMessage: JSONRPCMessage{JSONRPC: JSONRPCVersion},
		Method:         "test.notification",
		Params:         json.RawMessage(`{}`),
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	if !IsNotification(data) {
		t.Error("Expected IsNotification to return true for valid notification")
	}

	// Invalid JSON
	if IsNotification([]byte(`{"jsonrpc": "2.0", "method": "test"`)) {
		t.Error("Expected IsNotification to return false for invalid JSON")
	}

	// Has ID (should be a request, not a notification)
	if IsNotification([]byte(`{"jsonrpc": "2.0", "id": 1, "method": "test"}`)) {
		t.Error("Expected IsNotification to return false for message with ID")
	}

	// Missing method
	if IsNotification([]byte(`{"jsonrpc": "2.0"}`)) {
		t.Error("Expected IsNotification to return false for notification without method")
	}

	// Wrong JSON-RPC version
	if IsNotification([]byte(`{"jsonrpc": "1.0", "method": "test"}`)) {
		t.Error("Expected IsNotification to return false for notification with wrong JSON-RPC version")
	}
}

func TestError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name: "Typical error",
			err:      &Error{Code: InvalidRequest, Message: "Invalid Request"},
			expected: fmt.Sprintf("jsonrpc: code %d, message: Invalid Request", InvalidRequest),
		},
		{
			name: "Error with data",
			err:      &Error{Code: InternalError, Message: "Internal Error", Data: "some data"},
			expected: fmt.Sprintf("jsonrpc: code %d, message: Internal Error", InternalError),
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}
