package benchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// BenchmarkServerOperations benchmarks various server operations
func BenchmarkServerOperations(b *testing.B) {
	b.Run("HandleRequest", func(b *testing.B) {
		benchmarkServerHandleRequest(b)
	})

	b.Run("HandleBatchRequest/10", func(b *testing.B) {
		benchmarkServerHandleBatchRequest(b, 10)
	})

	b.Run("HandleBatchRequest/100", func(b *testing.B) {
		benchmarkServerHandleBatchRequest(b, 100)
	})

	b.Run("WithProviders", func(b *testing.B) {
		benchmarkServerWithProviders(b)
	})

	b.Run("ConcurrentRequests/10", func(b *testing.B) {
		benchmarkServerConcurrentRequests(b, 10)
	})

	b.Run("ConcurrentRequests/100", func(b *testing.B) {
		benchmarkServerConcurrentRequests(b, 100)
	})

	b.Run("ResourceSubscriptions", func(b *testing.B) {
		benchmarkServerResourceSubscriptions(b)
	})
}

// benchmarkServerHandleRequest benchmarks single request handling
func benchmarkServerHandleRequest(b *testing.B) {
	ctx := context.Background()
	s, t, cleanup := createTestServer(b)
	defer cleanup()

	// Start server
	if err := s.Start(ctx); err != nil {
		b.Fatal(err)
	}

	// Create test request
	req := &protocol.Request{
		JSONRPCMessage: protocol.JSONRPCMessage{
			JSONRPC: protocol.JSONRPCVersion,
		},
		ID:     "123",
		Method: "tools/call",
		Params: json.RawMessage(`{"name":"test_tool","arguments":{"input":"test"}}`),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := t.HandleRequest(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkServerHandleBatchRequest benchmarks batch request handling
func benchmarkServerHandleBatchRequest(b *testing.B, batchSize int) {
	ctx := context.Background()
	s, t, cleanup := createTestServer(b)
	defer cleanup()

	// Start server
	if err := s.Start(ctx); err != nil {
		b.Fatal(err)
	}

	// Create batch request
	batch := &protocol.JSONRPCBatchRequest{}
	for i := 0; i < batchSize; i++ {
		req, _ := protocol.NewRequest(i, "tools/list", nil)
		*batch = append(*batch, req)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := t.HandleBatchRequest(ctx, batch)
		if err != nil {
			b.Fatal(err)
		}
		if resp.Len() != batchSize {
			b.Fatalf("Expected %d responses, got %d", batchSize, resp.Len())
		}
	}
}

// benchmarkServerWithProviders benchmarks server with all providers
func benchmarkServerWithProviders(b *testing.B) {
	ctx := context.Background()

	// Create server with all providers
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = mockReader()
	config.StdioWriter = mockWriter()

	t, err := transport.NewTransport(config)
	if err != nil {
		b.Fatal(err)
	}

	s := server.New(t,
		server.WithName("benchmark-server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(&benchmarkToolsProvider{}),
		server.WithResourcesProvider(&benchmarkResourcesProvider{}),
		server.WithPromptsProvider(&benchmarkPromptsProvider{}),
	)

	// Start server
	if err := s.Start(ctx); err != nil {
		b.Fatal(err)
	}
	defer s.Stop()

	// Benchmark different operations
	b.Run("ListTools", func(b *testing.B) {
		req := &protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             "1",
			Method:         "tools/list",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			t.HandleRequest(ctx, req)
		}
	})

	b.Run("CallTool", func(b *testing.B) {
		req := &protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             "1",
			Method:         "tools/call",
			Params:         json.RawMessage(`{"name":"test_tool","arguments":{}}`),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			t.HandleRequest(ctx, req)
		}
	})

	b.Run("ListResources", func(b *testing.B) {
		req := &protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             "1",
			Method:         "resources/list",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			t.HandleRequest(ctx, req)
		}
	})
}

// benchmarkServerConcurrentRequests benchmarks concurrent request handling
func benchmarkServerConcurrentRequests(b *testing.B, concurrency int) {
	ctx := context.Background()
	s, t, cleanup := createTestServer(b)
	defer cleanup()

	// Start server
	if err := s.Start(ctx); err != nil {
		b.Fatal(err)
	}

	// Create test requests
	requests := make([]*protocol.Request, concurrency)
	for i := 0; i < concurrency; i++ {
		requests[i] = &protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             i,
			Method:         "tools/list",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := requests[i%concurrency]
			_, err := t.HandleRequest(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// benchmarkServerResourceSubscriptions benchmarks resource subscription handling
func benchmarkServerResourceSubscriptions(b *testing.B) {
	ctx := context.Background()
	s, t, cleanup := createTestServer(b)
	defer cleanup()

	// Start server
	if err := s.Start(ctx); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Subscribe
		subReq := &protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             i,
			Method:         "resources/subscribe",
			Params:         json.RawMessage(`{"uri":"test://resource/1"}`),
		}
		_, err := t.HandleRequest(ctx, subReq)
		if err != nil {
			b.Fatal(err)
		}

		// Unsubscribe
		unsubReq := &protocol.Request{
			JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
			ID:             i + 1000000,
			Method:         "resources/unsubscribe",
			Params:         json.RawMessage(`{"uri":"test://resource/1"}`),
		}
		_, err = t.HandleRequest(ctx, unsubReq)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Provider implementations for benchmarking

type benchmarkToolsProvider struct{}

func (p *benchmarkToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	tools := make([]protocol.Tool, 100)
	for i := 0; i < 100; i++ {
		tools[i] = protocol.Tool{
			Name:        "test_tool",
			Description: "Benchmark test tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}
	}

	if pagination != nil && pagination.Limit > 0 && pagination.Limit < 100 {
		return tools[:pagination.Limit], 100, "next", true, nil
	}

	return tools, 100, "", false, nil
}

func (p *benchmarkToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
	// Simulate some processing
	time.Sleep(100 * time.Microsecond)

	return &protocol.CallToolResult{
		Result: json.RawMessage(`{"status":"success","processed":true}`),
	}, nil
}

type benchmarkResourcesProvider struct {
	subscriptions map[string]bool
	mu            sync.RWMutex
}

func (p *benchmarkResourcesProvider) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, int, string, bool, error) {
	resources := make([]protocol.Resource, 100)
	for i := 0; i < 100; i++ {
		resources[i] = protocol.Resource{
			URI:         fmt.Sprintf("test://resource/%d", i),
			Name:        "Test Resource",
			Description: "Benchmark test resource",
			Type:        "application/json",
		}
	}

	if pagination != nil && pagination.Limit > 0 && pagination.Limit < 100 {
		return resources[:pagination.Limit], nil, 100, "next", true, nil
	}

	return resources, nil, 100, "", false, nil
}

func (p *benchmarkResourcesProvider) ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error) {
	content := fmt.Sprintf(`{"test":"data","timestamp":%d}`, time.Now().Unix())
	return &protocol.ResourceContents{
		URI:     uri,
		Type:    "application/json",
		Content: json.RawMessage(content),
	}, nil
}

func (p *benchmarkResourcesProvider) SubscribeResource(ctx context.Context, uri string, recursive bool) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.subscriptions == nil {
		p.subscriptions = make(map[string]bool)
	}
	p.subscriptions[uri] = true
	return true, nil
}

func (p *benchmarkResourcesProvider) UnsubscribeResource(ctx context.Context, uri string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.subscriptions, uri)
	return nil
}

type benchmarkPromptsProvider struct{}

func (p *benchmarkPromptsProvider) ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, int, string, bool, error) {
	prompts := make([]protocol.Prompt, 10)
	for i := 0; i < 10; i++ {
		prompts[i] = protocol.Prompt{
			ID:          fmt.Sprintf("test_prompt_%d", i),
			Name:        fmt.Sprintf("Test Prompt %d", i),
			Description: "Benchmark test prompt",
		}
	}
	return prompts, 10, "", false, nil
}

