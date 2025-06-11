package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
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

func (m *mockTransport) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	// Simple implementation for tests
	return &protocol.JSONRPCBatchResponse{}, nil
}

func (m *mockTransport) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	// Simple implementation for tests
	return &protocol.JSONRPCBatchResponse{}, nil
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

// HandleResponse implements the transport.Transport interface
func (m *mockTransport) HandleResponse(response *protocol.Response) {
	// No-op for mock
}

// HandleRequest implements the transport.Transport interface
func (m *mockTransport) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	if handler, ok := m.requestHandlers[request.Method]; ok {
		result, err := handler(ctx, request.Params)
		if err != nil {
			return nil, err
		}
		return protocol.NewResponse(request.ID, result)
	}
	return nil, errors.New("method not found")
}

// HandleNotification implements the transport.Transport interface
func (m *mockTransport) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	if handler, ok := m.notificationHandlers[notification.Method]; ok {
		return handler(ctx, notification.Params)
	}
	return errors.New("notification handler not found")
}

// GetRequestIDPrefix implements the transport.Transport interface
func (m *mockTransport) GetRequestIDPrefix() string {
	return "test"
}

// GetNextID implements the transport.Transport interface
func (m *mockTransport) GetNextID() int64 {
	return 1
}

// Cleanup implements the transport.Transport interface
func (m *mockTransport) Cleanup() {
	// No-op for mock
}

// Lock implements the transport.Transport interface
func (m *mockTransport) Lock() {
	// No-op for mock
}

// Unlock implements the transport.Transport interface
func (m *mockTransport) Unlock() {
	// No-op for mock
}

// Custom mock implementations for providers

// TestServerConcurrentInitialize tests that concurrent initialization is safe
func TestServerConcurrentInitialize(t *testing.T) {
	transport := newMockTransport()
	server := New(transport, WithName("test-server"))

	// Create multiple goroutines that try to initialize concurrently
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Create unique client info for each goroutine
			params := &protocol.InitializeParams{
				Name:    fmt.Sprintf("client-%d", id),
				Version: "1.0.0",
				ClientInfo: &protocol.ClientInfo{
					Name:    fmt.Sprintf("test-client-%d", id),
					Version: "1.0.0",
				},
			}

			ctx := context.Background()
			_, err := server.handleInitialize(ctx, params)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check if any errors occurred
	for err := range errors {
		t.Errorf("Unexpected error during concurrent initialization: %v", err)
	}

	// Verify server is initialized
	if !server.isInitialized() {
		t.Error("Server should be initialized after handleInitialize")
	}

	// Verify clientInfo is set (it should be from one of the goroutines)
	clientInfo := server.getClientInfo()
	if clientInfo == nil {
		t.Error("Client info should be set after initialization")
	}
}

// TestServerGetClientInfoSafety tests that getClientInfo is safe to call concurrently
func TestServerGetClientInfoSafety(t *testing.T) {
	transport := newMockTransport()
	server := New(transport, WithName("test-server"))

	// Initialize the server first
	params := &protocol.InitializeParams{
		Name:    "test-client",
		Version: "1.0.0",
		ClientInfo: &protocol.ClientInfo{
			Name:    "test-client-info",
			Version: "1.0.0",
		},
	}

	ctx := context.Background()
	_, err := server.handleInitialize(ctx, params)
	if err != nil {
		t.Fatalf("Failed to initialize server: %v", err)
	}

	// Now test concurrent reads
	const numReaders = 100
	var wg sync.WaitGroup
	wg.Add(numReaders)

	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			info := server.getClientInfo()
			if info == nil {
				t.Error("Client info should not be nil")
				return
			}
			if info.Name != "test-client-info" {
				t.Errorf("Expected client name 'test-client-info', got '%s'", info.Name)
			}
		}()
	}

	wg.Wait()
}

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

