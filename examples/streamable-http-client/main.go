package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/model-context-protocol/go-mcp/pkg/client"
	"github.com/model-context-protocol/go-mcp/pkg/protocol"
	"github.com/model-context-protocol/go-mcp/pkg/transport"
)

const (
	serverURL = "http://localhost:8080/mcp"
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

	// Create a streamable HTTP transport for connecting to the server with a longer timeout
	t := transport.NewStreamableHTTPTransport(serverURL, transport.WithRequestTimeout(2*time.Minute))

	// Set a custom session ID for resumability (optional)
	sessionID := "session-" + time.Now().Format("20060102-150405")
	t.SetSessionID(sessionID)
	log.Printf("Using session ID: %s", sessionID)

	// Set custom headers if needed
	t.SetHeader("User-Agent", "MCP-Streamable-Client/1.0")

	// Create client with needed capabilities
	c := client.New(t,
		client.WithName("StreamableHTTPClient"),
		client.WithVersion("1.0.0"),
		client.WithCapability(protocol.CapabilitySampling, true),
		client.WithCapability(protocol.CapabilityLogging, true),
	)

	// Initialize the client (connects to the server)
	log.Println("Connecting to MCP server...")
	if err := c.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer c.Close()

	// Output server info
	serverInfo := c.ServerInfo()
	log.Printf("Connected to server: %s %s", serverInfo.Name, serverInfo.Version)
	if serverInfo.Description != "" {
		log.Printf("Description: %s", serverInfo.Description)
	}

	// Example: Check server capabilities
	log.Println("Server capabilities:")
	checkCapability(c, protocol.CapabilityTools, "Tools")
	checkCapability(c, protocol.CapabilityResources, "Resources")
	checkCapability(c, protocol.CapabilityPrompts, "Prompts")
	checkCapability(c, protocol.CapabilityComplete, "Completion")
	checkCapability(c, protocol.CapabilityRoots, "Roots")
	checkCapability(c, protocol.CapabilityResourceSubscriptions, "Resource Subscriptions")
	checkCapability(c, protocol.CapabilityLogging, "Logging")
	checkCapability(c, protocol.CapabilitySampling, "Sampling")

	// Example: List tools if supported
	if c.HasCapability(protocol.CapabilityTools) {
		log.Println("Listing available tools...")
		tools, _, err := c.ListTools(ctx, "", nil)
		if err != nil {
			log.Printf("Error listing tools: %v", err)
		} else {
			log.Printf("Found %d tools:", len(tools))
			for _, tool := range tools {
				log.Printf("  - %s: %s", tool.Name, tool.Description)
			}

			// Call the hello tool if it exists
			callHelloTool(ctx, c, tools)

			// Call the streaming countToTen tool if it exists
			callStreamingTool(ctx, c, tools)
		}
	}

	// Example: List resources if supported
	if c.HasCapability(protocol.CapabilityResources) {
		log.Println("Listing available resources...")
		resources, templates, _, err := c.ListResources(ctx, "", false, nil)
		if err != nil {
			log.Printf("Error listing resources: %v", err)
		} else {
			log.Printf("Found %d resources:", len(resources))
			for _, resource := range resources {
				log.Printf("  - %s: %s", resource.URI, resource.Name)
			}

			log.Printf("Found %d resource templates:", len(templates))
			for _, template := range templates {
				log.Printf("  - %s: %s", template.URI, template.Name)
			}

			// Example: Read a resource if any exist
			if len(resources) > 0 {
				log.Printf("Reading first resource: %s", resources[0].URI)
				content, err := c.ReadResource(ctx, resources[0].URI, nil, nil)
				if err != nil {
					log.Printf("Error reading resource: %v", err)
				} else {
					log.Printf("Resource content type: %s", content.Type)
					var contentStr string
					if err := json.Unmarshal(content.Content, &contentStr); err == nil {
						log.Printf("Content: %s", contentStr)
					} else {
						log.Printf("Content: %s", string(content.Content))
					}
				}
			}
		}
	}

	// Wait for a moment to allow logs to be displayed
	time.Sleep(1 * time.Second)
	log.Println("Client example completed.")
}

func callHelloTool(ctx context.Context, c *client.Client, tools []protocol.Tool) {
	for _, tool := range tools {
		if tool.Name == "hello" {
			log.Println("Calling 'hello' tool...")
			input, _ := json.Marshal(map[string]string{"name": "Streamable MCP Client"})
			result, err := c.CallTool(ctx, "hello", input, nil)
			if err != nil {
				log.Printf("Error calling tool: %v", err)
			} else {
				var greeting struct {
					Greeting string `json:"greeting"`
				}
				if err := json.Unmarshal(result.Result, &greeting); err != nil {
					log.Printf("Error parsing result: %v", err)
				} else {
					log.Printf("Tool result: %s", greeting.Greeting)
				}
			}
			break
		}
	}
}

func callStreamingTool(ctx context.Context, c *client.Client, tools []protocol.Tool) {
	for _, tool := range tools {
		if tool.Name == "countToTen" {
			log.Println("Calling 'countToTen' streaming tool...")

			// Create a context with timeout for the streaming call
			streamCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			// Call the tool with streaming handler
			_, err := c.CallToolStreaming(streamCtx, "countToTen", nil, nil, func(update json.RawMessage) {
				var result struct {
					Count int  `json:"count"`
					Done  bool `json:"done"`
				}
				if err := json.Unmarshal(update, &result); err != nil {
					log.Printf("Error parsing streaming update: %v", err)
					return
				}
				log.Printf("Stream update: Count = %d, Done = %t", result.Count, result.Done)
			})

			if err != nil {
				log.Printf("Error calling streaming tool: %v", err)
			} else {
				log.Println("Streaming tool completed successfully")
			}
			break
		}
	}
}

func checkCapability(c *client.Client, capability protocol.CapabilityType, name string) {
	if c.HasCapability(capability) {
		log.Printf("  ✓ %s", name)
	} else {
		log.Printf("  ✗ %s", name)
	}
}
