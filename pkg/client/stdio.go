package client

import (
	"os"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// NewStdioClient creates a new client that communicates over stdin/stdout.
// According to the MCP specification, clients should support stdio whenever possible.
func NewStdioClient(options ...ClientOption) *ClientConfig {
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	t, err := transport.NewTransport(config)
	if err != nil {
		panic(err) // This should not happen with default config
	}

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
func NewStdioClientWithStreams(reader, writer *os.File, options ...ClientOption) *ClientConfig {
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.StdioReader = reader
	config.StdioWriter = writer
	t, err := transport.NewTransport(config)
	if err != nil {
		panic(err) // This should not happen with default config
	}

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
