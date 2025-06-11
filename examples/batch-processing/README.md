# Batch Processing Example

This example demonstrates JSON-RPC 2.0 batch processing functionality in the MCP Go SDK. It showcases how to create, serialize, parse, and process batch requests with full JSON-RPC 2.0 compliance.

## Features Demonstrated

### 🔧 Core Batch Processing
- Creating JSON-RPC 2.0 compliant batch requests
- Mixing requests and notifications in batches
- Proper response ordering and handling
- Batch serialization and parsing

### 📊 Advanced Scenarios
- **Math Operations**: Multiple computational requests in a single batch
- **Mixed Request Types**: Different tool calls batched together
- **Notification Handling**: Notifications that don't expect responses
- **Error Handling**: Proper error propagation for individual batch items
- **Performance Testing**: Large batch processing with timing metrics

### 🏗️ Architecture Features
- Full JSON-RPC 2.0 specification compliance
- Middleware integration (observability, reliability)
- Thread-safe batch processing
- Comprehensive error handling
- Performance optimization strategies

## JSON-RPC 2.0 Batch Compliance

This implementation follows the JSON-RPC 2.0 specification exactly:

1. **Batch Requests**: Arrays of Request/Notification objects
2. **Batch Responses**: Arrays of Response objects in the same order
3. **Notification Handling**: Notifications don't generate responses
4. **Error Handling**: Individual errors don't break the entire batch
5. **Empty Responses**: All-notification batches return empty response arrays

### Example Batch Request
```json
[
  {"jsonrpc": "2.0", "id": "1", "method": "callTool", "params": {"name": "math.add", "input": {"a": 10, "b": 5}}},
  {"jsonrpc": "2.0", "id": "2", "method": "callTool", "params": {"name": "string.concat", "input": {"a": "Hello", "b": " World"}}},
  {"jsonrpc": "2.0", "method": "notification", "params": {"message": "Processing batch"}}
]
```

### Example Batch Response
```json
[
  {"jsonrpc": "2.0", "id": "1", "result": {"content": [{"type": "text", "text": "Result: 10.00 + 5.00 = 15.00"}]}},
  {"jsonrpc": "2.0", "id": "2", "result": {"content": [{"type": "text", "text": "Result: 'Hello' + ' World' = 'Hello World'"}]}}
]
```

Note: No response for the notification as per JSON-RPC 2.0 spec.

## Running the Example

```bash
cd examples/batch-processing
go run main.go
```

## Key Components

### 1. JSON-RPC Batch Types

```go
// Create individual requests
req1, _ := protocol.NewRequest("1", "callTool", params1)
req2, _ := protocol.NewRequest("2", "callTool", params2)
notif, _ := protocol.NewNotification("status", statusParams)

// Create batch
batch, _ := protocol.NewJSONRPCBatchRequest(req1, req2, notif)

// Serialize to JSON
batchJSON, _ := batch.ToJSON()

// Parse from JSON
parsedBatch, _ := protocol.ParseJSONRPCBatchRequest(batchJSON)
```

### 2. Batch Processing

```go
// Server-side batch handling
response, err := transport.HandleBatchRequest(ctx, batch)

// Client-side batch sending
response, err := transport.SendBatchRequest(ctx, batch)
```

### 3. Tools Provider

The example includes a comprehensive tools provider supporting:
- **math.add**: Addition operations
- **math.multiply**: Multiplication operations
- **string.concat**: String concatenation
- **time.now**: Current timestamp

## Scenarios Demonstrated

### Scenario 1: Multiple Math Operations
Processes several mathematical operations in a single batch request.

### Scenario 2: Mixed Request Types
Combines different types of operations (math, string, time) in one batch.

### Scenario 3: Batch with Notifications
Shows how notifications are processed without generating responses.

### Scenario 4: Error Handling
Demonstrates proper error handling where some requests succeed and others fail.

### Scenario 5: Large Batch Performance
Tests performance with large batches (50+ requests) and provides timing metrics.

## Performance Benefits

Batch processing provides significant advantages:

1. **Network Efficiency**: Reduced round trips
2. **Resource Optimization**: Better connection utilization
3. **Throughput Improvement**: Higher requests per second
4. **Latency Reduction**: Lower average response times

### Sample Performance Output
```
Processing completed in 45ms
Successful requests: 48
Failed requests: 2
Average per request: 0.9ms
Throughput: 1066.7 requests/second
```

## Implementation Details

### JSON-RPC 2.0 Compliance
- Proper `jsonrpc: "2.0"` versioning
- Correct ID handling and response ordering
- Notification handling (no responses)
- Error propagation for individual items

### Error Handling
- Individual request errors don't break the batch
- Proper error response generation
- Context cancellation support
- Timeout handling

### Middleware Integration
- Observability: Logging and metrics for batch operations
- Reliability: Retry logic for failed batch requests
- Security: Origin validation and authentication

## Best Practices

1. **Batch Size**: Keep batches reasonable (10-100 requests typically)
2. **Error Handling**: Always check individual response errors
3. **Timeouts**: Set appropriate context timeouts for large batches
4. **Notifications**: Use notifications for fire-and-forget operations
5. **Performance**: Monitor batch processing metrics

## Related Documentation

- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [MCP Protocol Documentation](../../README.md)
- [Transport Layer Guide](../../pkg/transport/doc.go)
- [Protocol Layer Guide](../../pkg/protocol/doc.go)

## Future Enhancements

- Batch request prioritization
- Streaming batch responses
- Batch request optimization strategies
- Advanced error recovery patterns
- Performance auto-tuning
