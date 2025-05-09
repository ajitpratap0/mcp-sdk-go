// Package transport provides communication mechanisms for the Model Context Protocol.
//
// This package implements various transport layers that can be used by MCP clients
// and servers to communicate with each other. The package includes:
//
//   - Transport: The main interface that all transport implementations must satisfy
//   - BaseTransport: A common implementation that handles most of the transport logic
//   - StdioTransport: Transport over standard input/output streams (recommended for CLI tools)
//   - HTTPTransport: Transport over HTTP/HTTPS connections
//   - StreamableHTTPTransport: Transport over HTTP/HTTPS with streaming support
//
// # Handler Types
//
// The transport layer uses several handler types to process different kinds of messages:
//
//   - RequestHandler: Processes incoming request messages and returns a response
//   - NotificationHandler: Processes incoming notification messages (one-way, no response)
//   - ProgressHandler: Processes progress notifications for long-running operations
//   - ReceiveHandler: Processes raw incoming message data
//   - ErrorHandler: Processes errors that occur during transport operations
//
// # Creating a Transport
//
// To create a transport for use in a client or server:
//
//	// Create a stdio transport (for CLI applications)
//	transport := transport.NewStdioTransport()
//
//	// Or create an HTTP transport (for web applications)
//	transport := transport.NewHTTPTransport("https://example.com/api", nil)
//
//	// Initialize the transport before use
//	if err := transport.Initialize(ctx); err != nil {
//	    // Handle error
//	}
//
//	// Start the transport to begin processing messages
//	go func() {
//	    if err := transport.Start(ctx); err != nil {
//	        // Handle error
//	    }
//	}()
package transport

// ReceiveHandler is called when raw message data is received.
// It processes the raw byte data of an incoming message.
type ReceiveHandler func(data []byte)

// ErrorHandler is called when an error occurs during transport operations.
// It handles errors that occur while sending or receiving messages.
type ErrorHandler func(err error)
