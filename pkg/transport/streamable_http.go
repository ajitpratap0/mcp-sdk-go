package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// StreamableHTTPTransport implements Transport using the Streamable HTTP protocol
// which provides advanced features over the basic HTTP+SSE transport like
// resumability, session management, and multiple connection support.
type StreamableHTTPTransport struct {
	*BaseTransport
	endpoint         string
	client           *http.Client
	eventSources     sync.Map // map[string]*StreamableEventSource - multiple connections support
	running          atomic.Bool
	options          *Options
	headers          map[string]string
	requestIDPrefix  string
	sessionID        string
	mu               sync.Mutex
	progressHandlers map[string]ProgressHandler
	responseHandlers map[string]func(*protocol.Response)
	pendingRequests  map[string]chan *protocol.Response
	logger           *log.Logger
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
		BaseTransport:    NewBaseTransport(),
		endpoint:         endpoint,
		client:           client,
		options:          opts,
		headers:          make(map[string]string),
		requestIDPrefix:  "streamable-http",
		pendingRequests:  make(map[string]chan *protocol.Response),
		progressHandlers: make(map[string]ProgressHandler),
		responseHandlers: make(map[string]func(*protocol.Response)),
		logger:           log.New(os.Stdout, "StreamableHTTPTransport: ", log.LstdFlags),
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

	// Create a context with timeout for initialization
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Set up session ID tracking
	if t.sessionID != "" {
		t.logger.Printf("[DEBUG] Reusing existing session ID: %s\n", t.sessionID)
	}

	// Try to initialize the server connection with a timeout
	// This will establish the session ID if needed
	t.logger.Printf("[DEBUG] Sending initialize request with capabilities\n")
	_, err := t.SendRequest(initCtx, "initialize", map[string]interface{}{
		"clientName":    "Go MCP Client",
		"clientVersion": "1.0.0",
		"capabilities": map[string]bool{
			"logging":  true,
			"sampling": true,
		},
	})

	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// Log the session ID after initialization
	t.logger.Printf("[DEBUG] Transport has session ID after initialize: %s\n", t.sessionID)

	// Create a listener connection for server-initiated messages
	// This is done in a separate goroutine to not block initialization
	go func() {
		err := t.openListenerConnection(ctx)
		if err != nil {
			// Just log the error, don't fail initialization if the server doesn't support GET
			// as this is optional according to the spec
			t.logger.Printf("[DEBUG] Failed to open listener connection: %v\n", err)
		}
	}()

	return nil
}

// openListenerConnection opens a GET connection to listen for server-initiated messages
func (t *StreamableHTTPTransport) openListenerConnection(ctx context.Context) error {
	streamID := fmt.Sprintf("listener-%d", time.Now().UnixNano())

	es, err := t.createEventSource(streamID, "", ctx) // Pass ctx here
	if err != nil {
		return err
	}

	t.eventSources.Store(streamID, es)

	go t.processEventSource(ctx, es)

	return nil
}

// createEventSource sets up a new SSE connection
func (t *StreamableHTTPTransport) createEventSource(streamID string, initialLastEventID string, ctx context.Context) (*StreamableEventSource, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", t.endpoint, nil)
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

	// Add Last-Event-ID if resuming with a specific ID for this stream
	if initialLastEventID != "" {
		req.Header.Set("Last-Event-ID", initialLastEventID)
	}

	// Create the event source
	es := &StreamableEventSource{
		URL:         t.endpoint,
		Headers:     t.headers, // Headers for potential reconnections by EventSource itself
		Client:      t.client,
		MessageChan: make(chan []byte, 100),
		ErrorChan:   make(chan error, 10),
		CloseChan:   make(chan struct{}),
		StreamID:    streamID,
		LastEventID: initialLastEventID, // Initialize with the passed ID
	}

	// Connect to the event source
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to event source: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log or handle close error, primary error is status code
			t.logger.Printf("[DEBUG] error closing response body after non-OK status: %v\n", closeErr)
		}
		if resp.StatusCode == http.StatusMethodNotAllowed {
			return nil, fmt.Errorf("server does not support GET for SSE (405 Method Not Allowed)")
		}
		errMsg := fmt.Sprintf("failed to connect to event source, status: %s", resp.Status)
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = fmt.Sprintf("%s, body: %s", errMsg, string(bodyBytes))
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		if err := resp.Body.Close(); err != nil {
			return nil, fmt.Errorf("error closing response body: %w", err)
		}
		return nil, fmt.Errorf("server did not return text/event-stream content type")
	}

	es.Connection = resp
	es.isConnected.Store(true)

	// Start reading events
	go es.readEvents(ctx)

	return es, nil
}

