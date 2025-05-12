# Go Model Context Protocol SDK Documentation

## Package mcp

```go
import "github.com/ajitpratap0/mcp-sdk-go"
```

Package mcp provides a comprehensive implementation of the Model Context Protocol (MCP) specification (2025-03-26) in Go. MCP is a protocol that standardizes communication between AI models and client applications, enabling rich context sharing and structured interactions.

### Variables

```go
const Version = "1.0.0"
```
Version represents the current version of the SDK.

```go
// Protocol constants for capabilities
const (
    CapabilityTools                 = protocol.CapabilityTools
    CapabilityResources             = protocol.CapabilityResources
    CapabilityResourceSubscriptions = protocol.CapabilityResourceSubscriptions
    CapabilityPrompts               = protocol.CapabilityPrompts
    CapabilityComplete              = protocol.CapabilityComplete
    CapabilityRoots                 = protocol.CapabilityRoots
    CapabilitySampling              = protocol.CapabilitySampling
    CapabilityLogging               = protocol.CapabilityLogging
    CapabilityPagination            = protocol.CapabilityPagination
)
```
Constants for various MCP capabilities that can be negotiated between clients and servers.

### Functions

```go
var NewClient = client.New
```
NewClient creates a new MCP client instance.

```go
var NewServer = server.New
```
NewServer creates a new MCP server instance.

```go
var NewStdioTransport = transport.NewStdioTransport
```
NewStdioTransport creates a new stdio transport, which is the recommended transport mechanism in the MCP specification.

```go
var NewHTTPTransport = transport.NewHTTPTransport
```
NewHTTPTransport creates a new HTTP transport for MCP communication.

```go
var NewStreamableHTTPTransport = transport.NewStreamableHTTPTransport
```
NewStreamableHTTPTransport creates a new HTTP transport with Server-Sent Events (SSE) support for streaming interactions.

### Client Options

```go
var WithClientName = client.WithName
```
WithClientName sets the client name for identification.

```go
var WithClientVersion = client.WithVersion
```
WithClientVersion sets the client version.

```go
var WithClientCapability = client.WithCapability
```
WithClientCapability enables or disables a specific capability for the client.

```go
var WithClientFeatureOptions = client.WithFeatureOptions
```
WithClientFeatureOptions allows setting additional feature options for capabilities.

### Server Options

```go
var WithServerName = server.WithName
```
WithServerName sets the server name for identification.

```go
var WithServerVersion = server.WithVersion
```
WithServerVersion sets the server version.

```go
var WithServerDescription = server.WithDescription
```
WithServerDescription sets a human-readable description of the server.

```go
var WithServerHomepage = server.WithHomepage
```
WithServerHomepage sets the URL of the server's homepage.

```go
var WithServerCapability = server.WithCapability
```
WithServerCapability enables or disables a specific capability for the server.

```go
var WithServerFeatureOptions = server.WithFeatureOptions
```
WithServerFeatureOptions allows setting additional feature options for capabilities.

```go
var WithToolsProvider = server.WithToolsProvider
```
WithToolsProvider registers a tools provider with the server.

```go
var WithResourcesProvider = server.WithResourcesProvider
```
WithResourcesProvider registers a resources provider with the server.

```go
var WithPromptsProvider = server.WithPromptsProvider
```
WithPromptsProvider registers a prompts provider with the server.

```go
var WithCompletionProvider = server.WithCompletionProvider
```
WithCompletionProvider registers a completion provider with the server.

```go
var WithRootsProvider = server.WithRootsProvider
```
WithRootsProvider registers a roots provider with the server.

```go
var WithLogger = server.WithLogger
```
WithLogger sets the logger for the server.

### Provider Creation

```go
var NewBaseToolsProvider = server.NewBaseToolsProvider
```
NewBaseToolsProvider creates a new base tools provider implementation.

```go
var NewBaseResourcesProvider = server.NewBaseResourcesProvider
```
NewBaseResourcesProvider creates a new base resources provider implementation.

