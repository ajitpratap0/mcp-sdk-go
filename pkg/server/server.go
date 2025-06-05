package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// Server represents an MCP server
type Server struct {
	transport      transport.Transport
	name           string
	version        string
	description    string
	homepage       string
	capabilities   map[string]bool
	featureOptions map[string]interface{}

	// Server feature implementations
	toolsProvider      ToolsProvider
	resourcesProvider  ResourcesProvider
	promptsProvider    PromptsProvider
	completionProvider CompletionProvider
	rootsProvider      RootsProvider

	// Server state
	initialized     bool
	initializedLock sync.RWMutex
	clientInfo      *protocol.ClientInfo
	ctx             context.Context
	cancel          context.CancelFunc

	// Logger
	logger Logger
}

// Logger defines the interface for logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// DefaultLogger is a simple implementation of Logger
type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+msg+"\n", args...)
}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] "+msg+"\n", args...)
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}

// ServerOption defines options for creating a server
type ServerOption func(*Server)

// WithName sets the server name
func WithName(name string) ServerOption {
	return func(s *Server) {
		s.name = name
	}
}

// WithVersion sets the server version
func WithVersion(version string) ServerOption {
	return func(s *Server) {
		s.version = version
	}
}

// WithDescription sets the server description
func WithDescription(description string) ServerOption {
	return func(s *Server) {
		s.description = description
	}
}

// WithHomepage sets the server homepage
func WithHomepage(homepage string) ServerOption {
	return func(s *Server) {
		s.homepage = homepage
	}
}

// WithCapability enables a server capability
func WithCapability(capability protocol.CapabilityType, enabled bool) ServerOption {
	return func(s *Server) {
		s.capabilities[string(capability)] = enabled
	}
}

// WithFeatureOptions sets feature options for the server
func WithFeatureOptions(options map[string]interface{}) ServerOption {
	return func(s *Server) {
		for k, v := range options {
			s.featureOptions[k] = v
		}
	}
}

// WithToolsProvider sets the tools provider
func WithToolsProvider(provider ToolsProvider) ServerOption {
	return func(s *Server) {
		s.toolsProvider = provider
		s.capabilities[string(protocol.CapabilityTools)] = true
	}
}

// WithResourcesProvider sets the resources provider
func WithResourcesProvider(provider ResourcesProvider) ServerOption {
	return func(s *Server) {
		s.resourcesProvider = provider
		s.capabilities[string(protocol.CapabilityResources)] = true
	}
}

// WithPromptsProvider sets the prompts provider
func WithPromptsProvider(provider PromptsProvider) ServerOption {
	return func(s *Server) {
		s.promptsProvider = provider
		s.capabilities[string(protocol.CapabilityPrompts)] = true
	}
}

// WithCompletionProvider sets the completion provider
func WithCompletionProvider(provider CompletionProvider) ServerOption {
	return func(s *Server) {
		s.completionProvider = provider
		s.capabilities[string(protocol.CapabilityComplete)] = true
	}
}

// WithRootsProvider sets the roots provider
func WithRootsProvider(provider RootsProvider) ServerOption {
	return func(s *Server) {
		s.rootsProvider = provider
		s.capabilities[string(protocol.CapabilityRoots)] = true
	}
}

// WithLogger sets the logger
func WithLogger(logger Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// New creates a new MCP server
func New(t transport.Transport, options ...ServerOption) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		transport:      t,
		name:           "go-mcp-server",
		version:        "1.0.0",
		capabilities:   make(map[string]bool),
		featureOptions: make(map[string]interface{}),
		logger:         &DefaultLogger{},
		ctx:            ctx,
		cancel:         cancel,
	}

	// Apply options
	for _, option := range options {
		option(server)
	}

	// Register request handlers
	t.RegisterRequestHandler(protocol.MethodInitialize, server.handleInitialize)
	t.RegisterNotificationHandler(protocol.MethodInitialized, server.handleInitialized)
	t.RegisterRequestHandler(protocol.MethodSetCapability, server.handleSetCapability)
	t.RegisterRequestHandler(protocol.MethodCancel, server.handleCancel)
	t.RegisterRequestHandler(protocol.MethodPing, server.handlePing)
	t.RegisterRequestHandler(protocol.MethodSetLogLevel, server.handleSetLogLevel)

	// Register feature handlers based on capabilities
	if server.capabilities[string(protocol.CapabilityTools)] {
		t.RegisterRequestHandler(protocol.MethodListTools, server.handleListTools)
		t.RegisterRequestHandler(protocol.MethodCallTool, server.handleCallTool)
	}

	if server.capabilities[string(protocol.CapabilityResources)] {
		t.RegisterRequestHandler(protocol.MethodListResources, server.handleListResources)
		t.RegisterRequestHandler(protocol.MethodReadResource, server.handleReadResource)

		if server.capabilities[string(protocol.CapabilityResourceSubscriptions)] {
			t.RegisterRequestHandler(protocol.MethodSubscribeResource, server.handleSubscribeResource)
		}
	}

	if server.capabilities[string(protocol.CapabilityPrompts)] {
		t.RegisterRequestHandler(protocol.MethodListPrompts, server.handleListPrompts)
		t.RegisterRequestHandler(protocol.MethodGetPrompt, server.handleGetPrompt)
	}

	if server.capabilities[string(protocol.CapabilityComplete)] {
		t.RegisterRequestHandler(protocol.MethodComplete, server.handleComplete)
	}

	if server.capabilities[string(protocol.CapabilityRoots)] {
		t.RegisterRequestHandler(protocol.MethodListRoots, server.handleListRoots)
	}

	return server
}

