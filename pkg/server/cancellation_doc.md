# Request Cancellation in Go MCP SDK Server

## Overview

The Go MCP SDK Server now supports request cancellation, allowing clients to cancel long-running operations. This feature is particularly useful for:

- Long-running tool calls
- Resource-intensive operations
- User-initiated cancellation of tasks
- Graceful shutdown scenarios

## Implementation Details

### Request Tracking

The server tracks active requests using a thread-safe map:

```go
// Server state
activeRequests     map[string]context.CancelFunc
activeRequestsLock sync.RWMutex
```

Each request is tracked with:
- **Request ID**: A unique identifier for the request (converted to string for consistency)
- **Cancel Function**: A context.CancelFunc that can be called to cancel the request

### Key Methods

#### `trackRequest(requestID string, cancelFunc context.CancelFunc)`
Adds a request to the active requests map with its cancellation function.

#### `completeRequest(requestID string)`
Removes a request from the active requests map when it completes normally.

#### `cancelRequest(requestID string) bool`
Cancels a specific request by ID and returns whether the cancellation was successful.

### Cancel Handler

The `handleCancel` method processes cancellation requests:

```go
func (s *Server) handleCancel(ctx context.Context, params interface{}) (interface{}, error) {
    var cancelParams protocol.CancelParams
    if err := s.validateParams(params, &cancelParams, protocol.MethodCancel); err != nil {
        return nil, err
    }

    requestID := fmt.Sprintf("%v", cancelParams.ID)
    cancelled := s.cancelRequest(requestID)

    return &protocol.CancelResult{Cancelled: cancelled}, nil
}
```

## Usage Examples

### Client-Side Cancellation

A client can cancel a long-running request by sending a cancel request:

```go
// Start a long-running operation
requestID := "unique-request-123"
go client.CallTool(ctx, "long-operation", params)

// Cancel the operation
cancelParams := &protocol.CancelParams{
    ID: requestID,
}
result, err := client.SendRequest(ctx, protocol.MethodCancel, cancelParams)
```

### Server-Side Handling

Tool providers should respect context cancellation:

```go
func (p *ToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, toolContext json.RawMessage) (*protocol.CallToolResult, error) {
    // Long-running operation
    select {
    case <-time.After(10 * time.Second):
        return &protocol.CallToolResult{
            Result: json.RawMessage(`"Operation completed"`),
        }, nil
    case <-ctx.Done():
        // Handle cancellation
        return nil, context.Canceled
    }
}
```

### Graceful Shutdown

When the server stops, all active requests are automatically cancelled:

```go
func (s *Server) Stop() error {
    s.cancel()

    // Cancel all active requests
    s.activeRequestsLock.Lock()
    for _, cancelFunc := range s.activeRequests {
        cancelFunc()
    }
    s.activeRequests = make(map[string]context.CancelFunc)
    s.activeRequestsLock.Unlock()

    return s.transport.Stop(context.Background())
}
```

## Best Practices

1. **Always Check Context**: Long-running operations should periodically check `ctx.Done()` to respond to cancellation requests promptly.

2. **Clean Up Resources**: When a request is cancelled, ensure all associated resources are properly cleaned up.

3. **Return Appropriate Errors**: When a tool call is cancelled, return an appropriate error message:
   ```go
   if ctx.Err() == context.Canceled {
       return &protocol.CallToolResult{
           Error: "Tool call was cancelled",
       }, nil
   }
   ```

4. **Thread Safety**: The implementation uses RWMutex to ensure thread-safe access to the active requests map.

## Limitations

### Current Limitations

1. **Transport Layer Integration**: The current implementation tracks requests at the server level. Full integration with the transport layer would allow automatic request ID propagation.

2. **Request ID Management**: Clients need to manage request IDs themselves to use the cancellation feature effectively.

3. **Partial Results**: The current implementation doesn't support returning partial results from cancelled operations.

### Future Enhancements

1. **Automatic Request ID Injection**: Modify the transport layer to automatically inject request IDs into handler contexts.

2. **Progress Reporting**: Combine with progress reporting to show partial completion before cancellation.

3. **Cancellation Deadlines**: Support for cancellation with deadlines or timeouts.

4. **Cascading Cancellation**: Support for cancelling dependent operations when a parent operation is cancelled.

## Testing

The implementation includes comprehensive tests covering:

- Basic request tracking and cancellation
- Concurrent cancellation requests
- Server shutdown cancelling all requests
- Long-running tool cancellation
- Normal request completion without cancellation

Run the tests with:
```bash
go test -v ./pkg/server -run "Test.*Cancel"
```

## Conclusion

Request cancellation is now a core feature of the Go MCP SDK Server, providing better control over long-running operations and improving the overall user experience. The implementation is thread-safe, well-tested, and follows Go best practices for context-based cancellation.
