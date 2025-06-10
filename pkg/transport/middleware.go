package transport

import (
	"context"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// Middleware represents a transport middleware that can wrap a transport
// to add additional functionality like reliability, observability, etc.
type Middleware interface {
	// Wrap wraps the given transport with middleware functionality
	Wrap(transport Transport) Transport
}

// MiddlewareFunc is an adapter to allow the use of ordinary functions as middleware
type MiddlewareFunc func(Transport) Transport

// Wrap implements the Middleware interface
func (f MiddlewareFunc) Wrap(t Transport) Transport {
	return f(t)
}

// ChainMiddleware chains multiple middleware together
func ChainMiddleware(middleware ...Middleware) Middleware {
	return MiddlewareFunc(func(transport Transport) Transport {
		// Apply middleware in reverse order so the first middleware is the outermost
		for i := len(middleware) - 1; i >= 0; i-- {
			transport = middleware[i].Wrap(transport)
		}
		return transport
	})
}

// middlewareTransport is a base type for middleware implementations
type middlewareTransport struct {
	next Transport
}

// Initialize delegates to the wrapped transport
func (m *middlewareTransport) Initialize(ctx context.Context) error {
	return m.next.Initialize(ctx)
}

// SendRequest delegates to the wrapped transport
func (m *middlewareTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	return m.next.SendRequest(ctx, method, params)
}

// SendNotification delegates to the wrapped transport
func (m *middlewareTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	return m.next.SendNotification(ctx, method, params)
}

// Start delegates to the wrapped transport
func (m *middlewareTransport) Start(ctx context.Context) error {
	return m.next.Start(ctx)
}

// Stop delegates to the wrapped transport
func (m *middlewareTransport) Stop(ctx context.Context) error {
	return m.next.Stop(ctx)
}

// RegisterRequestHandler delegates to the wrapped transport
func (m *middlewareTransport) RegisterRequestHandler(method string, handler RequestHandler) {
	m.next.RegisterRequestHandler(method, handler)
}

// RegisterNotificationHandler delegates to the wrapped transport
func (m *middlewareTransport) RegisterNotificationHandler(method string, handler NotificationHandler) {
	m.next.RegisterNotificationHandler(method, handler)
}

// RegisterProgressHandler delegates to the wrapped transport
func (m *middlewareTransport) RegisterProgressHandler(id interface{}, handler ProgressHandler) {
	m.next.RegisterProgressHandler(id, handler)
}

// UnregisterProgressHandler delegates to the wrapped transport
func (m *middlewareTransport) UnregisterProgressHandler(id interface{}) {
	m.next.UnregisterProgressHandler(id)
}

// HandleResponse delegates to the wrapped transport
func (m *middlewareTransport) HandleResponse(response *protocol.Response) {
	m.next.HandleResponse(response)
}

// HandleRequest delegates to the wrapped transport
func (m *middlewareTransport) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	return m.next.HandleRequest(ctx, request)
}

// HandleNotification delegates to the wrapped transport
func (m *middlewareTransport) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	return m.next.HandleNotification(ctx, notification)
}

// GenerateID delegates to the wrapped transport
func (m *middlewareTransport) GenerateID() string {
	return m.next.GenerateID()
}

// GetRequestIDPrefix delegates to the wrapped transport
func (m *middlewareTransport) GetRequestIDPrefix() string {
	return m.next.GetRequestIDPrefix()
}

// GetNextID delegates to the wrapped transport
func (m *middlewareTransport) GetNextID() int64 {
	return m.next.GetNextID()
}

// Cleanup delegates to the wrapped transport
func (m *middlewareTransport) Cleanup() {
	m.next.Cleanup()
}

// MiddlewareBuilder builds middleware from configuration
type MiddlewareBuilder struct {
	config TransportConfig
}

// NewMiddlewareBuilder creates a new middleware builder
func NewMiddlewareBuilder(config TransportConfig) *MiddlewareBuilder {
	return &MiddlewareBuilder{config: config}
}

// Build constructs the middleware chain based on configuration
func (mb *MiddlewareBuilder) Build() []Middleware {
	var middleware []Middleware

	// Order matters - innermost middleware first
	if mb.config.Features.EnableReliability {
		middleware = append(middleware, NewReliabilityMiddleware(mb.config.Reliability))
	}

	if mb.config.Features.EnableObservability {
		middleware = append(middleware, NewObservabilityMiddleware(mb.config.Observability))
	}

	return middleware
}
