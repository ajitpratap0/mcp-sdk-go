// Package client provides the client-side implementation of the MCP protocol,
// allowing applications to connect to MCP servers and consume their capabilities.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/logging"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/pagination"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// Client is the main interface for an MCP client.
type Client interface {
	// Initialize initializes the client with the provided context.
	// It performs capability negotiation with the server.
	Initialize(ctx context.Context) error

	// Start starts the client's transport.
	// This begins the message handling loop.
	Start(ctx context.Context) error

	// InitializeAndStart combines Initialize and Start operations for convenience.
	InitializeAndStart(ctx context.Context) error

	// Close closes the client and its transport.
	// Any ongoing operations will be cancelled.
	Close() error

	// HasCapability checks if a specific capability is supported by the server.
	HasCapability(capability protocol.CapabilityType) bool

	// Capabilities returns the map of all supported capabilities.
	// The keys are capability names and the values indicate whether they are enabled.
	Capabilities() map[string]bool

	// ListTools lists available tools from the server.
	// The category parameter can be used to filter tools by category.
	// Pagination can be used to limit results or fetch subsequent pages.
	ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, *protocol.PaginationResult, error)

	// ListAllTools automatically paginates to retrieve all tools matching the category.
	// This is a convenience method that handles pagination internally.
	ListAllTools(ctx context.Context, category string) ([]protocol.Tool, error)

	// CallTool invokes a tool with the given name, input, and context.
	// The input and context are typically JSON-encoded objects.
	CallTool(ctx context.Context, name string, input interface{}, toolContext interface{}) (*protocol.CallToolResult, error)

	// CallToolStreaming invokes a tool with streaming updates.
	// It's similar to CallTool but provides incremental updates via the handler function.
	CallToolStreaming(ctx context.Context, name string, input interface{}, toolContext interface{}, updateHandler StreamingUpdateHandler) (*protocol.CallToolResult, error)

	// ListResources lists available resources from the server.
	// The uri parameter specifies the resource URI to list.
	// The recursive parameter indicates whether to list resources recursively.
	// Pagination can be used to limit results or fetch subsequent pages.
	ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, *protocol.PaginationResult, error)

	// ListAllResources automatically paginates to retrieve all resources.
	// This is a convenience method that handles pagination internally.
	ListAllResources(ctx context.Context, uri string, recursive bool) ([]protocol.Resource, []protocol.ResourceTemplate, error)

	// ReadResource retrieves a specific resource by URI.
	// The templateParams parameter can be used to customize template resources.
	// The rangeOpt parameter can be used to request a specific range of the resource.
	ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error)

	// SubscribeResource subscribes to resource updates.
	// After subscribing, the ResourceChangedCallback will be called when the resource changes.
	// The recursive parameter indicates whether to subscribe to child resources.
	SubscribeResource(ctx context.Context, uri string, recursive bool) error

	// ListPrompts lists available prompts from the server.
	// The tag parameter can be used to filter prompts by tag.
	// Pagination can be used to limit results or fetch subsequent pages.
	ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, *protocol.PaginationResult, error)

	// ListAllPrompts automatically paginates to retrieve all prompts matching the tag.
	// This is a convenience method that handles pagination internally.
	ListAllPrompts(ctx context.Context, tag string) ([]protocol.Prompt, error)

	// GetPrompt retrieves a specific prompt by ID.
	GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error)

	// ListRoots lists root resources from the server.
	// The tag parameter can be used to filter roots by tag.
	// Pagination can be used to limit results or fetch subsequent pages.
	ListRoots(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Root, *protocol.PaginationResult, error)

	// ListAllRoots automatically paginates to retrieve all roots matching the tag.
	// This is a convenience method that handles pagination internally.
	ListAllRoots(ctx context.Context, tag string) ([]protocol.Root, error)

	// Complete requests a completion from the server.
	// This is used for text completion operations with AI models.
	Complete(ctx context.Context, params *protocol.CompleteParams) (*protocol.CompleteResult, error)

	// SetSamplingCallback sets a callback function for sampling events.
	// This is used to receive intermediate results during completion.
	SetSamplingCallback(callback SamplingCallback)

	// SetResourceChangedCallback sets a callback function for resource change notifications.
	// This is called when a subscribed resource changes.
	SetResourceChangedCallback(callback ResourceChangedCallback)

	// ServerInfo returns information about the server.
	ServerInfo() *protocol.ServerInfo

	// Cancel cancels an ongoing operation by ID.
	Cancel(ctx context.Context, id interface{}) (bool, error)

	// Ping checks if the server is responding.
	Ping(ctx context.Context) (*protocol.PingResult, error)

	// SetLogLevel sets the logging level for the client.
	SetLogLevel(ctx context.Context, level protocol.LogLevel) error
}

