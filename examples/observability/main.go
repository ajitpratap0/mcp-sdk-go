// Example demonstrating comprehensive observability with OpenTelemetry and Prometheus
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/examples/shared"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/observability"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Create observability middleware
	obsMiddleware, err := createObservabilityMiddleware()
	if err != nil {
		return fmt.Errorf("failed to create observability middleware: %w", err)
	}

	// Start server with observability
	serverErrChan := make(chan error, 1)
	go func() {
		serverErrChan <- runServer(ctx, obsMiddleware)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Run client with observability
	if err := runClient(ctx, obsMiddleware); err != nil {
		return fmt.Errorf("client error: %w", err)
	}

	// Wait for server or shutdown
	select {
	case err := <-serverErrChan:
		return err
	case <-ctx.Done():
		return nil
	}
}

func createObservabilityMiddleware() (transport.Middleware, error) {
	// Configure OpenTelemetry tracing
	tracingConfig := observability.TracingConfig{
		ServiceName:    "mcp-example",
		ServiceVersion: "1.0.0",
		Environment:    "development",

		// For local development with Jaeger
		ExporterType: observability.ExporterTypeOTLPGRPC,
		Endpoint:     "localhost:4317",
		Insecure:     true,

		// Sampling configuration
		SampleRate:   1.0, // Sample everything in development
		AlwaysSample: []string{"callTool", "readResource"},

		// Resource attributes
		ResourceAttributes: map[string]string{
			"deployment.region": "local",
			"deployment.type":   "example",
		},
	}

	// Configure Prometheus metrics
	metricsConfig := observability.MetricsConfig{
		ServiceName:    "mcp-example",
		ServiceVersion: "1.0.0",
		Environment:    "development",

		// Metrics endpoint
		MetricsPath: "/metrics",
		MetricsPort: 9090,

		// Metric configuration
		Namespace:        "mcp_example",
		HistogramBuckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
	}

	// Create enhanced observability middleware
	obsConfig := observability.ObservabilityConfig{
		EnableTracing: true,
		TracingConfig: tracingConfig,
		EnableMetrics: true,
		MetricsConfig: metricsConfig,
		EnableLogging: true,
		LogLevel:      "info",

		// Capture payloads for debugging (disable in production)
		CaptureRequestPayload:  true,
		CaptureResponsePayload: true,
		RecordPanics:           true,
	}

	return observability.NewEnhancedObservabilityMiddleware(obsConfig)
}

