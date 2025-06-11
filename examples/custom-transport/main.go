// Custom Transport Implementation Example
// This example demonstrates how to create a custom transport implementation
// for the MCP Go SDK, including WebSocket, gRPC, and message queue transports.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/examples/shared"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
	"github.com/gorilla/websocket"
)

// Example 1: WebSocket Transport Implementation
// This demonstrates creating a bidirectional WebSocket transport

type WebSocketTransport struct {
	// Connection management
	conn     *websocket.Conn
	connLock sync.RWMutex

	// Message handling
	incomingMessages chan []byte
	outgoingMessages chan []byte

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Configuration
	endpoint string
	upgrader websocket.Upgrader
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(endpoint string) *WebSocketTransport {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketTransport{
		endpoint:         endpoint,
		incomingMessages: make(chan []byte, 100),
		outgoingMessages: make(chan []byte, 100),
		ctx:              ctx,
		cancel:           cancel,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
		},
	}
}

// Implement the Transport interface

func (wst *WebSocketTransport) SendRequest(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	// Serialize request
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send via WebSocket
	select {
	case wst.outgoingMessages <- data:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-wst.ctx.Done():
		return nil, fmt.Errorf("transport closed")
	}

	// Wait for response (simplified - in practice, match request ID)
	select {
	case responseData := <-wst.incomingMessages:
		var response protocol.JSONRPCResponse
		if err := json.Unmarshal(responseData, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		return &response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-wst.ctx.Done():
		return nil, fmt.Errorf("transport closed")
	}
}

func (wst *WebSocketTransport) SendNotification(ctx context.Context, notification *protocol.JSONRPCNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	select {
	case wst.outgoingMessages <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-wst.ctx.Done():
		return fmt.Errorf("transport closed")
	}
}

func (wst *WebSocketTransport) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	select {
	case wst.outgoingMessages <- data:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-wst.ctx.Done():
		return nil, fmt.Errorf("transport closed")
	}

	// Wait for batch response
	select {
	case responseData := <-wst.incomingMessages:
		var response protocol.JSONRPCBatchResponse
		if err := json.Unmarshal(responseData, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal batch response: %w", err)
		}
		return &response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-wst.ctx.Done():
		return nil, fmt.Errorf("transport closed")
	}
}

func (wst *WebSocketTransport) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest, handler transport.RequestHandler) (*protocol.JSONRPCBatchResponse, error) {
	// Process each request in the batch
	responses := make(protocol.JSONRPCBatchResponse, 0, len(*batch))

	for _, item := range *batch {
		switch v := item.(type) {
		case *protocol.JSONRPCRequest:
			response, err := handler.HandleRequest(ctx, v)
			if err != nil {
				// Create error response
				errorResp := &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      v.ID,
					Error: &protocol.JSONRPCError{
						Code:    -32603,
						Message: err.Error(),
					},
				}
				responses = append(responses, errorResp)
			} else {
				responses = append(responses, response)
			}
		case *protocol.JSONRPCNotification:
			// Notifications don't generate responses
			if err := handler.HandleNotification(ctx, v); err != nil {
				log.Printf("Error handling notification: %v", err)
			}
		}
	}

	return &responses, nil
}

func (wst *WebSocketTransport) ReceiveMessage(ctx context.Context) (interface{}, error) {
	select {
	case data := <-wst.incomingMessages:
		// Try to parse as different message types
		var message interface{}

		// Try batch first
		var batch protocol.JSONRPCBatchRequest
		if err := json.Unmarshal(data, &batch); err == nil && len(batch) > 0 {
			return &batch, nil
		}

		// Try request
		var request protocol.JSONRPCRequest
		if err := json.Unmarshal(data, &request); err == nil && request.Method != "" {
			return &request, nil
		}

		// Try notification
		var notification protocol.JSONRPCNotification
		if err := json.Unmarshal(data, &notification); err == nil {
			return &notification, nil
		}

		// Try response
		var response protocol.JSONRPCResponse
		if err := json.Unmarshal(data, &response); err == nil {
			return &response, nil
		}

		return message, fmt.Errorf("unknown message type")

	case <-ctx.Done():
		return nil, ctx.Err()
	case <-wst.ctx.Done():
		return nil, fmt.Errorf("transport closed")
	}
}

