package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// mockTransport implements the transport.Transport interface for testing
type mockTransport struct {
	initialized          bool
	initializeErr        error
	startErr             error
	stopErr              error
	sendRequestResponse  interface{}
	sendRequestErr       error
	sentRequests         map[string]interface{}
	sentNotifications    map[string]interface{}
	requestHandlers      map[string]func(context.Context, interface{}) (interface{}, error)
	notificationHandlers map[string]func(context.Context, interface{}) error
	progressHandlers     map[string]func(interface{}) error
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		sentRequests:         make(map[string]interface{}),
		sentNotifications:    make(map[string]interface{}),
		requestHandlers:      make(map[string]func(context.Context, interface{}) (interface{}, error)),
		notificationHandlers: make(map[string]func(context.Context, interface{}) error),
		progressHandlers:     make(map[string]func(interface{}) error),
	}
}

func (m *mockTransport) Initialize(ctx context.Context) error {
	if m.initializeErr != nil {
		return m.initializeErr
	}
	m.initialized = true
	return nil
}

func (m *mockTransport) Start(ctx context.Context) error {
	return m.startErr
}

func (m *mockTransport) Stop(ctx context.Context) error {
	return m.stopErr
}

func (m *mockTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	m.sentRequests[method] = params
	return m.sendRequestResponse, m.sendRequestErr
}

func (m *mockTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.sentNotifications[method] = params
	return nil
}

func (m *mockTransport) RegisterRequestHandler(method string, handler transport.RequestHandler) {
	m.requestHandlers[method] = handler
}

func (m *mockTransport) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	m.notificationHandlers[method] = handler
}

func (m *mockTransport) RegisterProgressHandler(id interface{}, handler transport.ProgressHandler) {
	m.progressHandlers[id.(string)] = handler
}

func (m *mockTransport) UnregisterProgressHandler(id interface{}) {
	delete(m.progressHandlers, id.(string))
}

func (m *mockTransport) GenerateID() string {
	return "test-id"
}

// Send implements the transport.Transport interface for basic message sending
func (m *mockTransport) Send(data []byte) error {
	return nil
}

// SetReceiveHandler sets the handler for received messages
func (m *mockTransport) SetReceiveHandler(handler transport.ReceiveHandler) {
	// No-op for mock
}

// SetErrorHandler sets the handler for errors
func (m *mockTransport) SetErrorHandler(handler transport.ErrorHandler) {
	// No-op for mock
}

// Tests for client creation and options
func TestNewClient(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.name != "go-mcp-client" {
		t.Errorf("Expected default client name to be 'go-mcp-client', got %q", client.name)
	}

	if client.version != "1.0.0" {
		t.Errorf("Expected default client version to be '1.0.0', got %q", client.version)
	}

	if _, ok := client.capabilities[string(protocol.CapabilitySampling)]; !ok || !client.capabilities[string(protocol.CapabilitySampling)] {
		t.Error("Expected default sampling capability to be enabled")
	}
}

func TestClientOptions(t *testing.T) {
	mt := newMockTransport()
	testName := "test-client"
	testVersion := "2.0.0"

	client := New(mt,
		WithName(testName),
		WithVersion(testVersion),
		WithCapability(protocol.CapabilityTools, true),
		WithFeatureOptions(map[string]interface{}{
			"test": "value",
		}),
	)

	if client.name != testName {
		t.Errorf("Expected client name to be %q, got %q", testName, client.name)
	}

	if client.version != testVersion {
		t.Errorf("Expected client version to be %q, got %q", testVersion, client.version)
	}

	if !client.capabilities[string(protocol.CapabilityTools)] {
		t.Error("Expected tools capability to be enabled")
	}

	if val, ok := client.featureOptions["test"]; !ok || val != "value" {
		t.Error("Expected feature option 'test' to be set to 'value'")
	}
}

// Test client initialization
func TestClientInitialize(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Mock the transport to return a valid initialize result
	serverInfo := &protocol.ServerInfo{
		Name:        "test-server",
		Version:     "1.0.0",
		Description: "Test server",
		Homepage:    "http://example.com",
	}

	mt.sendRequestResponse = &protocol.InitializeResult{
		ServerInfo: serverInfo,
	}

	// Initialize the client
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Expected Initialize to succeed, got error: %v", err)
	}

	// Check that the client sent the initialize request
	if _, ok := mt.sentRequests[string(protocol.MethodInitialize)]; !ok {
		t.Error("Expected client to send initialize request")
	}

	// Check that the client sent the initialized notification
	if _, ok := mt.sentNotifications[string(protocol.MethodInitialized)]; !ok {
		t.Error("Expected client to send initialized notification")
	}

	// Check that the client stored the server info
	if client.serverInfo == nil {
		t.Fatal("Expected client to store server info, got nil")
	}

	if client.serverInfo.Name != serverInfo.Name {
		t.Errorf("Expected server name to be %q, got %q", serverInfo.Name, client.serverInfo.Name)
	}

	// Test that the client is now initialized
	client.initializedLock.RLock()
	initialized := client.initialized
	client.initializedLock.RUnlock()
	if !initialized {
		t.Error("Expected client to be marked as initialized")
	}
}

func TestClientInitializeError(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Set up transport to fail initialization
	mt.initializeErr = errors.New("initialize error")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Initialize(ctx)
	if err == nil {
		t.Fatal("Expected Initialize to fail with transport error")
	}

	// Test error during request sending
	mt.initializeErr = nil
	mt.sendRequestErr = errors.New("request error")

	err = client.Initialize(ctx)
	if err == nil {
		t.Fatal("Expected Initialize to fail with request error")
	}

	// Test error parsing initialize result
	mt.sendRequestErr = nil
	mt.sendRequestResponse = "invalid json"

	err = client.Initialize(ctx)
	if err == nil {
		t.Fatal("Expected Initialize to fail with parsing error")
	}
}

func TestClientHasCapability(t *testing.T) {
	mt := newMockTransport()
	client := New(mt, WithCapability(protocol.CapabilityTools, true))

	// No ServerInfo yet
	if client.HasCapability(protocol.CapabilityTools) {
		t.Error("Expected HasCapability to return false before initialization")
	}

	// Initialize with server info
	client.serverInfo = &protocol.ServerInfo{
		Name:    "test-server",
		Version: "1.0.0",
	}

	// The client's capability is already set to true in the New call above
	if !client.HasCapability(protocol.CapabilityTools) {
		t.Error("Expected HasCapability to return true for enabled capability")
	}

	// This capability was not enabled
	if client.HasCapability(protocol.CapabilityResources) {
		t.Error("Expected HasCapability to return false for disabled capability")
	}
}

// Test that a client properly closes
func TestClientClose(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	err := client.Close()
	if err != nil {
		t.Errorf("Expected Close to succeed, got error: %v", err)
	}

	// Test close with error
	mt.stopErr = errors.New("stop error")
	err = client.Close()
	if err == nil {
		t.Fatal("Expected Close to fail with stop error")
	}
}
