// Package transport provides various transport mechanisms for MCP communication.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// ProgressHandler handles progress notifications for streaming operations
type ProgressHandler func(params interface{}) error

// Transport defines the interface for MCP transport mechanisms.
// Transports are responsible for sending and receiving messages between
// MCP clients and servers.
type Transport interface {
	// Initialize prepares the transport for use.
	Initialize(ctx context.Context) error

	// SendRequest sends a request and returns the response
	SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error)

	// SendNotification sends a notification (one-way message)
	SendNotification(ctx context.Context, method string, params interface{}) error

	// RegisterRequestHandler registers a handler for incoming requests
	RegisterRequestHandler(method string, handler RequestHandler)

	// RegisterNotificationHandler registers a handler for incoming notifications
	RegisterNotificationHandler(method string, handler NotificationHandler)

	// RegisterProgressHandler registers a handler for progress events
	RegisterProgressHandler(id interface{}, handler ProgressHandler)

	// UnregisterProgressHandler removes a progress handler
	UnregisterProgressHandler(id interface{})

	// GenerateID generates a unique ID for requests
	GenerateID() string

	// Start begins reading messages and processing them.
	// This method blocks until the context is canceled or an error occurs.
	Start(ctx context.Context) error

	// Stop halts the transport and cleans up resources.
	Stop(ctx context.Context) error

	// Send transmits a message over the transport.
	Send(data []byte) error

	// SetErrorHandler sets the handler for transport errors.
	SetErrorHandler(handler ErrorHandler)
}

// RequestHandler handles incoming requests
type RequestHandler func(ctx context.Context, params interface{}) (interface{}, error)

// NotificationHandler handles incoming notifications
type NotificationHandler func(ctx context.Context, params interface{}) error

// ErrUnsupportedMethod is returned when a method is not supported
var ErrUnsupportedMethod = errors.New("unsupported method")

// BaseTransport provides common functionality for transports
type BaseTransport struct {
	sync.RWMutex
	requestHandlers      map[string]RequestHandler
	notificationHandlers map[string]NotificationHandler
	progressHandlers     map[string]ProgressHandler
	nextID               int64
	pendingRequests      map[string]chan *protocol.Response
	logger               *log.Logger
}

// NewBaseTransport creates a new BaseTransport
func NewBaseTransport() *BaseTransport {
	return &BaseTransport{
		requestHandlers:      make(map[string]RequestHandler),
		notificationHandlers: make(map[string]NotificationHandler),
		progressHandlers:     make(map[string]ProgressHandler),
		nextID:               1,
		pendingRequests:      make(map[string]chan *protocol.Response),
		logger:               log.New(os.Stderr, "BaseTransport: ", log.LstdFlags|log.Lshortfile),
	}
}

// SetLogger sets a custom logger for the BaseTransport.
func (t *BaseTransport) SetLogger(logger *log.Logger) {
	t.Lock()
	defer t.Unlock()
	t.logger = logger
}

// Logf logs a formatted string using the transport's logger.
func (t *BaseTransport) Logf(format string, v ...interface{}) {
	t.RLock()
	logger := t.logger
	t.RUnlock()
	if logger != nil {
		logger.Printf(format, v...)
	}
}

// RegisterRequestHandler registers a handler for incoming requests
func (t *BaseTransport) RegisterRequestHandler(method string, handler RequestHandler) {
	t.Lock()
	defer t.Unlock()
	t.requestHandlers[method] = handler
}

// RegisterNotificationHandler registers a handler for incoming notifications
func (t *BaseTransport) RegisterNotificationHandler(method string, handler NotificationHandler) {
	t.Lock()
	defer t.Unlock()
	t.notificationHandlers[method] = handler
}

// GetNextID returns the next unique request ID
func (t *BaseTransport) GetNextID() int64 {
	t.Lock()
	defer t.Unlock()
	id := t.nextID
	t.nextID++
	return id
}

