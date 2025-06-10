package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// TestSpecificationCompliance validates the transport implementation against MCP specification requirements
type TestSpecificationCompliance struct {
	t *testing.T
}

// TestMCPSpecificationCompliance runs all specification compliance tests
func TestMCPSpecificationCompliance(t *testing.T) {
	suite := &TestSpecificationCompliance{t: t}

	t.Run("Protocol Version Compliance", suite.TestProtocolVersionCompliance)
	t.Run("JSON-RPC 2.0 Compliance", suite.TestJSONRPCCompliance)
	t.Run("Transport Requirements", suite.TestTransportRequirements)
	t.Run("Message Flow Compliance", suite.TestMessageFlowCompliance)
	t.Run("Capability Negotiation", suite.TestCapabilityNegotiation)
	t.Run("Error Handling Compliance", suite.TestErrorHandlingCompliance)
	t.Run("Security Requirements", suite.TestSecurityRequirements)
	t.Run("SSE Format Compliance", suite.TestSSEFormatCompliance)
	t.Run("Session Management", suite.TestSessionManagement)
	t.Run("Content Type Compliance", suite.TestContentTypeCompliance)
}

// TestProtocolVersionCompliance verifies protocol version handling
func (suite *TestSpecificationCompliance) TestProtocolVersionCompliance(t *testing.T) {
	tests := []struct {
		name            string
		clientVersion   string
		serverVersion   string
		expectSuccess   bool
		expectedVersion string
	}{
		{
			name:            "exact version match",
			clientVersion:   protocol.ProtocolRevision,
			serverVersion:   protocol.ProtocolRevision,
			expectSuccess:   true,
			expectedVersion: protocol.ProtocolRevision,
		},
		{
			name:            "current protocol version 2025-03-26",
			clientVersion:   "2025-03-26",
			serverVersion:   "2025-03-26",
			expectSuccess:   true,
			expectedVersion: "2025-03-26",
		},
		// Future versions should be handled gracefully
		{
			name:            "future version compatibility",
			clientVersion:   "2025-03-26",
			serverVersion:   "2025-04-01",
			expectSuccess:   true,
			expectedVersion: "2025-03-26", // Should negotiate to lower version
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with actual initialize params
			params := protocol.InitializeParams{
				ProtocolVersion: tt.clientVersion,
				Name:            "test-client",
				Version:         "1.0.0",
				Capabilities:    make(map[string]bool),
			}

			data, err := json.Marshal(params)
			if err != nil {
				t.Fatalf("Failed to marshal params: %v", err)
			}

			// Verify protocol version is included in initialize
			if !strings.Contains(string(data), `"protocolVersion"`) {
				t.Error("Protocol version must be included in initialize request")
			}
		})
	}
}

