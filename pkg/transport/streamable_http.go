package transport

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// ReconnectionConfig configures the reconnection behavior
type ReconnectionConfig struct {
	InitialBackoff      time.Duration
	MaxBackoff          time.Duration
	MaxRetries          int
	BackoffMultiplier   float64
	JitterPercent       float64
	HealthCheckInterval time.Duration
}

// DefaultReconnectionConfig returns sensible defaults for reconnection
func DefaultReconnectionConfig() *ReconnectionConfig {
	return &ReconnectionConfig{
		InitialBackoff:      time.Second,
		MaxBackoff:          30 * time.Second,
		MaxRetries:          10,
		BackoffMultiplier:   2.0,
		JitterPercent:       20.0,
		HealthCheckInterval: 30 * time.Second,
	}
}

// ConnectionState represents the current state of a connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateFailed
)

// String returns a human-readable representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ErrorType classifies errors for retry logic
type ErrorType int

const (
	ErrorTypeRetryable ErrorType = iota
	ErrorTypeNonRetryable
	ErrorTypeBackoff
	ErrorTypeCircuitBreaker
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for connection reliability
type CircuitBreaker struct {
	state            CircuitBreakerState
	failureCount     int
	lastFailureTime  time.Time
	lastSuccessTime  time.Time
	failureThreshold int
	recoveryTimeout  time.Duration
	halfOpenMaxCalls int
	halfOpenCalls    int
	mu               sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with default settings
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: 5,
		recoveryTimeout:  60 * time.Second,
		halfOpenMaxCalls: 3,
	}
}

// ReconnectionMetrics tracks reconnection statistics
type ReconnectionMetrics struct {
	attempts      int64
	successes     int64
	failures      int64
	totalDuration time.Duration
	lastAttempt   time.Time
	mu            sync.RWMutex
}

// EventBuffer buffers events during brief disconnections for improved reliability
type EventBuffer struct {
	events  [][]byte
	maxSize int
	mu      sync.Mutex
}

// NewEventBuffer creates a new event buffer with the specified maximum size
func NewEventBuffer(maxSize int) *EventBuffer {
	if maxSize <= 0 {
		maxSize = 100 // Default buffer size
	}
	return &EventBuffer{
		events:  make([][]byte, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add adds an event to the buffer, removing old events if necessary
func (eb *EventBuffer) Add(event []byte) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Create a copy of the event data to avoid reference issues
	eventCopy := make([]byte, len(event))
	copy(eventCopy, event)

	if len(eb.events) >= eb.maxSize {
		// Remove oldest event (FIFO)
		eb.events = eb.events[1:]
	}

	eb.events = append(eb.events, eventCopy)
}

// DrainTo drains all buffered events to the provided channel
func (eb *EventBuffer) DrainTo(ch chan []byte) int {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	drained := 0
	for _, event := range eb.events {
		select {
		case ch <- event:
			drained++
		default:
			// Channel is full, stop draining
			break
		}
	}

	// Clear the buffer after draining
	eb.events = eb.events[:0]
	return drained
}

// Size returns the current number of buffered events
func (eb *EventBuffer) Size() int {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	return len(eb.events)
}

// drainBufferedEvents drains any buffered events to the message channel
// This should be called when a connection is successfully (re)established
func (es *StreamableEventSource) drainBufferedEvents() {
	if es.eventBuffer == nil {
		return
	}

	bufferSize := es.eventBuffer.Size()
	if bufferSize > 0 {
		drained := es.eventBuffer.DrainTo(es.MessageChan)
		fmt.Printf("[DEBUG] Drained %d/%d buffered events for stream %s\n", drained, bufferSize, es.StreamID)
	}
}

// StreamableHTTPTransport implements Transport using the Streamable HTTP protocol
// which provides advanced features over the basic HTTP+SSE transport like
// resumability, session management, multiple connection support, and built-in reliability.
type StreamableHTTPTransport struct {
	*BaseTransport
	endpoint        string
	client          *http.Client
	eventSources    sync.Map // map[string]*StreamableEventSource - multiple connections support
	running         atomic.Bool
	headers         map[string]string
	requestIDPrefix string
	sessionID       string
	allowedOrigins  []string // For client-side origin validation

	// Enhanced connection management
	reconnectConfig *ReconnectionConfig
	circuitBreaker  *CircuitBreaker
	connectionState ConnectionState
	metrics         *ReconnectionMetrics

	mu               sync.Mutex
	progressHandlers map[string]ProgressHandler
	responseHandlers map[string]func(*protocol.Response)
	pendingRequests  map[string]chan *protocol.Response
	logger           *log.Logger
	sessionHandler   SessionHandler
}

// StreamableEventSource is an enhanced client for Server-Sent Events
// with support for resumability and event IDs
type StreamableEventSource struct {
	URL           string
	Headers       map[string]string
	Client        *http.Client
	Connection    *http.Response
	MessageChan   chan []byte
	ErrorChan     chan error
	CloseChan     chan struct{}
	LastEventID   string
	StreamID      string        // Unique identifier for this stream
	retryInterval time.Duration // SSE retry interval from server
	eventBuffer   *EventBuffer  // Buffer events during brief disconnections
	mu            sync.Mutex
	isConnected   atomic.Bool
}

// newStreamableHTTPTransport creates a new Streamable HTTP transport from config
func newStreamableHTTPTransport(config TransportConfig) (Transport, error) {
	client := &http.Client{
		Timeout: config.Performance.RequestTimeout,
	}

	t := &StreamableHTTPTransport{
		BaseTransport:    NewBaseTransport(),
		endpoint:         config.Endpoint,
		client:           client,
		headers:          make(map[string]string),
		requestIDPrefix:  "streamable-http",
		allowedOrigins:   []string{}, // Empty by default, configured via middleware
		connectionState:  StateDisconnected,
		pendingRequests:  make(map[string]chan *protocol.Response),
		progressHandlers: make(map[string]ProgressHandler),
		responseHandlers: make(map[string]func(*protocol.Response)),
		logger:           log.New(os.Stdout, "StreamableHTTPTransport: ", log.LstdFlags),
	}

	// Configure based on clean config
	t.reconnectConfig = &ReconnectionConfig{
		InitialBackoff:      config.Reliability.InitialRetryDelay,
		MaxBackoff:          config.Reliability.MaxRetryDelay,
		MaxRetries:          config.Reliability.MaxRetries,
		BackoffMultiplier:   config.Reliability.RetryBackoffFactor,
		JitterPercent:       20.0, // Standard jitter
		HealthCheckInterval: 30 * time.Second,
	}

	// Circuit breaker configuration
	if config.Reliability.CircuitBreaker.Enabled {
		t.circuitBreaker = NewCircuitBreaker()
		t.circuitBreaker.failureThreshold = config.Reliability.CircuitBreaker.FailureThreshold
		t.circuitBreaker.recoveryTimeout = config.Reliability.CircuitBreaker.Timeout
		t.circuitBreaker.halfOpenMaxCalls = config.Reliability.CircuitBreaker.SuccessThreshold
	}

	// Metrics initialization (replaced by middleware)
	t.metrics = &ReconnectionMetrics{}

	return t, nil
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
	// Validate origin if configured
	if err := t.validateOrigin(); err != nil {
		return err
	}

	// Set default headers
	t.SetHeader("Accept", "application/json, text/event-stream")

	// Connection pooling will be started via middleware when implemented

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
		return mcperrors.HTTPTransportError("initialize", t.endpoint, 0, err).
			WithContext(&mcperrors.Context{
				Component: "StreamableHTTPTransport",
				Operation: "initialize",
			})
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

	// Reliability features will be started via middleware when implemented

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
		LastEventID: initialLastEventID,  // Initialize with the passed ID
		eventBuffer: NewEventBuffer(100), // Buffer up to 100 events during disconnections
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
			return nil, mcperrors.HTTPTransportError("method_not_allowed", t.endpoint, resp.StatusCode,
				fmt.Errorf("server does not support GET for SSE")).
				WithContext(&mcperrors.Context{
					Component: "StreamableHTTPTransport",
					Operation: "connect_event_source",
				})
		}
		errMsg := fmt.Sprintf("failed to connect to event source, status: %s", resp.Status)
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = fmt.Sprintf("%s, body: %s", errMsg, string(bodyBytes))
		}
		return nil, mcperrors.HTTPTransportError("connect_failed", t.endpoint, resp.StatusCode,
			errors.New(errMsg)).
			WithContext(&mcperrors.Context{
				Component: "StreamableHTTPTransport",
				Operation: "connect_event_source",
			})
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		if err := resp.Body.Close(); err != nil {
			return nil, mcperrors.HTTPTransportError("close_response_body", t.endpoint, resp.StatusCode, err)
		}
		return nil, mcperrors.HTTPTransportError("invalid_content_type", t.endpoint, resp.StatusCode,
			fmt.Errorf("server did not return text/event-stream content type")).
			WithContext(&mcperrors.Context{
				Component: "StreamableHTTPTransport",
				Operation: "validate_content_type",
			})
	}

	es.Connection = resp
	es.isConnected.Store(true)

	// Drain any buffered events from previous disconnection
	es.drainBufferedEvents()

	// Start reading events
	go es.readEvents(ctx)

	return es, nil
}

