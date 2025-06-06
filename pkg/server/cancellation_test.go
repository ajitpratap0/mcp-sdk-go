package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// MockLongRunningToolsProvider simulates a long-running tool that can be cancelled
type MockLongRunningToolsProvider struct {
	BaseToolsProvider
	callDuration time.Duration
	mu           sync.Mutex
	cancelled    bool
}

func (m *MockLongRunningToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, toolContext json.RawMessage) (*protocol.CallToolResult, error) {
	m.mu.Lock()
	m.cancelled = false
	m.mu.Unlock()

	// Simulate a long-running operation
	select {
	case <-time.After(m.callDuration):
		result, _ := json.Marshal("Operation completed successfully")
		return &protocol.CallToolResult{
			Result: result,
		}, nil
	case <-ctx.Done():
		m.mu.Lock()
		m.cancelled = true
		m.mu.Unlock()
		return nil, context.Canceled
	}
}

func (m *MockLongRunningToolsProvider) WasCancelled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancelled
}

// TestRequestCancellation tests the basic request cancellation functionality
func TestRequestCancellation(t *testing.T) {
	server := createTestServer()

	// Test tracking and cancelling a request
	requestID := "test-request-123"
	ctx, cancel := context.WithCancel(context.Background())

	// Track the request
	server.trackRequest(requestID, cancel)

	// Verify the request is tracked
	server.activeRequestsLock.RLock()
	_, exists := server.activeRequests[requestID]
	server.activeRequestsLock.RUnlock()
	if !exists {
		t.Error("Request should be tracked")
	}

	// Cancel the request
	cancelled := server.cancelRequest(requestID)
	if !cancelled {
		t.Error("Request should have been cancelled")
	}

	// Verify the request is removed
	server.activeRequestsLock.RLock()
	_, exists = server.activeRequests[requestID]
	server.activeRequestsLock.RUnlock()
	if exists {
		t.Error("Request should be removed after cancellation")
	}

	// Verify the context is cancelled
	select {
	case <-ctx.Done():
		// Good, context was cancelled
	default:
		t.Error("Context should be cancelled")
	}
}

// TestHandleCancel tests the handleCancel method
func TestHandleCancel(t *testing.T) {
	server := createTestServer()

	// Track a request
	requestID := "request-456"
	ctx, cancel := context.WithCancel(context.Background())
	server.trackRequest(requestID, cancel)

	// Test cancelling an existing request
	params := &protocol.CancelParams{
		ID: requestID,
	}
	result, err := server.handleCancel(context.Background(), params)
	if err != nil {
		t.Fatalf("handleCancel failed: %v", err)
	}

	// Verify the context was cancelled
	select {
	case <-ctx.Done():
		// Good
	default:
		t.Error("Context should be cancelled")
	}

	cancelResult, ok := result.(*protocol.CancelResult)
	if !ok {
		t.Fatalf("Expected CancelResult, got %T", result)
	}

	if !cancelResult.Cancelled {
		t.Error("Request should have been cancelled")
	}

	// Test cancelling a non-existent request
	params2 := &protocol.CancelParams{
		ID: "non-existent",
	}
	result2, err := server.handleCancel(context.Background(), params2)
	if err != nil {
		t.Fatalf("handleCancel failed: %v", err)
	}

	cancelResult2, ok := result2.(*protocol.CancelResult)
	if !ok {
		t.Fatalf("Expected CancelResult, got %T", result2)
	}

	if cancelResult2.Cancelled {
		t.Error("Non-existent request should not be cancelled")
	}
}

// TestConcurrentCancellation tests concurrent cancellation requests
func TestConcurrentCancellation(t *testing.T) {
	server := createTestServer()

	// Create multiple requests
	numRequests := 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer wg.Done()

			requestID := fmt.Sprintf("request-%d", id)
			_, cancel := context.WithCancel(context.Background())

			// Track the request
			server.trackRequest(requestID, cancel)

			// Small delay to ensure all requests are tracked
			time.Sleep(10 * time.Millisecond)

			// Cancel the request
			cancelled := server.cancelRequest(requestID)
			if !cancelled {
				t.Errorf("Request %s should have been cancelled", requestID)
			}
		}(i)
	}

	wg.Wait()

	// Verify all requests are removed
	server.activeRequestsLock.RLock()
	remaining := len(server.activeRequests)
	server.activeRequestsLock.RUnlock()

	if remaining != 0 {
		t.Errorf("Expected 0 active requests, got %d", remaining)
	}
}

