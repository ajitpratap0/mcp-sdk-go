package transport

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"log"
	"math"
	"math/big"
	"sync"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// ReliabilityMiddleware adds retry logic, circuit breaking, and error recovery
type ReliabilityMiddleware struct {
	config         ReliabilityConfig
	circuitBreaker *reliabilityCircuitBreaker
	logger         *log.Logger
}

// NewReliabilityMiddleware creates a new reliability middleware
func NewReliabilityMiddleware(config ReliabilityConfig) Middleware {
	rm := &ReliabilityMiddleware{
		config: config,
		logger: log.New(log.Writer(), "[ReliabilityMiddleware] ", log.LstdFlags),
	}

	if config.CircuitBreaker.Enabled {
		rm.circuitBreaker = newReliabilityCircuitBreaker(config.CircuitBreaker)
	}

	return rm
}

// Wrap implements the Middleware interface
func (rm *ReliabilityMiddleware) Wrap(transport Transport) Transport {
	return &reliabilityTransport{
		middlewareTransport: middlewareTransport{next: transport},
		middleware:          rm,
	}
}

// reliabilityTransport wraps a transport with reliability features
type reliabilityTransport struct {
	middlewareTransport
	middleware *ReliabilityMiddleware
}

// SendRequest wraps the underlying SendRequest with retry logic
func (rt *reliabilityTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	config := rt.middleware.config

	// Circuit breaker check
	if rt.middleware.circuitBreaker != nil {
		if !rt.middleware.circuitBreaker.canMakeCall() {
			return nil, mcperrors.NewError(
				mcperrors.CodeResourceUnavailable,
				"Circuit breaker is open",
				mcperrors.CategoryTransport,
				mcperrors.SeverityError,
			).WithContext(&mcperrors.Context{
				Method:    method,
				Component: "ReliabilityMiddleware",
				Operation: "circuit_breaker_check",
			})
		}
	}

	var lastErr error
	maxAttempts := config.MaxRetries + 1 // Initial attempt + retries

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Calculate delay for this attempt (skip for first attempt)
		if attempt > 0 {
			delay := rt.calculateBackoff(attempt, config)
			rt.middleware.logger.Printf("Retry attempt %d/%d for %s after %v delay",
				attempt, config.MaxRetries, method, delay)

			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Make the actual request
		result, err := rt.middlewareTransport.SendRequest(ctx, method, params)

		// Success - record with circuit breaker and return
		if err == nil {
			if rt.middleware.circuitBreaker != nil {
				rt.middleware.circuitBreaker.recordSuccess()
			}
			return result, nil
		}

		// Error occurred
		lastErr = err

		// Check if error is retryable
		if !rt.isRetryableError(err) {
			rt.middleware.logger.Printf("Non-retryable error for %s: %v", method, err)
			if rt.middleware.circuitBreaker != nil {
				rt.middleware.circuitBreaker.recordFailure()
			}
			return nil, err
		}

		// Record failure with circuit breaker
		if rt.middleware.circuitBreaker != nil {
			rt.middleware.circuitBreaker.recordFailure()
		}

		rt.middleware.logger.Printf("Retryable error for %s (attempt %d/%d): %v",
			method, attempt+1, maxAttempts, err)
	}

	// All retries exhausted
	return nil, mcperrors.NewError(
		mcperrors.CodeOperationFailed,
		fmt.Sprintf("Request failed after %d attempts", maxAttempts),
		mcperrors.CategoryTransport,
		mcperrors.SeverityError,
	).WithContext(&mcperrors.Context{
		Method:    method,
		Component: "ReliabilityMiddleware",
		Operation: "retry_exhausted",
	}).WithDetail(fmt.Sprintf("Last error: %v", lastErr))
}

