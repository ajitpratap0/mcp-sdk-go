// Package pkg provides the core components of the Model Context Protocol (MCP) SDK.
//
// The Model Context Protocol is a standardized communication protocol that enables
// AI models to interact with their environment through a well-defined interface.
// This package contains several sub-packages that implement different aspects of the protocol.
//
// # Client Usage
//
// To create a client that connects to an MCP server:
//
//	import (
//	    "context"
//	    "github.com/ajitpratap0/mcp-sdk-go"
//	)
//
//	func main() {
//	    // Create a client with stdio transport
//	    client := mcp.NewClient(
//	        mcp.NewStdioTransport(),
//	        mcp.WithClientName("MyClient"),
//	        mcp.WithClientVersion("1.0.0"),
//	    )
//
//	    // Initialize and connect to the server
//	    ctx := context.Background()
//	    if err := client.InitializeAndStart(ctx); err != nil {
//	        // Handle error
//	    }
//	    defer client.Close()
//
//	    // Use client capabilities...
//	}
//
// # Server Implementation
//
// To create a server that implements the MCP protocol:
//
//	import (
//	    "context"
//	    "github.com/ajitpratap0/mcp-sdk-go"
//	)
//
//	func main() {
//	    // Create providers for the server's capabilities
//	    toolsProvider := mcp.NewBaseToolsProvider()
//	    resourcesProvider := mcp.NewBaseResourcesProvider()
//
//	    // Create and configure the server
//	    server := mcp.NewServer(
//	        mcp.NewStdioTransport(),
//	        mcp.WithServerName("MyServer"),
//	        mcp.WithServerVersion("1.0.0"),
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
// # Sub-packages
//
// The MCP SDK consists of several sub-packages:
//
//   - client: Implements the client-side of the MCP protocol
//   - server: Implements the server-side of the MCP protocol
//   - protocol: Defines the core protocol types and messages
//   - transport: Provides transport mechanisms for communication
//   - pagination: Utilities for handling paginated results
//   - utils: Common utility functions used throughout the SDK
package pkg
