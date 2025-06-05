package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// mockHTTPTransport for HTTP handler testing
type mockHTTPTransport struct {
	*mockTransport
	sendRequestFunc func(ctx context.Context, method string, params interface{}) (interface{}, error)
}

func newMockHTTPTransport() *mockHTTPTransport {
	return &mockHTTPTransport{
		mockTransport: newMockTransport(),
	}
}

func (m *mockHTTPTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if m.sendRequestFunc != nil {
		return m.sendRequestFunc(ctx, method, params)
	}
	return m.mockTransport.SendRequest(ctx, method, params)
}

func TestNewHTTPHandler(t *testing.T) {
	handler := NewHTTPHandler()
	if handler == nil {
		t.Fatal("Expected NewHTTPHandler to return a handler")
	}

	if len(handler.allowedOrigins) != 1 || handler.allowedOrigins[0] != "*" {
		t.Errorf("Expected default allowed origins to be ['*'], got %v", handler.allowedOrigins)
	}
}

func TestNewStreamableHTTPHandler(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	if handler == nil {
		t.Fatal("Expected NewStreamableHTTPHandler to return a handler")
	}

	if handler.sessions == nil {
		t.Error("Expected sessions map to be initialized")
	}

	if handler.sseConnections == nil {
		t.Error("Expected sseConnections map to be initialized")
	}
}

func TestHTTPHandlerSetTransport(t *testing.T) {
	handler := NewHTTPHandler()
	transport := newMockHTTPTransport()

	handler.SetTransport(transport)

	// Access the transport via reflection or through public methods would be ideal,
	// but since it's private, we'll test by making a request
	if handler.transport != transport {
		t.Error("Expected transport to be set")
	}
}

func TestHTTPHandlerSetAllowedOrigins(t *testing.T) {
	handler := NewHTTPHandler()
	origins := []string{"https://example.com", "https://test.com"}

	handler.SetAllowedOrigins(origins)

	if len(handler.allowedOrigins) != 2 {
		t.Errorf("Expected 2 allowed origins, got %d", len(handler.allowedOrigins))
	}
}

func TestHTTPHandlerAddAllowedOrigin(t *testing.T) {
	handler := NewHTTPHandler()

	handler.AddAllowedOrigin("https://example.com")
	handler.AddAllowedOrigin("https://test.com")

	if len(handler.allowedOrigins) != 3 { // "*" + 2 added
		t.Errorf("Expected 3 allowed origins, got %d", len(handler.allowedOrigins))
	}
}

func TestHTTPHandlerIsOriginAllowed(t *testing.T) {
	handler := NewHTTPHandler()

	// Test wildcard (default)
	if !handler.isOriginAllowed("https://example.com") {
		t.Error("Expected wildcard to allow all origins")
	}

	// Test empty origin
	if !handler.isOriginAllowed("") {
		t.Error("Expected empty origin to be allowed")
	}

	// Test specific origins
	handler.SetAllowedOrigins([]string{"https://example.com"})

	if !handler.isOriginAllowed("https://example.com") {
		t.Error("Expected https://example.com to be allowed")
	}

	if handler.isOriginAllowed("https://evil.com") {
		t.Error("Expected https://evil.com to be denied")
	}
}

