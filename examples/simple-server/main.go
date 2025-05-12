package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

func main() {
	// Create a stdio transport
	t := transport.NewStdioTransportWithStdInOut()

	// Create and register tool provider
	basicToolsProvider := server.NewBaseToolsProvider()
	customToolsProvider := registerExampleTools(basicToolsProvider)

	// Create and register resources provider
	basicResourcesProvider := server.NewBaseResourcesProvider()
	customResourcesProvider := registerExampleResources(basicResourcesProvider)

	// Create prompts provider
	promptsProvider := server.NewBasePromptsProvider()
	registerExamplePrompts(promptsProvider)

	// Create server
	s := server.New(t,
		server.WithName("SimpleExample"),
		server.WithVersion("1.0.0"),
		server.WithDescription("A simple example MCP server"),
		server.WithHomepage("https://github.com/ajitpratap0/mcp-sdk-go"),
		server.WithToolsProvider(customToolsProvider),
		server.WithResourcesProvider(customResourcesProvider),
		server.WithPromptsProvider(promptsProvider),
		server.WithCapability(protocol.CapabilityResourceSubscriptions, true),
		server.WithCapability(protocol.CapabilityLogging, true),
	)

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
	}()

	// Start server (blocking)
	log.Println("Starting MCP server...")
	if err := s.Start(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
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

	// Example time tool
	timeTool := protocol.Tool{
		Name:        "currentTime",
		Description: "Returns the current time",
		OutputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"time": {
					"type": "string",
					"description": "The current time in RFC3339 format"
				},
				"timestamp": {
					"type": "number",
					"description": "The current Unix timestamp"
				}
			}
		}`),
		Categories: []string{"demo", "utility"},
	}

	provider.RegisterTool(helloTool)
	provider.RegisterTool(timeTool)

	// Create a custom provider that wraps the base provider and implements the tools
	customProvider := &CustomToolsProvider{provider: provider}
	return customProvider
}

// CustomToolsProvider extends BaseToolsProvider to implement the tools
type CustomToolsProvider struct {
	provider *server.BaseToolsProvider
}

// ListTools delegates to the base provider
func (p *CustomToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	return p.provider.ListTools(ctx, category, pagination)
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

	case "currentTime":
		now := time.Now()
		result, _ := json.Marshal(map[string]interface{}{
			"time":      now.Format(time.RFC3339),
			"timestamp": now.Unix(),
		})
		return &protocol.CallToolResult{Result: result}, nil

	default:
		// Call the parent implementation
		return p.provider.CallTool(ctx, name, input, contextData)
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
	customProvider := &CustomResourcesProvider{provider: provider}
	return customProvider
}

// CustomResourcesProvider extends BaseResourcesProvider to implement resource reading
type CustomResourcesProvider struct {
	provider *server.BaseResourcesProvider
}

// ListResources delegates to the base provider
func (p *CustomResourcesProvider) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, int, string, bool, error) {
	return p.provider.ListResources(ctx, uri, recursive, pagination)
}

// SubscribeResource delegates to the base provider
func (p *CustomResourcesProvider) SubscribeResource(ctx context.Context, uri string, recursive bool) (bool, error) {
	return p.provider.SubscribeResource(ctx, uri, recursive)
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
			"appName":  "MCP Example",
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
		return p.provider.ReadResource(ctx, uri, templateParams, rangeOpt)
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
				Role:       "user",
				Content:    "Hello, my name is {{name}}. {{question}}",
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

	// Example technical prompt
	technicalPrompt := protocol.Prompt{
		ID:          "code-review",
		Name:        "Code Review",
		Description: "Review code and provide suggestions",
		Messages: []protocol.PromptMessage{
			{
				Role:    "system",
				Content: "You are an expert code reviewer. Analyze the following code and provide constructive feedback.",
			},
			{
				Role:         "user",
				Content:      "Please review this {{language}} code: {{code}}",
				Parameters:   []string{"language", "code"},
				ResourceRefs: []string{"code"},
			},
		},
		Parameters: []protocol.PromptParameter{
			{
				Name:        "language",
				Description: "The programming language",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "code",
				Description: "The code to review",
				Type:        "string",
				Required:    true,
			},
		},
		Tags: []string{"technical", "code"},
	}

	provider.RegisterPrompt(basicPrompt)
	provider.RegisterPrompt(technicalPrompt)
}