func (wst *WebSocketTransport) Start(ctx context.Context) error {
	// Start WebSocket server
	http.HandleFunc("/ws", wst.handleWebSocket)

	server := &http.Server{
		Addr:    wst.endpoint,
		Handler: nil,
	}

	wst.wg.Add(1)
	go func() {
		defer wst.wg.Done()
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	// Handle shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	return nil
}

func (wst *WebSocketTransport) Stop() {
	wst.cancel()

	wst.connLock.Lock()
	if wst.conn != nil {
		wst.conn.Close()
	}
	wst.connLock.Unlock()

	wst.wg.Wait()
}

func (wst *WebSocketTransport) Cleanup() {
	wst.Stop()
}

func (wst *WebSocketTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wst.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	wst.connLock.Lock()
	wst.conn = conn
	wst.connLock.Unlock()

	defer func() {
		wst.connLock.Lock()
		wst.conn = nil
		wst.connLock.Unlock()
		conn.Close()
	}()

	// Start message handlers
	wst.wg.Add(2)

	// Reader goroutine
	go func() {
		defer wst.wg.Done()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				return
			}

			select {
			case wst.incomingMessages <- message:
			case <-wst.ctx.Done():
				return
			default:
				log.Printf("Incoming message buffer full, dropping message")
			}
		}
	}()

	// Writer goroutine
	go func() {
		defer wst.wg.Done()
		for {
			select {
			case message := <-wst.outgoingMessages:
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					log.Printf("WebSocket write error: %v", err)
					return
				}
			case <-wst.ctx.Done():
				return
			}
		}
	}()

	// Keep connection alive
	<-wst.ctx.Done()
}

// Example 2: Message Queue Transport (Redis Pub/Sub simulation)
// This demonstrates an asynchronous message queue-based transport

