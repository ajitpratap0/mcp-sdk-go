package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

const (
	serverURL = "http://localhost:8081/mcp" // Must match the server port (8081)
)

func main() {
	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Starting MCP Streamable HTTP Client Example")

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

	// Set custom headers if needed
	t.SetHeader("User-Agent", "MCP-Streamable-Client/1.0")
	// Set Origin header for security validation
	t.SetHeader("Origin", "http://localhost")

	// Note: We don't need to set a session ID manually anymore as the transport
	// will automatically handle session management based on the server's response
	// When the server returns a MCP-Session-ID header, the transport will store and use it

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

	// If we found tools in the previous section, use those for batch example
	if c.HasCapability(protocol.CapabilityTools) {
		// Find available tools
		tools, _, err := c.ListTools(ctx, "", nil)
		if err != nil {
			log.Printf("Error listing tools: %v", err)
		} else {
			// Example: Call tools in a batch to demonstrate batch functionality
			callToolsBatch(ctx, c, tools, t)
		}
	}

	// Note: We're using the original transport instance directly
	// In a real application, you'd typically have a client method to access the transport
	{
		// We don't need to type assert since we already know it's a StreamableHTTPTransport
		// Use reflection to get the session ID since there's no direct getter
		typ := reflect.ValueOf(t).Elem()
		field := typ.FieldByName("sessionID")
		if field.IsValid() {
			sessionID := field.String()
			if sessionID != "" {
				log.Printf("Server assigned session ID: %s", sessionID)

				// Now demonstrate session termination by sending a DELETE request
				log.Println("Terminating session explicitly...")
				termCtx, termCancel := context.WithTimeout(ctx, 5*time.Second)
				defer termCancel()

				// Create a custom HTTP request to terminate the session
				req, _ := http.NewRequestWithContext(termCtx, "DELETE", serverURL, nil)
				req.Header.Set("MCP-Session-ID", sessionID)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					log.Printf("Error terminating session: %v", err)
				} else {
					defer resp.Body.Close()
					log.Printf("Session termination response: %s", resp.Status)
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

// callToolsBatch demonstrates calling multiple tools in a single batch request
func callToolsBatch(ctx context.Context, c *client.Client, tools []protocol.Tool, t *transport.StreamableHTTPTransport) {
	// Check if we have both hello and countToTen tools available
	hasHello := false
	hasCountToTen := false
	for _, tool := range tools {
		if tool.Name == "hello" {
			hasHello = true
		} else if tool.Name == "countToTen" {
			hasCountToTen = true
		}
	}

	if hasHello && hasCountToTen {
		log.Println("Calling multiple tools in a batch...")

		// Prepare the batch requests
		helloInput, _ := json.Marshal(map[string]string{"name": "Batch Request"})
		helloReq, _ := protocol.NewRequest("batch-hello", "tools/call", map[string]interface{}{
			"name":  "hello",
			"input": helloInput,
		})

		countReq, _ := protocol.NewRequest("batch-count", "tools/call", map[string]interface{}{
			"name": "countToTen",
		})

		// Send the batch request using our transport instance
		if err := t.SendBatch(ctx, []interface{}{helloReq, countReq}); err != nil {
			log.Printf("Error sending batch request: %v", err)
		} else {
			log.Println("Batch request sent successfully")
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