// Test logging methods
func TestServerLogging(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	// Test default logger methods (access through logger field)
	server.logger.Debug("Debug message: %s", "test")
	server.logger.Warn("Warning message: %d", 42)
	server.logger.Error("Error message: %v", errors.New("test error"))

	// Test with custom logger
	customLogger := &mockLogger{}
	server = New(mt, WithLogger(customLogger))

	server.logger.Debug("Custom debug")
	server.logger.Info("Custom info")
	server.logger.Warn("Custom warn")
	server.logger.Error("Custom error")

	if customLogger.debugCalled == 0 {
		t.Error("Expected Debug to be called on custom logger")
	}
	if customLogger.infoCalled == 0 {
		t.Error("Expected Info to be called on custom logger")
	}
	if customLogger.warnCalled == 0 {
		t.Error("Expected Warn to be called on custom logger")
	}
	if customLogger.errorCalled == 0 {
		t.Error("Expected Error to be called on custom logger")
	}
}

// mockLogger for testing
type mockLogger struct {
	debugCalled int
	infoCalled  int
	warnCalled  int
	errorCalled int
}

func (l *mockLogger) Debug(msg string, args ...interface{}) {
	l.debugCalled++
}

func (l *mockLogger) Info(msg string, args ...interface{}) {
	l.infoCalled++
}

func (l *mockLogger) Warn(msg string, args ...interface{}) {
	l.warnCalled++
}

func (l *mockLogger) Error(msg string, args ...interface{}) {
	l.errorCalled++
}

// Test provider setters
func TestServerProviderOptions(t *testing.T) {
	mt := newMockTransport()

	// Test WithResourcesProvider
	resourcesProvider := &mockServerResourcesProvider{}
	server := New(mt, WithResourcesProvider(resourcesProvider))

	if server.resourcesProvider == nil {
		t.Error("Expected resources provider to be set")
	}

	if !server.capabilities[string(protocol.CapabilityResources)] {
		t.Error("Expected resources capability to be enabled")
	}

	// Check that resource handlers are registered
	if _, ok := mt.requestHandlers[string(protocol.MethodListResources)]; !ok {
		t.Error("Expected ListResources handler to be registered")
	}

	if _, ok := mt.requestHandlers[string(protocol.MethodReadResource)]; !ok {
		t.Error("Expected ReadResource handler to be registered")
	}

	// Test WithPromptsProvider
	promptsProvider := &mockPromptsProvider{}
	server = New(mt, WithPromptsProvider(promptsProvider))

	if server.promptsProvider == nil {
		t.Error("Expected prompts provider to be set")
	}

	if !server.capabilities[string(protocol.CapabilityPrompts)] {
		t.Error("Expected prompts capability to be enabled")
	}

	// Test WithCompletionProvider
	completionProvider := &mockCompletionProvider{}
	server = New(mt, WithCompletionProvider(completionProvider))

	if server.completionProvider == nil {
		t.Error("Expected completion provider to be set")
	}

	if !server.capabilities[string(protocol.CapabilityComplete)] {
		t.Error("Expected completion capability to be enabled")
	}

	// Test WithRootsProvider
	rootsProvider := &mockRootsProvider{}
	server = New(mt, WithRootsProvider(rootsProvider))

	if server.rootsProvider == nil {
		t.Error("Expected roots provider to be set")
	}

	if !server.capabilities[string(protocol.CapabilityRoots)] {
		t.Error("Expected roots capability to be enabled")
	}
}

// Mock providers for testing (avoid conflict with providers_test.go)
type mockServerResourcesProvider struct {
	resources []protocol.Resource
	templates []protocol.ResourceTemplate
	err       error
}

func (m *mockServerResourcesProvider) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, int, string, bool, error) {
	if m.err != nil {
		return nil, nil, 0, "", false, m.err
	}
	return m.resources, m.templates, len(m.resources), "", false, nil
}

func (m *mockServerResourcesProvider) ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &protocol.ResourceContents{
		URI:     uri,
		Type:    "text/plain",
		Content: json.RawMessage(`"test content"`),
	}, nil
}

func (m *mockServerResourcesProvider) SubscribeResource(ctx context.Context, uri string, recursive bool) (bool, error) {
	return true, m.err
}

