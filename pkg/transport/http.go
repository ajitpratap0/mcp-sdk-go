package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// HTTPTransport implements Transport using HTTP with Server-Sent Events (SSE)
type HTTPTransport struct {
	*BaseTransport
	serverURL       string
	client          *http.Client
	eventSource     *EventSource
	running         atomic.Bool
	options         *Options
	headers         map[string]string
	requestIDPrefix string
	mu              sync.Mutex
}

// EventSource is a client for Server-Sent Events
type EventSource struct {
	URL         string
	Headers     map[string]string
	Client      *http.Client
	Connection  *http.Response
	MessageChan chan []byte
	ErrorChan   chan error
	CloseChan   chan struct{}
	mu          sync.Mutex
	isConnected atomic.Bool
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(serverURL string, options ...Option) *HTTPTransport {
	opts := NewOptions(options...)

	client := &http.Client{
		Timeout: opts.RequestTimeout,
	}

	return &HTTPTransport{
		BaseTransport:   NewBaseTransport(),
		serverURL:       serverURL,
		client:          client,
		options:         opts,
		headers:         make(map[string]string),
		requestIDPrefix: "http",
	}
}

// SetRequestIDPrefix sets the prefix for request IDs
func (t *HTTPTransport) SetRequestIDPrefix(prefix string) {
	t.requestIDPrefix = prefix
}

// SetHeader sets a HTTP header for all requests
func (t *HTTPTransport) SetHeader(key, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.headers[key] = value
}

// Initialize sets up the HTTP transport
func (t *HTTPTransport) Initialize(ctx context.Context) error {
	// Create event source for SSE
	t.eventSource = &EventSource{
		URL:         fmt.Sprintf("%s/events", t.serverURL),
		Headers:     t.headers,
		Client:      t.client,
		MessageChan: make(chan []byte, 100),
		ErrorChan:   make(chan error, 10),
		CloseChan:   make(chan struct{}),
	}

	return nil
}

// SendRequest sends a request and waits for the response
func (t *HTTPTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	id := fmt.Sprintf("%s-%d", t.requestIDPrefix, t.GetNextID())

	req, err := protocol.NewRequest(id, method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, t.options.RequestTimeout)
	defer cancel()

	// Send HTTP request
	if err := t.sendHTTPRequest(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response through SSE channel
	resp, err := t.WaitForResponse(reqCtx, id)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("server error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	return resp.Result, nil
}

// SendNotification sends a notification (one-way message)
func (t *HTTPTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	notif, err := protocol.NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return t.sendHTTPRequest(notif)
}

// Start begins processing messages (blocking)
func (t *HTTPTransport) Start(ctx context.Context) error {
	if !t.running.CompareAndSwap(false, true) {
		return fmt.Errorf("transport already running")
	}

	defer t.running.Store(false)

	// Connect to SSE endpoint
	if err := t.eventSource.Connect(); err != nil {
		return fmt.Errorf("failed to connect to event source: %w", err)
	}

	// Process incoming SSE messages
	for {
		select {
		case <-ctx.Done():
			t.eventSource.Close()
			return ctx.Err()

		case err := <-t.eventSource.ErrorChan:
			// Try to reconnect on error
			t.eventSource.Close()
			if reconnectErr := t.eventSource.Connect(); reconnectErr != nil {
				return fmt.Errorf("failed to reconnect after error %v: %w", err, reconnectErr)
			}

		case data := <-t.eventSource.MessageChan:
			if err := t.handleMessage(ctx, data); err != nil {
				// Just log errors, don't stop processing
				fmt.Printf("Error handling message: %v\n", err)
			}
		}
	}
}

// Stop gracefully shuts down the transport
func (t *HTTPTransport) Stop(ctx context.Context) error {
	if t.eventSource != nil {
		t.eventSource.Close()
	}
	return nil
}

// sendHTTPRequest sends a JSON-RPC message via HTTP POST
func (t *HTTPTransport) sendHTTPRequest(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequest("POST", t.serverURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// handleMessage processes an incoming JSON-RPC message from the SSE stream
func (t *HTTPTransport) handleMessage(ctx context.Context, data []byte) error {
	if protocol.IsRequest(data) {
		var req protocol.Request
		if err := json.Unmarshal(data, &req); err != nil {
			return fmt.Errorf("failed to unmarshal request: %w", err)
		}

		resp, err := t.HandleRequest(ctx, &req)
		if err != nil {
			errResp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
			return t.sendHTTPRequest(errResp)
		}

		return t.sendHTTPRequest(resp)

	} else if protocol.IsResponse(data) {
		var resp protocol.Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}

		t.HandleResponse(&resp)
		return nil

	} else if protocol.IsNotification(data) {
		var notif protocol.Notification
		if err := json.Unmarshal(data, &notif); err != nil {
			return fmt.Errorf("failed to unmarshal notification: %w", err)
		}

		return t.HandleNotification(ctx, &notif)

	} else {
		return fmt.Errorf("unknown message type: %s", string(data))
	}
}

// Connect establishes a connection to the SSE endpoint
func (es *EventSource) Connect() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.isConnected.Load() {
		return nil // Already connected
	}

	req, err := http.NewRequest("GET", es.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for key, value := range es.Headers {
		req.Header.Set(key, value)
	}

	resp, err := es.Client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			return fmt.Errorf("error closing response body: %w", err)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	es.Connection = resp
	es.isConnected.Store(true)

	// Start reading events
	go es.readEvents()

	return nil
}

// Close terminates the SSE connection
func (es *EventSource) Close() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.isConnected.Load() {
		return
	}

	// Signal close to readEvents goroutine
	close(es.CloseChan)

	// Close the connection
	if es.Connection != nil && es.Connection.Body != nil {
		if err := es.Connection.Body.Close(); err != nil {
			// Just log the error as we're already in a cleanup path
			fmt.Printf("Error closing connection body: %v\n", err)
		}
		es.Connection = nil
	}

	es.isConnected.Store(false)
}

// readEvents processes the SSE stream
func (es *EventSource) readEvents() {
	defer func() {
		if es.Connection != nil && es.Connection.Body != nil {
			if err := es.Connection.Body.Close(); err != nil {
				// Just log the error as we're already in a cleanup path
				fmt.Printf("Error closing connection body: %v\n", err)
			}
		}
		es.isConnected.Store(false)
	}()

	reader := bufio.NewReader(es.Connection.Body)
	var buffer bytes.Buffer

	for {
		select {
		case <-es.CloseChan:
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				select {
				case es.ErrorChan <- fmt.Errorf("read error: %w", err):
				default:
					// Error channel is full, skip
				}
				return
			}

			// Empty line marks the end of an event
			if len(line) <= 2 { // Just \r\n or \n
				if buffer.Len() > 0 {
					data := buffer.Bytes()
					buffer.Reset()

					// Skip comments and other SSE-specific lines
					if len(data) > 0 && data[0] != ':' {
						es.MessageChan <- data
					}
				}
				continue
			}

			// Check for "data:" prefix and append to buffer
			if bytes.HasPrefix(line, []byte("data:")) {
				buffer.Write(line[5:]) // Skip "data:"
			}
		}
	}
}

// Send transmits a message over the transport.
// For HTTPTransport, this is not fully applicable since it operates in a request/response model.
// This is here to satisfy the Transport interface.
func (t *HTTPTransport) Send(data []byte) error {
	return fmt.Errorf("Send method not applicable for HTTPTransport")
}

// SetReceiveHandler sets the handler for received messages.
func (t *HTTPTransport) SetReceiveHandler(handler ReceiveHandler) {
	// This is a placeholder implementation
}

// SetErrorHandler sets the handler for transport errors.
func (t *HTTPTransport) SetErrorHandler(handler ErrorHandler) {
	// This is a placeholder implementation
}
