// Package mcp provides a comprehensive implementation of the Model Context Protocol.
//
// The Model Context Protocol (MCP) is a standardized communication protocol that enables
// AI models to interact with their environment through a well-defined interface. This
// package is the root of the MCP SDK for Go, providing convenient exports of the core
// components from the sub-packages.
//
// # Overview
//
// The MCP SDK consists of several sub-packages:
//
//   - pkg/client: Implements the client-side of the protocol
//   - pkg/server: Implements the server-side of the protocol
//   - pkg/protocol: Defines the core protocol types and messages
//   - pkg/transport: Provides transport mechanisms for communication
//   - pkg/pagination: Utilities for handling paginated results
//
// # Creating a Client
//
// To create a client that connects to an MCP server:
//
//	import (
//	    "context"
//	    "github.com/ajitpratap0/mcp-sdk-go"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
//	)
//
//	func main() {
//	    // Create a client with stdio transport
//	    client := mcp.NewClient(
//	        mcp.NewStdioTransport(),
//	        mcp.WithClientName("MyClient"),
//	        mcp.WithClientVersion("1.0.0"),
//	        mcp.WithClientCapability(protocol.CapabilitySampling, true),
//	    )
//
//	    // Initialize and connect to the server
//	    ctx := context.Background()
//	    if err := client.Initialize(ctx); err != nil {
//	        // Handle error
//	    }
//	    defer client.Close()
//
//	    // Use client capabilities...
//	}
//
// # Creating a Server
//
// To create a server that implements the MCP protocol:
//
//	import (
//	    "context"
//	    "github.com/ajitpratap0/mcp-sdk-go"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
//	)
//
//	func main() {
//	    // Create providers for the server's capabilities
//	    toolsProvider := mcp.NewBaseToolsProvider()
//	    resourcesProvider := mcp.NewBaseResourcesProvider()
//
//	    // Add a tool
//	    toolsProvider.AddTool("hello", "Hello Tool", "Says hello",
//	        func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
//	            name, _ := params["name"].(string)
//	            if name == "" {
//	                name = "World"
//	            }
//	            return map[string]string{"greeting": "Hello, " + name + "!"}, nil
//	        })
//
//	    // Create and configure the server
//	    server := mcp.NewServer(
//	        mcp.NewStdioTransport(),
//	        mcp.WithServerName("MyServer"),
//	        mcp.WithServerVersion("1.0.0"),
//	        mcp.WithServerDescription("My MCP Server"),
//	        mcp.WithToolsProvider(toolsProvider),
//	        mcp.WithResourcesProvider(resourcesProvider),
//	    )
//
//	    // Start the server (blocks until context is canceled)
//	    ctx := context.Background()
//	    if err := server.Start(ctx); err != nil {
//	        // Handle error
//	    }
//	}
//
// # Examples
//
// The SDK includes several examples in the examples directory:
//
//   - simple-client: A basic client that connects to a server
//   - simple-server: A basic server with tool and resource capabilities
//   - http-server: A server that uses HTTP transport
//   - completion: A server that provides completion capabilities
package mcp