// SendRequest sends a request and waits for the response
func (t *StreamableHTTPTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	idStr := fmt.Sprintf("%s-%d", t.requestIDPrefix, t.GetNextID()) // Ensure string ID for protocol compatibility

	reqMsg, err := protocol.NewRequest(idStr, method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use a unique stream ID for this request to differentiate its SSE stream
	// if the server upgrades the POST to SSE.
	requestStreamID := "request-" + idStr

	responseChan := make(chan *protocol.Response, 1)
	t.mu.Lock()
	t.responseHandlers[idStr] = func(resp *protocol.Response) {
		select {
		case responseChan <- resp:
		default:
			t.logger.Printf("[WARN] SendRequest: responseChan for %s is full or closed, discarding response\n", idStr)
		}
	}
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.responseHandlers, idStr)
		t.mu.Unlock()
		close(responseChan)
	}()

	// Send HTTP request. The context passed here (ctx) is the one that might be cancelled.
	if err := t.sendHTTPRequest(ctx, reqMsg, requestStreamID); err != nil {
		// Check if the error was due to context cancellation during the POST operation
		if ctx.Err() != nil {
			t.logger.Printf("[DEBUG] SendRequest's initial POST for request %s failed due to context cancellation: %v. Attempting cancel notification.\n", idStr, ctx.Err())
			cancelNotifParams := protocol.CancelParams{ID: idStr}
			// Use a new, short-lived context for the cancellation notification
			cancelCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelFunc()
			if notifErr := t.SendNotification(cancelCtx, "$/cancelRequest", cancelNotifParams); notifErr != nil {
				t.logger.Printf("[WARN] SendRequest: failed to send $/cancelRequest notification for %s after POST error: %v\n", idStr, notifErr)
			}
		}
		return nil, fmt.Errorf("failed to send HTTP request for %s: %w", idStr, err)
	}

	// Wait for the response, or for the original context or a specific wait timeout to be done.
	waitCtx, waitCancel := context.WithTimeout(ctx, t.options.RequestTimeout) // waitCtx inherits cancellation from ctx
	defer waitCancel()                                                        // Cleans up the timer for waitCtx

	select {
	case resp := <-responseChan:
		if resp == nil {
			// This case should ideally not happen if sendHTTPRequest was successful and no context cancelled it
			// before a response or error was processed by handleMessage/processEventSource.
			// However, if the channel was closed by the defer without a response, this might occur.
			return nil, fmt.Errorf("received nil response for request %s, channel may have been closed prematurely", idStr)
		}
		t.logger.Printf("[DEBUG] Received response for request %s\n", idStr)
		if resp.Error != nil {
			return nil, &protocol.ErrorObject{Code: int(resp.Error.Code), Message: resp.Error.Message, Data: resp.Error.Data}
		}
		var result interface{}
		if len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result for request %s: %w", idStr, err)
			}
		}
		return result, nil

	case <-waitCtx.Done(): // Handles both waitCtx timeout and cancellation from parent ctx
		err := waitCtx.Err()
		t.logger.Printf("[DEBUG] SendRequest: waitCtx done for request %s: %v\n", idStr, err)

		// Attempt to send $/cancelRequest notification
		cancelNotifParams := protocol.CancelParams{ID: idStr}
		cancelAttemptCtx, cancelAttemptDone := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelAttemptDone()
		if notifErr := t.SendNotification(cancelAttemptCtx, "$/cancelRequest", cancelNotifParams); notifErr != nil {
			t.logger.Printf("[WARN] SendRequest: failed to send $/cancelRequest notification for %s: %v\n", idStr, notifErr)
		}

		// Clean up any dedicated SSE stream for this request
		if esVal, loaded := t.eventSources.LoadAndDelete(requestStreamID); loaded {
			if es, ok := esVal.(*StreamableEventSource); ok {
				t.logger.Printf("[DEBUG] SendRequest: closing event source %s due to context cancellation/timeout\n", requestStreamID)
				es.Close() // This will also ensure its goroutines (readEvents, processEventSource) terminate
			}
		}

		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			// This was a timeout specific to waitCtx, not the parent ctx
			return nil, fmt.Errorf("request %s timed out after %s: %w", idStr, t.options.RequestTimeout, err)
		} else if errors.Is(err, context.Canceled) || (ctx.Err() != nil) {
			// This was a cancellation, either from parent ctx or explicitly on waitCtx (though less common)
			return nil, fmt.Errorf("request %s cancelled: %w", idStr, ctx.Err()) // Prefer parent context error if available
		}
		return nil, fmt.Errorf("request %s failed: %w", idStr, err) // Fallback
	}
}

