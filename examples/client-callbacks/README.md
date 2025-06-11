# MCP Client Callbacks Example

This example demonstrates the complete client callback functionality in the MCP Go SDK, showing how to register and handle server-initiated events and notifications.

## Features Demonstrated

### 🔍 Sampling Callbacks
- Receive sampling requests from MCP servers
- Process sampling events with proper error handling
- Handle server-initiated sampling for AI model operations

### 📂 Resource Change Callbacks
- Subscribe to resource change notifications
- Handle resource additions, modifications, and deletions
- Process both `ResourcesChanged` and `ResourceUpdated` events

### 🛡️ Safety Features
- Thread-safe callback registration and invocation
- Panic recovery to prevent client crashes
- Graceful handling of nil callbacks

## Usage

### Prerequisites
1. An MCP server running on `http://localhost:8080/mcp`
2. Server should support the capabilities you want to test

### Running the Example

```bash
# Navigate to the example directory
cd examples/client-callbacks

# Run the example
go run main.go
```

### Expected Output

```
🚀 MCP Client Callbacks Example
This example demonstrates sampling and resource change callbacks

🔌 Connecting to MCP server at http://localhost:8080/mcp...
✅ Connected successfully!
📋 Server capabilities: map[logging:true sampling:true resources:true]
🖥️  Server: example-server v1.0.0

📂 Subscribing to resource changes...
   ✅ Subscribed to file:///tmp/example.txt
   ✅ Subscribed to file:///project/src/
   ✅ Subscribed to db://localhost/users

🔧 Listing available tools...
   Found 3 tools
   - file_read: Read contents of a file
   - web_search: Search the web for information
   - calculator: Perform mathematical calculations

📁 Listing available resources...
   Found 5 resources and 2 templates
   - config.json (file:///app/config.json)
   - users.db (db://localhost/users)
   - project files (file:///project/)

⏳ Client is running and listening for callbacks...
   Press Ctrl+C to stop
   Callbacks will be displayed as they arrive

🔍 SAMPLING EVENT RECEIVED:
   Type: sample_request
   Token ID: req-123
   Messages: 1
   Request ID: req-123
   Max Tokens: 100
   First Message: What is the weather like today?
   ✅ Sampling event processed

📂 RESOURCE CHANGE NOTIFICATION:
   URI: file:///tmp/example.txt
   Type: Resource Updated
   Deleted: false
   Content Type: text/plain
   Content Length: 156 bytes
   ✅ Resource change processed
```

## Implementation Details

### Callback Registration

```go
// Sampling callback - handles server sampling requests
mcpClient.SetSamplingCallback(func(event protocol.SamplingEvent) {
    // Process sampling event
    fmt.Printf("Sampling event: %s\n", event.Type)
})

// Resource change callback - handles resource notifications
mcpClient.SetResourceChangedCallback(func(uri string, data interface{}) {
    // Process resource change
    fmt.Printf("Resource changed: %s\n", uri)
})
```

### Thread Safety

All callback operations are thread-safe:
- Callbacks can be registered concurrently
- Callback invocation is protected from panics
- Callbacks can be set to `nil` to disable

### Error Handling

The implementation includes comprehensive error handling:
- Invalid parameters are properly validated
- Callback panics are recovered and logged
- Network errors are handled gracefully

## Integration with Real Servers

To use with real MCP servers:

1. **Update the endpoint** in the transport configuration
2. **Configure authentication** if required by your server
3. **Handle specific event types** based on your server's capabilities
4. **Implement business logic** in the callback functions

### Example with Authentication

```go
config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
config.Endpoint = "https://your-mcp-server.com/mcp"
config.Security.AllowedOrigins = []string{"https://your-domain.com"}

// Add authentication headers if needed
config.Headers = map[string]string{
    "Authorization": "Bearer your-token-here",
}
```

## Common Use Cases

### AI Model Integration
Use sampling callbacks to:
- Receive model sampling requests from servers
- Implement custom sampling logic
- Provide feedback to AI training processes

### Resource Monitoring
Use resource change callbacks to:
- Monitor file system changes
- Track database updates
- Sync distributed resource states
- Update UI when resources change

### Development Workflow
Use callbacks for:
- Live reloading during development
- Hot module replacement
- Build system integration
- Testing automation

## Troubleshooting

### No Callbacks Received
1. Check server capabilities match client expectations
2. Verify network connectivity
3. Ensure proper resource subscriptions
4. Check server logs for errors

### Callback Errors
1. Check callback function signatures
2. Verify error handling in callbacks
3. Look for panic recovery logs
4. Test with nil callback handling

### Connection Issues
1. Verify server endpoint is correct
2. Check firewall and network settings
3. Test with curl or other HTTP clients
4. Review transport configuration

## Next Steps

After running this example:

1. **Explore the client package** documentation
2. **Implement custom callbacks** for your use case
3. **Test with different server implementations**
4. **Build production applications** using the patterns shown

For more examples, see the other directories in the `examples/` folder.
