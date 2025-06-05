package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
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
	sendRequestFunc      func(ctx context.Context, method string, params interface{}) (interface{}, error)
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
	if m.sendRequestFunc != nil {
		return m.sendRequestFunc(ctx, method, params)
	}
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
	// Create a protocol.Response with invalid JSON in the Result field
	mt.sendRequestResponse = &protocol.Response{
		JSONRPCMessage: protocol.JSONRPCMessage{JSONRPC: protocol.JSONRPCVersion},
		ID:             "test-id",
		Result:         json.RawMessage("invalid json"),
	}

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

// TestListTools tests the ListTools method
func TestListTools(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityTools)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful list
	mt.sendRequestResponse = &protocol.ListToolsResult{
		Tools: []protocol.Tool{
			{Name: "tool1", Description: "Tool 1"},
			{Name: "tool2", Description: "Tool 2"},
		},
		PaginationResult: protocol.PaginationResult{
			NextCursor: "cursor123",
			HasMore:    true,
		},
	}

	tools, pagination, err := client.ListTools(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("Expected ListTools to succeed, got error: %v", err)
	}

	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	if pagination == nil || pagination.NextCursor != "cursor123" {
		t.Error("Expected pagination cursor to be 'cursor123'")
	}

	// Test with category filter
	_, _, err = client.ListTools(context.Background(), "test-category", nil)
	if err != nil {
		t.Errorf("Expected ListTools with category to succeed, got error: %v", err)
	}

	// Verify request params
	if sentParams, ok := mt.sentRequests[protocol.MethodListTools]; ok {
		params := sentParams.(*protocol.ListToolsParams)
		if params.Category != "test-category" {
			t.Errorf("Expected category 'test-category', got %s", params.Category)
		}
	}

	// Test when server doesn't support tools
	client.capabilities[string(protocol.CapabilityTools)] = false
	_, _, err = client.ListTools(context.Background(), "", nil)
	if err == nil || err.Error() != "server does not support tools" {
		t.Error("Expected error when server doesn't support tools")
	}
}

// TestCallTool tests the CallTool method
func TestCallTool(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityTools)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful tool call
	mt.sendRequestResponse = &protocol.CallToolResult{
		Result: json.RawMessage(`{"output": "success"}`),
	}

	result, err := client.CallTool(context.Background(), "test-tool", map[string]interface{}{"param": "value"}, nil)
	if err != nil {
		t.Fatalf("Expected CallTool to succeed, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Test tool call with error response
	mt.sendRequestResponse = &protocol.CallToolResult{
		Error: "Tool execution failed",
	}

	result, err = client.CallTool(context.Background(), "error-tool", nil, nil)
	if err != nil {
		t.Fatalf("Expected CallTool to succeed even with error in result, got error: %v", err)
	}

	if result.Error != "Tool execution failed" {
		t.Errorf("Expected error 'Tool execution failed', got %s", result.Error)
	}

	// Test when server doesn't support tools
	client.capabilities[string(protocol.CapabilityTools)] = false
	_, err = client.CallTool(context.Background(), "test-tool", nil, nil)
	if err == nil || err.Error() != "server does not support tools" {
		t.Error("Expected error when server doesn't support tools")
	}
}

// TestListResources tests the ListResources method
func TestListResources(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityResources)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful list
	mt.sendRequestResponse = &protocol.ListResourcesResult{
		Resources: []protocol.Resource{
			{URI: "resource1", Name: "Resource 1"},
			{URI: "resource2", Name: "Resource 2"},
		},
		Templates: []protocol.ResourceTemplate{
			{URI: "template1", Name: "Template 1"},
		},
	}

	resources, templates, pagination, err := client.ListResources(context.Background(), "", false, nil)
	if err != nil {
		t.Fatalf("Expected ListResources to succeed, got error: %v", err)
	}

	if len(resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resources))
	}

	if len(templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(templates))
	}

	// Verify pagination is returned
	if pagination == nil {
		t.Log("No pagination result returned")
	}

	// Test with URI filter and recursive
	_, _, _, err = client.ListResources(context.Background(), "folder/", true, nil)
	if err != nil {
		t.Errorf("Expected ListResources with URI to succeed, got error: %v", err)
	}

	// Verify request params
	if sentParams, ok := mt.sentRequests[protocol.MethodListResources]; ok {
		params := sentParams.(*protocol.ListResourcesParams)
		if params.URI != "folder/" {
			t.Errorf("Expected URI 'folder/', got %s", params.URI)
		}
		if !params.Recursive {
			t.Error("Expected recursive to be true")
		}
	}

	// Test when server doesn't support resources
	client.capabilities[string(protocol.CapabilityResources)] = false
	_, _, _, err = client.ListResources(context.Background(), "", false, nil)
	if err == nil || err.Error() != "server does not support resources" {
		t.Error("Expected error when server doesn't support resources")
	}
}