// SendNotification sends a notification (one-way message)
func (t *StreamableHTTPTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	notification, err := protocol.NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	// Send the notification via HTTP POST
	// No need to create a stream for a notification as we don't expect a response
	err = t.sendHTTPRequest(ctx, notification, "")
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
	return t.sendHTTPRequest(ctx, messages, "")
}

// sendHTTPRequest sends a JSON-RPC message or batch via HTTP POST
func (t *StreamableHTTPTransport) sendHTTPRequest(ctx context.Context, message interface{}, streamID string) error {
	// Marshal the message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create a new request with the provided context
	req, err := http.NewRequestWithContext(ctx, "POST", t.endpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Copy all custom headers
	t.mu.Lock()
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// Add session ID if available
	sessionID := t.sessionID
	t.mu.Unlock()

	if sessionID != "" {
		t.logger.Printf("[DEBUG] Adding session ID to request: %s\n", sessionID)
		req.Header.Set("MCP-Session-ID", sessionID)
	} else {
		t.logger.Printf("[DEBUG] No session ID available for request\n")
	}

	// Add stream ID if available, to associate with a specific stream
	// This is mainly for the server to know which GET stream a POST might relate to, if any.
	// For POSTs upgraded to SSE, the client manages this association via requestStreamID.
	if streamID != "" && !strings.HasPrefix(streamID, "request-") { // Don't send request- specific streamIDs as headers
		req.Header.Set("MCP-Stream-ID", streamID)
	}

	// Send the request with context timeout
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// sseStreamHandedOff is true if the response body (SSE stream) is passed to an EventSource
	sseStreamHandedOff := false
	defer func() {
		if !sseStreamHandedOff && resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				t.logger.Printf("[DEBUG] Error closing response body in sendHTTPRequest defer: %v\n", err)
			}
		}
	}()

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body) // Body is closed by defer
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Extract and store session ID from response headers
	if respSessionID := resp.Header.Get("MCP-Session-ID"); respSessionID != "" {
		t.logger.Printf("[DEBUG] Received session ID from server: %s\n", respSessionID)
		t.mu.Lock()
		if t.sessionID == "" || t.sessionID != respSessionID {
			t.sessionID = respSessionID
			t.logger.Printf("[DEBUG] Updated session ID for future requests: %s\n", respSessionID)
		}
		t.mu.Unlock()
	} else {
		t.logger.Printf("[DEBUG] No session ID received from server in response headers\n")
		// fmt.Printf("[DEBUG] Response headers: %+v\n", resp.Header) // Can be noisy
	}

	// Handle the response based on content type
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "text/event-stream") {
		t.logger.Printf("[DEBUG] Received SSE response (Content-Type: %s) for streamID: %s\n", contentType, streamID)
		// This POST request is being upgraded to an SSE stream for its response.
		// The streamID here should be the request-specific one (e.g., "request-http-123")
		if strings.HasPrefix(streamID, "request-") {
			// Create a new event source using the live HTTP response body (which is the SSE stream)
			// The context (ctx) passed to processEventSource must be the original request's context.
			lastEventID := resp.Header.Get("Last-Event-ID") // Though unlikely for a fresh POST-SSE stream
			es := &StreamableEventSource{
				URL:         t.endpoint, // For potential future reconnections, though this ES is tied to this specific resp
				Headers:     t.headers,  // Headers for potential reconnections
				Client:      t.client,   // Client for potential reconnections
				Connection:  resp,       // The live http.Response with the SSE stream
				MessageChan: make(chan []byte, 100),
				ErrorChan:   make(chan error, 10),
				CloseChan:   make(chan struct{}),
				StreamID:    streamID,
				LastEventID: lastEventID,
			}
			es.isConnected.Store(true)         // Mark as connected since we have the live response
			t.eventSources.Store(streamID, es) // Store it so SendRequest can find it

			// Start reading events from this stream using the response body directly.
			// This goroutine is tied to the lifecycle of the original request's context (ctx).
			go es.readEvents(ctx)     // readEvents will handle closing resp.Body when done or on error
			sseStreamHandedOff = true // The resp.Body is now managed by es.readEvents

			// Start processing messages from this new event source.
			// This goroutine is also tied to the lifecycle of the original request's context (ctx).
			go t.processEventSource(ctx, es)
			t.logger.Printf("[DEBUG] Handed off POST response's SSE stream to new EventSource for streamID: %s\n", streamID)
		} else {
			// This is an SSE stream not directly tied to a SendRequest call (e.g., from SendBatch or other internal POSTs)
			// We don't expect SSE responses for simple notifications/batches normally, but if it happens, log and close.
			t.logger.Printf("[WARN] Received unexpected SSE stream for non-request streamID: %s. Closing.\n", streamID)
			// Body will be closed by the main defer as sseStreamHandedOff is false.
		}
	} else if strings.Contains(contentType, "application/json") {
		t.logger.Printf("[DEBUG] Received JSON response (Content-Type: %s)\n", contentType)
		body, err := io.ReadAll(resp.Body) // Body is closed by defer
		if err != nil {
			return fmt.Errorf("failed to read JSON response body: %w", err)
		}
		if len(body) > 0 {
			if err := t.handleMessage(context.Background(), body); err != nil { // Use background context as we have the full response
				return fmt.Errorf("failed to process JSON message: %w", err)
			}
		}
	} else if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent { // HTTP 202 or 204
		t.logger.Printf("[DEBUG] Received HTTP %d %s response. No content to process.\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		// No body to process, and it will be closed by the main defer.
	} else {
		t.logger.Printf("[DEBUG] Unexpected Content-Type in response: '%s' or unhandled status code: %d\n", contentType, resp.StatusCode)
		body, err := io.ReadAll(resp.Body) // Body is closed by defer
		if err != nil {
			return fmt.Errorf("failed to read response body with unknown content type: %w", err)
		}
		if len(body) > 0 {
			t.logger.Printf("[DEBUG] Response body with unknown content type: %s\n", string(body))
			// Potentially try to handle as JSON if it's a server error without proper Content-Type
			if resp.StatusCode >= 400 {
				if err := t.handleMessage(context.Background(), body); err != nil {
					t.logger.Printf("[DEBUG] Tried to handle unknown content type as JSON error, but failed: %v\n", err)
				}
			}
		}
	}

	return nil
}