// SendRequest sends a request and waits for the response
func (t *StreamableHTTPTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	idStr := fmt.Sprintf("%s-%d", t.requestIDPrefix, t.GetNextID()) // Ensure string ID for protocol compatibility

	// Reliability will be handled via middleware when implemented

	reqMsg, err := protocol.NewRequest(idStr, method, params)
	if err != nil {
		return nil, mcperrors.WrapProtocolError(err, method, idStr)
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
		return nil, mcperrors.HTTPTransportError("send_request", t.endpoint, 0, err).
			WithContext(&mcperrors.Context{
				RequestID: idStr,
				Method:    method,
				Component: "StreamableHTTPTransport",
				Operation: "send_http_request",
			})
	}

	// Wait for the response, or for the original context or a specific wait timeout to be done.
	waitCtx, waitCancel := context.WithTimeout(ctx, 30*time.Second) // Default timeout, now handled by middleware
	defer waitCancel()                                              // Cleans up the timer for waitCtx

	select {
	case resp := <-responseChan:
		if resp == nil {
			// This case should ideally not happen if sendHTTPRequest was successful and no context cancelled it
			// before a response or error was processed by handleMessage/processEventSource.
			// However, if the channel was closed by the defer without a response, this might occur.
			return nil, mcperrors.NewError(
				mcperrors.CodeInternalError,
				"Received nil response, channel may have been closed prematurely",
				mcperrors.CategoryInternal,
				mcperrors.SeverityError,
			).WithContext(&mcperrors.Context{
				RequestID: idStr,
				Method:    method,
				Component: "StreamableHTTPTransport",
				Operation: "wait_response",
			})
		}
		t.logger.Printf("[DEBUG] Received response for request %s\n", idStr)
		if resp.Error != nil {
			return nil, &protocol.ErrorObject{Code: int(resp.Error.Code), Message: resp.Error.Message, Data: resp.Error.Data}
		}
		var result interface{}
		if len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				return nil, mcperrors.CreateInternalError("unmarshal_result", err).
					WithContext(&mcperrors.Context{
						RequestID: idStr,
						Method:    method,
						Component: "StreamableHTTPTransport",
						Operation: "unmarshal_result",
					})
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
			return nil, mcperrors.ResponseTimeout("streamable_http", idStr, 30*time.Second).
				WithContext(&mcperrors.Context{
					RequestID: idStr,
					Method:    method,
					Component: "StreamableHTTPTransport",
					Operation: "wait_response",
				})
		} else if errors.Is(err, context.Canceled) || (ctx.Err() != nil) {
			// This was a cancellation, either from parent ctx or explicitly on waitCtx (though less common)
			return nil, mcperrors.NewError(
				mcperrors.CodeOperationCancelled,
				"Request was cancelled",
				mcperrors.CategoryCancelled,
				mcperrors.SeverityInfo,
			).WithContext(&mcperrors.Context{
				RequestID: idStr,
				Method:    method,
				Component: "StreamableHTTPTransport",
				Operation: "wait_response",
			}).WithDetail(fmt.Sprintf("Context error: %v", ctx.Err()))
		}
		return nil, mcperrors.NewError(
			mcperrors.CodeInternalError,
			"Request failed",
			mcperrors.CategoryInternal,
			mcperrors.SeverityError,
		).WithContext(&mcperrors.Context{
			RequestID: idStr,
			Method:    method,
			Component: "StreamableHTTPTransport",
			Operation: "wait_response",
		}).WithDetail(fmt.Sprintf("Error: %v", err))
	}
}

