# Advanced Error Recovery Patterns

This example demonstrates comprehensive error handling and recovery strategies for production MCP applications. It implements industry-standard resilience patterns to ensure robust operation under various failure conditions.

## 🎯 Overview

Production systems must handle various types of failures gracefully:
- Network timeouts and connection issues
- Service overload and resource exhaustion
- Cascading failures across dependent services
- Transient errors that resolve themselves
- Partial system failures

This guide implements four key resilience patterns:

1. **Circuit Breaker Pattern** - Prevents cascading failures
2. **Retry Pattern with Exponential Backoff** - Handles transient failures
3. **Bulkhead Pattern** - Isolates resources and prevents resource exhaustion
4. **Comprehensive Metrics** - Monitor and tune resilience behavior

## 🔧 Resilience Patterns

### 1. Circuit Breaker Pattern

The Circuit Breaker pattern monitors failures and "opens" the circuit when failure rates exceed thresholds, preventing further calls to failing services.

```go
type CircuitBreaker struct {
    state      CircuitState  // CLOSED, OPEN, HALF_OPEN
    failures   int64         // Current failure count
    successes  int64         // Current success count
    config     CircuitBreakerConfig
}

// States:
// CLOSED   - Normal operation, requests pass through
// OPEN     - Failures detected, requests fail fast
// HALF_OPEN - Testing if service recovered
```

#### Configuration

```go
config := CircuitBreakerConfig{
    FailureThreshold:   5,                // Open after 5 failures
    RecoveryTimeout:    60 * time.Second, // Wait 60s before retry
    SuccessThreshold:   3,                // Close after 3 successes
    RequestTimeout:     30 * time.Second, // Timeout individual requests
    MonitoringWindow:   2 * time.Minute,  // Failure rate window
}
```

#### Key Features

- **Failure Detection**: Monitors error rates and patterns
- **Fast Fail**: Immediately fails requests when circuit is open
- **Automatic Recovery**: Tests service recovery periodically
- **Configurable Thresholds**: Customizable failure and success thresholds
- **Metrics**: Comprehensive metrics for monitoring

### 2. Retry Pattern with Exponential Backoff

Automatically retries failed operations with increasing delays to handle transient failures.

```go
type RetryManager struct {
    config   RetryConfig
    attempts map[string]int // Track attempts per operation
}

// Exponential backoff formula:
// delay = initial_delay * (backoff_factor ^ attempt_number)
// With jitter: delay ± (10% random variation)
```

#### Configuration

```go
config := RetryConfig{
    MaxAttempts:     3,                              // Maximum retry attempts
    InitialDelay:    1 * time.Second,                // Initial delay
    MaxDelay:        30 * time.Second,               // Maximum delay cap
    BackoffFactor:   2.0,                            // Exponential multiplier
    JitterEnabled:   true,                           // Add randomness
    RetryableErrors: []string{"timeout", "connection"}, // Which errors to retry
}
```

#### Key Features

- **Exponential Backoff**: Delays increase exponentially
- **Jitter**: Prevents thundering herd problems
- **Selective Retry**: Only retries specific error types
- **Context Awareness**: Respects context cancellation
- **Attempt Tracking**: Tracks retry attempts per operation

### 3. Bulkhead Pattern

Isolates resources into separate pools to prevent one failing component from affecting others.

```go
type Bulkhead struct {
    semaphore chan struct{}     // Concurrency control
    queue     chan *request     // Request queue
    config    BulkheadConfig
}

// Resource isolation:
// - Tools operations: 5 concurrent, 20 queued
// - Resource operations: 10 concurrent, 50 queued
// - Completion operations: 3 concurrent, 10 queued
```

#### Configuration

```go
bulkheads := map[string]BulkheadConfig{
    "tools": {
        MaxConcurrentRequests: 5,                // 5 concurrent tool calls
        QueueSize:            20,               // 20 queued requests
        RequestTimeout:       15 * time.Second, // 15s timeout
        ResourceName:         "tools",
    },
    "resources": {
        MaxConcurrentRequests: 10,
        QueueSize:            50,
        RequestTimeout:       30 * time.Second,
        ResourceName:         "resources",
    },
}
```