```go
var NewBasePromptsProvider = server.NewBasePromptsProvider
```
NewBasePromptsProvider creates a new base prompts provider implementation.

### Transport Options

```go
var WithRequestTimeout = transport.WithRequestTimeout
```
WithRequestTimeout sets the request timeout for transports.

## Package mcp/pkg/client

```go
import "github.com/ajitpratap0/mcp-sdk-go/pkg/client"
```

Package client provides the client-side implementation of the MCP protocol, allowing applications to connect to MCP servers and consume their capabilities.

### Types

```go
type Client interface {
    // Initialize initializes the client with the provided context
    Initialize(ctx context.Context) error

    // Start starts the client's transport
    Start(ctx context.Context) error

    // InitializeAndStart combines Initialize and Start operations
    InitializeAndStart(ctx context.Context) error

    // Close closes the client and its transport
    Close() error

    // HasCapability checks if a specific capability is supported
    HasCapability(capability string) bool

    // Capabilities returns the map of all supported capabilities
    Capabilities() map[string]bool

    // ListTools lists available tools from the server
    ListTools(ctx context.Context, query string, pagination *protocol.PaginationRequest) ([]protocol.Tool, *protocol.PaginationResult, error)

    // ListAllTools automatically paginates to retrieve all tools matching the query
    ListAllTools(ctx context.Context, query string) ([]protocol.Tool, error)

    // GetTool retrieves a specific tool by ID
    GetTool(ctx context.Context, id string) (*protocol.Tool, error)

    // InvokeTool invokes a tool with the given parameters
    InvokeTool(ctx context.Context, id string, params map[string]interface{}) (interface{}, error)

    // ListResources lists available resources from the server
    ListResources(ctx context.Context, query string, pagination *protocol.PaginationRequest) ([]protocol.Resource, *protocol.PaginationResult, error)

    // ListAllResources automatically paginates to retrieve all resources matching the query
    ListAllResources(ctx context.Context, query string) ([]protocol.Resource, error)

    // GetResource retrieves a specific resource by ID
    GetResource(ctx context.Context, id string) (*protocol.Resource, error)

    // SubscribeResource subscribes to resource updates
    SubscribeResource(ctx context.Context, id string) error

    // UnsubscribeResource unsubscribes from resource updates
    UnsubscribeResource(ctx context.Context, id string) error

    // ListPrompts lists available prompts from the server
    ListPrompts(ctx context.Context, query string, pagination *protocol.PaginationRequest) ([]protocol.Prompt, *protocol.PaginationResult, error)

    // ListAllPrompts automatically paginates to retrieve all prompts matching the query
    ListAllPrompts(ctx context.Context, query string) ([]protocol.Prompt, error)

    // GetPrompt retrieves a specific prompt by ID
    GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error)

    // GetRoots retrieves the root resources from the server
    GetRoots(ctx context.Context) ([]string, error)

    // Complete requests a completion from the server
    Complete(ctx context.Context, request *protocol.CompletionRequest) (*protocol.CompletionResponse, error)

    // SetSamplingCallback sets a callback function for sampling events
    SetSamplingCallback(callback SamplingCallback)

    // SetResourceChangedCallback sets a callback function for resource change notifications
    SetResourceChangedCallback(callback ResourceChangedCallback)

    // RegisterHandler registers a custom method handler
    RegisterHandler(method string, handler MessageHandler)
}
```
Client is the main interface for an MCP client.

```go
type Option func(*ClientConfig)
```
Option represents a client configuration option.

```go
type SamplingCallback func(protocol.SamplingEvent)
```
SamplingCallback is called when sampling events are received.

```go
type ResourceChangedCallback func(string, interface{})
```
ResourceChangedCallback is called when resource change notifications are received.

```go
type MessageHandler func(context.Context, json.RawMessage) (interface{}, error)
```
MessageHandler handles custom JSON-RPC method calls.

### Functions

```go
func New(transport transport.Transport, options ...Option) Client
```
New creates a new MCP client with the specified transport and options.

