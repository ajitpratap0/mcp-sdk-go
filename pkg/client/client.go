// Package client provides the client-side implementation of the MCP protocol,
// allowing applications to connect to MCP servers and consume their capabilities.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

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
		return fmt.Errorf("transport initialization failed: %w", err)
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
	fmt.Printf("[DEBUG] Sending initialize request with capabilities: %v\n", c.capabilities)
	result, err := c.transport.SendRequest(ctx, protocol.MethodInitialize, params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// Parse the result
	var initResult protocol.InitializeResult
	if err := parseResult(result, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
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
			fmt.Printf("[DEBUG] Transport has session ID after initialize: %s\n", sidField.String())
		} else {
			fmt.Printf("[DEBUG] Transport does not have session ID after initialize\n")
		}
	}

	// Mark as initialized
	c.initializedLock.Lock()
	c.initialized = true
	c.initializedLock.Unlock()

	// Send initialized notification
	err = c.transport.SendNotification(ctx, protocol.MethodInitialized, nil)
	if err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
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
	if !c.HasCapability(protocol.CapabilityTools) {
		return nil, nil, errors.New("server does not support tools")
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
		return nil, nil, fmt.Errorf("list tools request failed: %w", err)
	}

	var toolsResult protocol.ListToolsResult
	if err := parseResult(result, &toolsResult); err != nil {
		return nil, nil, fmt.Errorf("failed to parse list tools result: %w", err)
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
	if !c.HasCapability(protocol.CapabilityTools) {
		return nil, errors.New("server does not support tools")
	}

	var inputJSON, contextJSON json.RawMessage
	var err error

	if input != nil {
		if inputJSON, err = json.Marshal(input); err != nil {
			return nil, fmt.Errorf("failed to marshal input: %w", err)
		}
	}

	if context != nil {
		if contextJSON, err = json.Marshal(context); err != nil {
			return nil, fmt.Errorf("failed to marshal context: %w", err)
		}
	}

	params := &protocol.CallToolParams{
		Name:    name,
		Input:   inputJSON,
		Context: contextJSON,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodCallTool, params)
	if err != nil {
		return nil, fmt.Errorf("call tool request failed: %w", err)
	}

	var toolResult protocol.CallToolResult
	if err := parseResult(result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse call tool result: %w", err)
	}

	return &toolResult, nil
}

// ListResources retrieves the list of resources from the server
func (c *ClientConfig) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, *protocol.PaginationResult, error) {
	if !c.HasCapability(protocol.CapabilityResources) {
		return nil, nil, nil, errors.New("server does not support resources")
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
		return nil, nil, nil, fmt.Errorf("list resources request failed: %w", err)
	}

	var resourcesResult protocol.ListResourcesResult
	if err := parseResult(result, &resourcesResult); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse list resources result: %w", err)
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
	if !c.HasCapability(protocol.CapabilityResources) {
		return nil, errors.New("server does not support resources")
	}

	params := &protocol.ReadResourceParams{
		URI:            uri,
		TemplateParams: templateParams,
		Range:          rangeOpt,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodReadResource, params)
	if err != nil {
		return nil, fmt.Errorf("read resource request failed: %w", err)
	}

	var readResult protocol.ReadResourceResult
	if err := parseResult(result, &readResult); err != nil {
		return nil, fmt.Errorf("failed to parse read resource result: %w", err)
	}

	return &readResult.Contents, nil
}

// SubscribeResource subscribes to resource changes
func (c *ClientConfig) SubscribeResource(ctx context.Context, uri string, recursive bool) error {
	if !c.HasCapability(protocol.CapabilityResourceSubscriptions) {
		return errors.New("server does not support resource subscriptions")
	}

	params := &protocol.SubscribeResourceParams{
		URI:       uri,
		Recursive: recursive,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodSubscribeResource, params)
	if err != nil {
		return fmt.Errorf("subscribe resource request failed: %w", err)
	}

	var subscribeResult protocol.SubscribeResourceResult
	if err := parseResult(result, &subscribeResult); err != nil {
		return fmt.Errorf("failed to parse subscribe resource result: %w", err)
	}

	if !subscribeResult.Success {
		return errors.New("server declined subscription request")
	}

	return nil
}

