// Package transport provides a modern, configuration-driven transport layer for the Model Context Protocol.
//
// This package implements a unified transport system that supports multiple protocols
// with automatic middleware composition and production-ready reliability features.
//
// # Key Features
//
//   - Unified config-driven transport creation with NewTransport(config)
//   - Automatic middleware composition (reliability, observability)
//   - Support for stdio and HTTP/SSE transports
//   - Production-ready reliability (retries, circuit breakers, connection recovery)
//   - Comprehensive observability (metrics, logging, tracing)
//   - Clean separation of concerns with middleware pattern
//
// # Supported Transport Types
//
// StdioTransport:
//   - Uses standard input/output for communication
//   - Required by MCP specification for CLI tools
//   - Simple and reliable for local process communication
//
// StreamableHTTPTransport:
//   - HTTP with Server-Sent Events (SSE) for real-time streaming
//   - Session management and automatic reconnection
//   - Circuit breaker pattern for fault tolerance
//   - Exponential backoff with jitter for reconnection
//   - Origin header validation for security
//
// # Modern Usage Pattern
//
// All transports are now created using the unified configuration approach:
//
//	// Create a stdio transport with middleware
//	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
//	config.Features.EnableReliability = true
//	config.Features.EnableObservability = true
//	t, err := transport.NewTransport(config)
//
//	// Create an HTTP transport with custom settings
//	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
//	config.Endpoint = "https://api.example.com/mcp"
//	config.Reliability.MaxRetries = 5
//	config.Reliability.CircuitBreaker.Enabled = true
//	config.Observability.LogLevel = "debug"
//	t, err := transport.NewTransport(config)
//
//	// Initialize and start
//	if err := t.Initialize(ctx); err != nil {
//	    return err
//	}
//	go t.Start(ctx)
//
// # Middleware System
//
// The transport layer uses a composable middleware system:
//
//   - ReliabilityMiddleware: Retry logic, circuit breaker, error classification
//   - ObservabilityMiddleware: Metrics collection, structured logging, performance tracking
//   - Custom middleware can be added by implementing the Middleware interface
//
// Middleware is automatically applied based on configuration:
//
//	config.Features.EnableReliability = true    // Adds retry and circuit breaker
//	config.Features.EnableObservability = true  // Adds metrics and logging
//
// # Configuration Structure
//
// TransportConfig provides unified configuration for all transport types:
//
//   - Type: Transport type (stdio, streamable_http)
//   - Features: Which middleware to enable
//   - Reliability: Retry settings, circuit breaker configuration
//   - Observability: Logging and metrics settings
//   - Performance: Buffer sizes, timeouts, concurrency limits
//   - Connection: HTTP-specific connection pooling settings
//
// # Handler Types
//
//   - RequestHandler: Processes incoming request messages and returns a response
//   - NotificationHandler: Processes incoming notification messages (one-way)
//   - ProgressHandler: Processes progress notifications for long-running operations
//   - ReceiveHandler: Processes raw incoming message data (streaming transports)
//   - ErrorHandler: Processes transport-level errors
package transport
