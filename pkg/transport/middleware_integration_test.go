package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// Integration tests for middleware functionality with real transports

// TestMiddlewareWithTransportCreation tests that middleware is properly applied when creating transports
func TestMiddlewareWithTransportCreation(t *testing.T) {
	tests := []struct {
		name                    string
		transportType           TransportType
		enableReliability       bool
		enableObservability     bool
		expectReliabilityWrap   bool
		expectObservabilityWrap bool
	}{
		{
			name:                    "stdio with both middlewares",
			transportType:           TransportTypeStdio,
			enableReliability:       true,
			enableObservability:     true,
			expectReliabilityWrap:   true,
			expectObservabilityWrap: true,
		},
		{
			name:                    "http with observability only",
			transportType:           TransportTypeStreamableHTTP,
			enableReliability:       false,
			enableObservability:     true,
			expectReliabilityWrap:   false,
			expectObservabilityWrap: true,
		},
		{
			name:                    "no middleware",
			transportType:           TransportTypeStdio,
			enableReliability:       false,
			enableObservability:     false,
			expectReliabilityWrap:   false,
			expectObservabilityWrap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultTransportConfig(tt.transportType)
			if tt.transportType == TransportTypeStreamableHTTP {
				config.Endpoint = "http://localhost:8080"
			}

			config.Features.EnableReliability = tt.enableReliability
			config.Features.EnableObservability = tt.enableObservability
			config.Observability.EnableLogging = tt.enableObservability
			config.Observability.EnableMetrics = tt.enableObservability

			transport, err := NewTransport(config)
			if err != nil {
				t.Fatalf("Failed to create transport: %v", err)
			}

			// Check if observability middleware is applied by checking the type
			// When no middleware is applied, transport should be the base type
			// When observability is applied, it should be the outermost wrapper
			switch v := transport.(type) {
			case *observabilityTransport:
				if !tt.expectObservabilityWrap {
					t.Errorf("Unexpected observability wrap on transport")
				}
			case *StdioTransport, *StreamableHTTPTransport:
				// Base transport types - no middleware applied
				if tt.expectObservabilityWrap {
					t.Errorf("Expected observability wrap, but got base transport type %T", v)
				}
			default:
				// Could be reliability middleware or other wrapper
				// For now, we can't easily inspect nested middleware
				if tt.expectObservabilityWrap && !tt.enableReliability {
					t.Errorf("Expected observability wrap, but got %T", v)
				}
			}
		})
	}
}

// TestReliabilityMiddlewareRetries tests retry behavior with a real HTTP server
func TestReliabilityMiddlewareRetries(t *testing.T) {
	// Counter for requests
	var requestCount int32

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body once
		var req protocol.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error"}}`)
			return
		}

		// Handle initialize request - always succeed
		if req.Method == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			idStr := fmt.Sprintf("%v", req.ID)
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":"%s","result":{"serverInfo":{"name":"test","version":"1.0"},"capabilities":{}}}`, idStr)
			return
		}

		count := atomic.AddInt32(&requestCount, 1)

		// Fail first two requests to test retry
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"temporary error"}}`)
			return
		}

		// Success on third attempt
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		idStr := fmt.Sprintf("%v", req.ID)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":"%s","result":{"status":"ok"}}`, idStr)
	}))
	defer server.Close()

	// Create config with retry enabled
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = true
	config.Reliability.MaxRetries = 3
	config.Reliability.InitialRetryDelay = 10 * time.Millisecond

	// Create transport with middleware
	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// Initialize only - we don't need Start for this test
	ctx := context.Background()
	if err := transport.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	// Note: We're not calling Start() because it tries to open a listener connection
	// which our test server doesn't support. For testing SendRequest retry behavior,
	// Initialize is sufficient.

	// Send request - should succeed after retries
	result, err := transport.SendRequest(ctx, "test.method", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("SendRequest failed after retries: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Verify retry happened
	finalCount := atomic.LoadInt32(&requestCount)
	if finalCount != 3 {
		t.Errorf("Expected 3 requests (2 retries + 1 success), got %d", finalCount)
	}
}

// TestMiddlewareErrorPropagation tests that errors are properly propagated through middleware
func TestMiddlewareErrorPropagation(t *testing.T) {
	// Create a test server that allows initialize but returns errors for other requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle initialize request - always succeed
		if r.URL.Path == "/" && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			var req protocol.Request
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Method == "initialize" {
				w.Header().Set("Content-Type", "application/json")
				// Properly format the ID - it needs to be a string
				idStr := fmt.Sprintf("%v", req.ID)
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":"%s","result":{"serverInfo":{"name":"test","version":"1.0"},"capabilities":{}}}`, idStr)
				return
			}
		}

		// For other requests, return error
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"}}`)
	}))
	defer server.Close()

	// Create config without retry to test error propagation
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = true
	config.Reliability.MaxRetries = 0 // No retries

	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// Initialize only - we don't need Start for this test
	ctx := context.Background()
	if err := transport.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	// Note: We're not calling Start() because it tries to open a listener connection
	// which our test server doesn't support. For testing error propagation,
	// Initialize is sufficient.

	// Send request - should get error
	_, err = transport.SendRequest(ctx, "test.method", nil)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check that error message is preserved
	if !strings.Contains(err.Error(), "Invalid Request") {
		t.Errorf("Expected error to contain 'Invalid Request', got: %v", err)
	}
}

