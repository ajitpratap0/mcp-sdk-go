package transport

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// Test HandleBatchRequest with mixed requests and notifications
func TestBaseTransport_HandleBatchRequest(t *testing.T) {
	t.Run("BatchWithRequestsAndNotifications", func(t *testing.T) {
		transport := NewBaseTransport()

		// Register request handler
		transport.RegisterRequestHandler("test.method", func(ctx context.Context, params interface{}) (interface{}, error) {
			return map[string]string{"result": "success"}, nil
		})

		// Register notification handler
		notificationReceived := false
		transport.RegisterNotificationHandler("test.notification", func(ctx context.Context, params interface{}) error {
			notificationReceived = true
			return nil
		})

		// Create batch with requests and notifications
		req1, _ := protocol.NewRequest("1", "test.method", map[string]string{"param": "value1"})
		req2, _ := protocol.NewRequest("2", "test.method", map[string]string{"param": "value2"})
		notif, _ := protocol.NewNotification("test.notification", map[string]string{"param": "value3"})

		batch, _ := protocol.NewJSONRPCBatchRequest(req1, req2, notif)

		// Process batch
		ctx := context.Background()
		response, err := transport.HandleBatchRequest(ctx, batch)

		if err != nil {
			t.Fatalf("HandleBatchRequest failed: %v", err)
		}

		// Should have 2 responses (for the 2 requests, none for notification)
		if response.Len() != 2 {
			t.Errorf("Expected 2 responses, got %d", response.Len())
		}

		// Notification should have been processed
		if !notificationReceived {
			t.Error("Expected notification to be processed")
		}

		// Verify responses are for correct request IDs
		for _, resp := range *response {
			if resp.ID != "1" && resp.ID != "2" {
				t.Errorf("Unexpected response ID: %v", resp.ID)
			}
			if resp.Error != nil {
				t.Errorf("Unexpected error in response: %v", resp.Error)
			}
		}
	})

	t.Run("BatchWithOnlyNotifications", func(t *testing.T) {
		transport := NewBaseTransport()

		notificationCount := 0
		transport.RegisterNotificationHandler("test.notification", func(ctx context.Context, params interface{}) error {
			notificationCount++
			return nil
		})

		// Create batch with only notifications
		notif1, _ := protocol.NewNotification("test.notification", map[string]string{"param": "value1"})
		notif2, _ := protocol.NewNotification("test.notification", map[string]string{"param": "value2"})

		batch, _ := protocol.NewJSONRPCBatchRequest(notif1, notif2)

		// Process batch
		ctx := context.Background()
		response, err := transport.HandleBatchRequest(ctx, batch)

		if err != nil {
			t.Fatalf("HandleBatchRequest failed: %v", err)
		}

		// Should have empty response (no requests, only notifications)
		if !response.IsEmpty() {
			t.Errorf("Expected empty response for notifications-only batch, got %d responses", response.Len())
		}

		// Both notifications should have been processed
		if notificationCount != 2 {
			t.Errorf("Expected 2 notifications processed, got %d", notificationCount)
		}
	})

	t.Run("BatchWithRequestErrors", func(t *testing.T) {
		transport := NewBaseTransport()

		// Register request handler that returns an error
		transport.RegisterRequestHandler("test.error", func(ctx context.Context, params interface{}) (interface{}, error) {
			return nil, fmt.Errorf("test error")
		})

		// Register successful request handler
		transport.RegisterRequestHandler("test.success", func(ctx context.Context, params interface{}) (interface{}, error) {
			return map[string]string{"result": "success"}, nil
		})

		// Create batch with error and success requests
		errorReq, _ := protocol.NewRequest("1", "test.error", nil)
		successReq, _ := protocol.NewRequest("2", "test.success", nil)

		batch, _ := protocol.NewJSONRPCBatchRequest(errorReq, successReq)

		// Process batch
		ctx := context.Background()
		response, err := transport.HandleBatchRequest(ctx, batch)

		if err != nil {
			t.Fatalf("HandleBatchRequest failed: %v", err)
		}

		// Should have 2 responses
		if response.Len() != 2 {
			t.Errorf("Expected 2 responses, got %d", response.Len())
		}

		// Check that we have one error and one success response
		var errorResp, successResp *protocol.Response
		for _, resp := range *response {
			if resp.ID == "1" {
				errorResp = resp
			} else if resp.ID == "2" {
				successResp = resp
			}
		}

		if errorResp == nil {
			t.Error("Expected error response for request ID 1")
		} else if errorResp.Error == nil {
			t.Error("Expected error in response for request ID 1")
		}

		if successResp == nil {
			t.Error("Expected success response for request ID 2")
		} else if successResp.Error != nil {
			t.Errorf("Expected no error in response for request ID 2, got: %v", successResp.Error)
		}
	})

	t.Run("BatchWithUnregisteredMethods", func(t *testing.T) {
		transport := NewBaseTransport()

		// Create request for unregistered method
		req, _ := protocol.NewRequest("1", "unregistered.method", nil)
		batch, _ := protocol.NewJSONRPCBatchRequest(req)

		// Process batch
		ctx := context.Background()
		response, err := transport.HandleBatchRequest(ctx, batch)

		if err != nil {
			t.Fatalf("HandleBatchRequest failed: %v", err)
		}

		// Should have 1 error response
		if response.Len() != 1 {
			t.Errorf("Expected 1 response, got %d", response.Len())
		}

		resp := (*response)[0]
		if resp.Error == nil {
			t.Error("Expected error response for unregistered method")
		}
	})

	t.Run("EmptyBatch", func(t *testing.T) {
		transport := NewBaseTransport()

		// Test with nil batch
		ctx := context.Background()
		_, err := transport.HandleBatchRequest(ctx, nil)
		if err == nil {
			t.Error("Expected error for nil batch")
		}

		// Test with empty batch
		emptyBatch := &protocol.JSONRPCBatchRequest{}
		_, err = transport.HandleBatchRequest(ctx, emptyBatch)
		if err == nil {
			t.Error("Expected error for empty batch")
		}
	})

	t.Run("ConcurrentBatchProcessing", func(t *testing.T) {
		transport := NewBaseTransport()

		// Register request handler that simulates work
		transport.RegisterRequestHandler("test.method", func(ctx context.Context, params interface{}) (interface{}, error) {
			time.Sleep(10 * time.Millisecond) // Simulate processing time
			return map[string]string{"result": "success"}, nil
		})

		// Create multiple batches and process them concurrently
		numBatches := 10
		results := make(chan error, numBatches)
		var wg sync.WaitGroup

		for i := 0; i < numBatches; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				req1, _ := protocol.NewRequest(fmt.Sprintf("%d-1", id), "test.method", nil)
				req2, _ := protocol.NewRequest(fmt.Sprintf("%d-2", id), "test.method", nil)
				batch, _ := protocol.NewJSONRPCBatchRequest(req1, req2)

				ctx := context.Background()
				response, err := transport.HandleBatchRequest(ctx, batch)
				if err != nil {
					results <- err
					return
				}

				if response.Len() != 2 {
					results <- fmt.Errorf("expected 2 responses, got %d", response.Len())
					return
				}

				results <- nil
			}(i)
		}

		wg.Wait()
		close(results)

		// Check results
		for err := range results {
			if err != nil {
				t.Errorf("Concurrent batch processing error: %v", err)
			}
		}
	})
}