// TestReadResource tests the ReadResource method
func TestReadResource(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityResources)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful read
	mt.sendRequestResponse = &protocol.ReadResourceResult{
		Contents: protocol.ResourceContents{
			URI:     "file.txt",
			Type:    "text/plain",
			Content: json.RawMessage(`"Hello, world!"`),
		},
	}

	contents, err := client.ReadResource(context.Background(), "file.txt", nil, nil)
	if err != nil {
		t.Fatalf("Expected ReadResource to succeed, got error: %v", err)
	}

	if contents == nil {
		t.Fatal("Expected contents, got nil")
	}

	if contents.URI != "file.txt" {
		t.Errorf("Expected URI 'file.txt', got %s", contents.URI)
	}

	// Test with template params
	templateParams := map[string]interface{}{"name": "Alice"}
	_, err = client.ReadResource(context.Background(), "template://greeting", templateParams, nil)
	if err != nil {
		t.Errorf("Expected ReadResource with template params to succeed, got error: %v", err)
	}

	// Test when server doesn't support resources
	client.capabilities[string(protocol.CapabilityResources)] = false
	_, err = client.ReadResource(context.Background(), "file.txt", nil, nil)
	if err == nil || err.Error() != "server does not support resources" {
		t.Error("Expected error when server doesn't support resources")
	}
}

// TestListPrompts tests the ListPrompts method
func TestListPrompts(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityPrompts)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful list
	mt.sendRequestResponse = &protocol.ListPromptsResult{
		Prompts: []protocol.Prompt{
			{ID: "prompt1", Name: "Prompt 1"},
			{ID: "prompt2", Name: "Prompt 2"},
		},
	}

	prompts, pagination, err := client.ListPrompts(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("Expected ListPrompts to succeed, got error: %v", err)
	}

	if len(prompts) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(prompts))
	}

	// Verify pagination is returned
	if pagination == nil {
		t.Log("No pagination result returned")
	}

	// Test with tag filter
	_, _, err = client.ListPrompts(context.Background(), "greeting", nil)
	if err != nil {
		t.Errorf("Expected ListPrompts with tag to succeed, got error: %v", err)
	}

	// Verify request params
	if sentParams, ok := mt.sentRequests[protocol.MethodListPrompts]; ok {
		params := sentParams.(*protocol.ListPromptsParams)
		if params.Tag != "greeting" {
			t.Errorf("Expected tag 'greeting', got %s", params.Tag)
		}
	}

	// Test when server doesn't support prompts
	client.capabilities[string(protocol.CapabilityPrompts)] = false
	_, _, err = client.ListPrompts(context.Background(), "", nil)
	if err == nil || err.Error() != "server does not support prompts" {
		t.Error("Expected error when server doesn't support prompts")
	}
}

// TestGetPrompt tests the GetPrompt method
func TestGetPrompt(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityPrompts)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful get
	mt.sendRequestResponse = &protocol.GetPromptResult{
		Prompt: protocol.Prompt{
			ID:   "greeting",
			Name: "Greeting Prompt",
			Messages: []protocol.PromptMessage{
				{Role: "user", Content: "Say hello"},
			},
		},
	}

	prompt, err := client.GetPrompt(context.Background(), "greeting")
	if err != nil {
		t.Fatalf("Expected GetPrompt to succeed, got error: %v", err)
	}

	if prompt == nil {
		t.Fatal("Expected prompt, got nil")
	}

	if prompt.ID != "greeting" {
		t.Errorf("Expected prompt ID 'greeting', got %s", prompt.ID)
	}

	if len(prompt.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(prompt.Messages))
	}

	// Test when server doesn't support prompts
	client.capabilities[string(protocol.CapabilityPrompts)] = false
	_, err = client.GetPrompt(context.Background(), "greeting")
	if err == nil || err.Error() != "server does not support prompts" {
		t.Error("Expected error when server doesn't support prompts")
	}
}

// TestStart tests the Start method
func TestStart(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	err := client.Start(context.Background())
	if err != nil {
		t.Errorf("Expected Start to succeed, got error: %v", err)
	}

	// Test with transport error
	mt.startErr = errors.New("start failed")
	err = client.Start(context.Background())
	if err == nil || err.Error() != "start failed" {
		t.Error("Expected Start to fail with transport error")
	}
}

// TestServerInfo tests the ServerInfo method
func TestServerInfo(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Initially nil
	info := client.ServerInfo()
	if info != nil {
		t.Error("Expected ServerInfo to be nil before initialization")
	}

	// Set server info
	client.serverInfo = &protocol.ServerInfo{
		Name:    "test-server",
		Version: "1.0.0",
	}

	info = client.ServerInfo()
	if info == nil {
		t.Fatal("Expected ServerInfo, got nil")
	}

	if info.Name != "test-server" {
		t.Errorf("Expected server name 'test-server', got %s", info.Name)
	}
}