// Reliability methods will be implemented via middleware

// SendNotification sends a notification (one-way message)
func (t *StreamableHTTPTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	// Reliability will be handled via middleware when implemented

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

// SendBatchRequest sends a batch of requests and/or notifications
func (t *StreamableHTTPTransport) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	if batch == nil || batch.Len() == 0 {
		return nil, fmt.Errorf("batch request is empty")
	}

	// For HTTP transport, we can send the entire batch in a single request
	// and expect a batch response
	batchID := fmt.Sprintf("batch-%s-%d", t.requestIDPrefix, t.GetNextID())

	// Create maps to track responses for each request
	responseMap := make(map[string]chan *protocol.Response)
	requests := batch.GetRequests()

	// Set up response handlers for each request in the batch
	t.mu.Lock()
	for _, req := range requests {
		idStr := fmt.Sprintf("%v", req.ID)
		ch := make(chan *protocol.Response, 1)
		responseMap[idStr] = ch
		t.responseHandlers[idStr] = func(resp *protocol.Response) {
			select {
			case ch <- resp:
			default:
				t.logger.Printf("[WARN] SendBatchRequest: response channel for %s is full or closed\n", idStr)
			}
		}
	}
	t.mu.Unlock()

	// Clean up handlers on exit
	defer func() {
		t.mu.Lock()
		for _, req := range requests {
			idStr := fmt.Sprintf("%v", req.ID)
			delete(t.responseHandlers, idStr)
		}
		t.mu.Unlock()
		for _, ch := range responseMap {
			close(ch)
		}
	}()

	// Send the batch request
	if err := t.sendHTTPRequest(ctx, batch, batchID); err != nil {
		return nil, fmt.Errorf("failed to send batch request: %w", err)
	}

	// If batch contains only notifications, return empty response immediately
	if len(requests) == 0 {
		return &protocol.JSONRPCBatchResponse{}, nil
	}

	// Wait for all responses with timeout
	responses := make([]*protocol.Response, 0, len(requests))
	responseTimeout := 30 * time.Second
	timer := time.NewTimer(responseTimeout)
	defer timer.Stop()

	// Create a context for waiting on responses
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Collect responses
	for len(responses) < len(requests) {
		select {
		case <-waitCtx.Done():
			return nil, waitCtx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("timeout waiting for batch responses after %v", responseTimeout)
		default:
			// Try to collect responses
			hasResponse := false
			for id, ch := range responseMap {
				select {
				case resp := <-ch:
					if resp != nil {
						responses = append(responses, resp)
						delete(responseMap, id)
						hasResponse = true
					}
				default:
					// Non-blocking
				}
			}
			// If no response received in this iteration, wait briefly
			if !hasResponse {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	// Order responses according to request order
	orderedResponses := make([]*protocol.Response, 0, len(responses))
	for _, req := range requests {
		for _, resp := range responses {
			if fmt.Sprintf("%v", resp.ID) == fmt.Sprintf("%v", req.ID) {
				orderedResponses = append(orderedResponses, resp)
				break
			}
		}
	}

	return protocol.NewJSONRPCBatchResponse(orderedResponses...), nil
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

	// Close all event sources and wait for goroutines to finish
	t.eventSources.Range(func(key, value interface{}) bool {
		if es, ok := value.(*StreamableEventSource); ok {
			es.Close()
		}
		t.eventSources.Delete(key)
		return true
	})

	// Note: Individual event sources use errgroup internally for coordination
	// No need for manual timeout here

	// Clear all handler maps
	t.mu.Lock()
	for k := range t.progressHandlers {
		delete(t.progressHandlers, k)
	}

	for k := range t.responseHandlers {
		delete(t.responseHandlers, k)
	}

	// Close and clear pending request channels
	for k, ch := range t.pendingRequests {
		select {
		case <-ch:
			// Channel already has a response, don't close
		default:
			close(ch)
		}
		delete(t.pendingRequests, k)
	}

	// Clear headers map
	for k := range t.headers {
		delete(t.headers, k)
	}
	t.mu.Unlock()

	// Reliability cleanup will be handled via middleware when implemented

	// Connection pool cleanup will be handled via middleware when implemented

	// Clean up BaseTransport resources
	t.BaseTransport.Cleanup()

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

	// Copy headers with proper locking
	t.mu.Lock()
	headersCopy := make(map[string]string, len(t.headers))
	for k, v := range t.headers {
		headersCopy[k] = v
	}
	sessionID := t.sessionID
	t.mu.Unlock()

	// Apply headers outside the lock
	for k, v := range headersCopy {
		req.Header.Set(k, v)
	}

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
	// Send the request
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
				eventBuffer: NewEventBuffer(100), // Buffer up to 100 events during disconnections
			}
			es.isConnected.Store(true) // Mark as connected since we have the live response

			// Drain any buffered events from previous disconnection
			es.drainBufferedEvents()

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
			return mcperrors.HTTPTransportError("read_response_body", t.endpoint, resp.StatusCode, err).
				WithContext(&mcperrors.Context{
					Component: "StreamableHTTPTransport",
					Operation: "read_json_response",
				})
		}
		if len(body) > 0 {
			if err := t.handleMessage(context.Background(), body); err != nil { // Use background context as we have the full response
				return mcperrors.CreateInternalError("process_json_message", err).
					WithContext(&mcperrors.Context{
						Component: "StreamableHTTPTransport",
						Operation: "handle_json_response",
					})
			}
		}
	} else if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent { // HTTP 202 or 204
		t.logger.Printf("[DEBUG] Received HTTP %d %s response. No content to process.\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		// No body to process, and it will be closed by the main defer.
	} else {
		t.logger.Printf("[DEBUG] Unexpected Content-Type in response: '%s' or unhandled status code: %d\n", contentType, resp.StatusCode)
		body, err := io.ReadAll(resp.Body) // Body is closed by defer
		if err != nil {
			return mcperrors.HTTPTransportError("read_unknown_response", t.endpoint, resp.StatusCode, err).
				WithContext(&mcperrors.Context{
					Component: "StreamableHTTPTransport",
					Operation: "read_unknown_content_type",
				})
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

	defer func() {
		t.logger.Printf("processEventSource: Exiting for stream %s", es.StreamID)
		es.Close()
		// If this event source was associated with a request, remove it from active event sources
		if strings.HasPrefix(es.StreamID, "request-") {
			t.eventSources.Delete(es.StreamID)
			t.logger.Printf("processEventSource: Deleted event source %s from active map.", es.StreamID)
		}
	}()

	// Enhanced reconnection logic with circuit breaker and configurable backoff
	attempt := 0
	t.updateConnectionState(StateConnecting)

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("processEventSource: Parent context done for stream %s: %v", es.StreamID, ctx.Err())
			t.updateConnectionState(StateDisconnected)
			return
		default:
		}

		// Check circuit breaker before attempting connection
		circuitState := t.circuitBreaker.State()
		if circuitState == CircuitOpen {
			t.logger.Printf("processEventSource: Circuit breaker is open for stream %s, waiting for recovery", es.StreamID)
			t.updateConnectionState(StateFailed)

			// Wait for circuit breaker recovery timeout
			select {
			case <-time.After(t.circuitBreaker.recoveryTimeout):
				t.logger.Printf("processEventSource: Circuit breaker recovery timeout elapsed for stream %s", es.StreamID)
			case <-ctx.Done():
				t.logger.Printf("processEventSource: Parent context done during circuit breaker wait for stream %s", es.StreamID)
				return
			}
			continue
		}

		// Check retry limits
		if attempt >= t.reconnectConfig.MaxRetries {
			t.logger.Printf("processEventSource: Maximum retry attempts (%d) reached for stream %s",
				t.reconnectConfig.MaxRetries, es.StreamID)
			t.updateConnectionState(StateFailed)
			return
		}

		// Calculate backoff with jitter for this attempt
		backoffDuration := t.calculateBackoffWithJitter(attempt, es)

		// Apply backoff delay (except for first attempt)
		if attempt > 0 {
			t.logger.Printf("processEventSource: Waiting %v before reconnection attempt %d for stream %s",
				backoffDuration, attempt+1, es.StreamID)

			select {
			case <-time.After(backoffDuration):
				// Continue with reconnection attempt
			case <-ctx.Done():
				t.logger.Printf("processEventSource: Parent context done during backoff for stream %s", es.StreamID)
				t.updateConnectionState(StateDisconnected)
				return
			}
		}

		// Record reconnection attempt start
		reconnectStart := time.Now()
		t.updateConnectionState(StateReconnecting)

		// Attempt connection within circuit breaker
		connectionErr := t.circuitBreaker.Call(func() error {
			return t.attemptEventSourceConnection(ctx, es, attempt)
		})

		reconnectDuration := time.Since(reconnectStart)

		if connectionErr != nil {
			// Classify error to determine retry strategy
			errorType := classifyError(connectionErr)

			t.logger.Printf("processEventSource: Connection attempt %d failed for stream %s: %v (error type: %d)",
				attempt+1, es.StreamID, connectionErr, errorType)

			// Record failed attempt
			t.recordReconnectionAttempt(false, reconnectDuration)

			// Handle different error types
			switch errorType {
			case ErrorTypeNonRetryable:
				t.logger.Printf("processEventSource: Non-retryable error for stream %s, stopping reconnection", es.StreamID)
				t.updateConnectionState(StateFailed)
				return
			case ErrorTypeCircuitBreaker:
				t.logger.Printf("processEventSource: Circuit breaker error for stream %s, will retry after recovery", es.StreamID)
				attempt++ // Increment for circuit breaker scenarios
				continue
			default:
				// Retryable error - increment attempt and continue
				attempt++
				continue
			}
		}

		// Connection successful
		t.logger.Printf("processEventSource: Successfully reconnected stream %s after %d attempts in %v",
			es.StreamID, attempt+1, reconnectDuration)

		t.recordReconnectionAttempt(true, reconnectDuration)
		t.updateConnectionState(StateConnected)

		// Reset attempt counter on successful connection
		attempt = 0

		// Process the connected stream
		connectionLost := t.processConnectedEventSource(ctx, es)

		if !connectionLost {
			// Connection was deliberately closed, not lost
			t.logger.Printf("processEventSource: Connection deliberately closed for stream %s", es.StreamID)
			t.updateConnectionState(StateDisconnected)
			return
		}

		// Connection lost, will retry
		t.logger.Printf("processEventSource: Connection lost for stream %s, preparing to reconnect", es.StreamID)
		attempt++
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
			return mcperrors.HTTPTransportError("send_batch_responses", t.endpoint, 0, err).
				WithContext(&mcperrors.Context{
					Component: "StreamableHTTPTransport",
					Operation: "send_batch_responses",
				})
		}
	}

	return nil
}

