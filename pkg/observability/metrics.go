package observability

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsConfig configures the metrics provider
type MetricsConfig struct {
	// Service identification
	ServiceName    string
	ServiceVersion string
	Environment    string

	// Prometheus configuration
	MetricsPath    string // HTTP path for metrics endpoint (default: /metrics)
	MetricsPort    int    // Port for metrics server (default: 9090)
	EnablePush     bool   // Enable push gateway support
	PushGatewayURL string // Push gateway URL

	// Metric options
	Namespace        string    // Prometheus namespace (default: mcp)
	Subsystem        string    // Prometheus subsystem
	HistogramBuckets []float64 // Custom histogram buckets for latency

	// Labels to add to all metrics
	ConstLabels prometheus.Labels
}

// MetricsProvider manages Prometheus metrics
type MetricsProvider interface {
	// Record MCP operations
	RecordRequest(ctx context.Context, method, status string, duration time.Duration)
	RecordNotification(ctx context.Context, method, status string, duration time.Duration)
	RecordBatchRequest(ctx context.Context, size int, status string, duration time.Duration)
	RecordIncomingRequest(ctx context.Context, method, status string, duration time.Duration)
	RecordIncomingNotification(ctx context.Context, method, status string, duration time.Duration)
	RecordIncomingBatchRequest(ctx context.Context, size int, status string, duration time.Duration)

	// Record transport events
	RecordTransportEvent(ctx context.Context, event, status string, duration time.Duration)
	RecordConnectionState(ctx context.Context, state string)
	RecordActiveConnections(ctx context.Context, delta int)

	// Record resource metrics
	RecordResourceOperation(ctx context.Context, operation, resource, status string, duration time.Duration)
	RecordToolCall(ctx context.Context, tool, status string, duration time.Duration)
	RecordPromptExecution(ctx context.Context, prompt, status string, duration time.Duration)

	// Custom metrics
	RecordGauge(name string, value float64, labels prometheus.Labels)
	RecordCounter(name string, labels prometheus.Labels)
	RecordHistogram(name string, value float64, labels prometheus.Labels)

	// Management
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// PrometheusMetricsProvider implements MetricsProvider using Prometheus
type PrometheusMetricsProvider struct {
	config MetricsConfig
	server *http.Server

	// Core MCP metrics
	requestDuration      *prometheus.HistogramVec
	requestTotal         *prometheus.CounterVec
	notificationDuration *prometheus.HistogramVec
	notificationTotal    *prometheus.CounterVec
	batchRequestDuration *prometheus.HistogramVec
	batchRequestTotal    *prometheus.CounterVec
	batchRequestSize     *prometheus.HistogramVec

	// Incoming metrics
	incomingRequestDuration *prometheus.HistogramVec
	incomingRequestTotal    *prometheus.CounterVec
	// Reserved for future use:
	// incomingNotificationDuration *prometheus.HistogramVec
	// incomingNotificationTotal    *prometheus.CounterVec

	// Transport metrics
	transportEventDuration *prometheus.HistogramVec
	// transportEventTotal    *prometheus.CounterVec // Reserved for future use
	connectionState   *prometheus.GaugeVec
	activeConnections prometheus.Gauge

	// Resource metrics
	resourceOperationDuration *prometheus.HistogramVec
	// resourceOperationTotal    *prometheus.CounterVec // Reserved for future use
	toolCallDuration *prometheus.HistogramVec
	// toolCallTotal             *prometheus.CounterVec // Reserved for future use
	// promptExecutionDuration   *prometheus.HistogramVec // Reserved for future use
	// promptExecutionTotal      *prometheus.CounterVec // Reserved for future use

	// Error metrics
	errorTotal *prometheus.CounterVec

	// Custom metrics registry
	customMetrics map[string]prometheus.Collector
	mu            sync.RWMutex
}

// NewMetricsProvider creates a new Prometheus metrics provider
func NewMetricsProvider(config MetricsConfig) (MetricsProvider, error) {
	// Set defaults
	if config.Namespace == "" {
		config.Namespace = "mcp"
	}
	if config.MetricsPath == "" {
		config.MetricsPath = "/metrics"
	}
	if config.MetricsPort == 0 {
		config.MetricsPort = 9090
	}
	if config.HistogramBuckets == nil {
		// Default buckets for milliseconds
		config.HistogramBuckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
	}

	// Add service labels to const labels
	if config.ConstLabels == nil {
		config.ConstLabels = prometheus.Labels{}
	}
	config.ConstLabels["service"] = config.ServiceName
	config.ConstLabels["version"] = config.ServiceVersion
	config.ConstLabels["environment"] = config.Environment

	provider := &PrometheusMetricsProvider{
		config:        config,
		customMetrics: make(map[string]prometheus.Collector),
	}

	// Initialize metrics
	provider.initializeMetrics()

	// Register metrics
	if err := provider.registerMetrics(); err != nil {
		return nil, fmt.Errorf("failed to register metrics: %w", err)
	}

	return provider, nil
}

// initializeMetrics creates all metric collectors
func (p *PrometheusMetricsProvider) initializeMetrics() {
	// Request metrics
	p.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "request_duration_milliseconds",
			Help:        "Duration of MCP requests in milliseconds",
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"method", "status"},
	)

	p.requestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "request_total",
			Help:        "Total number of MCP requests",
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"method", "status"},
	)

	// Notification metrics
	p.notificationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "notification_duration_milliseconds",
			Help:        "Duration of MCP notifications in milliseconds",
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"method", "status"},
	)

	p.notificationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "notification_total",
			Help:        "Total number of MCP notifications",
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"method", "status"},
	)

	// Batch request metrics
	p.batchRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "batch_request_duration_milliseconds",
			Help:        "Duration of MCP batch requests in milliseconds",
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"status"},
	)

	p.batchRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "batch_request_total",
			Help:        "Total number of MCP batch requests",
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"status"},
	)

	p.batchRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "batch_request_size",
			Help:        "Size of MCP batch requests",
			Buckets:     []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"status"},
	)

	// Incoming request metrics
	p.incomingRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "incoming_request_duration_milliseconds",
			Help:        "Duration of incoming MCP requests in milliseconds",
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"method", "status"},
	)

	p.incomingRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "incoming_request_total",
			Help:        "Total number of incoming MCP requests",
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"method", "status"},
	)

	// Transport metrics
	p.transportEventDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "transport_event_duration_milliseconds",
			Help:        "Duration of transport events in milliseconds",
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"event", "status"},
	)

	p.connectionState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "connection_state",
			Help:        "Current connection state (1=connected, 0=disconnected)",
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"state"},
	)

	p.activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "active_connections",
			Help:        "Number of active connections",
			ConstLabels: p.config.ConstLabels,
		},
	)

	// Resource metrics
	p.resourceOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "resource_operation_duration_milliseconds",
			Help:        "Duration of resource operations in milliseconds",
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"operation", "resource", "status"},
	)

	p.toolCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "tool_call_duration_milliseconds",
			Help:        "Duration of tool calls in milliseconds",
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"tool", "status"},
	)

	// Error metrics
	p.errorTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   p.config.Subsystem,
			Name:        "error_total",
			Help:        "Total number of errors",
			ConstLabels: p.config.ConstLabels,
		},
		[]string{"type", "method"},
	)
}

