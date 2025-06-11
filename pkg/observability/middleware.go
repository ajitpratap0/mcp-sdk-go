package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// EnhancedObservabilityMiddleware provides comprehensive observability with OpenTelemetry
type EnhancedObservabilityMiddleware struct {
	config    ObservabilityConfig
	tracer    *TracingProvider
	metrics   MetricsProvider
	transport transport.Transport
}

// ObservabilityConfig configures the enhanced observability middleware
type ObservabilityConfig struct {
	// Tracing configuration
	EnableTracing bool
	TracingConfig TracingConfig

	// Metrics configuration
	EnableMetrics bool
	MetricsConfig MetricsConfig

	// Logging configuration
	EnableLogging bool
	LogLevel      string

	// Feature flags
	CaptureRequestPayload  bool // Capture request payloads in spans
	CaptureResponsePayload bool // Capture response payloads in spans
	RecordPanics           bool // Record panics as span events
}

// NewEnhancedObservabilityMiddleware creates a new enhanced observability middleware
func NewEnhancedObservabilityMiddleware(config ObservabilityConfig) (transport.Middleware, error) {
	var tracer *TracingProvider
	var metrics MetricsProvider

	// Initialize tracing if enabled
	if config.EnableTracing {
		t, err := NewTracingProvider(config.TracingConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create tracing provider: %w", err)
		}
		tracer = t
	}

	// Initialize metrics if enabled (will be implemented next)
	if config.EnableMetrics {
		m, err := NewMetricsProvider(config.MetricsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics provider: %w", err)
		}
		metrics = m
	}

	return &EnhancedObservabilityMiddleware{
		config:  config,
		tracer:  tracer,
		metrics: metrics,
	}, nil
}

// Wrap implements the Middleware interface
func (m *EnhancedObservabilityMiddleware) Wrap(transport transport.Transport) transport.Transport {
	m.transport = transport
	return &observabilityTransport{
		middleware: m,
		next:       transport,
	}
}

// observabilityTransport implements the Transport interface with observability
type observabilityTransport struct {
	middleware *EnhancedObservabilityMiddleware
	next       transport.Transport
}

// SendRequest sends a request with full observability
func (ot *observabilityTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Start span if tracing is enabled
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		var span trace.Span
		ctx, span = ot.middleware.tracer.StartMethodSpan(ctx, method, trace.SpanKindClient)
		defer span.End()

		// Add request attributes
		span.SetAttributes(
			attribute.String("rpc.method", method),
			attribute.String("rpc.system", "mcp"),
			attribute.String("rpc.service", "mcp-client"),
		)

		// Capture request payload if configured
		if ot.middleware.config.CaptureRequestPayload && params != nil {
			if payload, err := json.Marshal(params); err == nil {
				span.SetAttributes(attribute.String("rpc.request.payload", string(payload)))
			}
		}

		// Handle panics
		if ot.middleware.config.RecordPanics {
			defer func() {
				if r := recover(); r != nil {
					span.RecordError(fmt.Errorf("panic: %v", r))
					span.SetStatus(codes.Error, "panic occurred")
					panic(r) // Re-panic after recording
				}
			}()
		}
	}

	// Record metrics start
	start := time.Now()

	// Execute request
	result, err := ot.next.SendRequest(ctx, method, params)

	// Record duration
	duration := time.Since(start)

	// Update metrics if enabled
	if ot.middleware.config.EnableMetrics && ot.middleware.metrics != nil {
		labels := map[string]string{
			"method": method,
			"type":   "request",
		}

		if err != nil {
			labels["status"] = "error"
			labels["error_type"] = getErrorType(err)
			ot.middleware.metrics.RecordRequest(ctx, method, "error", duration)
		} else {
			labels["status"] = "success"
			ot.middleware.metrics.RecordRequest(ctx, method, "success", duration)
		}
	}

	// Update span with response
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.IsRecording() {
			span.SetAttributes(attribute.Float64("rpc.duration_ms", float64(duration.Milliseconds())))

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "")

				// Capture response payload if configured
				if ot.middleware.config.CaptureResponsePayload && result != nil {
					if payload, err := json.Marshal(result); err == nil {
						span.SetAttributes(attribute.String("rpc.response.payload", string(payload)))
					}
				}
			}
		}
	}

	return result, err
}

// SendNotification sends a notification with observability
func (ot *observabilityTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	// Start span if tracing is enabled
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		var span trace.Span
		ctx, span = ot.middleware.tracer.StartMethodSpan(ctx, method, trace.SpanKindClient)
		defer span.End()

		span.SetAttributes(
			attribute.String("rpc.method", method),
			attribute.String("rpc.system", "mcp"),
			attribute.String("rpc.service", "mcp-client"),
			attribute.Bool("rpc.notification", true),
		)

		if ot.middleware.config.CaptureRequestPayload && params != nil {
			if payload, err := json.Marshal(params); err == nil {
				span.SetAttributes(attribute.String("rpc.request.payload", string(payload)))
			}
		}
	}

	start := time.Now()
	err := ot.next.SendNotification(ctx, method, params)
	duration := time.Since(start)

	// Update metrics
	if ot.middleware.config.EnableMetrics && ot.middleware.metrics != nil {
		status := "success"
		if err != nil {
			status = "error"
		}
		ot.middleware.metrics.RecordNotification(ctx, method, status, duration)
	}

	// Update span
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.IsRecording() {
			span.SetAttributes(attribute.Float64("rpc.duration_ms", float64(duration.Milliseconds())))
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		}
	}

	return err
}