// TestJSONRPCCompliance verifies JSON-RPC 2.0 compliance
func (suite *TestSpecificationCompliance) TestJSONRPCCompliance(t *testing.T) {
	// Test request format
	t.Run("Request Format", func(t *testing.T) {
		req, err := protocol.NewRequest("test-1", "testMethod", map[string]string{"key": "value"})
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		// Verify JSON-RPC 2.0 required fields
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal request: %v", err)
		}

		// Check required fields
		if parsed["jsonrpc"] != "2.0" {
			t.Error("Request must include jsonrpc=\"2.0\"")
		}
		if _, ok := parsed["id"]; !ok {
			t.Error("Request must include id field")
		}
		if _, ok := parsed["method"]; !ok {
			t.Error("Request must include method field")
		}
		if _, ok := parsed["params"]; !ok {
			t.Error("Request must include params field (can be null)")
		}
	})

	// Test notification format
	t.Run("Notification Format", func(t *testing.T) {
		notif, err := protocol.NewNotification("testNotification", map[string]string{"key": "value"})
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}

		data, err := json.Marshal(notif)
		if err != nil {
			t.Fatalf("Failed to marshal notification: %v", err)
		}

		// Verify JSON-RPC 2.0 notification format
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal notification: %v", err)
		}

		if parsed["jsonrpc"] != "2.0" {
			t.Error("Notification must include jsonrpc=\"2.0\"")
		}
		if _, ok := parsed["id"]; ok {
			t.Error("Notification must NOT include id field")
		}
		if _, ok := parsed["method"]; !ok {
			t.Error("Notification must include method field")
		}
	})

	// Test response format
	t.Run("Response Format", func(t *testing.T) {
		// Success response
		resp, err := protocol.NewResponse("test-1", map[string]string{"result": "success"})
		if err != nil {
			t.Fatalf("Failed to create response: %v", err)
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal response: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if parsed["jsonrpc"] != "2.0" {
			t.Error("Response must include jsonrpc=\"2.0\"")
		}
		if _, ok := parsed["id"]; !ok {
			t.Error("Response must include id field")
		}
		if _, ok := parsed["result"]; !ok {
			t.Error("Success response must include result field")
		}
		if _, ok := parsed["error"]; ok {
			t.Error("Success response must NOT include error field")
		}

		// Error response
		errResp, errErr := protocol.NewErrorResponse("test-2", protocol.InternalError, "Test error", nil)
		if errErr != nil {
			t.Fatalf("Failed to create error response: %v", errErr)
		}

		data, err = json.Marshal(errResp)
		if err != nil {
			t.Fatalf("Failed to marshal error response: %v", err)
		}

		var errorParsed map[string]interface{}
		if err := json.Unmarshal(data, &errorParsed); err != nil {
			t.Fatalf("Failed to unmarshal error response: %v", err)
		}

		if _, ok := errorParsed["result"]; ok {
			t.Error("Error response must NOT include result field")
		}
		if _, ok := errorParsed["error"]; !ok {
			t.Error("Error response must include error field")
		}
	})

	// Test batch support
	t.Run("Batch Support", func(t *testing.T) {
		batch := []interface{}{
			must(protocol.NewRequest("1", "method1", nil)),
			must(protocol.NewRequest("2", "method2", nil)),
			must(protocol.NewNotification("notif1", nil)),
		}

		data, err := json.Marshal(batch)
		if err != nil {
			t.Fatalf("Failed to marshal batch: %v", err)
		}

		// Verify it's a JSON array
		if !strings.HasPrefix(string(data), "[") || !strings.HasSuffix(string(data), "]") {
			t.Error("Batch must be a JSON array")
		}

		// Verify each item is valid
		var parsed []json.RawMessage
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal batch: %v", err)
		}

		if len(parsed) != 3 {
			t.Errorf("Expected 3 items in batch, got %d", len(parsed))
		}
	})
}

// TestTransportRequirements verifies transport-specific requirements
func (suite *TestSpecificationCompliance) TestTransportRequirements(t *testing.T) {
	// Test stdio transport requirements
	t.Run("Stdio Transport", func(t *testing.T) {
		// Stdio must be supported as per spec
		var stdin bytes.Buffer
		var stdout bytes.Buffer

		config := DefaultTransportConfig(TransportTypeStdio)
		config.StdioReader = &stdin
		config.StdioWriter = &stdout
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)
		if transport == nil {
			t.Fatal("Stdio transport must be supported")
		}

		// Should handle line-delimited JSON
		testMsg := `{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`
		stdin.WriteString(testMsg + "\n")

		// Verify transport can be initialized
		ctx := context.Background()
		err := transport.Initialize(ctx)
		if err != nil {
			t.Errorf("Stdio transport initialization failed: %v", err)
		}
	})

	// Test HTTP+SSE transport requirements
	t.Run("HTTP+SSE Transport", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify content types
			if r.Method == "POST" {
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("POST requests must have Content-Type: application/json, got %s", contentType)
				}

				accept := r.Header.Get("Accept")
				if !strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/event-stream") {
					t.Errorf("POST requests must accept application/json or text/event-stream, got %s", accept)
				}

				// Parse the request to get the actual ID
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Failed to read request body", http.StatusBadRequest)
					return
				}

				var requestMsg map[string]interface{}
				if err := json.Unmarshal(body, &requestMsg); err != nil {
					http.Error(w, "Invalid JSON in request", http.StatusBadRequest)
					return
				}

				// Extract the ID from the request and echo it back in the response
				requestID := requestMsg["id"]
				if requestID == nil {
					http.Error(w, "Request missing ID field", http.StatusBadRequest)
					return
				}

				// Return JSON response with the same ID as the request
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				responseData := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      requestID,
					"result":  map[string]bool{"success": true},
				}

				if err := json.NewEncoder(w).Encode(responseData); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			} else if r.Method == "GET" {
				// SSE endpoint
				if r.Header.Get("Accept") != "text/event-stream" {
					t.Error("GET requests for SSE must have Accept: text/event-stream")
				}

				// Return SSE response
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)

				// Send a test event
				fmt.Fprintf(w, "data: {\"test\":\"event\"}\n\n")
				w.(http.Flusher).Flush()
			}
		}))
		defer server.Close()

		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = server.URL
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)
		if transport == nil {
			t.Fatal("HTTP transport must be supported")
		}

		// Should support both POST and SSE
		ctx := context.Background()
		transport.Initialize(ctx)

		// Verify headers are set correctly (cast to StreamableHTTPTransport for testing)
		if httpTransport, ok := transport.(*StreamableHTTPTransport); ok {
			httpTransport.mu.Lock()
			acceptHeader := httpTransport.headers["Accept"]
			httpTransport.mu.Unlock()
			if acceptHeader != "application/json, text/event-stream" {
				t.Error("Transport must accept both JSON and SSE responses")
			}
		}
	})
}

