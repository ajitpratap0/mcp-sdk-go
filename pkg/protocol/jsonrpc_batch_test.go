package protocol

import (
	"encoding/json"
	"testing"
)

// Test creating new batch requests
func TestNewJSONRPCBatchRequest(t *testing.T) {
	t.Run("ValidBatchWithRequests", func(t *testing.T) {
		req1, _ := NewRequest("1", "test.method1", map[string]string{"param": "value1"})
		req2, _ := NewRequest("2", "test.method2", map[string]string{"param": "value2"})

		batch, err := NewJSONRPCBatchRequest(req1, req2)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if batch.Len() != 2 {
			t.Errorf("Expected batch length 2, got %d", batch.Len())
		}

		requests := batch.GetRequests()
		if len(requests) != 2 {
			t.Errorf("Expected 2 requests, got %d", len(requests))
		}
	})

	t.Run("ValidBatchWithNotifications", func(t *testing.T) {
		notif1, _ := NewNotification("test.notification1", map[string]string{"param": "value1"})
		notif2, _ := NewNotification("test.notification2", map[string]string{"param": "value2"})

		batch, err := NewJSONRPCBatchRequest(notif1, notif2)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if batch.Len() != 2 {
			t.Errorf("Expected batch length 2, got %d", batch.Len())
		}

		notifications := batch.GetNotifications()
		if len(notifications) != 2 {
			t.Errorf("Expected 2 notifications, got %d", len(notifications))
		}
	})

	t.Run("MixedBatch", func(t *testing.T) {
		req, _ := NewRequest("1", "test.method", map[string]string{"param": "value"})
		notif, _ := NewNotification("test.notification", map[string]string{"param": "value"})

		batch, err := NewJSONRPCBatchRequest(req, notif)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if batch.Len() != 2 {
			t.Errorf("Expected batch length 2, got %d", batch.Len())
		}

		requests := batch.GetRequests()
		notifications := batch.GetNotifications()

		if len(requests) != 1 {
			t.Errorf("Expected 1 request, got %d", len(requests))
		}
		if len(notifications) != 1 {
			t.Errorf("Expected 1 notification, got %d", len(notifications))
		}
	})

	t.Run("EmptyBatch", func(t *testing.T) {
		_, err := NewJSONRPCBatchRequest()
		if err == nil {
			t.Error("Expected error for empty batch, got nil")
		}
	})

	t.Run("NilRequest", func(t *testing.T) {
		var nilReq *Request = nil
		_, err := NewJSONRPCBatchRequest(nilReq)
		if err == nil {
			t.Error("Expected error for nil request, got nil")
		}
	})

	t.Run("InvalidType", func(t *testing.T) {
		_, err := NewJSONRPCBatchRequest("invalid")
		if err == nil {
			t.Error("Expected error for invalid type, got nil")
		}
	})
}

// Test creating new batch responses
func TestNewJSONRPCBatchResponse(t *testing.T) {
	t.Run("ValidBatchResponse", func(t *testing.T) {
		resp1, _ := NewResponse("1", map[string]string{"result": "value1"})
		resp2, _ := NewResponse("2", map[string]string{"result": "value2"})

		batch := NewJSONRPCBatchResponse(resp1, resp2)
		if batch.Len() != 2 {
			t.Errorf("Expected batch length 2, got %d", batch.Len())
		}

		if batch.IsEmpty() {
			t.Error("Expected non-empty batch")
		}
	})

	t.Run("EmptyBatchResponse", func(t *testing.T) {
		batch := NewJSONRPCBatchResponse()
		if !batch.IsEmpty() {
			t.Error("Expected empty batch")
		}
	})

	t.Run("NilResponsesFiltered", func(t *testing.T) {
		resp1, _ := NewResponse("1", map[string]string{"result": "value1"})
		var nilResp *Response = nil

		batch := NewJSONRPCBatchResponse(resp1, nilResp)
		if batch.Len() != 1 {
			t.Errorf("Expected batch length 1 (nil filtered), got %d", batch.Len())
		}
	})
}

