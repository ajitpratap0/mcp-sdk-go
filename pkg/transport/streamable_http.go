package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/model-context-protocol/go-mcp/pkg/protocol"
)

// StreamableHTTPTransport implements Transport using the Streamable HTTP protocol
// which provides advanced features over the basic HTTP+SSE transport like
// resumability, session management, and multiple connection support.
type StreamableHTTPTransport struct {
	*BaseTransport
	endpoint        string
	client          *http.Client
	eventSources    sync.Map // map[string]*StreamableEventSource - multiple connections support
	running         atomic.Bool
	options         *Options
	headers         map[string]string
	requestIDPrefix string
	sessionID       string
	lastEventID     string
	mu              sync.Mutex
}

// StreamableEventSource is an enhanced client for Server-Sent Events
// with support for resumability and event IDs
type StreamableEventSource struct {
	URL         string
	Headers     map[string]string
	Client      *http.Client
	Connection  *http.Response
	MessageChan chan []byte
	ErrorChan   chan error
	CloseChan   chan struct{}
	LastEventID string
	StreamID    string // Unique identifier for this stream
	mu          sync.Mutex
	isConnected atomic.Bool
}

// NewStreamableHTTPTransport creates a new Streamable HTTP transport
func NewStreamableHTTPTransport(endpoint string, options ...Option) *StreamableHTTPTransport {
	opts := NewOptions(options...)

	client := &http.Client{
		Timeout: opts.RequestTimeout,
	}

	return &StreamableHTTPTransport{
		BaseTransport:   NewBaseTransport(),
		endpoint:        endpoint,
		client:          client,
		options:         opts,
		headers:         make(map[string]string),
		requestIDPrefix: "streamable-http",
	}
}

// SetRequestIDPrefix sets the prefix for request IDs
func (t *StreamableHTTPTransport) SetRequestIDPrefix(prefix string) {
	t.requestIDPrefix = prefix
}

// SetHeader sets a HTTP header for all requests
func (t *StreamableHTTPTransport) SetHeader(key, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.headers[key] = value
}

// SetSessionID sets the session ID for this transport
func (t *StreamableHTTPTransport) SetSessionID(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionID = sessionID
}

// Initialize sets up the Streamable HTTP transport
func (t *StreamableHTTPTransport) Initialize(ctx context.Context) error {
	// Set default headers
	t.SetHeader("Accept", "application/json, text/event-stream")

	// Create a listener connection
	go func() {
		err := t.openListenerConnection(ctx)
		if err != nil {
			// Just log the error, don't fail initialization if the server doesn't support GET
			// as this is optional according to the spec
			fmt.Printf("Failed to open listener connection: %v\n", err)
		}
	}()

	return nil
}

// openListenerConnection opens a GET connection to listen for server-initiated messages
func (t *StreamableHTTPTransport) openListenerConnection(ctx context.Context) error {
	streamID := fmt.Sprintf("listener-%d", time.Now().UnixNano())

	es, err := t.createEventSource(streamID)
	if err != nil {
		return err
	}

	t.eventSources.Store(streamID, es)

	go t.processEventSource(ctx, es)

	return nil
}

// createEventSource sets up a new SSE connection
func (t *StreamableHTTPTransport) createEventSource(streamID string) (*StreamableEventSource, error) {
	req, err := http.NewRequest("GET", t.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// Add session ID if available
	if t.sessionID != "" {
		req.Header.Set("MCP-Session-ID", t.sessionID)
	}

	// Add Last-Event-ID if resuming
	if t.lastEventID != "" {
		req.Header.Set("Last-Event-ID", t.lastEventID)
	}

	// Create the event source
	es := &StreamableEventSource{
		URL:         t.endpoint,
		Headers:     t.headers,
		Client:      t.client,
		MessageChan: make(chan []byte, 100),
		ErrorChan:   make(chan error, 10),
		CloseChan:   make(chan struct{}),
		StreamID:    streamID,
		LastEventID: t.lastEventID,
	}

	// Connect to the event source
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to event source: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			return nil, fmt.Errorf("server does not support GET for SSE (405 Method Not Allowed)")
		}
		return nil, fmt.Errorf("failed to connect to event source: HTTP %d", resp.StatusCode)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		resp.Body.Close()
		return nil, fmt.Errorf("server did not return text/event-stream content type")
	}

	es.Connection = resp
	es.isConnected.Store(true)

	// Start reading events
	go es.readEvents()

	return es, nil
}