```go
func NewStdioClient(options ...Option) Client
```
NewStdioClient creates a new client with stdio transport.

### Options

```go
func WithName(name string) Option
```
WithName sets the client name.

```go
func WithVersion(version string) Option
```
WithVersion sets the client version.

```go
func WithCapability(capability string, enabled bool) Option
```
WithCapability enables or disables a capability.

```go
func WithFeatureOptions(capability string, options map[string]interface{}) Option
```
WithFeatureOptions sets additional options for a capability.

## Package mcp/pkg/server

```go
import "github.com/ajitpratap0/mcp-sdk-go/pkg/server"
```

Package server provides the server-side implementation of the MCP protocol, allowing applications to expose resources, tools, and other capabilities to MCP clients.

### Types

```go
type Server interface {
    // Start starts the server with the provided context
    Start(ctx context.Context) error

    // Stop stops the server
    Stop() error

    // RegisterErrorHandler registers a custom error handler
    RegisterErrorHandler(handler ErrorHandler)

    // RegisterMethodHandler registers a custom method handler
    RegisterMethodHandler(method string, handler MethodHandler)
}
```
Server is the main interface for an MCP server.

```go
type Option func(*ServerConfig)
```
Option represents a server configuration option.

```go
type ErrorHandler func(err error)
```
ErrorHandler is a function that handles server errors.

```go
type MethodHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)
```
MethodHandler handles custom method calls.

```go
type ToolsProvider interface {
    // ListTools lists the available tools
    ListTools(ctx context.Context, query string, pagination *protocol.PaginationRequest) ([]protocol.Tool, *protocol.PaginationResult, error)

    // GetTool gets a specific tool by ID
    GetTool(ctx context.Context, id string) (*protocol.Tool, error)

    // InvokeTool invokes a tool with the given parameters
    InvokeTool(ctx context.Context, id string, params map[string]interface{}) (interface{}, error)
}
```
ToolsProvider provides tool-related functionality.

```go
type ResourcesProvider interface {
    // ListResources lists the available resources
    ListResources(ctx context.Context, query string, pagination *protocol.PaginationRequest) ([]protocol.Resource, *protocol.PaginationResult, error)

    // GetResource gets a specific resource by ID
    GetResource(ctx context.Context, id string) (*protocol.Resource, error)

    // SubscribeResource subscribes to updates for a resource
    SubscribeResource(ctx context.Context, id string) error

    // UnsubscribeResource unsubscribes from updates for a resource
    UnsubscribeResource(ctx context.Context, id string) error

    // NotifyResourceChanged notifies subscribers of resource changes
    NotifyResourceChanged(ctx context.Context, id string, value interface{})
}
```
ResourcesProvider provides resource-related functionality.

```go
type PromptsProvider interface {
    // ListPrompts lists the available prompts
    ListPrompts(ctx context.Context, query string, pagination *protocol.PaginationRequest) ([]protocol.Prompt, *protocol.PaginationResult, error)

    // GetPrompt gets a specific prompt by ID
    GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error)
}
```
PromptsProvider provides prompt-related functionality.

```go
type CompletionProvider interface {
    // Complete generates a completion for the given request
    Complete(ctx context.Context, request *protocol.CompletionRequest) (*protocol.CompletionResponse, error)
}
```
CompletionProvider provides completion-related functionality.

```go
type RootsProvider interface {
    // GetRoots gets the root resources
    GetRoots(ctx context.Context) ([]string, error)
}
```
RootsProvider provides access to root resources.

```go
type LoggingProvider interface {
    // Log logs a message
    Log(ctx context.Context, level string, message string, data map[string]interface{})
}
```
LoggingProvider provides logging functionality.

### Functions

```go
func New(transport transport.Transport, options ...Option) Server
```
New creates a new MCP server with the specified transport and options.

```go
func NewBaseToolsProvider() *BaseToolsProvider
```
NewBaseToolsProvider creates a new base tools provider.