// Test SendBatchRequest default implementation
func TestBaseTransport_SendBatchRequest(t *testing.T) {
	transport := NewBaseTransport()

	req, _ := protocol.NewRequest("1", "test.method", nil)
	batch, _ := protocol.NewJSONRPCBatchRequest(req)

	ctx := context.Background()
	_, err := transport.SendBatchRequest(ctx, batch)

	// Base implementation should return error
	if err == nil {
		t.Error("Expected error from base SendBatchRequest implementation")
	}
}

// Test batch processing with context cancellation
func TestBatchProcessingWithContextCancellation(t *testing.T) {
	transport := NewBaseTransport()

	// Register request handler that checks context
	transport.RegisterRequestHandler("test.method", func(ctx context.Context, params interface{}) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return map[string]string{"result": "success"}, nil
		}
	})

	// Create batch
	req, _ := protocol.NewRequest("1", "test.method", nil)
	batch, _ := protocol.NewJSONRPCBatchRequest(req)

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Process batch with cancelled context
	response, err := transport.HandleBatchRequest(ctx, batch)

	if err != nil {
		t.Fatalf("HandleBatchRequest failed: %v", err)
	}

	// Should have 1 response with context error
	if response.Len() != 1 {
		t.Errorf("Expected 1 response, got %d", response.Len())
	}

	resp := (*response)[0]
	if resp.Error == nil {
		t.Error("Expected error response due to context cancellation")
	}
}

