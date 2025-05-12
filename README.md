# Go Model Context Protocol SDK

A professional, high-performance implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) specification (2025-03-26) in Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/ajitpratap0/mcp-sdk-go.svg)](https://pkg.go.dev/github.com/ajitpratap0/mcp-sdk-go)
[![GitHub license](https://img.shields.io/github/license/ajitpratap0/mcp-sdk-go)](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/LICENSE)
[![codecov](https://codecov.io/gh/ajitpratap0/mcp-sdk-go/branch/main/graph/badge.svg)](https://codecov.io/gh/ajitpratap0/mcp-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/ajitpratap0/mcp-sdk-go)](https://goreportcard.com/report/github.com/ajitpratap0/mcp-sdk-go)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ajitpratap0/mcp-sdk-go)](https://go.dev/)
[![CI Status](https://github.com/ajitpratap0/mcp-sdk-go/workflows/CI/badge.svg)](https://github.com/ajitpratap0/mcp-sdk-go/actions)
[![Release](https://img.shields.io/github/release/ajitpratap0/mcp-sdk-go.svg)](https://github.com/ajitpratap0/mcp-sdk-go/releases)

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

- stdio transport (required by spec) with proper line buffering and full JSON-RPC support
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

- Go 1.24.2 or later
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

[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://makeapullrequest.com)
[![Contributors](https://img.shields.io/github/contributors/ajitpratap0/mcp-sdk-go)](https://github.com/ajitpratap0/mcp-sdk-go/graphs/contributors)
[![Last Commit](https://img.shields.io/github/last-commit/ajitpratap0/mcp-sdk-go)](https://github.com/ajitpratap0/mcp-sdk-go/commits/main)
[![CI Status](https://github.com/ajitpratap0/mcp-sdk-go/workflows/CI/badge.svg)](https://github.com/ajitpratap0/mcp-sdk-go/actions?query=workflow%3ACI)

Contributions are welcome! Please read our [Contributing Guide](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

Information about our code quality checks, CI pipeline, and development tools can be found in [CHECKS.md](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/CHECKS.md).

The MCP SDK Go project welcomes contributions from everyone! Whether you're fixing a bug, improving documentation, or implementing a new feature, your contributions make this project better for everyone.

### Ways to Contribute

- **Code Contributions**: Add features, fix bugs, or improve performance
- **Documentation**: Improve or correct the documentation
- **Testing**: Add test cases or improve existing tests
- **Issue Triage**: Help process issues, reproduce bugs
- **Community Support**: Help answer questions in discussions

### Contribution Workflow

1. **Find an Issue**: Check our [issues page](https://github.com/ajitpratap0/mcp-sdk-go/issues) for "good first issue" or "help wanted" tags
2. **Fork & Clone**: Fork the repository and clone it locally
3. **Branch**: Create a branch for your contribution
4. **Code**: Make your changes following our code style
5. **Test**: Add tests and ensure all tests pass
6. **Submit**: Open a PR with a clear description of your changes

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

4. Run linting

   ```bash
   # Install golangci-lint if not already installed
   # See: https://golangci-lint.run/usage/install/
   golangci-lint run
   ```

### Coding Guidelines

- Follow Go best practices and idiomatic code patterns
- Maintain backward compatibility unless explicitly noted
- Add proper error handling and documentation
- Keep code simple and maintainable
- Include new tests for new functionality

### Pull Request Process

1. Ensure your PR addresses a specific issue or clearly describes the improvement
2. Update documentation relevant to your changes
3. Make sure all tests pass and code lints successfully
4. Request a review from maintainers
5. Address review feedback promptly

### Reporting Bugs

When reporting a bug, please include:

- A clear, descriptive title
- Detailed steps to reproduce the issue
- Expected vs. actual behavior
- Version information (Go version, OS, etc.)
- Any relevant logs or error messages

For more detailed information, please read our complete [Contributing Guide](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/CONTRIBUTING.md).

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/ajitpratap0/mcp-sdk-go/blob/main/LICENSE) file for details.

---

Copyright (c) 2025 Go Model Context Protocol SDK Contributors