// HandleResponse overrides BaseTransport's HandleResponse to use StreamableHTTPTransport's own response handlers
func (t *StreamableHTTPTransport) HandleResponse(resp *protocol.Response) {
	// Convert response ID to string for consistent map lookup
	stringID := fmt.Sprintf("%v", resp.ID)

	// Message acknowledgment will be handled via middleware when implemented

	// Try custom handlers first
	t.mu.Lock()
	handler, hasCustomHandler := t.responseHandlers[stringID]
	handlersCount := len(t.responseHandlers)
	t.mu.Unlock()

	if hasCustomHandler && handler != nil {
		t.logger.Printf("[DEBUG] HandleResponse: Using custom handler for response ID %s (total handlers: %d)", stringID, handlersCount)
		handler(resp)

		// Clean up the handler after use
		t.mu.Lock()
		delete(t.responseHandlers, stringID)
		t.mu.Unlock()
		return
	}

	// Check pending requests channel
	t.mu.Lock()
	respChan, hasPending := t.pendingRequests[stringID]
	t.mu.Unlock()

	if hasPending && respChan != nil {
		select {
		case respChan <- resp:
			t.logger.Printf("[DEBUG] HandleResponse: Sent response to pending channel for ID %s", stringID)
		case <-time.After(5 * time.Second):
			t.logger.Printf("[DEBUG] HandleResponse: Timeout sending response to channel for ID %s", stringID)
		}

		// Clean up
		t.mu.Lock()
		delete(t.pendingRequests, stringID)
		t.mu.Unlock()
		return
	}

	// Fall back to BaseTransport's handler
	t.logger.Printf("[DEBUG] HandleResponse: No custom handler or pending request for ID %s, using BaseTransport", stringID)
	t.BaseTransport.HandleResponse(resp)
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
			return mcperrors.CreateInternalError("unmarshal_request", err).
				WithContext(&mcperrors.Context{
					Component: "StreamableHTTPTransport",
					Operation: "handle_message",
				})
		}

		// Handle the request using the provided context
		resp, err := t.HandleRequest(ctx, &req)
		if err != nil {
			t.logger.Printf("[ERROR] Failed to handle request %s: %v\n", req.Method, err)
			return mcperrors.CreateInternalError("handle_request", err).
				WithContext(&mcperrors.Context{
					Method:    req.Method,
					Component: "StreamableHTTPTransport",
					Operation: "handle_request",
				})
		}

		// Send the response if not nil
		if resp != nil {
			// Use a short timeout context for the response
			respCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			err = t.sendHTTPRequest(respCtx, resp, "")
			if err != nil {
				t.logger.Printf("[ERROR] Failed to send response for request %s: %v\n", req.Method, err)
				return mcperrors.HTTPTransportError("send_response", t.endpoint, 0, err).
					WithContext(&mcperrors.Context{
						Method:    req.Method,
						Component: "StreamableHTTPTransport",
						Operation: "send_response",
					})
			}
		}

		return nil
	} else if protocol.IsResponse(data) {
		t.logger.Printf("[DEBUG] Processing response message\n")
		// Parse as a response
		var resp protocol.Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcperrors.CreateInternalError("unmarshal_response", err).
				WithContext(&mcperrors.Context{
					Component: "StreamableHTTPTransport",
					Operation: "handle_message",
				})
		}

		// Handle the response
		t.HandleResponse(&resp)

		return nil
	} else if protocol.IsNotification(data) {
		t.logger.Printf("[DEBUG] Processing notification message\n")
		// Parse as a notification
		var notif protocol.Notification
		if err := json.Unmarshal(data, &notif); err != nil {
			return mcperrors.CreateInternalError("unmarshal_notification", err).
				WithContext(&mcperrors.Context{
					Component: "StreamableHTTPTransport",
					Operation: "handle_message",
				})
		}

		// Handle the notification with the provided context
		err := t.HandleNotification(ctx, &notif)
		if err != nil {
			t.logger.Printf("[ERROR] Failed to handle notification %s: %v\n", notif.Method, err)
			return mcperrors.CreateInternalError("handle_notification", err).
				WithContext(&mcperrors.Context{
					Method:    notif.Method,
					Component: "StreamableHTTPTransport",
					Operation: "handle_notification",
				})
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
		return mcperrors.NewError(
			mcperrors.CodeProtocolError,
			"Invalid JSON message received",
			mcperrors.CategoryProtocol,
			mcperrors.SeverityError,
		).WithContext(&mcperrors.Context{
			Component: "StreamableHTTPTransport",
			Operation: "handle_message",
		}).WithDetail(fmt.Sprintf("JSON error: %v, data: %s", jsonErr, sampleData))
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

	// Drain any buffered events from previous disconnection
	es.drainBufferedEvents()

	// Start reading events
	go es.readEvents(context.Background())

	return nil
}

// Close terminates the SSE connection
func (es *StreamableEventSource) Close() {
	// Ensure we only close once using atomic operation
	if !es.isConnected.CompareAndSwap(true, false) {
		fmt.Printf("[DEBUG] EventSource %s already closed\n", es.StreamID)
		return
	}

	es.mu.Lock()
	defer es.mu.Unlock()

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
func (es *StreamableEventSource) readEvents(_ context.Context) {
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
					es.mu.Lock()
					es.LastEventID = eventID
					es.mu.Unlock()
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

			// Try to parse retry time if provided (in milliseconds per SSE spec)
			if retry, err := strconv.Atoi(retryStr); err == nil && retry > 0 {
				es.mu.Lock()
				es.retryInterval = time.Duration(retry) * time.Millisecond
				es.mu.Unlock()
				fmt.Printf("[DEBUG] SSE retry interval set to %dms for stream %s\n", retry, es.StreamID)
			}
		} else {
			// Unknown line type
			fmt.Printf("[DEBUG] Unknown SSE line format for stream %s: %s\n", es.StreamID, line)
		}
	}
}

