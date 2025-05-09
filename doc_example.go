//go:build ignore
// +build ignore

// This file is an example documentation file that's not meant to be included in builds.
// It contains examples of how to use the MCP SDK and is provided for reference only.
// The actual SDK implementation is in the pkg directory.

// Package mcp provides a comprehensive implementation of the Model Context Protocol (MCP)
// specification (2025-03-26) in Go. MCP is a protocol that standardizes communication
// between AI models and client applications, enabling rich context sharing and
// structured interactions.
//
// This package contains the core components for implementing MCP clients and servers.
// It includes support for all MCP capabilities:
//
//   - Tools: Allow clients to invoke operations on the server
//   - Resources: Allow clients to access structured data from the server
//   - Prompts: Allow clients to use predefined prompt templates
//   - Completion: Allow clients to generate text completions
//   - Roots: Allow clients to discover available resources
//   - Sampling: Allow servers to stream intermediate generation results
//   - Logging: Allow clients and servers to exchange log messages
//
// The package is designed with the following principles:
//
//   - Simplicity: Clear, idiomatic Go API with minimal dependencies
//   - Performance: Efficient message handling and streaming support
//   - Scalability: Support for high-throughput scenarios
//   - Conformance: Full implementation of the MCP specification
//   - Robustness: Comprehensive validation and error handling
//
// # Creating an MCP Client
//
// To create a client that connects to an MCP server:
//
//	import (
//	    "context"
//	    "log"
//	    "github.com/ajitpratap0/mcp-sdk-go"
//	)
//
//	func main() {
//	    // Create a client with stdio transport
//	    client := mcp.NewClient(
//	        mcp.NewStdioTransport(),
//	        mcp.WithClientName("ExampleClient"),
//	        mcp.WithClientVersion("1.0.0"),
//	        mcp.WithClientCapability(mcp.CapabilityTools, true),
//	    )
//
//	    // Initialize and start the client
//	    ctx := context.Background()
//	    if err := client.InitializeAndStart(ctx); err != nil {
//	        log.Fatalf("Failed to initialize client: %v", err)
//	    }
//	    defer client.Close()
//
//	    // Use client capabilities...
//	    if client.HasCapability(mcp.CapabilityTools) {
//	        tools, err := client.ListAllTools(ctx, "")
//	        if err != nil {
//	            log.Fatalf("Failed to list tools: %v", err)
//	        }
//
//	        for _, tool := range tools {
//	            log.Printf("Tool: %s - %s", tool.Name, tool.Description)
//	        }
//	    }
//	}
//
// # Creating an MCP Server
//
// To create a server that implements the MCP protocol:
//
//	import (
//	    "context"
//	    "log"
//	    "os"
//	    "os/signal"
//	    "syscall"
//	    "github.com/ajitpratap0/mcp-sdk-go"
//	)
//
//	func main() {
//	    // Create providers for server capabilities
//	    toolsProvider := mcp.NewBaseToolsProvider()
//	    resourcesProvider := mcp.NewBaseResourcesProvider()
//
//	    // Add a sample tool
//	    toolsProvider.AddTool("sample-tool", "Sample Tool", "A sample tool",
//	        func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
//	            return map[string]string{"result": "success"}, nil
//	        })
//
//	    // Create and configure the server
//	    server := mcp.NewServer(
//	        mcp.NewStdioTransport(),
//	        mcp.WithServerName("ExampleServer"),
//	        mcp.WithServerVersion("1.0.0"),
//	        mcp.WithServerDescription("An example MCP server"),
//	        mcp.WithToolsProvider(toolsProvider),
//	        mcp.WithResourcesProvider(resourcesProvider),
//	    )
//
//	    // Set up graceful shutdown
//	    ctx, cancel := context.WithCancel(context.Background())
//	    defer cancel()
//
//	    sigChan := make(chan os.Signal, 1)
//	    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
//	    go func() {
//	        <-sigChan
//	        log.Println("Shutting down...")
//	        cancel()
//	    }()
//
//	    // Start server (blocks until context is canceled)
//	    log.Println("Starting MCP server...")
//	    if err := server.Start(ctx); err != nil {
//	        log.Fatalf("Server error: %v", err)
//	    }
//	}
package mcp_examples

import "context"

// Version represents the current version of the SDK.
const Version = "1.0.0"

// ClientOption configures a client instance.
// Use the WithClient* functions to create options.
type ClientOption func(*ClientConfig)

// ServerOption configures a server instance.
// Use the WithServer* functions to create options.
type ServerOption func(*ServerConfig)

// TransportOption configures a transport instance.
// Use the transport-specific With* functions to create options.
type TransportOption func(*TransportConfig)

