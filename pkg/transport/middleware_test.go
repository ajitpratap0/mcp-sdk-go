package transport

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockTransport is a test transport that allows us to control behavior
type mockTransport struct {
	BaseTransport
	sendRequestFunc      func(ctx context.Context, method string, params interface{}) (interface{}, error)
	sendNotificationFunc func(ctx context.Context, method string, params interface{}) error
	initializeFunc       func(ctx context.Context) error
	callCount            int
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		BaseTransport: *NewBaseTransport(),
	}
}

func (m *mockTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	m.callCount++
	if m.sendRequestFunc != nil {
		return m.sendRequestFunc(ctx, method, params)
	}
	return nil, errors.New("mock error")
}

func (m *mockTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.callCount++
	if m.sendNotificationFunc != nil {
		return m.sendNotificationFunc(ctx, method, params)
	}
	return errors.New("mock error")
}

func (m *mockTransport) Initialize(ctx context.Context) error {
	m.callCount++
	if m.initializeFunc != nil {
		return m.initializeFunc(ctx)
	}
	return nil
}

func (m *mockTransport) Start(ctx context.Context) error { return nil }
func (m *mockTransport) Stop(ctx context.Context) error  { return nil }

func TestReliabilityMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		config        ReliabilityConfig
		mockBehavior  func(*mockTransport)
		expectedCalls int
		expectError   bool
	}{
		{
			name: "successful request on first try",
			config: ReliabilityConfig{
				MaxRetries:         3,
				InitialRetryDelay:  10 * time.Millisecond,
				MaxRetryDelay:      100 * time.Millisecond,
				RetryBackoffFactor: 2.0,
			},
			mockBehavior: func(mock *mockTransport) {
				mock.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
					return "success", nil
				}
			},
			expectedCalls: 1,
			expectError:   false,
		},
		{
			name: "retry until success",
			config: ReliabilityConfig{
				MaxRetries:         3,
				InitialRetryDelay:  10 * time.Millisecond,
				MaxRetryDelay:      100 * time.Millisecond,
				RetryBackoffFactor: 2.0,
			},
			mockBehavior: func(mock *mockTransport) {
				callCount := 0
				mock.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
					callCount++
					if callCount <= 2 {
						return nil, errors.New("temporary error")
					}
					return "success", nil
				}
			},
			expectedCalls: 3,
			expectError:   false,
		},
		{
			name: "retry exhaustion",
			config: ReliabilityConfig{
				MaxRetries:         2,
				InitialRetryDelay:  10 * time.Millisecond,
				MaxRetryDelay:      100 * time.Millisecond,
				RetryBackoffFactor: 2.0,
			},
			mockBehavior: func(mock *mockTransport) {
				mock.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
					return nil, errors.New("persistent error")
				}
			},
			expectedCalls: 3, // Initial attempt + 2 retries
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockTransport()
			tt.mockBehavior(mock)

			// Create reliability middleware
			middleware := NewReliabilityMiddleware(tt.config)
			wrappedTransport := middleware.Wrap(mock)

			// Test SendRequest
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := wrappedTransport.SendRequest(ctx, "test/method", nil)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if mock.callCount != tt.expectedCalls {
				t.Errorf("Expected %d calls but got %d", tt.expectedCalls, mock.callCount)
			}
		})
	}
}

func TestObservabilityMiddleware(t *testing.T) {
	config := ObservabilityConfig{
		EnableMetrics: true,
		EnableLogging: false, // Disable logging for cleaner test output
		LogLevel:      "info",
		MetricsPrefix: "test_transport",
	}

	mock := newMockTransport()
	mock.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		if method == "test/success" {
			return "result", nil
		}
		return nil, errors.New("test error")
	}

	// Create observability middleware
	middleware := NewObservabilityMiddleware(config)
	wrappedTransport := middleware.Wrap(mock)

	ctx := context.Background()

	// Test successful request
	_, err := wrappedTransport.SendRequest(ctx, "test/success", nil)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Test failed request
	_, err = wrappedTransport.SendRequest(ctx, "test/failure", nil)
	if err == nil {
		t.Errorf("Expected error but got none")
	}

	// Check if the transport supports metrics
	if obsTransport, ok := wrappedTransport.(interface {
		GetMetrics() *TransportMetricsSnapshot
	}); ok {
		metrics := obsTransport.GetMetrics()
		if metrics == nil {
			t.Errorf("Expected metrics but got nil")
			return
		}

		// Verify success metrics
		if successMetrics, exists := metrics.Requests["test/success"]; exists {
			if successMetrics.Total != 1 {
				t.Errorf("Expected 1 total request for test/success, got %d", successMetrics.Total)
			}
			if successMetrics.Success != 1 {
				t.Errorf("Expected 1 successful request for test/success, got %d", successMetrics.Success)
			}
			if successMetrics.Errors != 0 {
				t.Errorf("Expected 0 errors for test/success, got %d", successMetrics.Errors)
			}
		} else {
			t.Errorf("Expected metrics for test/success but not found")
		}

		// Verify failure metrics
		if failureMetrics, exists := metrics.Requests["test/failure"]; exists {
			if failureMetrics.Total != 1 {
				t.Errorf("Expected 1 total request for test/failure, got %d", failureMetrics.Total)
			}
			if failureMetrics.Success != 0 {
				t.Errorf("Expected 0 successful requests for test/failure, got %d", failureMetrics.Success)
			}
			if failureMetrics.Errors != 1 {
				t.Errorf("Expected 1 error for test/failure, got %d", failureMetrics.Errors)
			}
		} else {
			t.Errorf("Expected metrics for test/failure but not found")
		}
	} else {
		t.Errorf("Transport does not support metrics")
	}
}