type mockPromptsProvider struct {
	prompts []protocol.Prompt
	err     error
}

func (m *mockPromptsProvider) ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, int, string, bool, error) {
	if m.err != nil {
		return nil, 0, "", false, m.err
	}
	return m.prompts, len(m.prompts), "", false, nil
}

func (m *mockPromptsProvider) GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, prompt := range m.prompts {
		if prompt.ID == id {
			return &prompt, nil
		}
	}
	return nil, errors.New("prompt not found")
}

type mockCompletionProvider struct {
	err error
}

func (m *mockCompletionProvider) Complete(ctx context.Context, params *protocol.CompleteParams) (*protocol.CompleteResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &protocol.CompleteResult{
		Content:      "Completion result",
		Model:        "test-model",
		FinishReason: "stop",
	}, nil
}

type mockRootsProvider struct {
	roots []protocol.Root
	err   error
}

func (m *mockRootsProvider) ListRoots(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Root, int, string, bool, error) {
	if m.err != nil {
		return nil, 0, "", false, m.err
	}
	return m.roots, len(m.roots), "", false, nil
}

// Test notification methods
func TestServerNotifications(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	// Initialize server first
	server.initializedLock.Lock()
	server.initialized = true
	server.initializedLock.Unlock()

	// Test NotifyResourcesChanged
	resources := []protocol.Resource{
		{URI: "test://resource1", Type: "text"},
	}
	err := server.NotifyResourcesChanged("test://", resources, []string{"removed1"}, resources, resources)
	if err != nil {
		t.Errorf("Expected NotifyResourcesChanged to succeed, got error: %v", err)
	}

	if _, ok := mt.sentNotifications[string(protocol.MethodResourcesChanged)]; !ok {
		t.Error("Expected resourcesChanged notification to be sent")
	}

	// Test NotifyResourceUpdated
	contents := &protocol.ResourceContents{
		URI:     "test://resource1",
		Type:    "text/plain",
		Content: json.RawMessage(`"updated content"`),
	}
	err = server.NotifyResourceUpdated("test://resource1", contents, false)
	if err != nil {
		t.Errorf("Expected NotifyResourceUpdated to succeed, got error: %v", err)
	}

	// Test NotifyResourceUpdated with deletion
	err = server.NotifyResourceUpdated("test://deleted", nil, true)
	if err != nil {
		t.Errorf("Expected NotifyResourceUpdated (delete) to succeed, got error: %v", err)
	}

	// Test NotifyPromptsChanged
	prompts := []protocol.Prompt{
		{ID: "prompt1", Name: "Test Prompt"},
	}
	err = server.NotifyPromptsChanged(prompts, []string{"removed1"}, prompts)
	if err != nil {
		t.Errorf("Expected NotifyPromptsChanged to succeed, got error: %v", err)
	}

	// Test NotifyRootsChanged
	roots := []protocol.Root{
		{ID: "root1", Name: "Test Root"},
	}
	err = server.NotifyRootsChanged(roots, []string{"removed1"}, roots)
	if err != nil {
		t.Errorf("Expected NotifyRootsChanged to succeed, got error: %v", err)
	}

	// Test notifications when server not initialized
	server.initializedLock.Lock()
	server.initialized = false
	server.initializedLock.Unlock()

	err = server.NotifyResourcesChanged("", nil, nil, nil, nil)
	if err == nil {
		t.Error("Expected NotifyResourcesChanged to fail when server not initialized")
	}

	err = server.NotifyResourceUpdated("", nil, false)
	if err == nil {
		t.Error("Expected NotifyResourceUpdated to fail when server not initialized")
	}

	err = server.NotifyPromptsChanged(nil, nil, nil)
	if err == nil {
		t.Error("Expected NotifyPromptsChanged to fail when server not initialized")
	}

	err = server.NotifyRootsChanged(nil, nil, nil)
	if err == nil {
		t.Error("Expected NotifyRootsChanged to fail when server not initialized")
	}
}