func runServer(ctx context.Context, obsMiddleware transport.Middleware) error {
	// Create transport config with observability
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"
	config.Features.EnableObservability = true
	config.Observability.EnableMetrics = true
	config.Observability.EnableLogging = true

	// Create transport with middleware
	t, err := transport.NewTransport(config)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// Wrap transport with observability middleware
	t = obsMiddleware.Wrap(t)

	// Create providers with instrumentation
	toolsProvider := &instrumentedToolsProvider{
		base: shared.CreateToolsProvider(),
	}
	resourcesProvider := &instrumentedResourcesProvider{
		base: shared.CreateResourcesProvider(),
	}
	promptsProvider := &instrumentedPromptsProvider{
		base: shared.CreatePromptsProvider(),
	}

	// Create server with providers
	s := server.New(t,
		server.WithName("Observability Example Server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(toolsProvider),
		server.WithResourcesProvider(resourcesProvider),
		server.WithPromptsProvider(promptsProvider),
	)

	log.Println("Server starting on http://localhost:8080/mcp")
	log.Println("Metrics available at http://localhost:9090/metrics")
	log.Println("Send traces to Jaeger at localhost:4317")

	// Start server
	return s.Start(ctx)
}

func runClient(ctx context.Context, obsMiddleware transport.Middleware) error {
	// Create transport config
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"

	// Create transport
	t, err := transport.NewTransport(config)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// Wrap with observability
	t = obsMiddleware.Wrap(t)

	// Create client
	c := client.New(t,
		client.WithName("Observability Example Client"),
		client.WithVersion("1.0.0"),
	)

	// Initialize
	if err := c.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Perform various operations to generate metrics and traces
	log.Println("\n=== Running Operations ===")

	// List tools
	log.Println("\n1. Listing tools...")
	tools, _, err := c.ListTools(ctx, "", &protocol.PaginationParams{Limit: 10})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	log.Printf("Found %d tools", len(tools))

	// Call a tool multiple times
	log.Println("\n2. Calling tools...")
	for i := 0; i < 5; i++ {
		result, err := c.CallTool(ctx, "calculator", map[string]interface{}{
			"operation": "add",
			"a":         i,
			"b":         i + 1,
		}, nil)
		if err != nil {
			log.Printf("Tool call %d failed: %v", i, err)
		} else {
			log.Printf("Tool call %d result: %v", i, result)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// List and read resources
	log.Println("\n3. Working with resources...")
	resources, _, _, err := c.ListResources(ctx, "", false, &protocol.PaginationParams{Limit: 10})
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}
	log.Printf("Found %d resources", len(resources))

	if len(resources) > 0 {
		content, err := c.ReadResource(ctx, resources[0].URI, nil, nil)
		if err != nil {
			log.Printf("Failed to read resource: %v", err)
		} else {
			log.Printf("Read resource %s (%d bytes)", resources[0].URI, len(content.Content))
		}
	}

	// Test batch operations
	log.Println("\n4. Testing batch operations...")
	batch := &protocol.JSONRPCBatchRequest{}
	for i := 0; i < 10; i++ {
		req, _ := protocol.NewRequest(fmt.Sprintf("%d", i), "listTools", nil)
		*batch = append(*batch, req)
	}

	batchResp, err := t.SendBatchRequest(ctx, batch)
	if err != nil {
		log.Printf("Batch request failed: %v", err)
	} else {
		log.Printf("Batch request completed with %d responses", batchResp.Len())
	}

	// Generate some errors for error metrics
	log.Println("\n5. Testing error scenarios...")
	_, err = c.CallTool(ctx, "nonexistent", nil, nil)
	if err != nil {
		log.Printf("Expected error: %v", err)
	}

	log.Println("\n=== Operations Complete ===")
	log.Println("\nCheck metrics at http://localhost:9090/metrics")
	log.Println("Check traces in Jaeger UI (typically http://localhost:16686)")

	// Keep running for a bit to allow metric collection
	time.Sleep(5 * time.Second)

	return nil
}

// Instrumented providers that add custom metrics and traces

type instrumentedToolsProvider struct {
	base *shared.CustomToolsProvider
}

func (p *instrumentedToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	// Custom instrumentation can be added here
	return p.base.ListTools(ctx, category, pagination)
}

func (p *instrumentedToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
	// Add custom metrics or traces
	start := time.Now()
	result, err := p.base.CallTool(ctx, name, input, contextData)
	duration := time.Since(start)

	// Record custom metric (would use the metrics provider in real implementation)
	log.Printf("Tool '%s' execution took %v", name, duration)

	return result, err
}

type instrumentedResourcesProvider struct {
	base *shared.CustomResourcesProvider
}

func (p *instrumentedResourcesProvider) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, int, string, bool, error) {
	return p.base.ListResources(ctx, uri, recursive, pagination)
}

func (p *instrumentedResourcesProvider) ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error) {
	return p.base.ReadResource(ctx, uri, templateParams, rangeOpt)
}

func (p *instrumentedResourcesProvider) SubscribeResource(ctx context.Context, uri string, recursive bool) (bool, error) {
	return p.base.SubscribeResource(ctx, uri, recursive)
}

type instrumentedPromptsProvider struct {
	base *server.BasePromptsProvider
}

func (p *instrumentedPromptsProvider) ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, int, string, bool, error) {
	return p.base.ListPrompts(ctx, tag, pagination)
}

func (p *instrumentedPromptsProvider) GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error) {
	return p.base.GetPrompt(ctx, id)
}