// SendBatchRequest sends a batch request with observability
func (ot *observabilityTransport) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		var span trace.Span
		ctx, span = ot.middleware.tracer.StartMethodSpan(ctx, "batch", trace.SpanKindClient)
		defer span.End()

		span.SetAttributes(
			attribute.String("rpc.method", "batch"),
			attribute.Int("rpc.batch.size", batch.Len()),
			attribute.String("rpc.system", "mcp"),
		)
	}

	start := time.Now()
	result, err := ot.next.SendBatchRequest(ctx, batch)
	duration := time.Since(start)

	// Update metrics
	if ot.middleware.config.EnableMetrics && ot.middleware.metrics != nil {
		status := "success"
		if err != nil {
			status = "error"
		}
		ot.middleware.metrics.RecordBatchRequest(ctx, batch.Len(), status, duration)
	}

	// Update span
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.IsRecording() {
			span.SetAttributes(
				attribute.Float64("rpc.duration_ms", float64(duration.Milliseconds())),
			)
			if result != nil {
				span.SetAttributes(attribute.Int("rpc.batch.responses", result.Len()))
			}
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		}
	}

	return result, err
}

// HandleRequest handles an incoming request with observability
func (ot *observabilityTransport) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	// Extract trace context if present
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		// Extract from headers if available
		ctx = ot.extractTraceContext(ctx, request)

		var span trace.Span
		ctx, span = ot.middleware.tracer.StartMethodSpan(ctx, request.Method, trace.SpanKindServer)
		defer span.End()

		span.SetAttributes(
			attribute.String("rpc.method", request.Method),
			attribute.String("rpc.system", "mcp"),
			attribute.String("rpc.service", "mcp-server"),
		)

		if request.ID != nil {
			span.SetAttributes(attribute.String("rpc.request.id", fmt.Sprintf("%v", request.ID)))
		}
	}

	start := time.Now()
	response, err := ot.next.HandleRequest(ctx, request)
	duration := time.Since(start)

	// Update metrics
	if ot.middleware.config.EnableMetrics && ot.middleware.metrics != nil {
		status := "success"
		if err != nil || (response != nil && response.Error != nil) {
			status = "error"
		}
		ot.middleware.metrics.RecordIncomingRequest(ctx, request.Method, status, duration)
	}

	// Update span
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.IsRecording() {
			span.SetAttributes(attribute.Float64("rpc.duration_ms", float64(duration.Milliseconds())))
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else if response != nil && response.Error != nil {
				span.SetAttributes(
					attribute.Int("rpc.error.code", response.Error.Code),
					attribute.String("rpc.error.message", response.Error.Message),
				)
				span.SetStatus(codes.Error, response.Error.Message)
			}
		}
	}

	return response, err
}

// HandleNotification handles an incoming notification with observability
func (ot *observabilityTransport) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		ctx = ot.extractTraceContext(ctx, notification)

		var span trace.Span
		ctx, span = ot.middleware.tracer.StartMethodSpan(ctx, notification.Method, trace.SpanKindServer)
		defer span.End()

		span.SetAttributes(
			attribute.String("rpc.method", notification.Method),
			attribute.String("rpc.system", "mcp"),
			attribute.Bool("rpc.notification", true),
		)
	}

	start := time.Now()
	err := ot.next.HandleNotification(ctx, notification)
	duration := time.Since(start)

	// Update metrics
	if ot.middleware.config.EnableMetrics && ot.middleware.metrics != nil {
		status := "success"
		if err != nil {
			status = "error"
		}
		ot.middleware.metrics.RecordIncomingNotification(ctx, notification.Method, status, duration)
	}

	// Update span
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.IsRecording() {
			span.SetAttributes(attribute.Float64("rpc.duration_ms", float64(duration.Milliseconds())))
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		}
	}

	return err
}