// processEventSource processes messages from an event source
func (t *StreamableHTTPTransport) processEventSource(ctx context.Context, es *StreamableEventSource) {
	t.logger.Printf("processEventSource: Starting for stream %s", es.StreamID)
	var eventReadingWG sync.WaitGroup

	defer func() {
		t.logger.Printf("processEventSource: Exiting for stream %s. Waiting for event reading goroutine to finish...", es.StreamID)
		eventReadingWG.Wait() // Wait for the event reading goroutine to finish before fully exiting
		t.logger.Printf("processEventSource: Event reading goroutine finished for stream %s.", es.StreamID)
		if es != nil {
			es.Close() // Ensure event source's underlying connection (Body) is closed
			t.logger.Printf("processEventSource: Closed event source for stream %s in defer.", es.StreamID)
		}
		// If this event source was associated with a request, remove it from active event sources
		if strings.HasPrefix(es.StreamID, "request-") {
			t.eventSources.Delete(es.StreamID)
			t.logger.Printf("processEventSource: Deleted event source %s from active map.", es.StreamID)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("processEventSource: Parent context done for stream %s: %v", es.StreamID, ctx.Err())
			return
		default:
		}

		// Create a new context for this connection attempt, derived from the parent context
		// This allows us to cancel this specific connection attempt if needed (e.g., for retry)
		esConnectionCtx, esConnectionCancel := context.WithCancel(ctx)

		eventReadingWG.Add(1)
		go func() {
			defer eventReadingWG.Done()
			defer esConnectionCancel() // Crucially, cancel the connection-specific context when this goroutine exits
			t.logger.Printf("processEventSource: Starting event reading goroutine for stream %s", es.StreamID)
			es.readEvents(esConnectionCtx) // Pass the connection-specific context
			t.logger.Printf("processEventSource: Event reading goroutine finished for stream %s", es.StreamID)
		}()

		keepProcessingThisSource := true
		for keepProcessingThisSource {
			select {
			case <-ctx.Done(): // Parent context cancelled
				t.logger.Printf("processEventSource: Parent context done during message processing for stream %s: %v", es.StreamID, ctx.Err())
				esConnectionCancel() // Signal the event reading goroutine to stop
				return               // Exit processEventSource

			case <-esConnectionCtx.Done(): // Current connection attempt's context cancelled (e.g. readEvents finished or error)
				t.logger.Printf("processEventSource: Connection context done for stream %s: %v. Will attempt reconnect if appropriate.", es.StreamID, esConnectionCtx.Err())
				keepProcessingThisSource = false // Break inner loop to retry connection

			case msg, ok := <-es.MessageChan:
				if !ok {
					t.logger.Printf("processEventSource: MessageChan closed for stream %s. Connection likely lost.", es.StreamID)
					esConnectionCancel()             // Ensure event reading goroutine is stopped
					keepProcessingThisSource = false // Break inner loop to retry connection
					continue
				}
				t.logger.Printf("processEventSource: Received message on stream %s: %s", es.StreamID, string(msg))
				if err := t.handleMessage(ctx, msg); err != nil {
					t.logger.Printf("Error handling message on stream %s: %v", es.StreamID, err)
					// Depending on error type, might decide to stop or continue
				}

			case err, ok := <-es.ErrorChan:
				if !ok {
					t.logger.Printf("processEventSource: ErrorChan closed for stream %s.", es.StreamID)
					esConnectionCancel()             // Ensure event reading goroutine is stopped
					keepProcessingThisSource = false // Break inner loop to retry connection
					continue
				}
				t.logger.Printf("processEventSource: Received error on stream %s: %v. Will attempt reconnect.", es.StreamID, err)
				esConnectionCancel()             // Ensure event reading goroutine is stopped
				keepProcessingThisSource = false // Break inner loop to retry connection
			}
		}

		// Before retrying, wait for the current event reading goroutine to complete fully.
		t.logger.Printf("processEventSource: Waiting for active event reader to finish before potential retry for stream %s...", es.StreamID)
		eventReadingWG.Wait()
		t.logger.Printf("processEventSource: Active event reader finished for stream %s.", es.StreamID)

		// If the parent context is done, don't attempt to reconnect.
		if ctx.Err() != nil {
			t.logger.Printf("processEventSource: Parent context done, not retrying connection for stream %s.", es.StreamID)
			return
		}

		// Reconnect logic
		t.logger.Printf("processEventSource: Attempting to reconnect stream %s with LastEventID: %s", es.StreamID, es.LastEventID)
		var err error
		var newEs *StreamableEventSource // Placeholder for re-establishing or reusing 'es'
		// In a real scenario, you'd try to create a new connection for 'es'
		// For simplicity in this example, we'll just simulate a delay and potentially re-use 'es' if the design allows.
		// If 'es' internally handles reconnection, then this part changes.

		// This is where createEventSource or a similar mechanism would be called again.
		// For the main listener, it should try indefinitely. For request streams, it might depend on request context.
		if es.StreamID == "listener" {
			t.logger.Printf("processEventSource: Reconnecting listener stream %s", es.StreamID)
			newEs, err = t.createEventSource(es.StreamID, es.LastEventID, ctx) // Pass parent ctx
		} else if strings.HasPrefix(es.StreamID, "request-") {
			// For request-specific streams, their lifecycle is tied to the request's context.
			// If the request context (which should be `ctx` here) is done, no need to reconnect.
			if ctx.Err() != nil {
				t.logger.Printf("processEventSource: Request context done for stream %s, not reconnecting.", es.StreamID)
				return
			}
			t.logger.Printf("processEventSource: Reconnecting request stream %s", es.StreamID)
			newEs, err = t.createEventSource(es.StreamID, es.LastEventID, ctx) // Pass parent (request) ctx
		} else {
			t.logger.Printf("processEventSource: Unknown stream type %s, not attempting reconnect.", es.StreamID)
			return
		}

		if err != nil {
			t.logger.Printf("processEventSource: Failed to recreate event source for stream %s: %v. Retrying after delay.", es.StreamID, err)
			select {
			case <-time.After(5 * time.Second): // Backoff before retrying
			case <-ctx.Done():
				t.logger.Printf("processEventSource: Parent context done during reconnect backoff for stream %s.", es.StreamID)
				return
			}
			continue // Retry the outer loop (which will attempt to create and process a new connection)
		}
		// Selectively update fields instead of copying the entire struct with mutex
		es.URL = newEs.URL
		es.Headers = newEs.Headers
		es.Client = newEs.Client
		es.Connection = newEs.Connection
		es.LastEventID = newEs.LastEventID
		es.StreamID = newEs.StreamID
		// Note: we don't copy mutex (es.mu) or channels which should be preserved
		t.logger.Printf("processEventSource: Successfully re-established event source for stream %s.", es.StreamID)
		// Loop continues, a new esConnectionCtx will be created, and readEvents will be launched for the new 'es'
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
		err := t.sendHTTPRequest(ctx, responsesBatch, "")
		if err != nil {
			return fmt.Errorf("failed to send batch responses: %w", err)
		}
	}

	return nil
}

