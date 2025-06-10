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

// TestOriginValidation tests the Origin header validation functionality
func TestOriginValidation(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		expectAllowed  bool
	}{
		{
			name:           "localhost HTTP allowed",
			allowedOrigins: []string{"http://localhost", "https://localhost"},
			requestOrigin:  "http://localhost",
			expectAllowed:  true,
		},
		{
			name:           "localhost HTTPS allowed",
			allowedOrigins: []string{"http://localhost", "https://localhost"},
			requestOrigin:  "https://localhost",
			expectAllowed:  true,
		},
		{
			name:           "localhost with port allowed",
			allowedOrigins: []string{"http://localhost", "https://localhost"},
			requestOrigin:  "http://localhost:3000",
			expectAllowed:  true,
		},
		{
			name:           "127.0.0.1 allowed when localhost configured",
			allowedOrigins: []string{"http://localhost", "https://localhost"},
			requestOrigin:  "http://127.0.0.1",
			expectAllowed:  true,
		},
		{
			name:           "IPv6 localhost allowed",
			allowedOrigins: []string{"http://localhost", "https://localhost"},
			requestOrigin:  "http://::1",
			expectAllowed:  true,
		},
		{
			name:           "wildcard allows everything",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://example.com",
			expectAllowed:  true,
		},
		{
			name:           "specific origin allowed",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "https://example.com",
			expectAllowed:  true,
		},
		{
			name:           "unauthorized origin rejected",
			allowedOrigins: []string{"https://authorized.com"},
			requestOrigin:  "https://malicious.com",
			expectAllowed:  false,
		},
		{
			name:           "empty origin rejected by default",
			allowedOrigins: []string{"http://localhost", "https://localhost"},
			requestOrigin:  "",
			expectAllowed:  false,
		},
		{
			name:           "mixed schemes rejected for non-localhost",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "http://example.com",
			expectAllowed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHTTPHandler()
			handler.SetAllowedOrigins(tt.allowedOrigins)

			allowed := handler.isOriginAllowed(tt.requestOrigin)
			if allowed != tt.expectAllowed {
				t.Errorf("isOriginAllowed(%q) = %v, want %v", tt.requestOrigin, allowed, tt.expectAllowed)
			}
		})
	}
}

// TestOriginMatchingHelpers tests the helper functions for origin matching
func TestOriginMatchingHelpers(t *testing.T) {
	handler := NewHTTPHandler()

	// Test isLocalhostPattern
	localhostPatterns := []string{
		"http://localhost",
		"https://localhost",
		"http://127.0.0.1",
		"https://127.0.0.1",
		"http://::1",
		"https://::1",
	}

	for _, pattern := range localhostPatterns {
		if !handler.isLocalhostPattern(pattern) {
			t.Errorf("isLocalhostPattern(%q) should return true", pattern)
		}
	}

	nonLocalhostPatterns := []string{
		"https://example.com",
		"http://192.168.1.1",
		"https://192.168.1.1",
	}

	for _, pattern := range nonLocalhostPatterns {
		if handler.isLocalhostPattern(pattern) {
			t.Errorf("isLocalhostPattern(%q) should return false", pattern)
		}
	}

	// Test isLocalhostOrigin
	localhostOrigins := []string{
		"http://localhost",
		"https://localhost",
		"http://localhost:3000",
		"https://localhost:8080",
		"http://127.0.0.1",
		"http://127.0.0.1:5000",
		"http://::1",
		"https://::1:4000",
	}

	for _, origin := range localhostOrigins {
		if !handler.isLocalhostOrigin(origin) {
			t.Errorf("isLocalhostOrigin(%q) should return true", origin)
		}
	}

	nonLocalhostOrigins := []string{
		"https://example.com",
		"http://192.168.1.1",
		"https://192.168.1.1:8080",
	}

	for _, origin := range nonLocalhostOrigins {
		if handler.isLocalhostOrigin(origin) {
			t.Errorf("isLocalhostOrigin(%q) should return false", origin)
		}
	}
}