// WaitForResponse waits for a response with the specified ID
func (t *BaseTransport) WaitForResponse(ctx context.Context, id interface{}) (*protocol.Response, error) {
	t.Logf("WaitForResponse: Waiting for response with ID '%v'", id)
	ch := make(chan *protocol.Response, 1)

	// Ensure id is a string for map key consistency
	stringID := fmt.Sprintf("%v", id)

	t.Lock()
	t.pendingRequests[stringID] = ch
	t.Unlock()

	defer func() {
		t.Lock()
		delete(t.pendingRequests, stringID)
		t.Unlock()
	}()

	t.Logf("WaitForResponse: Setting up channel for ID '%s'", stringID)
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleResponse handles an incoming response
func (t *BaseTransport) HandleResponse(resp *protocol.Response) {
	t.Logf("HandleResponse: Received response for ID '%v'", resp.ID)
	// Convert response ID to string for consistent map lookup
	stringID := fmt.Sprintf("%v", resp.ID)

	t.RLock()
	ch, ok := t.pendingRequests[stringID]
	pendingReqsCount := len(t.pendingRequests)
	t.RUnlock()

	t.Logf("HandleResponse: Received response for ID '%s' (original: %v, type: %T). Pending requests: %d. Channel found: %t", stringID, resp.ID, resp.ID, pendingReqsCount, ok)

	if ok {
		select {
		case ch <- resp:
			t.Logf("HandleResponse: Successfully sent response for ID '%s' to channel.", stringID)
		default:
			// Response channel is full or closed, or context expired waiting for send
			t.Logf("HandleResponse: Failed to send response for ID '%s' to channel (channel full, closed, or context expired).", stringID)
		}
	} else {
		t.Logf("HandleResponse: No pending request found for response ID '%s'.", stringID)
	}
}

// HandleRequest processes an incoming request
func (t *BaseTransport) HandleRequest(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	t.RLock()
	handler, ok := t.requestHandlers[req.Method]
	t.RUnlock()

	if !ok {
		return protocol.NewErrorResponse(req.ID, protocol.MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method), nil)
	}

	// Params are passed through directly to the handler
	// The handler will determine how to interpret the params
	params := req.Params

	result, err := handler(ctx, params)
	if err != nil {
		return protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
	}

	return protocol.NewResponse(req.ID, result)
}

// HandleNotification processes an incoming notification
func (t *BaseTransport) HandleNotification(ctx context.Context, notif *protocol.Notification) error {
	// Special handling for progress notifications
	if notif.Method == protocol.MethodProgress && len(notif.Params) > 0 {
		// Extract the request ID from the progress params
		var progressParams protocol.ProgressParams
		if err := json.Unmarshal(notif.Params, &progressParams); err == nil && progressParams.ID != nil {
			// Convert ID to string for map lookup
			idStr := fmt.Sprintf("%v", progressParams.ID)

			// Find and call progress handler if registered
			t.RLock()
			handler, ok := t.progressHandlers[idStr]
			t.RUnlock()

			if ok {
				return handler(notif.Params)
			}
		}
	}

	// Regular notification handling
	t.RLock()
	handler, ok := t.notificationHandlers[notif.Method]
	t.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedMethod, notif.Method)
	}

	return handler(ctx, notif.Params)
}

// DefaultTimeout is the default timeout for requests
const DefaultTimeout = 30 * time.Second

// Options contains configuration options for transports
type Options struct {
	RequestTimeout time.Duration
}

// Option is a function that sets an option
type Option func(*Options)

// WithRequestTimeout sets the request timeout
func WithRequestTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.RequestTimeout = timeout
	}
}

// RegisterProgressHandler registers a handler for progress notifications
func (t *BaseTransport) RegisterProgressHandler(id interface{}, handler ProgressHandler) {
	idStr := fmt.Sprintf("%v", id)

	t.Lock()
	defer t.Unlock()
	t.progressHandlers[idStr] = handler
}

// UnregisterProgressHandler removes a progress handler
func (t *BaseTransport) UnregisterProgressHandler(id interface{}) {
	idStr := fmt.Sprintf("%v", id)

	t.Lock()
	defer t.Unlock()
	delete(t.progressHandlers, idStr)
}

// GenerateID generates a unique string ID for requests
func (t *BaseTransport) GenerateID() string {
	return fmt.Sprintf("%d", t.GetNextID())
}

// NewOptions creates default options
func NewOptions(options ...Option) *Options {
	opts := &Options{
		RequestTimeout: DefaultTimeout,
	}

	for _, option := range options {
		option(opts)
	}

	return opts
}