// handleMessage processes an incoming JSON-RPC message, which could be a single message or batch
func (t *StreamableHTTPTransport) handleMessage(ctx context.Context, data []byte) error {
	// Check for empty or whitespace-only messages
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		t.logger.Printf("[DEBUG] Ignoring empty message\n")
		return nil // Just ignore empty messages
	}

	// Check if it's an empty JSON object {}
	if string(trimmed) == "{}" {
		t.logger.Printf("[DEBUG] Ignoring empty JSON object message\n")
		// This is likely a heartbeat or initialization message, just ignore it
		return nil
	}

	// First, try to check if it's a connection event or other special SSE event
	var connectionEvent struct {
		ConnectionID string `json:"connectionId"`
	}

	if err := json.Unmarshal(trimmed, &connectionEvent); err == nil && connectionEvent.ConnectionID != "" {
		t.logger.Printf("[DEBUG] Received connection event with ID: %s\n", connectionEvent.ConnectionID)
		// This is a connection event from the server, not a JSON-RPC message
		return nil
	}

	// Check if this is a batch (JSON array)
	if len(data) > 0 && data[0] == '[' {
		t.logger.Printf("[DEBUG] Processing batch message\n")
		return t.handleBatchMessage(ctx, data)
	}

	// Process single message
	// Determine message type and handle accordingly
	if protocol.IsRequest(data) {
		t.logger.Printf("[DEBUG] Processing request message\n")
		// Parse as a request
		var req protocol.Request
		if err := json.Unmarshal(data, &req); err != nil {
			return fmt.Errorf("failed to unmarshal request: %w", err)
		}

		// Handle the request using the provided context
		resp, err := t.HandleRequest(ctx, &req)
		if err != nil {
			t.logger.Printf("[ERROR] Failed to handle request %s: %v\n", req.Method, err)
			return fmt.Errorf("failed to handle request: %w", err)
		}

		// Send the response if not nil
		if resp != nil {
			// Use a short timeout context for the response
			respCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			err = t.sendHTTPRequest(respCtx, resp, "")
			if err != nil {
				t.logger.Printf("[ERROR] Failed to send response for request %s: %v\n", req.Method, err)
				return fmt.Errorf("failed to send response: %w", err)
			}
		}

		return nil
	} else if protocol.IsResponse(data) {
		t.logger.Printf("[DEBUG] Processing response message\n")
		// Parse as a response
		var resp protocol.Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Handle the response
		t.HandleResponse(&resp)

		return nil
	} else if protocol.IsNotification(data) {
		t.logger.Printf("[DEBUG] Processing notification message\n")
		// Parse as a notification
		var notif protocol.Notification
		if err := json.Unmarshal(data, &notif); err != nil {
			return fmt.Errorf("failed to unmarshal notification: %w", err)
		}

		// Handle the notification with the provided context
		err := t.HandleNotification(ctx, &notif)
		if err != nil {
			t.logger.Printf("[ERROR] Failed to handle notification %s: %v\n", notif.Method, err)
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
	t.logger.Printf("[DEBUG] Received unrecognized message: %s\n", sampleData)

	// Check if it's at least valid JSON
	var anyJSON interface{}
	jsonErr := json.Unmarshal(data, &anyJSON)

	if jsonErr != nil {
		return fmt.Errorf("invalid JSON message: %w, data: %s", jsonErr, sampleData)
	} else {
		// It's valid JSON but not a recognized JSON-RPC message type
		// Since we're already handling empty objects above, this must be another format
		t.logger.Printf("[DEBUG] Valid JSON but not recognized as JSON-RPC message: %s\n", sampleData)
		return nil // Don't treat as error, just ignore unrecognized message formats
	}
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
		if err := resp.Body.Close(); err != nil {
			return fmt.Errorf("error closing response body: %w", err)
		}
		return fmt.Errorf("failed to connect: HTTP %d", resp.StatusCode)
	}

	es.Connection = resp
	es.isConnected.Store(true)

	// Start reading events
	go es.readEvents(context.Background())

	return nil
}

