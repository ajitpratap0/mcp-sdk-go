package server

import (
	"context"
	"encoding/json"
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
	requestHandlers      map[string]transport.RequestHandler
	notificationHandlers map[string]transport.NotificationHandler
	progressHandlers     map[string]transport.ProgressHandler
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		sentRequests:         make(map[string]interface{}),
		sentNotifications:    make(map[string]interface{}),
		requestHandlers:      make(map[string]transport.RequestHandler),
		notificationHandlers: make(map[string]transport.NotificationHandler),
		progressHandlers:     make(map[string]transport.ProgressHandler),
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

// Custom mock implementations for providers
type mockToolsProvider struct {
	tools []protocol.Tool
	err   error
}

func (m *mockToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	if m.err != nil {
		return nil, 0, "", false, m.err
	}
	return m.tools, len(m.tools), "", false, nil
}

func (m *mockToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, toolContext json.RawMessage) (*protocol.CallToolResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	resultJSON := []byte(`{"message": "success"}`)
	return &protocol.CallToolResult{
		Result: json.RawMessage(resultJSON),
	}, nil
}

// mockResourcesProvider is defined in providers_test.go

// Tests for server creation and options
func TestNewServer(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.name != "go-mcp-server" {
		t.Errorf("Expected default server name to be 'go-mcp-server', got %q", server.name)
	}

	if server.version != "1.0.0" {
		t.Errorf("Expected default server version to be '1.0.0', got %q", server.version)
	}

	// Check that default capabilities are set
	if len(server.capabilities) != 0 {
		t.Errorf("Expected no default capabilities, got %d", len(server.capabilities))
	}

	// Check that request handlers are registered
	if len(mt.requestHandlers) == 0 {
		t.Error("Expected request handlers to be registered")
	}

	// Verify specific handlers
	if _, ok := mt.requestHandlers[string(protocol.MethodInitialize)]; !ok {
		t.Error("Expected initialize handler to be registered")
	}

	if _, ok := mt.requestHandlers[string(protocol.MethodCancel)]; !ok {
		t.Error("Expected cancel handler to be registered")
	}
}

func TestServerOptions(t *testing.T) {
	mt := newMockTransport()
	testName := "test-server"
	testVersion := "2.0.0"
	testDescription := "Test server description"
	testHomepage := "https://example.com"

	toolsProvider := &mockToolsProvider{
		tools: []protocol.Tool{
			{
				Name:        "test-tool",
				Description: "A test tool",
			},
		},
	}

	server := New(mt,
		WithName(testName),
		WithVersion(testVersion),
		WithDescription(testDescription),
		WithHomepage(testHomepage),
		WithCapability(protocol.CapabilityTools, true),
		WithToolsProvider(toolsProvider),
		WithFeatureOptions(map[string]interface{}{
			"test": "value",
		}),
	)

	if server.name != testName {
		t.Errorf("Expected server name to be %q, got %q", testName, server.name)
	}

	if server.version != testVersion {
		t.Errorf("Expected server version to be %q, got %q", testVersion, server.version)
	}

	if server.description != testDescription {
		t.Errorf("Expected server description to be %q, got %q", testDescription, server.description)
	}

	if server.homepage != testHomepage {
		t.Errorf("Expected server homepage to be %q, got %q", testHomepage, server.homepage)
	}

	if !server.capabilities[string(protocol.CapabilityTools)] {
		t.Error("Expected tools capability to be enabled")
	}

	if val, ok := server.featureOptions["test"]; !ok || val != "value" {
		t.Error("Expected feature option 'test' to be set to 'value'")
	}

	// Check that the tools provider was set and the tools handler was registered
	if server.toolsProvider == nil {
		t.Error("Expected tools provider to be set")
	}

	if _, ok := mt.requestHandlers[string(protocol.MethodListTools)]; !ok {
		t.Error("Expected ListTools handler to be registered")
	}
}

// Test server initialization handler
func TestHandleInitialize(t *testing.T) {
	mt := newMockTransport()
	server := New(mt, WithName("test-server"), WithVersion("1.0.0"))

	// Create initialize request
	initializeParams := &protocol.InitializeParams{
		ProtocolVersion: protocol.ProtocolRevision,
		Name:            "test-client",
		Version:         "1.0.0",
		Capabilities: map[string]bool{
			string(protocol.CapabilityTools): true,
		},
		ClientInfo: &protocol.ClientInfo{
			Name:     "test-client",
			Version:  "1.0.0",
			Platform: "test",
		},
	}

	// Convert to JSON for the mock handler
	paramsJSON, _ := json.Marshal(initializeParams)

	// Call the handler
	result, err := server.handleInitialize(context.Background(), json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("Expected handleInitialize to succeed, got error: %v", err)
	}

	// Check the result
	initResult, ok := result.(*protocol.InitializeResult)
	if !ok {
		t.Fatalf("Expected result to be *protocol.InitializeResult, got %T", result)
	}

	if initResult.ServerInfo == nil {
		t.Fatal("Expected ServerInfo to not be nil")
	}

	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("Expected ServerInfo.Name to be 'test-server', got %q", initResult.ServerInfo.Name)
	}

	if initResult.ServerInfo.Version != "1.0.0" {
		t.Errorf("Expected ServerInfo.Version to be '1.0.0', got %q", initResult.ServerInfo.Version)
	}

	// Test that the server is now initialized
	if !server.isInitialized() {
		t.Error("Expected server to be marked as initialized")
	}

	// Check that client info was stored
	if server.clientInfo == nil {
		t.Fatal("Expected ClientInfo to be stored")
	}

	if server.clientInfo.Name != "test-client" {
		t.Errorf("Expected ClientInfo.Name to be 'test-client', got %q", server.clientInfo.Name)
	}
}

