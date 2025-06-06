package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/logging"
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

	// Request tracking for cancellation
	activeRequests     map[string]context.CancelFunc
	activeRequestsLock sync.RWMutex

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

// DefaultLogger is a simple implementation of Logger using structured logging
type DefaultLogger struct {
	logger logging.Logger
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger() *DefaultLogger {
	// Create a structured logger with text output
	logger := logging.New(nil, logging.NewTextFormatter()).WithFields(
		logging.String("component", "mcp-server"),
	)
	logger.SetLevel(logging.InfoLevel)
	return &DefaultLogger{logger: logger}
}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	logging.NewLegacyAdapter(l.logger).Debug(msg, args...)
}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	logging.NewLegacyAdapter(l.logger).Info(msg, args...)
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	logging.NewLegacyAdapter(l.logger).Warn(msg, args...)
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	logging.NewLegacyAdapter(l.logger).Error(msg, args...)
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

// WithStructuredLogger sets a structured logger
func WithStructuredLogger(logger logging.Logger) ServerOption {
	return func(s *Server) {
		// Wrap the structured logger with the legacy adapter
		s.logger = &DefaultLogger{logger: logger}
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
		logger:         NewDefaultLogger(),
		ctx:            ctx,
		cancel:         cancel,
		activeRequests: make(map[string]context.CancelFunc),
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
		return mcperrors.TransportError("server", "initialization", err).
			WithContext(&mcperrors.Context{
				Component: "Server",
				Operation: "Start",
				Timestamp: time.Now(),
			}).
			WithDetail(fmt.Sprintf("Transport type: %T", s.transport))
	}

	s.logger.Info("Server starting with capabilities: %v", s.capabilities)

	// Start transport (blocking)
	return s.transport.Start(ctx)
}

// Stop gracefully shuts down the server
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