type MessageQueueTransport struct {
	// Queue configuration
	requestQueue  string
	responseQueue string

	// Message handling
	messages    map[string]chan []byte // Request ID -> Response channel
	messagesMux sync.RWMutex

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewMessageQueueTransport(requestQueue, responseQueue string) *MessageQueueTransport {
	ctx, cancel := context.WithCancel(context.Background())

	return &MessageQueueTransport{
		requestQueue:  requestQueue,
		responseQueue: responseQueue,
		messages:      make(map[string]chan []byte),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (mqt *MessageQueueTransport) SendRequest(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	// Create response channel
	responseChan := make(chan []byte, 1)
	requestID := fmt.Sprintf("%v", request.ID)

	mqt.messagesMux.Lock()
	mqt.messages[requestID] = responseChan
	mqt.messagesMux.Unlock()

	defer func() {
		mqt.messagesMux.Lock()
		delete(mqt.messages, requestID)
		mqt.messagesMux.Unlock()
		close(responseChan)
	}()

	// Simulate publishing to message queue
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("Publishing to queue %s: %s", mqt.requestQueue, string(data))

	// Wait for response
	select {
	case responseData := <-responseChan:
		var response protocol.JSONRPCResponse
		if err := json.Unmarshal(responseData, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		return &response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

func (mqt *MessageQueueTransport) SendNotification(ctx context.Context, notification *protocol.JSONRPCNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	log.Printf("Publishing notification to queue %s: %s", mqt.requestQueue, string(data))
	return nil
}

func (mqt *MessageQueueTransport) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	// For simplicity, process requests sequentially
	// In a real implementation, you might want to handle them in parallel

	responses := make(protocol.JSONRPCBatchResponse, 0, len(*batch))

	for _, item := range *batch {
		switch v := item.(type) {
		case *protocol.JSONRPCRequest:
			response, err := mqt.SendRequest(ctx, v)
			if err != nil {
				// Create error response
				errorResp := &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      v.ID,
					Error: &protocol.JSONRPCError{
						Code:    -32603,
						Message: err.Error(),
					},
				}
				responses = append(responses, errorResp)
			} else {
				responses = append(responses, response)
			}
		case *protocol.JSONRPCNotification:
			if err := mqt.SendNotification(ctx, v); err != nil {
				log.Printf("Error sending notification: %v", err)
			}
		}
	}

	return &responses, nil
}

func (mqt *MessageQueueTransport) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest, handler transport.RequestHandler) (*protocol.JSONRPCBatchResponse, error) {
	responses := make(protocol.JSONRPCBatchResponse, 0, len(*batch))

	for _, item := range *batch {
		switch v := item.(type) {
		case *protocol.JSONRPCRequest:
			response, err := handler.HandleRequest(ctx, v)
			if err != nil {
				errorResp := &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      v.ID,
					Error: &protocol.JSONRPCError{
						Code:    -32603,
						Message: err.Error(),
					},
				}
				responses = append(responses, errorResp)
			} else {
				responses = append(responses, response)
			}
		case *protocol.JSONRPCNotification:
			if err := handler.HandleNotification(ctx, v); err != nil {
				log.Printf("Error handling notification: %v", err)
			}
		}
	}

	return &responses, nil
}

func (mqt *MessageQueueTransport) ReceiveMessage(ctx context.Context) (interface{}, error) {
	// Simulate receiving from message queue
	// In a real implementation, this would listen to the queue

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// Return nil to indicate no message (polling pattern)
		return nil, nil
	}
}

func (mqt *MessageQueueTransport) Start(ctx context.Context) error {
	log.Printf("Message queue transport started (request: %s, response: %s)",
		mqt.requestQueue, mqt.responseQueue)

	// Start response listener
	mqt.wg.Add(1)
	go mqt.responseListener()

	return nil
}

func (mqt *MessageQueueTransport) Stop() {
	mqt.cancel()
	mqt.wg.Wait()
}

func (mqt *MessageQueueTransport) Cleanup() {
	mqt.Stop()
}

func (mqt *MessageQueueTransport) responseListener() {
	defer mqt.wg.Done()

	// Simulate listening for responses
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Simulate receiving responses from queue
			// In a real implementation, this would use Redis BLPOP or similar
		case <-mqt.ctx.Done():
			return
		}
	}
}

// Example 3: Custom Transport Registration and Usage

