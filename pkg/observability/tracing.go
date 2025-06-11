// Package observability provides enhanced monitoring, metrics, and tracing for MCP
package observability

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig configures OpenTelemetry tracing
type TracingConfig struct {
	// Service identification
	ServiceName    string
	ServiceVersion string
	Environment    string

	// Exporter configuration
	ExporterType ExporterType
	Endpoint     string // OTLP endpoint
	Headers      map[string]string
	Insecure     bool // Use insecure connection (for development)

	// Sampling configuration
	SampleRate   float64  // 0.0 to 1.0
	AlwaysSample []string // Method names to always sample
	NeverSample  []string // Method names to never sample

	// Performance options
	BatchTimeout int // Batch timeout in seconds
	MaxBatchSize int // Maximum batch size
	MaxQueueSize int // Maximum queue size

	// Additional attributes
	ResourceAttributes map[string]string
}

// ExporterType defines the type of trace exporter
type ExporterType string

const (
	// ExporterTypeOTLPGRPC exports traces via OTLP over gRPC
	ExporterTypeOTLPGRPC ExporterType = "otlp-grpc"

	// ExporterTypeOTLPHTTP exports traces via OTLP over HTTP
	ExporterTypeOTLPHTTP ExporterType = "otlp-http"

	// ExporterTypeJaeger exports traces to Jaeger (deprecated, use OTLP)
	ExporterTypeJaeger ExporterType = "jaeger"

	// ExporterTypeNoop disables trace export (for testing)
	ExporterTypeNoop ExporterType = "noop"
)

// TracingProvider manages OpenTelemetry tracing
type TracingProvider struct {
	config         TracingConfig
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	propagator     propagation.TextMapPropagator
	mu             sync.RWMutex
	shutdown       func(context.Context) error
}

// NewTracingProvider creates a new tracing provider
func NewTracingProvider(config TracingConfig) (*TracingProvider, error) {
	// Set defaults
	if config.ServiceName == "" {
		config.ServiceName = "mcp-service"
	}
	if config.ServiceVersion == "" {
		config.ServiceVersion = "unknown"
	}
	if config.Environment == "" {
		config.Environment = "development"
	}
	if config.SampleRate == 0 {
		config.SampleRate = 1.0
	}
	if config.BatchTimeout == 0 {
		config.BatchTimeout = 5
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 512
	}
	if config.MaxQueueSize == 0 {
		config.MaxQueueSize = 2048
	}

	// Create resource
	res, err := createResource(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	exporter, err := createExporter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create sampler
	sampler := createSampler(config)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Create propagator
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		propagator = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
		otel.SetTextMapPropagator(propagator)
	}

	return &TracingProvider{
		config:         config,
		tracerProvider: tp,
		tracer:         tp.Tracer("mcp-sdk"),
		propagator:     propagator,
		shutdown:       tp.Shutdown,
	}, nil
}

// createResource creates the OpenTelemetry resource
func createResource(config TracingConfig) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(config.ServiceName),
		semconv.ServiceVersion(config.ServiceVersion),
		semconv.DeploymentEnvironment(config.Environment),
	}

	// Add custom resource attributes
	for k, v := range config.ResourceAttributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attrs...,
	), nil
}

// createExporter creates the configured trace exporter
func createExporter(config TracingConfig) (sdktrace.SpanExporter, error) {
	switch config.ExporterType {
	case ExporterTypeOTLPGRPC:
		return createOTLPGRPCExporter(config)
	case ExporterTypeOTLPHTTP:
		return createOTLPHTTPExporter(config)
	case ExporterTypeNoop:
		return &noopExporter{}, nil
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", config.ExporterType)
	}
}

// createOTLPGRPCExporter creates an OTLP gRPC exporter
func createOTLPGRPCExporter(config TracingConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.Endpoint),
		otlptracegrpc.WithHeaders(config.Headers),
	}

	if config.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	client := otlptracegrpc.NewClient(opts...)
	return otlptrace.New(context.Background(), client)
}

// createOTLPHTTPExporter creates an OTLP HTTP exporter
func createOTLPHTTPExporter(config TracingConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.Endpoint),
		otlptracehttp.WithHeaders(config.Headers),
	}

	if config.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	client := otlptracehttp.NewClient(opts...)
	return otlptrace.New(context.Background(), client)
}

