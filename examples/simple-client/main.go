package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

func main() {
	// Create a context that can be canceled on interrupt
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Create a stdio transport for connecting to the server
	t := transport.NewStdioTransport()

	// Set the receive handler
	t.SetReceiveHandler(func(data []byte) {
		log.Printf("Client received message: %s", string(data))
	})

	// Create client with needed capabilities
	c := client.New(t,
		client.WithName("SimpleClient"),
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

			// Example: Call the hello tool if it exists
			if len(tools) > 0 {
				for _, tool := range tools {
					if tool.Name == "hello" {
						log.Println("Calling 'hello' tool...")
						input, _ := json.Marshal(map[string]string{"name": "MCP Client"})
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

	// Example: List prompts if supported
	if c.HasCapability(protocol.CapabilityPrompts) {
		log.Println("Listing available prompts...")
		prompts, _, err := c.ListPrompts(ctx, "", nil)
		if err != nil {
			log.Printf("Error listing prompts: %v", err)
		} else {
			log.Printf("Found %d prompts:", len(prompts))
			for _, prompt := range prompts {
				log.Printf("  - %s: %s", prompt.ID, prompt.Name)
			}

			// Example: Get a prompt if any exist
			if len(prompts) > 0 {
				log.Printf("Getting prompt details: %s", prompts[0].ID)
				prompt, err := c.GetPrompt(ctx, prompts[0].ID)
				if err != nil {
					log.Printf("Error getting prompt: %v", err)
				} else {
					log.Printf("Prompt name: %s", prompt.Name)
					log.Printf("Prompt description: %s", prompt.Description)
					log.Printf("Prompt has %d messages", len(prompt.Messages))
					log.Printf("Prompt has %d parameters", len(prompt.Parameters))
				}
			}
		}
	}

	// Wait for a moment to allow logs to be displayed
	time.Sleep(1 * time.Second)
	log.Println("Client example completed.")
}

// checkCapability checks if the client supports a specific capability and logs the result.
// It provides a simple visual indicator for capability support in the console output.
func checkCapability(c client.Client, capability protocol.CapabilityType, name string) {
	if c.HasCapability(capability) {
		log.Printf("  ✓ %s", name)
	} else {
		log.Printf("  ✗ %s", name)
	}
}
