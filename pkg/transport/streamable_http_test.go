package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func TestNewStreamableHTTPTransport(t *testing.T) {
	t.Run("Valid Endpoint", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "http://example.com/mcp"
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		baseTransport, _ := NewTransport(config)

		// Cast to StreamableHTTPTransport to access specific fields
		transport, ok := baseTransport.(*StreamableHTTPTransport)
		if !ok {
			t.Fatal("Expected StreamableHTTPTransport")
		}

		if transport.endpoint != "http://example.com/mcp" {
			t.Errorf("Expected endpoint to be set, got %s", transport.endpoint)
		}

		// eventSources is a sync.Map, we can't check if it's nil but it should be initialized
		// Try to store and load to verify it works
		transport.eventSources.Store("test", "value")
		if _, ok := transport.eventSources.Load("test"); !ok {
			t.Error("Expected eventSources map to be initialized and working")
		}

		if transport.pendingRequests == nil {
			t.Error("Expected pendingRequests map to be initialized")
		}
	})

	t.Run("With Config Options", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "http://example.com/mcp"
		config.Performance.RequestTimeout = 30 * time.Second
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		baseTransport, _ := NewTransport(config)

		// Verify transport was created successfully
		if baseTransport == nil {
			t.Fatal("Expected transport to be created")
		}

		// Configuration validation is handled internally - no need to access internal fields
	})
}

func TestStreamableHTTPTransportInitialize(t *testing.T) {
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "http://example.com/mcp"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)

	ctx := context.Background()
	err := transport.Initialize(ctx)

	if err != nil {
		t.Errorf("Expected Initialize to succeed, got error: %v", err)
	}
}

func TestStreamableHTTPTransportSendRequest(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			// Check session header
			sessionID := r.Header.Get("MCP-Session-ID")

			// Parse request
			var req protocol.Request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// Send response
			resp := &protocol.Response{
				JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
				ID:             req.ID,
				Result:         json.RawMessage(`{"status":"success"}`),
			}

			// Set session ID on initialize
			if req.Method == protocol.MethodInitialize && sessionID == "" {
				w.Header().Set("MCP-Session-ID", "test-session-123")
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	baseTransport, _ := NewTransport(config)

	// Cast to StreamableHTTPTransport to access session methods
	transport, ok := baseTransport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}

	ctx := context.Background()
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize")

	t.Run("Request without session", func(t *testing.T) {
		result, err := transport.SendRequest(ctx, protocol.MethodInitialize, map[string]string{"name": "test"})
		AssertNoError(t, err, "Failed to send request")

		if result == nil {
			t.Fatal("Expected result, got nil")
		}

		// Should have session ID now
		if transport.sessionID == "" {
			t.Error("Expected session ID to be set")
		}
	})

	t.Run("Request with session", func(t *testing.T) {
		// Set session ID
		transport.SetSessionID("test-session-123")

		result, err := transport.SendRequest(ctx, protocol.MethodPing, nil)
		AssertNoError(t, err, "Failed to send request")

		if result == nil {
			t.Fatal("Expected result, got nil")
		}
	})

	t.Run("Request timeout", func(t *testing.T) {
		// Create a slow server
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer slowServer.Close()

		timeoutConfig := DefaultTransportConfig(TransportTypeStreamableHTTP)
		timeoutConfig.Endpoint = slowServer.URL
		timeoutConfig.Performance.RequestTimeout = 50 * time.Millisecond
		timeoutConfig.Features.EnableReliability = false   // Disable for test
		timeoutConfig.Features.EnableObservability = false // Disable for test
		timeoutTransport, _ := NewTransport(timeoutConfig)

		_, err := timeoutTransport.SendRequest(ctx, protocol.MethodPing, nil)
		AssertError(t, err, "Expected timeout error")
	})
}

func TestStreamableHTTPTransportSendNotification(t *testing.T) {
	var notificationReceived bool
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var notif protocol.Notification
			if err := json.NewDecoder(r.Body).Decode(&notif); err == nil {
				mu.Lock()
				notificationReceived = true
				mu.Unlock()
			}
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer server.Close()

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)

	ctx := context.Background()
	err := transport.SendNotification(ctx, protocol.MethodInitialized, map[string]string{"status": "ready"})
	AssertNoError(t, err, "Failed to send notification")

	// Give async operation time to complete
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	received := notificationReceived
	mu.Unlock()

	if !received {
		t.Error("Expected notification to be received")
	}
}