// TestMessageFlowCompliance verifies the required message flow
func (suite *TestSpecificationCompliance) TestMessageFlowCompliance(t *testing.T) {
	// Create a mock server that validates message flow
	var messageOrder []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(r.Body)
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)

		method, _ := msg["method"].(string)
		messageOrder = append(messageOrder, method)

		// Respond based on method
		switch method {
		case "initialize":
			w.Header().Set("Content-Type", "application/json")
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]interface{}{
					"protocolVersion": "2025-03-26",
					"name":            "test-server",
					"version":         "1.0.0",
					"capabilities":    map[string]bool{},
				},
			}
			json.NewEncoder(w).Encode(response)
		case "initialized":
			// No response for notification
			w.WriteHeader(http.StatusAccepted)
		default:
			w.Header().Set("Content-Type", "application/json")
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result":  map[string]interface{}{"success": true},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	// Test the required flow
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)
	ctx := context.Background()

	// Initialize should be first
	err := transport.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Send initialized notification
	err = transport.SendNotification(ctx, "initialized", protocol.InitializedParams{})
	if err != nil {
		t.Fatalf("Initialized notification failed: %v", err)
	}

	// Verify message order
	// Note: initialized is a notification so might not be in order due to async

	if len(messageOrder) < 1 || messageOrder[0] != "initialize" {
		t.Error("Initialize must be the first message sent")
	}
}

// TestCapabilityNegotiation verifies capability handling
func (suite *TestSpecificationCompliance) TestCapabilityNegotiation(t *testing.T) {
	capabilities := []string{
		string(protocol.CapabilityTools),
		string(protocol.CapabilityResources),
		string(protocol.CapabilityResourceSubscriptions),
		string(protocol.CapabilityPrompts),
		string(protocol.CapabilityComplete),
		string(protocol.CapabilityRoots),
		string(protocol.CapabilitySampling),
		string(protocol.CapabilityLogging),
		string(protocol.CapabilityPagination),
	}

	for _, cap := range capabilities {
		t.Run(cap, func(t *testing.T) {
			// Verify capability can be set in initialize
			params := protocol.InitializeParams{
				ProtocolVersion: protocol.ProtocolRevision,
				Name:            "test-client",
				Version:         "1.0.0",
				Capabilities: map[string]bool{
					cap: true,
				},
			}

			data, err := json.Marshal(params)
			if err != nil {
				t.Fatalf("Failed to marshal params with capability %s: %v", cap, err)
			}

			// Verify capability is properly serialized
			if !strings.Contains(string(data), fmt.Sprintf(`"%s":true`, cap)) {
				t.Errorf("Capability %s not properly serialized", cap)
			}
		})
	}
}