// SendRequest sends a request and waits for the response
func (t *StreamableHTTPTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	t.mu.Lock()
	id := fmt.Sprintf("%s-%d", t.requestIDPrefix, t.GetNextID())
	t.mu.Unlock()

	req, err := protocol.NewRequest(id, method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create a stream ID for this request
	streamID := fmt.Sprintf("request-%s", id)

	// Create wait context with timeout
	waitCtx, cancel := context.WithTimeout(ctx, t.options.RequestTimeout)
	defer cancel()

	// Debug logging for outgoing requests
	fmt.Printf("[DEBUG] Sending request: method=%s id=%v\n", method, id)

	// Send the request via HTTP POST
	err = t.sendHTTPRequest(req, streamID)
	if err != nil {
		fmt.Printf("[ERROR] Failed to send HTTP request: %v\n", err)
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}

	// Debug log successful request
	fmt.Printf("[DEBUG] Request sent successfully, waiting for response to id=%v\n", id)

	// Wait for the response
	resp, err := t.WaitForResponse(waitCtx, id)
	if err != nil {
		return nil, fmt.Errorf("waiting for response: %w", err)
	}

	// Handle error response
	if resp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	// Parse the result
	var result interface{}
	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return result, nil
}

// SendNotification sends a notification (one-way message)
func (t *StreamableHTTPTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	notification, err := protocol.NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	// Send the notification via HTTP POST
	// No need to create a stream for a notification as we don't expect a response
	err = t.sendHTTPRequest(notification, "")
	if err != nil {
		return fmt.Errorf("failed to send HTTP notification: %w", err)
	}

	return nil
}

// Start begins processing messages (blocking)
func (t *StreamableHTTPTransport) Start(ctx context.Context) error {
	if !t.running.CompareAndSwap(false, true) {
		return fmt.Errorf("transport already running")
	}

	// We already started a listener connection in Initialize, so now we just wait
	<-ctx.Done()

	return ctx.Err()
}

// Stop gracefully shuts down the transport
func (t *StreamableHTTPTransport) Stop(ctx context.Context) error {
	if !t.running.CompareAndSwap(true, false) {
		return nil // Already stopped
	}

	// Close all event sources
	t.eventSources.Range(func(key, value interface{}) bool {
		if es, ok := value.(*StreamableEventSource); ok {
			es.Close()
		}
		t.eventSources.Delete(key)
		return true
	})

	return nil
}

// SendBatch sends a batch of JSON-RPC messages
func (t *StreamableHTTPTransport) SendBatch(ctx context.Context, messages []interface{}) error {
	// Send the batch via HTTP POST
	// No stream ID needed since we're not expecting specific stream responses for batches
	return t.sendHTTPRequest(messages, "")
}

// sendHTTPRequest sends a JSON-RPC message or batch via HTTP POST
func (t *StreamableHTTPTransport) sendHTTPRequest(message interface{}, streamID string) error {
	// Marshal the message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create a new request
	req, err := http.NewRequest("POST", t.endpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// Add session ID if available
	if t.sessionID != "" {
		req.Header.Set("MCP-Session-ID", t.sessionID)
	}

	// Add stream ID if available, to associate with a specific stream
	if streamID != "" {
		req.Header.Set("MCP-Stream-ID", streamID)
	}

	// Send the request
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Handle response/notification
	// Check if this is a single JSON-RPC request or a batch
	isRequest := protocol.IsRequest(data)
	isBatch := len(data) > 0 && data[0] == '['

	if isRequest || isBatch {
		// We expect a response for requests or batches containing requests
		// Check content type
		contentType := resp.Header.Get("Content-Type")

		// Check for session ID in response headers
		if sessionID := resp.Header.Get("MCP-Session-ID"); sessionID != "" && t.sessionID == "" {
			// Store session ID for future requests
			t.SetSessionID(sessionID)
		}

		if strings.Contains(contentType, "text/event-stream") {
			// Server returned an SSE stream
			// Create a new event source for this request if not already created
			if streamID != "" {
				if _, ok := t.eventSources.Load(streamID); !ok {
					// Store the Last-Event-ID from response headers if provided
					lastEventID := resp.Header.Get("Last-Event-ID")

					es := &StreamableEventSource{
						URL:         t.endpoint,
						Headers:     t.headers,
						Client:      t.client,
						Connection:  resp,
						MessageChan: make(chan []byte, 100),
						ErrorChan:   make(chan error, 10),
						CloseChan:   make(chan struct{}),
						StreamID:    streamID,
						LastEventID: lastEventID,
					}

					es.isConnected.Store(true)

					// Start reading events from this stream
					go es.readEvents()

					// Store the event source
					t.eventSources.Store(streamID, es)

					// Process messages from this stream
					go t.processEventSource(context.Background(), es)
				}
			}
		} else if strings.Contains(contentType, "application/json") {
			// Server returned a direct JSON response
			// Parse the response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}

			if len(body) > 0 {
				// Process the response
				t.handleMessage(context.Background(), body)
			}
		}
	}

	return nil
}

// processEventSource processes messages from an event source
func (t *StreamableHTTPTransport) processEventSource(ctx context.Context, es *StreamableEventSource) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-es.MessageChan:
			if len(data) > 0 {
				// Process the message
				err := t.handleMessage(ctx, data)
				if err != nil {
					fmt.Printf("Error processing message: %v\n", err)
				}
			}
		case err := <-es.ErrorChan:
			fmt.Printf("Event source error: %v\n", err)
			// If the connection is closed, try to reconnect
			if !es.isConnected.Load() {
				// Remove this event source
				t.eventSources.Delete(es.StreamID)

				// If this was the listener connection, try to reconnect
				if strings.HasPrefix(es.StreamID, "listener") {
					time.Sleep(1 * time.Second) // Wait before retrying
					err := t.openListenerConnection(ctx)
					if err != nil {
						fmt.Printf("Failed to reopen listener connection: %v\n", err)
					}
				}

				return
			}
		case <-es.CloseChan:
			// Connection closed, remove this event source
			t.eventSources.Delete(es.StreamID)
			return
		}
	}
}

