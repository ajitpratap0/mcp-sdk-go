package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/model-context-protocol/go-mcp/pkg/protocol"
	"github.com/model-context-protocol/go-mcp/pkg/server"
	"github.com/model-context-protocol/go-mcp/pkg/transport"
)

const (
	serverAddr = "localhost:8081" // Using port 8081 to avoid conflicts
)

func main() {
	// Create an HTTP handler for the streamable HTTP server
	handler := http.NewServeMux()

	// Register the MCP endpoint handler
	mcpHandler := server.NewStreamableHTTPHandler()
	
	// Set allowed origins for security (prevent DNS rebinding attacks)
	// In a production environment, you would restrict this to specific trusted origins
	mcpHandler.SetAllowedOrigins([]string{"http://localhost", "http://127.0.0.1"})
	
	handler.Handle("/mcp", mcpHandler)

	// Create a streamable HTTP transport with proper endpoint URL and longer timeout
	endpoint := fmt.Sprintf("http://%s/mcp", serverAddr)
	streamableTransport := transport.NewStreamableHTTPTransport(endpoint, transport.WithRequestTimeout(2*time.Minute))

	// Set the transport for the handler
	mcpHandler.SetTransport(streamableTransport)

	// Create and register tool provider
	basicToolsProvider := server.NewBaseToolsProvider()
	customToolsProvider := registerExampleTools(basicToolsProvider)

	// Create and register resources provider
	basicResourcesProvider := server.NewBaseResourcesProvider()
	customResourcesProvider := registerExampleResources(basicResourcesProvider)

	// Create prompts provider
	promptsProvider := server.NewBasePromptsProvider()
	registerExamplePrompts(promptsProvider)

	// Create server with the streamable transport
	s := server.New(streamableTransport,
		server.WithName("StreamableHTTPExample"),
		server.WithVersion("1.0.0"),
		server.WithDescription("A streamable HTTP example MCP server"),
		server.WithHomepage("https://github.com/model-context-protocol/go-mcp"),
		server.WithToolsProvider(customToolsProvider),
		server.WithResourcesProvider(customResourcesProvider),
		server.WithPromptsProvider(promptsProvider),
		server.WithCapability(protocol.CapabilityResourceSubscriptions, true),
		server.WithCapability(protocol.CapabilityLogging, true),
		// Additional capabilities for streamable HTTP
		server.WithCapability(protocol.CapabilitySampling, true),
	)

	// Create the HTTP server
	httpServer := &http.Server{
		Addr:    serverAddr,
		Handler: handler,
	}

	// Create a context that is canceled on interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
		
		// Gracefully shut down the HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	// Start the server (non-blocking)
	go func() {
		log.Printf("Starting MCP server on http://%s/mcp...", serverAddr)
		log.Printf("Server supports session management and origin validation")
		log.Printf("Allowed origins: http://localhost, http://127.0.0.1")
		if err := s.Start(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Start HTTP server (blocking)
	log.Printf("HTTP server listening on http://%s", serverAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

	log.Println("Server stopped")
}

func registerExampleTools(provider *server.BaseToolsProvider) *CustomToolsProvider {
	// Example hello tool
	helloTool := protocol.Tool{
		Name:        "hello",
		Description: "Says hello to the specified name",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "The name to greet"
				}
			},
			"required": ["name"]
		}`),
		OutputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"greeting": {
					"type": "string",
					"description": "The greeting message"
				}
			}
		}`),
		Examples: []protocol.ToolExample{
			{
				Name:        "Basic usage",
				Description: "Greet a person",
				Input:       json.RawMessage(`{"name": "World"}`),
				Output:      json.RawMessage(`{"greeting": "Hello, World!"}`),
			},
		},
		Categories: []string{"demo", "basic"},
	}

	// Example streaming tool that demonstrates the streaming capability
	streamingTool := protocol.Tool{
		Name:        "countToTen",
		Description: "Counts from 1 to 10 with a delay, demonstrating streaming responses",
		OutputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"count": {
					"type": "number",
					"description": "The current count"
				},
				"done": {
					"type": "boolean",
					"description": "Whether counting is complete"
				}
			}
		}`),
		Categories: []string{"demo", "streaming"},
	}

	provider.RegisterTool(helloTool)
	provider.RegisterTool(streamingTool)

	// Create a custom provider that wraps the base provider and implements the tools
	customProvider := &CustomToolsProvider{BaseToolsProvider: *provider}
	return customProvider
}

// CustomToolsProvider extends BaseToolsProvider to implement the tools
type CustomToolsProvider struct {
	server.BaseToolsProvider
}

// CallTool implements the actual tool execution
func (p *CustomToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
	switch name {
	case "hello":
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}

		greeting := fmt.Sprintf("Hello, %s!", params.Name)
		result, _ := json.Marshal(map[string]string{"greeting": greeting})
		return &protocol.CallToolResult{Result: result}, nil

	case "countToTen":
		// Instead of returning a streaming result directly, we'll use the available fields
		// in CallToolResult to indicate this is a streaming operation and handle it
		// using the OperationID
		
		// Generate a simple unique operation ID using timestamp
		opID := fmt.Sprintf("stream-%d", time.Now().UnixNano())
		
		// Start the streaming process in a goroutine
		go func() {
			for i := 1; i <= 10; i++ {
				// In a real implementation, you would send these updates back to the client
				// through your transport mechanism.
				
				// Create proper SSE-compatible update that can be streamed
				update := map[string]interface{}{
					"operationId": opID,
					"count": i,        // The current count value
					"progress": i * 10, // Progress percentage
					"message": fmt.Sprintf("Processing step %d of 10", i),
					"partial": i < 10,  // Partial result flag
					"done": i == 10,    // Completion flag
				}
				
				// Log the update for demonstration purposes
				updateJSON, _ := json.Marshal(update)
				fmt.Printf("Stream update: %s\n", string(updateJSON))
				
				time.Sleep(500 * time.Millisecond) // Delay to demonstrate streaming
			}
		}()
		
		// Return initial response with operationId to track the streaming operation
		streamingData, _ := json.Marshal(map[string]interface{}{
			"streaming": true,
			"message": "Streaming operation started",
		})
		
		return &protocol.CallToolResult{
			Result: streamingData,
			Partial: true, // Mark as partial to indicate this is part of streaming operation
			OperationID: opID, // Use OperationID to track this specific streaming operation
		}, nil

	default:
		// Call the parent implementation
		return p.BaseToolsProvider.CallTool(ctx, name, input, contextData)
	}
}

func registerExampleResources(provider *server.BaseResourcesProvider) *CustomResourcesProvider {
	// Example text resource
	textResource := protocol.Resource{
		URI:         "example://text/greeting.txt",
		Name:        "Greeting",
		Description: "A simple greeting message",
		Type:        "text/plain",
		Size:        13,
		ModTime:     time.Now(),
	}

	// Example JSON resource
	jsonResource := protocol.Resource{
		URI:         "example://json/config.json",
		Name:        "Configuration",
		Description: "Example configuration data",
		Type:        "application/json",
		Size:        42,
		ModTime:     time.Now(),
	}

	// Example resource template
	template := protocol.ResourceTemplate{
		URI:         "example://template/greeting",
		Name:        "Custom Greeting",
		Description: "Generate a custom greeting message",
		Type:        "text/plain",
		Parameters: []protocol.ResourceParameter{
			{
				Name:        "name",
				Description: "The name to greet",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "formal",
				Description: "Whether to use a formal greeting",
				Required:    false,
				Type:        "boolean",
			},
		},
		Examples: []protocol.ResourceExample{
			{
				Name: "Casual greeting",
				Parameters: map[string]interface{}{
					"name":   "World",
					"formal": false,
				},
				URI: "example://template/greeting?name=World&formal=false",
			},
			{
				Name: "Formal greeting",
				Parameters: map[string]interface{}{
					"name":   "Mr. Smith",
					"formal": true,
				},
				URI: "example://template/greeting?name=Mr.%20Smith&formal=true",
			},
		},
	}

	provider.RegisterResource(textResource)
	provider.RegisterResource(jsonResource)
	provider.RegisterTemplate(template)

	// Create a custom provider that wraps the base provider and implements resources
	customProvider := &CustomResourcesProvider{BaseResourcesProvider: *provider}
	return customProvider
}

// CustomResourcesProvider extends BaseResourcesProvider to implement resource reading
type CustomResourcesProvider struct {
	server.BaseResourcesProvider
}

// ReadResource implements custom resource reading
func (p *CustomResourcesProvider) ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error) {
	switch uri {
	case "example://text/greeting.txt":
		return &protocol.ResourceContents{
			URI:     uri,
			Type:    "text/plain",
			Content: json.RawMessage(`"Hello, World!"`),
		}, nil

	case "example://json/config.json":
		config := map[string]interface{}{
			"appName":  "Streamable MCP Example",
			"version":  "1.0.0",
			"debug":    false,
			"maxItems": 100,
		}
		content, _ := json.Marshal(config)
		return &protocol.ResourceContents{
			URI:     uri,
			Type:    "application/json",
			Content: content,
		}, nil

	case "example://template/greeting":
		name, ok := templateParams["name"].(string)
		if !ok {
			name = ""
		}

		if name == "" {
			name = "User"
		}

		formal, _ := templateParams["formal"].(bool)
		var greeting string
		if formal {
			greeting = fmt.Sprintf("Good day, %s. How may I assist you today?", name)
		} else {
			greeting = fmt.Sprintf("Hey %s! What's up?", name)
		}

		return &protocol.ResourceContents{
			URI:     uri,
			Type:    "text/plain",
			Content: json.RawMessage(`"` + greeting + `"`),
		}, nil

	default:
		// Call the parent implementation
		return p.BaseResourcesProvider.ReadResource(ctx, uri, templateParams, rangeOpt)
	}
}

func registerExamplePrompts(provider *server.BasePromptsProvider) {
	// Example prompt
	basicPrompt := protocol.Prompt{
		ID:          "greeting",
		Name:        "Basic Greeting",
		Description: "A simple greeting prompt",
		Messages: []protocol.PromptMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello, my name is {{name}}. {{question}}",
				Parameters: []string{"name", "question"},
			},
		},
		Parameters: []protocol.PromptParameter{
			{
				Name:        "name",
				Description: "The user's name",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "question",
				Description: "An optional question to ask",
				Type:        "string",
				Required:    false,
				Default:     "How are you today?",
			},
		},
		Examples: []protocol.PromptExample{
			{
				Name:        "Basic greeting",
				Description: "A simple greeting with a name",
				Parameters: map[string]interface{}{
					"name":     "Alice",
					"question": "What's the weather like today?",
				},
				Result: "Hello Alice! I don't have access to real-time weather information, but I'd be happy to help you with other questions.",
			},
		},
		Tags: []string{"basic", "greeting"},
	}

	provider.RegisterPrompt(basicPrompt)
}
