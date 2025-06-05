# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go implementation of the Model Context Protocol (MCP), following the MCP 2025-03-26 specification. The goal is to achieve feature parity with the official TypeScript SDK while leveraging Go's strengths in performance and concurrency.

## Essential Development Commands

### Primary Build Commands (via Makefile)
- `make check` - Run all checks (same as CI)
- `make precommit` - Run pre-commit hooks (run before commits)
- `make test` - Run tests with race detection
- `make lint` - Run golangci-lint
- `make transport-check` - Test transport implementations specifically
- `make install-tools` - Install all development tools
- `make coverage` - Generate test coverage report

### Testing Commands
- `make test` - Run all tests with race detection
- `go test ./pkg/client -v` - Run tests for a specific package
- `go test -run TestName ./pkg/...` - Run a specific test
- `go test -race -cover ./...` - Run tests with race detection and coverage
- `go test -bench=. ./pkg/...` - Run benchmarks

## Architecture Overview

### Core Components

1. **Protocol Layer** (`/pkg/protocol/`)
   - JSON-RPC 2.0 implementation
   - MCP message types and constants
   - Error definitions and codes
   - Protocol versioning

2. **Transport Layer** (`/pkg/transport/`)
   - `Transport` interface - base abstraction
   - `StdioTransport` - Primary transport via stdin/stdout
   - `HTTPTransport` - HTTP with Server-Sent Events
   - `StreamableHTTPTransport` - Advanced HTTP with sessions
   - Transport options and configuration

3. **Client Implementation** (`/pkg/client/`)
   - `Client` struct - main client interface
   - Capability negotiation
   - Request/response handling
   - Subscription management
   - Automatic pagination support

4. **Server Implementation** (`/pkg/server/`)
   - `Server` struct - main server interface
   - Provider interfaces (Tools, Resources, Prompts)
   - Request routing and handling
   - Capability advertisement

5. **Utilities** (`/pkg/utils/`, `/pkg/pagination/`)
   - JSON Schema validation
   - Pagination helpers
   - Common utilities

### Key Design Patterns

1. **Transport Abstraction**
   - All transports implement the `Transport` interface
   - Pluggable transport layer
   - Context-based lifecycle management

2. **Provider Pattern**
   - `ToolProvider` - Implements tool capabilities
   - `ResourceProvider` - Implements resource capabilities
   - `PromptProvider` - Implements prompt capabilities
   - Composable server capabilities

3. **Concurrency Safety**
   - Mutex-protected shared state
   - Goroutine-safe operations
   - Context propagation for cancellation

4. **Error Handling**
   - Protocol-defined error codes
   - Wrapped errors with context
   - Consistent error propagation

## Current Implementation Status

### ✅ Completed Features
- Core JSON-RPC 2.0 protocol
- Basic client and server implementation
- Stdio transport (primary)
- HTTP transport with SSE
- Streamable HTTP transport
- Pagination support
- Basic error handling

### 🚧 In Progress
- Authentication support (OAuth 2.1)
- Advanced client features (sampling callbacks)
- Server middleware support
- Comprehensive test coverage

### ❌ Not Yet Implemented
- WebSocket transport
- Request queuing and rate limiting
- Protocol extensions
- Metrics and observability
- Mock implementations for testing

## Development Guidelines

### Code Style
- Follow standard Go conventions
- Use `gofmt` and `goimports`
- Meaningful variable and function names
- Comprehensive godoc comments for exported items

### Error Handling
```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to send request: %w", err)
}

// Use custom error types for protocol errors
return &protocol.ErrorObject{
    Code:    protocol.InvalidRequest,
    Message: "Invalid request format",
    Data:    additionalInfo,
}
```

### Concurrency Patterns
```go
// Use context for cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Protect shared state with mutexes
t.mu.Lock()
defer t.mu.Unlock()

// Use channels for communication
responseChan := make(chan *protocol.Response, 1)
```

### Testing Approach
- Table-driven tests for comprehensive coverage
- Integration tests for transport implementations
- Race detection enabled by default
- Mock implementations for external dependencies
- Benchmark tests for performance-critical paths

## Common Development Tasks

### Adding a New Transport
1. Create new file in `/pkg/transport/`
2. Implement the `Transport` interface
3. Add transport-specific options
4. Write comprehensive tests
5. Add integration example
6. Update documentation

### Adding Server Capabilities
1. Implement provider interface (e.g., `ToolProvider`)
2. Add registration method (e.g., `WithToolProvider()`)
3. Handle requests in provider methods
4. Write unit tests
5. Add example usage
6. Update server documentation

### Implementing Client Features
1. Add method to `Client` struct
2. Implement request/response handling
3. Add appropriate error handling
4. Write tests with mock transport
5. Add usage example
6. Update client documentation

### Working with Examples
- Examples in `/examples/` demonstrate each feature
- Each example should be self-contained
- Include error handling and logging
- Add comments explaining the flow
- Test with `go test ./examples/tests/...`

## Testing Strategy

### Unit Tests
- Test individual functions and methods
- Use table-driven tests
- Mock external dependencies
- Aim for >80% coverage

### Integration Tests
- Test complete workflows
- Use real transport implementations
- Verify protocol compliance
- Test error scenarios

### Performance Tests
- Benchmark critical paths
- Monitor memory allocations
- Test concurrent operations
- Profile CPU and memory usage

## Security Considerations

### Authentication
- OAuth 2.1 support (in progress)
- Token management and refresh
- Secure credential storage
- Session management

### Transport Security
- TLS configuration for HTTP transports
- Certificate validation
- Secure defaults
- Origin validation for HTTP

### Input Validation
- Validate all incoming requests
- Sanitize user inputs
- Implement rate limiting
- Audit logging for security events

## Performance Guidelines

### Memory Management
- Use sync.Pool for frequent allocations
- Avoid unnecessary copying
- Stream large responses
- Profile memory usage regularly

### Concurrency
- Limit goroutine creation
- Use worker pools for parallel operations
- Avoid lock contention
- Profile with race detector

### Network Optimization
- Connection pooling
- Request batching where appropriate
- Compression support
- Efficient serialization

## Debugging Tips

### Common Issues
1. **Transport connection failures**
   - Check transport initialization
   - Verify endpoint configuration
   - Enable debug logging

2. **Protocol errors**
   - Validate message format
   - Check protocol version
   - Review error responses

3. **Concurrency issues**
   - Run with `-race` flag
   - Check mutex usage
   - Review goroutine lifecycles

### Debugging Tools
```bash
# Enable verbose logging
export MCP_DEBUG=1

# Run with race detector
go test -race ./...

# Generate CPU profile
go test -cpuprofile=cpu.prof -bench=.

# Analyze profile
go tool pprof cpu.prof
```

## Contributing Guidelines

### Before Submitting
1. Run `make precommit`
2. Ensure tests pass with `make test`
3. Update documentation
4. Add tests for new features
5. Follow commit message conventions

### Code Review Checklist
- [ ] Tests included and passing
- [ ] Documentation updated
- [ ] Error handling appropriate
- [ ] No race conditions
- [ ] Performance impact considered
- [ ] Security implications reviewed

## Resources

- [MCP Specification](https://modelcontextprotocol.io/specification/2025-03-26)
- [TypeScript SDK](https://github.com/modelcontextprotocol/typescript-sdk) (reference implementation)
- [Go MCP SDK Issues](https://github.com/ajitpratap0/mcp-sdk-go/issues)
- [MCP Community](https://modelcontextprotocol.io/community)