// TestCapabilities tests the Capabilities method
func TestCapabilities(t *testing.T) {
	mt := newMockTransport()
	client := New(mt,
		WithCapability(protocol.CapabilityTools, true),
		WithCapability(protocol.CapabilityResources, true),
	)

	caps := client.Capabilities()
	if caps == nil {
		t.Fatal("Expected capabilities map, got nil")
	}

	if !caps[string(protocol.CapabilityTools)] {
		t.Error("Expected tools capability to be true")
	}

	if !caps[string(protocol.CapabilityResources)] {
		t.Error("Expected resources capability to be true")
	}
}

// TestSubscribeResource tests the SubscribeResource method
func TestSubscribeResource(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityResources)] = true
	client.capabilities[string(protocol.CapabilityResourceSubscriptions)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful subscription
	mt.sendRequestResponse = &protocol.SubscribeResourceResult{
		Success: true,
	}

	err := client.SubscribeResource(context.Background(), "resource1", false)
	if err != nil {
		t.Fatalf("Expected SubscribeResource to succeed, got error: %v", err)
	}

	// Test when server doesn't support subscriptions
	client.capabilities[string(protocol.CapabilityResourceSubscriptions)] = false
	err = client.SubscribeResource(context.Background(), "resource1", false)
	if err == nil || err.Error() != "server does not support resource subscriptions" {
		t.Error("Expected error when server doesn't support subscriptions")
	}
}

// TestListRoots tests the ListRoots method
func TestListRoots(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityRoots)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful list
	mt.sendRequestResponse = &protocol.ListRootsResult{
		Roots: []protocol.Root{
			{ID: "root1", Name: "Root 1"},
			{ID: "root2", Name: "Root 2"},
		},
	}

	roots, pagination, err := client.ListRoots(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("Expected ListRoots to succeed, got error: %v", err)
	}

	if len(roots) != 2 {
		t.Errorf("Expected 2 roots, got %d", len(roots))
	}

	// Verify pagination is returned
	if pagination == nil {
		t.Log("No pagination result returned")
	}

	// Test when server doesn't support roots
	client.capabilities[string(protocol.CapabilityRoots)] = false
	_, _, err = client.ListRoots(context.Background(), "", nil)
	if err == nil || err.Error() != "server does not support roots" {
		t.Error("Expected error when server doesn't support roots")
	}
}

// TestSetLogLevel tests the SetLogLevel method
func TestSetLogLevel(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityLogging)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful set log level
	mt.sendRequestResponse = &protocol.SetLogLevelResult{
		Success: true,
	}

	err := client.SetLogLevel(context.Background(), protocol.LogLevelDebug)
	if err != nil {
		t.Fatalf("Expected SetLogLevel to succeed, got error: %v", err)
	}

	// Verify request params
	if sentParams, ok := mt.sentRequests[protocol.MethodSetLogLevel]; ok {
		params := sentParams.(*protocol.SetLogLevelParams)
		if params.Level != protocol.LogLevelDebug {
			t.Errorf("Expected log level debug, got %v", params.Level)
		}
	}

	// Test when server doesn't support logging
	client.capabilities[string(protocol.CapabilityLogging)] = false
	err = client.SetLogLevel(context.Background(), protocol.LogLevelDebug)
	if err == nil || err.Error() != "server does not support logging" {
		t.Error("Expected error when server doesn't support logging")
	}
}

// TestCancel tests the Cancel method
func TestCancel(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful cancel
	mt.sendRequestResponse = &protocol.CancelResult{
		Cancelled: true,
	}

	cancelled, err := client.Cancel(context.Background(), "request-123")
	if err != nil {
		t.Fatalf("Expected Cancel to succeed, got error: %v", err)
	}

	if !cancelled {
		t.Error("Expected cancellation to be successful")
	}

	// Verify request params
	if sentParams, ok := mt.sentRequests[protocol.MethodCancel]; ok {
		params := sentParams.(*protocol.CancelParams)
		if params.ID != "request-123" {
			t.Errorf("Expected request ID 'request-123', got %v", params.ID)
		}
	}
}

// TestPing tests the Ping method
func TestPing(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful ping
	expectedTimestamp := time.Now().UnixNano() / int64(time.Millisecond)
	mt.sendRequestResponse = &protocol.PingResult{
		Timestamp: expectedTimestamp,
	}

	result, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Expected Ping to succeed, got error: %v", err)
	}

	if result.Timestamp != expectedTimestamp {
		t.Errorf("Expected timestamp %d, got %d", expectedTimestamp, result.Timestamp)
	}
}

// TestSendProgress tests the SendProgress method
func TestSendProgress(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test send progress
	err := client.SendProgress(context.Background(), "request-123", "Processing...", 50, false)
	if err != nil {
		t.Fatalf("Expected SendProgress to succeed, got error: %v", err)
	}

	// Verify notification was sent
	if sentParams, ok := mt.sentNotifications[protocol.MethodProgress]; ok {
		params := sentParams.(*protocol.ProgressParams)
		if params.Message != "Processing..." {
			t.Errorf("Expected message 'Processing...', got %s", params.Message)
		}
		if params.Percent != 50 {
			t.Errorf("Expected percent 50, got %f", params.Percent)
		}
		if params.Completed {
			t.Error("Expected completed to be false")
		}
	} else {
		t.Error("Expected progress notification to be sent")
	}
}