// Test protocol method handlers
func TestServerProtocolHandlers(t *testing.T) {
	mt := newMockTransport()

	// Create providers
	toolsProvider := &mockToolsProvider{
		tools: []protocol.Tool{
			{Name: "test-tool", Description: "A test tool"},
		},
	}
	resourcesProvider := &mockServerResourcesProvider{
		resources: []protocol.Resource{
			{URI: "test://resource1", Type: "text"},
		},
	}
	promptsProvider := &mockPromptsProvider{
		prompts: []protocol.Prompt{
			{ID: "prompt1", Name: "Test Prompt"},
		},
	}
	completionProvider := &mockCompletionProvider{}
	rootsProvider := &mockRootsProvider{
		roots: []protocol.Root{
			{ID: "root1", Name: "Test Root"},
		},
	}

	server := New(mt,
		WithToolsProvider(toolsProvider),
		WithResourcesProvider(resourcesProvider),
		WithPromptsProvider(promptsProvider),
		WithCompletionProvider(completionProvider),
		WithRootsProvider(rootsProvider),
	)

	ctx := context.Background()

	// Test handleListTools
	listToolsParams := &protocol.ListToolsParams{
		Category: "",
		PaginationParams: protocol.PaginationParams{
			Limit: 10,
		},
	}
	result, err := server.handleListTools(ctx, listToolsParams)
	if err != nil {
		t.Errorf("Expected handleListTools to succeed, got error: %v", err)
	}
	listResult, ok := result.(*protocol.ListToolsResult)
	if !ok {
		t.Errorf("Expected *protocol.ListToolsResult, got %T", result)
	} else if len(listResult.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(listResult.Tools))
	}

	// Test handleCallTool
	callToolParams := &protocol.CallToolParams{
		Name:  "test-tool",
		Input: json.RawMessage(`{"param": "value"}`),
	}
	_, err = server.handleCallTool(ctx, callToolParams)
	if err != nil {
		t.Errorf("Expected handleCallTool to succeed, got error: %v", err)
	}

	// Test handleListResources
	listResourcesParams := &protocol.ListResourcesParams{
		URI: "",
		PaginationParams: protocol.PaginationParams{
			Limit: 10,
		},
	}
	_, err = server.handleListResources(ctx, listResourcesParams)
	if err != nil {
		t.Errorf("Expected handleListResources to succeed, got error: %v", err)
	}

	// Test handleReadResource
	readResourceParams := &protocol.ReadResourceParams{
		URI: "test://resource1",
	}
	_, err = server.handleReadResource(ctx, readResourceParams)
	if err != nil {
		t.Errorf("Expected handleReadResource to succeed, got error: %v", err)
	}

	// Test handleListPrompts
	listPromptsParams := &protocol.ListPromptsParams{
		Tag: "",
		PaginationParams: protocol.PaginationParams{
			Limit: 10,
		},
	}
	_, err = server.handleListPrompts(ctx, listPromptsParams)
	if err != nil {
		t.Errorf("Expected handleListPrompts to succeed, got error: %v", err)
	}

	// Test handleGetPrompt
	getPromptParams := &protocol.GetPromptParams{
		ID: "prompt1",
	}
	_, err = server.handleGetPrompt(ctx, getPromptParams)
	if err != nil {
		t.Errorf("Expected handleGetPrompt to succeed, got error: %v", err)
	}

	// Test handleComplete
	completeParams := &protocol.CompleteParams{
		Messages: []protocol.Message{
			{Role: "user", Content: "Hello"},
		},
	}
	_, err = server.handleComplete(ctx, completeParams)
	if err != nil {
		t.Errorf("Expected handleComplete to succeed, got error: %v", err)
	}

	// Test handleListRoots
	listRootsParams := &protocol.ListRootsParams{
		Tag: "",
		PaginationParams: protocol.PaginationParams{
			Limit: 10,
		},
	}
	_, err = server.handleListRoots(ctx, listRootsParams)
	if err != nil {
		t.Errorf("Expected handleListRoots to succeed, got error: %v", err)
	}
}