// Client represents an MCP client.
// It connects to an MCP server and provides access to the server's capabilities.
type Client interface {
	// Initialize initializes the client with the provided context.
	Initialize(ctx context.Context) error

	// Start starts the client's transport.
	Start(ctx context.Context) error

	// InitializeAndStart combines Initialize and Start operations for convenience.
	InitializeAndStart(ctx context.Context) error

	// Close closes the client and its transport.
	Close() error

	// HasCapability checks if the server supports a specific capability.
	HasCapability(capability string) bool
}

// Server represents an MCP server.
// It implements the server-side of the MCP protocol and exposes capabilities to clients.
type Server interface {
	// Start starts the server and begins accepting client connections.
	// This method blocks until the context is canceled.
	Start(ctx context.Context) error

	// Stop stops the server and closes all client connections.
	Stop() error
}

// Transport represents a communication channel between client and server.
// It is responsible for sending and receiving messages.
type Transport interface {
	// Initialize prepares the transport for use.
	Initialize() error

	// Start begins processing messages.
	// This method blocks until the context is canceled.
	Start(ctx context.Context) error

	// Stop halts the transport and cleans up resources.
	Stop() error

	// Send transmits a message over the transport.
	Send(data []byte) error
}

// Tool represents an operation that can be invoked by clients.
type Tool struct {
	// ID is a unique identifier for the tool.
	ID string

	// Name is a human-readable name for the tool.
	Name string

	// Description provides information about what the tool does.
	Description string

	// Parameters describes the input parameters for the tool.
	Parameters map[string]interface{}
}

// Resource represents data that can be accessed by clients.
type Resource struct {
	// ID is a unique identifier for the resource.
	ID string

	// Type indicates the kind of resource.
	Type string

	// Name is a human-readable name for the resource.
	Name string

	// Description provides information about what the resource contains.
	Description string

	// Data contains the resource content.
	Data interface{}
}

// Prompt represents a template for generating text.
type Prompt struct {
	// ID is a unique identifier for the prompt.
	ID string

	// Name is a human-readable name for the prompt.
	Name string

	// Description provides information about what the prompt is for.
	Description string

	// Template contains the prompt template text.
	Template string

	// Parameters describes the parameters for the prompt.
	Parameters map[string]interface{}
}

// NewClient creates a new MCP client with the specified transport and options.
func NewClient(transport Transport, options ...ClientOption) Client {
	// Implementation would be here in a real file
	return nil
}

// NewServer creates a new MCP server with the specified transport and options.
func NewServer(transport Transport, options ...ServerOption) Server {
	// Implementation would be here in a real file
	return nil
}

// NewStdioTransport creates a new stdio transport.
// This is the recommended transport mechanism in the MCP specification.
func NewStdioTransport(options ...TransportOption) Transport {
	// Implementation would be here in a real file
	return nil
}

// WithClientName sets the client name for identification.
func WithClientName(name string) ClientOption {
	// Implementation would be here in a real file
	return nil
}

// WithClientVersion sets the client version.
func WithClientVersion(version string) ClientOption {
	// Implementation would be here in a real file
	return nil
}

// WithClientCapability enables or disables a specific capability for the client.
func WithClientCapability(capability string, enabled bool) ClientOption {
	// Implementation would be here in a real file
	return nil
}

// WithServerName sets the server name for identification.
func WithServerName(name string) ServerOption {
	// Implementation would be here in a real file
	return nil
}

// WithServerVersion sets the server version.
func WithServerVersion(version string) ServerOption {
	// Implementation would be here in a real file
	return nil
}

// WithServerDescription sets a human-readable description of the server.
func WithServerDescription(description string) ServerOption {
	// Implementation would be here in a real file
	return nil
}

// NewBaseToolsProvider creates a new base tools provider implementation.
func NewBaseToolsProvider() ToolsProvider {
	// Implementation would be here in a real file
	return nil
}

// NewBaseResourcesProvider creates a new base resources provider implementation.
func NewBaseResourcesProvider() ResourcesProvider {
	// Implementation would be here in a real file
	return nil
}

// ToolsProvider provides tool-related functionality for a server.
type ToolsProvider interface {
	// AddTool adds a tool to the provider.
	AddTool(id, name, description string, handler ToolHandler)

	// GetTool retrieves a tool by ID.
	GetTool(id string) (*Tool, error)
}

// ResourcesProvider provides resource-related functionality for a server.
type ResourcesProvider interface {
	// AddResource adds a resource to the provider.
	AddResource(id, name, description string, data interface{})

	// GetResource retrieves a resource by ID.
	GetResource(id string) (*Resource, error)
}

// ToolHandler is a function that implements a tool's logic.
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)