// Test batch JSON serialization/deserialization
func TestJSONRPCBatchSerialization(t *testing.T) {
	t.Run("BatchRequestSerialization", func(t *testing.T) {
		req1, _ := NewRequest("1", "test.method1", map[string]string{"param": "value1"})
		req2, _ := NewRequest("2", "test.method2", map[string]string{"param": "value2"})

		batch, _ := NewJSONRPCBatchRequest(req1, req2)
		data, err := batch.ToJSON()
		if err != nil {
			t.Fatalf("Failed to serialize batch: %v", err)
		}

		// Verify it's a valid JSON array
		var jsonArray []interface{}
		if err := json.Unmarshal(data, &jsonArray); err != nil {
			t.Fatalf("Serialized data is not valid JSON array: %v", err)
		}

		if len(jsonArray) != 2 {
			t.Errorf("Expected 2 items in JSON array, got %d", len(jsonArray))
		}
	})

	t.Run("BatchResponseSerialization", func(t *testing.T) {
		resp1, _ := NewResponse("1", map[string]string{"result": "value1"})
		resp2, _ := NewResponse("2", map[string]string{"result": "value2"})

		batch := NewJSONRPCBatchResponse(resp1, resp2)
		data, err := batch.ToJSON()
		if err != nil {
			t.Fatalf("Failed to serialize batch: %v", err)
		}

		// Verify it's a valid JSON array
		var jsonArray []interface{}
		if err := json.Unmarshal(data, &jsonArray); err != nil {
			t.Fatalf("Serialized data is not valid JSON array: %v", err)
		}

		if len(jsonArray) != 2 {
			t.Errorf("Expected 2 items in JSON array, got %d", len(jsonArray))
		}
	})

	t.Run("NilBatchSerialization", func(t *testing.T) {
		var nilBatch *JSONRPCBatchRequest = nil
		_, err := nilBatch.ToJSON()
		if err == nil {
			t.Error("Expected error for nil batch serialization")
		}
	})
}

// Test batch parsing
func TestParseJSONRPCBatch(t *testing.T) {
	t.Run("ParseValidBatchRequest", func(t *testing.T) {
		jsonData := `[
			{"jsonrpc": "2.0", "id": "1", "method": "test.method1", "params": {"param": "value1"}},
			{"jsonrpc": "2.0", "id": "2", "method": "test.method2", "params": {"param": "value2"}},
			{"jsonrpc": "2.0", "method": "test.notification", "params": {"param": "value3"}}
		]`

		batch, err := ParseJSONRPCBatchRequest([]byte(jsonData))
		if err != nil {
			t.Fatalf("Failed to parse batch request: %v", err)
		}

		if batch.Len() != 3 {
			t.Errorf("Expected batch length 3, got %d", batch.Len())
		}

		requests := batch.GetRequests()
		notifications := batch.GetNotifications()

		if len(requests) != 2 {
			t.Errorf("Expected 2 requests, got %d", len(requests))
		}
		if len(notifications) != 1 {
			t.Errorf("Expected 1 notification, got %d", len(notifications))
		}
	})

	t.Run("ParseValidBatchResponse", func(t *testing.T) {
		jsonData := `[
			{"jsonrpc": "2.0", "id": "1", "result": {"value": "result1"}},
			{"jsonrpc": "2.0", "id": "2", "error": {"code": -32602, "message": "Invalid params"}}
		]`

		batch, err := ParseJSONRPCBatchResponse([]byte(jsonData))
		if err != nil {
			t.Fatalf("Failed to parse batch response: %v", err)
		}

		if batch.Len() != 2 {
			t.Errorf("Expected batch length 2, got %d", batch.Len())
		}
	})

	t.Run("ParseEmptyBatch", func(t *testing.T) {
		jsonData := `[]`
		_, err := ParseJSONRPCBatchRequest([]byte(jsonData))
		if err == nil {
			t.Error("Expected error for empty batch array")
		}
	})

	t.Run("ParseInvalidJSON", func(t *testing.T) {
		jsonData := `{"not": "an array"}`
		_, err := ParseJSONRPCBatchRequest([]byte(jsonData))
		if err == nil {
			t.Error("Expected error for non-array JSON")
		}
	})

	t.Run("ParseInvalidBatchItem", func(t *testing.T) {
		jsonData := `[
			{"jsonrpc": "2.0", "id": "1", "method": "test.method"},
			{"invalid": "object"}
		]`
		_, err := ParseJSONRPCBatchRequest([]byte(jsonData))
		if err == nil {
			t.Error("Expected error for invalid batch item")
		}
	})
}