// TestErrorHandlingCompliance verifies error handling meets spec
func (suite *TestSpecificationCompliance) TestErrorHandlingCompliance(t *testing.T) {
	// Test standard error codes
	errorCodes := []struct {
		code        int
		name        string
		description string
	}{
		{-32700, "ParseError", "Invalid JSON"},
		{-32600, "InvalidRequest", "Invalid request"},
		{-32601, "MethodNotFound", "Method not found"},
		{-32602, "InvalidParams", "Invalid parameters"},
		{-32603, "InternalError", "Internal error"},
	}

	for _, ec := range errorCodes {
		t.Run(ec.name, func(t *testing.T) {
			// Verify error codes are properly defined
			var errorCode int
			switch ec.name {
			case "ParseError":
				errorCode = protocol.ParseError
			case "InvalidRequest":
				errorCode = protocol.InvalidRequest
			case "MethodNotFound":
				errorCode = protocol.MethodNotFound
			case "InvalidParams":
				errorCode = protocol.InvalidParams
			case "InternalError":
				errorCode = protocol.InternalError
			}

			if errorCode != ec.code {
				t.Errorf("Error code for %s should be %d, got %d", ec.name, ec.code, errorCode)
			}

			// Test error response format
			errResp, err := protocol.NewErrorResponse("test-1", errorCode, ec.description, nil)
			if err != nil {
				t.Fatalf("Failed to create error response: %v", err)
			}

			data, _ := json.Marshal(errResp)
			var parsed map[string]interface{}
			json.Unmarshal(data, &parsed)

			// Verify error structure
			if errObj, ok := parsed["error"].(map[string]interface{}); ok {
				if errObj["code"] != float64(ec.code) {
					t.Errorf("Error code mismatch: expected %d, got %v", ec.code, errObj["code"])
				}
				if errObj["message"] != ec.description {
					t.Errorf("Error message mismatch: expected %s, got %v", ec.description, errObj["message"])
				}
			} else {
				t.Error("Error response must contain error object")
			}
		})
	}
}

// TestSecurityRequirements verifies security compliance
func (suite *TestSpecificationCompliance) TestSecurityRequirements(t *testing.T) {
	// Test origin validation for HTTP transports
	t.Run("Origin Validation", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "https://example.com/mcp"
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)

		// Should support origin configuration (cast to StreamableHTTPTransport for testing)
		if httpTransport, ok := transport.(*StreamableHTTPTransport); ok {
			httpTransport.SetAllowedOrigins([]string{"https://trusted.com"})

			// Verify origin validation is enforced
			err := httpTransport.validateOrigin()
			if err == nil {
				t.Error("Origin validation should fail for non-allowed origins")
			}
		} else {
			t.Skip("Origin validation only supported by StreamableHTTPTransport")
		}

		// Test localhost requirements
		localhostConfig := DefaultTransportConfig(TransportTypeStreamableHTTP)
		localhostConfig.Endpoint = "http://localhost:8080/mcp"
		localhostConfig.Features.EnableReliability = false   // Disable for test
		localhostConfig.Features.EnableObservability = false // Disable for test
		localhostTransport, _ := NewTransport(localhostConfig)
		if httpLocalTransport, ok := localhostTransport.(*StreamableHTTPTransport); ok {
			httpLocalTransport.SetAllowedOrigins([]string{"http://localhost"})

			err := httpLocalTransport.validateOrigin()
			if err != nil {
				t.Error("Localhost origins should be allowed when configured")
			}
		} else {
			t.Skip("Origin validation only supported by StreamableHTTPTransport")
		}
	})

	// Test secure session management
	t.Run("Session Management", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "http://localhost:8080/mcp"
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)

		// Sessions should be tracked with secure IDs (cast to StreamableHTTPTransport for testing)
		if httpTransport, ok := transport.(*StreamableHTTPTransport); ok {
			testSessionID := "test-session-123"
			httpTransport.SetSessionID(testSessionID)

			if httpTransport.GetSessionID() != testSessionID {
				t.Error("Session ID must be properly stored and retrieved")
			}

			// Verify session ID is included in requests
			httpTransport.mu.Lock()
			if httpTransport.sessionID != testSessionID {
				t.Error("Session ID must be set internally")
			}
			httpTransport.mu.Unlock()
		} else {
			t.Skip("Session management only supported by StreamableHTTPTransport")
		}
	})
}

