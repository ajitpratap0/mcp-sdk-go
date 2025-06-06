# Resource Subscriptions in Go MCP SDK Server

## Overview

The Go MCP SDK Server now supports resource subscriptions, allowing clients to receive real-time notifications when resources change. This feature enables reactive applications that can respond to resource modifications, additions, and deletions.

## Features

- **Real-time Notifications**: Clients receive immediate notifications when subscribed resources change
- **Recursive Subscriptions**: Subscribe to a URI and automatically get notifications for all child resources
- **Batched Notifications**: Multiple changes are batched together for efficiency
- **Thread-safe Operations**: All subscription operations are thread-safe and can be used concurrently
- **Change Types**: Support for added, modified, removed, and updated resource notifications

## Architecture

### ResourceSubscriptionManager

The core component that manages all resource subscriptions and notifications:

```go
type ResourceSubscriptionManager struct {
    subscriptions map[string]*Subscription
    notifier      ResourceNotifier
    logger        Logger
    changes       chan ResourceChange
}
```

### Resource Changes

Different types of resource changes are supported:

```go
type ResourceChangeType int

const (
    ResourceAdded    ResourceChangeType = iota // New resource created
    ResourceModified                           // Resource metadata changed
    ResourceRemoved                            // Resource deleted
    ResourceUpdated                            // Resource content changed
)
```

### Subscription States

Each subscription tracks:

```go
type Subscription struct {
    URI        string    // The resource URI being subscribed to
    Recursive  bool      // Whether to include child resources
    CreatedAt  time.Time // When the subscription was created
    LastUpdate time.Time // Last time a notification was sent
}
```

## Usage

### Enabling Resource Subscriptions

To enable resource subscriptions on your server:

```go
server := server.New(
    transport,
    server.WithResourcesProvider(myResourcesProvider),
    server.WithResourceSubscriptions(), // Enable subscriptions
)
```

### Subscribing to Resources

Clients can subscribe to resources using the standard MCP protocol:

```json
{
    "jsonrpc": "2.0",
    "id": "subscribe-1",
    "method": "subscribeResource",
    "params": {
        "uri": "/documents",
        "recursive": true
    }
}
```

### Implementing a Resource Provider with Subscriptions

```go
type MyResourcesProvider struct {
    *server.BaseResourcesProvider
    subscriptionManager *server.ResourceSubscriptionManager
}

func (p *MyResourcesProvider) NotifyFileChanged(uri string) {
    if p.subscriptionManager != nil {
        change := server.ResourceChange{
            Type: server.ResourceModified,
            Resource: &protocol.Resource{
                URI:  uri,
                Name: extractFileName(uri),
                Type: "text/plain",
            },
            URI: uri,
        }
        p.subscriptionManager.NotifyResourceChange(change)
    }
}
```

### Using the Subscribable Wrapper

For automatic subscription management, use the subscribable wrapper:

```go
// Wrap your provider with subscription support
baseProvider := NewMyResourcesProvider()
subscriptionManager := server.NewResourceSubscriptionManager(server, logger)
subscribableProvider := server.NewSubscribableResourcesProvider(baseProvider, subscriptionManager)

server := server.New(
    transport,
    server.WithResourcesProvider(subscribableProvider),
    server.WithResourceSubscriptions(),
)
```

## Notification Types

### Resource Changes Notification

Sent when resources are added, modified, or removed:

```json
{
    "jsonrpc": "2.0",
    "method": "resourcesChanged",
    "params": {
        "uri": "/documents",
        "added": [
            {
                "uri": "/documents/new-file.txt",
                "name": "new-file.txt",
                "type": "text/plain"
            }
        ],
        "modified": [
            {
                "uri": "/documents/existing-file.txt",
                "name": "existing-file.txt",
                "type": "text/plain"
            }
        ],
        "removed": ["/documents/deleted-file.txt"]
    }
}
```

### Resource Updated Notification

Sent when resource content changes:

```json
{
    "jsonrpc": "2.0",
    "method": "resourceUpdated",
    "params": {
        "uri": "/documents/file.txt",
        "contents": {
            "uri": "/documents/file.txt",
            "type": "text/plain",
            "content": "Updated file content"
        },
        "deleted": false
    }
}
```