// Test handler error cases
func TestServerHandlerErrors(t *testing.T) {
	mt := newMockTransport()
	server := New(mt) // No providers configured

	ctx := context.Background()

	// Test handlers without providers
	_, err := server.handleListTools(ctx, &protocol.ListToolsParams{})
	if err == nil {
		t.Error("Expected handleListTools to fail without tools provider")
	}

	_, err = server.handleCallTool(ctx, &protocol.CallToolParams{})
	if err == nil {
		t.Error("Expected handleCallTool to fail without tools provider")
	}

	_, err = server.handleListResources(ctx, &protocol.ListResourcesParams{})
	if err == nil {
		t.Error("Expected handleListResources to fail without resources provider")
	}

	_, err = server.handleReadResource(ctx, &protocol.ReadResourceParams{})
	if err == nil {
		t.Error("Expected handleReadResource to fail without resources provider")
	}

	_, err = server.handleListPrompts(ctx, &protocol.ListPromptsParams{})
	if err == nil {
		t.Error("Expected handleListPrompts to fail without prompts provider")
	}

	_, err = server.handleGetPrompt(ctx, &protocol.GetPromptParams{})
	if err == nil {
		t.Error("Expected handleGetPrompt to fail without prompts provider")
	}

	_, err = server.handleComplete(ctx, &protocol.CompleteParams{})
	if err == nil {
		t.Error("Expected handleComplete to fail without completion provider")
	}

	_, err = server.handleListRoots(ctx, &protocol.ListRootsParams{})
	if err == nil {
		t.Error("Expected handleListRoots to fail without roots provider")
	}
}

// Test SendLog and SendProgress methods
func TestServerSendMethods(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	// Initialize server first
	server.initializedLock.Lock()
	server.initialized = true
	server.initializedLock.Unlock()

	// Test SendLog
	err := server.SendLog(protocol.LogLevelInfo, "Test log message", "test-source", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("Expected SendLog to succeed, got error: %v", err)
	}

	// Test SendLog with nil data
	err = server.SendLog(protocol.LogLevelError, "Error message", "test-source", nil)
	if err != nil {
		t.Errorf("Expected SendLog with nil data to succeed, got error: %v", err)
	}

	// Test SendProgress
	err = server.SendProgress("progress-id", "Processing...", 50.0, false)
	if err != nil {
		t.Errorf("Expected SendProgress to succeed, got error: %v", err)
	}

	// Test SendProgress completion
	err = server.SendProgress("progress-id", "Completed", 100.0, true)
	if err != nil {
		t.Errorf("Expected SendProgress completion to succeed, got error: %v", err)
	}

	// Test methods when server not initialized
	server.initializedLock.Lock()
	server.initialized = false
	server.initializedLock.Unlock()

	err = server.SendLog(protocol.LogLevelInfo, "Test", "test", nil)
	if err == nil {
		t.Error("Expected SendLog to fail when server not initialized")
	}

	err = server.SendProgress("id", "test", 0, false)
	if err == nil {
		t.Error("Expected SendProgress to fail when server not initialized")
	}
}

