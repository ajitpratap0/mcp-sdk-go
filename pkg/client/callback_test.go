package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// Test sampling callback functionality
func TestSamplingCallbackIntegration(t *testing.T) {
	// Create a mock transport
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Test callback registration
	var receivedEvent protocol.SamplingEvent
	called := false
	callback := func(event protocol.SamplingEvent) {
		receivedEvent = event
		called = true
	}

	client.SetSamplingCallback(callback)

	// Simulate a sample request from server
	sampleParams := protocol.SampleParams{
		Messages: []protocol.Message{
			{Role: "user", Content: "Test message"},
		},
		RequestID: "test-123",
		MaxTokens: 100,
	}

	// Call the handle sample method directly
	result, err := client.handleSample(context.Background(), sampleParams)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("handleSample failed: %v", err)
	}

	// Verify result was returned
	if result == nil {
		t.Fatal("Expected non-nil result from handleSample")
	}

	sampleResult, ok := result.(*protocol.SampleResult)
	if !ok {
		t.Fatalf("Expected *protocol.SampleResult, got %T", result)
	}

	if sampleResult.Content == "" {
		t.Error("Expected non-empty content in sample result")
	}

	// Verify callback was called
	if !called {
		t.Error("Expected sampling callback to be called")
	}

	// Verify callback received correct event
	if receivedEvent.Type != "sample_request" {
		t.Errorf("Expected event type 'sample_request', got %s", receivedEvent.Type)
	}

	if receivedEvent.TokenID != "test-123" {
		t.Errorf("Expected token ID 'test-123', got %s", receivedEvent.TokenID)
	}

	// Verify the event data contains the sample params
	eventData, ok := receivedEvent.Data.(protocol.SampleParams)
	if !ok {
		t.Fatalf("Expected event data to be protocol.SampleParams, got %T", receivedEvent.Data)
	}

	if eventData.RequestID != "test-123" {
		t.Errorf("Expected request ID 'test-123', got %s", eventData.RequestID)
	}

	if len(eventData.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(eventData.Messages))
	}
}

// Test sampling callback without registration
func TestSamplingCallbackNotRegistered(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Don't register a callback - should return error
	sampleParams := protocol.SampleParams{
		Messages:  []protocol.Message{{Role: "user", Content: "Test"}},
		RequestID: "test-456",
	}

	result, err := client.handleSample(context.Background(), sampleParams)

	// Should return an error when no callback is registered
	if err == nil {
		t.Error("Expected error when no sampling callback registered")
	}

	if result != nil {
		t.Error("Expected nil result when no sampling callback registered")
	}
}

// Test resource changed callback functionality
func TestResourceChangedCallbackIntegration(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Test callback registration
	var receivedURI string
	var receivedData interface{}
	called := false
	callback := func(uri string, data interface{}) {
		receivedURI = uri
		receivedData = data
		called = true
	}

	client.SetResourceChangedCallback(callback)

	// Simulate a resources changed notification
	resourcesChangedParams := protocol.ResourcesChangedParams{
		URI: "file:///test/resource",
		Added: []protocol.Resource{
			{URI: "file:///test/new.txt", Name: "new.txt"},
		},
		Modified: []protocol.Resource{
			{URI: "file:///test/modified.txt", Name: "modified.txt"},
		},
		Removed: []string{"file:///test/deleted.txt"},
	}

	// Call the handle resources changed method directly
	err := client.handleResourcesChanged(context.Background(), resourcesChangedParams)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("handleResourcesChanged failed: %v", err)
	}

	// Verify callback was called
	if !called {
		t.Error("Expected resource changed callback to be called")
	}

	// Verify callback received correct parameters
	if receivedURI != "file:///test/resource" {
		t.Errorf("Expected URI 'file:///test/resource', got %s", receivedURI)
	}

	// Verify the received data is correct
	receivedParams, ok := receivedData.(protocol.ResourcesChangedParams)
	if !ok {
		t.Fatalf("Expected protocol.ResourcesChangedParams, got %T", receivedData)
	}

	if len(receivedParams.Added) != 1 {
		t.Errorf("Expected 1 added resource, got %d", len(receivedParams.Added))
	}

	if len(receivedParams.Modified) != 1 {
		t.Errorf("Expected 1 modified resource, got %d", len(receivedParams.Modified))
	}

	if len(receivedParams.Removed) != 1 {
		t.Errorf("Expected 1 removed resource, got %d", len(receivedParams.Removed))
	}
}