// TestInitializeAndStart tests the InitializeAndStart method
func TestInitializeAndStart(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Set up successful initialize response
	mt.sendRequestResponse = &protocol.InitializeResult{
		ProtocolVersion: "1.0",
		ServerInfo: &protocol.ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
		Capabilities: map[string]bool{
			"tools": true,
		},
	}

	err := client.InitializeAndStart(context.Background())
	if err != nil {
		t.Fatalf("Expected InitializeAndStart to succeed, got error: %v", err)
	}

	// Verify client is initialized
	if !client.initialized {
		t.Error("Expected client to be initialized")
	}

	// Test with initialize error
	client2 := New(newMockTransport())
	client2.transport.(*mockTransport).sendRequestErr = errors.New("init failed")

	err = client2.InitializeAndStart(context.Background())
	if err == nil || !strings.Contains(err.Error(), "init failed") {
		t.Error("Expected InitializeAndStart to fail with init error")
	}

	// Test with start error
	client3 := New(newMockTransport())
	client3.transport.(*mockTransport).sendRequestResponse = &protocol.InitializeResult{
		ProtocolVersion: "1.0",
		ServerInfo: &protocol.ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
		Capabilities: map[string]bool{
			"tools": true,
		},
	}
	client3.transport.(*mockTransport).startErr = errors.New("start failed")

	err = client3.InitializeAndStart(context.Background())
	if err == nil || err.Error() != "start failed" {
		t.Error("Expected InitializeAndStart to fail with start error")
	}
}

// TestHandleToolsChanged tests the handleToolsChanged notification handler
func TestHandleToolsChanged(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"added": [{"name": "tool1", "description": "Tool 1"}, {"name": "tool2", "description": "Tool 2"}]}`)
	err := client.handleToolsChanged(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handleToolsChanged to succeed, got error: %v", err)
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	err = client.handleToolsChanged(context.Background(), invalidParams)
	if err == nil {
		t.Error("Expected handleToolsChanged to fail with invalid params")
	}
}

// TestHandleResourcesChanged tests the handleResourcesChanged notification handler
func TestHandleResourcesChanged(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"uri": "test://", "resources": [{"uri": "res1", "type": "text"}, {"uri": "res2", "type": "text"}]}`)
	err := client.handleResourcesChanged(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handleResourcesChanged to succeed, got error: %v", err)
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	err = client.handleResourcesChanged(context.Background(), invalidParams)
	if err == nil {
		t.Error("Expected handleResourcesChanged to fail with invalid params")
	}
}

// TestHandleResourceUpdated tests the handleResourceUpdated notification handler
func TestHandleResourceUpdated(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"uri": "file://test.txt", "resource": {"content": "updated"}}`)
	err := client.handleResourceUpdated(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handleResourceUpdated to succeed, got error: %v", err)
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	err = client.handleResourceUpdated(context.Background(), invalidParams)
	if err == nil {
		t.Error("Expected handleResourceUpdated to fail with invalid params")
	}
}

// TestHandlePromptsChanged tests the handlePromptsChanged notification handler
func TestHandlePromptsChanged(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"added": [{"id": "prompt1", "name": "Prompt 1"}, {"id": "prompt2", "name": "Prompt 2"}]}`)
	err := client.handlePromptsChanged(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handlePromptsChanged to succeed, got error: %v", err)
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	err = client.handlePromptsChanged(context.Background(), invalidParams)
	if err == nil {
		t.Error("Expected handlePromptsChanged to fail with invalid params")
	}
}

// TestHandleRootsChanged tests the handleRootsChanged notification handler
func TestHandleRootsChanged(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"added": [{"id": "root1", "name": "Root 1"}, {"id": "root2", "name": "Root 2"}]}`)
	err := client.handleRootsChanged(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handleRootsChanged to succeed, got error: %v", err)
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	err = client.handleRootsChanged(context.Background(), invalidParams)
	if err == nil {
		t.Error("Expected handleRootsChanged to fail with invalid params")
	}
}

// TestHandleLog tests the handleLog notification handler
func TestHandleLog(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"level": "info", "message": "test log", "source": "test"}`)
	err := client.handleLog(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handleLog to succeed, got error: %v", err)
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	err = client.handleLog(context.Background(), invalidParams)
	if err == nil {
		t.Error("Expected handleLog to fail with invalid params")
	}
}