func TestMiddlewareChaining(t *testing.T) {
	mock := newMockTransport()

	// Create both middleware
	reliabilityConfig := ReliabilityConfig{
		MaxRetries:         2,
		InitialRetryDelay:  10 * time.Millisecond,
		MaxRetryDelay:      100 * time.Millisecond,
		RetryBackoffFactor: 2.0,
	}
	observabilityConfig := ObservabilityConfig{
		EnableMetrics: true,
		EnableLogging: false,
		LogLevel:      "info",
		MetricsPrefix: "chained_transport",
	}

	reliabilityMiddleware := NewReliabilityMiddleware(reliabilityConfig)
	observabilityMiddleware := NewObservabilityMiddleware(observabilityConfig)

	// Chain middleware
	wrappedTransport := ChainMiddleware(
		reliabilityMiddleware,
		observabilityMiddleware,
	).Wrap(mock)

	// Configure mock to fail initially, then succeed
	callCount := 0
	mock.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		callCount++
		if callCount <= 1 {
			return nil, errors.New("temporary error")
		}
		return "success", nil
	}

	ctx := context.Background()

	// This should succeed after one retry
	result, err := wrappedTransport.SendRequest(ctx, "test/method", nil)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success' but got: %v", result)
	}

	// Verify that both middleware worked
	// Reliability: should have made 2 calls (initial + 1 retry)
	if callCount != 2 {
		t.Errorf("Expected 2 calls (initial + retry), got %d", callCount)
	}

	// Observability: should have collected metrics
	if obsTransport, ok := wrappedTransport.(interface {
		GetMetrics() *TransportMetricsSnapshot
	}); ok {
		metrics := obsTransport.GetMetrics()
		if metrics == nil {
			t.Errorf("Expected metrics but got nil")
			return
		}

		if methodMetrics, exists := metrics.Requests["test/method"]; exists {
			if methodMetrics.Total != 1 {
				t.Errorf("Expected 1 total request, got %d", methodMetrics.Total)
			}
			if methodMetrics.Success != 1 {
				t.Errorf("Expected 1 successful request, got %d", methodMetrics.Success)
			}
		} else {
			t.Errorf("Expected metrics for test/method but not found")
		}
	}
}

func TestNewTransportWithMiddleware(t *testing.T) {
	config := TransportConfig{
		Type:     TransportTypeStreamableHTTP,
		Endpoint: "http://localhost:8080/mcp",
		Features: FeatureConfig{
			EnableReliability:   false, // Disable to get direct access to observability
			EnableObservability: true,
		},
		Observability: ObservabilityConfig{
			EnableMetrics: true,
			EnableLogging: false, // Disable for test
			LogLevel:      "info",
			MetricsPrefix: "integration_test",
		},
	}

	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// Verify the transport has middleware applied
	// We can check this by verifying it has observability features
	if obsTransport, ok := transport.(interface {
		GetMetrics() *TransportMetricsSnapshot
	}); ok {
		metrics := obsTransport.GetMetrics()
		if metrics == nil {
			t.Errorf("Expected metrics to be available but got nil")
		}
	} else {
		t.Errorf("Transport does not support metrics, middleware may not be applied")
	}

	// Test with both middleware enabled
	configBoth := config
	configBoth.Features.EnableReliability = true
	configBoth.Reliability = ReliabilityConfig{
		MaxRetries:         2,
		InitialRetryDelay:  10 * time.Millisecond,
		MaxRetryDelay:      100 * time.Millisecond,
		RetryBackoffFactor: 2.0,
	}

	transportBoth, err := NewTransport(configBoth)
	if err != nil {
		t.Fatalf("Failed to create transport with both middleware: %v", err)
	}

	// With both middleware enabled, the outer transport is reliability,
	// but we should still be able to make requests
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This will fail but should demonstrate middleware is working
	_, err = transportBoth.SendRequest(ctx, "test/method", nil)
	// We expect an error since no server is running, but the middleware should be active
	if err == nil {
		t.Errorf("Expected error due to no server, but got success")
	}
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	config := ReliabilityConfig{
		MaxRetries:         1,
		InitialRetryDelay:  10 * time.Millisecond,
		MaxRetryDelay:      100 * time.Millisecond,
		RetryBackoffFactor: 2.0,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2,
			SuccessThreshold: 1,
			Timeout:          100 * time.Millisecond,
		},
	}

	mock := newMockTransport()
	mock.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		return nil, errors.New("persistent error")
	}

	middleware := NewReliabilityMiddleware(config)
	wrappedTransport := middleware.Wrap(mock)

	ctx := context.Background()

	// Make enough requests to trip the circuit breaker
	for i := 0; i < 3; i++ {
		_, err := wrappedTransport.SendRequest(ctx, "test/method", nil)
		if err == nil {
			t.Errorf("Expected error on request %d", i+1)
		}
	}

	// Circuit breaker should now be open, so the next request should fail immediately
	initialCallCount := mock.callCount
	_, err := wrappedTransport.SendRequest(ctx, "test/method", nil)
	if err == nil {
		t.Errorf("Expected circuit breaker to reject request")
	}

	// Verify that the underlying transport was not called (circuit breaker prevented it)
	if mock.callCount != initialCallCount {
		t.Errorf("Circuit breaker should have prevented call to underlying transport")
	}
}