func TestHTTPHandlerServeHTTP_OriginBlocked(t *testing.T) {
	handler := NewHTTPHandler()
	handler.SetAllowedOrigins([]string{"https://example.com"})

	req := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestHTTPHandlerServeHTTP_MethodNotAllowed(t *testing.T) {
	handler := NewHTTPHandler()

	req := httptest.NewRequest("PATCH", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHTTPHandlerHandleOptionsRequest(t *testing.T) {
	handler := NewHTTPHandler()

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("Expected CORS origin header to be set")
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected CORS methods header to be set")
	}
}

func TestHTTPHandlerHandlePostRequest_NoTransport(t *testing.T) {
	handler := NewHTTPHandler()
	// No transport set

	initReq := &protocol.Request{
		Method: protocol.MethodInitialize,
		ID:     "test-id",
		Params: json.RawMessage(`{"name": "test-client", "version": "1.0.0"}`),
	}

	reqJSON, _ := json.Marshal(initReq)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHTTPHandlerHandlePostRequest_InvalidJSON(t *testing.T) {
	handler := NewHTTPHandler()
	transport := newMockHTTPTransport()
	handler.SetTransport(transport)

	req := httptest.NewRequest("POST", "/", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHTTPHandlerHandlePostRequest_ValidRequest(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	transport := newMockHTTPTransport()

	// Mock successful response
	transport.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		return &protocol.InitializeResult{
			ProtocolVersion: protocol.ProtocolRevision,
			Name:            "test-server",
			Version:         "1.0.0",
			Capabilities:    map[string]bool{},
		}, nil
	}

	handler.SetTransport(transport)

	initReq := &protocol.Request{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		Method:         protocol.MethodInitialize,
		ID:             "test-id",
		Params:         json.RawMessage(`{"name": "test-client", "version": "1.0.0"}`),
	}

	reqJSON, _ := json.Marshal(initReq)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should have session ID header for initialize request
	sessionID := w.Header().Get("MCP-Session-ID")
	if sessionID == "" {
		t.Error("Expected MCP-Session-ID header for initialize request")
	}
}

func TestHTTPHandlerHandlePostRequest_Notification(t *testing.T) {
	handler := NewHTTPHandler()
	transport := newMockHTTPTransport()
	handler.SetTransport(transport)

	notif := &protocol.Notification{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		Method:         protocol.MethodInitialized,
		Params:         json.RawMessage(`{}`),
	}

	notifJSON, _ := json.Marshal(notif)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(notifJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestHTTPHandlerHandleDeleteRequest_NoSessionID(t *testing.T) {
	handler := NewStreamableHTTPHandler()

	req := httptest.NewRequest("DELETE", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHTTPHandlerHandleDeleteRequest_SessionNotFound(t *testing.T) {
	handler := NewStreamableHTTPHandler()

	req := httptest.NewRequest("DELETE", "/", nil)
	req.Header.Set("MCP-Session-ID", "nonexistent-session")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHTTPHandlerHandleDeleteRequest_Success(t *testing.T) {
	handler := NewStreamableHTTPHandler()

	// Create a session
	sessionID := "test-session"
	handler.sessions[sessionID] = &SessionInfo{
		ID:        sessionID,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest("DELETE", "/", nil)
	req.Header.Set("MCP-Session-ID", sessionID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Session should be removed
	if _, exists := handler.sessions[sessionID]; exists {
		t.Error("Expected session to be removed")
	}
}

func TestHTTPHandlerHandleGetRequest_SSE(t *testing.T) {
	handler := NewStreamableHTTPHandler()

	// Create a session
	sessionID := "test-session"
	handler.sessions[sessionID] = &SessionInfo{
		ID:        sessionID,
		CreatedAt: time.Now(),
	}

	// Create a context that we can cancel to stop the SSE
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
	req.Header.Set("MCP-Session-ID", sessionID)

	// Use a custom ResponseWriter that supports Flusher
	w := &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		flusher:          true,
	}

	// Test in a goroutine since this will block
	done := make(chan bool, 1)
	go func() {
		defer func() { done <- true }()
		handler.ServeHTTP(w, req)
	}()

	// Wait a bit for headers to be written
	time.Sleep(100 * time.Millisecond)

	// Check that SSE headers are set
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Error("Expected Content-Type to be text/event-stream")
	}

	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Error("Expected Cache-Control to be no-cache")
	}

	// Cancel the request to stop the SSE stream
	cancel()

	// Wait for handler to finish
	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Handler did not finish in time")
	}
}

func TestHTTPHandlerBroadcastNotification(t *testing.T) {
	handler := NewStreamableHTTPHandler()

	// Create a mock SSE connection
	w := &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		flusher:          true,
	}

	conn := &SSEConnection{
		flusher:   w,
		writer:    w,
		sessionID: "test-session",
		closeCh:   make(chan struct{}),
	}

	handler.sseConnections["test-conn"] = conn

	// Test broadcast
	params := map[string]interface{}{
		"message": "test notification",
	}

	handler.BroadcastNotification("test.notification", params)

	// Check if notification was written (we can't easily verify the exact content
	// without more complex mocking, but we can verify no panic occurred)
}

// mockResponseWriter implements http.ResponseWriter and http.Flusher
type mockResponseWriter struct {
	*httptest.ResponseRecorder
	flusher bool
}

func (m *mockResponseWriter) Flush() {
	// Mock flush behavior - implementation not needed for tests
	_ = m.flusher
}

func TestHTTPHandlerSessionManagement(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	transport := newMockHTTPTransport()
	handler.SetTransport(transport)

	// Mock initialize response
	transport.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		return &protocol.InitializeResult{
			ProtocolVersion: protocol.ProtocolRevision,
			Name:            "test-server",
			Version:         "1.0.0",
			Capabilities:    map[string]bool{},
		}, nil
	}

	// Test initialize without session ID (should create new session)
	initReq := &protocol.Request{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		Method:         protocol.MethodInitialize,
		ID:             "test-id",
		Params:         json.RawMessage(`{"name": "test-client", "version": "1.0.0"}`),
	}

	reqJSON, _ := json.Marshal(initReq)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	sessionID := w.Header().Get("MCP-Session-ID")
	if sessionID == "" {
		t.Error("Expected session ID to be created for initialize request")
	}

	// Verify session was stored
	if _, exists := handler.sessions[sessionID]; !exists {
		t.Error("Expected session to be stored")
	}

	// Test subsequent request with session ID
	pingReq := &protocol.Request{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		Method:         protocol.MethodPing,
		ID:             "ping-id",
		Params:         json.RawMessage(`{}`),
	}

	reqJSON, _ = json.Marshal(pingReq)
	req = httptest.NewRequest("POST", "/", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Session-ID", sessionID)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHTTPHandlerInvalidSession(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	transport := newMockHTTPTransport()
	handler.SetTransport(transport)

	// Test non-initialize request with invalid session
	pingReq := &protocol.Request{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		Method:         protocol.MethodPing,
		ID:             "ping-id",
		Params:         json.RawMessage(`{}`),
	}

	reqJSON, _ := json.Marshal(pingReq)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Session-ID", "invalid-session")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHTTPHandlerStreamingRequest(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	transport := newMockHTTPTransport()
	handler.SetTransport(transport)

	// Mock response
	transport.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		return &protocol.PingResult{Timestamp: time.Now().UnixNano()}, nil
	}

	// Create session
	sessionID := "test-session"
	handler.sessions[sessionID] = &SessionInfo{
		ID:        sessionID,
		CreatedAt: time.Now(),
	}

	pingReq := &protocol.Request{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		Method:         protocol.MethodPing,
		ID:             "ping-id",
		Params:         json.RawMessage(`{}`),
	}

	reqJSON, _ := json.Marshal(pingReq)

	// Create a context that we can cancel to stop the SSE
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, "POST", "/", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Session-ID", sessionID)
	req.Header.Set("Accept", "text/event-stream")

	// Use mock ResponseWriter with Flusher support
	w := &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		flusher:          true,
	}

	// Test in goroutine since it will block
	done := make(chan bool, 1)
	go func() {
		defer func() { done <- true }()
		handler.ServeHTTP(w, req)
	}()

	// Wait for initial setup
	time.Sleep(100 * time.Millisecond)

	// Check SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Error("Expected Content-Type to be text/event-stream")
	}

	// Cancel to stop the stream
	cancel()

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Streaming request did not complete in time")
	}
}