// Test additional handler methods
func TestServerAdditionalHandlers(t *testing.T) {
	mt := newMockTransport()
	server := New(mt, WithCapability(protocol.CapabilityLogging, true))

	ctx := context.Background()

	// Test handleSetCapability
	setCapParams := &protocol.SetCapabilityParams{
		Capability: "test-capability",
		Value:      true,
	}
	result, err := server.handleSetCapability(ctx, setCapParams)
	if err != nil {
		t.Errorf("Expected handleSetCapability to succeed, got error: %v", err)
	}
	setCapResult, ok := result.(*protocol.SetCapabilityResult)
	if !ok {
		t.Errorf("Expected *protocol.SetCapabilityResult, got %T", result)
	} else if !setCapResult.Success {
		t.Error("Expected SetCapability to return success true")
	}

	// Test handleCancel
	cancelParams := &protocol.CancelParams{
		ID: "test-request-id",
	}
	_, err = server.handleCancel(ctx, cancelParams)
	if err != nil {
		t.Errorf("Expected handleCancel to succeed, got error: %v", err)
	}

	// Test handlePing
	pingParams := &protocol.PingParams{
		Timestamp: 1234567890,
	}
	result, err = server.handlePing(ctx, pingParams)
	if err != nil {
		t.Errorf("Expected handlePing to succeed, got error: %v", err)
	}
	pingResult, ok := result.(*protocol.PingResult)
	if !ok {
		t.Errorf("Expected *protocol.PingResult, got %T", result)
	} else if pingResult.Timestamp != 1234567890 {
		t.Errorf("Expected timestamp 1234567890, got %d", pingResult.Timestamp)
	}

	// Test handlePing with zero timestamp
	pingParams.Timestamp = 0
	result, err = server.handlePing(ctx, pingParams)
	if err != nil {
		t.Errorf("Expected handlePing with zero timestamp to succeed, got error: %v", err)
	}
	pingResult, ok = result.(*protocol.PingResult)
	if !ok {
		t.Errorf("Expected *protocol.PingResult, got %T", result)
	} else if pingResult.Timestamp == 0 {
		t.Error("Expected non-zero timestamp when input timestamp is 0")
	}

	// Test handleSetLogLevel
	logLevelParams := &protocol.SetLogLevelParams{
		Level: protocol.LogLevelDebug,
	}
	_, err = server.handleSetLogLevel(ctx, logLevelParams)
	if err != nil {
		t.Errorf("Expected handleSetLogLevel to succeed, got error: %v", err)
	}

	// Test handleSetLogLevel without logging capability
	serverNoLogging := New(mt) // No logging capability
	_, err = serverNoLogging.handleSetLogLevel(ctx, logLevelParams)
	if err == nil {
		t.Error("Expected handleSetLogLevel to fail without logging capability")
	}
}

// Test resource subscription
func TestServerResourceSubscription(t *testing.T) {
	mt := newMockTransport()
	resourcesProvider := &mockServerResourcesProvider{}
	server := New(mt,
		WithResourcesProvider(resourcesProvider),
		WithCapability(protocol.CapabilityResourceSubscriptions, true),
	)

	ctx := context.Background()

	// Test handleSubscribeResource
	subParams := &protocol.SubscribeResourceParams{
		URI:       "test://resource",
		Recursive: true,
	}
	result, err := server.handleSubscribeResource(ctx, subParams)
	if err != nil {
		t.Errorf("Expected handleSubscribeResource to succeed, got error: %v", err)
	}
	subResult, ok := result.(*protocol.SubscribeResourceResult)
	if !ok {
		t.Errorf("Expected *protocol.SubscribeResourceResult, got %T", result)
	} else if !subResult.Success {
		t.Error("Expected SubscribeResource to return success true")
	}

	// Test handleSubscribeResource without subscription capability
	serverNoSub := New(mt, WithResourcesProvider(resourcesProvider))
	_, err = serverNoSub.handleSubscribeResource(ctx, subParams)
	if err == nil {
		t.Error("Expected handleSubscribeResource to fail without subscription capability")
	}
}

