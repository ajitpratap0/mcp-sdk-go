# Custom Transport Implementation Guide

This guide demonstrates how to create custom transport implementations for the MCP Go SDK. It provides examples, patterns, and best practices for extending the transport layer to support new protocols and communication mechanisms.

## 📋 Table of Contents

1. [Overview](#overview)
2. [Transport Interface](#transport-interface)
3. [Implementation Examples](#implementation-examples)
4. [Design Patterns](#design-patterns)
5. [Best Practices](#best-practices)
6. [Testing](#testing)
7. [Performance Considerations](#performance-considerations)
8. [Real-World Examples](#real-world-examples)

## 🎯 Overview

The MCP Go SDK provides a flexible transport abstraction that allows you to implement custom communication mechanisms while maintaining compatibility with the MCP protocol. This guide shows you how to:

- Implement the `Transport` interface
- Handle JSON-RPC 2.0 message serialization
- Manage connection lifecycle and error handling
- Create factory patterns for transport selection
- Integrate with the existing middleware system

## 🔌 Transport Interface

All custom transports must implement the `Transport` interface:

```go
type Transport interface {
    // Send a JSON-RPC request and wait for response
    SendRequest(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error)

    // Send a JSON-RPC notification (no response expected)
    SendNotification(ctx context.Context, notification *protocol.JSONRPCNotification) error

    // Send a batch of JSON-RPC requests
    SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error)

    // Handle incoming batch requests (server-side)
    HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest, handler RequestHandler) (*protocol.JSONRPCBatchResponse, error)

    // Receive incoming messages
    ReceiveMessage(ctx context.Context) (interface{}, error)

    // Lifecycle management
    Start(ctx context.Context) error
    Stop()
    Cleanup()
}
```

### Key Responsibilities

1. **Message Serialization**: Convert between Go structs and wire format
2. **Connection Management**: Establish, maintain, and close connections
3. **Error Handling**: Handle network errors and protocol errors gracefully
4. **Concurrency**: Support concurrent operations safely
5. **Resource Cleanup**: Properly clean up resources on shutdown

## 🛠️ Implementation Examples

### 1. WebSocket Transport

Perfect for real-time, bidirectional communication:

```go
type WebSocketTransport struct {
    conn     *websocket.Conn
    connLock sync.RWMutex

    incomingMessages chan []byte
    outgoingMessages chan []byte

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup

    endpoint string
    upgrader websocket.Upgrader
}
```

**Key Features:**
- Bidirectional real-time communication
- Built-in message framing
- Support for text and binary messages
- Connection upgrade from HTTP

**Use Cases:**
- Real-time collaborative applications
- Interactive debugging tools
- Live data streaming
- Browser-based MCP clients

### 2. Message Queue Transport

Great for asynchronous, reliable message delivery:

```go
type MessageQueueTransport struct {
    requestQueue  string
    responseQueue string

    messages     map[string]chan []byte // Request ID -> Response channel
    messagesMux  sync.RWMutex

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}
```

**Key Features:**
- Asynchronous message processing
- Reliable delivery guarantees
- Load balancing across consumers
- Message persistence

**Use Cases:**
- Microservice architectures
- Event-driven systems
- Background processing
- High-throughput scenarios

### 3. gRPC Transport

For high-performance, type-safe communication:

```go
type GRPCTransport struct {
    conn   *grpc.ClientConn
    client MCPServiceClient
    server *grpc.Server

    ctx    context.Context
    cancel context.CancelFunc
}

// Example service definition (protocol buffers)
service MCPService {
    rpc SendRequest(JSONRPCRequest) returns (JSONRPCResponse);
    rpc SendNotification(JSONRPCNotification) returns (google.protobuf.Empty);
    rpc SendBatchRequest(JSONRPCBatchRequest) returns (JSONRPCBatchResponse);
    rpc StreamMessages(stream JSONRPCMessage) returns (stream JSONRPCMessage);
}
```

**Key Features:**
- High performance binary protocol
- Strong typing with Protocol Buffers
- Built-in streaming support
- HTTP/2 multiplexing

**Use Cases:**
- High-performance applications
- Microservice communication
- Cross-language interoperability
- Production systems requiring reliability

## 📐 Design Patterns

### 1. Factory Pattern

Create transports based on configuration:

```go
type TransportFactory struct {
    transports map[string]TransportCreator
}

type TransportCreator func(config map[string]interface{}) (Transport, error)

func (tf *TransportFactory) Register(name string, creator TransportCreator) {
    tf.transports[name] = creator
}

func (tf *TransportFactory) Create(transportType string, config map[string]interface{}) (Transport, error) {
    creator, exists := tf.transports[transportType]
    if !exists {
        return nil, fmt.Errorf("unknown transport type: %s", transportType)
    }
    return creator(config)
}

// Usage
factory := &TransportFactory{transports: make(map[string]TransportCreator)}
factory.Register("websocket", NewWebSocketTransport)
factory.Register("messagequeue", NewMessageQueueTransport)
factory.Register("grpc", NewGRPCTransport)

transport, err := factory.Create("websocket", map[string]interface{}{
    "endpoint": ":8080",
    "path":     "/ws",
})
```

### 2. Decorator Pattern

Add functionality to existing transports:

```go
type RetryTransport struct {
    transport   Transport
    maxRetries  int
    backoff     time.Duration
}

func (rt *RetryTransport) SendRequest(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
    var lastErr error

    for attempt := 0; attempt <= rt.maxRetries; attempt++ {
        if attempt > 0 {
            select {
            case <-time.After(rt.backoff * time.Duration(attempt)):
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }

        response, err := rt.transport.SendRequest(ctx, request)
        if err == nil {
            return response, nil
        }

        lastErr = err
        if !isRetryableError(err) {
            break
        }
    }

    return nil, lastErr
}

func NewRetryTransport(transport Transport, maxRetries int, backoff time.Duration) *RetryTransport {
    return &RetryTransport{
        transport:  transport,
        maxRetries: maxRetries,
        backoff:    backoff,
    }
}
```

### 3. Connection Pool Pattern

Manage multiple connections for scalability:

```go
type PooledTransport struct {
    pool     chan Transport
    factory  func() (Transport, error)
    maxConns int

    mu       sync.RWMutex
    conns    map[Transport]bool
}

func (pt *PooledTransport) getConnection() (Transport, error) {
    select {
    case conn := <-pt.pool:
        return conn, nil
    default:
        return pt.factory()
    }
}

func (pt *PooledTransport) returnConnection(conn Transport) {
    select {
    case pt.pool <- conn:
    default:
        // Pool is full, close the connection
        conn.Cleanup()
    }
}
```

## ✅ Best Practices

### 1. Error Handling

```go
// Define custom error types
type TransportError struct {
    Type    ErrorType
    Message string
    Cause   error
}

type ErrorType string

const (
    ErrorTypeConnection ErrorType = "connection"
    ErrorTypeTimeout    ErrorType = "timeout"
    ErrorTypeProtocol   ErrorType = "protocol"
    ErrorTypeSerialization ErrorType = "serialization"
)

func (te *TransportError) Error() string {
    if te.Cause != nil {
        return fmt.Sprintf("%s error: %s (caused by: %v)", te.Type, te.Message, te.Cause)
    }
    return fmt.Sprintf("%s error: %s", te.Type, te.Message)
}

// Classify errors for retry logic
func isRetryableError(err error) bool {
    if te, ok := err.(*TransportError); ok {
        return te.Type == ErrorTypeConnection || te.Type == ErrorTypeTimeout
    }
    return false
}
```

### 2. Context Handling

```go
func (t *CustomTransport) SendRequest(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
    // Check context before starting
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    // Create a timeout context for the operation
    opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Use select to handle cancellation
    done := make(chan result, 1)
    go func() {
        resp, err := t.doSendRequest(request)
        done <- result{resp: resp, err: err}
    }()

    select {
    case res := <-done:
        return res.resp, res.err
    case <-opCtx.Done():
        return nil, opCtx.Err()
    }
}
```

### 3. Resource Management

```go
type ResourceManagedTransport struct {
    conn     net.Conn
    mu       sync.RWMutex
    closed   bool
    cleanups []func()
}

func (rmt *ResourceManagedTransport) addCleanup(cleanup func()) {
    rmt.mu.Lock()
    defer rmt.mu.Unlock()

    if rmt.closed {
        cleanup() // Execute immediately if already closed
        return
    }

    rmt.cleanups = append(rmt.cleanups, cleanup)
}

func (rmt *ResourceManagedTransport) Cleanup() {
    rmt.mu.Lock()
    defer rmt.mu.Unlock()

    if rmt.closed {
        return
    }

    rmt.closed = true

    // Execute all cleanup functions
    for _, cleanup := range rmt.cleanups {
        cleanup()
    }

    // Close the connection
    if rmt.conn != nil {
        rmt.conn.Close()
    }
}
```

### 4. Concurrency Safety

```go
type ThreadSafeTransport struct {
    mu         sync.RWMutex
    connection Connection
    messageID  int64
    pending    map[int64]chan *protocol.JSONRPCResponse
}

func (tst *ThreadSafeTransport) nextMessageID() int64 {
    return atomic.AddInt64(&tst.messageID, 1)
}

func (tst *ThreadSafeTransport) SendRequest(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
    // Generate unique message ID
    id := tst.nextMessageID()
    request.ID = id

    // Create response channel
    responseChan := make(chan *protocol.JSONRPCResponse, 1)

    tst.mu.Lock()
    tst.pending[id] = responseChan
    tst.mu.Unlock()

    defer func() {
        tst.mu.Lock()
        delete(tst.pending, id)
        tst.mu.Unlock()
        close(responseChan)
    }()

    // Send request...
    // Wait for response...
}
```

## 🧪 Testing

### Unit Testing

```go
func TestWebSocketTransport(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        upgrader := websocket.Upgrader{}
        conn, err := upgrader.Upgrade(w, r, nil)
        require.NoError(t, err)
        defer conn.Close()

        // Echo messages back
        for {
            messageType, message, err := conn.ReadMessage()
            if err != nil {
                break
            }
            conn.WriteMessage(messageType, message)
        }
    }))
    defer server.Close()

    // Test transport
    transport := NewWebSocketTransport(server.URL)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := transport.Start(ctx)
    require.NoError(t, err)
    defer transport.Stop()

    // Test sending request
    request, _ := protocol.NewRequest("1", "test", json.RawMessage(`{}`))
    response, err := transport.SendRequest(ctx, request)
    require.NoError(t, err)
    require.NotNil(t, response)
}
```

### Integration Testing

```go
func TestTransportIntegration(t *testing.T) {
    // Start server with custom transport
    transport := NewCustomTransport()
    server := server.New(transport,
        server.WithToolsProvider(&testToolsProvider{}),
    )

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    err := server.Start(ctx)
    require.NoError(t, err)
    defer server.Stop()

    // Create client with same transport type
    clientTransport := NewCustomTransport()
    client := client.New(clientTransport)

    err = client.Initialize(ctx)
    require.NoError(t, err)

    // Test actual MCP operations
    tools, err := client.ListTools(ctx, nil)
    require.NoError(t, err)
    require.NotEmpty(t, tools)
}
```

### Benchmark Testing

```go
func BenchmarkCustomTransport(b *testing.B) {
    transport := NewCustomTransport()
    ctx := context.Background()

    request, _ := protocol.NewRequest("1", "test", json.RawMessage(`{}`))

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := transport.SendRequest(ctx, request)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## ⚡ Performance Considerations

### 1. Connection Pooling

For high-throughput scenarios:

```go
type PoolConfig struct {
    MinConnections int
    MaxConnections int
    IdleTimeout    time.Duration
    MaxLifetime    time.Duration
}

type ConnectionPool struct {
    config  PoolConfig
    factory func() (Connection, error)

    idle   chan Connection
    active map[Connection]time.Time
    mu     sync.RWMutex
}
```

### 2. Message Batching

Reduce overhead by batching messages:

```go
type BatchingTransport struct {
    transport   Transport
    batchSize   int
    batchDelay  time.Duration

    pending     []*pendingMessage
    pendingMu   sync.Mutex
    flushTimer  *time.Timer
}

type pendingMessage struct {
    request     *protocol.JSONRPCRequest
    responseChan chan *protocol.JSONRPCResponse
}

func (bt *BatchingTransport) SendRequest(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
    responseChan := make(chan *protocol.JSONRPCResponse, 1)

    bt.pendingMu.Lock()
    bt.pending = append(bt.pending, &pendingMessage{
        request:     request,
        responseChan: responseChan,
    })

    if len(bt.pending) >= bt.batchSize {
        bt.flushPending()
    } else if bt.flushTimer == nil {
        bt.flushTimer = time.AfterFunc(bt.batchDelay, bt.flushPending)
    }
    bt.pendingMu.Unlock()

    select {
    case response := <-responseChan:
        return response, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### 3. Memory Management

Efficient buffer management:

```go
var messagePool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 1024)
    },
}

func (t *CustomTransport) writeMessage(message []byte) error {
    // Get buffer from pool
    buf := messagePool.Get().([]byte)
    defer messagePool.Put(buf[:0])

    // Use buffer for processing
    buf = append(buf, message...)

    return t.conn.Write(buf)
}
```

## 🌍 Real-World Examples

### 1. WebRTC Transport

For peer-to-peer communication:

```go
type WebRTCTransport struct {
    peerConnection *webrtc.PeerConnection
    dataChannel    *webrtc.DataChannel
    signaling      SignalingChannel
}
```

### 2. MQTT Transport

For IoT and edge computing:

```go
type MQTTTransport struct {
    client     mqtt.Client
    requestTopic  string
    responseTopic string
}
```

### 3. Named Pipe Transport

For local inter-process communication:

```go
type NamedPipeTransport struct {
    pipeName string
    conn     net.Conn
}
```

### 4. Unix Domain Socket Transport

For high-performance local communication:

```go
type UnixSocketTransport struct {
    socketPath string
    listener   net.Listener
    conn       net.Conn
}
```

## 🔧 Running the Examples

### Prerequisites

```bash
# Install dependencies
go mod download
go install github.com/gorilla/websocket@latest
```

### Basic Usage

```bash
# Run the main example
go run main.go

# Run with different transport types
export TRANSPORT_TYPE=websocket
go run main.go

export TRANSPORT_TYPE=messagequeue
go run main.go
```

### Testing Custom Transports

```bash
# Run unit tests
go test -v ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Test with race detection
go test -race -v ./...
```

## 📚 Additional Resources

- [Transport Package Documentation](../../pkg/transport/)
- [Protocol Package Documentation](../../pkg/protocol/)
- [Standard Transport Implementations](../../pkg/transport/)
- [MCP Specification](https://spec.modelcontextprotocol.io/)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)

## 🤝 Contributing

When contributing custom transport implementations:

1. Follow the interface contract exactly
2. Include comprehensive tests
3. Document configuration options
4. Provide usage examples
5. Consider performance implications
6. Handle errors gracefully
7. Support context cancellation
8. Implement proper cleanup

## ⚠️ Important Notes

- **Thread Safety**: All transport implementations must be thread-safe
- **Error Handling**: Distinguish between retryable and non-retryable errors
- **Resource Cleanup**: Always implement proper resource cleanup
- **Context Support**: Respect context cancellation in all operations
- **Testing**: Include both unit and integration tests
- **Documentation**: Document configuration options and usage patterns

---

This guide provides the foundation for creating custom transport implementations. Adapt these patterns and examples to your specific use case and requirements.