// Option represents a client configuration option.
// Options are used to configure a client during creation.
type Option func(*ClientConfig)

// SamplingCallback is called when sampling events are received.
// These events provide intermediate results during completion operations.
type SamplingCallback func(protocol.SamplingEvent)

// ResourceChangedCallback is called when resource change notifications are received.
// It receives the resource ID and the new resource data.
type ResourceChangedCallback func(string, interface{})

// MessageHandler handles custom JSON-RPC method calls.
// It takes a context and the raw JSON parameters, and returns a result or an error.
type MessageHandler func(context.Context, json.RawMessage) (interface{}, error)

// Client represents an MCP client
type ClientConfig struct {
	transport       transport.Transport
	name            string
	version         string
	capabilities    map[string]bool
	initialized     bool
	initializedLock sync.RWMutex
	serverInfo      *protocol.ServerInfo
	featureOptions  map[string]interface{}
	ctx             context.Context
	cancel          context.CancelFunc
	logger          logging.Logger

	// Callback management - thread-safe storage for client callbacks
	samplingCallback        SamplingCallback
	resourceChangedCallback ResourceChangedCallback
	callbackMutex           sync.RWMutex
}

// ClientOption defines options for creating a client
type ClientOption func(*ClientConfig)

// WithName sets the client name
func WithName(name string) ClientOption {
	return func(c *ClientConfig) {
		c.name = name
	}
}

// WithVersion sets the client version
func WithVersion(version string) ClientOption {
	return func(c *ClientConfig) {
		c.version = version
	}
}

// WithCapability enables a client capability
func WithCapability(capability protocol.CapabilityType, enabled bool) ClientOption {
	return func(c *ClientConfig) {
		c.capabilities[string(capability)] = enabled
	}
}

// WithFeatureOptions sets feature options for the client
func WithFeatureOptions(options map[string]interface{}) ClientOption {
	return func(c *ClientConfig) {
		for k, v := range options {
			c.featureOptions[k] = v
		}
	}
}

// WithLogger sets the logger
func WithLogger(logger logging.Logger) ClientOption {
	return func(c *ClientConfig) {
		c.logger = logger
	}
}

// New creates a new MCP client
func New(t transport.Transport, options ...ClientOption) *ClientConfig {
	ctx, cancel := context.WithCancel(context.Background())

	client := &ClientConfig{
		transport:      t,
		name:           "go-mcp-client",
		version:        "1.0.0",
		capabilities:   make(map[string]bool),
		featureOptions: make(map[string]interface{}),
		ctx:            ctx,
		cancel:         cancel,
	}

	// Apply options
	for _, option := range options {
		option(client)
	}

	// Initialize logger if not set
	if client.logger == nil {
		client.logger = logging.New(nil, logging.NewTextFormatter()).WithFields(
			logging.String("component", "mcp-client"),
		)
		client.logger.SetLevel(logging.InfoLevel)
	}

	// Default capabilities
	if _, ok := client.capabilities[string(protocol.CapabilitySampling)]; !ok {
		client.capabilities[string(protocol.CapabilitySampling)] = true
	}

	// Register notification handlers
	t.RegisterNotificationHandler(protocol.MethodToolsChanged, client.handleToolsChanged)
	t.RegisterNotificationHandler(protocol.MethodResourcesChanged, client.handleResourcesChanged)
	t.RegisterNotificationHandler(protocol.MethodResourceUpdated, client.handleResourceUpdated)
	t.RegisterNotificationHandler(protocol.MethodPromptsChanged, client.handlePromptsChanged)
	t.RegisterNotificationHandler(protocol.MethodRootsChanged, client.handleRootsChanged)
	t.RegisterNotificationHandler(protocol.MethodLog, client.handleLog)

	// Register request handlers
	t.RegisterRequestHandler(protocol.MethodSample, client.handleSample)
	t.RegisterRequestHandler(protocol.MethodCancel, client.handleCancel)
	t.RegisterRequestHandler(protocol.MethodPing, client.handlePing)

	return client
}

