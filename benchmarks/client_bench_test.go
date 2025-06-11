package benchmarks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// BenchmarkClientOperations benchmarks various client operations
func BenchmarkClientOperations(b *testing.B) {
	b.Run("CallTool", func(b *testing.B) {
		benchmarkClientCallTool(b)
	})

	b.Run("ReadResource", func(b *testing.B) {
		benchmarkClientReadResource(b)
	})

	b.Run("ListTools", func(b *testing.B) {
		benchmarkClientListTools(b)
	})

	b.Run("ListResources", func(b *testing.B) {
		benchmarkClientListResources(b)
	})

	b.Run("ConcurrentToolCalls/10", func(b *testing.B) {
		benchmarkConcurrentToolCalls(b, 10)
	})

	b.Run("ConcurrentToolCalls/100", func(b *testing.B) {
		benchmarkConcurrentToolCalls(b, 100)
	})

	b.Run("WithCallbacks", func(b *testing.B) {
		benchmarkClientWithCallbacks(b)
	})
}

// benchmarkClientCallTool benchmarks tool calling performance
func benchmarkClientCallTool(b *testing.B) {
	ctx := context.Background()
	c, cleanup := createTestClient(b)
	defer cleanup()

	// Initialize client
	err := c.Initialize(ctx)
	if err != nil {
		b.Fatal(err)
	}

	// Prepare tool call arguments
	args := map[string]interface{}{
		"input": "test data",
		"options": map[string]interface{}{
			"format":   "json",
			"validate": true,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := c.CallTool(ctx, "test_tool", args, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkClientReadResource benchmarks resource reading performance
func benchmarkClientReadResource(b *testing.B) {
	ctx := context.Background()
	c, cleanup := createTestClient(b)
	defer cleanup()

	// Initialize client
	err := c.Initialize(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := c.ReadResource(ctx, "test://resource/1", nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkClientListTools benchmarks tool listing performance
func benchmarkClientListTools(b *testing.B) {
	ctx := context.Background()
	c, cleanup := createTestClient(b)
	defer cleanup()

	// Initialize client
	err := c.Initialize(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := c.ListTools(ctx, "", nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkClientListResources benchmarks resource listing performance
func benchmarkClientListResources(b *testing.B) {
	ctx := context.Background()
	c, cleanup := createTestClient(b)
	defer cleanup()

	// Initialize client
	err := c.Initialize(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, _, err := c.ListResources(ctx, "", false, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkConcurrentToolCalls benchmarks concurrent tool calling
func benchmarkConcurrentToolCalls(b *testing.B, concurrency int) {
	ctx := context.Background()
	c, cleanup := createTestClient(b)
	defer cleanup()

	// Initialize client
	err := c.Initialize(ctx)
	if err != nil {
		b.Fatal(err)
	}

	args := map[string]interface{}{
		"input": "concurrent test",
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := c.CallTool(ctx, "test_tool", args, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// benchmarkClientWithCallbacks benchmarks client with callbacks enabled
func benchmarkClientWithCallbacks(b *testing.B) {
	ctx := context.Background()

	// Create client with callbacks
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = mockReader()
	config.StdioWriter = mockWriter()

	t, err := transport.NewTransport(config)
	if err != nil {
		b.Fatal(err)
	}

	callbackCount := 0
	c := client.New(t,
		client.WithName("benchmark-client"),
		client.WithVersion("1.0.0"),
	)

	// Set callbacks after creating the client
	c.SetSamplingCallback(func(event protocol.SamplingEvent) {
		callbackCount++
	})

	c.SetResourceChangedCallback(func(id string, data interface{}) {
		callbackCount++
	})

	// Set up mock server responses
	setupMockServer(t)

	// Initialize client
	err = c.Initialize(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate server-initiated sampling
		t.HandleRequest(ctx, &protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{
				JSONRPC: protocol.JSONRPCVersion,
			},
			ID:     i,
			Method: "sampling/createMessage",
			Params: json.RawMessage(`{"messages":[{"role":"user","content":{"type":"text","text":"test"}}],"systemPrompt":"test"}`),
		})
	}

	b.Logf("Callbacks triggered: %d", callbackCount)
}

// BenchmarkPaginatedOperations benchmarks paginated list operations
func BenchmarkPaginatedOperations(b *testing.B) {
	b.Run("ListTools/PageSize10", func(b *testing.B) {
		benchmarkPaginatedList(b, "tools/list", 10)
	})

	b.Run("ListTools/PageSize100", func(b *testing.B) {
		benchmarkPaginatedList(b, "tools/list", 100)
	})

	b.Run("ListResources/PageSize10", func(b *testing.B) {
		benchmarkPaginatedList(b, "resources/list", 10)
	})

	b.Run("ListResources/PageSize100", func(b *testing.B) {
		benchmarkPaginatedList(b, "resources/list", 100)
	})
}

func benchmarkPaginatedList(b *testing.B, method string, pageSize int) {
	ctx := context.Background()
	c, cleanup := createTestClient(b)
	defer cleanup()

	// Initialize client
	err := c.Initialize(ctx)
	if err != nil {
		b.Fatal(err)
	}

	pagination := &protocol.PaginationParams{
		Limit: pageSize,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var cursor string
		hasMore := true

		// Fetch all pages
		for hasMore {
			if cursor != "" {
				pagination.Cursor = cursor
			}

			if method == "tools/list" {
				_, result, err := c.ListTools(ctx, "", pagination)
				if err != nil {
					b.Fatal(err)
				}
				hasMore = result.HasMore
				cursor = result.NextCursor
			} else {
				_, _, result, err := c.ListResources(ctx, "", false, pagination)
				if err != nil {
					b.Fatal(err)
				}
				hasMore = result.HasMore
				cursor = result.NextCursor
			}
		}
	}
}

// Helper functions

func createTestClient(b *testing.B) (*client.ClientConfig, func()) {
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = mockReader()
	config.StdioWriter = mockWriter()

	t, err := transport.NewTransport(config)
	if err != nil {
		b.Fatal(err)
	}

	c := client.New(t,
		client.WithName("benchmark-client"),
		client.WithVersion("1.0.0"),
	)

	// Set up mock server
	setupMockServer(t)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		t.Stop(ctx)
	}

	return c, cleanup
}

func setupMockServer(t transport.Transport) {
	// Initialize handler
	t.RegisterRequestHandler("initialize", func(ctx context.Context, params interface{}) (interface{}, error) {
		return &protocol.InitializeResult{
			ProtocolVersion: "1.0",
			ServerInfo: &protocol.ServerInfo{
				Name:    "benchmark-server",
				Version: "1.0.0",
			},
			Capabilities: map[string]bool{
				"tools":     true,
				"resources": true,
				"sampling":  true,
			},
		}, nil
	})

	// Tools handlers
	t.RegisterRequestHandler("tools/list", func(ctx context.Context, params interface{}) (interface{}, error) {
		tools := make([]protocol.Tool, 50) // Return 50 tools
		for i := 0; i < 50; i++ {
			tools[i] = protocol.Tool{
				Name:        "test_tool",
				Description: "Test tool for benchmarking",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			}
		}
		return &protocol.ListToolsResult{
			Tools: tools,
		}, nil
	})

	t.RegisterRequestHandler("tools/call", func(ctx context.Context, params interface{}) (interface{}, error) {
		return &protocol.CallToolResult{
			Result: json.RawMessage(`{"status":"success","data":"test"}`),
		}, nil
	})

	// Resources handlers
	t.RegisterRequestHandler("resources/list", func(ctx context.Context, params interface{}) (interface{}, error) {
		resources := make([]protocol.Resource, 50) // Return 50 resources
		for i := 0; i < 50; i++ {
			resources[i] = protocol.Resource{
				URI:         "test://resource/1",
				Name:        "Test Resource",
				Description: "Test resource for benchmarking",
				Type:        "application/json",
			}
		}
		return &protocol.ListResourcesResult{
			Resources: resources,
		}, nil
	})

	t.RegisterRequestHandler("resources/read", func(ctx context.Context, params interface{}) (interface{}, error) {
		return &protocol.ReadResourceResult{
			Contents: protocol.ResourceContents{
				URI:     "test://resource/1",
				Type:    "application/json",
				Content: json.RawMessage(`{"test":"data"}`),
			},
		}, nil
	})

	// Start transport in background
	go func() {
		ctx := context.Background()
		t.Initialize(ctx)
		t.Start(ctx)
	}()

	// Give transport time to start
	time.Sleep(10 * time.Millisecond)
}

func mockReader() *mockReadWriter {
	return &mockReadWriter{
		data: make(chan []byte, 1000),
	}
}

func mockWriter() *mockReadWriter {
	return &mockReadWriter{
		data: make(chan []byte, 1000),
	}
}

type mockReadWriter struct {
	data chan []byte
}

func (m *mockReadWriter) Read(p []byte) (n int, err error) {
	select {
	case data := <-m.data:
		copy(p, data)
		return len(data), nil
	case <-time.After(100 * time.Millisecond):
		return 0, nil
	}
}

func (m *mockReadWriter) Write(p []byte) (n int, err error) {
	data := make([]byte, len(p))
	copy(data, p)
	select {
	case m.data <- data:
		return len(p), nil
	default:
		return len(p), nil
	}
}

// strPtr returns a pointer to a string - unused but kept for future use
// func strPtr(s string) *string {
// 	return &s
// }
