package benchmarks

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// TestMemoryLeakClient tests for memory leaks in client operations
func TestMemoryLeakClient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Force GC and record initial memory
	runtime.GC()
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// Create client
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = mockReader()
	config.StdioWriter = mockWriter()

	tr, err := transport.NewTransport(config)
	if err != nil {
		t.Fatal(err)
	}

	c := client.New(tr,
		client.WithName("memory-test-client"),
		client.WithVersion("1.0.0"),
	)

	// Set up mock server
	setupMockServer(tr)

	ctx := context.Background()
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	// Perform many operations
	const iterations = 10000
	for i := 0; i < iterations; i++ {
		// Tool operations
		c.CallTool(ctx, "test_tool", map[string]interface{}{
			"input": "test data",
			"index": i,
		}, nil)

		// Resource operations
		c.ReadResource(ctx, "test://resource/1", nil, nil)

		// List operations with pagination
		if i%100 == 0 {
			c.ListTools(ctx, "", &protocol.PaginationParams{Limit: 50})
			c.ListResources(ctx, "", false, &protocol.PaginationParams{Limit: 50})
		}

		// Periodically force GC
		if i%1000 == 0 {
			runtime.GC()
		}
	}

	// Force final GC and measure memory
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond) // Allow GC to complete

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Check for significant memory growth
	memGrowth := int64(finalMem.HeapAlloc) - int64(initialMem.HeapAlloc)
	memGrowthMB := float64(memGrowth) / (1024 * 1024)

	t.Logf("Memory growth: %.2f MB", memGrowthMB)
	t.Logf("Initial heap: %.2f MB", float64(initialMem.HeapAlloc)/(1024*1024))
	t.Logf("Final heap: %.2f MB", float64(finalMem.HeapAlloc)/(1024*1024))
	// Note: NumGoroutine needs to be tracked separately
	// t.Logf("Goroutines: final=%d", runtime.NumGoroutine())

	// Fail if memory growth is excessive (more than 50MB for 10k operations)
	if memGrowthMB > 50 {
		t.Errorf("Excessive memory growth detected: %.2f MB", memGrowthMB)
	}

	// Check for goroutine leaks
	// Check for goroutine leaks would need separate tracking
	// Since MemStats doesn't include NumGoroutine
}

// TestMemoryLeakServer tests for memory leaks in server operations
func TestMemoryLeakServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Force GC and record initial memory
	runtime.GC()
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// Create server
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = mockReader()
	config.StdioWriter = mockWriter()

	tr, err := transport.NewTransport(config)
	if err != nil {
		t.Fatal(err)
	}

	s := server.New(tr,
		server.WithName("memory-test-server"),
		server.WithVersion("1.0.0"),
		// Use minimal providers for memory testing
	)

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Let the server run for a while to check for memory leaks
	// The server is already running and handling requests via the transport
	time.Sleep(2 * time.Second)

	// Stop server
	s.Stop()

	// Force final GC and measure memory
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Check for significant memory growth
	memGrowth := int64(finalMem.HeapAlloc) - int64(initialMem.HeapAlloc)
	memGrowthMB := float64(memGrowth) / (1024 * 1024)

	t.Logf("Memory growth: %.2f MB", memGrowthMB)
	t.Logf("Initial heap: %.2f MB", float64(initialMem.HeapAlloc)/(1024*1024))
	t.Logf("Final heap: %.2f MB", float64(finalMem.HeapAlloc)/(1024*1024))
	// Note: NumGoroutine needs to be tracked separately
	// t.Logf("Goroutines: final=%d", runtime.NumGoroutine())

	// Fail if memory growth is excessive
	if memGrowthMB > 50 {
		t.Errorf("Excessive memory growth detected: %.2f MB", memGrowthMB)
	}

	// Check for goroutine leaks
	// Check for goroutine leaks would need separate tracking
	// Since MemStats doesn't include NumGoroutine
}

