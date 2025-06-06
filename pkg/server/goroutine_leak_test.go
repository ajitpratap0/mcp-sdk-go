package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/utils"
)

// MockTransportForLeakTest is a simple mock transport for testing
type MockTransportForLeakTest struct {
	*transport.BaseTransport
	initialized bool
	started     bool
	stopped     bool
}

func NewMockTransportForLeakTest() *MockTransportForLeakTest {
	return &MockTransportForLeakTest{
		BaseTransport: transport.NewBaseTransport(),
	}
}

func (m *MockTransportForLeakTest) Initialize(ctx context.Context) error {
	m.initialized = true
	return nil
}

func (m *MockTransportForLeakTest) Start(ctx context.Context) error {
	m.started = true
	<-ctx.Done()
	return ctx.Err()
}

func (m *MockTransportForLeakTest) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

func (m *MockTransportForLeakTest) Send(data []byte) error {
	return nil
}

func (m *MockTransportForLeakTest) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Simple mock implementation
	return map[string]string{"status": "ok"}, nil
}

func (m *MockTransportForLeakTest) SendNotification(ctx context.Context, method string, params interface{}) error {
	// Simple mock implementation
	return nil
}

func (m *MockTransportForLeakTest) SetErrorHandler(handler transport.ErrorHandler) {
	// Simple mock implementation
}

// TestServerGoroutineLeak tests for goroutine leaks in Server
func TestServerGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(2). // Allow some growth for test infrastructure
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create transport and server
	mockTransport := NewMockTransportForLeakTest()
	server := New(mockTransport,
		WithName("test-server"),
		WithVersion("1.0.0"),
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	done := make(chan error)
	go func() {
		done <- server.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop server
	cancel()

	err := server.Stop()
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Wait for Start to finish
	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after Stop")
	}

	// Check for leaks
	detector.Check()
}

// TestHTTPHandlerGoroutineLeak tests for leaks in HTTPHandler
func TestHTTPHandlerGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(5). // HTTP server may create a few goroutines
		SetStabilizeDelay(500 * time.Millisecond)
	detector.Start()

	// Create handler and mock transport
	handler := NewHTTPHandler()
	mockTransport := NewMockTransportForLeakTest()
	handler.SetTransport(mockTransport)

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Send some requests
	client := &http.Client{Timeout: 1 * time.Second}

	for i := 0; i < 5; i++ {
		// Just send empty POST requests to test handler
		resp, err := client.Post(server.URL, "application/json", nil)
		if err == nil {
			resp.Body.Close()
		}
	}

	// Check for leaks
	detector.Check()
}

// TestStreamableHTTPHandlerGoroutineLeak tests for leaks in StreamableHTTPHandler
func TestStreamableHTTPHandlerGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetAllowedGrowth(5). // HTTP server and SSE may create goroutines
		SetStabilizeDelay(500 * time.Millisecond)
	detector.Start()

	// Create handler and mock transport
	handler := NewStreamableHTTPHandler()
	mockTransport := NewMockTransportForLeakTest()
	handler.SetTransport(mockTransport)

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test SSE connection
	client := &http.Client{Timeout: 1 * time.Second}

	req, _ := http.NewRequest("GET", server.URL, nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err == nil {
		// Read a bit then close
		buf := make([]byte, 1024)
		resp.Body.Read(buf)
		resp.Body.Close()
	}

	// Also test POST requests
	for i := 0; i < 3; i++ {
		resp, err := client.Post(server.URL, "application/json", nil)
		if err == nil {
			resp.Body.Close()
		}
	}

	// Check for leaks
	detector.Check()
}

// TestBaseProvidersGoroutineLeak tests for leaks in base providers
func TestBaseProvidersGoroutineLeak(t *testing.T) {
	detector := utils.NewGoroutineLeakDetector(t).
		SetStabilizeDelay(300 * time.Millisecond)
	detector.Start()

	// Create providers
	toolsProvider := NewBaseToolsProvider()
	resourcesProvider := NewBaseResourcesProvider()
	promptsProvider := NewBasePromptsProvider()
	rootsProvider := NewBaseRootsProvider()

	ctx := context.Background()

	// Simulate concurrent operations
	done := make(chan struct{}, 40) // Buffer for all operations

	// Tools operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() {
				done <- struct{}{}
			}()

			tool := protocol.Tool{
				Name: string(rune('A' + id)),
			}
			toolsProvider.RegisterTool(tool)
			toolsProvider.ListTools(ctx, "", nil)
		}(i)
	}

	// Resources operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() {
				done <- struct{}{}
			}()

			resource := protocol.Resource{
				URI: string(rune('A' + id)),
			}
			resourcesProvider.RegisterResource(resource)
			resourcesProvider.ListResources(ctx, "", false, nil)
		}(i)
	}

	// Prompts operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() {
				done <- struct{}{}
			}()

			prompt := protocol.Prompt{
				ID: string(rune('A' + id)),
			}
			promptsProvider.RegisterPrompt(prompt)
			promptsProvider.ListPrompts(ctx, "", nil)
			promptsProvider.GetPrompt(ctx, prompt.ID)
		}(i)
	}

	// Roots operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() {
				done <- struct{}{}
			}()

			root := protocol.Root{
				ID: string(rune('A' + id)),
			}
			rootsProvider.RegisterRoot(root)
			rootsProvider.ListRoots(ctx, "", nil)
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < 40; i++ {
		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Operation %d did not complete", i)
		}
	}

	// Check for leaks
	detector.Check()
}