// registerMetrics registers all metrics with Prometheus
func (p *PrometheusMetricsProvider) registerMetrics() error {
	collectors := []prometheus.Collector{
		p.requestDuration,
		p.requestTotal,
		p.notificationDuration,
		p.notificationTotal,
		p.batchRequestDuration,
		p.batchRequestTotal,
		p.batchRequestSize,
		p.incomingRequestDuration,
		p.incomingRequestTotal,
		p.transportEventDuration,
		p.connectionState,
		p.activeConnections,
		p.resourceOperationDuration,
		p.toolCallDuration,
		p.errorTotal,
	}

	for _, collector := range collectors {
		if err := prometheus.Register(collector); err != nil {
			// Check if already registered
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return err
			}
		}
	}

	return nil
}

// RecordRequest records an outgoing request
func (p *PrometheusMetricsProvider) RecordRequest(ctx context.Context, method, status string, duration time.Duration) {
	ms := float64(duration.Milliseconds())
	p.requestDuration.WithLabelValues(method, status).Observe(ms)
	p.requestTotal.WithLabelValues(method, status).Inc()
}

// RecordNotification records an outgoing notification
func (p *PrometheusMetricsProvider) RecordNotification(ctx context.Context, method, status string, duration time.Duration) {
	ms := float64(duration.Milliseconds())
	p.notificationDuration.WithLabelValues(method, status).Observe(ms)
	p.notificationTotal.WithLabelValues(method, status).Inc()
}

// RecordBatchRequest records a batch request
func (p *PrometheusMetricsProvider) RecordBatchRequest(ctx context.Context, size int, status string, duration time.Duration) {
	ms := float64(duration.Milliseconds())
	p.batchRequestDuration.WithLabelValues(status).Observe(ms)
	p.batchRequestTotal.WithLabelValues(status).Inc()
	p.batchRequestSize.WithLabelValues(status).Observe(float64(size))
}

// RecordIncomingRequest records an incoming request
func (p *PrometheusMetricsProvider) RecordIncomingRequest(ctx context.Context, method, status string, duration time.Duration) {
	ms := float64(duration.Milliseconds())
	p.incomingRequestDuration.WithLabelValues(method, status).Observe(ms)
	p.incomingRequestTotal.WithLabelValues(method, status).Inc()
}

// RecordIncomingNotification records an incoming notification
func (p *PrometheusMetricsProvider) RecordIncomingNotification(ctx context.Context, method, status string, duration time.Duration) {
	// Use incoming request metrics for notifications too
	p.RecordIncomingRequest(ctx, method, status, duration)
}

// RecordIncomingBatchRequest records an incoming batch request
func (p *PrometheusMetricsProvider) RecordIncomingBatchRequest(ctx context.Context, size int, status string, duration time.Duration) {
	// Record as batch with size information
	p.RecordBatchRequest(ctx, size, status, duration)
}

