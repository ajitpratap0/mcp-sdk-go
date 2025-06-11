package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// This example demonstrates how to use middleware with MCP transports

func main() {
	// Example 1: Creating a client with reliability and observability middleware
	fmt.Println("=== Example 1: Client with Middleware ===")
	demoClientWithMiddleware()

	fmt.Println("\n=== Example 2: Server with Middleware ===")
	demoServerWithMiddleware()

	fmt.Println("\n=== Example 3: Custom Middleware ===")
	demoCustomMiddleware()
}

func demoClientWithMiddleware() {
	// Create transport config with middleware features enabled
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)

	// Enable reliability features
	config.Features.EnableReliability = true
	config.Reliability.MaxRetries = 3
	config.Reliability.InitialRetryDelay = 100 * time.Millisecond
	config.Reliability.CircuitBreaker.Enabled = true
	config.Reliability.CircuitBreaker.FailureThreshold = 5

	// Enable observability features
	config.Features.EnableObservability = true
	config.Observability.EnableLogging = true
	config.Observability.LogLevel = "debug"
	config.Observability.EnableMetrics = true

	// Create transport (middleware is automatically applied based on config)
	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create client with the middleware-enabled transport
	c := client.New(t,
		client.WithName("middleware-demo-client"),
		client.WithVersion("1.0.0"),
	)
	_ = c // avoid unused variable error

	fmt.Println("Created client with reliability and observability middleware")
	fmt.Printf("- Retry enabled: %v (max retries: %d)\n",
		config.Features.EnableReliability,
		config.Reliability.MaxRetries)
	fmt.Printf("- Circuit breaker enabled: %v (threshold: %d)\n",
		config.Reliability.CircuitBreaker.Enabled,
		config.Reliability.CircuitBreaker.FailureThreshold)
	fmt.Printf("- Logging enabled: %v (level: %s)\n",
		config.Observability.EnableLogging,
		config.Observability.LogLevel)
	fmt.Printf("- Metrics enabled: %v\n", config.Observability.EnableMetrics)

	// The client will now automatically:
	// - Retry failed requests with exponential backoff
	// - Open circuit breaker after repeated failures
	// - Log all requests and responses
	// - Collect metrics on request performance
}