// createSampler creates a sampler based on configuration
func createSampler(config TracingConfig) sdktrace.Sampler {
	// Create method-based sampler if configured
	if len(config.AlwaysSample) > 0 || len(config.NeverSample) > 0 {
		return &methodSampler{
			defaultRate:  config.SampleRate,
			alwaysSample: makeStringSet(config.AlwaysSample),
			neverSample:  makeStringSet(config.NeverSample),
		}
	}

	// Use standard sampler
	if config.SampleRate >= 1.0 {
		return sdktrace.AlwaysSample()
	} else if config.SampleRate <= 0.0 {
		return sdktrace.NeverSample()
	}
	return sdktrace.TraceIDRatioBased(config.SampleRate)
}

// StartSpan starts a new span with the given name and options
func (tp *TracingProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tp.tracer.Start(ctx, name, opts...)
}

// StartMethodSpan starts a span for an MCP method
func (tp *TracingProvider) StartMethodSpan(ctx context.Context, method string, spanKind trace.SpanKind) (context.Context, trace.Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(spanKind),
		trace.WithAttributes(
			attribute.String("mcp.method", method),
			attribute.String("mcp.service", tp.config.ServiceName),
		),
	}

	return tp.tracer.Start(ctx, fmt.Sprintf("mcp.%s", method), opts...)
}

// RecordError records an error on the current span
func (tp *TracingProvider) RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.RecordError(err, opts...)
		span.SetStatus(codes.Error, err.Error())
	}
}

// AddEvent adds an event to the current span
func (tp *TracingProvider) AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// SetAttributes sets attributes on the current span
func (tp *TracingProvider) SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// Extract extracts trace context from a carrier
func (tp *TracingProvider) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return tp.propagator.Extract(ctx, carrier)
}

// Inject injects trace context into a carrier
func (tp *TracingProvider) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	tp.propagator.Inject(ctx, carrier)
}

// Shutdown gracefully shuts down the tracing provider
func (tp *TracingProvider) Shutdown(ctx context.Context) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.shutdown != nil {
		return tp.shutdown(ctx)
	}
	return nil
}

// methodSampler samples based on method name
type methodSampler struct {
	defaultRate  float64
	alwaysSample map[string]struct{}
	neverSample  map[string]struct{}
}

func (ms *methodSampler) ShouldSample(params sdktrace.SamplingParameters) sdktrace.SamplingResult {
	// Extract method name from span name or attributes
	method := params.Name
	for _, attr := range params.Attributes {
		if attr.Key == "mcp.method" {
			method = attr.Value.AsString()
			break
		}
	}

	// Check always/never lists
	if _, ok := ms.alwaysSample[method]; ok {
		return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
	}
	if _, ok := ms.neverSample[method]; ok {
		return sdktrace.SamplingResult{Decision: sdktrace.Drop}
	}

	// Use default rate
	if ms.defaultRate >= 1.0 {
		return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
	} else if ms.defaultRate <= 0.0 {
		return sdktrace.SamplingResult{Decision: sdktrace.Drop}
	}

	// Use trace ID ratio
	return sdktrace.TraceIDRatioBased(ms.defaultRate).ShouldSample(params)
}

func (ms *methodSampler) Description() string {
	return fmt.Sprintf("MethodSampler{defaultRate=%.2f}", ms.defaultRate)
}

// noopExporter is a no-op span exporter for testing
type noopExporter struct{}

func (n *noopExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

func (n *noopExporter) Shutdown(ctx context.Context) error {
	return nil
}

// Helper functions

func makeStringSet(items []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}

// TracingContextKey is used to store tracing metadata in context
type tracingContextKey struct{}

// TracingMetadata contains tracing-related metadata
type TracingMetadata struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Baggage   map[string]string
	RequestID string
}

// WithTracingMetadata adds tracing metadata to context
func WithTracingMetadata(ctx context.Context, metadata TracingMetadata) context.Context {
	return context.WithValue(ctx, tracingContextKey{}, metadata)
}

// GetTracingMetadata retrieves tracing metadata from context
func GetTracingMetadata(ctx context.Context) (TracingMetadata, bool) {
	metadata, ok := ctx.Value(tracingContextKey{}).(TracingMetadata)
	return metadata, ok
}
