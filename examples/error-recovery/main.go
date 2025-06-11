// Advanced Error Recovery Patterns for MCP Go SDK
// This example demonstrates comprehensive error handling, recovery strategies,
// and resilience patterns for production MCP applications.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/examples/shared"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// Error Recovery Patterns

// 1. Circuit Breaker Pattern
// Prevents cascading failures by monitoring error rates

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures to open circuit
	RecoveryTimeout  time.Duration // Time to wait before trying again
	SuccessThreshold int           // Consecutive successes to close circuit
	RequestTimeout   time.Duration // Timeout for individual requests
	MonitoringWindow time.Duration // Window for failure rate calculation
}

type CircuitBreaker struct {
	config CircuitBreakerConfig

	state       CircuitState
	failures    int64
	successes   int64
	lastFailure time.Time
	lastSuccess time.Time

	mutex sync.RWMutex

	// Metrics for monitoring
	totalRequests  int64
	totalFailures  int64
	totalSuccesses int64

	// State change callbacks
	onStateChange func(from, to CircuitState)
}

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.RecoveryTimeout == 0 {
		config.RecoveryTimeout = 60 * time.Second
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 3
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.MonitoringWindow == 0 {
		config.MonitoringWindow = 2 * time.Minute
	}

	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

func (cb *CircuitBreaker) Execute(ctx context.Context, operation func(context.Context) error) error {
	atomic.AddInt64(&cb.totalRequests, 1)

	if cb.getState() == CircuitOpen {
		return &CircuitBreakerError{
			Type:    "circuit_open",
			Message: "Circuit breaker is open",
		}
	}

	// Create timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, cb.config.RequestTimeout)
	defer cancel()

	// Execute the operation
	err := operation(opCtx)

	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) getState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	now := time.Now()

	switch cb.state {
	case CircuitOpen:
		if now.Sub(cb.lastFailure) >= cb.config.RecoveryTimeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			if cb.state == CircuitOpen && now.Sub(cb.lastFailure) >= cb.config.RecoveryTimeout {
				cb.setState(CircuitHalfOpen)
				cb.successes = 0
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
		}
	case CircuitHalfOpen:
		if cb.successes >= int64(cb.config.SuccessThreshold) {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			if cb.state == CircuitHalfOpen && cb.successes >= int64(cb.config.SuccessThreshold) {
				cb.setState(CircuitClosed)
				cb.failures = 0
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
		}
	}

	return cb.state
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	atomic.AddInt64(&cb.totalFailures, 1)
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == CircuitClosed && cb.failures >= int64(cb.config.FailureThreshold) {
		cb.setState(CircuitOpen)
	} else if cb.state == CircuitHalfOpen {
		cb.setState(CircuitOpen)
		cb.failures = 0
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	atomic.AddInt64(&cb.totalSuccesses, 1)
	cb.successes++
	cb.lastSuccess = time.Now()

	if cb.state == CircuitHalfOpen && cb.successes >= int64(cb.config.SuccessThreshold) {
		cb.setState(CircuitClosed)
		cb.failures = 0
	} else if cb.state == CircuitClosed {
		cb.failures = 0 // Reset failure count on success
	}
}

func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState

	if cb.onStateChange != nil && oldState != newState {
		go cb.onStateChange(oldState, newState)
	}
}

func (cb *CircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return CircuitBreakerMetrics{
		State:            cb.state,
		TotalRequests:    atomic.LoadInt64(&cb.totalRequests),
		TotalFailures:    atomic.LoadInt64(&cb.totalFailures),
		TotalSuccesses:   atomic.LoadInt64(&cb.totalSuccesses),
		CurrentFailures:  cb.failures,
		CurrentSuccesses: cb.successes,
		LastFailure:      cb.lastFailure,
		LastSuccess:      cb.lastSuccess,
	}
}

type CircuitBreakerMetrics struct {
	State            CircuitState `json:"state"`
	TotalRequests    int64        `json:"total_requests"`
	TotalFailures    int64        `json:"total_failures"`
	TotalSuccesses   int64        `json:"total_successes"`
	CurrentFailures  int64        `json:"current_failures"`
	CurrentSuccesses int64        `json:"current_successes"`
	LastFailure      time.Time    `json:"last_failure"`
	LastSuccess      time.Time    `json:"last_success"`
}

type CircuitBreakerError struct {
	Type    string
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return fmt.Sprintf("CircuitBreaker[%s]: %s", e.Type, e.Message)
}

// 2. Retry Pattern with Exponential Backoff and Jitter

type RetryConfig struct {
	MaxAttempts     int           // Maximum number of retry attempts
	InitialDelay    time.Duration // Initial delay between retries
	MaxDelay        time.Duration // Maximum delay between retries
	BackoffFactor   float64       // Exponential backoff multiplier
	JitterEnabled   bool          // Add randomness to prevent thundering herd
	RetryableErrors []string      // Error types that should trigger retries
}

type RetryManager struct {
	config   RetryConfig
	attempts map[string]int // Track attempts per operation
	mutex    sync.RWMutex
}

func NewRetryManager(config RetryConfig) *RetryManager {
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay == 0 {
		config.InitialDelay = 1 * time.Second
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.BackoffFactor == 0 {
		config.BackoffFactor = 2.0
	}
	if len(config.RetryableErrors) == 0 {
		config.RetryableErrors = []string{"timeout", "connection", "temporary"}
	}

	return &RetryManager{
		config:   config,
		attempts: make(map[string]int),
	}
}

func (rm *RetryManager) ExecuteWithRetry(ctx context.Context, operationID string, operation func(context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt < rm.config.MaxAttempts; attempt++ {
		// Execute the operation
		err := operation(ctx)
		if err == nil {
			rm.resetAttempts(operationID)
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !rm.isRetryableError(err) {
			return err
		}

		// Check if we've reached max attempts
		if attempt == rm.config.MaxAttempts-1 {
			break
		}

		// Calculate delay with exponential backoff and jitter
		delay := rm.calculateDelay(attempt)

		log.Printf("Operation %s failed (attempt %d/%d): %v. Retrying in %v",
			operationID, attempt+1, rm.config.MaxAttempts, err, delay)

		// Wait for the delay or context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	rm.resetAttempts(operationID)
	return fmt.Errorf("operation failed after %d attempts: %w", rm.config.MaxAttempts, lastErr)
}

func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: delay = initial * (factor ^ attempt)
	delay := float64(rm.config.InitialDelay) * math.Pow(rm.config.BackoffFactor, float64(attempt))

	// Cap at max delay
	if delay > float64(rm.config.MaxDelay) {
		delay = float64(rm.config.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if rm.config.JitterEnabled {
		jitter := rand.Float64() * 0.1 * delay // ±10% jitter
		delay += jitter - (0.1 * delay / 2)
	}

	return time.Duration(delay)
}

func (rm *RetryManager) isRetryableError(err error) bool {
	errStr := err.Error()
	for _, retryableType := range rm.config.RetryableErrors {
		if containsString(errStr, retryableType) {
			return true
		}
	}
	return false
}

func (rm *RetryManager) resetAttempts(operationID string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	delete(rm.attempts, operationID)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && s[1:len(substr)+1] == substr)))
}

// 3. Bulkhead Pattern - Isolate Resources

type BulkheadConfig struct {
	MaxConcurrentRequests int
	QueueSize             int
	RequestTimeout        time.Duration
	ResourceName          string
}

type Bulkhead struct {
	config    BulkheadConfig
	semaphore chan struct{}
	queue     chan *bulkheadRequest
	metrics   *BulkheadMetrics

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type bulkheadRequest struct {
	operation func(context.Context) error
	response  chan error
	ctx       context.Context
}

type BulkheadMetrics struct {
	ActiveRequests    int64         `json:"active_requests"`
	QueuedRequests    int64         `json:"queued_requests"`
	RejectedRequests  int64         `json:"rejected_requests"`
	CompletedRequests int64         `json:"completed_requests"`
	AverageWaitTime   time.Duration `json:"average_wait_time"`
}

func NewBulkhead(config BulkheadConfig) *Bulkhead {
	if config.MaxConcurrentRequests == 0 {
		config.MaxConcurrentRequests = 10
	}
	if config.QueueSize == 0 {
		config.QueueSize = 100
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &Bulkhead{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrentRequests),
		queue:     make(chan *bulkheadRequest, config.QueueSize),
		metrics:   &BulkheadMetrics{},
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start worker goroutines
	for i := 0; i < config.MaxConcurrentRequests; i++ {
		b.wg.Add(1)
		go b.worker()
	}

	return b
}

func (b *Bulkhead) Execute(ctx context.Context, operation func(context.Context) error) error {
	request := &bulkheadRequest{
		operation: operation,
		response:  make(chan error, 1),
		ctx:       ctx,
	}

	// Try to enqueue the request
	select {
	case b.queue <- request:
		atomic.AddInt64(&b.metrics.QueuedRequests, 1)
	default:
		atomic.AddInt64(&b.metrics.RejectedRequests, 1)
		return &BulkheadError{
			Type:         "queue_full",
			Message:      fmt.Sprintf("Bulkhead queue full for resource %s", b.config.ResourceName),
			ResourceName: b.config.ResourceName,
		}
	}

	// Wait for response
	select {
	case err := <-request.response:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-b.ctx.Done():
		return errors.New("bulkhead shutting down")
	}
}

func (b *Bulkhead) worker() {
	defer b.wg.Done()

	for {
		select {
		case request := <-b.queue:
			atomic.AddInt64(&b.metrics.QueuedRequests, -1)
			atomic.AddInt64(&b.metrics.ActiveRequests, 1)

			// Execute with timeout
			opCtx, cancel := context.WithTimeout(request.ctx, b.config.RequestTimeout)
			err := request.operation(opCtx)
			cancel()

			atomic.AddInt64(&b.metrics.ActiveRequests, -1)
			atomic.AddInt64(&b.metrics.CompletedRequests, 1)

			// Send response
			select {
			case request.response <- err:
			default:
				// Request context cancelled, ignore
			}

		case <-b.ctx.Done():
			return
		}
	}
}

func (b *Bulkhead) Stop() {
	b.cancel()
	b.wg.Wait()
}

func (b *Bulkhead) GetMetrics() BulkheadMetrics {
	return BulkheadMetrics{
		ActiveRequests:    atomic.LoadInt64(&b.metrics.ActiveRequests),
		QueuedRequests:    atomic.LoadInt64(&b.metrics.QueuedRequests),
		RejectedRequests:  atomic.LoadInt64(&b.metrics.RejectedRequests),
		CompletedRequests: atomic.LoadInt64(&b.metrics.CompletedRequests),
		AverageWaitTime:   b.metrics.AverageWaitTime,
	}
}

type BulkheadError struct {
	Type         string
	Message      string
	ResourceName string
}

func (e *BulkheadError) Error() string {
	return fmt.Sprintf("Bulkhead[%s]: %s", e.Type, e.Message)
}

// 4. Resilient MCP Client Implementation

type ResilientMCPClient struct {
	client         client.Client
	circuitBreaker *CircuitBreaker
	retryManager   *RetryManager
	bulkheads      map[string]*Bulkhead

	config ResilientClientConfig

	metrics *ResilientClientMetrics
	mutex   sync.RWMutex
}

type ResilientClientConfig struct {
	CircuitBreaker CircuitBreakerConfig
	Retry          RetryConfig
	Bulkheads      map[string]BulkheadConfig // Resource type -> bulkhead config

	EnableMetrics       bool
	MetricsInterval     time.Duration
	HealthCheckInterval time.Duration
}

type ResilientClientMetrics struct {
	TotalRequests       int64 `json:"total_requests"`
	SuccessfulRequests  int64 `json:"successful_requests"`
	FailedRequests      int64 `json:"failed_requests"`
	RetriedRequests     int64 `json:"retried_requests"`
	CircuitBreakerTrips int64 `json:"circuit_breaker_trips"`
	BulkheadRejections  int64 `json:"bulkhead_rejections"`

	AverageLatency time.Duration `json:"average_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
}

func NewResilientMCPClient(transport transport.Transport, config ResilientClientConfig) *ResilientMCPClient {
	// Set defaults
	if config.MetricsInterval == 0 {
		config.MetricsInterval = 30 * time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 60 * time.Second
	}

	rmc := &ResilientMCPClient{
		client:         client.New(transport),
		circuitBreaker: NewCircuitBreaker(config.CircuitBreaker),
		retryManager:   NewRetryManager(config.Retry),
		bulkheads:      make(map[string]*Bulkhead),
		config:         config,
		metrics:        &ResilientClientMetrics{},
	}

	// Create bulkheads for each resource type
	for resourceType, bulkheadConfig := range config.Bulkheads {
		rmc.bulkheads[resourceType] = NewBulkhead(bulkheadConfig)
	}

	// Set up circuit breaker state change monitoring
	rmc.circuitBreaker.onStateChange = func(from, to CircuitState) {
		log.Printf("Circuit breaker state changed: %s -> %s",
			rmc.stateString(from), rmc.stateString(to))
		if to == CircuitOpen {
			atomic.AddInt64(&rmc.metrics.CircuitBreakerTrips, 1)
		}
	}

	return rmc
}

func (rmc *ResilientMCPClient) stateString(state CircuitState) string {
	switch state {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

func (rmc *ResilientMCPClient) Initialize(ctx context.Context) error {
	return rmc.executeWithResilience(ctx, "initialize", func(ctx context.Context) error {
		return rmc.client.Initialize(ctx)
	})
}

func (rmc *ResilientMCPClient) ListTools(ctx context.Context, pagination *protocol.PaginationParams) ([]protocol.Tool, error) {
	var result []protocol.Tool
	err := rmc.executeWithResilience(ctx, "list_tools", func(ctx context.Context) error {
		tools, _, err := rmc.client.ListTools(ctx, "", pagination)
		if err != nil {
			return err
		}
		result = tools
		return nil
	})
	return result, err
}

func (rmc *ResilientMCPClient) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*protocol.CallToolResult, error) {
	var result *protocol.CallToolResult
	err := rmc.executeWithResilience(ctx, "call_tool", func(ctx context.Context) error {
		res, err := rmc.client.CallTool(ctx, name, arguments, nil)
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func (rmc *ResilientMCPClient) ListResources(ctx context.Context, pagination *protocol.PaginationParams) ([]protocol.Resource, error) {
	var result []protocol.Resource
	err := rmc.executeWithResilience(ctx, "list_resources", func(ctx context.Context) error {
		resources, _, _, err := rmc.client.ListResources(ctx, "", false, pagination)
		if err != nil {
			return err
		}
		result = resources
		return nil
	})
	return result, err
}

func (rmc *ResilientMCPClient) ReadResource(ctx context.Context, uri string) (*protocol.ResourceContents, error) {
	var result *protocol.ResourceContents
	err := rmc.executeWithResilience(ctx, "read_resource", func(ctx context.Context) error {
		res, err := rmc.client.ReadResource(ctx, uri, nil, nil)
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func (rmc *ResilientMCPClient) executeWithResilience(ctx context.Context, operationType string, operation func(context.Context) error) error {
	startTime := time.Now()
	atomic.AddInt64(&rmc.metrics.TotalRequests, 1)

	defer func() {
		duration := time.Since(startTime)
		rmc.updateLatencyMetrics(duration)
	}()

	// Get bulkhead for this operation type
	bulkhead := rmc.getBulkhead(operationType)

	// Execute through bulkhead if available
	if bulkhead != nil {
		err := bulkhead.Execute(ctx, func(bulkheadCtx context.Context) error {
			return rmc.executeWithCircuitBreakerAndRetry(bulkheadCtx, operationType, operation)
		})

		if err != nil {
			if _, isBulkheadError := err.(*BulkheadError); isBulkheadError {
				atomic.AddInt64(&rmc.metrics.BulkheadRejections, 1)
			}
		}

		return err
	}

	// Execute without bulkhead
	return rmc.executeWithCircuitBreakerAndRetry(ctx, operationType, operation)
}

func (rmc *ResilientMCPClient) executeWithCircuitBreakerAndRetry(ctx context.Context, operationType string, operation func(context.Context) error) error {
	// Execute with retry and circuit breaker
	err := rmc.retryManager.ExecuteWithRetry(ctx, operationType, func(retryCtx context.Context) error {
		return rmc.circuitBreaker.Execute(retryCtx, operation)
	})

	if err != nil {
		atomic.AddInt64(&rmc.metrics.FailedRequests, 1)
		return err
	}

	atomic.AddInt64(&rmc.metrics.SuccessfulRequests, 1)
	return nil
}

func (rmc *ResilientMCPClient) getBulkhead(operationType string) *Bulkhead {
	rmc.mutex.RLock()
	defer rmc.mutex.RUnlock()
	return rmc.bulkheads[operationType]
}

func (rmc *ResilientMCPClient) updateLatencyMetrics(duration time.Duration) {
	// Simplified latency tracking - in production, use proper histogram
	rmc.mutex.Lock()
	defer rmc.mutex.Unlock()

	// Update average latency (simple moving average)
	if rmc.metrics.AverageLatency == 0 {
		rmc.metrics.AverageLatency = duration
	} else {
		rmc.metrics.AverageLatency = (rmc.metrics.AverageLatency + duration) / 2
	}

	// Update percentiles (simplified)
	if duration > rmc.metrics.P95Latency {
		rmc.metrics.P95Latency = duration
	}
	if duration > rmc.metrics.P99Latency {
		rmc.metrics.P99Latency = duration
	}
}

func (rmc *ResilientMCPClient) GetMetrics() ResilientClientMetrics {
	rmc.mutex.RLock()
	defer rmc.mutex.RUnlock()

	return ResilientClientMetrics{
		TotalRequests:       atomic.LoadInt64(&rmc.metrics.TotalRequests),
		SuccessfulRequests:  atomic.LoadInt64(&rmc.metrics.SuccessfulRequests),
		FailedRequests:      atomic.LoadInt64(&rmc.metrics.FailedRequests),
		RetriedRequests:     atomic.LoadInt64(&rmc.metrics.RetriedRequests),
		CircuitBreakerTrips: atomic.LoadInt64(&rmc.metrics.CircuitBreakerTrips),
		BulkheadRejections:  atomic.LoadInt64(&rmc.metrics.BulkheadRejections),
		AverageLatency:      rmc.metrics.AverageLatency,
		P95Latency:          rmc.metrics.P95Latency,
		P99Latency:          rmc.metrics.P99Latency,
	}
}

func (rmc *ResilientMCPClient) Stop() {
	for _, bulkhead := range rmc.bulkheads {
		bulkhead.Stop()
	}
}

// 5. Demonstration Functions

func demonstrateErrorRecoveryPatterns() {
	log.Println("=== Advanced Error Recovery Patterns Demo ===")

	// Create transport
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create resilient client with comprehensive error recovery
	clientConfig := ResilientClientConfig{
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: 3,
			RecoveryTimeout:  30 * time.Second,
			SuccessThreshold: 2,
			RequestTimeout:   10 * time.Second,
		},
		Retry: RetryConfig{
			MaxAttempts:     3,
			InitialDelay:    1 * time.Second,
			MaxDelay:        10 * time.Second,
			BackoffFactor:   2.0,
			JitterEnabled:   true,
			RetryableErrors: []string{"timeout", "connection", "temporary"},
		},
		Bulkheads: map[string]BulkheadConfig{
			"list_tools": {
				MaxConcurrentRequests: 5,
				QueueSize:             20,
				RequestTimeout:        15 * time.Second,
				ResourceName:          "tools",
			},
			"call_tool": {
				MaxConcurrentRequests: 3,
				QueueSize:             10,
				RequestTimeout:        30 * time.Second,
				ResourceName:          "tool_execution",
			},
		},
		EnableMetrics:       true,
		MetricsInterval:     10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}

	resilientClient := NewResilientMCPClient(t, clientConfig)
	defer resilientClient.Stop()

	// Create server for testing
	mcpServer := server.New(t,
		server.WithName("Error Recovery Demo Server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(shared.CreateToolsProvider()),
		server.WithResourcesProvider(shared.CreateResourcesProvider()),
		server.WithPromptsProvider(shared.CreatePromptsProvider()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start server
	go func() {
		if err := mcpServer.Start(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Demonstrate resilient operations
	log.Println("\n1. Testing normal operations")
	testNormalOperations(ctx, resilientClient)

	log.Println("\n2. Testing error recovery")
	testErrorRecovery(ctx, resilientClient)

	log.Println("\n3. Testing bulkhead isolation")
	testBulkheadIsolation(ctx, resilientClient)

	log.Println("\n4. Final metrics")
	printMetrics(resilientClient)

	mcpServer.Stop()
}

func testNormalOperations(ctx context.Context, client *ResilientMCPClient) {
	// Initialize client
	err := client.Initialize(ctx)
	if err != nil {
		log.Printf("Initialize failed: %v", err)
		return
	}
	log.Println("✓ Client initialized successfully")

	// List tools
	tools, err := client.ListTools(ctx, nil)
	if err != nil {
		log.Printf("ListTools failed: %v", err)
		return
	}
	log.Printf("✓ Listed %d tools successfully", len(tools))

	// List resources
	resources, err := client.ListResources(ctx, nil)
	if err != nil {
		log.Printf("ListResources failed: %v", err)
		return
	}
	log.Printf("✓ Listed %d resources successfully", len(resources))
}

func testErrorRecovery(ctx context.Context, client *ResilientMCPClient) {
	// Simulate operations that might fail
	for i := 0; i < 10; i++ {
		_, err := client.CallTool(ctx, "potentially_failing_tool", json.RawMessage(`{"test": true}`))
		if err != nil {
			log.Printf("Tool call %d failed (expected): %v", i+1, err)
		} else {
			log.Printf("✓ Tool call %d succeeded", i+1)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func testBulkheadIsolation(ctx context.Context, client *ResilientMCPClient) {
	// Test concurrent operations to demonstrate bulkhead isolation
	var wg sync.WaitGroup

	// High load on tool operations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := client.CallTool(ctx, fmt.Sprintf("tool_%d", i), json.RawMessage(`{}`))
			if err != nil {
				log.Printf("Concurrent tool call %d failed: %v", i, err)
			}
		}(i)
	}

	// Resource operations should still work despite tool overload
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := client.ListResources(ctx, nil)
			if err != nil {
				log.Printf("Concurrent resource list %d failed: %v", i, err)
			} else {
				log.Printf("✓ Resource list %d succeeded despite tool overload", i)
			}
		}(i)
	}

	wg.Wait()
}

func printMetrics(client *ResilientMCPClient) {
	metrics := client.GetMetrics()

	log.Println("=== Resilient Client Metrics ===")
	log.Printf("Total Requests: %d", metrics.TotalRequests)
	log.Printf("Successful Requests: %d", metrics.SuccessfulRequests)
	log.Printf("Failed Requests: %d", metrics.FailedRequests)
	log.Printf("Circuit Breaker Trips: %d", metrics.CircuitBreakerTrips)
	log.Printf("Bulkhead Rejections: %d", metrics.BulkheadRejections)
	log.Printf("Average Latency: %v", metrics.AverageLatency)
	log.Printf("P95 Latency: %v", metrics.P95Latency)
	log.Printf("P99 Latency: %v", metrics.P99Latency)

	if metrics.TotalRequests > 0 {
		successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests) * 100
		log.Printf("Success Rate: %.2f%%", successRate)
	}
}

func main() {
	log.Println("Advanced Error Recovery Patterns for MCP Go SDK")
	log.Println("==============================================")

	// Handle graceful shutdown
	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nReceived shutdown signal...")
		cancel()
	}()

	// Run demonstrations
	demonstrateErrorRecoveryPatterns()

	// Wait for potential cleanup
	select {
	case <-mainCtx.Done():
		log.Println("Shutdown completed")
	default:
		// Continue normally
	}

	log.Println("\n=== Error Recovery Patterns Demo Complete ===")
	log.Println("These patterns provide comprehensive resilience for production MCP applications:")
	log.Println("- Circuit Breaker: Prevents cascading failures")
	log.Println("- Retry with Backoff: Handles transient failures")
	log.Println("- Bulkhead: Isolates resources and prevents resource exhaustion")
	log.Println("- Comprehensive Metrics: Monitor and tune resilience behavior")
}