// attemptEventSourceConnection attempts to create a new event source connection
func (t *StreamableHTTPTransport) attemptEventSourceConnection(ctx context.Context, es *StreamableEventSource, attempt int) error {
	// Check if we should reconnect based on stream type and context
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled")
	}

	// Check transport state
	if !t.running.Load() {
		return fmt.Errorf("transport not running")
	}

	// For request-specific streams, check if the response handler still exists
	if strings.HasPrefix(es.StreamID, "request-") {
		t.mu.Lock()
		_, hasHandler := t.responseHandlers[es.StreamID]
		t.mu.Unlock()
		if !hasHandler {
			return fmt.Errorf("no response handler for request stream")
		}
	}

	// Get LastEventID safely
	es.mu.Lock()
	lastEventID := es.LastEventID
	streamID := es.StreamID
	es.mu.Unlock()

	t.logger.Printf("attemptEventSourceConnection: Attempting to reconnect stream %s with LastEventID: %s (attempt %d)",
		streamID, lastEventID, attempt+1)

	// Create new event source based on stream type
	var newEs *StreamableEventSource
	var err error

	if strings.HasPrefix(es.StreamID, "listener") {
		newEs, err = t.createEventSource(es.StreamID, lastEventID, ctx)
	} else if strings.HasPrefix(es.StreamID, "request-") {
		newEs, err = t.createEventSource(es.StreamID, lastEventID, ctx)
	} else {
		return fmt.Errorf("unknown stream type for stream %s", es.StreamID)
	}

	if err != nil {
		return fmt.Errorf("failed to create event source: %w", err)
	}

	// Safely update event source fields with new connection
	es.mu.Lock()
	es.URL = newEs.URL
	es.Headers = newEs.Headers
	es.Client = newEs.Client
	es.Connection = newEs.Connection
	es.LastEventID = newEs.LastEventID
	es.StreamID = newEs.StreamID
	es.mu.Unlock()

	return nil
}