// TestServerStopCancelsAllRequests tests that stopping the server cancels all active requests
func TestServerStopCancelsAllRequests(t *testing.T) {
	server := createTestServer()

	// Track multiple requests
	numRequests := 5
	contexts := make([]context.Context, numRequests)

	for i := 0; i < numRequests; i++ {
		requestID := fmt.Sprintf("request-%d", i)
		ctx, cancel := context.WithCancel(context.Background())
		contexts[i] = ctx
		server.trackRequest(requestID, cancel)
	}

	// Verify requests are tracked
	server.activeRequestsLock.RLock()
	activeCount := len(server.activeRequests)
	server.activeRequestsLock.RUnlock()

	if activeCount != numRequests {
		t.Errorf("Expected %d active requests, got %d", numRequests, activeCount)
	}

	// Stop the server
	_ = server.Stop()

	// Verify all contexts are cancelled
	for i, ctx := range contexts {
		select {
		case <-ctx.Done():
			// Good, context was cancelled
		default:
			t.Errorf("Context %d should be cancelled", i)
		}
	}

	// Verify no active requests remain
	server.activeRequestsLock.RLock()
	remaining := len(server.activeRequests)
	server.activeRequestsLock.RUnlock()

	if remaining != 0 {
		t.Errorf("Expected 0 active requests after stop, got %d", remaining)
	}
}

// TestLongRunningToolCancellation tests cancelling a long-running tool call
func TestLongRunningToolCancellation(t *testing.T) {
	mockProvider := &MockLongRunningToolsProvider{
		BaseToolsProvider: BaseToolsProvider{
			tools: map[string]protocol.Tool{
				"long-operation": {
					Name:        "long-operation",
					Description: "A long-running operation",
					InputSchema: json.RawMessage(`{"type":"object"}`),
				},
			},
		},
		callDuration: 2 * time.Second,
	}

	server := New(
		createMockTransport(),
		WithToolsProvider(mockProvider),
	)

	// Start a long-running tool call in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	var result interface{}
	var err error
	done := make(chan struct{})

	go func() {
		inputData, _ := json.Marshal(map[string]interface{}{"test": "data"})
		params := &protocol.CallToolParams{
			Name:  "long-operation",
			Input: inputData,
		}
		result, err = server.handleCallTool(ctx, params)
		close(done)
	}()

	// Give the tool call time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancel()

	// Wait for the tool call to complete
	select {
	case <-done:
		// Good, operation completed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Tool call did not complete after cancellation")
	}

	// Verify the tool was cancelled
	if !mockProvider.WasCancelled() {
		t.Error("Tool should have been cancelled")
	}

	// Check the result
	if err != nil {
		t.Fatalf("handleCallTool returned error: %v", err)
	}

	toolResult, ok := result.(*protocol.CallToolResult)
	if !ok {
		t.Fatalf("Expected CallToolResult, got %T", result)
	}

	if toolResult.Error != "Tool call was cancelled" {
		t.Errorf("Expected cancellation error, got: %s", toolResult.Error)
	}
}

// TestRequestCompletion tests normal request completion
func TestRequestCompletion(t *testing.T) {
	server := createTestServer()

	requestID := "test-request"
	ctx, cancel := context.WithCancel(context.Background())

	// Track the request
	server.trackRequest(requestID, cancel)

	// Complete the request normally
	server.completeRequest(requestID)

	// Verify the request is removed
	server.activeRequestsLock.RLock()
	_, exists := server.activeRequests[requestID]
	server.activeRequestsLock.RUnlock()

	if exists {
		t.Error("Request should be removed after completion")
	}

	// Verify the context is NOT cancelled (normal completion)
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled on normal completion")
	default:
		// Good, context is still active
	}
}

// Helper function to create a test server
func createTestServer() *Server {
	return New(createMockTransport())
}

// Helper function to create a mock transport
func createMockTransport() *MockTransport {
	return &MockTransport{
		BaseTransport: transport.NewBaseTransport(),
	}
}

// MockTransport implements a basic mock transport for testing
type MockTransport struct {
	*transport.BaseTransport
}

func (m *MockTransport) Initialize(ctx context.Context) error {
	return nil
}

func (m *MockTransport) Start(ctx context.Context) error {
	return nil
}

func (m *MockTransport) Stop(ctx context.Context) error {
	return nil
}

func (m *MockTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *MockTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	return errors.New("not implemented")
}

func (m *MockTransport) Send(data []byte) error {
	return errors.New("not implemented")
}

func (m *MockTransport) SetErrorHandler(handler transport.ErrorHandler) {
	// No-op for testing
}
