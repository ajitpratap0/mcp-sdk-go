// Package transport provides a modern, config-driven transport layer for MCP communication.
//
// Key Features:
// - Unified TransportConfig-based creation (no legacy Options pattern)
// - Automatic middleware composition (reliability, observability)
// - Support for stdio and HTTP transports
// - Production-ready reliability features (retries, circuit breakers)
// - Comprehensive observability (metrics, logging, tracing)
//
// Usage:
//
//	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
//	config.Endpoint = "https://api.example.com/mcp"
//	transport, err := transport.NewTransport(config)
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// Transport defines the core interface for MCP transport mechanisms.
// All transports must implement this minimal interface.
type Transport interface {
	// Initialize prepares the transport for use
	Initialize(ctx context.Context) error

	// Core communication methods
	SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
	SendNotification(ctx context.Context, method string, params interface{}) error

	// Batch processing methods
	SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error)
	HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error)

	// Handler registration
	RegisterRequestHandler(method string, handler RequestHandler)
	RegisterNotificationHandler(method string, handler NotificationHandler)
	RegisterProgressHandler(id interface{}, handler ProgressHandler)
	UnregisterProgressHandler(id interface{})

	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Message handling
	HandleResponse(response *protocol.Response)
	HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error)
	HandleNotification(ctx context.Context, notification *protocol.Notification) error

	// Utilities
	GenerateID() string
	GetRequestIDPrefix() string
	GetNextID() int64
	Cleanup()
}

// StreamingTransport extends Transport with raw message capabilities.
// Used by transports like stdio that deal with raw byte streams.
type StreamingTransport interface {
	Transport

	// Raw message handling
	Send(data []byte) error
	SetReceiveHandler(handler ReceiveHandler)
	SetErrorHandler(handler ErrorHandler)
}

// Handlers for various transport operations

// RequestHandler handles incoming requests
type RequestHandler func(ctx context.Context, params interface{}) (interface{}, error)

// NotificationHandler handles incoming notifications
type NotificationHandler func(ctx context.Context, params interface{}) error

// ProgressHandler handles progress notifications
type ProgressHandler func(params interface{}) error

// ReceiveHandler processes raw incoming message data
type ReceiveHandler func(data []byte)

// ErrorHandler handles transport errors
type ErrorHandler func(err error)

// TransportType identifies the base transport implementation
type TransportType string

const (
	TransportTypeStdio          TransportType = "stdio"
	TransportTypeHTTP           TransportType = "http"
	TransportTypeStreamableHTTP TransportType = "streamable_http"
)

// TransportConfig is the unified configuration for all transports
type TransportConfig struct {
	// Type of transport to create
	Type TransportType `json:"type"`

	// Transport-specific settings
	Endpoint string `json:"endpoint,omitempty"` // For HTTP transports

	// Testing support (for custom readers/writers in stdio)
	StdioReader io.Reader `json:"-"` // Custom reader for stdio (testing only)
	StdioWriter io.Writer `json:"-"` // Custom writer for stdio (testing only)

	// Feature configuration
	Features FeatureConfig `json:"features"`

	// Component configurations
	Connection    ConnectionConfig    `json:"connection"`
	Reliability   ReliabilityConfig   `json:"reliability"`
	Observability ObservabilityConfig `json:"observability"`
	Performance   PerformanceConfig   `json:"performance"`
	Security      SecurityConfig      `json:"security"`

	// Advanced features (when middleware implemented)
	LoadBalancer *LoadBalancerConfig `json:"load_balancer,omitempty"`
	Queuing      *QueuingConfig      `json:"queuing,omitempty"`
	Parallelism  *ParallelismConfig  `json:"parallelism,omitempty"`
}

// FeatureConfig controls which middleware are enabled
type FeatureConfig struct {
	EnableReliability    bool `json:"enable_reliability"`
	EnableObservability  bool `json:"enable_observability"`
	EnableConnectionPool bool `json:"enable_connection_pool"`
	EnableLoadBalancing  bool `json:"enable_load_balancing"`
	EnableQueuing        bool `json:"enable_queuing"`
	EnableParallelism    bool `json:"enable_parallelism"`
	EnableStreaming      bool `json:"enable_streaming"`
	EnableBatching       bool `json:"enable_batching"`
	EnableAsyncTools     bool `json:"enable_async_tools"`
	EnableAuthentication bool `json:"enable_authentication"`
	EnableRateLimiting   bool `json:"enable_rate_limiting"`
}