func demonstrateCustomTransports() {
	log.Println("=== Custom Transport Implementation Demo ===")

	// Example 1: WebSocket Transport
	log.Println("\n1. WebSocket Transport Example")
	wsTransport := NewWebSocketTransport(":8080")

	// Create server with WebSocket transport
	wsServer := server.New(wsTransport,
		server.WithName("WebSocket MCP Server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(shared.CreateToolsProvider()),
		server.WithResourcesProvider(shared.CreateResourcesProvider()),
		server.WithPromptsProvider(shared.CreatePromptsProvider()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start WebSocket server
	if err := wsTransport.Start(ctx); err != nil {
		log.Printf("Failed to start WebSocket transport: %v", err)
	}

	if err := wsServer.Start(ctx); err != nil {
		log.Printf("Failed to start WebSocket server: %v", err)
	}

	log.Println("WebSocket server started on :8080/ws")

	// Example 2: Message Queue Transport
	log.Println("\n2. Message Queue Transport Example")
	mqTransport := NewMessageQueueTransport("mcp-requests", "mcp-responses")

	mqServer := server.New(mqTransport,
		server.WithName("Message Queue MCP Server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(shared.CreateToolsProvider()),
		server.WithResourcesProvider(shared.CreateResourcesProvider()),
		server.WithPromptsProvider(shared.CreatePromptsProvider()),
	)

	if err := mqTransport.Start(ctx); err != nil {
		log.Printf("Failed to start message queue transport: %v", err)
	}

	if err := mqServer.Start(ctx); err != nil {
		log.Printf("Failed to start message queue server: %v", err)
	}

	log.Println("Message queue server started")

	// Demonstration of sending messages
	log.Println("\n3. Testing Custom Transports")

	// Create a sample request
	request, _ := protocol.NewRequest("1", "listTools", json.RawMessage(`{}`))

	// Test WebSocket transport (would need client connection)
	log.Println("WebSocket transport ready for connections at ws://localhost:8080/ws")

	// Test message queue transport
	log.Println("Message queue transport ready for messages")

	// Cleanup
	time.Sleep(2 * time.Second)
	wsTransport.Stop()
	mqTransport.Stop()
	wsServer.Stop()
	mqServer.Stop()
}

// Example 4: Transport Factory Pattern
// This demonstrates how to create a factory for different transport types

type TransportType string

const (
	TransportTypeWebSocket    TransportType = "websocket"
	TransportTypeMessageQueue TransportType = "messagequeue"
	TransportTypeCustomHTTP   TransportType = "custom-http"
)

type CustomTransportConfig struct {
	Type     TransportType          `json:"type"`
	Endpoint string                 `json:"endpoint"`
	Options  map[string]interface{} `json:"options"`
}

func CreateCustomTransport(config CustomTransportConfig) (transport.Transport, error) {
	switch config.Type {
	case TransportTypeWebSocket:
		return NewWebSocketTransport(config.Endpoint), nil

	case TransportTypeMessageQueue:
		requestQueue := "mcp-requests"
		responseQueue := "mcp-responses"

		if rq, ok := config.Options["request_queue"].(string); ok {
			requestQueue = rq
		}
		if rq, ok := config.Options["response_queue"].(string); ok {
			responseQueue = rq
		}

		return NewMessageQueueTransport(requestQueue, responseQueue), nil

	case TransportTypeCustomHTTP:
		// Could implement custom HTTP transport with different features
		// For now, fall back to standard transport
		stdConfig := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
		stdConfig.Endpoint = config.Endpoint
		return transport.NewTransport(stdConfig)

	default:
		return nil, fmt.Errorf("unknown transport type: %s", config.Type)
	}
}

func demonstrateTransportFactory() {
	log.Println("\n=== Transport Factory Pattern Demo ===")

	configs := []CustomTransportConfig{
		{
			Type:     TransportTypeWebSocket,
			Endpoint: ":8081",
			Options:  map[string]interface{}{},
		},
		{
			Type:     TransportTypeMessageQueue,
			Endpoint: "",
			Options: map[string]interface{}{
				"request_queue":  "custom-requests",
				"response_queue": "custom-responses",
			},
		},
		{
			Type:     TransportTypeCustomHTTP,
			Endpoint: "http://localhost:8082/mcp",
			Options:  map[string]interface{}{},
		},
	}

	for i, config := range configs {
		log.Printf("Creating transport %d: %s", i+1, config.Type)

		transport, err := CreateCustomTransport(config)
		if err != nil {
			log.Printf("Failed to create transport: %v", err)
			continue
		}

		// Create server with the transport
		s := server.New(transport,
			server.WithName(fmt.Sprintf("Custom Server %d", i+1)),
			server.WithVersion("1.0.0"),
		)

		log.Printf("Server created with %s transport", config.Type)

		// Start and stop for demonstration
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

		if err := transport.Start(ctx); err != nil {
			log.Printf("Failed to start transport: %v", err)
		}

		if err := s.Start(ctx); err != nil {
			log.Printf("Failed to start server: %v", err)
		}

		time.Sleep(500 * time.Millisecond)

		transport.Stop()
		s.Stop()
		cancel()

		log.Printf("Transport %s stopped", config.Type)
	}
}

func main() {
	log.Println("Custom Transport Implementation Examples")
	log.Println("=======================================")

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nReceived shutdown signal...")
		cancel()
	}()

	// Run demonstrations
	demonstrateCustomTransports()
	demonstrateTransportFactory()

	log.Println("\n=== Custom Transport Examples Complete ===")
	log.Println("See the source code for implementation details and patterns.")
	log.Println("Adapt these examples for your specific transport requirements.")
}
