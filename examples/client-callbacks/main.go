// Package main demonstrates the MCP client callback functionality.
// This example shows how to register and use sampling and resource change callbacks
// for receiving server-initiated events and notifications.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/logging"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

func main() {
	fmt.Println("🚀 MCP Client Callbacks Example")
	fmt.Println("This example demonstrates sampling and resource change callbacks")

	// Create transport configuration for HTTP+SSE
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"
	config.Features.EnableObservability = true

	// Enable features for better demonstration
	config.Features.EnableReliability = true
	config.Reliability.MaxRetries = 3

	// Create transport
	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create logger for better output
	logger := logging.New(nil, logging.NewTextFormatter()).WithFields(
		logging.String("component", "client-callbacks-example"),
	)
	logger.SetLevel(logging.InfoLevel)

	// Create client with callbacks
	mcpClient := client.New(t,
		client.WithName("callback-demo-client"),
		client.WithVersion("1.0.0"),
		client.WithCapability(protocol.CapabilitySampling, true),
		client.WithCapability(protocol.CapabilityResourceSubscriptions, true),
		client.WithLogger(logger),
	)

	// Configure sampling callback - called when server requests sampling
	mcpClient.SetSamplingCallback(func(event protocol.SamplingEvent) {
		fmt.Printf("\n🔍 SAMPLING EVENT RECEIVED:\n")
		fmt.Printf("   Type: %s\n", event.Type)
		fmt.Printf("   Token ID: %s\n", event.TokenID)

		// Process the sampling data
		if sampleParams, ok := event.Data.(protocol.SampleParams); ok {
			fmt.Printf("   Messages: %d\n", len(sampleParams.Messages))
			fmt.Printf("   Request ID: %s\n", sampleParams.RequestID)
			fmt.Printf("   Max Tokens: %d\n", sampleParams.MaxTokens)

			// Show first message if available
			if len(sampleParams.Messages) > 0 {
				fmt.Printf("   First Message: %s\n", sampleParams.Messages[0].Content[:min(100, len(sampleParams.Messages[0].Content))])
			}
		}

		// In a real implementation, you might:
		// - Process the sampling request
		// - Return results through some mechanism
		// - Log sampling statistics
		// - Update UI with sampling progress

		fmt.Printf("   ✅ Sampling event processed\n")
	})

	// Configure resource change callback - called when subscribed resources change
	mcpClient.SetResourceChangedCallback(func(uri string, data interface{}) {
		fmt.Printf("\n📂 RESOURCE CHANGE NOTIFICATION:\n")
		fmt.Printf("   URI: %s\n", uri)

		switch changeData := data.(type) {
		case protocol.ResourcesChangedParams:
			fmt.Printf("   Type: Resources Changed\n")
			if len(changeData.Added) > 0 {
				fmt.Printf("   Added: %d resources\n", len(changeData.Added))
				for _, resource := range changeData.Added {
					fmt.Printf("     + %s (%s)\n", resource.Name, resource.URI)
				}
			}
			if len(changeData.Modified) > 0 {
				fmt.Printf("   Modified: %d resources\n", len(changeData.Modified))
				for _, resource := range changeData.Modified {
					fmt.Printf("     ~ %s (%s)\n", resource.Name, resource.URI)
				}
			}
			if len(changeData.Removed) > 0 {
				fmt.Printf("   Removed: %d resources\n", len(changeData.Removed))
				for _, resourceURI := range changeData.Removed {
					fmt.Printf("     - %s\n", resourceURI)
				}
			}

		case protocol.ResourceUpdatedParams:
			fmt.Printf("   Type: Resource Updated\n")
			fmt.Printf("   Deleted: %t\n", changeData.Deleted)
			if !changeData.Deleted {
				fmt.Printf("   Content Type: %s\n", changeData.Contents.Type)
				fmt.Printf("   Content Length: %d bytes\n", len(changeData.Contents.Content))
			}

		default:
			fmt.Printf("   Type: Unknown (%T)\n", data)
			if jsonData, err := json.MarshalIndent(data, "     ", "  "); err == nil {
				fmt.Printf("   Data: %s\n", string(jsonData))
			}
		}

		// In a real implementation, you might:
		// - Update local cache of resources
		// - Refresh UI displays
		// - Trigger dependent operations
		// - Log resource changes for audit

		fmt.Printf("   ✅ Resource change processed\n")
	})

	// Initialize and start the client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("\n🔌 Connecting to MCP server at %s...\n", config.Endpoint)

	err = mcpClient.InitializeAndStart(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize and start client: %v", err)
	}
	defer mcpClient.Close()

	fmt.Printf("✅ Connected successfully!\n")
	fmt.Printf("📋 Server capabilities: %v\n", mcpClient.Capabilities())

	if serverInfo := mcpClient.ServerInfo(); serverInfo != nil {
		fmt.Printf("🖥️  Server: %s v%s\n", serverInfo.Name, serverInfo.Version)
	}

	// Demonstrate resource subscription (to trigger resource change callbacks)
	if mcpClient.HasCapability(protocol.CapabilityResourceSubscriptions) {
		fmt.Printf("\n📂 Subscribing to resource changes...\n")

		// Subscribe to some example resources
		exampleResources := []string{
			"file:///tmp/example.txt",
			"file:///project/src/",
			"db://localhost/users",
		}

		for _, resourceURI := range exampleResources {
			err := mcpClient.SubscribeResource(ctx, resourceURI, false)
			if err != nil {
				fmt.Printf("   ⚠️  Failed to subscribe to %s: %v\n", resourceURI, err)
			} else {
				fmt.Printf("   ✅ Subscribed to %s\n", resourceURI)
			}
		}
	} else {
		fmt.Printf("   ⚠️  Server does not support resource subscriptions\n")
	}

	// Example: List available tools (this may trigger callbacks in some implementations)
	if mcpClient.HasCapability(protocol.CapabilityTools) {
		fmt.Printf("\n🔧 Listing available tools...\n")
		tools, err := mcpClient.ListAllTools(ctx, "")
		if err != nil {
			fmt.Printf("   ⚠️  Failed to list tools: %v\n", err)
		} else {
			fmt.Printf("   Found %d tools\n", len(tools))
			for i, tool := range tools {
				if i >= 3 { // Limit output
					fmt.Printf("   ... and %d more\n", len(tools)-3)
					break
				}
				fmt.Printf("   - %s: %s\n", tool.Name, tool.Description)
			}
		}
	}

	// Example: List available resources
	if mcpClient.HasCapability(protocol.CapabilityResources) {
		fmt.Printf("\n📁 Listing available resources...\n")
		resources, templates, err := mcpClient.ListAllResources(ctx, "", false)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to list resources: %v\n", err)
		} else {
			fmt.Printf("   Found %d resources and %d templates\n", len(resources), len(templates))
			for i, resource := range resources {
				if i >= 3 { // Limit output
					fmt.Printf("   ... and %d more\n", len(resources)-3)
					break
				}
				fmt.Printf("   - %s (%s)\n", resource.Name, resource.URI)
			}
		}
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("\n⏳ Client is running and listening for callbacks...\n")
	fmt.Printf("   Press Ctrl+C to stop\n")
	fmt.Printf("   Callbacks will be displayed as they arrive\n")

	// Keep the client running and show periodic status
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Printf("\n👋 Shutting down gracefully...\n")
			return

		case <-ctx.Done():
			fmt.Printf("\n❌ Context cancelled: %v\n", ctx.Err())
			return

		case <-ticker.C:
			fmt.Printf("\n📊 Status check - client still active, callbacks registered\n")

			// Optional: ping the server to keep connection alive
			if result, err := mcpClient.Ping(ctx); err == nil {
				fmt.Printf("   🏓 Server ping: %dms roundtrip\n", result.Timestamp)
			}
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