// Initialize initializes the client with the provided context.
func (c *ClientConfig) Initialize(ctx context.Context) error {
	// Check if already initialized
	c.initializedLock.RLock()
	initialized := c.initialized
	c.initializedLock.RUnlock()
	if initialized {
		return nil
	}

	// Initialize the transport
	if err := c.transport.Initialize(ctx); err != nil {
		return mcperrors.TransportError("client", "initialization", err).
			WithContext(&mcperrors.Context{
				Component: "Client",
				Operation: "Initialize",
				Timestamp: time.Now(),
			}).
			WithDetail(fmt.Sprintf("Transport type: %T", c.transport))
	}

	// Create request params
	params := &protocol.InitializeParams{
		ClientInfo: &protocol.ClientInfo{
			Name:    c.name,
			Version: c.version,
		},
		Capabilities: c.capabilities,
	}

	// Add the current OS information
	if runtime.GOOS != "" {
		params.ClientInfo.Platform = runtime.GOOS
	}

	// Add feature options if any
	if len(c.featureOptions) > 0 {
		params.FeatureOptions = c.featureOptions
	}

	// Send initialize request
	c.logger.Debug("Sending initialize request", logging.Any("capabilities", c.capabilities))
	result, err := c.transport.SendRequest(ctx, protocol.MethodInitialize, params)
	if err != nil {
		return mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodInitialize, nil)).
			WithDetail("Failed to send initialize request to server")
	}

	// Parse the result
	var initResult protocol.InitializeResult
	if err := c.validateAndParseResult(result, &initResult, protocol.MethodInitialize); err != nil {
		return err
	}

	// Store server info
	c.serverInfo = initResult.ServerInfo

	// Store server capabilities
	for cap, enabled := range initResult.Capabilities {
		if enabled {
			c.capabilities[cap] = true
		}
	}

	// Check if we have a streamable HTTP transport and if we received a session ID
	if t, ok := c.transport.(*transport.StreamableHTTPTransport); ok {
		// Use reflection to check if the transport has a sessionID
		val := reflect.ValueOf(t).Elem()
		sidField := val.FieldByName("sessionID")
		if sidField.IsValid() && sidField.String() != "" {
			c.logger.Debug("Transport has session ID after initialize", logging.String("session_id", sidField.String()))
		} else {
			c.logger.Debug("Transport does not have session ID after initialize")
		}
	}

	// Mark as initialized
	c.initializedLock.Lock()
	c.initialized = true
	c.initializedLock.Unlock()

	// Send initialized notification
	err = c.transport.SendNotification(ctx, protocol.MethodInitialized, nil)
	if err != nil {
		return mcperrors.TransportError("client", "send_notification", err).
			WithContext(c.createRequestContext(protocol.MethodInitialized, nil)).
			WithDetail("Failed to send initialized notification to server")
	}

	return nil
}

// Start starts the client's transport
func (c *ClientConfig) Start(ctx context.Context) error {
	return c.transport.Start(ctx)
}

// InitializeAndStart combines Initialize and Start operations for convenience
func (c *ClientConfig) InitializeAndStart(ctx context.Context) error {
	if err := c.Initialize(ctx); err != nil {
		return err
	}
	return c.Start(ctx)
}

// Close shuts down the client
func (c *ClientConfig) Close() error {
	c.cancel()
	return c.transport.Stop(context.Background())
}

// ServerInfo returns information about the connected server
func (c *ClientConfig) ServerInfo() *protocol.ServerInfo {
	return c.serverInfo
}

// HasCapability checks if the server supports a specific capability
func (c *ClientConfig) HasCapability(capability protocol.CapabilityType) bool {
	if c.serverInfo == nil {
		return false
	}

	enabled, ok := c.capabilities[string(capability)]
	return ok && enabled
}

// Capabilities returns all capabilities
func (c *ClientConfig) Capabilities() map[string]bool {
	return c.capabilities
}

