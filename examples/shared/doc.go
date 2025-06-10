// Package shared provides reusable components for MCP server examples.
//
// This package eliminates code duplication between different MCP server examples
// by providing common implementations of tools, resources, prompts, and server
// factory functions.
//
// # Components
//
// The package includes:
//   - Common tool implementations (hello, currentTime, countToTen)
//   - Shared resource definitions and handlers
//   - Standard prompt templates
//   - Server factory functions for different transport types
//
// # Usage
//
// Create a simple stdio server:
//
//	server, err := shared.CreateStdioServer("MyServer", "1.0.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	ctx := context.Background()
//	if err := server.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// Create an HTTP server with custom configuration:
//
//	config := shared.DefaultServerConfig()
//	config.Name = "My HTTP Server"
//	config.TransportType = shared.TransportStreamableHTTP
//	config.Endpoint = "http://localhost:8080/mcp"
//	config.AllowedOrigins = []string{"http://localhost", "http://127.0.0.1"}
//
//	server, err := shared.CreateServerWithConfig(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Use individual providers in custom server setups:
//
//	toolsProvider := shared.CreateToolsProvider()
//	resourcesProvider := shared.CreateResourcesProvider()
//	promptsProvider := shared.CreatePromptsProvider()
//
//	// Use providers with custom server configuration...
//
// # Transport Types
//
// The package supports multiple transport types:
//   - TransportStdio: Standard input/output (for CLI tools)
//   - TransportHTTP: Basic HTTP transport with Server-Sent Events
//   - TransportStreamableHTTP: Advanced HTTP with streaming and session management
//   - TransportReliableHTTP: HTTP with message reliability and retry mechanisms
package shared