// processConnectedEventSource handles the message processing loop for a connected event source
func (t *StreamableHTTPTransport) processConnectedEventSource(ctx context.Context, es *StreamableEventSource) bool {
	// Create a new context for this connection session
	esConnectionCtx, esConnectionCancel := context.WithCancel(ctx)
	readEventsDone := make(chan struct{})

	// Start the event reading goroutine
	go func() {
		defer close(readEventsDone)
		defer esConnectionCancel()
		t.logger.Printf("processConnectedEventSource: Starting event reading goroutine for stream %s", es.StreamID)
		es.readEvents(esConnectionCtx)
		t.logger.Printf("processConnectedEventSource: Event reading goroutine finished for stream %s", es.StreamID)
	}()

	// Process messages until connection fails
	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("processConnectedEventSource: Parent context done for stream %s: %v", es.StreamID, ctx.Err())
			esConnectionCancel()
			<-readEventsDone // Wait for readEvents to complete
			return false     // Deliberate disconnection, not a connection loss

		case <-esConnectionCtx.Done():
			t.logger.Printf("processConnectedEventSource: Connection context done for stream %s: %v", es.StreamID, esConnectionCtx.Err())
			<-readEventsDone // Wait for readEvents to complete
			return true      // Connection lost, should retry

		case msg, ok := <-es.MessageChan:
			if !ok {
				t.logger.Printf("processConnectedEventSource: MessageChan closed for stream %s. Connection likely lost.", es.StreamID)
				esConnectionCancel()
				<-readEventsDone // Wait for readEvents to complete
				return true      // Connection lost, should retry
			}

			t.logger.Printf("processConnectedEventSource: Received message on stream %s: %s", es.StreamID, string(msg))
			if err := t.handleMessage(ctx, msg); err != nil {
				t.logger.Printf("Error handling message on stream %s: %v", es.StreamID, err)
			}

		case err, ok := <-es.ErrorChan:
			if !ok || err != nil {
				t.logger.Printf("processConnectedEventSource: Error on stream %s: %v", es.StreamID, err)
				esConnectionCancel()
				<-readEventsDone // Wait for readEvents to complete
				return true      // Connection lost due to error, should retry
			}
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
		return "", mcperrors.ConnectionTimeout("streamable_http", "", timeout)
	}
}

