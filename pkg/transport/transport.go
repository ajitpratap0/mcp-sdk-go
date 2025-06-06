// Package transport provides various transport mechanisms for MCP communication.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/logging"
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
	logAdapter           *logging.TransportAdapter // Structured logging adapter
}

// NewBaseTransport creates a new BaseTransport
func NewBaseTransport() *BaseTransport {
	// Create structured logger
	structuredLogger := logging.New(nil, logging.NewTextFormatter()).WithFields(
		logging.String("component", "transport"),
	)
	structuredLogger.SetLevel(logging.InfoLevel)

	// Create transport adapter
	logAdapter := logging.NewTransportAdapter(structuredLogger, "BaseTransport")

	return &BaseTransport{
		requestHandlers:      make(map[string]RequestHandler),
		notificationHandlers: make(map[string]NotificationHandler),
		progressHandlers:     make(map[string]ProgressHandler),
		nextID:               1,
		pendingRequests:      make(map[string]chan *protocol.Response),
		logger:               log.New(os.Stderr, "BaseTransport: ", log.LstdFlags|log.Lshortfile),
		logAdapter:           logAdapter,
	}
}

// SetLogger sets a custom logger for the BaseTransport.
func (t *BaseTransport) SetLogger(logger *log.Logger) {
	t.Lock()
	defer t.Unlock()
	t.logger = logger
}

// SetStructuredLogger sets a structured logger for the BaseTransport.
func (t *BaseTransport) SetStructuredLogger(logger logging.Logger, transportName string) {
	t.Lock()
	defer t.Unlock()
	t.logAdapter = logging.NewTransportAdapter(logger, transportName)
}

// Logf logs a formatted string using the transport's logger.
func (t *BaseTransport) Logf(format string, v ...interface{}) {
	t.RLock()
	logAdapter := t.logAdapter
	t.RUnlock()
	if logAdapter != nil {
		// Use structured logging adapter
		logAdapter.Logf(format, v...)
	} else {
		// Fallback to standard logger
		t.RLock()
		logger := t.logger
		t.RUnlock()
		if logger != nil {
			logger.Printf(format, v...)
		}
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

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleResponse handles an incoming response
func (t *BaseTransport) HandleResponse(resp *protocol.Response) {
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
func (t *BaseTransport) HandleRequest(ctx context.Context, req *protocol.Request) (response *protocol.Response, err error) {
	// Recover from panics and convert to proper error response
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			t.Logf("ERROR: Panic in HandleRequest for method %s: %v\nStack trace:\n%s", req.Method, r, stackTrace)

			// Create structured panic recovery error with backward-compatible message
			message := fmt.Sprintf("Internal server error processing %s", req.Method)
			panicErr := mcperrors.NewError(
				mcperrors.CodeInternalError,
				message,
				mcperrors.CategoryInternal,
				mcperrors.SeverityError,
			).WithContext(&mcperrors.Context{
				RequestID: fmt.Sprintf("%v", req.ID),
				Method:    req.Method,
				Component: "BaseTransport",
				Operation: "handle_request",
			}).WithDetail(fmt.Sprintf("Panic: %v, Stack trace: %s", r, stackTrace))

			response, err = mcperrors.ToJSONRPCResponse(panicErr, req.ID)
		}
	}()

	t.RLock()
	handler, ok := t.requestHandlers[req.Method]
	t.RUnlock()

	if !ok {
		methodNotFoundErr := mcperrors.CreateMethodNotFoundError(req.Method, req.ID)
		return mcperrors.ToJSONRPCResponse(methodNotFoundErr, req.ID)
	}

	// Params are passed through directly to the handler
	// The handler will determine how to interpret the params
	params := req.Params

	result, err := handler(ctx, params)
	if err != nil {
		// Wrap handler errors with context
		wrappedErr := mcperrors.WrapProtocolError(err, req.Method, req.ID)
		return mcperrors.ToJSONRPCResponse(wrappedErr, req.ID)
	}

	return protocol.NewResponse(req.ID, result)
}

// HandleNotification processes an incoming notification
func (t *BaseTransport) HandleNotification(ctx context.Context, notif *protocol.Notification) (err error) {
	// Recover from panics in notification handlers
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			t.Logf("ERROR: Panic in HandleNotification for method %s: %v\nStack trace:\n%s", notif.Method, r, stackTrace)

			// Create structured panic recovery error with backward-compatible message
			message := fmt.Sprintf("internal error processing notification %s: %v", notif.Method, r)
			err = mcperrors.NewError(
				mcperrors.CodeInternalError,
				message,
				mcperrors.CategoryInternal,
				mcperrors.SeverityError,
			).WithContext(&mcperrors.Context{
				Method:    notif.Method,
				Component: "BaseTransport",
				Operation: "handle_notification",
			}).WithDetail(fmt.Sprintf("Stack trace: %s", stackTrace))
		}
	}()

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
		return mcperrors.NewError(
			mcperrors.CodeMethodNotFound,
			fmt.Sprintf("Unsupported notification method: %s", notif.Method),
			mcperrors.CategoryProtocol,
			mcperrors.SeverityError,
		).WithContext(&mcperrors.Context{
			Method:    notif.Method,
			Component: "BaseTransport",
			Operation: "handle_notification",
		})
	}

	return handler(ctx, notif.Params)
}

// DefaultTimeout is the default timeout for requests
const DefaultTimeout = 30 * time.Second

// SafeGo runs a function in a goroutine with panic recovery
func SafeGo(logger func(format string, args ...interface{}), name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stackTrace := string(debug.Stack())
				logger("ERROR: Panic in goroutine %s: %v\nStack trace:\n%s", name, r, stackTrace)
			}
		}()
		fn()
	}()
}

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

// Cleanup cleans up all resources used by BaseTransport
// This should be called when the transport is no longer needed
func (t *BaseTransport) Cleanup() {
	t.Lock()
	defer t.Unlock()

	// Close and remove all pending request channels
	for id, ch := range t.pendingRequests {
		select {
		case <-ch:
			// Channel already has a response, don't close
		default:
			close(ch)
		}
		delete(t.pendingRequests, id)
	}

	// Clear all handler maps
	for k := range t.requestHandlers {
		delete(t.requestHandlers, k)
	}
	for k := range t.notificationHandlers {
		delete(t.notificationHandlers, k)
	}
	for k := range t.progressHandlers {
		delete(t.progressHandlers, k)
	}

	// Reset ID counter
	t.nextID = 1

	// Note: We don't set logger to nil as it might be used for final cleanup logs
}
