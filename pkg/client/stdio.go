package client

import (
	"context"
	"os"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// NewStdioClient creates a new client that communicates over stdin/stdout.
// According to the MCP specification, clients should support stdio whenever possible.
func NewStdioClient(options ...ClientOption) *Client {
	t := transport.NewStdioTransport()

	// Apply default options
	defaultOptions := []ClientOption{
		WithName("go-mcp-client"),
		WithVersion("1.0.0"),
		WithCapability(protocol.CapabilitySampling, true),
		WithCapability(protocol.CapabilityLogging, true),
	}

	// Create the client
	c := New(t, defaultOptions...)

	// Apply user options (overriding defaults)
	for _, option := range options {
		option(c)
	}

	return c
}

// NewStdioClientWithStreams creates a new client that communicates over the provided
// reader/writer streams instead of stdin/stdout.
func NewStdioClientWithStreams(reader, writer *os.File, options ...ClientOption) *Client {
	t := transport.NewStdioTransportWithStreams(reader, writer)

	// Apply default options
	defaultOptions := []ClientOption{
		WithName("go-mcp-client"),
		WithVersion("1.0.0"),
		WithCapability(protocol.CapabilitySampling, true),
		WithCapability(protocol.CapabilityLogging, true),
	}

	// Create the client
	c := New(t, defaultOptions...)

	// Apply user options (overriding defaults)
	for _, option := range options {
		option(c)
	}

	return c
}

// InitializeAndStart initializes the client and starts the transport in the background.
// This is a convenience method for typical stdio client usage.
func (c *Client) InitializeAndStart(ctx context.Context) error {
	// Initialize the client
	if err := c.Initialize(ctx); err != nil {
		return err
	}

	// Start the transport in a goroutine
	go func() {
		err := c.transport.Start(ctx)
		if err != nil && ctx.Err() == nil {
			// Only log if the error is not due to context cancellation
			// We can't use the standard logger here as it might cause circular references
			os.Stderr.WriteString("Error in transport: " + err.Error() + "\n")
		}
	}()

	return nil
}