// Send transmits a message over the transport.
// For StreamableHTTPTransport, this is not fully applicable since
// it operates in a request/response model. This is here to satisfy
// the Transport interface.
func (t *StreamableHTTPTransport) Send(data []byte) error {
	return mcperrors.NewError(
		mcperrors.CodeOperationNotSupported,
		"Send method not applicable for StreamableHTTPTransport",
		mcperrors.CategoryValidation,
		mcperrors.SeverityError,
	).WithContext(&mcperrors.Context{
		Component: "StreamableHTTPTransport",
		Operation: "send",
	})
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

// SetSessionHandler sets a handler for session lifecycle events
func (t *StreamableHTTPTransport) SetSessionHandler(handler SessionHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionHandler = handler
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

// SetAllowedOrigins configures allowed origins for this transport client
// This is important for client-side validation when connecting to servers
func (t *StreamableHTTPTransport) SetAllowedOrigins(origins []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.allowedOrigins = make([]string, len(origins))
	copy(t.allowedOrigins, origins)
}

// AddAllowedOrigin adds an allowed origin for this transport client
func (t *StreamableHTTPTransport) AddAllowedOrigin(origin string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.allowedOrigins = append(t.allowedOrigins, origin)
}

// validateOrigin validates that the current endpoint origin is allowed
func (t *StreamableHTTPTransport) validateOrigin() error {
	// If no origins are configured, skip validation (backward compatibility)
	t.mu.Lock()
	origins := make([]string, len(t.allowedOrigins))
	copy(origins, t.allowedOrigins)
	t.mu.Unlock()

	if len(origins) == 0 {
		return nil
	}

	// Extract origin from endpoint URL
	endpointOrigin, err := t.extractOriginFromURL(t.endpoint)
	if err != nil {
		return err
	}

	// Check if the endpoint origin is allowed
	for _, allowed := range origins {
		if allowed == "*" || allowed == endpointOrigin {
			return nil
		}
		// Special handling for localhost - if both are localhost, allow regardless of port
		if t.isLocalhostOrigin(endpointOrigin) && t.isLocalhostOrigin(allowed) {
			// Also check that the schemes match for localhost
			endpointScheme := strings.Split(endpointOrigin, "://")[0]
			allowedScheme := strings.Split(allowed, "://")[0]
			if endpointScheme == allowedScheme {
				return nil
			}
		}
	}

	return mcperrors.NewError(
		mcperrors.CodeValidationError,
		"Server origin not allowed",
		mcperrors.CategoryValidation,
		mcperrors.SeverityError,
	).WithDetail("Endpoint origin '" + endpointOrigin + "' is not in allowed origins list")
}

// isLocalhostOrigin checks if the origin is from localhost (for client-side validation)
func (t *StreamableHTTPTransport) isLocalhostOrigin(origin string) bool {
	// Extract the host part from the origin for comparison
	var host string
	if strings.HasPrefix(origin, "https://") {
		host = strings.TrimPrefix(origin, "https://")
	} else if strings.HasPrefix(origin, "http://") {
		host = strings.TrimPrefix(origin, "http://")
	} else {
		return false
	}

	// Handle IPv6 brackets first
	if strings.HasPrefix(host, "[") {
		// IPv6 address with brackets - extract the address and handle port separately
		endBracket := strings.Index(host, "]")
		if endBracket != -1 {
			ipv6Host := host[1:endBracket] // Extract IPv6 address without brackets
			host = ipv6Host
		}
	} else {
		// IPv4 or hostname - remove port if present
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
	}

	// Check if it's a localhost representation
	localhostHosts := []string{
		"localhost",
		"127.0.0.1",
		"::1",
	}

	for _, localhost := range localhostHosts {
		if host == localhost {
			return true
		}
	}

	return false
}

// Call executes the given function respecting the circuit breaker state
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return cb.callClosed(fn)
	case CircuitOpen:
		return cb.callOpen(fn)
	case CircuitHalfOpen:
		return cb.callHalfOpen(fn)
	default:
		return fmt.Errorf("unknown circuit breaker state: %v", cb.state)
	}
}

// callClosed handles calls when the circuit is closed
func (cb *CircuitBreaker) callClosed(fn func() error) error {
	err := fn()
	if err != nil {
		cb.recordFailure()
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.lastFailureTime = time.Now()
		}
		return err
	}
	cb.recordSuccess()
	return nil
}

// callOpen handles calls when the circuit is open
func (cb *CircuitBreaker) callOpen(fn func() error) error {
	if time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
		cb.state = CircuitHalfOpen
		cb.halfOpenCalls = 0
		return cb.callHalfOpen(fn)
	}
	return fmt.Errorf("circuit breaker is open")
}

// callHalfOpen handles calls when the circuit is half-open
func (cb *CircuitBreaker) callHalfOpen(fn func() error) error {
	if cb.halfOpenCalls >= cb.halfOpenMaxCalls {
		return fmt.Errorf("circuit breaker half-open limit exceeded")
	}

	cb.halfOpenCalls++
	err := fn()
	if err != nil {
		cb.recordFailure()
		cb.state = CircuitOpen
		cb.lastFailureTime = time.Now()
		return err
	}

	cb.recordSuccess()
	if cb.halfOpenCalls >= cb.halfOpenMaxCalls {
		cb.state = CircuitClosed
		cb.failureCount = 0
	}
	return nil
}

// recordSuccess records a successful operation
func (cb *CircuitBreaker) recordSuccess() {
	cb.lastSuccessTime = time.Now()
	cb.failureCount = 0
}

// recordFailure records a failed operation
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()
}

// State returns the current circuit breaker state
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// classifyError determines whether an error should be retried
func classifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeRetryable // This shouldn't happen, but be safe
	}

	// Check for specific error types
	errStr := err.Error()

	// Network errors that should be retried
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "EOF") {
		return ErrorTypeRetryable
	}

	// HTTP client errors that should NOT be retried
	if strings.Contains(errStr, "HTTP error 4") {
		return ErrorTypeNonRetryable
	}

	// HTTP server errors that should be retried with backoff
	if strings.Contains(errStr, "HTTP error 5") {
		return ErrorTypeBackoff
	}

	// Circuit breaker errors
	if strings.Contains(errStr, "circuit breaker") {
		return ErrorTypeCircuitBreaker
	}

	// Default to retryable for unknown errors
	return ErrorTypeRetryable
}

// cryptoRandFloat64 generates a cryptographically secure random float64 in [0, 1)
func cryptoRandFloat64() (float64, error) {
	// Generate a random integer in [0, 2^53)
	max := big.NewInt(1 << 53)
	n, err := cryptorand.Int(cryptorand.Reader, max)
	if err != nil {
		return 0, err
	}
	// Convert to float64 in [0, 1)
	return float64(n.Int64()) / float64(1<<53), nil
}