// Test resource updated callback functionality
func TestResourceUpdatedCallbackIntegration(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Test callback registration
	var receivedURI string
	var receivedData interface{}
	called := false
	callback := func(uri string, data interface{}) {
		receivedURI = uri
		receivedData = data
		called = true
	}

	client.SetResourceChangedCallback(callback)

	// Simulate a resource updated notification
	resourceUpdatedParams := protocol.ResourceUpdatedParams{
		URI: "file:///test/updated.txt",
		Contents: protocol.ResourceContents{
			URI:     "file:///test/updated.txt",
			Type:    "text/plain",
			Content: json.RawMessage(`"Updated content"`),
		},
		Deleted: false,
	}

	// Call the handle resource updated method directly
	err := client.handleResourceUpdated(context.Background(), resourceUpdatedParams)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("handleResourceUpdated failed: %v", err)
	}

	// Verify callback was called
	if !called {
		t.Error("Expected resource updated callback to be called")
	}

	// Verify callback received correct parameters
	if receivedURI != "file:///test/updated.txt" {
		t.Errorf("Expected URI 'file:///test/updated.txt', got %s", receivedURI)
	}

	// Verify the received data is correct
	receivedParams, ok := receivedData.(protocol.ResourceUpdatedParams)
	if !ok {
		t.Fatalf("Expected protocol.ResourceUpdatedParams, got %T", receivedData)
	}

	if receivedParams.URI != "file:///test/updated.txt" {
		t.Errorf("Expected URI 'file:///test/updated.txt', got %s", receivedParams.URI)
	}

	if receivedParams.Deleted != false {
		t.Errorf("Expected deleted=false, got %t", receivedParams.Deleted)
	}

	var content string
	if err := json.Unmarshal(receivedParams.Contents.Content, &content); err != nil {
		t.Fatalf("Failed to unmarshal content: %v", err)
	}
	if content != "Updated content" {
		t.Errorf("Expected content 'Updated content', got %s", content)
	}
}

// Test callback thread safety
func TestCallbackThreadSafety(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Test concurrent callback registration
	done := make(chan bool, 10)

	// Start multiple goroutines that register callbacks
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Register sampling callback
			client.SetSamplingCallback(func(event protocol.SamplingEvent) {
				// Do nothing
			})

			// Register resource callback
			client.SetResourceChangedCallback(func(uri string, data interface{}) {
				// Do nothing
			})
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent callback registration")
		}
	}
}

// Test callback panic handling
func TestCallbackPanicHandling(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Register a callback that panics
	client.SetSamplingCallback(func(event protocol.SamplingEvent) {
		panic("test panic")
	})

	// This should not crash the client
	sampleParams := protocol.SampleParams{
		Messages:  []protocol.Message{{Role: "user", Content: "Test"}},
		RequestID: "panic-test",
	}

	result, err := client.handleSample(context.Background(), sampleParams)

	// Should still return a result despite the panic
	if err != nil {
		t.Fatalf("handleSample failed: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result even when callback panics")
	}
}

// Test callback nil handling
func TestCallbackNilHandling(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Set callback to nil
	client.SetSamplingCallback(nil)
	client.SetResourceChangedCallback(nil)

	// Resource callbacks should handle nil gracefully
	err := client.handleResourcesChanged(context.Background(), protocol.ResourcesChangedParams{
		URI: "test://resource",
	})
	if err != nil {
		t.Errorf("handleResourcesChanged should handle nil callback gracefully: %v", err)
	}

	err = client.handleResourceUpdated(context.Background(), protocol.ResourceUpdatedParams{
		URI: "test://resource",
	})
	if err != nil {
		t.Errorf("handleResourceUpdated should handle nil callback gracefully: %v", err)
	}
}

// Test callback registration validation
func TestCallbackRegistrationValidation(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Test registering valid callbacks
	samplingCalled := false
	resourceCalled := false

	client.SetSamplingCallback(func(event protocol.SamplingEvent) {
		samplingCalled = true
	})

	client.SetResourceChangedCallback(func(uri string, data interface{}) {
		resourceCalled = true
	})

	// Trigger callbacks
	client.handleSample(context.Background(), protocol.SampleParams{
		Messages:  []protocol.Message{{Role: "user", Content: "Test"}},
		RequestID: "validation-test",
	})

	client.handleResourcesChanged(context.Background(), protocol.ResourcesChangedParams{
		URI: "test://validation",
	})

	// Verify callbacks were called
	if !samplingCalled {
		t.Error("Sampling callback was not called")
	}

	if !resourceCalled {
		t.Error("Resource callback was not called")
	}
}

// Test invalid parameters handling in callbacks
func TestCallbackInvalidParameters(t *testing.T) {
	mockTransport := newMockTransport()
	client := New(mockTransport)

	// Test with invalid sample parameters
	_, err := client.handleSample(context.Background(), "invalid")
	if err == nil {
		t.Error("Expected error for invalid sample parameters")
	}

	// Test with invalid resource parameters
	err = client.handleResourcesChanged(context.Background(), "invalid")
	if err == nil {
		t.Error("Expected error for invalid resource changed parameters")
	}

	err = client.handleResourceUpdated(context.Background(), "invalid")
	if err == nil {
		t.Error("Expected error for invalid resource updated parameters")
	}
}