// ListPrompts retrieves the list of prompts from the server
func (c *ClientConfig) ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, *protocol.PaginationResult, error) {
	if !c.HasCapability(protocol.CapabilityPrompts) {
		return nil, nil, errors.New("server does not support prompts")
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
		return nil, nil, fmt.Errorf("list prompts request failed: %w", err)
	}

	var promptsResult protocol.ListPromptsResult
	if err := parseResult(result, &promptsResult); err != nil {
		return nil, nil, fmt.Errorf("failed to parse list prompts result: %w", err)
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
	if !c.HasCapability(protocol.CapabilityPrompts) {
		return nil, errors.New("server does not support prompts")
	}

	params := &protocol.GetPromptParams{
		ID: id,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodGetPrompt, params)
	if err != nil {
		return nil, fmt.Errorf("get prompt request failed: %w", err)
	}

	var promptResult protocol.GetPromptResult
	if err := parseResult(result, &promptResult); err != nil {
		return nil, fmt.Errorf("failed to parse get prompt result: %w", err)
	}

	return &promptResult.Prompt, nil
}

// Complete requests a completion from the server
func (c *ClientConfig) Complete(ctx context.Context, params *protocol.CompleteParams) (*protocol.CompleteResult, error) {
	if !c.HasCapability(protocol.CapabilityComplete) {
		return nil, errors.New("server does not support completions")
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodComplete, params)
	if err != nil {
		return nil, fmt.Errorf("complete request failed: %w", err)
	}

	var completeResult protocol.CompleteResult
	if err := parseResult(result, &completeResult); err != nil {
		return nil, fmt.Errorf("failed to parse complete result: %w", err)
	}

	return &completeResult, nil
}

// ListRoots retrieves the list of roots from the server
func (c *ClientConfig) ListRoots(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Root, *protocol.PaginationResult, error) {
	if !c.HasCapability(protocol.CapabilityRoots) {
		return nil, nil, errors.New("server does not support roots")
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
		return nil, nil, fmt.Errorf("list roots request failed: %w", err)
	}

	var rootsResult protocol.ListRootsResult
	if err := parseResult(result, &rootsResult); err != nil {
		return nil, nil, fmt.Errorf("failed to parse list roots result: %w", err)
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
	if !c.HasCapability(protocol.CapabilityLogging) {
		return errors.New("server does not support logging")
	}

	params := &protocol.SetLogLevelParams{
		Level: level,
	}

	result, err := c.transport.SendRequest(ctx, protocol.MethodSetLogLevel, params)
	if err != nil {
		return fmt.Errorf("set log level request failed: %w", err)
	}

	var logResult protocol.SetLogLevelResult
	if err := parseResult(result, &logResult); err != nil {
		return fmt.Errorf("failed to parse set log level result: %w", err)
	}

	if !logResult.Success {
		return errors.New("server declined log level change")
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
		return false, fmt.Errorf("cancel request failed: %w", err)
	}

	var cancelResult protocol.CancelResult
	if err := parseResult(result, &cancelResult); err != nil {
		return false, fmt.Errorf("failed to parse cancel result: %w", err)
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
		return nil, fmt.Errorf("ping request failed: %w", err)
	}

	var pingResult protocol.PingResult
	if err := parseResult(result, &pingResult); err != nil {
		return nil, fmt.Errorf("failed to parse ping result: %w", err)
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
	// Implementation will be added later
}

// SetResourceChangedCallback sets a callback for resource changes
func (c *ClientConfig) SetResourceChangedCallback(callback ResourceChangedCallback) {
	// Implementation will be added later
}

// Request handlers

func (c *ClientConfig) handleSample(ctx context.Context, params interface{}) (interface{}, error) {
	var sampleParams protocol.SampleParams
	if err := parseParams(params, &sampleParams); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Handle sample request (implementation-specific)
	return nil, errors.New("sampling not implemented in this client")
}

func (c *ClientConfig) handleCancel(ctx context.Context, params interface{}) (interface{}, error) {
	var cancelParams protocol.CancelParams
	if err := parseParams(params, &cancelParams); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Cancel operation (implementation-specific)
	return &protocol.CancelResult{Cancelled: false}, nil
}

func (c *ClientConfig) handlePing(ctx context.Context, params interface{}) (interface{}, error) {
	var pingParams protocol.PingParams
	if err := parseParams(params, &pingParams); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
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
	if err := parseParams(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Handle tools changed notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handleResourcesChanged(ctx context.Context, params interface{}) error {
	var p protocol.ResourcesChangedParams
	if err := parseParams(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Handle resources changed notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handleResourceUpdated(ctx context.Context, params interface{}) error {
	var p protocol.ResourceUpdatedParams
	if err := parseParams(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Handle resource updated notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handlePromptsChanged(ctx context.Context, params interface{}) error {
	var p protocol.PromptsChangedParams
	if err := parseParams(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Handle prompts changed notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handleRootsChanged(ctx context.Context, params interface{}) error {
	var p protocol.RootsChangedParams
	if err := parseParams(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Handle roots changed notification (implementation-specific)
	return nil
}

func (c *ClientConfig) handleLog(ctx context.Context, params interface{}) error {
	var p protocol.LogParams
	if err := parseParams(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Handle log notification (implementation-specific)
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