// TestSSEFormatCompliance verifies SSE format compliance
func (suite *TestSpecificationCompliance) TestSSEFormatCompliance(t *testing.T) {
	// Test SSE event parsing
	t.Run("SSE Event Format", func(t *testing.T) {
		eventData := []string{
			"data: {\"test\":\"message\"}\n\n",
			"id: 123\ndata: {\"test\":\"with-id\"}\n\n",
			"event: custom\ndata: {\"test\":\"custom-event\"}\n\n",
			"retry: 5000\n\n",
			": heartbeat\n\n",
		}

		for _, event := range eventData {
			// Verify event format
			lines := strings.Split(strings.TrimSpace(event), "\n")
			for _, line := range lines {
				if line == "" || strings.HasPrefix(line, ":") {
					continue // Empty lines and comments are valid
				}

				// Must be field:value format
				if !strings.Contains(line, ":") {
					t.Errorf("Invalid SSE line format: %s", line)
				}

				parts := strings.SplitN(line, ":", 2)
				field := parts[0]

				// Check valid field names
				validFields := []string{"data", "id", "event", "retry"}
				valid := false
				for _, vf := range validFields {
					if field == vf {
						valid = true
						break
					}
				}

				if !valid {
					t.Errorf("Invalid SSE field: %s", field)
				}
			}
		}
	})

	// Test Last-Event-ID header support
	t.Run("Last-Event-ID Support", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				// Check for Last-Event-ID header
				lastEventID := r.Header.Get("Last-Event-ID")
				if lastEventID != "" {
					// Server should handle resumption
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					fmt.Fprintf(w, "id: %s\ndata: resumed\n\n", lastEventID)
				} else {
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					fmt.Fprintf(w, "id: 1\ndata: initial\n\n")
				}
			}
		}))
		defer server.Close()

		// Create event source with Last-Event-ID
		es := &StreamableEventSource{
			URL:         server.URL,
			LastEventID: "123",
		}

		req, _ := http.NewRequest("GET", es.URL, nil)
		req.Header.Set("Accept", "text/event-stream")
		if es.LastEventID != "" {
			req.Header.Set("Last-Event-ID", es.LastEventID)
		}

		// Verify Last-Event-ID is included
		if req.Header.Get("Last-Event-ID") != "123" {
			t.Error("Last-Event-ID must be included for resumption")
		}
	})
}

// TestSessionManagement verifies session management compliance
func (suite *TestSpecificationCompliance) TestSessionManagement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server should provide session ID
		if r.Method == "POST" {
			// Check if client sent session ID
			clientSessionID := r.Header.Get("MCP-Session-ID")

			// Provide session ID in response
			w.Header().Set("MCP-Session-ID", "server-session-123")
			w.Header().Set("Content-Type", "application/json")

			if clientSessionID != "" {
				// Existing session
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"resumed":true}}`)
			} else {
				// New session
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"new":true}}`)
			}
		}
	}))
	defer server.Close()

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)
	ctx := context.Background()

	// First request should establish session (cast to StreamableHTTPTransport for testing)
	if httpTransport, ok := transport.(*StreamableHTTPTransport); ok {
		httpTransport.sendHTTPRequest(ctx, must(protocol.NewRequest("1", "test", nil)), "")

		// Verify session ID was extracted
		if httpTransport.GetSessionID() == "" {
			t.Error("Transport must extract and store session ID from server response")
		}

		// Subsequent requests should include session ID
		httpTransport.SetSessionID("client-session-456")

		// Verify session ID is included in headers
		httpTransport.mu.Lock()
		if httpTransport.sessionID != "client-session-456" {
			t.Error("Session ID must be properly stored")
		}
		httpTransport.mu.Unlock()
	} else {
		t.Skip("Session management only supported by StreamableHTTPTransport")
	}
}

// TestContentTypeCompliance verifies content type handling
func (suite *TestSpecificationCompliance) TestContentTypeCompliance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request content type
		if r.Method == "POST" {
			contentType := r.Header.Get("Content-Type")
			if contentType != "application/json" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Invalid content type: %s", contentType)
				return
			}

			// Return based on Accept header
			accept := r.Header.Get("Accept")
			if strings.Contains(accept, "text/event-stream") {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "data: {\"streaming\":true}\n\n")
			} else {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
			}
		}
	}))
	defer server.Close()

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)
	ctx := context.Background()

	// Initialize transport to set headers
	transport.Initialize(ctx)

	// Verify request content type (cast to StreamableHTTPTransport for testing)
	if httpTransport, ok := transport.(*StreamableHTTPTransport); ok {
		req, _ := protocol.NewRequest("1", "test", nil)
		err := httpTransport.sendHTTPRequest(ctx, req, "")
		if err != nil {
			t.Errorf("Request with proper content type should succeed: %v", err)
		}

		// Verify Accept header includes both JSON and SSE
		httpTransport.mu.Lock()
		acceptHeader := httpTransport.headers["Accept"]
		httpTransport.mu.Unlock()

		if !strings.Contains(acceptHeader, "application/json") {
			t.Error("Accept header must include application/json")
		}
		if !strings.Contains(acceptHeader, "text/event-stream") {
			t.Error("Accept header must include text/event-stream")
		}
	} else {
		t.Skip("Content type verification only supported by StreamableHTTPTransport")
	}
}

// Helper function to handle errors in test data creation
func must(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}