// TestCircuitBreakerIntegration tests circuit breaker behavior
func TestCircuitBreakerIntegration(t *testing.T) {
	// Create a test server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Server Error"}}`)
	}))
	defer server.Close()

	// Create config with circuit breaker
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = true
	config.Reliability.MaxRetries = 0 // No retries
	config.Reliability.CircuitBreaker = CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		Timeout:          100 * time.Millisecond,
	}

	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	ctx := context.Background()
	transport.Initialize(ctx)
	transport.Start(ctx)
	defer transport.Stop(ctx)

	// Send requests until circuit opens
	for i := 0; i < 3; i++ {
		_, err := transport.SendRequest(ctx, "test.method", nil)
		if err == nil {
			t.Error("Expected error")
		}
	}

	// Next request should fail with circuit open
	_, err = transport.SendRequest(ctx, "test.method", nil)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker open") {
		t.Errorf("Expected circuit breaker open error, got: %v", err)
	}

	// Wait for circuit to reset
	time.Sleep(150 * time.Millisecond)

	// Should allow one more attempt
	_, err = transport.SendRequest(ctx, "test.method", nil)
	if err == nil {
		t.Error("Expected error (server still failing)")
	}
	// But it shouldn't be circuit breaker error anymore
	if strings.Contains(err.Error(), "circuit breaker open") {
		t.Error("Circuit breaker should be in half-open state")
	}
}

// TestConcurrentRequestsWithMiddleware tests middleware under concurrent load
func TestConcurrentRequestsWithMiddleware(t *testing.T) {
	var requestCount int32
	var concurrentCount int32
	var maxConcurrent int32

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&concurrentCount, 1)
		defer atomic.AddInt32(&concurrentCount, -1)

		// Track max concurrent
		for {
			max := atomic.LoadInt32(&maxConcurrent)
			if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
				break
			}
		}

		count := atomic.AddInt32(&requestCount, 1)

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":"test-%d","result":{"count":%d}}`, count, count)
	}))
	defer server.Close()

	// Create config with middleware
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = true
	config.Features.EnableObservability = true
	config.Observability.EnableMetrics = true

	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	ctx := context.Background()
	transport.Initialize(ctx)
	transport.Start(ctx)
	defer transport.Stop(ctx)

	// Run concurrent requests
	const numGoroutines = 10
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				_, err := transport.SendRequest(ctx, fmt.Sprintf("test.method%d", id),
					map[string]int{"goroutine": id, "request": j})
				if err != nil {
					t.Errorf("Request failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := atomic.LoadInt32(&requestCount)
	expectedRequests := int32(numGoroutines * requestsPerGoroutine)
	if totalRequests != expectedRequests {
		t.Errorf("Expected %d requests, got %d", expectedRequests, totalRequests)
	}

	t.Logf("Completed %d concurrent requests in %v", totalRequests, duration)
	t.Logf("Max concurrent requests: %d", atomic.LoadInt32(&maxConcurrent))
}

// TestObservabilityLogging tests that logging middleware works correctly
func TestObservabilityLogging(t *testing.T) {
	// Create a simple echo server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":"test-1","result":{"echo":"response"}}`)
	}))
	defer server.Close()

	// Create config with logging enabled
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableObservability = true
	config.Observability.EnableLogging = true
	config.Observability.LogLevel = "debug"

	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	ctx := context.Background()
	transport.Initialize(ctx)
	transport.Start(ctx)
	defer transport.Stop(ctx)

	// Send a request - logging will happen automatically
	result, err := transport.SendRequest(ctx, "test.log", map[string]string{"test": "data"})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Note: In a real test, we might capture log output to verify
	// For now, we just verify the request succeeded with logging enabled
}

// TestMiddlewareOrderPreservation tests that middleware order is preserved
func TestMiddlewareOrderPreservation(t *testing.T) {
	// This test verifies that when both reliability and observability are enabled,
	// they are applied in the correct order (observability wraps reliability)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return success
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":"test-1","result":{"status":"ok"}}`)
	}))
	defer server.Close()

	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = server.URL
	config.Features.EnableReliability = true
	config.Features.EnableObservability = true

	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// The outermost layer should be observability
	if _, ok := transport.(*observabilityTransport); !ok {
		t.Error("Expected outermost middleware to be observability")
	}

	// We can't easily check the inner layers without exposing internals,
	// but the behavior tests above verify the middleware works correctly
}