// ListTools retrieves the list of tools from the server
func (c *ClientConfig) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, *protocol.PaginationResult, error) {
	if err := c.requireCapability(protocol.CapabilityTools, "ListTools"); err != nil {
		return nil, nil, err
	}

	// Validate pagination parameters
	if err := c.validatePagination(pagination); err != nil {
		return nil, nil, err
	}

	// Apply defaults and create request params
	paginationParams := c.applyPaginationDefaults(pagination)
	params := &protocol.ListToolsParams{
		Category:         category,
		PaginationParams: *paginationParams,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodListTools, params)
	if err != nil {
		return nil, nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodListTools, nil)).
			WithDetail(fmt.Sprintf("Category: %s", category))
	}

	var toolsResult protocol.ListToolsResult
	if err := c.validateAndParseResult(result, &toolsResult, protocol.MethodListTools); err != nil {
		return nil, nil, err
	}

	paginationResult := &protocol.PaginationResult{
		TotalCount: toolsResult.TotalCount,
		NextCursor: toolsResult.NextCursor,
		HasMore:    toolsResult.HasMore,
	}

	return toolsResult.Tools, paginationResult, nil
}

// CallTool invokes a tool on the server
func (c *ClientConfig) CallTool(ctx context.Context, name string, input interface{}, context interface{}) (*protocol.CallToolResult, error) {
	if err := c.requireCapability(protocol.CapabilityTools, "CallTool"); err != nil {
		return nil, err
	}

	var inputJSON, contextJSON json.RawMessage
	var err error

	if input != nil {
		if inputJSON, err = json.Marshal(input); err != nil {
			return nil, mcperrors.CreateInternalError("marshal_input", err).
				WithContext(c.createRequestContext(protocol.MethodCallTool, nil)).
				WithDetail(fmt.Sprintf("Tool: %s, input type: %T", name, input))
		}
	}

	if context != nil {
		if contextJSON, err = json.Marshal(context); err != nil {
			return nil, mcperrors.CreateInternalError("marshal_context", err).
				WithContext(c.createRequestContext(protocol.MethodCallTool, nil)).
				WithDetail(fmt.Sprintf("Tool: %s, context type: %T", name, context))
		}
	}

	params := &protocol.CallToolParams{
		Name:    name,
		Input:   inputJSON,
		Context: contextJSON,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodCallTool, params)
	if err != nil {
		return nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodCallTool, nil)).
			WithDetail(fmt.Sprintf("Tool: %s", name))
	}

	var toolResult protocol.CallToolResult
	if err := c.validateAndParseResult(result, &toolResult, protocol.MethodCallTool); err != nil {
		return nil, err
	}

	return &toolResult, nil
}

// ListResources retrieves the list of resources from the server
func (c *ClientConfig) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, *protocol.PaginationResult, error) {
	if err := c.requireCapability(protocol.CapabilityResources, "ListResources"); err != nil {
		return nil, nil, nil, err
	}

	// Validate pagination parameters
	if err := c.validatePagination(pagination); err != nil {
		return nil, nil, nil, err
	}

	// Apply defaults and create request params
	paginationParams := c.applyPaginationDefaults(pagination)
	params := &protocol.ListResourcesParams{
		URI:              uri,
		Recursive:        recursive,
		PaginationParams: *paginationParams,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodListResources, params)
	if err != nil {
		return nil, nil, nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodListResources, nil)).
			WithDetail(fmt.Sprintf("URI: %s, Recursive: %t", uri, recursive))
	}

	var resourcesResult protocol.ListResourcesResult
	if err := c.validateAndParseResult(result, &resourcesResult, protocol.MethodListResources); err != nil {
		return nil, nil, nil, err
	}

	paginationResult := &protocol.PaginationResult{
		TotalCount: resourcesResult.TotalCount,
		NextCursor: resourcesResult.NextCursor,
		HasMore:    resourcesResult.HasMore,
	}

	return resourcesResult.Resources, resourcesResult.Templates, paginationResult, nil
}