// Test batch detection
func TestIsBatch(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "ValidArray",
			data:     `[{"jsonrpc": "2.0", "id": "1", "method": "test"}]`,
			expected: true,
		},
		{
			name:     "EmptyArray",
			data:     `[]`,
			expected: true,
		},
		{
			name:     "Object",
			data:     `{"jsonrpc": "2.0", "id": "1", "method": "test"}`,
			expected: false,
		},
		{
			name:     "InvalidJSON",
			data:     `invalid json`,
			expected: false,
		},
		{
			name:     "Null",
			data:     `null`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBatch([]byte(tt.data))
			if result != tt.expected {
				t.Errorf("IsBatch() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Test batch response manipulation
func TestBatchResponseManipulation(t *testing.T) {
	t.Run("AddResponseToBatch", func(t *testing.T) {
		batch := NewJSONRPCBatchResponse()
		if !batch.IsEmpty() {
			t.Error("Expected empty batch initially")
		}

		resp, _ := NewResponse("1", map[string]string{"result": "value"})
		batch.Add(resp)

		if batch.IsEmpty() {
			t.Error("Expected non-empty batch after adding response")
		}
		if batch.Len() != 1 {
			t.Errorf("Expected batch length 1, got %d", batch.Len())
		}
	})

	t.Run("AddNilResponse", func(t *testing.T) {
		batch := NewJSONRPCBatchResponse()
		batch.Add(nil)

		if !batch.IsEmpty() {
			t.Error("Expected batch to remain empty when adding nil")
		}
	})

	t.Run("AddToNilBatch", func(t *testing.T) {
		var batch *JSONRPCBatchResponse = nil
		resp, _ := NewResponse("1", map[string]string{"result": "value"})

		// This should not panic
		batch.Add(resp)
	})
}

// Test error scenarios
func TestBatchErrorScenarios(t *testing.T) {
	t.Run("NilBatchOperations", func(t *testing.T) {
		var batch *JSONRPCBatchRequest = nil

		if batch.Len() != 0 {
			t.Error("Expected 0 length for nil batch")
		}

		requests := batch.GetRequests()
		if requests != nil {
			t.Error("Expected nil requests for nil batch")
		}

		notifications := batch.GetNotifications()
		if notifications != nil {
			t.Error("Expected nil notifications for nil batch")
		}
	})

	t.Run("EmptyBatchOperations", func(t *testing.T) {
		batch := &JSONRPCBatchRequest{}

		if batch.Len() != 0 {
			t.Error("Expected 0 length for empty batch")
		}

		requests := batch.GetRequests()
		if len(requests) != 0 {
			t.Error("Expected no requests for empty batch")
		}

		notifications := batch.GetNotifications()
		if len(notifications) != 0 {
			t.Error("Expected no notifications for empty batch")
		}
	})
}

// Test comprehensive batch roundtrip
func TestBatchRoundtrip(t *testing.T) {
	t.Run("CompleteRoundtrip", func(t *testing.T) {
		// Create original batch
		req1, _ := NewRequest("1", "test.method1", map[string]string{"param": "value1"})
		req2, _ := NewRequest("2", "test.method2", map[string]string{"param": "value2"})
		notif, _ := NewNotification("test.notification", map[string]string{"param": "value3"})

		originalBatch, _ := NewJSONRPCBatchRequest(req1, req2, notif)

		// Serialize to JSON
		jsonData, err := originalBatch.ToJSON()
		if err != nil {
			t.Fatalf("Failed to serialize batch: %v", err)
		}

		// Parse back from JSON
		parsedBatch, err := ParseJSONRPCBatchRequest(jsonData)
		if err != nil {
			t.Fatalf("Failed to parse batch: %v", err)
		}

		// Verify the parsed batch matches original
		if parsedBatch.Len() != originalBatch.Len() {
			t.Errorf("Parsed batch length %d != original length %d", parsedBatch.Len(), originalBatch.Len())
		}

		originalRequests := originalBatch.GetRequests()
		parsedRequests := parsedBatch.GetRequests()
		if len(parsedRequests) != len(originalRequests) {
			t.Errorf("Parsed requests length %d != original requests length %d", len(parsedRequests), len(originalRequests))
		}

		originalNotifications := originalBatch.GetNotifications()
		parsedNotifications := parsedBatch.GetNotifications()
		if len(parsedNotifications) != len(originalNotifications) {
			t.Errorf("Parsed notifications length %d != original notifications length %d", len(parsedNotifications), len(originalNotifications))
		}

		// Verify specific request content
		if len(parsedRequests) >= 2 {
			if parsedRequests[0].Method != "test.method1" {
				t.Errorf("First request method mismatch: got %s, expected test.method1", parsedRequests[0].Method)
			}
			if parsedRequests[1].Method != "test.method2" {
				t.Errorf("Second request method mismatch: got %s, expected test.method2", parsedRequests[1].Method)
			}
		}

		// Verify notification content
		if len(parsedNotifications) >= 1 {
			if parsedNotifications[0].Method != "test.notification" {
				t.Errorf("Notification method mismatch: got %s, expected test.notification", parsedNotifications[0].Method)
			}
		}
	})
}

// Benchmark batch operations
func BenchmarkBatchCreation(b *testing.B) {
	req1, _ := NewRequest("1", "test.method1", map[string]string{"param": "value1"})
	req2, _ := NewRequest("2", "test.method2", map[string]string{"param": "value2"})
	notif, _ := NewNotification("test.notification", map[string]string{"param": "value3"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewJSONRPCBatchRequest(req1, req2, notif)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchSerialization(b *testing.B) {
	req1, _ := NewRequest("1", "test.method1", map[string]string{"param": "value1"})
	req2, _ := NewRequest("2", "test.method2", map[string]string{"param": "value2"})
	batch, _ := NewJSONRPCBatchRequest(req1, req2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := batch.ToJSON()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchParsing(b *testing.B) {
	jsonData := []byte(`[
		{"jsonrpc": "2.0", "id": "1", "method": "test.method1", "params": {"param": "value1"}},
		{"jsonrpc": "2.0", "id": "2", "method": "test.method2", "params": {"param": "value2"}},
		{"jsonrpc": "2.0", "method": "test.notification", "params": {"param": "value3"}}
	]`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseJSONRPCBatchRequest(jsonData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