```go
func NewBaseResourcesProvider() *BaseResourcesProvider
```
NewBaseResourcesProvider creates a new base resources provider.

```go
func NewBasePromptsProvider() *BasePromptsProvider
```
NewBasePromptsProvider creates a new base prompts provider.

### Options

```go
func WithName(name string) Option
```
WithName sets the server name.

```go
func WithVersion(version string) Option
```
WithVersion sets the server version.

```go
func WithDescription(description string) Option
```
WithDescription sets the server description.

```go
func WithHomepage(homepage string) Option
```
WithHomepage sets the server homepage URL.

```go
func WithCapability(capability string, enabled bool) Option
```
WithCapability enables or disables a capability.

```go
func WithFeatureOptions(capability string, options map[string]interface{}) Option
```
WithFeatureOptions sets additional options for a capability.

```go
func WithToolsProvider(provider ToolsProvider) Option
```
WithToolsProvider sets the tools provider.

```go
func WithResourcesProvider(provider ResourcesProvider) Option
```
WithResourcesProvider sets the resources provider.

```go
func WithPromptsProvider(provider PromptsProvider) Option
```
WithPromptsProvider sets the prompts provider.

```go
func WithCompletionProvider(provider CompletionProvider) Option
```
WithCompletionProvider sets the completion provider.

```go
func WithRootsProvider(provider RootsProvider) Option
```
WithRootsProvider sets the roots provider.

```go
func WithLogger(logger LoggingProvider) Option
```
WithLogger sets the logger for the server.

## Package mcp/pkg/transport

```go
import "github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
```

Package transport provides various transport mechanisms for MCP communication.

### Types

```go
type Transport interface {
    // Initialize initializes the transport
    Initialize() error

    // Start starts the transport
    Start(ctx context.Context) error

    // Stop stops the transport
    Stop()

    // Send sends a message
    Send(data []byte) error

    // SetReceiveHandler sets the handler for received messages
    SetReceiveHandler(handler ReceiveHandler)

    // SetErrorHandler sets the handler for transport errors
    SetErrorHandler(handler ErrorHandler)
}
```
Transport is the main interface for transport implementations.

```go
type ReceiveHandler func(data []byte)
```
ReceiveHandler handles received messages.

```go
type ErrorHandler func(err error)
```
ErrorHandler handles transport errors.

```go
type Option func(*TransportConfig)
```
Option represents a transport configuration option.

### Functions

```go
func NewStdioTransport(options ...Option) Transport
```
NewStdioTransport creates a new stdio transport.

```go
func NewHTTPTransport(options ...Option) Transport
```
NewHTTPTransport creates a new HTTP transport.

```go
func NewStreamableHTTPTransport(options ...Option) Transport
```
NewStreamableHTTPTransport creates a new HTTP transport with SSE support.

### Options

```go
func WithRequestTimeout(timeout time.Duration) Option
```
WithRequestTimeout sets the request timeout for HTTP transports.

## Package mcp/pkg/protocol

```go
import "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
```

Package protocol defines the core types and structures used in the MCP protocol.

### Constants

```go
// Capability constants
const (
    CapabilityTools                 = "tools"
    CapabilityResources             = "resources"
    CapabilityResourceSubscriptions = "resource_subscriptions"
    CapabilityPrompts               = "prompts"
    CapabilityComplete              = "complete"
    CapabilityRoots                 = "roots"
    CapabilitySampling              = "sampling"
    CapabilityLogging               = "logging"
    CapabilityPagination            = "pagination"
)
```
Constants for various MCP capabilities.

### Types

```go
type JSONRPC struct {
    Version string      `json:"jsonrpc"`
    ID      interface{} `json:"id,omitempty"`
    Method  string      `json:"method,omitempty"`
    Params  interface{} `json:"params,omitempty"`
    Result  interface{} `json:"result,omitempty"`
    Error   *JSONRPCError `json:"error,omitempty"`
}
```
JSONRPC represents a JSON-RPC 2.0 message.

```go
type JSONRPCError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```
JSONRPCError represents a JSON-RPC 2.0 error.