// ReadResource reads a resource from the server
func (c *ClientConfig) ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error) {
	if err := c.requireCapability(protocol.CapabilityResources, "ReadResource"); err != nil {
		return nil, err
	}

	params := &protocol.ReadResourceParams{
		URI:            uri,
		TemplateParams: templateParams,
		Range:          rangeOpt,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodReadResource, params)
	if err != nil {
		return nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodReadResource, nil)).
			WithDetail(fmt.Sprintf("URI: %s", uri))
	}

	var readResult protocol.ReadResourceResult
	if err := c.validateAndParseResult(result, &readResult, protocol.MethodReadResource); err != nil {
		return nil, err
	}

	return &readResult.Contents, nil
}

// SubscribeResource subscribes to resource changes
func (c *ClientConfig) SubscribeResource(ctx context.Context, uri string, recursive bool) error {
	if err := c.requireCapability(protocol.CapabilityResourceSubscriptions, "SubscribeResource"); err != nil {
		return err
	}

	params := &protocol.SubscribeResourceParams{
		URI:       uri,
		Recursive: recursive,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodSubscribeResource, params)
	if err != nil {
		return mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodSubscribeResource, nil)).
			WithDetail(fmt.Sprintf("URI: %s, Recursive: %t", uri, recursive))
	}

	var subscribeResult protocol.SubscribeResourceResult
	if err := c.validateAndParseResult(result, &subscribeResult, protocol.MethodSubscribeResource); err != nil {
		return err
	}

	if !subscribeResult.Success {
		return mcperrors.OperationFailed("subscribe_resource", "Server declined subscription request").
			WithContext(c.createRequestContext(protocol.MethodSubscribeResource, nil)).
			WithDetail(fmt.Sprintf("URI: %s, Recursive: %t", uri, recursive))
	}

	return nil
}

// ListPrompts retrieves the list of prompts from the server
func (c *ClientConfig) ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, *protocol.PaginationResult, error) {
	if err := c.requireCapability(protocol.CapabilityPrompts, "ListPrompts"); err != nil {
		return nil, nil, err
	}

	// Validate pagination parameters
	if err := c.validatePagination(pagination); err != nil {
		return nil, nil, err
	}

	// Apply defaults and create request params
	paginationParams := c.applyPaginationDefaults(pagination)
	params := &protocol.ListPromptsParams{
		Tag:              tag,
		PaginationParams: *paginationParams,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodListPrompts, params)
	if err != nil {
		return nil, nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodListPrompts, nil)).
			WithDetail(fmt.Sprintf("Tag: %s", tag))
	}

	var promptsResult protocol.ListPromptsResult
	if err := c.validateAndParseResult(result, &promptsResult, protocol.MethodListPrompts); err != nil {
		return nil, nil, err
	}

	paginationResult := &protocol.PaginationResult{
		TotalCount: promptsResult.TotalCount,
		NextCursor: promptsResult.NextCursor,
		HasMore:    promptsResult.HasMore,
	}

	return promptsResult.Prompts, paginationResult, nil
}

// GetPrompt retrieves a prompt from the server
func (c *ClientConfig) GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error) {
	if err := c.requireCapability(protocol.CapabilityPrompts, "GetPrompt"); err != nil {
		return nil, err
	}

	params := &protocol.GetPromptParams{
		ID: id,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodGetPrompt, params)
	if err != nil {
		return nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodGetPrompt, nil)).
			WithDetail(fmt.Sprintf("ID: %s", id))
	}

	var promptResult protocol.GetPromptResult
	if err := c.validateAndParseResult(result, &promptResult, protocol.MethodGetPrompt); err != nil {
		return nil, err
	}

	return &promptResult.Prompt, nil
}

// Complete requests a completion from the server
func (c *ClientConfig) Complete(ctx context.Context, params *protocol.CompleteParams) (*protocol.CompleteResult, error) {
	if err := c.requireCapability(protocol.CapabilityComplete, "Complete"); err != nil {
		return nil, err
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodComplete, params)
	if err != nil {
		return nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodComplete, nil))
	}

	var completeResult protocol.CompleteResult
	if err := c.validateAndParseResult(result, &completeResult, protocol.MethodComplete); err != nil {
		return nil, err
	}

	return &completeResult, nil
}