func TestStreamableHTTPTransportStart(t *testing.T) {
	// Create a server that supports SSE
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	sessions := make(map[string]bool)
	var sessionMu sync.Mutex

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			sessionID := r.Header.Get("MCP-Session-ID")
			if sessionID == "" {
				http.Error(w, "Missing session ID", http.StatusBadRequest)
				return
			}

			sessionMu.Lock()
			sessions[sessionID] = true
			sessionMu.Unlock()

			// SSE headers
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming not supported", http.StatusInternalServerError)
				return
			}

			// Send connected event
			fmt.Fprintf(w, "event: connected\ndata: {\"session\":\"%s\"}\n\n", sessionID)
			flusher.Flush()

			// Keep connection open
			<-r.Context().Done()
		}
	})

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	baseTransport, _ := NewTransport(config)

	// Cast to StreamableHTTPTransport to access session methods
	transport, ok := baseTransport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}
	transport.SetSessionID("test-session-123")

	ctx, cancel := context.WithCancel(context.Background())
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize")

	// Start in background
	startDone := make(chan struct{})
	go func() {
		transport.Start(ctx)
		close(startDone)
	}()

	// Give it time to connect
	time.Sleep(100 * time.Millisecond)

	// Check session was connected
	sessionMu.Lock()
	connected := sessions["test-session-123"]
	sessionMu.Unlock()

	if !connected {
		t.Error("Expected session to be connected")
	}

	// Stop
	cancel()

	select {
	case <-startDone:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for Start to finish")
	}
}

func TestStreamableHTTPTransportBatchRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			// Parse request - could be single or batch
			body, _ := io.ReadAll(r.Body)

			// Check if it's a batch request (starts with '[')
			if strings.HasPrefix(string(body), "[") {
				// Handle batch request
				var reqs []protocol.Request
				if err := json.Unmarshal(body, &reqs); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				// Create batch response
				var responses []protocol.Response
				for _, req := range reqs {
					resp := protocol.Response{
						JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
						ID:             req.ID,
						Result:         json.RawMessage(fmt.Sprintf(`{"batch_item":%v}`, req.ID)),
					}
					responses = append(responses, resp)
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(responses)
			} else {
				// Handle single request
				var req protocol.Request
				if err := json.Unmarshal(body, &req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				resp := protocol.Response{
					JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
					ID:             req.ID,
					Result:         json.RawMessage(`{"single":true}`),
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
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
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize")

	// Test batch sending using SendBatch which takes []interface{}
	messages := []interface{}{
		&protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             "batch-1",
			Method:         string(protocol.MethodPing),
		},
		&protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             "batch-2",
			Method:         string(protocol.MethodPing),
		},
	}

	// Cast to StreamableHTTPTransport to access SendBatch method
	httpTransport, ok := transport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}

	err = httpTransport.SendBatch(ctx, messages)
	AssertNoError(t, err, "Failed to send batch")
}

func TestStreamableHTTPTransportSSEHandling(t *testing.T) {
	// Create an SSE endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")

			flusher, _ := w.(http.Flusher)

			// Send different event types
			fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
			flusher.Flush()

			time.Sleep(10 * time.Millisecond)

			// Send a notification
			notif := TestNotification(string(protocol.MethodToolsChanged), map[string]string{"tools": "updated"})
			notifJSON, _ := json.Marshal(notif)
			fmt.Fprintf(w, "data: %s\n\n", string(notifJSON))
			flusher.Flush()

			time.Sleep(10 * time.Millisecond)

			// Send close event
			fmt.Fprintf(w, "event: close\ndata: goodbye\n\n")
			flusher.Flush()
		}
	}))
	defer server.Close()

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	baseTransport, _ := NewTransport(config)

	// Cast to StreamableHTTPTransport to access specific methods
	transport, ok := baseTransport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}

	// Register notification handler
	var notificationReceived bool
	transport.RegisterNotificationHandler(protocol.MethodToolsChanged, func(ctx context.Context, params interface{}) error {
		notificationReceived = true
		return nil
	})

	// Create event source manually using the factory method
	es, err := transport.createEventSource("test-stream", "", context.Background())
	if err != nil {
		t.Fatalf("Failed to create event source: %v", err)
	}

	transport.eventSources.Store("test", es)

	// Connect
	err = es.Connect()
	AssertNoError(t, err, "Failed to connect to SSE")

	// Process events
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		transport.processEventSource(ctx, es)
	}()

	// Wait for notification
	time.Sleep(100 * time.Millisecond)

	if !notificationReceived {
		t.Error("Expected notification to be received")
	}

	cancel()
}

