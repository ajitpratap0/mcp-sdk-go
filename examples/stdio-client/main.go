package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func main() {
	// Create a context that can be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Create a new client with stdio transport
	// This uses the helper function which creates the transport for us
	c := client.NewStdioClient(
		client.WithName("StdioExampleClient"),
		client.WithVersion("1.0.0"),
		client.WithCapability(protocol.CapabilitySampling, true),
		client.WithCapability(protocol.CapabilityLogging, true),
	)

	// Initialize the client and start the transport in the background
	log.Println("Connecting to MCP server...")
	if err := c.InitializeAndStart(ctx); err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer c.Close()

	// Output server info
	serverInfo := c.ServerInfo()
	log.Printf("Connected to server: %s %s", serverInfo.Name, serverInfo.Version)

	// Log available capabilities
	log.Println("Checking server capabilities...")
	checkCapabilities(c)

	// Example: ping the server every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := c.Ping(ctx)
				if err != nil {
					log.Printf("Ping error: %v", err)
					continue
				}
				log.Printf("Ping successful, timestamp: %d", result.Timestamp)
			}
		}
	}()

	// If server supports logging, set the log level
	if c.HasCapability(protocol.CapabilityLogging) {
		if err := c.SetLogLevel(ctx, protocol.LogLevelInfo); err != nil {
			log.Printf("Failed to set log level: %v", err)
		} else {
			log.Println("Log level set to INFO")
		}
	}

	// Demo: Sample request if server supports
	if c.HasCapability(protocol.CapabilitySampling) {
		log.Println("Sending a sample request...")

		// Create sample parameters
		params := &protocol.SampleParams{
			Messages: []protocol.Message{
				{
					Role:    "user",
					Content: "Hello, this is a test message from the stdio client example.",
				},
			},
			SystemPrompt: "You are a helpful assistant.",
			Stream:       true, // Request streaming response
		}

		// Process streaming results
		err := c.Sample(ctx, params, func(result *protocol.SampleResult) error {
			if result != nil {
				fmt.Println("Received content:", result.Content)
			}
			return nil
		})

		if err != nil {
			log.Printf("Sample error: %v", err)
		} else {
			log.Println("Sample completed successfully")
		}
	}

	// Wait for the context to be canceled (by Ctrl+C)
	<-ctx.Done()
	log.Println("Client stopped")
}

// checkCapabilities checks and logs the server's capabilities
func checkCapabilities(c client.Client) {
	capabilities := []struct {
		capability protocol.CapabilityType
		name       string
	}{
		{protocol.CapabilityTools, "Tools"},
		{protocol.CapabilityResources, "Resources"},
		{protocol.CapabilityResourceSubscriptions, "Resource Subscriptions"},
		{protocol.CapabilityPrompts, "Prompts"},
		{protocol.CapabilityComplete, "Complete"},
		{protocol.CapabilityRoots, "Roots"},
		{protocol.CapabilitySampling, "Sampling"},
		{protocol.CapabilityLogging, "Logging"},
		{protocol.CapabilityPagination, "Pagination"},
	}

	for _, cap := range capabilities {
		if c.HasCapability(cap.capability) {
			log.Printf("Server supports %s", cap.name)
		} else {
			log.Printf("Server does NOT support %s", cap.name)
		}
	}
}