// Test notification handlers
func TestNotifyToolsChanged(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	// Mark server as initialized
	server.initializedLock.Lock()
	server.initialized = true
	server.initializedLock.Unlock()

	// Test notification
	tools := []protocol.Tool{
		{
			Name:        "test-tool",
			Description: "A test tool",
		},
	}

	err := server.NotifyToolsChanged(tools, nil, nil)
	if err != nil {
		t.Fatalf("Expected NotifyToolsChanged to succeed, got error: %v", err)
	}

	// Check that notification was sent
	if _, ok := mt.sentNotifications[string(protocol.MethodToolsChanged)]; !ok {
		t.Error("Expected toolsChanged notification to be sent")
	}

	// Test notification with server not initialized
	server.initializedLock.Lock()
	server.initialized = false
	server.initializedLock.Unlock()

	err = server.NotifyToolsChanged(tools, nil, nil)
	if err == nil {
		t.Error("Expected NotifyToolsChanged to fail when server not initialized")
	}
}

// Test server start and stop
func TestServerStartStop(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	// Test start
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := server.Start(ctx)
	if err != nil && err != context.Canceled {
		t.Errorf("Expected Start to be cancelled, got error: %v", err)
	}

	// Test stop
	err = server.Stop()
	if err != nil {
		t.Errorf("Expected Stop to succeed, got error: %v", err)
	}

	// Test stop with error
	mt.stopErr = errors.New("stop error")
	err = server.Stop()
	if err == nil {
		t.Error("Expected Stop to return error")
	}
}

// Test utility methods
func TestParseParams(t *testing.T) {
	// Test with valid params
	paramsJSON := `{"name": "test", "value": 42}`
	var target struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err := parseParams(json.RawMessage(paramsJSON), &target)
	if err != nil {
		t.Fatalf("Expected parseParams to succeed, got error: %v", err)
	}

	if target.Name != "test" {
		t.Errorf("Expected Name to be 'test', got %q", target.Name)
	}

	if target.Value != 42 {
		t.Errorf("Expected Value to be 42, got %d", target.Value)
	}

	// Test with invalid params
	err = parseParams(json.RawMessage(`{"name": "test", "value": "not-a-number"}`), &target)
	if err == nil {
		t.Error("Expected parseParams to fail with invalid params")
	}

	// Test with nil params
	err = parseParams(nil, &target)
	if err == nil {
		t.Error("Expected parseParams to fail with nil params")
	}
}