// HandleBatchRequest handles a batch request with observability
func (ot *observabilityTransport) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		var span trace.Span
		ctx, span = ot.middleware.tracer.StartMethodSpan(ctx, "batch", trace.SpanKindServer)
		defer span.End()

		span.SetAttributes(
			attribute.String("rpc.method", "batch"),
			attribute.Int("rpc.batch.size", batch.Len()),
			attribute.String("rpc.system", "mcp"),
		)
	}

	start := time.Now()
	result, err := ot.next.HandleBatchRequest(ctx, batch)
	duration := time.Since(start)

	// Update metrics
	if ot.middleware.config.EnableMetrics && ot.middleware.metrics != nil {
		status := "success"
		if err != nil {
			status = "error"
		}
		ot.middleware.metrics.RecordIncomingBatchRequest(ctx, batch.Len(), status, duration)
	}

	// Update span
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span != nil && span.IsRecording() {
			span.SetAttributes(attribute.Float64("rpc.duration_ms", float64(duration.Milliseconds())))
			if result != nil {
				span.SetAttributes(attribute.Int("rpc.batch.responses", result.Len()))
			}
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		}
	}

	return result, err
}

// Initialize initializes the transport with observability
func (ot *observabilityTransport) Initialize(ctx context.Context) error {
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		var span trace.Span
		ctx, span = ot.middleware.tracer.StartSpan(ctx, "transport.initialize")
		defer span.End()
	}

	start := time.Now()
	err := ot.next.Initialize(ctx)
	duration := time.Since(start)

	if ot.middleware.config.EnableMetrics && ot.middleware.metrics != nil {
		if err != nil {
			ot.middleware.metrics.RecordTransportEvent(ctx, "initialize", "error", duration)
		} else {
			ot.middleware.metrics.RecordTransportEvent(ctx, "initialize", "success", duration)
		}
	}

	return err
}

// Start starts the transport with observability
func (ot *observabilityTransport) Start(ctx context.Context) error {
	if ot.middleware.config.EnableTracing && ot.middleware.tracer != nil {
		var span trace.Span
		ctx, span = ot.middleware.tracer.StartSpan(ctx, "transport.start")
		defer span.End()
	}

	return ot.next.Start(ctx)
}

// Stop stops the transport with observability
func (ot *observabilityTransport) Stop(ctx context.Context) error {
	err := ot.next.Stop(ctx)

	// Shutdown observability providers
	if ot.middleware.tracer != nil {
		if shutdownErr := ot.middleware.tracer.Shutdown(ctx); shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	}

	if ot.middleware.metrics != nil {
		if shutdownErr := ot.middleware.metrics.Shutdown(ctx); shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	}

	return err
}

// HandleResponse handles a response
func (ot *observabilityTransport) HandleResponse(response *protocol.Response) {
	ot.next.HandleResponse(response)
}

// RegisterRequestHandler registers a request handler
func (ot *observabilityTransport) RegisterRequestHandler(method string, handler transport.RequestHandler) {
	ot.next.RegisterRequestHandler(method, handler)
}

// RegisterNotificationHandler registers a notification handler
func (ot *observabilityTransport) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	ot.next.RegisterNotificationHandler(method, handler)
}

// RegisterProgressHandler registers a progress handler
func (ot *observabilityTransport) RegisterProgressHandler(id interface{}, handler transport.ProgressHandler) {
	ot.next.RegisterProgressHandler(id, handler)
}

// UnregisterProgressHandler unregisters a progress handler
func (ot *observabilityTransport) UnregisterProgressHandler(id interface{}) {
	ot.next.UnregisterProgressHandler(id)
}

// GenerateID generates a unique ID
func (ot *observabilityTransport) GenerateID() string {
	return ot.next.GenerateID()
}

// GetRequestIDPrefix gets the request ID prefix
func (ot *observabilityTransport) GetRequestIDPrefix() string {
	return ot.next.GetRequestIDPrefix()
}

// GetNextID gets the next ID
func (ot *observabilityTransport) GetNextID() int64 {
	return ot.next.GetNextID()
}

// Cleanup performs cleanup
func (ot *observabilityTransport) Cleanup() {
	ot.next.Cleanup()
}

// extractTraceContext extracts trace context from request metadata
func (ot *observabilityTransport) extractTraceContext(ctx context.Context, msg interface{}) context.Context {
	// In a real implementation, we would extract trace headers from the message
	// For now, we'll use the existing context
	return ctx
}

// getErrorType categorizes errors for metrics
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Check for JSON-RPC error types
	if rpcErr, ok := err.(*protocol.ErrorObject); ok {
		switch {
		case rpcErr.Code >= -32099 && rpcErr.Code <= -32000:
			return "server_error"
		case rpcErr.Code == -32700:
			return "parse_error"
		case rpcErr.Code == -32600:
			return "invalid_request"
		case rpcErr.Code == -32601:
			return "method_not_found"
		case rpcErr.Code == -32602:
			return "invalid_params"
		case rpcErr.Code == -32603:
			return "internal_error"
		default:
			return "unknown_error"
		}
	}

	// Generic error categorization
	errStr := err.Error()
	switch {
	case contains(errStr, "timeout"):
		return "timeout"
	case contains(errStr, "connection"):
		return "connection"
	case contains(errStr, "cancelled"):
		return "cancelled"
	default:
		return "unknown"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && s[0:len(substr)] == substr || len(s) > len(substr) && s[len(s)-len(substr):] == substr || (len(substr) > 0 && len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