func (p *benchmarkPromptsProvider) GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error) {
	return &protocol.Prompt{
		ID:          id,
		Name:        "Test Prompt",
		Description: "Benchmark test prompt",
		Messages: []protocol.PromptMessage{
			{
				Role:    "user",
				Content: "Test prompt message",
			},
		},
	}, nil
}

// Note: Sampling provider is not yet implemented in the MCP spec
// type benchmarkSamplingProvider struct{}
//
// func (p *benchmarkSamplingProvider) CreateMessage(ctx context.Context, req *protocol.CreateMessageRequest) (*protocol.CreateMessageResult, error) {
// 	return &protocol.CreateMessageResult{
// 		Content: protocol.TextContent{
// 			Type: "text",
// 			Text: "Sampled response for benchmark",
// 		},
// 		Model: "benchmark-model",
// 	}, nil
// }

// Helper functions

func createTestServer(b *testing.B) (*server.Server, transport.Transport, func()) {
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = mockReader()
	config.StdioWriter = mockWriter()

	t, err := transport.NewTransport(config)
	if err != nil {
		b.Fatal(err)
	}

	s := server.New(t,
		server.WithName("benchmark-server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(&benchmarkToolsProvider{}),
		server.WithResourcesProvider(&benchmarkResourcesProvider{}),
	)

	cleanup := func() {
		s.Stop()
	}

	return s, t, cleanup
}
