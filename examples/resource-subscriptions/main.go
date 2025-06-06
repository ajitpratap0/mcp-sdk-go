package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// FileSystemResourcesProvider simulates a file system with resource subscriptions
type FileSystemResourcesProvider struct {
	*server.BaseResourcesProvider
	subscriptionManager *server.ResourceSubscriptionManager
}

// NewFileSystemResourcesProvider creates a new file system provider
func NewFileSystemResourcesProvider() *FileSystemResourcesProvider {
	return &FileSystemResourcesProvider{
		BaseResourcesProvider: server.NewBaseResourcesProvider(),
	}
}

// SetSubscriptionManager sets the subscription manager for this provider
func (p *FileSystemResourcesProvider) SetSubscriptionManager(manager *server.ResourceSubscriptionManager) {
	p.subscriptionManager = manager
}

// SimulateFileChange simulates a file change and notifies subscribers
func (p *FileSystemResourcesProvider) SimulateFileChange(changeType server.ResourceChangeType, uri string) {
	if p.subscriptionManager == nil {
		return
	}

	switch changeType {
	case server.ResourceAdded:
		resource := &protocol.Resource{
			URI:  uri,
			Name: extractFileName(uri),
			Type: "text/plain",
		}
		change := server.ResourceChange{
			Type:     changeType,
			Resource: resource,
			URI:      uri,
		}
		p.subscriptionManager.NotifyResourceChange(change)

	case server.ResourceModified:
		resource := &protocol.Resource{
			URI:  uri,
			Name: extractFileName(uri),
			Type: "text/plain",
		}
		change := server.ResourceChange{
			Type:     changeType,
			Resource: resource,
			URI:      uri,
		}
		p.subscriptionManager.NotifyResourceChange(change)

	case server.ResourceRemoved:
		change := server.ResourceChange{
			Type: changeType,
			URI:  uri,
		}
		p.subscriptionManager.NotifyResourceChange(change)

	case server.ResourceUpdated:
		contents := &protocol.ResourceContents{
			URI:     uri,
			Type:    "text/plain",
			Content: json.RawMessage(`"Updated file content"`),
		}
		change := server.ResourceChange{
			Type:     changeType,
			Contents: contents,
			URI:      uri,
		}
		p.subscriptionManager.NotifyResourceChange(change)
	}
}

func extractFileName(uri string) string {
	// Simple extraction - in real implementation, use proper path parsing
	parts := make([]string, 0)
	current := ""
	for _, char := range uri {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return uri
}

func main() {
	// Create stdio transport
	stdioTransport := transport.NewStdioTransportWithStdInOut()

	// Create file system provider
	fsProvider := NewFileSystemResourcesProvider()

	// Create server with resource subscriptions enabled
	srv := server.New(
		stdioTransport,
		server.WithName("Resource Subscription Example"),
		server.WithVersion("1.0.0"),
		server.WithDescription("Example server demonstrating resource subscriptions"),
		server.WithResourcesProvider(fsProvider),
		server.WithResourceSubscriptions(), // Enable resource subscriptions
	)

	// Get the subscription manager from the server (this is a bit hacky for the example)
	// In a real implementation, you'd want a cleaner way to access this
	go func() {
		// Wait a bit for server to initialize
		time.Sleep(100 * time.Millisecond)

		// Simulate file system changes
		fmt.Println("Simulating file system changes...")

		time.Sleep(2 * time.Second)
		fmt.Println("Adding file: /documents/readme.txt")
		fsProvider.SimulateFileChange(server.ResourceAdded, "/documents/readme.txt")

		time.Sleep(2 * time.Second)
		fmt.Println("Modifying file: /documents/readme.txt")
		fsProvider.SimulateFileChange(server.ResourceModified, "/documents/readme.txt")

		time.Sleep(2 * time.Second)
		fmt.Println("Updating content of: /documents/readme.txt")
		fsProvider.SimulateFileChange(server.ResourceUpdated, "/documents/readme.txt")

		time.Sleep(2 * time.Second)
		fmt.Println("Adding nested file: /documents/subfolder/notes.txt")
		fsProvider.SimulateFileChange(server.ResourceAdded, "/documents/subfolder/notes.txt")

		time.Sleep(2 * time.Second)
		fmt.Println("Removing file: /documents/readme.txt")
		fsProvider.SimulateFileChange(server.ResourceRemoved, "/documents/readme.txt")
	}()

	// Start the server
	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