// TestSetAllowWildcardOrigin tests the wildcard origin management
func TestSetAllowWildcardOrigin(t *testing.T) {
	handler := NewHTTPHandler()

	// Initially should not have wildcard
	if handler.isOriginAllowed("https://example.com") {
		t.Error("Should not allow arbitrary origins by default")
	}

	// Enable wildcard
	handler.SetAllowWildcardOrigin(true)
	if !handler.isOriginAllowed("https://example.com") {
		t.Error("Should allow arbitrary origins after enabling wildcard")
	}

	// Disable wildcard
	handler.SetAllowWildcardOrigin(false)
	if handler.isOriginAllowed("https://example.com") {
		t.Error("Should not allow arbitrary origins after disabling wildcard")
	}
}

// TestHTTPHandlerOriginRejection tests that unauthorized origins are properly rejected
func TestHTTPHandlerOriginRejection(t *testing.T) {
	handler := NewHTTPHandler()

	// Create a mock transport
	mockTransport := newMockHTTPTransport()
	handler.SetTransport(mockTransport)

	// Test with unauthorized origin
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"jsonrpc":"2.0","method":"test","id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://malicious.com")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for unauthorized origin, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Origin not allowed") {
		t.Errorf("Expected 'Origin not allowed' in response body, got %q", w.Body.String())
	}
}

// TestHTTPHandlerOriginAcceptance tests that authorized origins are properly accepted
func TestHTTPHandlerOriginAcceptance(t *testing.T) {
	handler := NewHTTPHandler()

	// Create a mock transport that returns a simple response
	mockTransport := newMockHTTPTransport()
	mockTransport.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		return map[string]string{"result": "success"}, nil
	}
	handler.SetTransport(mockTransport)

	// Test with authorized origin (localhost)
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"jsonrpc":"2.0","method":"test","id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for authorized origin, got %d", w.Code)
	}
}

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

	expectedOrigins := []string{"http://localhost", "https://localhost"}
	if len(handler.allowedOrigins) != len(expectedOrigins) {
		t.Errorf("Expected default allowed origins to be %v, got %v", expectedOrigins, handler.allowedOrigins)
	}
	for i, expected := range expectedOrigins {
		if handler.allowedOrigins[i] != expected {
			t.Errorf("Expected default allowed origins to be %v, got %v", expectedOrigins, handler.allowedOrigins)
		}
	}
}

func TestNewStreamableHTTPHandler(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	if handler == nil {
		t.Fatal("Expected NewStreamableHTTPHandler to return a handler")
	}

	expectedOrigins := []string{"http://localhost", "https://localhost"}
	if len(handler.allowedOrigins) != len(expectedOrigins) {
		t.Errorf("Expected default allowed origins to be %v, got %v", expectedOrigins, handler.allowedOrigins)
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

	if len(handler.allowedOrigins) != 4 { // 2 default + 2 added
		t.Errorf("Expected 4 allowed origins, got %d", len(handler.allowedOrigins))
	}
}

func TestHTTPHandlerIsOriginAllowed(t *testing.T) {
	handler := NewHTTPHandler()

	// Test default localhost origins
	if !handler.isOriginAllowed("http://localhost") {
		t.Error("Expected http://localhost to be allowed by default")
	}

	// Test non-default origin (should not be allowed)
	if handler.isOriginAllowed("https://example.com") {
		t.Error("Expected https://example.com to not be allowed by default")
	}

	// Test empty origin (should not be allowed for security)
	if handler.isOriginAllowed("") {
		t.Error("Expected empty origin to not be allowed for security")
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHTTPHandlerHandleOptionsRequest(t *testing.T) {
	handler := NewHTTPHandler()
	handler.SetAllowedOrigins([]string{"https://example.com"})

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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestHTTPHandlerHandleDeleteRequest_NoSessionID(t *testing.T) {
	handler := NewStreamableHTTPHandler()

	req := httptest.NewRequest("DELETE", "/", nil)
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	now := time.Now()
	handler.sessions[sessionID] = &SessionInfo{
		ID:        sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour), // Set expiration
	}

	req := httptest.NewRequest("DELETE", "/", nil)
	req.Header.Set("MCP-Session-ID", sessionID)
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	now := time.Now()
	handler.sessions[sessionID] = &SessionInfo{
		ID:        sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour), // Set expiration
	}

	// Create a context that we can cancel to stop the SSE
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
	req.Header.Set("MCP-Session-ID", sessionID)
	req.Header.Set("Origin", "http://localhost") // Add valid origin

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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin
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
	now := time.Now()
	handler.sessions[sessionID] = &SessionInfo{
		ID:        sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour), // Set expiration
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
	req.Header.Set("Origin", "http://localhost") // Add valid origin

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
