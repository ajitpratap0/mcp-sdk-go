package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
)

// CreateToolsProvider creates a reusable tools provider with common example tools
func CreateToolsProvider() *CustomToolsProvider {
	// Create base provider
	baseProvider := server.NewBaseToolsProvider()

	// Define common tools
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

	// Streaming tool for advanced transports
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

	// Register tools
	baseProvider.RegisterTool(helloTool)
	baseProvider.RegisterTool(timeTool)
	baseProvider.RegisterTool(streamingTool)

	return &CustomToolsProvider{provider: baseProvider}
}

// CustomToolsProvider extends BaseToolsProvider with common tool implementations
type CustomToolsProvider struct {
	provider *server.BaseToolsProvider
}

// ListTools delegates to the base provider
func (p *CustomToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	return p.provider.ListTools(ctx, category, pagination)
}

// CallTool implements the common tool execution logic
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

	case "countToTen":
		// Generate operation ID for streaming
		opID := fmt.Sprintf("stream-%d", time.Now().UnixNano())

		// Start streaming in background
		go func() {
			for i := 1; i <= 10; i++ {
				update := map[string]interface{}{
					"operationId": opID,
					"count":       i,
					"progress":    i * 10,
					"message":     fmt.Sprintf("Processing step %d of 10", i),
					"partial":     i < 10,
					"done":        i == 10,
				}

				updateJSON, _ := json.Marshal(update)
				fmt.Printf("Stream update: %s\n", string(updateJSON))
				time.Sleep(500 * time.Millisecond)
			}
		}()

		// Return initial streaming response
		streamingData, _ := json.Marshal(map[string]interface{}{
			"streaming": true,
			"message":   "Streaming operation started",
		})

		return &protocol.CallToolResult{
			Result:      streamingData,
			Partial:     true,
			OperationID: opID,
		}, nil

	default:
		return p.provider.CallTool(ctx, name, input, contextData)
	}
}

// CreateResourcesProvider creates a reusable resources provider with common example resources
func CreateResourcesProvider() *CustomResourcesProvider {
	baseProvider := server.NewBaseResourcesProvider()

	// Define common resources
	textResource := protocol.Resource{
		URI:         "example://text/greeting.txt",
		Name:        "Greeting",
		Description: "A simple greeting message",
		Type:        "text/plain",
		Size:        13,
		ModTime:     time.Now(),
	}

	jsonResource := protocol.Resource{
		URI:         "example://json/config.json",
		Name:        "Configuration",
		Description: "Example configuration data",
		Type:        "application/json",
		Size:        42,
		ModTime:     time.Now(),
	}

	// Resource template
	greetingTemplate := protocol.ResourceTemplate{
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

	// Register resources
	baseProvider.RegisterResource(textResource)
	baseProvider.RegisterResource(jsonResource)
	baseProvider.RegisterTemplate(greetingTemplate)

	return &CustomResourcesProvider{provider: baseProvider}
}

// CustomResourcesProvider extends BaseResourcesProvider with common resource implementations
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

// ReadResource implements common resource reading logic
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
		if !ok || name == "" {
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
		return p.provider.ReadResource(ctx, uri, templateParams, rangeOpt)
	}
}

// CreatePromptsProvider creates a reusable prompts provider with common example prompts
func CreatePromptsProvider() *server.BasePromptsProvider {
	provider := server.NewBasePromptsProvider()

	// Basic greeting prompt
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

	// Technical code review prompt
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

	return provider
}
