package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"golang.org/x/sync/errgroup"
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
		return nil, mcperrors.WrapProtocolError(err, method, id)
	}

	reqCtx, cancel := context.WithTimeout(ctx, t.options.RequestTimeout)
	defer cancel()

	// Send HTTP request
	if err := t.sendHTTPRequest(req); err != nil {
		return nil, mcperrors.HTTPTransportError("send_request", t.serverURL, 0, err).
			WithContext(&mcperrors.Context{
				RequestID: id,
				Method:    method,
				Component: "HTTPTransport",
				Operation: "send_request",
			})
	}

	// Wait for response through SSE channel
	resp, err := t.WaitForResponse(reqCtx, id)
	if err != nil {
		return nil, mcperrors.ResponseTimeout("http", id, t.options.RequestTimeout).
			WithContext(&mcperrors.Context{
				RequestID: id,
				Method:    method,
				Component: "HTTPTransport",
				Operation: "wait_response",
			})
	}

	if resp.Error != nil {
		return nil, mcperrors.NewError(
			int(resp.Error.Code),
			resp.Error.Message,
			mcperrors.CategoryProtocol,
			mcperrors.SeverityError,
		).WithContext(&mcperrors.Context{
			RequestID: id,
			Method:    method,
			Component: "HTTPTransport",
			Operation: "process_response",
		}).WithData(resp.Error.Data)
	}

	return resp.Result, nil
}

// SendNotification sends a notification (one-way message)
func (t *HTTPTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	notif, err := protocol.NewNotification(method, params)
	if err != nil {
		return mcperrors.WrapProtocolError(err, method, nil)
	}

	return t.sendHTTPRequest(notif)
}

// Start begins processing messages (blocking)
func (t *HTTPTransport) Start(ctx context.Context) error {
	if !t.running.CompareAndSwap(false, true) {
		return mcperrors.TransportAlreadyRunning("http")
	}

	defer t.running.Store(false)

	// Connect to SSE endpoint
	if err := t.eventSource.Connect(); err != nil {
		return mcperrors.ConnectionFailed("http", t.eventSource.URL, err).
			WithContext(&mcperrors.Context{
				Component: "HTTPTransport",
				Operation: "connect_sse",
			})
	}

	// Create errgroup for coordinated goroutine management
	g, gctx := errgroup.WithContext(ctx)

	// Start message processing goroutine
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return gctx.Err()
			case data, ok := <-t.eventSource.MessageChan:
				if !ok {
					return nil // Channel closed
				}
				if err := t.handleMessage(gctx, data); err != nil {
					// Log error but continue processing
					fmt.Printf("Error handling message: %v\n", err)
				}
			}
		}
	})

	// Start error handling and reconnection goroutine
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return gctx.Err()
			case err, ok := <-t.eventSource.ErrorChan:
				if !ok {
					return nil // Channel closed
				}

				// Check if we should still be running
				if !t.running.Load() {
					return nil
				}

				// Log the error
				fmt.Printf("EventSource error: %v. Attempting to reconnect...\n", err)

				// Try to reconnect on error
				t.eventSource.Close()

				// Introduce a delay before reconnecting with context check
				select {
				case <-gctx.Done():
					return gctx.Err()
				case <-time.After(5 * time.Second):
					// Continue with reconnection
				}

				// Check again if we should still be running after the delay
				if !t.running.Load() {
					return nil
				}

				if reconnectErr := t.eventSource.Connect(); reconnectErr != nil {
					return mcperrors.ConnectionFailed("http", t.eventSource.URL, reconnectErr).
						WithContext(&mcperrors.Context{
							Component: "HTTPTransport",
							Operation: "reconnect_sse",
						}).WithDetail(fmt.Sprintf("Previous error: %v", err))
				}
				fmt.Println("EventSource reconnected successfully.")
			}
		}
	})

	// Wait for all goroutines to complete or for any error
	err := g.Wait()
	t.eventSource.Close()
	return err
}

// Stop gracefully shuts down the transport
func (t *HTTPTransport) Stop(ctx context.Context) error {
	t.running.Store(false)

	// Close event source and wait for goroutines to finish
	if t.eventSource != nil {
		t.eventSource.Close()

		// Give time for readEvents goroutine to finish
		select {
		case <-ctx.Done():
			// Context cancelled, continue with cleanup
		case <-time.After(500 * time.Millisecond):
			// Timeout waiting for goroutines, continue with cleanup
		}
	}

	// Clear headers map
	t.mu.Lock()
	for k := range t.headers {
		delete(t.headers, k)
	}
	t.mu.Unlock()

	// Clean up BaseTransport resources
	t.BaseTransport.Cleanup()

	return nil
}