#### Key Features

- **Resource Isolation**: Separate pools for different operations
- **Queue Management**: Configurable request queuing
- **Timeout Protection**: Request-level timeouts
- **Graceful Degradation**: Fails fast when resources exhausted
- **Metrics**: Tracks queue depth, rejections, and wait times

### 4. Resilient MCP Client

Combines all patterns into a production-ready client implementation.

```go
type ResilientMCPClient struct {
    client         *client.Client
    circuitBreaker *CircuitBreaker
    retryManager   *RetryManager
    bulkheads      map[string]*Bulkhead
    metrics        *ResilientClientMetrics
}
```

## 📊 Metrics and Monitoring

### Circuit Breaker Metrics

```go
type CircuitBreakerMetrics struct {
    State            CircuitState  `json:"state"`
    TotalRequests    int64         `json:"total_requests"`
    TotalFailures    int64         `json:"total_failures"`
    TotalSuccesses   int64         `json:"total_successes"`
    CurrentFailures  int64         `json:"current_failures"`
    CurrentSuccesses int64         `json:"current_successes"`
    LastFailure      time.Time     `json:"last_failure"`
    LastSuccess      time.Time     `json:"last_success"`
}
```

### Bulkhead Metrics

```go
type BulkheadMetrics struct {
    ActiveRequests   int64         `json:"active_requests"`
    QueuedRequests   int64         `json:"queued_requests"`
    RejectedRequests int64         `json:"rejected_requests"`
    CompletedRequests int64        `json:"completed_requests"`
    AverageWaitTime  time.Duration `json:"average_wait_time"`
}
```

### Client Metrics

```go
type ResilientClientMetrics struct {
    TotalRequests       int64         `json:"total_requests"`
    SuccessfulRequests  int64         `json:"successful_requests"`
    FailedRequests      int64         `json:"failed_requests"`
    RetriedRequests     int64         `json:"retried_requests"`
    CircuitBreakerTrips int64         `json:"circuit_breaker_trips"`
    BulkheadRejections  int64         `json:"bulkhead_rejections"`
    AverageLatency      time.Duration `json:"average_latency"`
    P95Latency          time.Duration `json:"p95_latency"`
    P99Latency          time.Duration `json:"p99_latency"`
}
```

## 🚀 Usage Examples

### Basic Resilient Client

```go
// Configure resilience patterns
config := ResilientClientConfig{
    CircuitBreaker: CircuitBreakerConfig{
        FailureThreshold: 3,
        RecoveryTimeout:  30 * time.Second,
        SuccessThreshold: 2,
    },
    Retry: RetryConfig{
        MaxAttempts:   3,
        InitialDelay:  1 * time.Second,
        BackoffFactor: 2.0,
        JitterEnabled: true,
    },
    Bulkheads: map[string]BulkheadConfig{
        "tools": {
            MaxConcurrentRequests: 5,
            QueueSize:            20,
            RequestTimeout:       15 * time.Second,
        },
    },
}

// Create resilient client
transport := createTransport()
client := NewResilientMCPClient(transport, config)
defer client.Stop()

// Use normally - resilience is automatic
tools, err := client.ListTools(ctx, nil)
if err != nil {
    log.Printf("Operation failed after all resilience attempts: %v", err)
}
```

### Custom Error Classification

```go
// Define custom retry logic
retryConfig := RetryConfig{
    MaxAttempts: 5,
    RetryableErrors: []string{
        "timeout",
        "connection refused",
        "temporary failure",
        "circuit breaker",
    },
    IsRetryableFunc: func(err error) bool {
        // Custom logic for determining retryable errors
        switch e := err.(type) {
        case *MCPError:
            return e.Code == ErrorCodeTemporary
        case *NetworkError:
            return e.Retryable
        default:
            return false
        }
    },
}
```

### Health Monitoring