## Configuration Options

### Subscription Manager Configuration

```go
// Create with custom buffer size
manager := server.NewResourceSubscriptionManager(notifier, logger)

// Start processing changes
manager.Start(ctx)

// Subscribe to resources
manager.Subscribe("/documents", true) // recursive

// Unsubscribe
manager.Unsubscribe("/documents")

// Check subscription status
isSubscribed := manager.IsSubscribed("/documents/file.txt")

// Notify of changes
change := server.ResourceChange{
    Type: server.ResourceAdded,
    Resource: &protocol.Resource{
        URI:  "/documents/new-file.txt",
        Name: "new-file.txt",
        Type: "text/plain",
    },
    URI: "/documents/new-file.txt",
}
manager.NotifyResourceChange(change)
```

### Batching Configuration

Notifications are automatically batched every 100ms for efficiency. This prevents flooding clients with too many individual notifications when many changes occur rapidly.

## Best Practices

### 1. Use Recursive Subscriptions Carefully

Recursive subscriptions can generate many notifications. Consider the scope carefully:

```go
// Good: Specific scope
manager.Subscribe("/documents/project1", true)

// Be careful: Very broad scope
manager.Subscribe("/", true) // Will notify for all resources
```

### 2. Implement Proper Error Handling

Always handle errors when sending notifications:

```go
func (p *MyProvider) notifyChange(change server.ResourceChange) {
    if err := p.subscriptionManager.NotifyResourceChange(change); err != nil {
        log.Printf("Failed to notify resource change: %v", err)
        // Consider retry logic or fallback behavior
    }
}
```

### 3. Clean Up Subscriptions

Ensure subscriptions are cleaned up when no longer needed:

```go
// Implement cleanup in your provider
func (p *MyProvider) Cleanup() {
    if p.subscriptionManager != nil {
        p.subscriptionManager.Stop()
    }
}
```

### 4. Use Appropriate Change Types

Choose the correct change type for better client experience:

- **ResourceAdded**: New resources created
- **ResourceModified**: Metadata changes (name, type, etc.)
- **ResourceRemoved**: Resources deleted
- **ResourceUpdated**: Content changes

## Limitations

### Current Limitations

1. **Transport Independence**: Subscriptions are managed at the server level, not transport level
2. **Memory Usage**: All subscriptions are kept in memory
3. **No Persistence**: Subscriptions don't survive server restarts

### Future Enhancements

1. **Persistent Subscriptions**: Store subscriptions in persistent storage
2. **Subscription Filtering**: Advanced filtering based on resource properties
3. **Rate Limiting**: Configurable rate limiting for notifications
4. **Subscription Events**: Notifications when subscriptions are added/removed

## Testing

The subscription system includes comprehensive tests:

```bash
# Run all subscription tests
go test -v ./pkg/server -run ".*Subscription.*"

# Run specific test categories
go test -v ./pkg/server -run "TestResourceChangeNotifications"
go test -v ./pkg/server -run "TestBatchedNotifications"
```

## Examples

See the complete working example in:
- `examples/resource-subscriptions/main.go`

## Troubleshooting

### Common Issues

1. **No Notifications Received**
   - Ensure `WithResourceSubscriptions()` is enabled
   - Check that client properly subscribed to the resource
   - Verify resource changes are being reported to the subscription manager

2. **Performance Issues with Many Notifications**
   - Use more specific (less recursive) subscriptions
   - Consider implementing custom batching logic
   - Monitor the notification channel buffer size

3. **Memory Leaks**
   - Always call `Stop()` on the subscription manager
   - Ensure subscriptions are properly cleaned up
   - Monitor subscription count in production

### Debug Logging

Enable debug logging to troubleshoot subscription issues:

```go
logger := server.NewDefaultLogger()
// Set to debug level to see subscription activity
manager := server.NewResourceSubscriptionManager(notifier, logger)
```

## Conclusion

Resource subscriptions provide a powerful way to build reactive applications with the Go MCP SDK. The batched notification system ensures efficiency while maintaining real-time responsiveness. The thread-safe design allows for safe use in concurrent environments.