// Start initializes and starts the server (blocking)
func (s *Server) Start(ctx context.Context) error {
	// Initialize transport
	if err := s.transport.Initialize(ctx); err != nil {
		return fmt.Errorf("transport initialization failed: %w", err)
	}

	s.logger.Info("Server starting with capabilities: %v", s.capabilities)

	// Start transport (blocking)
	return s.transport.Start(ctx)
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	s.cancel()
	return s.transport.Stop(context.Background())
}

// NotifyToolsChanged notifies the client about tool changes
func (s *Server) NotifyToolsChanged(added []protocol.Tool, removed []string, modified []protocol.Tool) error {
	if !s.isInitialized() {
		return errors.New("server not initialized")
	}

	params := &protocol.ToolsChangedParams{
		Added:    added,
		Removed:  removed,
		Modified: modified,
	}

	return s.transport.SendNotification(context.Background(), protocol.MethodToolsChanged, params)
}

// NotifyResourcesChanged notifies the client about resource changes
func (s *Server) NotifyResourcesChanged(uri string, resources []protocol.Resource, removed []string, added []protocol.Resource, modified []protocol.Resource) error {
	if !s.isInitialized() {
		return errors.New("server not initialized")
	}

	params := &protocol.ResourcesChangedParams{
		URI:       uri,
		Resources: resources,
		Removed:   removed,
		Added:     added,
		Modified:  modified,
	}

	return s.transport.SendNotification(context.Background(), protocol.MethodResourcesChanged, params)
}

// NotifyResourceUpdated notifies the client about a resource update
func (s *Server) NotifyResourceUpdated(uri string, contents *protocol.ResourceContents, deleted bool) error {
	if !s.isInitialized() {
		return errors.New("server not initialized")
	}

	params := &protocol.ResourceUpdatedParams{
		URI:     uri,
		Deleted: deleted,
	}

	if contents != nil && !deleted {
		params.Contents = *contents
	}

	return s.transport.SendNotification(context.Background(), protocol.MethodResourceUpdated, params)
}

// NotifyPromptsChanged notifies the client about prompt changes
func (s *Server) NotifyPromptsChanged(added []protocol.Prompt, removed []string, modified []protocol.Prompt) error {
	if !s.isInitialized() {
		return errors.New("server not initialized")
	}

	params := &protocol.PromptsChangedParams{
		Added:    added,
		Removed:  removed,
		Modified: modified,
	}

	return s.transport.SendNotification(context.Background(), protocol.MethodPromptsChanged, params)
}

// NotifyRootsChanged notifies the client about root changes
func (s *Server) NotifyRootsChanged(added []protocol.Root, removed []string, modified []protocol.Root) error {
	if !s.isInitialized() {
		return errors.New("server not initialized")
	}

	params := &protocol.RootsChangedParams{
		Added:    added,
		Removed:  removed,
		Modified: modified,
	}

	return s.transport.SendNotification(context.Background(), protocol.MethodRootsChanged, params)
}

// SendLog sends a log message to the client
func (s *Server) SendLog(level protocol.LogLevel, message string, source string, data interface{}) error {
	if !s.isInitialized() {
		return errors.New("server not initialized")
	}

	var dataJSON json.RawMessage
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal log data: %w", err)
		}
		dataJSON = bytes
	}

	params := &protocol.LogParams{
		Level:   level,
		Message: message,
		Source:  source,
		Time:    time.Now(),
		Data:    dataJSON,
	}

	return s.transport.SendNotification(context.Background(), protocol.MethodLog, params)
}

// SendProgress sends a progress notification to the client
func (s *Server) SendProgress(id interface{}, message string, percent float64, completed bool) error {
	if !s.isInitialized() {
		return errors.New("server not initialized")
	}

	params := &protocol.ProgressParams{
		ID:        id,
		Message:   message,
		Percent:   percent,
		Completed: completed,
	}

	return s.transport.SendNotification(context.Background(), protocol.MethodProgress, params)
}