```go
// Set up health monitoring
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            metrics := client.GetMetrics()

            // Check health indicators
            successRate := float64(metrics.SuccessfulRequests) /
                          float64(metrics.TotalRequests)

            if successRate < 0.95 {
                log.Printf("WARNING: Success rate below 95%%: %.2f%%",
                          successRate*100)
            }

            if metrics.CircuitBreakerTrips > 0 {
                log.Printf("Circuit breaker trips detected: %d",
                          metrics.CircuitBreakerTrips)
            }

            if metrics.BulkheadRejections > 0 {
                log.Printf("Bulkhead rejections: %d",
                          metrics.BulkheadRejections)
            }

        case <-ctx.Done():
            return
        }
    }
}()
```

## 🧪 Testing Resilience

### Chaos Testing

```go
func TestCircuitBreakerUnderFailure(t *testing.T) {
    // Create a transport that fails
    failingTransport := &FailingTransport{
        failureRate: 0.8, // 80% failure rate
    }

    config := ResilientClientConfig{
        CircuitBreaker: CircuitBreakerConfig{
            FailureThreshold: 3,
            RecoveryTimeout:  1 * time.Second,
        },
    }

    client := NewResilientMCPClient(failingTransport, config)

    // Generate load
    for i := 0; i < 20; i++ {
        _, err := client.ListTools(context.Background(), nil)
        // Should fail fast after circuit opens
    }

    metrics := client.GetMetrics()
    assert.Greater(t, metrics.CircuitBreakerTrips, int64(0))
}
```

### Load Testing

```go
func TestBulkheadUnderLoad(t *testing.T) {
    config := ResilientClientConfig{
        Bulkheads: map[string]BulkheadConfig{
            "tools": {
                MaxConcurrentRequests: 2,
                QueueSize:            5,
            },
        },
    }

    client := NewResilientMCPClient(transport, config)

    // Generate high concurrent load
    var wg sync.WaitGroup
    errors := make([]error, 100)

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            _, err := client.CallTool(context.Background(), "test", nil)
            errors[idx] = err
        }(i)
    }

    wg.Wait()

    // Count rejections
    rejections := 0
    for _, err := range errors {
        if _, isBulkheadError := err.(*BulkheadError); isBulkheadError {
            rejections++
        }
    }

    assert.Greater(t, rejections, 0, "Expected some requests to be rejected")
}
```

## 🔧 Configuration Best Practices

### Production Settings

```go
// Conservative settings for critical services
productionConfig := ResilientClientConfig{
    CircuitBreaker: CircuitBreakerConfig{
        FailureThreshold:   3,                // Open quickly
        RecoveryTimeout:    60 * time.Second, // Conservative recovery
        SuccessThreshold:   5,                // Require stable success
        RequestTimeout:     30 * time.Second,
        MonitoringWindow:   5 * time.Minute,
    },
    Retry: RetryConfig{
        MaxAttempts:     5,                   // More retry attempts
        InitialDelay:    500 * time.Millisecond,
        MaxDelay:        30 * time.Second,
        BackoffFactor:   2.0,
        JitterEnabled:   true,                // Prevent thundering herd
    },
    Bulkheads: map[string]BulkheadConfig{
        "critical": {
            MaxConcurrentRequests: 20,
            QueueSize:            100,
            RequestTimeout:       45 * time.Second,
        },
        "bulk": {
            MaxConcurrentRequests: 50,
            QueueSize:            200,
            RequestTimeout:       60 * time.Second,
        },
    },
}
```

### Development Settings

```go
// Faster feedback for development
devConfig := ResilientClientConfig{
    CircuitBreaker: CircuitBreakerConfig{
        FailureThreshold:   2,                // Fail faster
        RecoveryTimeout:    10 * time.Second, // Quick recovery
        SuccessThreshold:   2,                // Quick close
        RequestTimeout:     10 * time.Second,
    },
    Retry: RetryConfig{
        MaxAttempts:     3,
        InitialDelay:    100 * time.Millisecond,
        MaxDelay:        5 * time.Second,
        BackoffFactor:   1.5,
        JitterEnabled:   false,               // Predictable timing
    },
}
```

## 📈 Performance Impact

### Overhead Analysis

