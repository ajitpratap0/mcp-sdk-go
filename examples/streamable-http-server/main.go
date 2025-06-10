package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/examples/shared"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

const (
	serverAddr = "localhost:8081" // Using port 8081 to avoid conflicts
)

func main() {
	// Create an HTTP handler for the streamable HTTP server
	handler := http.NewServeMux()

	// Register the MCP endpoint handler
	mcpHandler := server.NewStreamableHTTPHandler()

	// Set allowed origins for security (prevent DNS rebinding attacks)
	allowedOrigins := []string{"http://localhost", "http://127.0.0.1"}
	mcpHandler.SetAllowedOrigins(allowedOrigins)

	handler.Handle("/mcp", mcpHandler)

	// Create streamable HTTP transport using modern config approach
	endpoint := fmt.Sprintf("http://%s/mcp", serverAddr)
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = endpoint
	config.Performance.RequestTimeout = 2 * time.Minute

	baseTransport, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Cast to StreamableHTTPTransport to access specific methods
	streamableTransport, ok := baseTransport.(*transport.StreamableHTTPTransport)
	if !ok {
		log.Fatal("Expected StreamableHTTPTransport")
	}
	streamableTransport.SetAllowedOrigins(allowedOrigins)

	// Set the transport for the handler
	mcpHandler.SetTransport(streamableTransport)

	// Create server using shared providers with the transport
	toolsProvider := shared.CreateToolsProvider()
	resourcesProvider := shared.CreateResourcesProvider()
	promptsProvider := shared.CreatePromptsProvider()

	s := server.New(streamableTransport,
		server.WithName("StreamableHTTPServer"),
		server.WithVersion("1.0.0"),
		server.WithDescription("An example MCP server with HTTP+SSE transport"),
		server.WithHomepage("https://github.com/ajitpratap0/mcp-sdk-go"),
		server.WithToolsProvider(toolsProvider),
		server.WithResourcesProvider(resourcesProvider),
		server.WithPromptsProvider(promptsProvider),
		server.WithCapability("resourceSubscriptions", true),
		server.WithCapability("logging", true),
		server.WithCapability("sampling", true),
	)

	// Create the HTTP server
	httpServer := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Add timeout to prevent Slowloris attacks
	}

	// Create a context that is canceled on interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()

		// Gracefully shut down the HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	// Start the server (non-blocking)
	go func() {
		log.Printf("Starting MCP server on http://%s/mcp...", serverAddr)
		log.Printf("Server supports session management and origin validation")
		log.Printf("Allowed origins: http://localhost, http://127.0.0.1")
		if err := s.Start(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Start HTTP server (blocking)
	log.Printf("HTTP server listening on http://%s", serverAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

	log.Println("Server stopped")
}