// handleMessage processes an incoming JSON-RPC message, which could be a single message or batch
func (t *StreamableHTTPTransport) handleMessage(ctx context.Context, data []byte) error {
	// Check for empty or whitespace-only messages
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil // Just ignore empty messages
	}

	// Check if it's an empty JSON object {}
	if string(trimmed) == "{}" {
		// This is likely a heartbeat or initialization message, just ignore it
		return nil
	}

	// Check if this is a batch (JSON array)
	if len(data) > 0 && data[0] == '[' {
		return t.handleBatchMessage(ctx, data)
	}

	// Process single message
	// Determine message type
	if protocol.IsRequest(data) {
		// Parse as a request
		var req protocol.Request
		if err := json.Unmarshal(data, &req); err != nil {
			return fmt.Errorf("failed to unmarshal request: %w", err)
		}

		// Handle the request
		resp, err := t.HandleRequest(ctx, &req)
		if err != nil {
			return fmt.Errorf("failed to handle request: %w", err)
		}

		// Send the response if not nil
		if resp != nil {
			err = t.sendHTTPRequest(resp, "")
			if err != nil {
				return fmt.Errorf("failed to send response: %w", err)
			}
		}

		return nil
	} else if protocol.IsResponse(data) {
		// Parse as a response
		var resp protocol.Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Handle the response
		t.HandleResponse(&resp)

		return nil
	} else if protocol.IsNotification(data) {
		// Parse as a notification
		var notif protocol.Notification
		if err := json.Unmarshal(data, &notif); err != nil {
			return fmt.Errorf("failed to unmarshal notification: %w", err)
		}

		// Handle the notification
		err := t.HandleNotification(ctx, &notif)
		if err != nil {
			return fmt.Errorf("failed to handle notification: %w", err)
		}

		return nil
	}

	// Log a sample of the message for debugging
	maxLen := 100
	sampleData := string(data)
	if len(sampleData) > maxLen {
		sampleData = sampleData[:maxLen] + "..."
	}

	// Debug log - this will help diagnose issues
	fmt.Printf("[DEBUG] Received unrecognized message: %s\n", sampleData)

	// Check if it's at least valid JSON
	var anyJSON interface{}
	jsonErr := json.Unmarshal(data, &anyJSON)

	if jsonErr != nil {
		return fmt.Errorf("invalid JSON message: %w, data: %s", jsonErr, sampleData)
	} else {
		// It's valid JSON but not a recognized JSON-RPC message type
		// Since we're already handling empty objects above, this must be another format
		return fmt.Errorf("unknown message type: %s", sampleData)
	}
}