```go
type InitializeParams struct {
    Name          string                 `json:"name"`
    Version       string                 `json:"version"`
    Description   string                 `json:"description,omitempty"`
    Homepage      string                 `json:"homepage,omitempty"`
    Capabilities  map[string]bool        `json:"capabilities"`
    FeatureOptions map[string]map[string]interface{} `json:"feature_options,omitempty"`
}
```
InitializeParams represents the parameters for the initialize method.

```go
type InitializeResult struct {
    Name          string                 `json:"name"`
    Version       string                 `json:"version"`
    Description   string                 `json:"description,omitempty"`
    Homepage      string                 `json:"homepage,omitempty"`
    Capabilities  map[string]bool        `json:"capabilities"`
    FeatureOptions map[string]map[string]interface{} `json:"feature_options,omitempty"`
}
```
InitializeResult represents the result of the initialize method.

```go
type Tool struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters,omitempty"`
}
```
Tool represents an MCP tool.

```go
type Resource struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Data        interface{}            `json:"data,omitempty"`
    Relations   map[string][]string    `json:"relations,omitempty"`
}
```
Resource represents an MCP resource.

```go
type Prompt struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Template    string                 `json:"template"`
    Parameters  map[string]interface{} `json:"parameters,omitempty"`
}
```
Prompt represents an MCP prompt.

```go
type CompletionRequest struct {
    Prompt      string                 `json:"prompt"`
    MaxTokens   int                    `json:"max_tokens,omitempty"`
    Temperature float64                `json:"temperature,omitempty"`
    Options     map[string]interface{} `json:"options,omitempty"`
}
```
CompletionRequest represents a request for completion.

```go
type CompletionResponse struct {
    Text        string                 `json:"text"`
    FinishReason string                `json:"finish_reason,omitempty"`
    Usage       *CompletionUsage       `json:"usage,omitempty"`
}
```
CompletionResponse represents a completion response.

```go
type CompletionUsage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```
CompletionUsage represents token usage information.

```go
type PaginationRequest struct {
    Limit  int    `json:"limit,omitempty"`
    Offset int    `json:"offset,omitempty"`
    After  string `json:"after,omitempty"`
}
```
PaginationRequest represents pagination parameters.

```go
type PaginationResult struct {
    Total   int    `json:"total,omitempty"`
    HasMore bool   `json:"has_more"`
    Offset  int    `json:"offset,omitempty"`
    Next    string `json:"next,omitempty"`
}
```
PaginationResult represents pagination results.

```go
type SamplingEvent struct {
    Type    string      `json:"type"`
    Data    interface{} `json:"data"`
    TokenID string      `json:"token_id,omitempty"`
}
```
SamplingEvent represents a sampling event.

## Package mcp/pkg/pagination

```go
import "github.com/ajitpratap0/mcp-sdk-go/pkg/pagination"
```

Package pagination provides utilities for handling paginated operations.

### Types

```go
type Paginator interface {
    // GetNextPage gets the next page of results
    GetNextPage(ctx context.Context) (interface{}, *protocol.PaginationResult, error)

    // HasMorePages checks if there are more pages
    HasMorePages() bool

    // GetAllPages gets all remaining pages
    GetAllPages(ctx context.Context) (interface{}, error)
}
```
Paginator provides pagination functionality.

### Functions

```go
func NewPaginator(initialResult interface{}, initialPagination *protocol.PaginationResult, pageFunc PageFunction) Paginator
```
NewPaginator creates a new paginator with the given parameters.

## Package mcp/pkg/utils

```go
import "github.com/ajitpratap0/mcp-sdk-go/pkg/utils"
```

Package utils provides utility functions for MCP operations.

### Functions

```go
func ValidateJSON(data []byte) error
```
ValidateJSON validates that the provided data is valid JSON.

```go
func ValidateJSONRPC(message *protocol.JSONRPC) error
```
ValidateJSONRPC validates a JSON-RPC message.

```go
func GenerateRequestID() interface{}
```
GenerateRequestID generates a unique request ID.

```go
func IsNotificationMethod(method string) bool
```
IsNotificationMethod checks if a method is a notification method.

## Examples

### Creating an MCP Client

```go
package main

