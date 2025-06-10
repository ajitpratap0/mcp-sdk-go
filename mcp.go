// Package mcp provides a comprehensive implementation of the Model Context Protocol (MCP)
// specification (2025-03-26) in Go. MCP is a protocol that standardizes communication
// between AI models and client applications, enabling rich context sharing and
// structured interactions.
package mcp

import (
	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// Version represents the current version of the SDK
const Version = "1.0.0"

// These exports provide direct access to the core SDK components
var (
	// NewClient creates a new MCP client
	NewClient = client.New

	// NewServer creates a new MCP server
	NewServer = server.New

	// NewTransport creates a new transport using modern config-driven approach
	NewTransport = transport.NewTransport

	// DefaultTransportConfig creates a default transport configuration
	DefaultTransportConfig = transport.DefaultTransportConfig
)

// Protocol constants for capabilities
const (
	CapabilityTools                 = protocol.CapabilityTools
	CapabilityResources             = protocol.CapabilityResources
	CapabilityResourceSubscriptions = protocol.CapabilityResourceSubscriptions
	CapabilityPrompts               = protocol.CapabilityPrompts
	CapabilityComplete              = protocol.CapabilityComplete
	CapabilityRoots                 = protocol.CapabilityRoots
	CapabilitySampling              = protocol.CapabilitySampling
	CapabilityLogging               = protocol.CapabilityLogging
	CapabilityPagination            = protocol.CapabilityPagination
)

// Client options
var (
	// WithClientName sets the client name for identification
	WithClientName = client.WithName

	// WithClientVersion sets the client version
	WithClientVersion = client.WithVersion

	// WithClientCapability enables or disables a specific capability for the client
	WithClientCapability = client.WithCapability

	// WithClientFeatureOptions allows setting additional feature options for capabilities
	WithClientFeatureOptions = client.WithFeatureOptions
)

// Server options
var (
	// WithServerName sets the server name for identification
	WithServerName = server.WithName

	// WithServerVersion sets the server version
	WithServerVersion = server.WithVersion

	// WithServerDescription sets a human-readable description of the server
	WithServerDescription = server.WithDescription

	// WithServerHomepage sets the URL of the server's homepage
	WithServerHomepage = server.WithHomepage

	// WithServerCapability enables or disables a specific capability for the server
	WithServerCapability = server.WithCapability

	// WithServerFeatureOptions allows setting additional feature options for capabilities
	WithServerFeatureOptions = server.WithFeatureOptions

	// WithToolsProvider registers a tools provider with the server
	WithToolsProvider = server.WithToolsProvider

	// WithResourcesProvider registers a resources provider with the server
	WithResourcesProvider = server.WithResourcesProvider

	// WithPromptsProvider registers a prompts provider with the server
	WithPromptsProvider = server.WithPromptsProvider

	// WithCompletionProvider registers a completion provider with the server
	WithCompletionProvider = server.WithCompletionProvider

	// WithRootsProvider registers a roots provider with the server
	WithRootsProvider = server.WithRootsProvider

	// WithLogger sets the logger for the server
	WithLogger = server.WithLogger
)

// Provider creation
var (
	// NewBaseToolsProvider creates a new base tools provider implementation
	NewBaseToolsProvider = server.NewBaseToolsProvider

	// NewBaseResourcesProvider creates a new base resources provider implementation
	NewBaseResourcesProvider = server.NewBaseResourcesProvider

	// NewBasePromptsProvider creates a new base prompts provider implementation
	NewBasePromptsProvider = server.NewBasePromptsProvider
)

// Transport types
var (
	// TransportTypeStdio represents stdio transport type
	TransportTypeStdio = transport.TransportTypeStdio

	// TransportTypeStreamableHTTP represents streamable HTTP transport type
	TransportTypeStreamableHTTP = transport.TransportTypeStreamableHTTP

	// TransportTypeHTTP represents HTTP transport type
	TransportTypeHTTP = transport.TransportTypeHTTP
)