// handleBatchMessage processes a batch of JSON-RPC messages
func (t *StreamableHTTPTransport) handleBatchMessage(ctx context.Context, data []byte) error {
	// Check what kind of batch this is (could be a batch of requests, responses, or notifications)
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(data, &rawMessages); err != nil {
		return fmt.Errorf("failed to unmarshal batch: %w", err)
	}

	// Process each message in the batch
	var responsesBatch []interface{}

	for _, rawMsg := range rawMessages {
		if protocol.IsRequest(rawMsg) {
			// Parse as a request
			var req protocol.Request
			if err := json.Unmarshal(rawMsg, &req); err != nil {
				continue // Skip invalid messages in batch
			}

			// Handle the request
			resp, err := t.HandleRequest(ctx, &req)
			if err != nil {
				// Create error response
				errResp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
				responsesBatch = append(responsesBatch, errResp)
			} else if resp != nil {
				// Add to batch of responses
				responsesBatch = append(responsesBatch, resp)
			}
		} else if protocol.IsResponse(rawMsg) {
			// Parse as a response
			var resp protocol.Response
			if err := json.Unmarshal(rawMsg, &resp); err != nil {
				continue // Skip invalid messages
			}

			// Handle the response
			t.HandleResponse(&resp)
		} else if protocol.IsNotification(rawMsg) {
			// Parse as a notification
			var notif protocol.Notification
			if err := json.Unmarshal(rawMsg, &notif); err != nil {
				continue // Skip invalid messages
			}

			// Handle the notification
			_ = t.HandleNotification(ctx, &notif) // Ignore errors for notifications in batch
		}
	}

	// Send batch of responses if we have any
	if len(responsesBatch) > 0 {
		err := t.sendHTTPRequest(responsesBatch, "")
		if err != nil {
			return fmt.Errorf("failed to send batch responses: %w", err)
		}
	}

	return nil
}

// Connect establishes a connection to the SSE endpoint
func (es *StreamableEventSource) Connect() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.isConnected.Load() {
		return nil // Already connected
	}

	// Create request
	req, err := http.NewRequest("GET", es.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range es.Headers {
		req.Header.Set(k, v)
	}

	// Add Last-Event-ID if resuming
	if es.LastEventID != "" {
		req.Header.Set("Last-Event-ID", es.LastEventID)
	}

	// Send request
	resp, err := es.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Check response
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("failed to connect: HTTP %d", resp.StatusCode)
	}

	es.Connection = resp
	es.isConnected.Store(true)

	// Start reading events
	go es.readEvents()

	return nil
}

// Close terminates the SSE connection
func (es *StreamableEventSource) Close() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.isConnected.Load() {
		return // Already closed
	}

	if es.Connection != nil && es.Connection.Body != nil {
		es.Connection.Body.Close()
	}

	es.isConnected.Store(false)
	close(es.CloseChan)
}

// readEvents processes the SSE stream
func (es *StreamableEventSource) readEvents() {
	defer func() {
		es.isConnected.Store(false)

		if es.Connection != nil && es.Connection.Body != nil {
			es.Connection.Body.Close()
		}

		// Notify of disconnection
		es.ErrorChan <- fmt.Errorf("disconnected")
	}()

	scanner := bufio.NewScanner(es.Connection.Body)
	eventData := ""
	eventID := ""
	eventType := ""

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comment lines (used as heartbeats)
		if strings.HasPrefix(line, ":") {
			continue
		}

		if line == "" {
			// End of event, process it if we have data
			if eventData != "" {
				// If we have an ID, update the last event ID
				if eventID != "" {
					es.LastEventID = eventID
				}

				// Check for close event type
				if eventType == "close" {
					// Server signaled to close the connection
					es.Close()
					return
				}

				// Send the event data
				es.MessageChan <- []byte(eventData)

				// Reset for next event
				eventData = ""
				eventID = ""
				eventType = ""
			}
			continue
		}

		// Process the line
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if eventData == "" {
				eventData = data
			} else {
				eventData += "\n" + data
			}
		} else if strings.HasPrefix(line, "id:") {
			eventID = strings.TrimPrefix(line, "id:")
			eventID = strings.TrimSpace(eventID)
		} else if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimPrefix(line, "event:")
			eventType = strings.TrimSpace(eventType)
		} else if strings.HasPrefix(line, "retry:") {
			// Handle retry directive - could be used to set reconnection time
			// Currently we don't use this, but we could add a configurable retry timer
		}
	}

	if scanner.Err() != nil {
		es.ErrorChan <- scanner.Err()
	}
}
