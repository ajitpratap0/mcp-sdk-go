# Go Model Context Protocol SDK

A professional, high-performance implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) specification (2025-03-26) in Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/ajitpratap0/mcp-sdk-go.svg)](https://pkg.go.dev/github.com/ajitpratap0/mcp-sdk-go)
[![GitHub license](https://img.shields.io/github/license/ajitpratap0/mcp-sdk-go)](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/LICENSE)

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
  - [Creating an MCP Client](#creating-an-mcp-client)
  - [Creating an MCP Server](#creating-an-mcp-server)
- [Examples](#examples)
- [Project Structure](#project-structure)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Overview

This SDK provides a comprehensive implementation of the MCP specification, allowing you to:

- Create MCP clients that connect to MCP servers
- Build MCP servers that expose resources, tools, and prompts
- Support the full range of MCP features (resources, tools, prompts, sampling, etc.)
- Implement custom transport mechanisms (stdio and HTTP/SSE are provided)

The implementation prioritizes:

- **Simplicity**: Clean, idiomatic Go API with minimal dependencies
- **Performance**: Zero-copy approach where possible, efficient message handling
- **Scalability**: Support for high-throughput scenarios
- **Conformance**: Full implementation of the MCP 2025-03-26 specification
- **Robustness**: Comprehensive validation, error handling, and recovery mechanisms

## Features

### Core Protocol
- Full JSON-RPC 2.0 implementation
- Complete lifecycle management
- Support for capabilities negotiation
- Request/response and notification handling

### Transports
- stdio transport (required by spec) with proper line buffering
- HTTP with Server-Sent Events (SSE) transport
- Streamable HTTP transport for enhanced reliability
- Extensible transport interface for custom implementations

### Server Features
- Resources implementation
- Tools implementation
- Prompts implementation
- Completion support
- Roots support
- Logging

### Client Features
- Sampling support
- Resource access with subscription support
- Tool invocation with context
- Prompt usage
- Comprehensive pagination utilities
- Automatic multi-page result collection

## Requirements

- Go 1.18 or later
- No external dependencies beyond the Go standard library

## Installation

To install the SDK, use the standard Go package manager:

```bash
go get github.com/ajitpratap0/mcp-sdk-go
```

## Quick Start

### Creating an MCP Client

```go
import (
    "context"
    "log"
    
    "github.com/ajitpratap0/mcp-sdk-go/pkg/client"
    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func main() {
    // Create a client with stdio transport (recommended by MCP spec)
    c := client.NewStdioClient(
        client.WithName("MyClient"),
        client.WithVersion("1.0.0"),
        client.WithCapability(protocol.CapabilitySampling, true),
    )
    
    // Initialize the client and start the transport in the background
    ctx := context.Background()
    if err := c.InitializeAndStart(ctx); err != nil {
        log.Fatalf("Failed to initialize: %v", err)
    }
    defer c.Close()
    
    // Use client functionality
    if c.HasCapability(protocol.CapabilityTools) {
        // Use automatic pagination to get all tools
        allTools, err := c.ListAllTools(ctx, "")
        if err != nil {
            log.Printf("Error listing tools: %v", err)
        } else {
            for _, tool := range allTools {
                log.Printf("Tool: %s - %s", tool.Name, tool.Description)
            }
        }
        
        // Or use manual pagination if needed
        tools, pagResult, err := c.ListTools(ctx, "", nil) // uses default pagination
        if err != nil {
            log.Printf("Error listing tools: %v", err)
        } else {
            log.Printf("Got %d tools, more available: %v", len(tools), pagResult.HasMore)
        }
    }
}
```

### Creating an MCP Server

```go
import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
    "github.com/ajitpratap0/mcp-sdk-go/pkg/server"
    "github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

func main() {
    // Create transport
    t := transport.NewStdioTransport()
    
    // Create providers
    toolsProvider := server.NewBaseToolsProvider()
    resourcesProvider := server.NewBaseResourcesProvider()
    promptsProvider := server.NewBasePromptsProvider()
    
    // Configure providers (add tools, resources, prompts)
    // ...
    
    // Create server with providers
    s := server.New(t,
        server.WithName("MyServer"),
        server.WithVersion("1.0.0"),
        server.WithDescription("Example MCP server"),
        server.WithToolsProvider(toolsProvider),
        server.WithResourcesProvider(resourcesProvider),
        server.WithPromptsProvider(promptsProvider),
    )
    
    // Set up graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigChan
        cancel()
    }()
    
    // Start server (blocking)
    log.Println("Starting MCP server...")
    if err := s.Start(ctx); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Examples

Check the `examples` directory for complete working examples:

- **simple-server**: A basic MCP server implementation with resources, tools, and prompts
- **simple-client**: A client that connects to an MCP server using stdio transport
- **stdio-client**: A client that demonstrates the recommended stdio transport implementation
- **streamable-http-client**: A client that uses HTTP+SSE for transport
- **streamable-http-server**: A server that exposes an HTTP+SSE endpoint
- **pagination-example**: Demonstrates both manual and automatic pagination

To run an example:

```bash
cd examples/simple-server
go run main.go
```

## Project Structure

- `pkg/protocol`: Core protocol types and definitions
- `pkg/transport`: Transport layer implementations (stdio, HTTP, streamable HTTP)
- `pkg/client`: Client implementation for connecting to MCP servers
- `pkg/pagination`: Utilities for handling paginated operations
- `pkg/server`: Server implementation for building MCP servers
- `pkg/utils`: Utility functions and helpers
- `examples`: Complete example applications

## Documentation

- [Go Reference Documentation](https://pkg.go.dev/github.com/ajitpratap0/mcp-sdk-go)
- [MCP Specification](https://modelcontextprotocol.io/)
- [Example Code](https://github.com/ajitpratap0/mcp-sdk-go/tree/main/examples)

## Contributing

Contributions are welcome! Please read our [Contributing Guide](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

### Development Setup

1. Clone the repository
   ```bash
   git clone https://github.com/ajitpratap0/mcp-sdk-go.git
   cd mcp-sdk-go
   ```

2. Install dependencies
   ```bash
   go mod tidy
   ```

3. Run tests
   ```bash
   go test ./...
   ```

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/LICENSE) file for details.

---

Copyright (c) 2025 Go Model Context Protocol SDK Contributors
