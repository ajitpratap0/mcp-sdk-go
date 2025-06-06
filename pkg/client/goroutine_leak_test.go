package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/utils"
)

// MockTransportForClientLeakTest is a simple mock transport for testing
type MockTransportForClientLeakTest struct {
	*transport.BaseTransport
	initialized bool
	started     bool
	stopped     bool
}

func NewMockTransportForClientLeakTest() *MockTransportForClientLeakTest {
	return &MockTransportForClientLeakTest{
		BaseTransport: transport.NewBaseTransport(),
	}
}

func (m *MockTransportForClientLeakTest) Initialize(ctx context.Context) error {
	m.initialized = true
	return nil
}

func (m *MockTransportForClientLeakTest) Start(ctx context.Context) error {
	m.started = true
	<-ctx.Done()
	return ctx.Err()
}

func (m *MockTransportForClientLeakTest) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

func (m *MockTransportForClientLeakTest) Send(data []byte) error {
	return nil
}

func (m *MockTransportForClientLeakTest) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Simulate server response for initialization
	if method == protocol.MethodInitialize {
		return &protocol.InitializeResult{
			ProtocolVersion: "1.0",
			ServerInfo: &protocol.ServerInfo{
				Name:    "mock-server",
				Version: "1.0.0",
			},
		}, nil
	}
	return map[string]string{"status": "ok"}, nil
}

func (m *MockTransportForClientLeakTest) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *MockTransportForClientLeakTest) SetErrorHandler(handler transport.ErrorHandler) {
	// Simple mock implementation
}

// TestClientGoroutineLeak tests for goroutine leaks in Client
func TestClientGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(2). // Allow some growth for test infrastructure
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create transport and client
	mockTransport := NewMockTransportForClientLeakTest()
	client := New(mockTransport, WithName("test-client"), WithVersion("1.0.0"))

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize client
	err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Make some requests
	for i := 0; i < 5; i++ {
		_, _ = client.CallTool(ctx, "test-tool", map[string]string{"id": string(rune('0' + i))}, nil)
	}

	// Close client
	cancel()
	_ = client.Close()

	// Check for leaks
	detector.Check()
}

// TestStdioClientGoroutineLeak tests for goroutine leaks in StdioClient
func TestStdioClientGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(3). // Allow some growth for stdio pipes
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create stdio client with default transport
	client := NewStdioClient(WithName("test-stdio-client"))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to initialize (will fail, but shouldn't leak)
	_ = client.Initialize(ctx)

	// Close client
	_ = client.Close()

	// Check for leaks
	detector.Check()
}

// TestStreamingToolCallGoroutineLeak tests for goroutine leaks in streaming tool calls
func TestStreamingToolCallGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(2).
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create transport and client
	mockTransport := NewMockTransportForClientLeakTest()
	client := New(mockTransport, WithName("test-streaming-client"), WithVersion("1.0.0"))

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize client
	err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Make some streaming tool calls
	for i := 0; i < 3; i++ {
		streamCtx, streamCancel := context.WithTimeout(ctx, 50*time.Millisecond)

		// Use CallToolStreaming
		_, _ = client.CallToolStreaming(streamCtx, "test-tool",
			map[string]string{"test": "data"}, nil,
			func(update json.RawMessage) {
				// Handle streaming updates
			})

		// Wait briefly then cancel
		time.Sleep(10 * time.Millisecond)
		streamCancel()
	}

	// Close client
	cancel()
	_ = client.Close()

	// Check for leaks
	detector.Check()
}

// TestPaginationGoroutineLeak tests for goroutine leaks in pagination
func TestPaginationGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create transport and client
	mockTransport := NewMockTransportForClientLeakTest()
	client := New(mockTransport, WithName("test-client"), WithVersion("1.0.0"))

	ctx := context.Background()

	// Initialize client
	err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Use the ListResources with pagination
	paginationParams := &protocol.PaginationParams{
		Limit: 10,
	}

	// Make a few paginated calls
	for i := 0; i < 3; i++ {
		_, _, _, _ = client.ListResources(ctx, "", false, paginationParams)
	}

	// Also test ListAllResources which does pagination internally
	_, _, _ = client.ListAllResources(ctx, "", false)

	// Close client
	_ = client.Close()

	// Check for leaks
	detector.Check()
}