// SendNotification wraps the underlying SendNotification with retry logic
func (rt *reliabilityTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	config := rt.middleware.config

	// Circuit breaker check
	if rt.middleware.circuitBreaker != nil {
		if !rt.middleware.circuitBreaker.canMakeCall() {
			return mcperrors.NewError(
				mcperrors.CodeResourceUnavailable,
				"Circuit breaker is open",
				mcperrors.CategoryTransport,
				mcperrors.SeverityError,
			).WithContext(&mcperrors.Context{
				Method:    method,
				Component: "ReliabilityMiddleware",
				Operation: "circuit_breaker_check",
			})
		}
	}

	var lastErr error
	maxAttempts := config.MaxRetries + 1

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if attempt > 0 {
			delay := rt.calculateBackoff(attempt, config)
			rt.middleware.logger.Printf("Retry attempt %d/%d for notification %s after %v delay",
				attempt, config.MaxRetries, method, delay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := rt.middlewareTransport.SendNotification(ctx, method, params)

		if err == nil {
			if rt.middleware.circuitBreaker != nil {
				rt.middleware.circuitBreaker.recordSuccess()
			}
			return nil
		}

		lastErr = err

		if !rt.isRetryableError(err) {
			if rt.middleware.circuitBreaker != nil {
				rt.middleware.circuitBreaker.recordFailure()
			}
			return err
		}

		if rt.middleware.circuitBreaker != nil {
			rt.middleware.circuitBreaker.recordFailure()
		}
	}

	return mcperrors.NewError(
		mcperrors.CodeOperationFailed,
		fmt.Sprintf("Notification failed after %d attempts", maxAttempts),
		mcperrors.CategoryTransport,
		mcperrors.SeverityError,
	).WithContext(&mcperrors.Context{
		Method:    method,
		Component: "ReliabilityMiddleware",
		Operation: "retry_exhausted",
	}).WithDetail(fmt.Sprintf("Last error: %v", lastErr))
}

// isRetryableError determines if an error should trigger a retry
func (rt *reliabilityTransport) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific MCP error codes
	if mcpErr, ok := err.(mcperrors.MCPError); ok {
		switch mcpErr.Code() {
		case mcperrors.CodeConnectionFailed,
			mcperrors.CodeConnectionLost,
			mcperrors.CodeConnectionTimeout,
			mcperrors.CodeResourceUnavailable,
			mcperrors.CodeOperationTimeout:
			return true
		case mcperrors.CodeInvalidParams,
			mcperrors.CodeInvalidRequest,
			mcperrors.CodeOperationNotSupported,
			mcperrors.CodeOperationCancelled:
			return false
		}
	}

	// Check for protocol errors
	if protoErr, ok := err.(*protocol.ErrorObject); ok {
		// Client errors (4xx equivalent) are not retryable
		if protoErr.Code >= -32099 && protoErr.Code <= -32000 {
			return false
		}
		// Server errors might be retryable
		if protoErr.Code == int(protocol.InternalError) {
			return true
		}
	}

	// Default to retryable for unknown errors
	return true
}

// secureRandFloat64 generates a cryptographically secure random float64 in [0, 1)
func secureRandFloat64() (float64, error) {
	// Generate a random integer in [0, 2^53)
	max := big.NewInt(1 << 53)
	n, err := cryptorand.Int(cryptorand.Reader, max)
	if err != nil {
		return 0, err
	}
	// Convert to float64 in [0, 1)
	return float64(n.Int64()) / float64(1<<53), nil
}

// calculateBackoff calculates the delay before the next retry
func (rt *reliabilityTransport) calculateBackoff(attempt int, config ReliabilityConfig) time.Duration {
	// Exponential backoff
	backoff := float64(config.InitialRetryDelay) * math.Pow(config.RetryBackoffFactor, float64(attempt-1))

	// Cap at max delay
	if backoff > float64(config.MaxRetryDelay) {
		backoff = float64(config.MaxRetryDelay)
	}

	// Add jitter (if configured in the future)
	// For now, add 10% jitter
	if randFloat, err := secureRandFloat64(); err == nil {
		jitter := backoff * 0.1 * (randFloat*2 - 1) // ±10%
		backoff += jitter
	}

	return time.Duration(backoff)
}

// reliabilityCircuitBreaker implements a simple circuit breaker
type reliabilityCircuitBreaker struct {
	config    CircuitBreakerConfig
	state     circuitState
	failures  int
	successes int
	lastError time.Time
	mu        sync.RWMutex
}

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

func newReliabilityCircuitBreaker(config CircuitBreakerConfig) *reliabilityCircuitBreaker {
	return &reliabilityCircuitBreaker{
		config: config,
		state:  circuitClosed,
	}
}

func (cb *reliabilityCircuitBreaker) canMakeCall() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastError) > cb.config.Timeout {
			cb.state = circuitHalfOpen
			cb.successes = 0
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	}

	return false
}

func (cb *reliabilityCircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0

	if cb.state == circuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = circuitClosed
		}
	}
}

func (cb *reliabilityCircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastError = time.Now()
	cb.failures++

	if cb.state == circuitHalfOpen {
		cb.state = circuitOpen
		return
	}

	if cb.failures >= cb.config.FailureThreshold {
		cb.state = circuitOpen
	}
}