// isInitialized checks if the server is initialized
func (s *Server) isInitialized() bool {
	s.initializedLock.RLock()
	defer s.initializedLock.RUnlock()
	return s.initialized
}

// getClientInfo safely returns the client info
func (s *Server) getClientInfo() *protocol.ClientInfo {
	s.initializedLock.RLock()
	defer s.initializedLock.RUnlock()
	return s.clientInfo
}

// Request handlers

func (s *Server) handleInitialize(ctx context.Context, params interface{}) (interface{}, error) {
	var initParams protocol.InitializeParams
	if err := parseParams(params, &initParams); err != nil {
		return nil, fmt.Errorf("invalid initialize params: %w", err)
	}

	s.logger.Info("Initializing connection with client: %s %s", initParams.Name, initParams.Version)

	// Protect all shared state modifications with the lock
	s.initializedLock.Lock()
	s.clientInfo = initParams.ClientInfo
	s.initialized = true
	s.initializedLock.Unlock()

	// Store client capabilities for future reference
	// (useful for determining if client supports sampling, etc.)

	result := &protocol.InitializeResult{
		ProtocolVersion: protocol.ProtocolRevision,
		Name:            s.name,
		Version:         s.version,
		Capabilities:    s.capabilities,
		ServerInfo: &protocol.ServerInfo{
			Name:        s.name,
			Version:     s.version,
			Description: s.description,
			Homepage:    s.homepage,
		},
		FeatureOptions: s.featureOptions,
	}

	return result, nil
}

func (s *Server) handleInitialized(ctx context.Context, params interface{}) error {
	s.initializedLock.Lock()
	s.initialized = true
	s.initializedLock.Unlock()

	s.logger.Info("Connection initialized")
	return nil
}

func (s *Server) handleSetCapability(ctx context.Context, params interface{}) (interface{}, error) {
	var capParams protocol.SetCapabilityParams
	if err := parseParams(params, &capParams); err != nil {
		return nil, fmt.Errorf("invalid setCapability params: %w", err)
	}

	// Check if this capability can be dynamically changed
	// For now, just log and accept but don't actually change anything
	s.logger.Info("Client requested capability change: %s = %v", capParams.Capability, capParams.Value)

	return &protocol.SetCapabilityResult{Success: true}, nil
}

func (s *Server) handleCancel(ctx context.Context, params interface{}) (interface{}, error) {
	var cancelParams protocol.CancelParams
	if err := parseParams(params, &cancelParams); err != nil {
		return nil, fmt.Errorf("invalid cancel params: %w", err)
	}

	// Handle cancellation (implementation-specific)
	s.logger.Info("Received cancel request for ID: %v", cancelParams.ID)

	return &protocol.CancelResult{Cancelled: false}, nil
}

func (s *Server) handlePing(ctx context.Context, params interface{}) (interface{}, error) {
	var pingParams protocol.PingParams
	if err := parseParams(params, &pingParams); err != nil {
		return nil, fmt.Errorf("invalid ping params: %w", err)
	}

	timestamp := pingParams.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	}

	return &protocol.PingResult{Timestamp: timestamp}, nil
}

func (s *Server) handleSetLogLevel(ctx context.Context, params interface{}) (interface{}, error) {
	if !s.capabilities[string(protocol.CapabilityLogging)] {
		return nil, errors.New("logging capability not supported")
	}

	var logParams protocol.SetLogLevelParams
	if err := parseParams(params, &logParams); err != nil {
		return nil, fmt.Errorf("invalid setLogLevel params: %w", err)
	}

	// Set log level (implementation-specific)
	s.logger.Info("Setting log level to: %s", logParams.Level)

	return &protocol.SetLogLevelResult{Success: true}, nil
}