// Close terminates the SSE connection
func (es *StreamableEventSource) Close() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.isConnected.Load() {
		fmt.Printf("[DEBUG] EventSource %s already closed\n", es.StreamID)
		return // Already closed
	}

	// Set the flag first to prevent concurrent access issues
	es.isConnected.Store(false)

	// Safely close the connection body if it exists
	if es.Connection != nil && es.Connection.Body != nil {
		if err := es.Connection.Body.Close(); err != nil {
			// Just log the error as we're in a cleanup path
			fmt.Printf("[DEBUG] Error closing connection body for %s: %v\n", es.StreamID, err)
		}
		// Explicitly set to nil to prevent accidental reuse
		es.Connection.Body = nil
		es.Connection = nil
	} else {
		fmt.Printf("[DEBUG] EventSource %s has nil connection or body\n", es.StreamID)
	}

	// Close the channel after handling the connection to prevent race conditions
	select {
	case <-es.CloseChan:
		// Channel already closed
		fmt.Printf("[DEBUG] EventSource %s close channel already closed\n", es.StreamID)
	default:
		fmt.Printf("[DEBUG] Closing EventSource %s channel\n", es.StreamID)
		close(es.CloseChan)
	}

	fmt.Printf("[DEBUG] EventSource %s successfully closed\n", es.StreamID)
}