// ConnectionConfig for connection management
type ConnectionConfig struct {
	Timeout         time.Duration `json:"timeout"`
	KeepAlive       time.Duration `json:"keep_alive"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	MaxConnsPerHost int           `json:"max_conns_per_host"`
	IdleConnTimeout time.Duration `json:"idle_conn_timeout"`
}

// ReliabilityConfig for retry and resilience
type ReliabilityConfig struct {
	MaxRetries         int                  `json:"max_retries"`
	InitialRetryDelay  time.Duration        `json:"initial_retry_delay"`
	MaxRetryDelay      time.Duration        `json:"max_retry_delay"`
	RetryBackoffFactor float64              `json:"retry_backoff_factor"`
	CircuitBreaker     CircuitBreakerConfig `json:"circuit_breaker"`
}

// CircuitBreakerConfig for circuit breaker pattern
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
}

// ObservabilityConfig for metrics and logging
type ObservabilityConfig struct {
	EnableMetrics bool   `json:"enable_metrics"`
	EnableLogging bool   `json:"enable_logging"`
	LogLevel      string `json:"log_level"`
	MetricsPrefix string `json:"metrics_prefix"`
}

// PerformanceConfig for performance tuning
type PerformanceConfig struct {
	BufferSize     int           `json:"buffer_size"`
	FlushInterval  time.Duration `json:"flush_interval"`
	MaxConcurrency int           `json:"max_concurrency"`
	RequestTimeout time.Duration `json:"request_timeout"`
}

// LoadBalancerConfig for load balancing (placeholder for future middleware)
type LoadBalancerConfig struct {
	Strategy  string   `json:"strategy"`
	Endpoints []string `json:"endpoints"`
}

// QueuingConfig for request queuing (placeholder for future middleware)
type QueuingConfig struct {
	MaxQueueSize   int           `json:"max_queue_size"`
	RequestTimeout time.Duration `json:"request_timeout"`
}

// ParallelismConfig for parallel processing (placeholder for future middleware)
type ParallelismConfig struct {
	MaxWorkers int `json:"max_workers"`
}

// SecurityConfig configures security features including authentication
type SecurityConfig struct {
	// Authentication configuration
	Authentication *AuthenticationConfig `json:"authentication,omitempty"`

	// Existing security settings
	AllowedOrigins      []string `json:"allowed_origins,omitempty"`
	AllowWildcardOrigin bool     `json:"allow_wildcard_origin"`

	// TLS configuration
	TLS *TLSConfig `json:"tls,omitempty"`

	// Rate limiting
	RateLimit *RateLimitConfig `json:"rate_limit,omitempty"`
}

// AuthenticationConfig configures authentication for transports
type AuthenticationConfig struct {
	// Type of authentication (bearer, apikey, oauth2, etc.)
	Type string `json:"type"`

	// Provider-specific configuration
	ProviderConfig map[string]interface{} `json:"provider_config,omitempty"`

	// Whether authentication is required for all requests
	Required bool `json:"required"`

	// Allow anonymous access for specific operations
	AllowAnonymous bool `json:"allow_anonymous"`

	// Token expiry settings
	TokenExpiry      time.Duration `json:"token_expiry,omitempty"`
	RefreshThreshold time.Duration `json:"refresh_threshold,omitempty"`

	// Cache configuration for validated tokens
	EnableCache bool          `json:"enable_cache"`
	CacheTTL    time.Duration `json:"cache_ttl,omitempty"`
}

// TLSConfig configures TLS settings
type TLSConfig struct {
	Enabled            bool     `json:"enabled"`
	CertFile           string   `json:"cert_file,omitempty"`
	KeyFile            string   `json:"key_file,omitempty"`
	CAFile             string   `json:"ca_file,omitempty"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify"`
	CipherSuites       []string `json:"cipher_suites,omitempty"`
	MinVersion         string   `json:"min_version,omitempty"`
}