func demoServerWithMiddleware() {
	// Create transport config
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"

	// Enable middleware features
	config.Features.EnableReliability = true
	config.Features.EnableObservability = true
	config.Observability.EnableLogging = true
	config.Observability.EnableMetrics = true

	// Create transport
	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create server with middleware-enabled transport
	s := server.New(t,
		server.WithName("middleware-demo-server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(&demoToolsProvider{}),
	)
	_ = s // avoid unused variable error

	fmt.Println("Created server with middleware")
	fmt.Println("The server will automatically:")
	fmt.Println("- Log all incoming requests and outgoing responses")
	fmt.Println("- Collect metrics on request handling")
	fmt.Println("- Retry failed downstream requests (if acting as a client)")
}

func demoCustomMiddleware() {
	// Create a custom middleware that adds request timing
	timingMiddleware := &requestTimingMiddleware{}

	// Create another custom middleware that adds request ID
	requestIDMiddleware := &requestIDMiddleware{}

	// Create base transport
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	baseTransport, _ := transport.NewTransport(config)

	// Apply middleware chain
	// Middleware is applied in order: requestID -> timing -> base
	t := transport.ChainMiddleware(
		requestIDMiddleware,
		timingMiddleware,
	).Wrap(baseTransport)

	// Create client with custom middleware
	c := client.New(t)

	fmt.Println("Created client with custom middleware chain:")
	fmt.Println("1. Request ID middleware - adds unique ID to each request")
	fmt.Println("2. Timing middleware - measures request duration")

	// Demonstrate middleware in action
	_ = context.Background()
	_ = c // avoid unused variable error

	// The middleware will:
	// 1. Add a request ID to the context
	// 2. Start timing the request
	// 3. Execute the request
	// 4. Log the duration
	// 5. Return the result

	fmt.Println("\nMiddleware execution order:")
	fmt.Println("→ Request ID middleware (pre)")
	fmt.Println("  → Timing middleware (pre)")
	fmt.Println("    → Transport.SendRequest()")
	fmt.Println("  ← Timing middleware (post)")
	fmt.Println("← Request ID middleware (post)")
}

// Demo tool provider
type demoToolsProvider struct{}

func (p *demoToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	tools := []protocol.Tool{
		{
			Name:        "demo_tool",
			Description: "A demonstration tool",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"message": {
						"type": "string",
						"description": "Message to echo"
					}
				}
			}`),
		},
	}
	return tools, len(tools), "", false, nil
}

func (p *demoToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
	// Tool implementation
	if name != "demo_tool" {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	return &protocol.CallToolResult{
		Result: json.RawMessage(`{"echo": "` + string(input) + `"}`),
	}, nil
}

// Custom middleware examples

// requestTimingMiddleware measures request duration
type requestTimingMiddleware struct {
	transport transport.Transport
}

func (m *requestTimingMiddleware) Wrap(t transport.Transport) transport.Transport {
	m.transport = t
	return m
}

func (m *requestTimingMiddleware) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	return m.transport.HandleBatchRequest(ctx, batch)
}

func (m *requestTimingMiddleware) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	start := time.Now()
	result, err := m.transport.SendBatchRequest(ctx, batch)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("[Timing] Batch request failed after %v: %v\n", duration, err)
	} else {
		fmt.Printf("[Timing] Batch request (%d items) completed in %v\n", batch.Len(), duration)
	}
	return result, err
}

func (m *requestTimingMiddleware) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	start := time.Now()
	result, err := m.transport.SendRequest(ctx, method, params)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("[Timing] Request %s failed after %v: %v\n", method, duration, err)
	} else {
		fmt.Printf("[Timing] Request %s completed in %v\n", method, duration)
	}

	return result, err
}

// Implement other Transport methods by delegating to wrapped transport
func (m *requestTimingMiddleware) Initialize(ctx context.Context) error {
	return m.transport.Initialize(ctx)
}

func (m *requestTimingMiddleware) Start(ctx context.Context) error {
	return m.transport.Start(ctx)
}

func (m *requestTimingMiddleware) Stop(ctx context.Context) error {
	return m.transport.Stop(ctx)
}

func (m *requestTimingMiddleware) SendNotification(ctx context.Context, method string, params interface{}) error {
	return m.transport.SendNotification(ctx, method, params)
}

func (m *requestTimingMiddleware) RegisterRequestHandler(method string, handler transport.RequestHandler) {
	m.transport.RegisterRequestHandler(method, handler)
}

func (m *requestTimingMiddleware) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	m.transport.RegisterNotificationHandler(method, handler)
}

func (m *requestTimingMiddleware) RegisterProgressHandler(id interface{}, handler transport.ProgressHandler) {
	m.transport.RegisterProgressHandler(id, handler)
}

func (m *requestTimingMiddleware) UnregisterProgressHandler(id interface{}) {
	m.transport.UnregisterProgressHandler(id)
}

func (m *requestTimingMiddleware) HandleResponse(response *protocol.Response) {
	m.transport.HandleResponse(response)
}

func (m *requestTimingMiddleware) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	return m.transport.HandleRequest(ctx, request)
}

func (m *requestTimingMiddleware) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	return m.transport.HandleNotification(ctx, notification)
}

func (m *requestTimingMiddleware) GenerateID() string {
	return m.transport.GenerateID()
}

func (m *requestTimingMiddleware) GetRequestIDPrefix() string {
	return m.transport.GetRequestIDPrefix()
}

func (m *requestTimingMiddleware) GetNextID() int64 {
	return m.transport.GetNextID()
}

func (m *requestTimingMiddleware) Cleanup() {
	m.transport.Cleanup()
}

// requestIDMiddleware adds a unique request ID
type requestIDMiddleware struct {
	transport transport.Transport
	counter   int64
}

func (m *requestIDMiddleware) Wrap(t transport.Transport) transport.Transport {
	m.transport = t
	return m
}

func (m *requestIDMiddleware) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	requestID := fmt.Sprintf("req-%d-%d", time.Now().Unix(), m.counter)
	m.counter++

	fmt.Printf("[RequestID] Assigning ID %s to request %s\n", requestID, method)

	// Could add request ID to context here
	// ctx = context.WithValue(ctx, "requestID", requestID)

	return m.transport.SendRequest(ctx, method, params)
}

func (m *requestIDMiddleware) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	return m.transport.HandleBatchRequest(ctx, batch)
}

func (m *requestIDMiddleware) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	batchID := fmt.Sprintf("batch-%d-%d", time.Now().Unix(), m.counter)
	m.counter++

	fmt.Printf("[RequestID] Assigning ID %s to batch request (%d items)\n", batchID, batch.Len())

	// Could add batch ID to context here
	// ctx = context.WithValue(ctx, "batchID", batchID)

	return m.transport.SendBatchRequest(ctx, batch)
}

func (m *requestIDMiddleware) Initialize(ctx context.Context) error {
	return m.transport.Initialize(ctx)
}

func (m *requestIDMiddleware) Start(ctx context.Context) error {
	return m.transport.Start(ctx)
}

func (m *requestIDMiddleware) Stop(ctx context.Context) error {
	return m.transport.Stop(ctx)
}

func (m *requestIDMiddleware) SendNotification(ctx context.Context, method string, params interface{}) error {
	return m.transport.SendNotification(ctx, method, params)
}

func (m *requestIDMiddleware) RegisterRequestHandler(method string, handler transport.RequestHandler) {
	m.transport.RegisterRequestHandler(method, handler)
}

func (m *requestIDMiddleware) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	m.transport.RegisterNotificationHandler(method, handler)
}

func (m *requestIDMiddleware) RegisterProgressHandler(id interface{}, handler transport.ProgressHandler) {
	m.transport.RegisterProgressHandler(id, handler)
}

func (m *requestIDMiddleware) UnregisterProgressHandler(id interface{}) {
	m.transport.UnregisterProgressHandler(id)
}

func (m *requestIDMiddleware) HandleResponse(response *protocol.Response) {
	m.transport.HandleResponse(response)
}

func (m *requestIDMiddleware) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	return m.transport.HandleRequest(ctx, request)
}

func (m *requestIDMiddleware) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	return m.transport.HandleNotification(ctx, notification)
}

func (m *requestIDMiddleware) GenerateID() string {
	return m.transport.GenerateID()
}

func (m *requestIDMiddleware) GetRequestIDPrefix() string {
	return m.transport.GetRequestIDPrefix()
}

func (m *requestIDMiddleware) GetNextID() int64 {
	return m.transport.GetNextID()
}

func (m *requestIDMiddleware) Cleanup() {
	m.transport.Cleanup()
}
