package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
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

	// Create HTTP transport using modern config approach
	serverURL := "http://localhost:8080"
	if len(os.Args) > 1 {
		serverURL = os.Args[1]
	}

	// Create transport with reliability features enabled
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = serverURL
	config.Features.EnableReliability = true
	config.Reliability.MaxRetries = 5

	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create client with pagination capability
	c := client.New(t,
		client.WithName("PaginationExampleClient"),
		client.WithVersion("1.0.0"),
		client.WithCapability(protocol.CapabilityPagination, true),
		client.WithCapability(protocol.CapabilityResources, true),
	)

	// Initialize the client
	log.Println("Connecting to MCP server...")
	if err := c.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer c.Close()

	// Start the transport
	go func() {
		if err := t.Start(ctx); err != nil && ctx.Err() == nil {
			log.Fatalf("Transport error: %v", err)
		}
	}()

	// Example 1: Manual pagination
	log.Println("Example 1: Manual pagination")
	demonstrateManualPagination(ctx, c)

	// Example 2: Automatic pagination using collector utility
	log.Println("\nExample 2: Automatic pagination")
	demonstrateAutomaticPagination(ctx, c)
}

// demonstrateManualPagination shows how to handle pagination manually
func demonstrateManualPagination(ctx context.Context, c client.Client) {
	// Create pagination parameters with a small page size
	pagination := &protocol.PaginationParams{
		Limit: 10, // Small limit to demonstrate pagination
	}

	// First page
	resources, templates, pagResult, err := c.ListResources(ctx, "", true, pagination)
	if err != nil {
		log.Printf("Failed to list resources: %v", err)
		return
	}

	// Print results from first page
	log.Printf("Page 1: Found %d resources", len(resources))
	for i, res := range resources {
		if i < 3 { // Show just first few for brevity
			log.Printf("  - %s (%s)", res.Name, res.URI)
		}
	}

	// Check if there are more pages
	if pagResult != nil && pagResult.HasMore {
		// Create new pagination params with the cursor for the next page
		nextPagination := &protocol.PaginationParams{
			Limit:  10,
			Cursor: pagResult.NextCursor,
		}

		// Get next page
		moreResources, _, nextPagResult, err := c.ListResources(ctx, "", true, nextPagination)
		if err != nil {
			log.Printf("Failed to fetch next page: %v", err)
			return
		}

		// Print results from second page
		log.Printf("Page 2: Found %d more resources", len(moreResources))
		for i, res := range moreResources {
			if i < 3 { // Show just first few for brevity
				log.Printf("  - %s (%s)", res.Name, res.URI)
			}
		}

		// Show how many more pages might be available
		if nextPagResult != nil && nextPagResult.HasMore {
			log.Printf("More pages available, next cursor: %s", nextPagResult.NextCursor)
		}
	}

	// Check resource templates if any
	if len(templates) > 0 {
		log.Printf("Found %d resource templates", len(templates))
		for i, tmpl := range templates {
			if i < 2 { // Show just first few for brevity
				log.Printf("  - %s (%s)", tmpl.Name, tmpl.URI)
			}
		}
	}
}

// demonstrateAutomaticPagination shows how to use the automatic pagination utilities
func demonstrateAutomaticPagination(ctx context.Context, c client.Client) {
	// Get all resources automatically with the utility method
	allResources, allTemplates, err := c.ListAllResources(ctx, "", true)
	if err != nil {
		log.Printf("Failed to list all resources: %v", err)
		return
	}

	log.Printf("Found total of %d resources across all pages", len(allResources))

	// Show summary of resources found
	typeCount := make(map[string]int)
	for _, res := range allResources {
		typeCount[res.Type]++
	}

	for resType, count := range typeCount {
		log.Printf("  - %d resources of type '%s'", count, resType)
	}

	// Check resource templates if any
	if len(allTemplates) > 0 {
		log.Printf("Found %d resource templates", len(allTemplates))
		for i, tmpl := range allTemplates {
			if i < 2 { // Show just first few for brevity
				log.Printf("  - %s (%s)", tmpl.Name, tmpl.URI)
			}
		}
	}
}
