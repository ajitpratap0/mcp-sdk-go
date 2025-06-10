package shared

import (
	"fmt"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// ServerConfig defines common server configuration options
type ServerConfig struct {
	Name        string
	Version     string
	Description string
	Homepage    string

	// Transport configuration
	TransportType TransportType
	Endpoint      string // For HTTP transports

	// Advanced options
	RequestTimeout    time.Duration
	AllowedOrigins    []string // For HTTP transports
	EnableReliability bool     // Enable reliable transport wrapper

	// Capabilities
	EnableResourceSubscriptions bool
	EnableLogging               bool
	EnableSampling              bool
}

// TransportType defines the type of transport to use
type TransportType int

const (
	TransportStdio TransportType = iota
	TransportStreamableHTTP
)

// DefaultServerConfig returns a default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Name:        "MCP Example Server",
		Version:     "1.0.0",
		Description: "Example MCP server with shared components",
		Homepage:    "https://github.com/ajitpratap0/mcp-sdk-go",

		TransportType:  TransportStdio,
		RequestTimeout: 2 * time.Minute,

		EnableResourceSubscriptions: true,
		EnableLogging:               true,
		EnableSampling:              false,
	}
}

// CreateServerWithConfig creates a server with the specified configuration
func CreateServerWithConfig(config *ServerConfig) (*server.Server, error) {
	// Create modern transport configuration
	var transportConfig transport.TransportConfig

	switch config.TransportType {
	case TransportStdio:
		transportConfig = transport.DefaultTransportConfig(transport.TransportTypeStdio)
		// Disable reliability for stdio (not needed)
		transportConfig.Features.EnableReliability = false

	case TransportStreamableHTTP:
		if config.Endpoint == "" {
			config.Endpoint = "http://localhost:8081/mcp"
		}
		transportConfig = transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
		transportConfig.Endpoint = config.Endpoint
		transportConfig.Features.EnableReliability = config.EnableReliability
		transportConfig.Performance.RequestTimeout = config.RequestTimeout

		// Configure observability based on server config
		transportConfig.Features.EnableObservability = config.EnableLogging
		if config.EnableLogging {
			transportConfig.Observability.LogLevel = "info"
		}

	default:
		// Default to stdio with modern config
		transportConfig = transport.DefaultTransportConfig(transport.TransportTypeStdio)
		transportConfig.Features.EnableReliability = false
	}

	// Create transport using modern factory
	t, err := transport.NewTransport(transportConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create providers
	toolsProvider := CreateToolsProvider()
	resourcesProvider := CreateResourcesProvider()
	promptsProvider := CreatePromptsProvider()

	// Build server options
	options := []server.ServerOption{
		server.WithName(config.Name),
		server.WithVersion(config.Version),
		server.WithDescription(config.Description),
		server.WithHomepage(config.Homepage),
		server.WithToolsProvider(toolsProvider),
		server.WithResourcesProvider(resourcesProvider),
		server.WithPromptsProvider(promptsProvider),
	}

	// Add capabilities based on configuration
	if config.EnableResourceSubscriptions {
		options = append(options, server.WithCapability(protocol.CapabilityResourceSubscriptions, true))
	}
	if config.EnableLogging {
		options = append(options, server.WithCapability(protocol.CapabilityLogging, true))
	}
	if config.EnableSampling {
		options = append(options, server.WithCapability(protocol.CapabilitySampling, true))
	}

	// Create server
	s := server.New(t, options...)
	return s, nil
}

// CreateStdioServer creates a server with stdio transport (for CLI usage)
func CreateStdioServer(name, version string) (*server.Server, error) {
	config := DefaultServerConfig()
	config.Name = name
	config.Version = version
	config.TransportType = TransportStdio
	return CreateServerWithConfig(config)
}

// CreateHTTPServer creates a server with HTTP transport and standard reliability
func CreateHTTPServer(name, version, endpoint string) (*server.Server, error) {
	config := DefaultServerConfig()
	config.Name = name
	config.Version = version
	config.TransportType = TransportStreamableHTTP
	config.Endpoint = endpoint
	config.EnableReliability = true // Enable by default for production
	config.EnableSampling = true
	return CreateServerWithConfig(config)
}

// CreateProductionHTTPServer creates a server optimized for production use
func CreateProductionHTTPServer(name, version, endpoint string) (*server.Server, error) {
	config := DefaultServerConfig()
	config.Name = name
	config.Version = version
	config.TransportType = TransportStreamableHTTP
	config.Endpoint = endpoint
	config.EnableReliability = true
	config.EnableLogging = true
	config.EnableSampling = true
	config.RequestTimeout = 60 * time.Second // Longer timeout for production
	return CreateServerWithConfig(config)
}

// CreateDevelopmentHTTPServer creates a server optimized for development
func CreateDevelopmentHTTPServer(name, version, endpoint string) (*server.Server, error) {
	config := DefaultServerConfig()
	config.Name = name
	config.Version = version
	config.TransportType = TransportStreamableHTTP
	config.Endpoint = endpoint
	config.EnableReliability = false // Faster failures for development
	config.EnableLogging = true      // Verbose logging for debugging
	config.RequestTimeout = 30 * time.Second
	return CreateServerWithConfig(config)
}
