// Package server implements the server-side of the Model Context Protocol (MCP).
//
// The server package provides the necessary components to create an MCP server that
// can expose capabilities to MCP clients. It includes:
//
//   - Server: The main implementation that manages connections and handles client requests
//   - Providers: Interfaces for implementing different capabilities (tools, resources, etc.)
//   - HTTP handlers: Integration with standard HTTP servers
//
// # Server Capabilities
//
// An MCP server can implement several capabilities:
//
//   - Tools: Allow clients to invoke operations on the server
//   - Resources: Allow clients to access structured data from the server
//   - Prompts: Allow clients to use predefined prompt templates
//   - Completion: Allow clients to generate text completions
//   - Roots: Allow clients to discover available resources
//
// # Creating a Server
//
// To create an MCP server with basic capabilities:
//
//	import (
//	    "context"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/server"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
//	)
//
//	func main() {
//	    // Create a transport for the server
//	    t := transport.NewStdioTransport()
//
//	    // Create providers for server capabilities
//	    toolsProvider := server.NewBaseToolsProvider()
//
//	    // Add a sample tool
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
//	    srv := server.New(t,
//	        server.WithName("ExampleServer"),
//	        server.WithVersion("1.0.0"),
//	        server.WithDescription("An example MCP server"),
//	        server.WithToolsProvider(toolsProvider),
//	    )
//
//	    // Start the server (blocks until context is canceled)
//	    ctx := context.Background()
//	    if err := srv.Start(ctx); err != nil {
//	        // Handle error
//	    }
//	}
//
// # Provider Interfaces
//
// The server uses providers to implement different capabilities:
//
//   - ToolsProvider: Manages tools that clients can invoke
//   - ResourcesProvider: Manages resources that clients can access
//   - PromptsProvider: Manages prompt templates that clients can use
//   - CompletionProvider: Generates text completions for clients
//   - RootsProvider: Manages root resources for discovery
//
// Each provider interface has a base implementation that can be used as a starting point.
package server