// sendHTTPRequest sends a JSON-RPC message via HTTP POST
func (t *HTTPTransport) sendHTTPRequest(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return mcperrors.CreateInternalError("marshal_message", err).
			WithContext(&mcperrors.Context{
				Component: "HTTPTransport",
				Operation: "marshal_message",
			})
	}

	req, err := http.NewRequest("POST", t.serverURL, bytes.NewBuffer(data))
	if err != nil {
		return mcperrors.HTTPTransportError("create_request", t.serverURL, 0, err).
			WithContext(&mcperrors.Context{
				Component: "HTTPTransport",
				Operation: "create_http_request",
			})
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return mcperrors.HTTPTransportError("send_http_request", t.serverURL, 0, err).
			WithContext(&mcperrors.Context{
				Component: "HTTPTransport",
				Operation: "execute_http_request",
			})
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return mcperrors.HTTPTransportError("http_error_response", t.serverURL, resp.StatusCode, errors.New(string(body))).
			WithContext(&mcperrors.Context{
				Component: "HTTPTransport",
				Operation: "process_http_response",
			})
	}

	return nil
}

// handleMessage processes an incoming JSON-RPC message from the SSE stream
func (t *HTTPTransport) handleMessage(ctx context.Context, data []byte) error {
	if protocol.IsRequest(data) {
		var req protocol.Request
		if err := json.Unmarshal(data, &req); err != nil {
			return mcperrors.CreateInternalError("unmarshal_request", err).
				WithContext(&mcperrors.Context{
					Component: "HTTPTransport",
					Operation: "unmarshal_request",
				})
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
			return mcperrors.CreateInternalError("unmarshal_response", err).
				WithContext(&mcperrors.Context{
					Component: "HTTPTransport",
					Operation: "unmarshal_response",
				})
		}

		t.HandleResponse(&resp)
		return nil

	} else if protocol.IsNotification(data) {
		var notif protocol.Notification
		if err := json.Unmarshal(data, &notif); err != nil {
			return mcperrors.CreateInternalError("unmarshal_notification", err).
				WithContext(&mcperrors.Context{
					Component: "HTTPTransport",
					Operation: "unmarshal_notification",
				})
		}

		return t.HandleNotification(ctx, &notif)

	} else {
		return mcperrors.NewError(
			mcperrors.CodeProtocolError,
			"Unknown message type received",
			mcperrors.CategoryProtocol,
			mcperrors.SeverityError,
		).WithContext(&mcperrors.Context{
			Component: "HTTPTransport",
			Operation: "process_message",
		}).WithDetail(fmt.Sprintf("Data: %s", string(data)))
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
		return mcperrors.HTTPTransportError("create_sse_request", es.URL, 0, err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for key, value := range es.Headers {
		req.Header.Set(key, value)
	}

	resp, err := es.Client.Do(req)
	if err != nil {
		return mcperrors.ConnectionFailed("http", es.URL, err)
	}

	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			return mcperrors.HTTPTransportError("close_response_body", es.URL, resp.StatusCode, err)
		}
		return mcperrors.HTTPTransportError("unexpected_status", es.URL, resp.StatusCode, fmt.Errorf("unexpected status code: %d", resp.StatusCode))
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		if err := resp.Body.Close(); err != nil {
			return mcperrors.HTTPTransportError("close_response_body", es.URL, resp.StatusCode, err)
		}
		return mcperrors.HTTPTransportError("invalid_content_type", es.URL, resp.StatusCode,
			fmt.Errorf("expected content-type 'text/event-stream', got '%s'", contentType))
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

	// Signal close to readEvents goroutine - protect against double close
	select {
	case <-es.CloseChan:
		// Already closed
	default:
		close(es.CloseChan)
	}

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
				case es.ErrorChan <- mcperrors.EventSourceError(es.URL, "read error", err):
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
	return mcperrors.NewError(
		mcperrors.CodeOperationNotSupported,
		"Send method not applicable for HTTPTransport",
		mcperrors.CategoryValidation,
		mcperrors.SeverityError,
	).WithContext(&mcperrors.Context{
		Component: "HTTPTransport",
		Operation: "send",
	})
}

// SetReceiveHandler sets the handler for received messages.
func (t *HTTPTransport) SetReceiveHandler(handler ReceiveHandler) {
	// This is a placeholder implementation
}

// SetErrorHandler sets the handler for transport errors.
func (t *HTTPTransport) SetErrorHandler(handler ErrorHandler) {
	// This is a placeholder implementation
}
