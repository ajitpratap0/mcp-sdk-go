package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// TestPanicRecovery tests that panics in handlers are recovered and converted to errors
func TestPanicRecovery(t *testing.T) {
	// Create a test transport - need to initialize the maps
	testTransport := transport.NewBaseTransport()

	// Override the transport's request handler to simulate a panic in server handler
	testTransport.RegisterRequestHandler("test.panic", func(ctx context.Context, params interface{}) (interface{}, error) {
		panic("test panic in handler")
	})

	// Create a test request
	req := &protocol.Request{
		ID:     json.RawMessage(`"test-1"`),
		Method: "test.panic",
		Params: json.RawMessage(`{}`),
	}

	// Call HandleRequest which should recover from the panic
	resp, err := testTransport.HandleRequest(context.Background(), req)

	// Should get an error response, not a panic
	if err != nil {
		t.Fatalf("HandleRequest returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected error response, got nil")
	}

	// Check that we got an internal error response
	var errorResp struct {
		Error *protocol.ErrorObject `json:"error"`
	}

	respJSON, _ := json.Marshal(resp)
	if err := json.Unmarshal(respJSON, &errorResp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if errorResp.Error == nil {
		t.Fatal("Expected error in response, got none")
	}

	if errorResp.Error.Code != protocol.InternalError {
		t.Errorf("Expected internal error code %d, got %d", protocol.InternalError, errorResp.Error.Code)
	}

	if errorResp.Error.Message != "Internal server error processing test.panic" {
		t.Errorf("Unexpected error message: %s", errorResp.Error.Message)
	}
}

// TestNotificationPanicRecovery tests panic recovery in notification handlers
func TestNotificationPanicRecovery(t *testing.T) {
	// Create a test transport
	testTransport := transport.NewBaseTransport()

	// Register a notification handler that panics
	testTransport.RegisterNotificationHandler("test.panic.notif", func(ctx context.Context, params interface{}) error {
		panic("test panic in notification handler")
	})

	// Create a test notification
	notif := &protocol.Notification{
		Method: "test.panic.notif",
		Params: json.RawMessage(`{}`),
	}

	// Call HandleNotification which should recover from the panic
	err := testTransport.HandleNotification(context.Background(), notif)

	// Should get an error, not a panic
	if err == nil {
		t.Fatal("Expected error from panic recovery, got nil")
	}

	expectedError := "internal error processing notification test.panic.notif: test panic in notification handler"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

// TestGoroutinePanicRecovery tests panic recovery in goroutines
// Note: SafeGo function was removed in clean architecture - skipping this test
func TestGoroutinePanicRecovery(t *testing.T) {
	t.Skip("SafeGo function was removed in clean architecture refactor")
}
