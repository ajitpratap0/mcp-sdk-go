// Package client provides the client-side implementation of the MCP protocol.
//
// The client package enables applications to connect to MCP servers and use their
// capabilities. It includes:
//
//   - Client interface: Defines the operations that can be performed by MCP clients
//   - ClientConfig: Implements the Client interface with configuration options
//   - Options: Various configuration options for client behavior
//
// # Client Capabilities
//
// An MCP client can use several capabilities provided by a server:
//
//   - Tools: Invoke operations on the server
//   - Resources: Access structured data from the server
//   - Prompts: Use predefined prompt templates
//   - Completion: Generate text completions
//   - Roots: Discover available resources
//
// # Creating a Client
//
// To create an MCP client and connect to a server:
//
//	import (
//	    "context"
//	    "fmt"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/client"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
//	)
//
//	func main() {
//	    // Create a transport for the client
//	    t := transport.NewStdioTransport()
//
//	    // Create and configure the client
//	    c := client.New(t,
//	        client.WithName("ExampleClient"),
//	        client.WithVersion("1.0.0"),
//	        client.WithCapability(protocol.CapabilitySampling, true),
//	    )
//
//	    // Initialize and connect to the server
//	    ctx := context.Background()
//	    if err := c.Initialize(ctx); err != nil {
//	        // Handle error
//	        return
//	    }
//
//	    // Use client capabilities
//	    if c.HasCapability(protocol.CapabilityTools) {
//	        tools, _, err := c.ListTools(ctx, "", nil)
//	        if err != nil {
//	            // Handle error
//	            return
//	        }
//
//	        fmt.Printf("Found %d tools:\n", len(tools))
//	        for _, tool := range tools {
//	            fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
//	        }
//	    }
//
//	    // Close the client when done
//	    c.Close()
//	}
//
// # Progress and Streaming
//
// The client supports progress reporting and streaming for long-running operations.
// You can register callbacks to handle these events:
//
//	// Register a callback for sampling events
//	c.SetSamplingCallback(func(event protocol.SamplingEvent) {
//	    fmt.Printf("Sampling event: %s\n", event.Type)
//	})
//
//	// Register a callback for resource changes
//	c.SetResourceChangedCallback(func(id string, data interface{}) {
//	    fmt.Printf("Resource changed: %s\n", id)
//	})
package client