func (s *Server) handleListTools(ctx context.Context, params interface{}) (interface{}, error) {
	if s.toolsProvider == nil {
		return nil, errors.New("tools provider not configured")
	}

	var listParams protocol.ListToolsParams
	if err := parseParams(params, &listParams); err != nil {
		return nil, fmt.Errorf("invalid listTools params: %w", err)
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	tools, totalCount, nextCursor, hasMore, err := s.toolsProvider.ListTools(ctx, listParams.Category, &pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	return &protocol.ListToolsResult{
		Tools: tools,
		PaginationResult: protocol.PaginationResult{
			TotalCount: totalCount,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *Server) handleCallTool(ctx context.Context, params interface{}) (interface{}, error) {
	if s.toolsProvider == nil {
		return nil, errors.New("tools provider not configured")
	}

	var callParams protocol.CallToolParams
	if err := parseParams(params, &callParams); err != nil {
		return nil, fmt.Errorf("invalid callTool params: %w", err)
	}

	result, err := s.toolsProvider.CallTool(ctx, callParams.Name, callParams.Input, callParams.Context)
	if err != nil {
		// Return a valid result with an error message rather than failing the request
		return &protocol.CallToolResult{
			Error: err.Error(),
		}, nil
	}

	return result, nil
}

func (s *Server) handleListResources(ctx context.Context, params interface{}) (interface{}, error) {
	if s.resourcesProvider == nil {
		return nil, errors.New("resources provider not configured")
	}

	var listParams protocol.ListResourcesParams
	if err := parseParams(params, &listParams); err != nil {
		return nil, fmt.Errorf("invalid listResources params: %w", err)
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	resources, templates, totalCount, nextCursor, hasMore, err := s.resourcesProvider.ListResources(ctx, listParams.URI, listParams.Recursive, &pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	return &protocol.ListResourcesResult{
		Resources: resources,
		Templates: templates,
		PaginationResult: protocol.PaginationResult{
			TotalCount: totalCount,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *Server) handleReadResource(ctx context.Context, params interface{}) (interface{}, error) {
	if s.resourcesProvider == nil {
		return nil, errors.New("resources provider not configured")
	}

	var readParams protocol.ReadResourceParams
	if err := parseParams(params, &readParams); err != nil {
		return nil, fmt.Errorf("invalid readResource params: %w", err)
	}

	contents, err := s.resourcesProvider.ReadResource(ctx, readParams.URI, readParams.TemplateParams, readParams.Range)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	return &protocol.ReadResourceResult{
		Contents: *contents,
	}, nil
}

func (s *Server) handleSubscribeResource(ctx context.Context, params interface{}) (interface{}, error) {
	if s.resourcesProvider == nil {
		return nil, errors.New("resources provider not configured")
	}

	if !s.capabilities[string(protocol.CapabilityResourceSubscriptions)] {
		return nil, errors.New("resource subscriptions not supported")
	}

	var subParams protocol.SubscribeResourceParams
	if err := parseParams(params, &subParams); err != nil {
		return nil, fmt.Errorf("invalid subscribeResource params: %w", err)
	}

	success, err := s.resourcesProvider.SubscribeResource(ctx, subParams.URI, subParams.Recursive)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to resource: %w", err)
	}

	return &protocol.SubscribeResourceResult{
		Success: success,
	}, nil
}

func (s *Server) handleListPrompts(ctx context.Context, params interface{}) (interface{}, error) {
	if s.promptsProvider == nil {
		return nil, errors.New("prompts provider not configured")
	}

	var listParams protocol.ListPromptsParams
	if err := parseParams(params, &listParams); err != nil {
		return nil, fmt.Errorf("invalid listPrompts params: %w", err)
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	prompts, totalCount, nextCursor, hasMore, err := s.promptsProvider.ListPrompts(ctx, listParams.Tag, &pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}

	return &protocol.ListPromptsResult{
		Prompts: prompts,
		PaginationResult: protocol.PaginationResult{
			TotalCount: totalCount,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *Server) handleGetPrompt(ctx context.Context, params interface{}) (interface{}, error) {
	if s.promptsProvider == nil {
		return nil, errors.New("prompts provider not configured")
	}

	var getParams protocol.GetPromptParams
	if err := parseParams(params, &getParams); err != nil {
		return nil, fmt.Errorf("invalid getPrompt params: %w", err)
	}

	prompt, err := s.promptsProvider.GetPrompt(ctx, getParams.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	return &protocol.GetPromptResult{
		Prompt: *prompt,
	}, nil
}

func (s *Server) handleComplete(ctx context.Context, params interface{}) (interface{}, error) {
	if s.completionProvider == nil {
		return nil, errors.New("completion provider not configured")
	}

	var completeParams protocol.CompleteParams
	if err := parseParams(params, &completeParams); err != nil {
		return nil, fmt.Errorf("invalid complete params: %w", err)
	}

	result, err := s.completionProvider.Complete(ctx, &completeParams)
	if err != nil {
		return nil, fmt.Errorf("failed to generate completion: %w", err)
	}

	return result, nil
}

func (s *Server) handleListRoots(ctx context.Context, params interface{}) (interface{}, error) {
	if s.rootsProvider == nil {
		return nil, errors.New("roots provider not configured")
	}

	var listParams protocol.ListRootsParams
	if err := parseParams(params, &listParams); err != nil {
		return nil, fmt.Errorf("invalid listRoots params: %w", err)
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	roots, totalCount, nextCursor, hasMore, err := s.rootsProvider.ListRoots(ctx, listParams.Tag, &pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list roots: %w", err)
	}

	return &protocol.ListRootsResult{
		Roots: roots,
		PaginationResult: protocol.PaginationResult{
			TotalCount: totalCount,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

// Utility functions

func parseParams(params interface{}, target interface{}) error {
	// Handle nil params case explicitly
	if params == nil {
		return fmt.Errorf("params cannot be nil")
	}

	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal params: %w", err)
	}

	return nil
}