// ListRoots retrieves the list of roots from the server
func (c *ClientConfig) ListRoots(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Root, *protocol.PaginationResult, error) {
	if err := c.requireCapability(protocol.CapabilityRoots, "ListRoots"); err != nil {
		return nil, nil, err
	}

	// Validate pagination parameters
	if err := c.validatePagination(pagination); err != nil {
		return nil, nil, err
	}

	// Apply defaults and create request params
	paginationParams := c.applyPaginationDefaults(pagination)
	params := &protocol.ListRootsParams{
		Tag:              tag,
		PaginationParams: *paginationParams,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodListRoots, params)
	if err != nil {
		return nil, nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodListRoots, nil)).
			WithDetail(fmt.Sprintf("Tag: %s", tag))
	}

	var rootsResult protocol.ListRootsResult
	if err := c.validateAndParseResult(result, &rootsResult, protocol.MethodListRoots); err != nil {
		return nil, nil, err
	}

	paginationResult := &protocol.PaginationResult{
		TotalCount: rootsResult.TotalCount,
		NextCursor: rootsResult.NextCursor,
		HasMore:    rootsResult.HasMore,
	}

	return rootsResult.Roots, paginationResult, nil
}

// SetLogLevel sets the log level on the server
func (c *ClientConfig) SetLogLevel(ctx context.Context, level protocol.LogLevel) error {
	if err := c.requireCapability(protocol.CapabilityLogging, "SetLogLevel"); err != nil {
		return err
	}

	params := &protocol.SetLogLevelParams{
		Level: level,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodSetLogLevel, params)
	if err != nil {
		return mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodSetLogLevel, nil)).
			WithDetail(fmt.Sprintf("Level: %s", level))
	}

	var logResult protocol.SetLogLevelResult
	if err := c.validateAndParseResult(result, &logResult, protocol.MethodSetLogLevel); err != nil {
		return err
	}

	if !logResult.Success {
		return mcperrors.OperationFailed("set_log_level", "Server declined log level change").
			WithContext(c.createRequestContext(protocol.MethodSetLogLevel, nil)).
			WithDetail(fmt.Sprintf("Level: %s", level))
	}

	return nil
}

// Cancel cancels a pending request
func (c *ClientConfig) Cancel(ctx context.Context, id interface{}) (bool, error) {
	params := &protocol.CancelParams{
		ID: id,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodCancel, params)
	if err != nil {
		return false, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodCancel, nil)).
			WithDetail(fmt.Sprintf("Request ID: %v", id))
	}

	var cancelResult protocol.CancelResult
	if err := c.validateAndParseResult(result, &cancelResult, protocol.MethodCancel); err != nil {
		return false, err
	}

	return cancelResult.Cancelled, nil
}

// Ping sends a ping to the server
func (c *ClientConfig) Ping(ctx context.Context) (*protocol.PingResult, error) {
	params := &protocol.PingParams{
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodPing, params)
	if err != nil {
		return nil, mcperrors.TransportError("client", "send_request", err).
			WithContext(c.createRequestContext(protocol.MethodPing, nil))
	}

	var pingResult protocol.PingResult
	if err := c.validateAndParseResult(result, &pingResult, protocol.MethodPing); err != nil {
		return nil, err
	}

	return &pingResult, nil
}

// SendProgress sends a progress notification to the client
func (c *ClientConfig) SendProgress(ctx context.Context, id interface{}, message string, percent float64, completed bool) error {
	params := &protocol.ProgressParams{
		ID:        id,
		Message:   message,
		Percent:   percent,
		Completed: completed,
	}

	return c.transport.SendNotification(ctx, protocol.MethodProgress, params)
}

// Sample sends a sample request to the client
func (c *ClientConfig) Sample(ctx context.Context, params *protocol.SampleParams, handler func(*protocol.SampleResult) error) error {
	// Implementation details...
	return nil
}

// SetSamplingCallback sets a callback for sampling events
func (c *ClientConfig) SetSamplingCallback(callback SamplingCallback) {
	c.callbackMutex.Lock()
	defer c.callbackMutex.Unlock()
	c.samplingCallback = callback
	c.logger.Debug("Sampling callback registered", logging.Bool("enabled", callback != nil))
}

// SetResourceChangedCallback sets a callback for resource changes
func (c *ClientConfig) SetResourceChangedCallback(callback ResourceChangedCallback) {
	c.callbackMutex.Lock()
	defer c.callbackMutex.Unlock()
	c.resourceChangedCallback = callback
	c.logger.Debug("Resource changed callback registered", logging.Bool("enabled", callback != nil))
}

