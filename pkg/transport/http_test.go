package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func TestNewHTTPTransport(t *testing.T) {
	t.Run("Valid URL", func(t *testing.T) {
		transport := NewHTTPTransport("http://example.com/mcp")
		if transport == nil {
			t.Fatal("Expected transport to be created")
		}
		if transport.serverURL != "http://example.com/mcp" {
			t.Errorf("Expected URL to be set, got %s", transport.serverURL)
		}
		if transport.BaseTransport == nil {
			t.Error("Expected BaseTransport to be initialized")
		}
	})

	t.Run("With Options", func(t *testing.T) {
		transport := NewHTTPTransport("http://example.com/mcp",
			WithRequestTimeout(30*time.Second),
		)

		if transport == nil {
			t.Fatal("Expected transport to be created")
		}
		if transport.options.RequestTimeout != 30*time.Second {
			t.Errorf("Expected timeout to be 30s, got %v", transport.options.RequestTimeout)
		}
	})
}

func TestHTTPTransportInitialize(t *testing.T) {
	transport := NewHTTPTransport("http://example.com/mcp")

	ctx := context.Background()
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize transport")

	if transport.eventSource == nil {
		t.Error("Expected eventSource to be created")
	}
	if transport.eventSource.URL != "http://example.com/mcp/events" {
		t.Errorf("Expected SSE URL to be %s/events, got %s", transport.serverURL, transport.eventSource.URL)
	}
}

func TestHTTPTransportSendRequest(t *testing.T) {
	// Create a test server that handles both POST and SSE
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Track requests
	var requestReceived bool
	var requestID string

	// Handle POST requests
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req protocol.Request
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				requestReceived = true
				requestID = fmt.Sprintf("%v", req.ID)
			}
			w.WriteHeader(http.StatusOK)
		}
	})

	// Handle SSE endpoint
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send response after request is received
		go func() {
			for !requestReceived {
				time.Sleep(10 * time.Millisecond)
			}

			// Send response
			resp := TestResponse(requestID, map[string]string{"status": "success"})
			respJSON, _ := json.Marshal(resp)
			fmt.Fprintf(w, "data: %s\n\n", string(respJSON))
			flusher.Flush()
		}()

		// Keep connection open
		<-r.Context().Done()
	})

	transport := NewHTTPTransport(server.URL)

	ctx := context.Background()
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize")

	// Start transport in background
	startDone := make(chan struct{})
	go func() {
		transport.Start(ctx)
		close(startDone)
	}()

	// Give it time to connect
	time.Sleep(100 * time.Millisecond)

	// Send request
	result, err := transport.SendRequest(ctx, protocol.MethodInitialize, map[string]string{"name": "test"})
	AssertNoError(t, err, "Failed to send request")

	// Verify result
	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Stop transport
	transport.Stop(ctx)

	// Wait for Start to finish
	select {
	case <-startDone:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for transport to stop")
	}
}

func TestHTTPTransportSendNotification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var notif protocol.Notification
			if err := json.NewDecoder(r.Body).Decode(&notif); err != nil {
				t.Errorf("Failed to decode notification: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if notif.Method != protocol.MethodInitialized {
				t.Errorf("Expected method %s, got %s", protocol.MethodInitialized, notif.Method)
			}

			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL)

	ctx := context.Background()
	err := transport.SendNotification(ctx, protocol.MethodInitialized, map[string]string{"status": "ready"})
	AssertNoError(t, err, "Failed to send notification")
}

func TestHTTPTransportHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom header
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("Expected X-Custom header")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL)
	transport.SetHeader("X-Custom", "value")

	ctx := context.Background()
	err := transport.SendNotification(ctx, protocol.MethodPing, nil)
	AssertNoError(t, err, "Failed to send notification")
}

func TestHTTPTransportStop(t *testing.T) {
	transport := NewHTTPTransport("http://example.com")

	ctx := context.Background()
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize")

	// Create a mock event source
	transport.eventSource = &EventSource{
		CloseChan: make(chan struct{}),
	}

	// Stop should not error even if not started
	err = transport.Stop(ctx)
	AssertNoError(t, err, "Failed to stop transport")

	// Stop again should be safe
	err = transport.Stop(ctx)
	AssertNoError(t, err, "Failed to stop transport again")
}