import (
    "context"
    "log"

    "github.com/ajitpratap0/mcp-sdk-go"
)

func main() {
    // Create a new client with stdio transport
    client := mcp.NewClient(
        mcp.NewStdioTransport(),
        mcp.WithClientName("ExampleClient"),
        mcp.WithClientVersion("1.0.0"),
        mcp.WithClientCapability(mcp.CapabilityTools, true),
        mcp.WithClientCapability(mcp.CapabilityResources, true),
    )

    // Initialize and start the client
    ctx := context.Background()
    if err := client.InitializeAndStart(ctx); err != nil {
        log.Fatalf("Failed to initialize client: %v", err)
    }
    defer client.Close()

    // List all tools
    tools, err := client.ListAllTools(ctx, "")
    if err != nil {
        log.Fatalf("Failed to list tools: %v", err)
    }

    // Process tools
    for _, tool := range tools {
        log.Printf("Tool: %s - %s", tool.Name, tool.Description)

        // Invoke a tool if needed
        if tool.ID == "example-tool" {
            result, err := client.InvokeTool(ctx, tool.ID, map[string]interface{}{
                "param1": "value1",
                "param2": 42,
            })
            if err != nil {
                log.Printf("Failed to invoke tool: %v", err)
            } else {
                log.Printf("Tool result: %v", result)
            }
        }
    }
}
```

### Creating an MCP Server

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/ajitpratap0/mcp-sdk-go"
)

func main() {
    // Create providers
    toolsProvider := mcp.NewBaseToolsProvider()
    resourcesProvider := mcp.NewBaseResourcesProvider()

    // Add a sample tool
    toolsProvider.AddTool(&protocol.Tool{
        ID:          "sample-tool",
        Name:        "Sample Tool",
        Description: "A sample tool for demonstration",
        Parameters: map[string]interface{}{
            "param1": map[string]interface{}{
                "type":        "string",
                "description": "A string parameter",
                "required":    true,
            },
            "param2": map[string]interface{}{
                "type":        "number",
                "description": "A numeric parameter",
                "required":    false,
            },
        },
    }, func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        // Tool implementation
        return map[string]interface{}{
            "result": "Success",
            "params": params,
        }, nil
    })

    // Add a sample resource
    resourcesProvider.AddResource(&protocol.Resource{
        ID:          "sample-resource",
        Type:        "text",
        Name:        "Sample Resource",
        Description: "A sample resource for demonstration",
        Data:        "This is a sample resource content",
    })

    // Create server
    server := mcp.NewServer(
        mcp.NewStdioTransport(),
        mcp.WithServerName("ExampleServer"),
        mcp.WithServerVersion("1.0.0"),
        mcp.WithServerDescription("An example MCP server"),
        mcp.WithServerHomepage("https://example.com"),
        mcp.WithServerCapability(mcp.CapabilityTools, true),
        mcp.WithServerCapability(mcp.CapabilityResources, true),
        mcp.WithToolsProvider(toolsProvider),
        mcp.WithResourcesProvider(resourcesProvider),
    )

    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigChan
        log.Println("Shutting down...")
        cancel()
    }()

    // Start server
    log.Println("Starting MCP server...")
    if err := server.Start(ctx); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

### Using HTTP Transport

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/ajitpratap0/mcp-sdk-go"
)

func main() {
    // Create HTTP transport with configuration
    transport := mcp.NewHTTPTransport(
        mcp.WithRequestTimeout(10 * time.Second),
    )

    // Create client with HTTP transport
    client := mcp.NewClient(
        transport,
        mcp.WithClientName("HTTPClient"),
        mcp.WithClientVersion("1.0.0"),
    )

    // Initialize and start
    ctx := context.Background()
    if err := client.InitializeAndStart(ctx); err != nil {
        log.Fatalf("Failed to initialize client: %v", err)
    }
    defer client.Close()

    // Use client...
}
```