| Pattern | CPU Overhead | Memory Overhead | Latency Impact |
|---------|-------------|----------------|----------------|
| Circuit Breaker | ~0.1% | ~1KB per instance | <1ms |
| Retry | Variable | ~100B per operation | 0-30s (on failure) |
| Bulkhead | ~0.2% | ~10KB per bulkhead | <5ms queuing |
| Combined | ~0.5% | ~50KB total | <10ms normal case |

### Tuning Guidelines

1. **Circuit Breaker**:
   - Lower thresholds for faster failure detection
   - Higher thresholds for stable but occasionally failing services
   - Adjust recovery timeout based on service recovery patterns

2. **Retry**:
   - Fewer attempts for user-facing operations
   - More attempts for background operations
   - Enable jitter in high-traffic scenarios

3. **Bulkhead**:
   - Size pools based on expected load and resource capacity
   - Monitor queue depths and adjust accordingly
   - Use separate bulkheads for different operation types

## 🚨 Error Types and Handling

### Error Classification

```go
// Retryable errors - temporary issues
type TemporaryError struct {
    Message string
    Cause   error
}

// Circuit breaker errors - fast fail
type CircuitBreakerError struct {
    State   CircuitState
    Message string
}

// Bulkhead errors - resource exhaustion
type BulkheadError struct {
    ResourceName string
    QueueFull    bool
}

// Non-retryable errors - permanent issues
type PermanentError struct {
    Code    string
    Message string
}
```

### Error Handling Strategy

```go
func handleMCPError(err error) RecoveryAction {
    switch e := err.(type) {
    case *TemporaryError:
        return RetryWithBackoff
    case *CircuitBreakerError:
        return FailFast
    case *BulkheadError:
        return QueueOrReject
    case *PermanentError:
        return LogAndFail
    default:
        return RetryLimited
    }
}
```

## 🔍 Troubleshooting

### Common Issues

#### High Circuit Breaker Trip Rate
```bash
# Check failure patterns
grep "Circuit breaker state changed" app.log | tail -20

# Monitor error rates
curl http://localhost:8080/metrics | grep circuit_breaker
```

**Solutions**:
- Increase failure threshold if false positives
- Investigate root cause of failures
- Adjust recovery timeout

#### Bulkhead Queue Full Errors
```bash
# Monitor bulkhead metrics
curl http://localhost:8080/metrics | grep bulkhead_queue

# Check concurrent request patterns
grep "BulkheadError" app.log | wc -l
```

**Solutions**:
- Increase queue size or concurrent request limit
- Implement request priority queuing
- Add auto-scaling for additional capacity

#### Excessive Retry Attempts
```bash
# Monitor retry patterns
grep "Retrying in" app.log | awk '{print $NF}' | sort | uniq -c
```

**Solutions**:
- Reduce max attempts for time-sensitive operations
- Implement exponential backoff limits
- Classify errors more precisely

## 📚 Additional Resources

- [Microservices Patterns - Chris Richardson](https://microservices.io/patterns/)
- [Release It! - Michael Nygard](https://pragprog.com/titles/mnee2/release-it-second-edition/)
- [Building Secure & Reliable Systems - Google](https://sre.google/books/)
- [Site Reliability Engineering - Google](https://sre.google/books/sre-book/)

## 🤝 Integration with Observability

This error recovery system integrates seamlessly with the observability stack:

```go
// Add tracing to resilience patterns
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func(context.Context) error) error {
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(
        attribute.String("circuit_breaker.state", cb.getStateString()),
        attribute.Int64("circuit_breaker.failures", cb.failures),
    )

    err := operation(ctx)

    if err != nil {
        span.SetStatus(codes.Error, err.Error())
        span.SetAttributes(attribute.Bool("circuit_breaker.failure", true))
    } else {
        span.SetAttributes(attribute.Bool("circuit_breaker.success", true))
    }

    return err
}
```

## ⚡ Quick Start

```bash
# Run the example
go run main.go

# Monitor metrics (if metrics endpoint enabled)
curl http://localhost:8080/metrics

# View logs
tail -f app.log | grep -E "(Circuit|Bulkhead|Retry)"
```

This comprehensive error recovery system ensures your MCP applications remain resilient and performant under various failure conditions while providing detailed insights into system behavior.
