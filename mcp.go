// Package mcp provides a Golang implementation of the Model Context Protocol (2025-03-26)
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

	// NewStdioTransport creates a new stdio transport
	NewStdioTransport = transport.NewStdioTransport

	// NewHTTPTransport creates a new HTTP transport
	NewHTTPTransport = transport.NewHTTPTransport

	// NewStreamableHTTPTransport creates a new Streamable HTTP transport
	NewStreamableHTTPTransport = transport.NewStreamableHTTPTransport
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
	WithClientName           = client.WithName
	WithClientVersion        = client.WithVersion
	WithClientCapability     = client.WithCapability
	WithClientFeatureOptions = client.WithFeatureOptions
)

// Server options
var (
	WithServerName           = server.WithName
	WithServerVersion        = server.WithVersion
	WithServerDescription    = server.WithDescription
	WithServerHomepage       = server.WithHomepage
	WithServerCapability     = server.WithCapability
	WithServerFeatureOptions = server.WithFeatureOptions
	WithToolsProvider        = server.WithToolsProvider
	WithResourcesProvider    = server.WithResourcesProvider
	WithPromptsProvider      = server.WithPromptsProvider
	WithCompletionProvider   = server.WithCompletionProvider
	WithRootsProvider        = server.WithRootsProvider
	WithLogger               = server.WithLogger
)

// Provider creation
var (
	NewBaseToolsProvider     = server.NewBaseToolsProvider
	NewBaseResourcesProvider = server.NewBaseResourcesProvider
	NewBasePromptsProvider   = server.NewBasePromptsProvider
)

// Transport options
var (
	WithRequestTimeout = transport.WithRequestTimeout
)