// RecordTransportEvent records a transport event
func (p *PrometheusMetricsProvider) RecordTransportEvent(ctx context.Context, event, status string, duration time.Duration) {
	ms := float64(duration.Milliseconds())
	p.transportEventDuration.WithLabelValues(event, status).Observe(ms)
}

// RecordConnectionState records the current connection state
func (p *PrometheusMetricsProvider) RecordConnectionState(ctx context.Context, state string) {
	// Reset all states to 0
	p.connectionState.WithLabelValues("connected").Set(0)
	p.connectionState.WithLabelValues("disconnected").Set(0)
	p.connectionState.WithLabelValues("connecting").Set(0)

	// Set current state to 1
	p.connectionState.WithLabelValues(state).Set(1)
}

// RecordActiveConnections records the change in active connections
func (p *PrometheusMetricsProvider) RecordActiveConnections(ctx context.Context, delta int) {
	if delta > 0 {
		p.activeConnections.Add(float64(delta))
	} else {
		p.activeConnections.Sub(float64(-delta))
	}
}

// RecordResourceOperation records a resource operation
func (p *PrometheusMetricsProvider) RecordResourceOperation(ctx context.Context, operation, resource, status string, duration time.Duration) {
	ms := float64(duration.Milliseconds())
	p.resourceOperationDuration.WithLabelValues(operation, resource, status).Observe(ms)
}

// RecordToolCall records a tool call
func (p *PrometheusMetricsProvider) RecordToolCall(ctx context.Context, tool, status string, duration time.Duration) {
	ms := float64(duration.Milliseconds())
	p.toolCallDuration.WithLabelValues(tool, status).Observe(ms)
}

// RecordPromptExecution records a prompt execution
func (p *PrometheusMetricsProvider) RecordPromptExecution(ctx context.Context, prompt, status string, duration time.Duration) {
	// Use tool call metrics for prompts
	p.RecordToolCall(ctx, "prompt:"+prompt, status, duration)
}

// RecordGauge records a custom gauge metric
func (p *PrometheusMetricsProvider) RecordGauge(name string, value float64, labels prometheus.Labels) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := name + fmt.Sprint(labels)
	if gauge, exists := p.customMetrics[key]; exists {
		if g, ok := gauge.(*prometheus.GaugeVec); ok {
			g.With(labels).Set(value)
			return
		}
	}

	// Create new gauge if it doesn't exist
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   "custom",
			Name:        name,
			Help:        fmt.Sprintf("Custom gauge metric: %s", name),
			ConstLabels: p.config.ConstLabels,
		},
		getLabelsKeys(labels),
	)

	prometheus.MustRegister(gauge)
	p.customMetrics[key] = gauge
	gauge.With(labels).Set(value)
}

// RecordCounter records a custom counter metric
func (p *PrometheusMetricsProvider) RecordCounter(name string, labels prometheus.Labels) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := name + fmt.Sprint(labels)
	if counter, exists := p.customMetrics[key]; exists {
		if c, ok := counter.(*prometheus.CounterVec); ok {
			c.With(labels).Inc()
			return
		}
	}

	// Create new counter if it doesn't exist
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   "custom",
			Name:        name,
			Help:        fmt.Sprintf("Custom counter metric: %s", name),
			ConstLabels: p.config.ConstLabels,
		},
		getLabelsKeys(labels),
	)

	prometheus.MustRegister(counter)
	p.customMetrics[key] = counter
	counter.With(labels).Inc()
}

// RecordHistogram records a custom histogram metric
func (p *PrometheusMetricsProvider) RecordHistogram(name string, value float64, labels prometheus.Labels) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := name + fmt.Sprint(labels)
	if histogram, exists := p.customMetrics[key]; exists {
		if h, ok := histogram.(*prometheus.HistogramVec); ok {
			h.With(labels).Observe(value)
			return
		}
	}

	// Create new histogram if it doesn't exist
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   p.config.Namespace,
			Subsystem:   "custom",
			Name:        name,
			Help:        fmt.Sprintf("Custom histogram metric: %s", name),
			Buckets:     p.config.HistogramBuckets,
			ConstLabels: p.config.ConstLabels,
		},
		getLabelsKeys(labels),
	)

	prometheus.MustRegister(histogram)
	p.customMetrics[key] = histogram
	histogram.With(labels).Observe(value)
}

// Start starts the metrics HTTP server
func (p *PrometheusMetricsProvider) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle(p.config.MetricsPath, promhttp.Handler())

	p.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", p.config.MetricsPort),
		Handler: mux,
	}

	go func() {
		_ = p.server.ListenAndServe()
	}()

	return nil
}

// Shutdown gracefully shuts down the metrics server
func (p *PrometheusMetricsProvider) Shutdown(ctx context.Context) error {
	if p.server != nil {
		return p.server.Shutdown(ctx)
	}
	return nil
}

// Helper function to extract label keys from a map
func getLabelsKeys(labels prometheus.Labels) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	return keys
}