// Test batch response ordering
func TestBatchResponseOrdering(t *testing.T) {
	transport := NewBaseTransport()

	// Register request handler that returns the request ID
	transport.RegisterRequestHandler("test.method", func(ctx context.Context, params interface{}) (interface{}, error) {
		// Extract request data if available
		return map[string]string{"processed": "ok"}, nil
	})

	// Create batch with specific order
	requestIDs := []string{"first", "second", "third", "fourth"}
	requests := make([]interface{}, len(requestIDs))

	for i, id := range requestIDs {
		req, _ := protocol.NewRequest(id, "test.method", map[string]string{"order": fmt.Sprintf("%d", i)})
		requests[i] = req
	}

	batch, _ := protocol.NewJSONRPCBatchRequest(requests...)

	// Process batch
	ctx := context.Background()
	response, err := transport.HandleBatchRequest(ctx, batch)

	if err != nil {
		t.Fatalf("HandleBatchRequest failed: %v", err)
	}

	// Verify response count
	if response.Len() != len(requestIDs) {
		t.Errorf("Expected %d responses, got %d", len(requestIDs), response.Len())
	}

	// Verify response order matches request order
	responseMap := make(map[string]int)
	for i, resp := range *response {
		if idStr, ok := resp.ID.(string); ok {
			responseMap[idStr] = i
		}
	}

	for i, expectedID := range requestIDs {
		if responseIndex, exists := responseMap[expectedID]; !exists {
			t.Errorf("Response for request ID %s not found", expectedID)
		} else if responseIndex != i {
			t.Errorf("Response for request ID %s at index %d, expected index %d", expectedID, responseIndex, i)
		}
	}
}

// Test large batch processing
func TestLargeBatchProcessing(t *testing.T) {
	transport := NewBaseTransport()

	// Register request handler
	transport.RegisterRequestHandler("test.method", func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"result": "success"}, nil
	})

	// Create large batch
	batchSize := 1000
	requests := make([]interface{}, batchSize)

	for i := 0; i < batchSize; i++ {
		req, _ := protocol.NewRequest(fmt.Sprintf("req-%d", i), "test.method", map[string]int{"index": i})
		requests[i] = req
	}

	batch, err := protocol.NewJSONRPCBatchRequest(requests...)
	if err != nil {
		t.Fatalf("Failed to create large batch: %v", err)
	}

	// Process large batch
	ctx := context.Background()
	start := time.Now()
	response, err := transport.HandleBatchRequest(ctx, batch)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("HandleBatchRequest failed: %v", err)
	}

	// Verify all requests were processed
	if response.Len() != batchSize {
		t.Errorf("Expected %d responses, got %d", batchSize, response.Len())
	}

	t.Logf("Processed %d requests in %v (%.2f req/ms)", batchSize, duration, float64(batchSize)/float64(duration.Nanoseconds()/1e6))

	// Verify no errors in responses
	for i, resp := range *response {
		if resp.Error != nil {
			t.Errorf("Response %d has error: %v", i, resp.Error)
		}
	}
}

// Test batch with mixed parameter types
func TestBatchWithMixedParameterTypes(t *testing.T) {
	transport := NewBaseTransport()

	// Register handlers for different parameter types
	transport.RegisterRequestHandler("test.string", func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"type": "string", "received": "ok"}, nil
	})

	transport.RegisterRequestHandler("test.object", func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"type": "object", "received": "ok"}, nil
	})

	transport.RegisterRequestHandler("test.array", func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"type": "array", "received": "ok"}, nil
	})

	// Create requests with different parameter types
	stringReq, _ := protocol.NewRequest("1", "test.string", "simple string")
	objectReq, _ := protocol.NewRequest("2", "test.object", map[string]interface{}{"key": "value", "number": 42})
	arrayReq, _ := protocol.NewRequest("3", "test.array", []interface{}{"item1", "item2", 123})

	batch, _ := protocol.NewJSONRPCBatchRequest(stringReq, objectReq, arrayReq)

	// Process batch
	ctx := context.Background()
	response, err := transport.HandleBatchRequest(ctx, batch)

	if err != nil {
		t.Fatalf("HandleBatchRequest failed: %v", err)
	}

	// Verify all responses
	if response.Len() != 3 {
		t.Errorf("Expected 3 responses, got %d", response.Len())
	}

	// Verify no errors
	for _, resp := range *response {
		if resp.Error != nil {
			t.Errorf("Unexpected error in response: %v", resp.Error)
		}
	}
}

// Benchmark batch processing performance
func BenchmarkBatchProcessing(b *testing.B) {
	transport := NewBaseTransport()

	// Register simple request handler
	transport.RegisterRequestHandler("test.method", func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"result": "success"}, nil
	})

	// Create reusable batch
	batchSize := 100
	requests := make([]interface{}, batchSize)
	for i := 0; i < batchSize; i++ {
		req, _ := protocol.NewRequest(fmt.Sprintf("req-%d", i), "test.method", nil)
		requests[i] = req
	}
	batch, _ := protocol.NewJSONRPCBatchRequest(requests...)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := transport.HandleBatchRequest(ctx, batch)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchCreationAndProcessing(b *testing.B) {
	transport := NewBaseTransport()

	transport.RegisterRequestHandler("test.method", func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"result": "success"}, nil
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create new batch each time
		req1, _ := protocol.NewRequest("1", "test.method", nil)
		req2, _ := protocol.NewRequest("2", "test.method", nil)
		batch, _ := protocol.NewJSONRPCBatchRequest(req1, req2)

		_, err := transport.HandleBatchRequest(ctx, batch)
		if err != nil {
			b.Fatal(err)
		}
	}
}