// readEvents processes the SSE stream
func (es *StreamableEventSource) readEvents(ctx context.Context) {
	defer func() {
		// Additional safety check in case of panics
		if r := recover(); r != nil {
			fmt.Printf("[DEBUG] Recovered from panic in readEvents for %s: %v\n", es.StreamID, r)
		}

		// Ensure connection is marked as disconnected
		wasConnected := es.isConnected.Swap(false)

		// Safely close the connection if we were previously connected
		if wasConnected {
			es.mu.Lock()
			if es.Connection != nil && es.Connection.Body != nil {
				if err := es.Connection.Body.Close(); err != nil {
					fmt.Printf("[DEBUG] Error closing connection body in readEvents for %s: %v\n", es.StreamID, err)
				}
				// Clear the body reference to avoid accidental reuse
				es.Connection.Body = nil
			}
			es.mu.Unlock()

			// Notify of disconnection only if we were previously connected and channels are still valid
			select {
			case <-es.CloseChan:
				fmt.Printf("[DEBUG] CloseChan already closed for %s\n", es.StreamID)
			default:
				select {
				case es.ErrorChan <- fmt.Errorf("connection closed"):
					fmt.Printf("[DEBUG] Sent disconnection notification for %s\n", es.StreamID)
				default:
					fmt.Printf("[DEBUG] ErrorChan is full or closed for %s, can't send error\n", es.StreamID)
				}
			}

			fmt.Printf("[DEBUG] SSE connection closed for stream %s\n", es.StreamID)
		}
	}()

	// Safety check before starting to read
	es.mu.Lock()
	if es.Connection == nil {
		fmt.Printf("[DEBUG] Cannot read from nil connection for stream %s\n", es.StreamID)
		es.mu.Unlock()
		return
	}

	if es.Connection.Body == nil {
		fmt.Printf("[DEBUG] Cannot read from nil connection body for stream %s\n", es.StreamID)
		es.mu.Unlock()
		return
	}

	// Use a buffered reader for efficiency
	reader := bufio.NewReaderSize(es.Connection.Body, 4096) // 4KB buffer, larger than default
	es.mu.Unlock()

	eventData := ""
	eventID := ""
	eventType := ""

	// Read events line by line
	for {
		// Check if connection is still active before attempting to read
		if !es.isConnected.Load() {
			fmt.Printf("[DEBUG] Connection marked as closed, stopping read loop for stream %s\n", es.StreamID)
			return
		}

		// Use a shorter read timeout to detect stalled connections
		line, err := readLineWithTimeout(reader, 60*time.Second)
		if err != nil {
			connectionError := ""
			if err == io.EOF {
				connectionError = "EOF"
			} else if strings.Contains(err.Error(), "timeout") {
				connectionError = "read timeout"
			} else if strings.Contains(err.Error(), "connection reset") {
				connectionError = "connection reset by peer"
			} else if strings.Contains(err.Error(), "closed network") {
				connectionError = "closed network connection"
			} else {
				connectionError = err.Error()
			}

			fmt.Printf("[DEBUG] SSE read error for %s: %s\n", es.StreamID, connectionError)

			// Only send error to channel if connection was active
			if es.isConnected.Load() {
				select {
				case <-es.CloseChan:
					// Already closed
				default:
					select {
					case es.ErrorChan <- err:
						fmt.Printf("[DEBUG] Sent read error for %s\n", es.StreamID)
					default:
						fmt.Printf("[DEBUG] Cannot send error for %s (channel full or closed)\n", es.StreamID)
					}
				}
			}
			return
		}

		// Trim the trailing newline
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Skip empty lines at the beginning
		if line == "" && eventData == "" && eventID == "" && eventType == "" {
			continue
		}

		// Skip comment lines (used as heartbeats)
		if strings.HasPrefix(line, ":") {
			fmt.Printf("[DEBUG] Received SSE heartbeat ping for stream %s: %s\n", es.StreamID, line)
			continue
		}

		if line == "" {
			// End of event, process it if we have data
			if eventData != "" {
				// If we have an ID, update the last event ID
				if eventID != "" {
					es.LastEventID = eventID
					fmt.Printf("[DEBUG] Updated LastEventID to %s for stream %s\n", eventID, es.StreamID)
				}

				// Check for specific event types
				if eventType == "close" {
					// Server signaled to close the connection
					fmt.Printf("[DEBUG] Received close event for stream %s\n", es.StreamID)
					es.Close()
					return
				}

				// Check if this is a connection event
				if eventType == "connected" || eventType == "ready" {
					fmt.Printf("[DEBUG] Received connection event for stream %s: %s\n", es.StreamID, eventData)

					// Try to parse the connection event
					var connectionEvent struct {
						ConnectionID string `json:"connectionId"`
					}

					if err := json.Unmarshal([]byte(eventData), &connectionEvent); err == nil && connectionEvent.ConnectionID != "" {
						fmt.Printf("[DEBUG] Parsed connection ID: %s for stream %s\n", connectionEvent.ConnectionID, es.StreamID)
					}
				}

				// Send the event data to the message channel if still connected
				if es.isConnected.Load() {
					select {
					case es.MessageChan <- []byte(eventData):
						fmt.Printf("[DEBUG] Dispatched event data for stream %s (type: %s)\n", es.StreamID, eventType)
					default:
						fmt.Printf("[DEBUG] Failed to dispatch event data for stream %s (channel full)\n", es.StreamID)
					}
				}

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
			retryStr := strings.TrimPrefix(line, "retry:")
			retryStr = strings.TrimSpace(retryStr)

			// Try to parse retry time if provided
			if retry, err := strconv.Atoi(retryStr); err == nil {
				fmt.Printf("[DEBUG] SSE retry interval set to %dms for stream %s\n", retry, es.StreamID)
			}
		} else {
			// Unknown line type
			fmt.Printf("[DEBUG] Unknown SSE line format for stream %s: %s\n", es.StreamID, line)
		}
	}
}

// Helper function to read a line with timeout
func readLineWithTimeout(r *bufio.Reader, timeout time.Duration) (string, error) {
	type readResult struct {
		line string
		err  error
	}

	ch := make(chan readResult, 1)

	go func() {
		line, err := r.ReadString('\n')
		ch <- readResult{line, err}
	}()

	select {
	case result := <-ch:
		return result.line, result.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for line")
	}
}

// Send transmits a message over the transport.
// For StreamableHTTPTransport, this is not fully applicable since
// it operates in a request/response model. This is here to satisfy
// the Transport interface.
func (t *StreamableHTTPTransport) Send(data []byte) error {
	return fmt.Errorf("Send method not applicable for StreamableHTTPTransport")
}

// SetErrorHandler sets the handler for transport errors.
func (t *StreamableHTTPTransport) SetErrorHandler(handler ErrorHandler) {
	// This is a placeholder implementation
}

// SetReceiveHandler sets the handler for received messages.
func (t *StreamableHTTPTransport) SetReceiveHandler(handler ReceiveHandler) {
	// This is a placeholder implementation
}

// RegisterProgressHandler registers a handler for progress notifications
// with the given request ID
func (t *StreamableHTTPTransport) RegisterProgressHandler(id interface{}, handler ProgressHandler) {
	idStr := fmt.Sprintf("%v", id)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Initialize the progress handlers map if needed
	if t.progressHandlers == nil {
		t.progressHandlers = make(map[string]ProgressHandler)
	}

	t.progressHandlers[idStr] = handler
}

// GetSessionID returns the current session ID
func (t *StreamableHTTPTransport) GetSessionID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessionID
}

// SetResponseHandler sets a custom handler for a specific response ID
func (t *StreamableHTTPTransport) SetResponseHandler(id string, handler func(*protocol.Response)) {
	// Create a channel for the response
	responseChan := make(chan *protocol.Response, 1)

	// Add the channel to the pending requests
	t.BaseTransport.Lock()
	t.BaseTransport.pendingRequests[id] = responseChan
	t.BaseTransport.Unlock()

	// Start a goroutine to handle the response
	go func() {
		resp, ok := <-responseChan
		if ok && resp != nil {
			handler(resp)
		}
	}()
}

// RemoveResponseHandler removes a custom response handler
func (t *StreamableHTTPTransport) RemoveResponseHandler(id string) {
	t.BaseTransport.Lock()
	delete(t.BaseTransport.pendingRequests, id)
	t.BaseTransport.Unlock()
}
