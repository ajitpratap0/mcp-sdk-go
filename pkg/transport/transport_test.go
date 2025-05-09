package transport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func TestNewBaseTransport(t *testing.T) {
	bt := NewBaseTransport()

	if bt.requestHandlers == nil {
		t.Error("Expected requestHandlers to be initialized")
	}

	if bt.notificationHandlers == nil {
		t.Error("Expected notificationHandlers to be initialized")
	}

	if bt.progressHandlers == nil {
		t.Error("Expected progressHandlers to be initialized")
	}

	if bt.pendingRequests == nil {
		t.Error("Expected pendingRequests to be initialized")
	}

	if bt.nextID != 1 {
		t.Errorf("Expected nextID to be 1, got %d", bt.nextID)
	}
}

func TestRegisterRequestHandler(t *testing.T) {
	bt := NewBaseTransport()
	handler := func(ctx context.Context, params interface{}) (interface{}, error) {
		return nil, nil
	}

	bt.RegisterRequestHandler("test", handler)

	bt.RLock()
	defer bt.RUnlock()

	h, ok := bt.requestHandlers["test"]
	if !ok {
		t.Error("Expected request handler to be registered")
	}

	if h == nil {
		t.Error("Expected registered handler to be non-nil")
	}
}

func TestRegisterNotificationHandler(t *testing.T) {
	bt := NewBaseTransport()
	handler := func(ctx context.Context, params interface{}) error {
		return nil
	}

	bt.RegisterNotificationHandler("test", handler)

	bt.RLock()
	defer bt.RUnlock()

	h, ok := bt.notificationHandlers["test"]
	if !ok {
		t.Error("Expected notification handler to be registered")
	}

	if h == nil {
		t.Error("Expected registered handler to be non-nil")
	}
}

func TestRegisterProgressHandler(t *testing.T) {
	bt := NewBaseTransport()
	handler := func(params interface{}) error {
		return nil
	}

	bt.RegisterProgressHandler("test-id", handler)

	bt.RLock()
	defer bt.RUnlock()

	h, ok := bt.progressHandlers["test-id"]
	if !ok {
		t.Error("Expected progress handler to be registered")
	}

	if h == nil {
		t.Error("Expected registered handler to be non-nil")
	}
}

func TestUnregisterProgressHandler(t *testing.T) {
	bt := NewBaseTransport()
	handler := func(params interface{}) error {
		return nil
	}

	bt.RegisterProgressHandler("test-id", handler)
	bt.UnregisterProgressHandler("test-id")

	bt.RLock()
	defer bt.RUnlock()

	_, ok := bt.progressHandlers["test-id"]
	if ok {
		t.Error("Expected progress handler to be unregistered")
	}
}

func TestGetNextID(t *testing.T) {
	bt := NewBaseTransport()

	id1 := bt.GetNextID()
	id2 := bt.GetNextID()

	if id1 != 1 {
		t.Errorf("Expected first ID to be 1, got %d", id1)
	}

	if id2 != 2 {
		t.Errorf("Expected second ID to be 2, got %d", id2)
	}
}

func TestGenerateID(t *testing.T) {
	bt := NewBaseTransport()

	id1 := bt.GenerateID()
	id2 := bt.GenerateID()

	if id1 == "" {
		t.Error("Expected generated ID to not be empty")
	}

	if id1 == id2 {
		t.Error("Expected generated IDs to be unique")
	}
}

func TestWaitForResponse(t *testing.T) {
	bt := NewBaseTransport()
	ctx := context.Background()

	// Create a response in a goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)

		resp := &protocol.Response{
			ID:     "test-id",
			Result: json.RawMessage(`"test-result"`),
		}

		bt.HandleResponse(resp)
	}()

	resp, err := bt.WaitForResponse(ctx, "test-id")
	if err != nil {
		t.Fatalf("Expected WaitForResponse to succeed, got error: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response to not be nil")
	}

	if resp.ID != "test-id" {
		t.Errorf("Expected response ID to be 'test-id', got %v", resp.ID)
	}
}

func TestWaitForResponseWithTimeout(t *testing.T) {
	bt := NewBaseTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := bt.WaitForResponse(ctx, "test-id")
	if err == nil {
		t.Fatal("Expected WaitForResponse to timeout")
	}
}

func TestHandleRequest(t *testing.T) {
	bt := NewBaseTransport()

	// Register a handler
	bt.RegisterRequestHandler("test", func(ctx context.Context, params interface{}) (interface{}, error) {
		return "success", nil
	})

	// Create a request
	req := &protocol.Request{
		ID:     "test-id",
		Method: "test",
		Params: json.RawMessage(`{}`),
	}

	// Handle the request
	resp, err := bt.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected HandleRequest to succeed, got error: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response to not be nil")
	}

	if resp.ID != "test-id" {
		t.Errorf("Expected response ID to be 'test-id', got %v", resp.ID)
	}

	// Check for error with unknown method
	req.Method = "unknown"
	resp, err = bt.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected HandleRequest to return error response, not error: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error in response for unknown method")
	}

	if resp.Error.Code != protocol.MethodNotFound {
		t.Errorf("Expected error code to be MethodNotFound, got %d", resp.Error.Code)
	}

	// Check for error in handler
	bt.RegisterRequestHandler("error", func(ctx context.Context, params interface{}) (interface{}, error) {
		return nil, errors.New("handler error")
	})

	req.Method = "error"
	resp, err = bt.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected HandleRequest to return error response, not error: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error in response for handler error")
	}

	if resp.Error.Code != protocol.InternalError {
		t.Errorf("Expected error code to be InternalError, got %d", resp.Error.Code)
	}
}

func TestHandleNotification(t *testing.T) {
	bt := NewBaseTransport()

	// Register a handler
	bt.RegisterNotificationHandler("test", func(ctx context.Context, params interface{}) error {
		return nil
	})

	// Create a notification
	notif := &protocol.Notification{
		Method: "test",
		Params: json.RawMessage(`{}`),
	}

	// Handle the notification
	err := bt.HandleNotification(context.Background(), notif)
	if err != nil {
		t.Fatalf("Expected HandleNotification to succeed, got error: %v", err)
	}

	// Check for error with unknown method
	notif.Method = "unknown"
	err = bt.HandleNotification(context.Background(), notif)
	if err == nil {
		t.Fatal("Expected HandleNotification to return error for unknown method")
	}

	if !errors.Is(err, ErrUnsupportedMethod) {
		t.Errorf("Expected error to be ErrUnsupportedMethod, got %v", err)
	}

	// Test progress notification handling
	progressHandler := func(params interface{}) error {
		return nil
	}

	bt.RegisterProgressHandler("123", progressHandler)

	progressParams := protocol.ProgressParams{
		ID:      "123",
		Message: "Processing",
		Percent: 50.0,
	}

	paramsBytes, _ := json.Marshal(progressParams)

	notif = &protocol.Notification{
		Method: protocol.MethodProgress,
		Params: paramsBytes,
	}

	err = bt.HandleNotification(context.Background(), notif)
	if err != nil {
		t.Fatalf("Expected progress notification to be handled, got error: %v", err)
	}
}

func TestWithRequestTimeout(t *testing.T) {
	opts := NewOptions(WithRequestTimeout(5 * time.Second))

	if opts.RequestTimeout != 5*time.Second {
		t.Errorf("Expected RequestTimeout to be 5s, got %v", opts.RequestTimeout)
	}
}