func TestStreamableHTTPTransportErrorHandling(t *testing.T) {
	t.Run("HTTP Error Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = server.URL
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)

		ctx := context.Background()
		_, err := transport.SendRequest(ctx, protocol.MethodPing, nil)
		AssertError(t, err, "Expected error from server")

		if !strings.Contains(err.Error(), "500") {
			t.Errorf("Expected 500 error, got: %v", err)
		}
	})

	t.Run("Invalid JSON Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = server.URL
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)

		ctx := context.Background()
		_, err := transport.SendRequest(ctx, protocol.MethodPing, nil)
		AssertError(t, err, "Expected JSON parse error")
	})

	t.Run("Network Error", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "http://localhost:0"      // Invalid port
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)

		ctx := context.Background()
		_, err := transport.SendRequest(ctx, protocol.MethodPing, nil)
		AssertError(t, err, "Expected network error")
	})
}

func TestStreamableHTTPTransportSessionManagement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			sessionID := r.Header.Get("MCP-Session-ID")

			var req protocol.Request
			json.NewDecoder(r.Body).Decode(&req)

			// Initialize creates new session
			if req.Method == protocol.MethodInitialize && sessionID == "" {
				w.Header().Set("MCP-Session-ID", "new-session-456")
			}

			resp := TestResponse(fmt.Sprintf("%v", req.ID), map[string]string{"session": sessionID})
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	t.Run("Session Creation", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = server.URL
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		baseTransport, _ := NewTransport(config)

		// Cast to StreamableHTTPTransport to access session fields
		transport, ok := baseTransport.(*StreamableHTTPTransport)
		if !ok {
			t.Fatal("Expected StreamableHTTPTransport")
		}

		ctx := context.Background()
		_, err := transport.SendRequest(ctx, protocol.MethodInitialize, nil)
		AssertNoError(t, err, "Failed to initialize")

		if transport.sessionID != "new-session-456" {
			t.Errorf("Expected session ID 'new-session-456', got %s", transport.sessionID)
		}
	})

	t.Run("Session Reuse", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = server.URL
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		baseTransport, _ := NewTransport(config)

		// Cast to StreamableHTTPTransport to access session methods
		transport, ok := baseTransport.(*StreamableHTTPTransport)
		if !ok {
			t.Fatal("Expected StreamableHTTPTransport")
		}
		transport.SetSessionID("existing-session")

		ctx := context.Background()
		result, err := transport.SendRequest(ctx, protocol.MethodPing, nil)
		AssertNoError(t, err, "Failed to send request")

		// Verify session was maintained
		if transport.sessionID != "existing-session" {
			t.Error("Expected session ID to be maintained")
		}

		// Check result contains correct session
		resultJSON, _ := json.Marshal(result)
		if !strings.Contains(string(resultJSON), "existing-session") {
			t.Error("Expected response to reflect existing session")
		}
	})
}

func TestStreamableHTTPTransportConcurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate processing delay
		time.Sleep(10 * time.Millisecond)

		var req protocol.Request
		json.NewDecoder(r.Body).Decode(&req)

		resp := TestResponse(fmt.Sprintf("%v", req.ID), map[string]string{"processed": "true"})
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)

	ctx := context.Background()
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize")

	// Send multiple concurrent requests
	numRequests := 20
	results := make(chan error, numRequests)

	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			_, err := transport.SendRequest(ctx, protocol.MethodPing, map[string]int{"id": n})
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	// Check all requests succeeded
	errorCount := 0
	for err := range results {
		if err != nil {
			errorCount++
			t.Errorf("Request failed: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("%d out of %d requests failed", errorCount, numRequests)
	}
}

func TestStreamableEventSource(t *testing.T) {
	t.Run("Connect and Receive", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)

			fmt.Fprintf(w, "data: {\"message\":\"hello\"}\n\n")
			flusher.Flush()

			time.Sleep(10 * time.Millisecond)
			fmt.Fprintf(w, "data: {\"message\":\"world\"}\n\n")
			flusher.Flush()

			// Keep open briefly
			time.Sleep(50 * time.Millisecond)
		}))
		defer server.Close()

		es := &StreamableEventSource{
			URL:         server.URL,
			Headers:     make(map[string]string),
			Client:      &http.Client{},
			MessageChan: make(chan []byte, 100),
			ErrorChan:   make(chan error, 10),
			CloseChan:   make(chan struct{}),
		}

		err := es.Connect()
		AssertNoError(t, err, "Failed to connect")

		// Read messages
		messages := make([]string, 0)
		timeout := time.After(200 * time.Millisecond)

	loop:
		for {
			select {
			case msg := <-es.MessageChan:
				messages = append(messages, string(msg))
				if len(messages) >= 2 {
					break loop
				}
			case <-timeout:
				break loop
			}
		}

		if len(messages) < 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}

		es.Close()
	})

	t.Run("Error Handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not found", http.StatusNotFound)
		}))
		defer server.Close()

		es := &StreamableEventSource{
			URL:         server.URL,
			Headers:     make(map[string]string),
			Client:      &http.Client{},
			MessageChan: make(chan []byte, 100),
			ErrorChan:   make(chan error, 10),
			CloseChan:   make(chan struct{}),
		}

		err := es.Connect()
		AssertError(t, err, "Expected connection error")
	})
}

