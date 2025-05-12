// Package transport provides various transport mechanisms for MCP communication.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
	"log"
	
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

	// SetReceiveHandler sets the handler for received messages.
	SetReceiveHandler(handler ReceiveHandler)

	// SetErrorHandler sets the handler for transport errors.
	SetErrorHandler(handler ErrorHandler)

	// Update the HandleRequest method signature
    HandleRequest(ctx context.Context, req *protocol.Request) (*protocol.Response, error)
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
	pendingRequests      map[interface{}]chan *protocol.Response
}

// NewBaseTransport creates a new BaseTransport
func NewBaseTransport() *BaseTransport {
	return &BaseTransport{
		requestHandlers:      make(map[string]RequestHandler),
		notificationHandlers: make(map[string]NotificationHandler),
		progressHandlers:     make(map[string]ProgressHandler),
		nextID:               1,
		pendingRequests:      make(map[interface{}]chan *protocol.Response),
	}
}

// RegisterRequestHandler registers a handler for incoming requests
func (t *BaseTransport) RegisterRequestHandler(method string, handler RequestHandler) {
    log.Printf("Registering request handler: method=%s", method)
    t.Lock()
    defer t.Unlock()
    t.requestHandlers[method] = handler
}

// RegisterNotificationHandler registers a handler for incoming notifications
func (t *BaseTransport) RegisterNotificationHandler(method string, handler NotificationHandler) {
    log.Printf("Registering notification handler: method=%s", method)
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
	ch := make(chan *protocol.Response, 1)

	t.Lock()
	t.pendingRequests[id] = ch
	t.Unlock()

	defer func() {
		t.Lock()
		delete(t.pendingRequests, id)
		t.Unlock()
	}()

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleResponse handles an incoming response
func (t *BaseTransport) HandleResponse(resp *protocol.Response) {
    log.Printf("Handling response: ID=%v", resp.ID)
    t.RLock()
    ch, ok := t.pendingRequests[resp.ID]
    t.RUnlock()

    if ok {
        log.Printf("Found pending request for ID=%v, sending response", resp.ID)
        select {
        case ch <- resp:
        default:
            log.Printf("Response channel for ID=%v is full or closed", resp.ID)
        }
    } else {
        log.Printf("No pending request found for ID=%v", resp.ID)
    }
}

// HandleRequest processes an incoming request
func (t *BaseTransport) HandleRequest(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
    log.Printf("Handling request: method=%s, ID=%v", req.Method, req.ID)
    t.RLock()
    handler, ok := t.requestHandlers[req.Method]
    t.RUnlock()

    if !ok {
        log.Printf("No handler found for method=%s", req.Method)
        return protocol.NewErrorResponse(req.ID, protocol.MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method), nil)
    }

    log.Printf("Calling handler for method=%s", req.Method)
    result, err := handler(ctx, req.Params)
    if err != nil {
        log.Printf("Handler for method=%s returned an error: %v", req.Method, err)
        return protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
    }

    log.Printf("Handler for method=%s returned result: %v", req.Method, result)
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