// calculateBackoffWithJitter computes backoff duration with jitter to prevent thundering herd
// If the SSE stream has specified a retry interval, use that instead of exponential backoff
func (t *StreamableHTTPTransport) calculateBackoffWithJitter(attempt int, es *StreamableEventSource) time.Duration {
	config := t.reconnectConfig

	// Check if SSE server provided a retry interval (per SSE specification)
	if es != nil {
		es.mu.Lock()
		sseRetryInterval := es.retryInterval
		es.mu.Unlock()

		if sseRetryInterval > 0 {
			// Use server-specified retry interval with minimal jitter for compliance
			jitter := time.Duration(0)
			if config.JitterPercent > 0 {
				jitterRange := float64(sseRetryInterval) * (config.JitterPercent / 2) / 100.0 // Reduced jitter for SSE
				if randFloat, err := cryptoRandFloat64(); err == nil {
					jitter = time.Duration(randFloat * jitterRange)
				}
			}
			return sseRetryInterval + jitter
		}
	}

	// Fallback to standard exponential backoff if no SSE retry interval
	// Calculate base backoff using exponential backoff
	backoff := time.Duration(float64(config.InitialBackoff) *
		math.Pow(config.BackoffMultiplier, float64(attempt)))

	// Cap at maximum backoff
	if backoff > config.MaxBackoff {
		backoff = config.MaxBackoff
	}

	// Add jitter to prevent synchronized reconnection attempts
	if config.JitterPercent > 0 {
		jitterRange := float64(backoff) * config.JitterPercent / 100.0
		if randFloat, err := cryptoRandFloat64(); err == nil {
			jitter := time.Duration(randFloat * jitterRange)
			backoff += jitter
		}
	}

	return backoff
}

// updateConnectionState safely updates the connection state
func (t *StreamableHTTPTransport) updateConnectionState(newState ConnectionState) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connectionState != newState {
		t.logger.Printf("[CONNECTION] State changed: %s -> %s",
			t.connectionState.String(), newState.String())
		t.connectionState = newState
	}
}

// recordReconnectionAttempt tracks reconnection metrics
func (t *StreamableHTTPTransport) recordReconnectionAttempt(success bool, duration time.Duration) {
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()

	t.metrics.attempts++
	t.metrics.lastAttempt = time.Now()
	t.metrics.totalDuration += duration

	if success {
		t.metrics.successes++
	} else {
		t.metrics.failures++
	}
}

// GetReconnectionMetrics returns a copy of the current metrics
func (t *StreamableHTTPTransport) GetReconnectionMetrics() ReconnectionMetrics {
	t.metrics.mu.RLock()
	defer t.metrics.mu.RUnlock()

	return ReconnectionMetrics{
		attempts:      t.metrics.attempts,
		successes:     t.metrics.successes,
		failures:      t.metrics.failures,
		totalDuration: t.metrics.totalDuration,
		lastAttempt:   t.metrics.lastAttempt,
	}
}

// SetReconnectionConfig allows customizing reconnection behavior
func (t *StreamableHTTPTransport) SetReconnectionConfig(config *ReconnectionConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reconnectConfig = config
}

// GetReconnectionConfig returns the current reconnection configuration
func (t *StreamableHTTPTransport) GetReconnectionConfig() *ReconnectionConfig {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reconnectConfig
}

// extractOriginFromURL extracts the origin (scheme + host + port) from a URL
func (t *StreamableHTTPTransport) extractOriginFromURL(urlStr string) (string, error) {
	if strings.HasPrefix(urlStr, "https://") {
		hostPath := strings.TrimPrefix(urlStr, "https://")
		// Handle IPv6 addresses with brackets
		if strings.HasPrefix(hostPath, "[") {
			// Find the closing bracket
			endBracket := strings.Index(hostPath, "]")
			if endBracket == -1 {
				return "", mcperrors.NewError(
					mcperrors.CodeValidationError,
					"Invalid IPv6 URL format",
					mcperrors.CategoryValidation,
					mcperrors.SeverityError,
				).WithDetail("Missing closing bracket for IPv6 address")
			}
			// Extract everything up to the first slash after the bracket (or end of string)
			remaining := hostPath[endBracket+1:]
			if strings.Contains(remaining, "/") {
				remaining = remaining[:strings.Index(remaining, "/")]
			}
			return "https://" + hostPath[:endBracket+1] + remaining, nil
		} else {
			// Regular hostname or IPv4
			host := strings.Split(hostPath, "/")[0]
			return "https://" + host, nil
		}
	} else if strings.HasPrefix(urlStr, "http://") {
		hostPath := strings.TrimPrefix(urlStr, "http://")
		// Handle IPv6 addresses with brackets
		if strings.HasPrefix(hostPath, "[") {
			// Find the closing bracket
			endBracket := strings.Index(hostPath, "]")
			if endBracket == -1 {
				return "", mcperrors.NewError(
					mcperrors.CodeValidationError,
					"Invalid IPv6 URL format",
					mcperrors.CategoryValidation,
					mcperrors.SeverityError,
				).WithDetail("Missing closing bracket for IPv6 address")
			}
			// Extract everything up to the first slash after the bracket (or end of string)
			remaining := hostPath[endBracket+1:]
			if strings.Contains(remaining, "/") {
				remaining = remaining[:strings.Index(remaining, "/")]
			}
			return "http://" + hostPath[:endBracket+1] + remaining, nil
		} else {
			// Regular hostname or IPv4
			host := strings.Split(hostPath, "/")[0]
			return "http://" + host, nil
		}
	} else {
		return "", mcperrors.NewError(
			mcperrors.CodeValidationError,
			"Invalid endpoint URL format",
			mcperrors.CategoryValidation,
			mcperrors.SeverityError,
		).WithDetail("Endpoint must start with http:// or https://")
	}
}

// Reliability statistics will be available via middleware when implemented