// NotifyToolsChanged notifies the client about tool changes
func (s *Server) NotifyToolsChanged(added []protocol.Tool, removed []string, modified []protocol.Tool) error {
	if err := s.requireInitialized("NotifyToolsChanged"); err != nil {
		return err
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
	if err := s.requireInitialized("NotifyResourcesChanged"); err != nil {
		return err
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
	if err := s.requireInitialized("NotifyResourceUpdated"); err != nil {
		return err
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
	if err := s.requireInitialized("NotifyPromptsChanged"); err != nil {
		return err
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
	if err := s.requireInitialized("SendLog"); err != nil {
		return err
	}

	var dataJSON json.RawMessage
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return mcperrors.CreateInternalError("marshal_log_data", err).
				WithContext(&mcperrors.Context{
					Component: "Server",
					Operation: "SendLog",
					Timestamp: time.Now(),
				}).WithDetail(fmt.Sprintf("Log level: %s, data type: %T", level, data))
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
	if err := s.requireInitialized("SendProgress"); err != nil {
		return err
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

// Helper functions for error handling

// createRequestContext creates error context for request handling
func (s *Server) createRequestContext(method string, requestID interface{}) *mcperrors.Context {
	return &mcperrors.Context{
		RequestID: fmt.Sprintf("%v", requestID),
		Method:    method,
		Component: "Server",
		Operation: method,
		Timestamp: time.Now(),
	}
}

// validateParams validates and parses request parameters with structured errors
func (s *Server) validateParams(params interface{}, target interface{}, method string) error {
	if params == nil {
		return mcperrors.MissingParameter("params").WithContext(
			s.createRequestContext(method, nil),
		)
	}

	data, err := json.Marshal(params)
	if err != nil {
		return mcperrors.CreateInternalError("marshal_params", err).
			WithContext(s.createRequestContext(method, nil)).
			WithDetail("Failed to process request parameters")
	}

	if err := json.Unmarshal(data, target); err != nil {
		return mcperrors.InvalidParameter("params", params, fmt.Sprintf("%T", target)).
			WithContext(s.createRequestContext(method, nil)).
			WithDetail(err.Error())
	}

	return nil
}

// requireInitialized checks if server is initialized and returns structured error
func (s *Server) requireInitialized(operation string) error {
	if !s.isInitialized() {
		return mcperrors.NewError(
			mcperrors.CodeServerNotReady,
			"Server must be initialized before this operation",
			mcperrors.CategoryInternal,
			mcperrors.SeverityError,
		).WithContext(&mcperrors.Context{
			Component: "Server",
			Operation: operation,
			Timestamp: time.Now(),
		})
	}
	return nil
}

// requireProvider checks if a provider is configured and returns structured error
func (s *Server) requireProvider(providerType string, provider interface{}, method string) error {
	if provider == nil {
		return mcperrors.ProviderNotConfigured(providerType).
			WithContext(s.createRequestContext(method, nil))
	}
	return nil
}

// trackRequest adds a request to the active requests map with a cancellation function
func (s *Server) trackRequest(requestID string, cancelFunc context.CancelFunc) {
	s.activeRequestsLock.Lock()
	defer s.activeRequestsLock.Unlock()
	s.activeRequests[requestID] = cancelFunc
	s.logger.Debug("Tracking request ID: %s", requestID)
}

// completeRequest removes a request from the active requests map
func (s *Server) completeRequest(requestID string) {
	s.activeRequestsLock.Lock()
	defer s.activeRequestsLock.Unlock()
	if cancelFunc, exists := s.activeRequests[requestID]; exists {
		delete(s.activeRequests, requestID)
		s.logger.Debug("Completed request ID: %s", requestID)
		// Note: We don't call cancelFunc here as the request completed normally
		_ = cancelFunc // Silence unused warning
	}
}

// cancelRequest cancels a specific request by ID
func (s *Server) cancelRequest(requestID string) bool {
	s.activeRequestsLock.Lock()
	defer s.activeRequestsLock.Unlock()
	if cancelFunc, exists := s.activeRequests[requestID]; exists {
		cancelFunc()
		delete(s.activeRequests, requestID)
		s.logger.Info("Cancelled request ID: %s", requestID)
		return true
	}
	s.logger.Warn("Request ID not found for cancellation: %s", requestID)
	return false
}

// RequestContext holds the context for a request with its ID
type RequestContext struct {
	ID string
	context.Context
}

// Request handlers

func (s *Server) handleInitialize(ctx context.Context, params interface{}) (interface{}, error) {
	var initParams protocol.InitializeParams
	if err := s.validateParams(params, &initParams, protocol.MethodInitialize); err != nil {
		return nil, err
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
	if err := s.validateParams(params, &capParams, protocol.MethodSetCapability); err != nil {
		return nil, err
	}

	// Check if this capability can be dynamically changed
	// For now, just log and accept but don't actually change anything
	s.logger.Info("Client requested capability change: %s = %v", capParams.Capability, capParams.Value)

	return &protocol.SetCapabilityResult{Success: true}, nil
}

func (s *Server) handleCancel(ctx context.Context, params interface{}) (interface{}, error) {
	var cancelParams protocol.CancelParams
	if err := s.validateParams(params, &cancelParams, protocol.MethodCancel); err != nil {
		return nil, err
	}

	// Convert the request ID to string for consistency
	requestID := fmt.Sprintf("%v", cancelParams.ID)

	// Cancel the request
	cancelled := s.cancelRequest(requestID)

	return &protocol.CancelResult{Cancelled: cancelled}, nil
}

func (s *Server) handlePing(ctx context.Context, params interface{}) (interface{}, error) {
	var pingParams protocol.PingParams
	if err := s.validateParams(params, &pingParams, protocol.MethodPing); err != nil {
		return nil, err
	}

	timestamp := pingParams.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	}

	return &protocol.PingResult{Timestamp: timestamp}, nil
}

func (s *Server) handleSetLogLevel(ctx context.Context, params interface{}) (interface{}, error) {
	if !s.capabilities[string(protocol.CapabilityLogging)] {
		return nil, mcperrors.CapabilityRequired(string(protocol.CapabilityLogging)).
			WithContext(s.createRequestContext(protocol.MethodSetLogLevel, nil))
	}

	var logParams protocol.SetLogLevelParams
	if err := s.validateParams(params, &logParams, protocol.MethodSetLogLevel); err != nil {
		return nil, err
	}

	// Set log level (implementation-specific)
	s.logger.Info("Setting log level to: %s", logParams.Level)

	return &protocol.SetLogLevelResult{Success: true}, nil
}

func (s *Server) handleListTools(ctx context.Context, params interface{}) (interface{}, error) {
	if err := s.requireProvider("tools", s.toolsProvider, protocol.MethodListTools); err != nil {
		return nil, err
	}

	var listParams protocol.ListToolsParams
	if err := s.validateParams(params, &listParams, protocol.MethodListTools); err != nil {
		return nil, err
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	tools, totalCount, nextCursor, hasMore, err := s.toolsProvider.ListTools(ctx, listParams.Category, &pagination)
	if err != nil {
		return nil, mcperrors.ProviderError("tools", "ListTools", err).
			WithContext(s.createRequestContext(protocol.MethodListTools, nil)).
			WithDetail(fmt.Sprintf("Category: %s", listParams.Category))
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
	if err := s.requireProvider("tools", s.toolsProvider, protocol.MethodCallTool); err != nil {
		return nil, err
	}

	var callParams protocol.CallToolParams
	if err := s.validateParams(params, &callParams, protocol.MethodCallTool); err != nil {
		return nil, err
	}

	// Call the tool with the context, which may be cancelled
	result, err := s.toolsProvider.CallTool(ctx, callParams.Name, callParams.Input, callParams.Context)
	if err != nil {
		// Check if the error is due to context cancellation
		if ctx.Err() == context.Canceled {
			return &protocol.CallToolResult{
				Error: "Tool call was cancelled",
			}, nil
		}
		// Return a valid result with an error message rather than failing the request
		return &protocol.CallToolResult{
			Error: err.Error(),
		}, nil
	}

	return result, nil
}

func (s *Server) handleListResources(ctx context.Context, params interface{}) (interface{}, error) {
	if err := s.requireProvider("resources", s.resourcesProvider, protocol.MethodListResources); err != nil {
		return nil, err
	}

	var listParams protocol.ListResourcesParams
	if err := s.validateParams(params, &listParams, protocol.MethodListResources); err != nil {
		return nil, err
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	resources, templates, totalCount, nextCursor, hasMore, err := s.resourcesProvider.ListResources(ctx, listParams.URI, listParams.Recursive, &pagination)
	if err != nil {
		return nil, mcperrors.ProviderError("resources", "ListResources", err).
			WithContext(s.createRequestContext(protocol.MethodListResources, nil)).
			WithDetail(fmt.Sprintf("URI: %s, Recursive: %t", listParams.URI, listParams.Recursive))
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
	if err := s.requireProvider("resources", s.resourcesProvider, protocol.MethodReadResource); err != nil {
		return nil, err
	}

	var readParams protocol.ReadResourceParams
	if err := s.validateParams(params, &readParams, protocol.MethodReadResource); err != nil {
		return nil, err
	}

	contents, err := s.resourcesProvider.ReadResource(ctx, readParams.URI, readParams.TemplateParams, readParams.Range)
	if err != nil {
		return nil, mcperrors.ProviderError("resources", "ReadResource", err).
			WithContext(s.createRequestContext(protocol.MethodReadResource, nil)).
			WithDetail(fmt.Sprintf("URI: %s", readParams.URI))
	}

	return &protocol.ReadResourceResult{
		Contents: *contents,
	}, nil
}

func (s *Server) handleSubscribeResource(ctx context.Context, params interface{}) (interface{}, error) {
	if err := s.requireProvider("resources", s.resourcesProvider, protocol.MethodSubscribeResource); err != nil {
		return nil, err
	}

	if !s.capabilities[string(protocol.CapabilityResourceSubscriptions)] {
		return nil, mcperrors.CapabilityRequired(string(protocol.CapabilityResourceSubscriptions)).
			WithContext(s.createRequestContext(protocol.MethodSubscribeResource, nil))
	}

	var subParams protocol.SubscribeResourceParams
	if err := s.validateParams(params, &subParams, protocol.MethodSubscribeResource); err != nil {
		return nil, err
	}

	success, err := s.resourcesProvider.SubscribeResource(ctx, subParams.URI, subParams.Recursive)
	if err != nil {
		return nil, mcperrors.ProviderError("resources", "SubscribeResource", err).
			WithContext(s.createRequestContext(protocol.MethodSubscribeResource, nil)).
			WithDetail(fmt.Sprintf("URI: %s, Recursive: %t", subParams.URI, subParams.Recursive))
	}

	return &protocol.SubscribeResourceResult{
		Success: success,
	}, nil
}

func (s *Server) handleListPrompts(ctx context.Context, params interface{}) (interface{}, error) {
	if err := s.requireProvider("prompts", s.promptsProvider, protocol.MethodListPrompts); err != nil {
		return nil, err
	}

	var listParams protocol.ListPromptsParams
	if err := s.validateParams(params, &listParams, protocol.MethodListPrompts); err != nil {
		return nil, err
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	prompts, totalCount, nextCursor, hasMore, err := s.promptsProvider.ListPrompts(ctx, listParams.Tag, &pagination)
	if err != nil {
		return nil, mcperrors.ProviderError("prompts", "ListPrompts", err).
			WithContext(s.createRequestContext(protocol.MethodListPrompts, nil)).
			WithDetail(fmt.Sprintf("Tag: %s", listParams.Tag))
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
	if err := s.requireProvider("prompts", s.promptsProvider, protocol.MethodGetPrompt); err != nil {
		return nil, err
	}

	var getParams protocol.GetPromptParams
	if err := s.validateParams(params, &getParams, protocol.MethodGetPrompt); err != nil {
		return nil, err
	}

	prompt, err := s.promptsProvider.GetPrompt(ctx, getParams.ID)
	if err != nil {
		return nil, mcperrors.ProviderError("prompts", "GetPrompt", err).
			WithContext(s.createRequestContext(protocol.MethodGetPrompt, nil)).
			WithDetail(fmt.Sprintf("ID: %s", getParams.ID))
	}

	return &protocol.GetPromptResult{
		Prompt: *prompt,
	}, nil
}

func (s *Server) handleComplete(ctx context.Context, params interface{}) (interface{}, error) {
	if err := s.requireProvider("completion", s.completionProvider, protocol.MethodComplete); err != nil {
		return nil, err
	}

	var completeParams protocol.CompleteParams
	if err := s.validateParams(params, &completeParams, protocol.MethodComplete); err != nil {
		return nil, err
	}

	result, err := s.completionProvider.Complete(ctx, &completeParams)
	if err != nil {
		return nil, mcperrors.ProviderError("completion", "Complete", err).
			WithContext(s.createRequestContext(protocol.MethodComplete, nil))
	}

	return result, nil
}

func (s *Server) handleListRoots(ctx context.Context, params interface{}) (interface{}, error) {
	if err := s.requireProvider("roots", s.rootsProvider, protocol.MethodListRoots); err != nil {
		return nil, err
	}

	var listParams protocol.ListRootsParams
	if err := s.validateParams(params, &listParams, protocol.MethodListRoots); err != nil {
		return nil, err
	}

	pagination := protocol.PaginationParams{
		Limit:  listParams.Limit,
		Cursor: listParams.Cursor,
	}

	roots, totalCount, nextCursor, hasMore, err := s.rootsProvider.ListRoots(ctx, listParams.Tag, &pagination)
	if err != nil {
		return nil, mcperrors.ProviderError("roots", "ListRoots", err).
			WithContext(s.createRequestContext(protocol.MethodListRoots, nil)).
			WithDetail(fmt.Sprintf("Tag: %s", listParams.Tag))
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

// parseParams is kept for backward compatibility with tests
// New code should use validateParams method for structured errors
func parseParams(params interface{}, target interface{}) error {
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