### Pagination Example

```go
package main

import (
    "context"
    "log"

    "github.com/ajitpratap0/mcp-sdk-go"
    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func main() {
    client := mcp.NewClient(/* ... */)
    ctx := context.Background()

    // Manual pagination
    var allTools []protocol.Tool
    pagination := &protocol.PaginationRequest{
        Limit: 10,
    }

    for {
        tools, pagResult, err := client.ListTools(ctx, "", pagination)
        if err != nil {
            log.Fatalf("Failed to list tools: %v", err)
        }

        allTools = append(allTools, tools...)

        if !pagResult.HasMore {
            break
        }

        // Update pagination for next request
        if pagResult.Next != "" {
            pagination = &protocol.PaginationRequest{
                After: pagResult.Next,
                Limit: 10,
            }
        } else {
            pagination = &protocol.PaginationRequest{
                Offset: pagResult.Offset + len(tools),
                Limit:  10,
            }
        }
    }

    log.Printf("Retrieved %d tools using manual pagination", len(allTools))

    // Automatic pagination
    tools, err := client.ListAllTools(ctx, "")
    if err != nil {
        log.Fatalf("Failed to list all tools: %v", err)
    }

    log.Printf("Retrieved %d tools using automatic pagination", len(tools))
}
```

## Performance Considerations

The Go MCP SDK is designed for high performance and efficiency:

1. **Zero-copy Approach**: The SDK minimizes memory copying where possible, using references and pointers to share data.

2. **Efficient Message Handling**: JSON parsing is optimized to avoid unnecessary allocations.

3. **Stream Processing**: Streaming responses are processed incrementally to reduce memory usage.

4. **Goroutine Management**: The SDK carefully manages goroutines to prevent leaks and ensure efficient concurrency.

5. **Connection Pooling**: HTTP transports use connection pooling to reduce connection overhead.

6. **Backpressure Handling**: The SDK includes mechanisms to handle backpressure in high-throughput scenarios.

## Thread Safety

The Go MCP SDK is designed to be thread-safe:

1. All public methods of the Client and Server interfaces can be safely called concurrently from multiple goroutines.

2. Callbacks, such as SamplingCallback and ResourceChangedCallback, may be invoked concurrently, so implementations should be thread-safe.

3. Provider implementations should ensure thread safety for their methods, as they may be called concurrently.

4. Transports handle concurrent read/write operations safely.

## Error Handling

The SDK uses Go's standard error handling patterns:

1. Every operation that can fail returns an error as the last return value.

2. Error types are specific and include context about what went wrong.

3. Errors from JSON-RPC are properly translated into Go errors with appropriate context.

4. Transports provide error handling mechanisms through error callbacks.

## Concurrency Model

The SDK uses goroutines and channels to implement concurrency:

1. Each transport runs in a dedicated goroutine for receiving messages.

2. Message handling is done in separate goroutines to prevent blocking.

3. Cancellation is properly handled through context.Context.

4. Resource subscriptions use channels to deliver updates.

## Best Practices

1. **Always Close Clients**: Call the Close method when finished to release resources.

2. **Use Context for Cancellation**: Pass appropriate contexts to operations to enable cancellation.

3. **Handle Errors**: Always check returned errors and handle them appropriately.

4. **Use Pagination**: For large result sets, use pagination to avoid memory issues.

5. **Implement Proper Error Recovery**: Servers should implement proper error recovery to prevent crashes.

6. **Set Timeouts**: Use appropriate timeouts for operations to prevent hanging.

7. **Use Feature Detection**: Always check capabilities before using features.

## Versioning

The SDK follows semantic versioning:

1. **Major Version**: Incompatible API changes
2. **Minor Version**: New features in a backward-compatible manner
3. **Patch Version**: Backward-compatible bug fixes

The version can be accessed through the `Version` constant in the mcp package.