func TestStreamableHTTPTransportProgressHandling(t *testing.T) {
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "http://example.com"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)

	// Register progress handler
	progressReceived := false
	transport.RegisterProgressHandler("op-123", func(params interface{}) error {
		progressReceived = true
		return nil
	})

	// Create a mock progress notification
	notif := &protocol.Notification{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		Method:         protocol.MethodProgress,
		Params:         json.RawMessage(`{"id":"op-123","progress":50}`),
	}

	// Process the notification
	ctx := context.Background()
	err := transport.HandleNotification(ctx, notif)
	AssertNoError(t, err, "Failed to handle progress notification")

	if !progressReceived {
		t.Error("Expected progress handler to be called")
	}

	// Unregister and verify it's not called
	transport.UnregisterProgressHandler("op-123")
	progressReceived = false

	err = transport.HandleNotification(ctx, notif)
	AssertNoError(t, err, "Failed to handle notification after unregister")

	if progressReceived {
		t.Error("Expected progress handler NOT to be called after unregister")
	}
}

func TestStreamableHTTPTransportEdgeCases(t *testing.T) {
	t.Run("Multiple Event Sources", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "http://example.com"
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		baseTransport, _ := NewTransport(config)

		// Cast to StreamableHTTPTransport to access eventSources
		transport, ok := baseTransport.(*StreamableHTTPTransport)
		if !ok {
			t.Fatal("Expected StreamableHTTPTransport")
		}

		// Store multiple event sources
		for i := 0; i < 3; i++ {
			transport.eventSources.Store(fmt.Sprintf("session-%d", i), fmt.Sprintf("es-%d", i))
		}

		// Verify all sessions exist
		count := 0
		transport.eventSources.Range(func(key, value interface{}) bool {
			count++
			return true
		})

		if count != 3 {
			t.Errorf("Expected 3 event sources, got %d", count)
		}
	})

	t.Run("Stop Without Start", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "http://example.com"
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)

		ctx := context.Background()
		err := transport.Stop(ctx)
		// Should not error
		if err != nil {
			t.Errorf("Expected Stop without Start to succeed, got: %v", err)
		}
	})

	t.Run("Double Initialize", func(t *testing.T) {
		config := DefaultTransportConfig(TransportTypeStreamableHTTP)
		config.Endpoint = "http://example.com"
		config.Features.EnableReliability = false   // Disable for test
		config.Features.EnableObservability = false // Disable for test
		transport, _ := NewTransport(config)

		ctx := context.Background()
		err := transport.Initialize(ctx)
		AssertNoError(t, err, "First initialize failed")

		err = transport.Initialize(ctx)
		AssertNoError(t, err, "Second initialize failed")
	})
}

// Test the createEventSource method if it's exposed
func TestStreamableHTTPTransportCreateEventSource(t *testing.T) {
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "http://example.com"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	baseTransport, _ := NewTransport(config)

	// Cast to StreamableHTTPTransport to access specific methods
	transport, ok := baseTransport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}
	transport.SetSessionID("test-session")
	transport.SetHeader("X-Custom", "value")

	ctx := context.Background()
	es, err := transport.createEventSource("test-stream", "last-event-123", ctx)

	AssertNoError(t, err, "Failed to create event source")

	if es == nil {
		t.Fatal("Expected event source to be created")
	}

	// Verify headers were set
	if es.Headers["MCP-Session-ID"] != "test-session" {
		t.Error("Expected session ID header to be set")
	}

	if es.Headers["X-Custom"] != "value" {
		t.Error("Expected custom header to be set")
	}

	if es.Headers["Last-Event-ID"] != "last-event-123" {
		t.Error("Expected Last-Event-ID header to be set")
	}
}