// Test missing provider methods
func TestBaseProviderMethods(t *testing.T) {
	// Test BaseResourcesProvider ReadResource and SubscribeResource
	resourcesProvider := NewBaseResourcesProvider()

	ctx := context.Background()

	// Test ReadResource
	contents, err := resourcesProvider.ReadResource(ctx, "test://resource", nil, nil)
	if err != nil {
		t.Errorf("Expected ReadResource to succeed, got error: %v", err)
	}
	if contents.URI != "test://resource" {
		t.Errorf("Expected URI to be 'test://resource', got %q", contents.URI)
	}

	// Test SubscribeResource
	success, err := resourcesProvider.SubscribeResource(ctx, "test://resource", true)
	if err != nil {
		t.Errorf("Expected SubscribeResource to succeed, got error: %v", err)
	}
	if !success {
		t.Error("Expected SubscribeResource to return true")
	}

	// Test BaseRootsProvider
	rootsProvider := NewBaseRootsProvider()

	// Test RegisterRoot
	root := protocol.Root{
		ID:   "root1",
		Name: "Test Root",
		Tags: []string{"test"},
	}
	rootsProvider.RegisterRoot(root)

	// Test ListRoots
	pagination := &protocol.PaginationParams{Limit: 10}
	roots, total, cursor, hasMore, err := rootsProvider.ListRoots(ctx, "", pagination)
	if err != nil {
		t.Errorf("Expected ListRoots to succeed, got error: %v", err)
	}
	if len(roots) != 1 {
		t.Errorf("Expected 1 root, got %d", len(roots))
	}
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if hasMore {
		t.Error("Expected hasMore to be false")
	}
	if cursor != "" {
		t.Errorf("Expected empty cursor, got %q", cursor)
	}
	if roots[0].ID != "root1" {
		t.Errorf("Expected root ID 'root1', got %q", roots[0].ID)
	}

	// Test ListRoots with tag filter
	roots, _, _, _, err = rootsProvider.ListRoots(ctx, "test", pagination)
	if err != nil {
		t.Errorf("Expected ListRoots with tag to succeed, got error: %v", err)
	}
	if len(roots) != 1 {
		t.Errorf("Expected 1 root with tag 'test', got %d", len(roots))
	}

	// Test ListRoots with non-matching tag
	roots, _, _, _, err = rootsProvider.ListRoots(ctx, "nonexistent", pagination)
	if err != nil {
		t.Errorf("Expected ListRoots with nonexistent tag to succeed, got error: %v", err)
	}
	if len(roots) != 0 {
		t.Errorf("Expected 0 roots with tag 'nonexistent', got %d", len(roots))
	}
}

// Test utility functions with missing coverage
func TestUtilityFunctions(t *testing.T) {
	// Test hasPrefix function (used in providers.go)
	if !hasPrefix("test/path", "test") {
		t.Error("Expected hasPrefix to return true for 'test/path' with prefix 'test'")
	}

	if hasPrefix("test", "test/path") {
		t.Error("Expected hasPrefix to return false for 'test' with prefix 'test/path'")
	}

	if hasPrefix("test", "other") {
		t.Error("Expected hasPrefix to return false for 'test' with prefix 'other'")
	}

	// Test errNotFound
	err := errNotFound("resource", "test-id")
	if err == nil {
		t.Fatal("Expected errNotFound to return an error")
	}

	expectedMsg := "resource not found: test-id"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// Test handleInitialized method
func TestHandleInitialized(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	ctx := context.Background()

	// Test handleInitialized
	err := server.handleInitialized(ctx, &protocol.InitializedParams{})
	if err != nil {
		t.Errorf("Expected handleInitialized to succeed, got error: %v", err)
	}

	// Server should be marked as initialized
	if !server.isInitialized() {
		t.Error("Expected server to be marked as initialized")
	}
}

// Test error handling in parseParams
func TestParseParamsErrorHandling(t *testing.T) {
	// Test with invalid JSON that can't be marshaled back
	type invalidStruct struct {
		InvalidFunc func() // functions can't be marshaled to JSON
	}

	invalid := invalidStruct{InvalidFunc: func() {}}
	var target struct{}

	err := parseParams(invalid, &target)
	if err == nil {
		t.Error("Expected parseParams to fail with unmarshalable input")
	}
}

// Test SendLog with marshaling error
func TestSendLogMarshalError(t *testing.T) {
	mt := newMockTransport()
	server := New(mt)

	// Initialize server
	server.initializedLock.Lock()
	server.initialized = true
	server.initializedLock.Unlock()

	// Create data that can't be marshaled
	invalidData := make(chan int) // channels can't be marshaled

	err := server.SendLog(protocol.LogLevelInfo, "Test", "test", invalidData)
	if err == nil {
		t.Error("Expected SendLog to fail with unmarshalable data")
	}

	if !strings.Contains(err.Error(), "marshal_log_data") {
		t.Errorf("Expected error to mention marshaling failure, got: %v", err)
	}
}
