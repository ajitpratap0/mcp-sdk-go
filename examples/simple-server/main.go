package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ajitpratap0/mcp-sdk-go/examples/shared"
)

func main() {
	// Create server using shared factory with stdio transport
	s, err := shared.CreateStdioServer("SimpleExample", "1.0.0")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
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
	}()

	// Start server (blocking)
	log.Println("Starting MCP server...")
	if err := s.Start(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