// RateLimitConfig configures rate limiting
type RateLimitConfig struct {
	Enabled           bool `json:"enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	RequestsPerHour   int  `json:"requests_per_hour"`
	BurstSize         int  `json:"burst_size"`
}

// SessionHandler handles session lifecycle events
type SessionHandler func(sessionID string, event SessionEvent)

// SessionEvent types
type SessionEvent int

const (
	SessionEventCreated SessionEvent = iota
	SessionEventResumed
	SessionEventTerminated
)

// Errors
var (
	ErrUnsupportedMethod        = errors.New("unsupported method")
	ErrUnsupportedTransportType = errors.New("unsupported transport type")
)

// NewTransport creates a new transport with the specified configuration
func NewTransport(config TransportConfig) (Transport, error) {
	// Validate configuration
	if err := validateTransportConfig(config); err != nil {
		return nil, err
	}

	// Create base transport using modern constructors
	var base Transport
	var err error

	switch config.Type {
	case TransportTypeStdio:
		base, err = newStdioTransport(config)
	case TransportTypeStreamableHTTP:
		base, err = newStreamableHTTPTransport(config)
	case TransportTypeHTTP:
		base, err = newStreamableHTTPTransport(config) // HTTP uses StreamableHTTP internally
	default:
		return nil, ErrUnsupportedTransportType
	}

	if err != nil {
		return nil, err
	}

	// Build middleware chain
	builder := NewMiddlewareBuilder(config)
	middleware := builder.Build()

	// Apply middleware chain
	transport := ChainMiddleware(middleware...).Wrap(base)

	return transport, nil
}

// validateTransportConfig validates the transport configuration
func validateTransportConfig(config TransportConfig) error {
	switch config.Type {
	case TransportTypeStdio:
		// No additional validation needed for stdio
		return nil
	case TransportTypeStreamableHTTP, TransportTypeHTTP:
		if config.Endpoint == "" {
			return errors.New("endpoint is required for HTTP transports")
		}
		return nil
	default:
		return ErrUnsupportedTransportType
	}
}

// BaseTransport provides common functionality for all transport implementations.
// It handles request/response management, handler registration, and ID generation.
type BaseTransport struct {
	sync.RWMutex
	requestHandlers      map[string]RequestHandler
	notificationHandlers map[string]NotificationHandler
	progressHandlers     map[interface{}]ProgressHandler
	nextID               int64
	pendingRequests      map[string]chan *protocol.Response
	requestIDPrefix      string
}

// Logf logs a formatted message (stub for compatibility)
func (t *BaseTransport) Logf(format string, args ...interface{}) {
	// This is a stub - logging should be handled by observability middleware
}

// HandleRequest processes an incoming request with panic recovery
func (t *BaseTransport) HandleRequest(ctx context.Context, request *protocol.Request) (resp *protocol.Response, err error) {
	// Recover from panics and convert to errors
	defer func() {
		if r := recover(); r != nil {
			resp = &protocol.Response{
				ID: request.ID,
				Error: &protocol.Error{
					Code:    protocol.InternalError,
					Message: fmt.Sprintf("Internal server error processing %s", request.Method),
				},
			}
			err = nil // We return the error in the response, not as an error
		}
	}()

	t.RLock()
	handler, ok := t.requestHandlers[request.Method]
	t.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler for method: %s", request.Method)
	}

	result, handlerErr := handler(ctx, request.Params)
	if handlerErr != nil {
		return &protocol.Response{
			ID:    request.ID,
			Error: &protocol.Error{Code: -32603, Message: handlerErr.Error()},
		}, nil
	}

	// Marshal result to json.RawMessage
	resultBytes, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return &protocol.Response{
			ID:    request.ID,
			Error: &protocol.Error{Code: -32603, Message: fmt.Sprintf("failed to marshal result: %v", marshalErr)},
		}, nil
	}

	return &protocol.Response{
		ID:     request.ID,
		Result: resultBytes,
	}, nil
}

// HandleResponse processes an incoming response
func (t *BaseTransport) HandleResponse(response *protocol.Response) {
	t.Lock()
	ch, ok := t.pendingRequests[fmt.Sprintf("%v", response.ID)]
	if ok {
		ch <- response
		delete(t.pendingRequests, fmt.Sprintf("%v", response.ID))
	}
	t.Unlock()
}

// HandleNotification processes an incoming notification with panic recovery
func (t *BaseTransport) HandleNotification(ctx context.Context, notification *protocol.Notification) (err error) {
	// Recover from panics and convert to errors
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error processing notification %s: %v", notification.Method, r)
		}
	}()

	t.RLock()
	handler, ok := t.notificationHandlers[notification.Method]
	t.RUnlock()

	if !ok {
		return fmt.Errorf("no handler for notification: %s", notification.Method)
	}

	return handler(ctx, notification.Params)
}

// WaitForResponse waits for a response with the given ID
func (t *BaseTransport) WaitForResponse(ctx context.Context, id string) (*protocol.Response, error) {
	t.Lock()
	ch := make(chan *protocol.Response, 1)
	t.pendingRequests[id] = ch
	t.Unlock()

	select {
	case response := <-ch:
		return response, nil
	case <-ctx.Done():
		t.Lock()
		delete(t.pendingRequests, id)
		t.Unlock()
		return nil, ctx.Err()
	}
}

// NewBaseTransport creates a new BaseTransport
func NewBaseTransport() *BaseTransport {
	return &BaseTransport{
		requestHandlers:      make(map[string]RequestHandler),
		notificationHandlers: make(map[string]NotificationHandler),
		progressHandlers:     make(map[interface{}]ProgressHandler),
		nextID:               1,
		pendingRequests:      make(map[string]chan *protocol.Response),
		requestIDPrefix:      "req",
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

// RegisterProgressHandler registers a handler for progress updates
func (t *BaseTransport) RegisterProgressHandler(id interface{}, handler ProgressHandler) {
	t.Lock()
	defer t.Unlock()
	t.progressHandlers[id] = handler
}

// UnregisterProgressHandler removes a progress handler
func (t *BaseTransport) UnregisterProgressHandler(id interface{}) {
	t.Lock()
	defer t.Unlock()
	delete(t.progressHandlers, id)
}

// GetNextID returns the next unique ID
func (t *BaseTransport) GetNextID() int64 {
	t.Lock()
	defer t.Unlock()
	id := t.nextID
	t.nextID++
	return id
}

// GenerateID generates a unique request ID
func (t *BaseTransport) GenerateID() string {
	return fmt.Sprintf("%s_%d", t.requestIDPrefix, t.GetNextID())
}

// GetRequestIDPrefix returns the prefix used for request IDs
func (t *BaseTransport) GetRequestIDPrefix() string {
	return t.requestIDPrefix
}

// HandleBatchRequest processes a batch of requests and returns a batch response
func (t *BaseTransport) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	if batch == nil || batch.Len() == 0 {
		return nil, fmt.Errorf("batch request is empty")
	}

	responses := make([]*protocol.Response, 0)

	// Process each item in the batch
	for _, item := range *batch {
		switch v := item.(type) {
		case *protocol.Request:
			// Process request and add response
			response, err := t.HandleRequest(ctx, v)
			if err != nil {
				// Create error response
				errorResp := &protocol.Response{
					JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
					ID:             v.ID,
					Error: &protocol.Error{
						Code:    protocol.InternalError,
						Message: err.Error(),
					},
				}
				responses = append(responses, errorResp)
			} else if response != nil {
				responses = append(responses, response)
			}
		case *protocol.Notification:
			// Process notification (no response expected)
			_ = t.HandleNotification(ctx, v)
			// Notifications don't generate responses in JSON-RPC 2.0
		}
	}

	// If all items were notifications, return empty response per JSON-RPC 2.0 spec
	if len(responses) == 0 {
		return &protocol.JSONRPCBatchResponse{}, nil
	}

	return protocol.NewJSONRPCBatchResponse(responses...), nil
}

// SendBatchRequest sends a batch request (default implementation processes sequentially)
func (t *BaseTransport) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	if batch == nil || batch.Len() == 0 {
		return nil, fmt.Errorf("batch request is empty")
	}

	responses := make([]*protocol.Response, 0)

	// Process each item in the batch
	for _, item := range *batch {
		switch item.(type) {
		case *protocol.Request:
			// For requests, we need to send them and wait for responses
			// This requires the transport to implement SendRequest
			// Since BaseTransport doesn't have direct send capability,
			// this default implementation returns an error
			return nil, fmt.Errorf("SendBatchRequest requires transport-specific implementation for sending requests")
		case *protocol.Notification:
			// For notifications, we need to send them without waiting for responses
			// This also requires transport-specific implementation
			return nil, fmt.Errorf("SendBatchRequest requires transport-specific implementation for sending notifications")
		}
	}

	// If we somehow get here with no items processed, return empty response
	return protocol.NewJSONRPCBatchResponse(responses...), nil
}

// Cleanup cleans up transport resources
func (t *BaseTransport) Cleanup() {
	t.Lock()
	defer t.Unlock()

	// Close any pending request channels
	for _, ch := range t.pendingRequests {
		close(ch)
	}
	t.pendingRequests = make(map[string]chan *protocol.Response)
}

// DefaultTransportConfig returns a transport configuration with sensible defaults
func DefaultTransportConfig(transportType TransportType) TransportConfig {
	return TransportConfig{
		Type: transportType,
		Features: FeatureConfig{
			EnableReliability:   true,
			EnableObservability: true,
		},
		Connection: ConnectionConfig{
			Timeout:         30 * time.Second,
			KeepAlive:       30 * time.Second,
			MaxIdleConns:    100,
			MaxConnsPerHost: 10,
			IdleConnTimeout: 90 * time.Second,
		},
		Reliability: ReliabilityConfig{
			MaxRetries:         3,
			InitialRetryDelay:  1 * time.Second,
			MaxRetryDelay:      30 * time.Second,
			RetryBackoffFactor: 2.0,
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 5,
				SuccessThreshold: 2,
				Timeout:          60 * time.Second,
			},
		},
		Observability: ObservabilityConfig{
			EnableMetrics: true,
			EnableLogging: true,
			LogLevel:      "info",
			MetricsPrefix: "mcp_transport",
		},
		Performance: PerformanceConfig{
			BufferSize:     8192,
			FlushInterval:  100 * time.Millisecond,
			MaxConcurrency: 100,
			RequestTimeout: 30 * time.Second,
		},
		Security: SecurityConfig{
			AllowedOrigins:      []string{},
			AllowWildcardOrigin: false,
		},
	}
}
