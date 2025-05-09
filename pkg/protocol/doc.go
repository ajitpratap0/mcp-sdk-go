// Package protocol defines the core types and structures used in the MCP protocol.
//
// The Model Context Protocol (MCP) is a JSON-RPC based communication protocol that
// enables AI models to interact with their environment through a standardized interface.
// This package contains the Go type definitions for all protocol messages and structures.
//
// # Package Organization
//
// The protocol package is organized into several files:
//
//   - mcp.go: Contains constants for method names, capabilities, log levels, etc.
//   - types.go: Contains type definitions for protocol messages (requests, responses, notifications)
//   - protocol.go: Contains type definitions for domain objects (Tool, Resource, Prompt, etc.)
//
// # Capability Types
//
// MCP supports several capabilities that can be enabled or disabled:
//
//   - tools: Allows clients to invoke operations on the server
//   - resources: Allows clients to access structured data from the server
//   - resourceSubscriptions: Allows clients to subscribe to resource changes
//   - prompts: Allows clients to use predefined prompt templates
//   - complete: Allows clients to generate text completions
//   - roots: Allows clients to discover available resources
//   - sampling: Allows servers to stream intermediate generation results
//   - logging: Allows clients and servers to exchange log messages
//   - pagination: Allows paginated results for large data sets
//
// # Message Flow
//
// The protocol follows a standard flow:
//
//  1. Client connects to server and sends an initialize request
//  2. Server responds with capabilities and server info
//  3. Client sends an initialized notification
//  4. Client and server exchange requests and responses based on capabilities
//  5. Client disconnects when done
//
// # Error Handling
//
// The protocol defines standard error codes and structures for error reporting.
// All errors include a code, message, and optional data for additional context.
//
// # Example Messages
//
// Initialize request:
//
//	{
//	    "jsonrpc": "2.0",
//	    "id": 1,
//	    "method": "initialize",
//	    "params": {
//	        "protocolVersion": "2025-03-26",
//	        "name": "ExampleClient",
//	        "version": "1.0.0",
//	        "capabilities": {
//	            "sampling": true
//	        }
//	    }
//	}
//
// Initialize response:
//
//	{
//	    "jsonrpc": "2.0",
//	    "id": 1,
//	    "result": {
//	        "protocolVersion": "2025-03-26",
//	        "name": "ExampleServer",
//	        "version": "1.0.0",
//	        "capabilities": {
//	            "tools": true,
//	            "resources": true
//	        },
//	        "serverInfo": {
//	            "name": "ExampleServer",
//	            "version": "1.0.0",
//	            "description": "An example MCP server"
//	        }
//	    }
//	}
package protocol

// Note: This package has duplicate type declarations due to being in development.
// The canonical definitions are in protocol.go, and other files with duplicates
// (such as mcp.go) need to be refactored to use the canonical types.
