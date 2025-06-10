package transport

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/utils"
)

// TestStdioTransportGoroutineLeak tests for goroutine leaks in StdioTransport
func TestStdioTransportGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(2). // Allow some growth for test infrastructure
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create and start transport
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	config := DefaultTransportConfig(TransportTypeStdio)
	config.StdioReader = inReader
	config.StdioWriter = outWriter
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)
	defer inWriter.Close()
	defer outReader.Close()

	// Register a simple handler
	transport.RegisterRequestHandler(protocol.MethodInitialize, func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"status": "ok"}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize transport
	err := transport.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}

	// Start transport in background
	done := make(chan error)
	go func() {
		done <- transport.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop transport
	cancel()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	err = transport.Stop(stopCtx)
	if err != nil {
		t.Errorf("Failed to stop transport: %v", err)
	}

	// Wait for Start to finish
	select {
	case <-done:
		// Good, Start returned
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after Stop")
	}

	// Check for leaks
	detector.Check()
}

// TestHTTPTransportGoroutineLeak tests for goroutine leaks in HTTPTransport
func TestHTTPTransportGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(5). // HTTP client may create a few goroutines
		SetStabilizeDelay(500 * time.Millisecond)
	detector.Start()

	// Create transport using StreamableHTTPTransport
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:9999/test"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)        // Non-existent server

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Try to initialize (will fail, but shouldn't leak)
	_ = transport.Initialize(ctx)

	// Try to send a request (will fail, but shouldn't leak)
	_, _ = transport.SendRequest(ctx, protocol.MethodInitialize, map[string]string{"test": "data"})

	// Stop transport
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer stopCancel()
	_ = transport.Stop(stopCtx)

	// Check for leaks
	detector.Check()
}

// TestStreamableHTTPTransportGoroutineLeak tests for goroutine leaks in StreamableHTTPTransport
func TestStreamableHTTPTransportGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(5). // HTTP client and logger may create goroutines
		SetStabilizeDelay(500 * time.Millisecond)
	detector.Start()

	// Create transport
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:9999/test"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)        // Non-existent server

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Try to initialize (will fail, but shouldn't leak)
	_ = transport.Initialize(ctx)

	// Try to send a request (will fail, but shouldn't leak)
	_, _ = transport.SendRequest(ctx, protocol.MethodInitialize, map[string]string{"test": "data"})

	// Stop transport
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer stopCancel()
	_ = transport.Stop(stopCtx)

	// Check for leaks
	detector.Check()
}

// TestBaseTransportGoroutineLeak tests for goroutine leaks in BaseTransport
func TestBaseTransportGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create base transport
	transport := NewBaseTransport()

	// Register handlers
	transport.RegisterRequestHandler(protocol.MethodInitialize, func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]string{"status": "ok"}, nil
	})

	transport.RegisterNotificationHandler(protocol.MethodInitialized, func(ctx context.Context, params interface{}) error {
		return nil
	})

	ctx := context.Background()

	// Simulate handling responses concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			resp := &protocol.Response{
				ID:     string(rune('0' + id)),
				Result: []byte(`{"status":"ok"}`),
			}
			transport.HandleResponse(resp)
		}(i)
	}

	// Simulate handling notifications
	for i := 0; i < 5; i++ {
		go func(id int) {
			transport.HandleNotification(ctx, &protocol.Notification{
				Method: protocol.MethodInitialized,
				Params: []byte(`{}`),
			})
		}(i)
	}

	// Let operations process
	time.Sleep(200 * time.Millisecond)

	// Check for leaks
	detector.Check()
}

// TestTransportStartStopCycle tests that transports don't leak during multiple start/stop cycles
func TestTransportStartStopCycle(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(3).
		SetStabilizeDelay(500 * time.Millisecond)
	detector.Start()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	config := DefaultTransportConfig(TransportTypeStdio)
	config.StdioReader = inReader
	config.StdioWriter = outWriter
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	transport, _ := NewTransport(config)
	defer inWriter.Close()
	defer outReader.Close()

	// Multiple start/stop cycles
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		err := transport.Initialize(ctx)
		if err != nil {
			t.Fatalf("Cycle %d: Failed to initialize: %v", i, err)
		}

		// Start in background
		done := make(chan error)
		go func() {
			done <- transport.Start(ctx)
		}()

		// Run briefly
		time.Sleep(50 * time.Millisecond)

		// Stop
		cancel()

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
		err = transport.Stop(stopCtx)
		stopCancel()

		if err != nil {
			t.Errorf("Cycle %d: Failed to stop: %v", i, err)
		}

		// Wait for Start to finish
		select {
		case <-done:
			// Good
		case <-time.After(1 * time.Second):
			t.Errorf("Cycle %d: Start did not return after Stop", i)
		}
	}

	// Final check for leaks
	detector.Check()
}