// Request handlers

func (c *ClientConfig) handleSample(ctx context.Context, params interface{}) (interface{}, error) {
	var sampleParams protocol.SampleParams
	if err := c.validateAndParseParams(params, &sampleParams, protocol.MethodSample); err != nil {
		return nil, err
	}

	// Check if sampling capability is enabled and callback is registered
	c.callbackMutex.RLock()
	callback := c.samplingCallback
	c.callbackMutex.RUnlock()

	if callback == nil {
		return nil, mcperrors.OperationNotSupported("sampling", "Client - no callback registered").
			WithContext(c.createRequestContext(protocol.MethodSample, sampleParams.RequestID))
	}

	// Create a sampling event from the sample params
	samplingEvent := protocol.SamplingEvent{
		Type:    "sample_request",
		Data:    sampleParams,
		TokenID: sampleParams.RequestID,
	}

	// Invoke the callback in a safe manner
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error("Sampling callback panicked", logging.Any("panic", r))
			}
		}()
		callback(samplingEvent)
	}()

	// Return a basic sample result - in a real implementation,
	// the callback would likely provide the actual content
	return &protocol.SampleResult{
		Content: "Sampling request processed by client callback",
		Model:   "client-handler",
		Usage: &protocol.TokenUsage{
			PromptTokens:     len(sampleParams.Messages),
			CompletionTokens: 1,
			TotalTokens:      len(sampleParams.Messages) + 1,
		},
	}, nil
}

func (c *ClientConfig) handleCancel(ctx context.Context, params interface{}) (interface{}, error) {
	var cancelParams protocol.CancelParams
	if err := c.validateAndParseParams(params, &cancelParams, protocol.MethodCancel); err != nil {
		return nil, err
	}

	// Cancel operation (implementation-specific)
	return &protocol.CancelResult{Cancelled: false}, nil
}

func (c *ClientConfig) handlePing(ctx context.Context, params interface{}) (interface{}, error) {
	var pingParams protocol.PingParams
	if err := c.validateAndParseParams(params, &pingParams, protocol.MethodPing); err != nil {
		return nil, err
	}

	timestamp := pingParams.Timestamp
	if timestamp == 0 {
		timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	}

	return &protocol.PingResult{Timestamp: timestamp}, nil
}

// Notification handlers

func (c *ClientConfig) handleToolsChanged(ctx context.Context, params interface{}) error {
	var p protocol.ToolsChangedParams
	if err := c.validateAndParseParams(params, &p, protocol.MethodToolsChanged); err != nil {
		return err
	}

	// Handle tools changed notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handleResourcesChanged(ctx context.Context, params interface{}) error {
	var p protocol.ResourcesChangedParams
	if err := c.validateAndParseParams(params, &p, protocol.MethodResourcesChanged); err != nil {
		return err
	}

	// Get the callback in a thread-safe manner
	c.callbackMutex.RLock()
	callback := c.resourceChangedCallback
	c.callbackMutex.RUnlock()

	if callback != nil {
		// Invoke the callback in a safe manner
		func() {
			defer func() {
				if r := recover(); r != nil {
					c.logger.Error("Resource changed callback panicked",
						logging.Any("panic", r),
						logging.String("uri", p.URI))
				}
			}()
			// Call the callback with the URI and the full change parameters
			callback(p.URI, p)
		}()
		c.logger.Debug("Resource changed notification processed",
			logging.String("uri", p.URI),
			logging.Int("added", len(p.Added)),
			logging.Int("modified", len(p.Modified)),
			logging.Int("removed", len(p.Removed)))
	} else {
		c.logger.Debug("Resource changed notification received but no callback registered",
			logging.String("uri", p.URI))
	}

	return nil
}