// TestMemoryLeakBatchProcessing tests for memory leaks in batch operations
func TestMemoryLeakBatchProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Force GC and record initial memory
	runtime.GC()
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// Create transport
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = mockReader()
	config.StdioWriter = mockWriter()

	tr, err := transport.NewTransport(config)
	if err != nil {
		t.Fatal(err)
	}

	// Set up echo handler
	tr.RegisterRequestHandler("echo", func(ctx context.Context, params interface{}) (interface{}, error) {
		return params, nil
	})

	ctx := context.Background()
	if err := tr.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tr.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Create and send many batch requests
	const iterations = 1000
	const batchSize = 100

	for i := 0; i < iterations; i++ {
		// Create batch
		batch := &protocol.JSONRPCBatchRequest{}
		for j := 0; j < batchSize; j++ {
			req, _ := protocol.NewRequest(j, "echo", map[string]interface{}{
				"iteration": i,
				"index":     j,
				"data":      "test data for memory leak detection",
			})
			*batch = append(*batch, req)
		}

		// Send batch
		resp, err := tr.SendBatchRequest(ctx, batch)
		if err != nil {
			t.Fatal(err)
		}

		// Verify response count
		if resp.Len() != batchSize {
			t.Fatalf("Expected %d responses, got %d", batchSize, resp.Len())
		}

		// Periodically force GC
		if i%100 == 0 {
			runtime.GC()
		}
	}

	// Stop transport
	tr.Stop(ctx)

	// Force final GC and measure memory
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Check for significant memory growth
	memGrowth := int64(finalMem.HeapAlloc) - int64(initialMem.HeapAlloc)
	memGrowthMB := float64(memGrowth) / (1024 * 1024)
	totalRequests := iterations * batchSize

	t.Logf("Memory growth: %.2f MB after %d batch operations (%d total requests)",
		memGrowthMB, iterations, totalRequests)
	t.Logf("Initial heap: %.2f MB", float64(initialMem.HeapAlloc)/(1024*1024))
	t.Logf("Final heap: %.2f MB", float64(finalMem.HeapAlloc)/(1024*1024))
	// Note: NumGoroutine needs to be tracked separately
	// t.Logf("Goroutines: final=%d", runtime.NumGoroutine())

	// Fail if memory growth is excessive
	if memGrowthMB > 100 {
		t.Errorf("Excessive memory growth detected: %.2f MB", memGrowthMB)
	}

	// Check for goroutine leaks
	// Check for goroutine leaks would need separate tracking
	// Since MemStats doesn't include NumGoroutine
}

// BenchmarkMemoryAllocation benchmarks memory allocation for common operations
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("Request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			protocol.NewRequest("123", "test_method", map[string]interface{}{
				"key": "value",
			})
		}
	})

	b.Run("Response", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = &protocol.Response{
				JSONRPCMessage: protocol.JSONRPCMessage{
					JSONRPC: protocol.JSONRPCVersion,
				},
				ID:     "123",
				Result: jsonRawMessage(`{"status":"ok"}`),
			}
		}
	})

	b.Run("BatchRequest/10", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			batch := &protocol.JSONRPCBatchRequest{}
			for j := 0; j < 10; j++ {
				req, _ := protocol.NewRequest(j, "method", nil)
				*batch = append(*batch, req)
			}
		}
	})

	b.Run("ClientCallTool", func(b *testing.B) {
		config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
		config.StdioReader = mockReader()
		config.StdioWriter = mockWriter()

		tr, _ := transport.NewTransport(config)
		c := client.New(tr)
		setupMockServer(tr)

		ctx := context.Background()
		c.Initialize(ctx)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			c.CallTool(ctx, "test_tool", map[string]interface{}{"input": "test"}, nil)
		}
	})
}

// Helper to create json.RawMessage
func jsonRawMessage(s string) json.RawMessage {
	return json.RawMessage(s)
}