// TestHandleSample tests the handleSample request handler
func TestHandleSample(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"messages": [{"role": "user", "content": "Hello"}]}`)
	_, err := client.handleSample(context.Background(), params)
	if err == nil || !strings.Contains(err.Error(), "sampling not implemented") {
		t.Error("Expected handleSample to return not implemented error")
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	_, err = client.handleSample(context.Background(), invalidParams)
	if err == nil || !strings.Contains(err.Error(), "invalid params") {
		t.Error("Expected handleSample to fail with invalid params")
	}
}

// TestHandleCancel tests the handleCancel request handler
func TestHandleCancel(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test valid params
	params := json.RawMessage(`{"id": "request-123"}`)
	result, err := client.handleCancel(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handleCancel to succeed, got error: %v", err)
	}

	cancelResult := result.(*protocol.CancelResult)
	if cancelResult.Cancelled {
		t.Error("Expected cancel result to be false")
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	_, err = client.handleCancel(context.Background(), invalidParams)
	if err == nil || !strings.Contains(err.Error(), "invalid params") {
		t.Error("Expected handleCancel to fail with invalid params")
	}
}

// TestHandlePing tests the handlePing request handler
func TestHandlePing(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test with timestamp
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	params := json.RawMessage(fmt.Sprintf(`{"timestamp": %d}`, timestamp))
	result, err := client.handlePing(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handlePing to succeed, got error: %v", err)
	}

	pingResult := result.(*protocol.PingResult)
	if pingResult.Timestamp != timestamp {
		t.Errorf("Expected timestamp %d, got %d", timestamp, pingResult.Timestamp)
	}

	// Test without timestamp
	params = json.RawMessage(`{}`)
	result, err = client.handlePing(context.Background(), params)
	if err != nil {
		t.Errorf("Expected handlePing to succeed, got error: %v", err)
	}

	pingResult = result.(*protocol.PingResult)
	if pingResult.Timestamp == 0 {
		t.Error("Expected handlePing to generate timestamp when not provided")
	}

	// Test invalid params
	invalidParams := json.RawMessage(`{invalid json}`)
	_, err = client.handlePing(context.Background(), invalidParams)
	if err == nil || !strings.Contains(err.Error(), "invalid params") {
		t.Error("Expected handlePing to fail with invalid params")
	}
}

// TestParseParams tests the parseParams utility function
func TestParseParams(t *testing.T) {
	type testStruct struct {
		Field1 string `json:"field1"`
		Field2 int    `json:"field2"`
	}

	// Test valid parsing
	params := map[string]interface{}{
		"field1": "test",
		"field2": 42,
	}

	var result testStruct
	err := parseParams(params, &result)
	if err != nil {
		t.Errorf("Expected parseParams to succeed, got error: %v", err)
	}

	if result.Field1 != "test" || result.Field2 != 42 {
		t.Error("Expected parseParams to correctly parse fields")
	}

	// Test with json.RawMessage
	rawParams := json.RawMessage(`{"field1": "raw", "field2": 99}`)
	var result2 testStruct
	err = parseParams(rawParams, &result2)
	if err != nil {
		t.Errorf("Expected parseParams with RawMessage to succeed, got error: %v", err)
	}

	if result2.Field1 != "raw" || result2.Field2 != 99 {
		t.Error("Expected parseParams to correctly parse RawMessage")
	}
}

// TestComplete tests the Complete method
func TestComplete(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityComplete)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful completion
	mt.sendRequestResponse = &protocol.CompleteResult{
		Content:      "This is the completion result",
		Model:        "test-model",
		FinishReason: "stop",
	}

	params := &protocol.CompleteParams{
		Messages: []protocol.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	result, err := client.Complete(context.Background(), params)
	if err != nil {
		t.Fatalf("Expected Complete to succeed, got error: %v", err)
	}

	if result.Content != "This is the completion result" {
		t.Errorf("Expected content 'This is the completion result', got %s", result.Content)
	}

	// Test when server doesn't support completion
	client.capabilities[string(protocol.CapabilityComplete)] = false
	_, err = client.Complete(context.Background(), params)
	if err == nil || err.Error() != "server does not support completions" {
		t.Error("Expected error when server doesn't support completions")
	}
}

// TestListAllTools tests the ListAllTools pagination helper
func TestListAllTools(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityTools)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Mock first page
	callCount := 0
	mt.sendRequestResponse = &protocol.ListToolsResult{
		Tools: []protocol.Tool{
			{Name: "tool1"},
			{Name: "tool2"},
		},
		PaginationResult: protocol.PaginationResult{
			NextCursor: "cursor1",
			HasMore:    true,
		},
	}

	// Override SendRequest to handle multiple calls
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		callCount++
		if callCount == 1 {
			return &protocol.ListToolsResult{
				Tools: []protocol.Tool{
					{Name: "tool1"},
					{Name: "tool2"},
				},
				PaginationResult: protocol.PaginationResult{
					NextCursor: "cursor1",
					HasMore:    true,
				},
			}, nil
		} else {
			return &protocol.ListToolsResult{
				Tools: []protocol.Tool{
					{Name: "tool3"},
				},
				PaginationResult: protocol.PaginationResult{
					HasMore: false,
				},
			}, nil
		}
	}

	tools, err := client.ListAllTools(context.Background(), "test-category")
	if err != nil {
		t.Fatalf("Expected ListAllTools to succeed, got error: %v", err)
	}

	if len(tools) != 3 {
		t.Errorf("Expected 3 tools total, got %d", len(tools))
	}

	// Restore original
	mt.sendRequestFunc = nil
}

// TestListAllResources tests the ListAllResources pagination helper
func TestListAllResources(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityResources)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Override SendRequest to handle multiple calls
	callCount := 0
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		callCount++
		if callCount == 1 {
			return &protocol.ListResourcesResult{
				Resources: []protocol.Resource{
					{URI: "resource1"},
					{URI: "resource2"},
				},
				Templates: []protocol.ResourceTemplate{
					{URI: "template1"},
				},
				PaginationResult: protocol.PaginationResult{
					NextCursor: "cursor1",
					HasMore:    true,
				},
			}, nil
		} else {
			return &protocol.ListResourcesResult{
				Resources: []protocol.Resource{
					{URI: "resource3"},
				},
				Templates: []protocol.ResourceTemplate{
					{URI: "template2"},
				},
				PaginationResult: protocol.PaginationResult{
					HasMore: false,
				},
			}, nil
		}
	}

	resources, templates, err := client.ListAllResources(context.Background(), "", false)
	if err != nil {
		t.Fatalf("Expected ListAllResources to succeed, got error: %v", err)
	}

	if len(resources) != 3 {
		t.Errorf("Expected 3 resources total, got %d", len(resources))
	}

	if len(templates) != 2 {
		t.Errorf("Expected 2 templates total, got %d", len(templates))
	}
}

// TestCallToolStreamingWithProgressUpdates tests CallToolStreaming with progress updates
func TestCallToolStreamingWithProgressUpdates(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityTools)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	var updates []json.RawMessage
	updateHandler := func(data json.RawMessage) {
		updates = append(updates, data)
	}

	// Set up to handle async progress updates
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		// Get the request ID from params
		var p map[string]interface{}
		var paramsBytes []byte
		switch v := params.(type) {
		case json.RawMessage:
			paramsBytes = v
		case []byte:
			paramsBytes = v
		default:
			if b, err := json.Marshal(params); err == nil {
				paramsBytes = b
			}
		}
		json.Unmarshal(paramsBytes, &p)
		requestID := p["requestId"].(string)

		// Simulate sending progress updates
		go func() {
			time.Sleep(10 * time.Millisecond)
			if handler, ok := mt.progressHandlers[requestID]; ok {
				// Send a progress update with data
				progressData := json.RawMessage(`{"data": {"status": "processing"}, "done": false}`)
				handler(progressData)

				// Send final update
				time.Sleep(10 * time.Millisecond)
				finalData := json.RawMessage(`{"data": {"status": "complete"}, "done": true}`)
				handler(finalData)
			}
		}()

		// Return result after a delay
		time.Sleep(50 * time.Millisecond)
		return &protocol.CallToolResult{
			Result: json.RawMessage(`{"output": "completed"}`),
		}, nil
	}

	result, err := client.CallToolStreaming(context.Background(), "progress-tool", nil, nil, updateHandler)
	if err != nil {
		t.Fatalf("Expected CallToolStreaming to succeed, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Give time for all updates to be processed
	time.Sleep(100 * time.Millisecond)

	// Check that we received updates
	if len(updates) < 2 {
		t.Errorf("Expected at least 2 updates, got %d", len(updates))
	}

	// Test with simple progress data (non-structured)
	updates = nil
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		var p map[string]interface{}
		var paramsBytes []byte
		switch v := params.(type) {
		case json.RawMessage:
			paramsBytes = v
		case []byte:
			paramsBytes = v
		default:
			if b, err := json.Marshal(params); err == nil {
				paramsBytes = b
			}
		}
		json.Unmarshal(paramsBytes, &p)
		requestID := p["requestId"].(string)

		go func() {
			time.Sleep(10 * time.Millisecond)
			if handler, ok := mt.progressHandlers[requestID]; ok {
				// Send simple progress data
				handler(json.RawMessage(`"simple progress"`))
			}
		}()

		time.Sleep(30 * time.Millisecond)
		return &protocol.CallToolResult{Result: json.RawMessage(`{"output": "done"}`)}, nil
	}

	_, err = client.CallToolStreaming(context.Background(), "simple-progress-tool", nil, nil, updateHandler)
	if err != nil {
		t.Fatalf("Expected CallToolStreaming to succeed, got error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Reset
	mt.sendRequestFunc = nil
}

// TestListAllPrompts tests the ListAllPrompts pagination helper
func TestListAllPrompts(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityPrompts)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Override SendRequest to handle multiple calls
	callCount := 0
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		callCount++
		if callCount == 1 {
			return &protocol.ListPromptsResult{
				Prompts: []protocol.Prompt{
					{ID: "prompt1"},
					{ID: "prompt2"},
				},
				PaginationResult: protocol.PaginationResult{
					NextCursor: "cursor1",
					HasMore:    true,
				},
			}, nil
		} else {
			return &protocol.ListPromptsResult{
				Prompts: []protocol.Prompt{
					{ID: "prompt3"},
				},
				PaginationResult: protocol.PaginationResult{
					HasMore: false,
				},
			}, nil
		}
	}

	prompts, err := client.ListAllPrompts(context.Background(), "test-tag")
	if err != nil {
		t.Fatalf("Expected ListAllPrompts to succeed, got error: %v", err)
	}

	if len(prompts) != 3 {
		t.Errorf("Expected 3 prompts total, got %d", len(prompts))
	}
}

// TestListAllRoots tests the ListAllRoots pagination helper
func TestListAllRoots(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityRoots)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Override SendRequest to handle multiple calls
	callCount := 0
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		callCount++
		if callCount == 1 {
			return &protocol.ListRootsResult{
				Roots: []protocol.Root{
					{ID: "root1"},
					{ID: "root2"},
				},
				PaginationResult: protocol.PaginationResult{
					NextCursor: "cursor1",
					HasMore:    true,
				},
			}, nil
		} else {
			return &protocol.ListRootsResult{
				Roots: []protocol.Root{
					{ID: "root3"},
				},
				PaginationResult: protocol.PaginationResult{
					HasMore: false,
				},
			}, nil
		}
	}

	roots, err := client.ListAllRoots(context.Background(), "test-tag")
	if err != nil {
		t.Fatalf("Expected ListAllRoots to succeed, got error: %v", err)
	}

	if len(roots) != 3 {
		t.Errorf("Expected 3 roots total, got %d", len(roots))
	}
}

// TestCallToolStreaming tests the CallToolStreaming method
func TestCallToolStreaming(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityTools)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test successful streaming tool call
	var updates []json.RawMessage
	updateHandler := func(data json.RawMessage) {
		updates = append(updates, data)
	}

	mt.sendRequestResponse = &protocol.CallToolResult{
		Result: json.RawMessage(`{"output": "final result"}`),
	}

	result, err := client.CallToolStreaming(context.Background(), "test-tool", map[string]interface{}{"param": "value"}, map[string]interface{}{"context": "data"}, updateHandler)
	if err != nil {
		t.Fatalf("Expected CallToolStreaming to succeed, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Test when server doesn't support tools
	client.capabilities[string(protocol.CapabilityTools)] = false
	_, err = client.CallToolStreaming(context.Background(), "test-tool", nil, nil, nil)
	if err == nil || err.Error() != "server does not support tools" {
		t.Error("Expected error when server doesn't support tools")
	}

	// Test with context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	client.capabilities[string(protocol.CapabilityTools)] = true

	// Set up a delayed response
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return &protocol.CallToolResult{Result: json.RawMessage(`{"output": "delayed"}`)}, nil
	}

	// Cancel context immediately
	cancel()

	_, err = client.CallToolStreaming(ctx, "test-tool", nil, nil, nil)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Error("Expected context.Canceled error")
	}

	// Test with transport error
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		return nil, errors.New("transport error")
	}

	_, err = client.CallToolStreaming(context.Background(), "test-tool", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "transport error") {
		t.Error("Expected transport error")
	}

	// Test with parse error
	mt.sendRequestFunc = func(ctx context.Context, method string, params interface{}) (interface{}, error) {
		return "invalid result", nil
	}

	_, err = client.CallToolStreaming(context.Background(), "test-tool", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to parse call tool result") {
		t.Error("Expected parse error")
	}

	// Reset
	mt.sendRequestFunc = nil
}

// TestNextRequestID tests the nextRequestID helper function
func TestNextRequestID(t *testing.T) {
	// Reset counter for test
	requestIDCounter = 0

	id1 := nextRequestID()
	id2 := nextRequestID()
	id3 := nextRequestID()

	if id1 != 1 {
		t.Errorf("Expected first ID to be 1, got %d", id1)
	}

	if id2 != 2 {
		t.Errorf("Expected second ID to be 2, got %d", id2)
	}

	if id3 != 3 {
		t.Errorf("Expected third ID to be 3, got %d", id3)
	}
}

// TestRegisterProgressHandler tests the registerProgressHandler method
func TestRegisterProgressHandler(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	var receivedData json.RawMessage
	handler := func(data json.RawMessage) {
		receivedData = data
	}

	client.registerProgressHandler("test-id", handler)

	// Verify the handler was registered
	if _, ok := mt.progressHandlers["test-id"]; !ok {
		t.Error("Expected progress handler to be registered")
	}

	// Test the handler with json.RawMessage
	testData := json.RawMessage(`{"test": "data"}`)
	if registeredHandler, ok := mt.progressHandlers["test-id"]; ok {
		err := registeredHandler(testData)
		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if string(receivedData) != string(testData) {
			t.Errorf("Expected handler to receive %s, got %s", testData, receivedData)
		}
	}

	// Test the handler with non-RawMessage data
	receivedData = nil
	testStruct := map[string]interface{}{"key": "value"}
	if registeredHandler, ok := mt.progressHandlers["test-id"]; ok {
		err := registeredHandler(testStruct)
		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		// Should have marshaled the struct
		if receivedData == nil {
			t.Error("Expected handler to receive marshaled data")
		}
	}

	// Test the handler with unmarshalable data
	receivedData = nil
	unmarshalable := make(chan int)
	if registeredHandler, ok := mt.progressHandlers["test-id"]; ok {
		err := registeredHandler(unmarshalable)
		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		// Should not have set receivedData since marshaling failed
		if receivedData != nil {
			t.Error("Expected handler to not receive data for unmarshalable input")
		}
	}
}

// TestSample tests the Sample method
func TestSample(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test that Sample returns nil (not implemented)
	params := &protocol.SampleParams{
		Messages: []protocol.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	err := client.Sample(context.Background(), params, func(result *protocol.SampleResult) error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected Sample to return nil, got error: %v", err)
	}
}

// TestSetSamplingCallback tests the SetSamplingCallback method
func TestSetSamplingCallback(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test that SetSamplingCallback doesn't panic
	client.SetSamplingCallback(func(event protocol.SamplingEvent) {})
}

// TestSetResourceChangedCallback tests the SetResourceChangedCallback method
func TestSetResourceChangedCallback(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)

	// Test that SetResourceChangedCallback doesn't panic
	client.SetResourceChangedCallback(func(id string, data interface{}) {})
}

// TestParseResultError tests error cases in parseResult
func TestParseResultError(t *testing.T) {
	// Test with unmarshalable data
	err := parseResult(make(chan int), &protocol.ListToolsResult{})
	if err == nil || !strings.Contains(err.Error(), "failed to marshal result") {
		t.Error("Expected parseResult to fail with marshal error")
	}
}

// TestParseParamsError tests error cases in parseParams
func TestParseParamsError(t *testing.T) {
	// Test with unmarshalable data
	err := parseParams(make(chan int), &protocol.ListToolsParams{})
	if err == nil || !strings.Contains(err.Error(), "failed to marshal params") {
		t.Error("Expected parseParams to fail with marshal error")
	}
}

// TestCallToolWithInvalidInput tests CallTool with unmarshalable input
func TestCallToolWithInvalidInput(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityTools)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test with unmarshalable input
	_, err := client.CallTool(context.Background(), "test-tool", make(chan int), nil)
	if err == nil || !strings.Contains(err.Error(), "failed to marshal input") {
		t.Error("Expected CallTool to fail with marshal input error")
	}

	// Test with unmarshalable context
	_, err = client.CallTool(context.Background(), "test-tool", nil, make(chan int))
	if err == nil || !strings.Contains(err.Error(), "failed to marshal context") {
		t.Error("Expected CallTool to fail with marshal context error")
	}
}

// TestCallToolStreamingWithInvalidInput tests CallToolStreaming with unmarshalable input
func TestCallToolStreamingWithInvalidInput(t *testing.T) {
	mt := newMockTransport()
	client := New(mt)
	client.initialized = true
	client.capabilities[string(protocol.CapabilityTools)] = true
	client.serverInfo = &protocol.ServerInfo{Name: "test-server", Version: "1.0"}

	// Test with unmarshalable input
	_, err := client.CallToolStreaming(context.Background(), "test-tool", make(chan int), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to marshal input") {
		t.Error("Expected CallToolStreaming to fail with marshal input error")
	}

	// Test with unmarshalable context
	_, err = client.CallToolStreaming(context.Background(), "test-tool", nil, make(chan int), nil)
	if err == nil || !strings.Contains(err.Error(), "failed to marshal context") {
		t.Error("Expected CallToolStreaming to fail with marshal context error")
	}
}

// TestInitializeWithStreamableHTTPTransport tests Initialize with streamable HTTP transport (for coverage)
func TestInitializeWithStreamableHTTPTransport(t *testing.T) {
	// This test is just for coverage of the reflection code path
	// Since we can't easily create a real StreamableHTTPTransport, we'll just test with our mock
	mt := newMockTransport()
	client := New(mt)

	// Mock the transport to return a valid initialize result
	mt.sendRequestResponse = &protocol.InitializeResult{
		ProtocolVersion: "1.0",
		ServerInfo: &protocol.ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
		Capabilities: map[string]bool{
			"tools": true,
		},
	}

	ctx := context.Background()
	err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Expected Initialize to succeed, got error: %v", err)
	}
}

// TestNewStdioClient tests the NewStdioClient function
func TestNewStdioClient(t *testing.T) {
	// Test creating a stdio client
	client := NewStdioClient(WithName("test-stdio-client"))
	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.name != "test-stdio-client" {
		t.Errorf("Expected client name to be 'test-stdio-client', got %q", client.name)
	}
}

// TestNewStdioClientWithStreams tests the NewStdioClientWithStreams function
func TestNewStdioClientWithStreams(t *testing.T) {
	// Create pipes for testing
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinR.Close()
	defer stdinW.Close()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutR.Close()
	defer stdoutW.Close()

	// Test creating a stdio client with custom streams
	client := NewStdioClientWithStreams(stdinR, stdoutW, WithName("test-stdio-streams"))
	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.name != "test-stdio-streams" {
		t.Errorf("Expected client name to be 'test-stdio-streams', got %q", client.name)
	}
}