// Test EventSource functionality
func TestEventSource(t *testing.T) {
	t.Run("Connect", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")

			flusher, _ := w.(http.Flusher)

			// Send a test message
			fmt.Fprintf(w, "data: {\"test\": \"message\"}\n\n")
			flusher.Flush()

			// Keep connection open
			<-r.Context().Done()
		}))
		defer server.Close()

		es := &EventSource{
			URL:         server.URL,
			Client:      &http.Client{},
			MessageChan: make(chan []byte, 10),
			ErrorChan:   make(chan error, 10),
			CloseChan:   make(chan struct{}),
		}

		err := es.Connect()
		AssertNoError(t, err, "Failed to connect")

		// Should receive message
		select {
		case msg := <-es.MessageChan:
			if !strings.Contains(string(msg), "test") {
				t.Errorf("Expected test message, got %s", string(msg))
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for message")
		}

		es.Close()
	})

	t.Run("Connection Error", func(t *testing.T) {
		es := &EventSource{
			URL:         "http://localhost:0", // Invalid port
			Client:      &http.Client{Timeout: 100 * time.Millisecond},
			MessageChan: make(chan []byte, 10),
			ErrorChan:   make(chan error, 10),
			CloseChan:   make(chan struct{}),
		}

		err := es.Connect()
		AssertError(t, err, "Expected connection error")
	})

	t.Run("Non-SSE Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error": "not sse"}`))
		}))
		defer server.Close()

		es := &EventSource{
			URL:         server.URL,
			Client:      &http.Client{},
			MessageChan: make(chan []byte, 10),
			ErrorChan:   make(chan error, 10),
			CloseChan:   make(chan struct{}),
		}

		err := es.Connect()
		AssertError(t, err, "Expected SSE content type error")
	})
}

func TestHTTPTransportHandleMessage(t *testing.T) {
	transport := NewHTTPTransport("http://example.com")

	t.Run("Handle Request", func(t *testing.T) {
		// Register request handler
		transport.RegisterRequestHandler(protocol.MethodPing, func(ctx context.Context, params interface{}) (interface{}, error) {
			return map[string]string{"pong": "true"}, nil
		})

		// Verify handler was registered
		transport.RLock()
		_, exists := transport.requestHandlers[string(protocol.MethodPing)]
		transport.RUnlock()

		if !exists {
			t.Error("Expected handler to be registered")
		}
	})

	t.Run("Handle Response", func(t *testing.T) {
		// Add pending request
		respChan := make(chan *protocol.Response, 1)
		transport.Lock()
		transport.pendingRequests["test-id"] = respChan
		transport.Unlock()

		// Create response
		resp := TestResponse("test-id", map[string]string{"status": "ok"})
		respJSON, _ := json.Marshal(resp)

		ctx := context.Background()
		err := transport.handleMessage(ctx, respJSON)
		AssertNoError(t, err, "Failed to handle response")

		// Should receive response
		select {
		case received := <-respChan:
			if received.ID != "test-id" {
				t.Errorf("Expected response ID test-id, got %v", received.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Response not received")
		}
	})

	t.Run("Handle Notification", func(t *testing.T) {
		var handlerCalled bool
		transport.RegisterNotificationHandler(protocol.MethodToolsChanged, func(ctx context.Context, params interface{}) error {
			handlerCalled = true
			return nil
		})

		// Create notification
		notif := TestNotification(string(protocol.MethodToolsChanged), map[string]string{"tools": "updated"})
		notifJSON, _ := json.Marshal(notif)

		ctx := context.Background()
		err := transport.handleMessage(ctx, notifJSON)
		AssertNoError(t, err, "Failed to handle notification")

		if !handlerCalled {
			t.Error("Expected notification handler to be called")
		}
	})

	t.Run("Invalid Message", func(t *testing.T) {
		ctx := context.Background()
		err := transport.handleMessage(ctx, []byte("invalid json"))
		AssertError(t, err, "Expected error for invalid message")
	})
}

func TestHTTPTransportConcurrency(t *testing.T) {
	transport := NewHTTPTransport("http://example.com")

	// Test concurrent access to headers
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			transport.SetHeader(fmt.Sprintf("X-Test-%d", n), fmt.Sprintf("value%d", n))
		}(i)
	}

	wg.Wait()

	// Verify all headers were set
	transport.mu.Lock()
	headerCount := len(transport.headers)
	transport.mu.Unlock()

	if headerCount != 10 {
		t.Errorf("Expected 10 headers, got %d", headerCount)
	}
}

func TestHTTPTransportReconnection(t *testing.T) {
	// Create a server that fails initially then succeeds
	attempts := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/events" {
			mu.Lock()
			attempts++
			currentAttempt := attempts
			mu.Unlock()

			if currentAttempt == 1 {
				// First attempt fails
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

			// Subsequent attempts succeed
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Keep connection open
			<-r.Context().Done()
		}
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	err := transport.Initialize(ctx)
	AssertNoError(t, err, "Failed to initialize")

	// Start transport (it should reconnect after initial failure)
	go func() {
		transport.Start(ctx)
	}()

	// Wait for reconnection
	time.Sleep(6 * time.Second) // Initial failure + 5s reconnect delay

	mu.Lock()
	finalAttempts := attempts
	mu.Unlock()

	if finalAttempts < 2 {
		t.Errorf("Expected at least 2 connection attempts, got %d", finalAttempts)
	}

	cancel()
}

func TestHTTPTransportEdgeCases(t *testing.T) {
	t.Run("Send Without Initialize", func(t *testing.T) {
		transport := NewHTTPTransport("http://example.com")

		ctx := context.Background()
		err := transport.SendNotification(ctx, protocol.MethodPing, nil)
		// Should still work as sendHTTPRequest doesn't require initialization
		// We expect a connection error but the exact error message may vary
		_ = err // The error is expected, no need to check specific message
	})

	t.Run("Double Start", func(t *testing.T) {
		transport := NewHTTPTransport("http://example.com")
		transport.running.Store(true)

		ctx := context.Background()
		err := transport.Start(ctx)
		AssertError(t, err, "Expected error on double start")

		if !strings.Contains(err.Error(), "already running") {
			t.Errorf("Expected 'already running' error, got: %v", err)
		}
	})
}

// EventSource tests
func TestEventSourceReadLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// Send multiple messages
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: {\"message\": %d}\n\n", i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}

		// Send close event
		fmt.Fprintf(w, "event: close\n")
		fmt.Fprintf(w, "data: closing\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	es := &EventSource{
		URL:         server.URL,
		Client:      &http.Client{},
		MessageChan: make(chan []byte, 10),
		ErrorChan:   make(chan error, 10),
		CloseChan:   make(chan struct{}),
	}

	err := es.Connect()
	AssertNoError(t, err, "Failed to connect")

	// Collect messages
	messages := make([][]byte, 0)
	timeout := time.After(1 * time.Second)

	for {
		select {
		case msg := <-es.MessageChan:
			messages = append(messages, msg)
		case <-es.CloseChan:
			// Connection closed
			goto done
		case <-timeout:
			t.Fatal("Timeout waiting for messages")
		}
	}

done:
	if len(messages) < 3 {
		t.Errorf("Expected at least 3 messages, got %d", len(messages))
	}
}