func (c *ClientConfig) handleResourceUpdated(ctx context.Context, params interface{}) error {
	var p protocol.ResourceUpdatedParams
	if err := c.validateAndParseParams(params, &p, protocol.MethodResourceUpdated); err != nil {
		return err
	}

	// Get the callback in a thread-safe manner
	c.callbackMutex.RLock()
	callback := c.resourceChangedCallback
	c.callbackMutex.RUnlock()

	if callback != nil {
		// Invoke the callback in a safe manner
		func() {
			defer func() {
				if r := recover(); r != nil {
					c.logger.Error("Resource updated callback panicked",
						logging.Any("panic", r),
						logging.String("uri", p.URI))
				}
			}()
			// Call the callback with the URI and the update parameters
			callback(p.URI, p)
		}()
		c.logger.Debug("Resource updated notification processed",
			logging.String("uri", p.URI),
			logging.Bool("deleted", p.Deleted))
	} else {
		c.logger.Debug("Resource updated notification received but no callback registered",
			logging.String("uri", p.URI))
	}

	return nil
}

func (c *ClientConfig) handlePromptsChanged(ctx context.Context, params interface{}) error {
	var p protocol.PromptsChangedParams
	if err := c.validateAndParseParams(params, &p, protocol.MethodPromptsChanged); err != nil {
		return err
	}

	// Handle prompts changed notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handleRootsChanged(ctx context.Context, params interface{}) error {
	var p protocol.RootsChangedParams
	if err := c.validateAndParseParams(params, &p, protocol.MethodRootsChanged); err != nil {
		return err
	}

	// Handle roots changed notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handleLog(ctx context.Context, params interface{}) error {
	var p protocol.LogParams
	if err := c.validateAndParseParams(params, &p, protocol.MethodLog); err != nil {
		return err
	}

	// Handle log notification (implementation-specific)
	return nil
}

// Helper functions for error handling

// createRequestContext creates error context for request handling
func (c *ClientConfig) createRequestContext(method string, requestID interface{}) *mcperrors.Context {
	return &mcperrors.Context{
		RequestID: fmt.Sprintf("%v", requestID),
		Method:    method,
		Component: "Client",
		Operation: method,
		Timestamp: time.Now(),
	}
}

// requireCapability checks if a capability is supported and returns structured error
func (c *ClientConfig) requireCapability(capability protocol.CapabilityType, operation string) error {
	if !c.HasCapability(capability) {
		return mcperrors.CapabilityRequired(string(capability)).
			WithContext(&mcperrors.Context{
				Component: "Client",
				Operation: operation,
				Timestamp: time.Now(),
			})
	}
	return nil
}

// validateAndParseResult validates and parses response results with structured errors
func (c *ClientConfig) validateAndParseResult(result interface{}, target interface{}, method string) error {
	if result == nil {
		return mcperrors.CreateInternalError("nil_result", nil).
			WithContext(c.createRequestContext(method, nil)).
			WithDetail("Received nil result from server")
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcperrors.CreateInternalError("marshal_result", err).
			WithContext(c.createRequestContext(method, nil)).
			WithDetail("Failed to process server response")
	}

	if err := json.Unmarshal(data, target); err != nil {
		return mcperrors.InvalidParameter("result", result, fmt.Sprintf("%T", target)).
			WithContext(c.createRequestContext(method, nil)).
			WithDetail(err.Error())
	}

	return nil
}

// validateAndParseParams validates and parses request parameters with structured errors
func (c *ClientConfig) validateAndParseParams(params interface{}, target interface{}, method string) error {
	if params == nil {
		return mcperrors.MissingParameter("params").WithContext(
			c.createRequestContext(method, nil),
		)
	}

	data, err := json.Marshal(params)
	if err != nil {
		return mcperrors.CreateInternalError("marshal_params", err).
			WithContext(c.createRequestContext(method, nil)).
			WithDetail("Failed to process request parameters")
	}

	if err := json.Unmarshal(data, target); err != nil {
		return mcperrors.InvalidParameter("params", params, fmt.Sprintf("%T", target)).
			WithContext(c.createRequestContext(method, nil)).
			WithDetail(err.Error())
	}

	return nil
}

// Utility functions

func parseResult(result interface{}, target interface{}) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return nil
}

func parseParams(params interface{}, target interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal params: %w", err)
	}

	return nil
}

// validatePagination validates pagination parameters using the pagination utility
func (c *ClientConfig) validatePagination(params *protocol.PaginationParams) error {
	return pagination.ValidateParams(params)
}

// applyPaginationDefaults applies default values to pagination parameters
func (c *ClientConfig) applyPaginationDefaults(params *protocol.PaginationParams) *protocol.PaginationParams {
	return pagination.ApplyDefaults(params)
}
