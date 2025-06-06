package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// ToolsProvider defines the interface for providing tools functionality
type ToolsProvider interface {
	// ListTools returns a list of available tools
	ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error)

	// CallTool executes a tool and returns the result
	CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error)
}

// ResourcesProvider defines the interface for providing resources functionality
type ResourcesProvider interface {
	// ListResources returns a list of available resources
	ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, int, string, bool, error)

	// ReadResource reads a resource and returns its contents
	ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error)

	// SubscribeResource subscribes to resource changes
	SubscribeResource(ctx context.Context, uri string, recursive bool) (bool, error)
}

// PromptsProvider defines the interface for providing prompts functionality
type PromptsProvider interface {
	// ListPrompts returns a list of available prompts
	ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, int, string, bool, error)

	// GetPrompt returns a specific prompt
	GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error)
}

// CompletionProvider defines the interface for providing completion functionality
type CompletionProvider interface {
	// Complete generates a completion based on the provided parameters
	Complete(ctx context.Context, params *protocol.CompleteParams) (*protocol.CompleteResult, error)
}

// RootsProvider defines the interface for providing roots functionality
type RootsProvider interface {
	// ListRoots returns a list of available roots
	ListRoots(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Root, int, string, bool, error)
}

// PaginationCursor represents the state of pagination
type PaginationCursor struct {
	Offset int `json:"offset"`
}

// encodeCursor creates a base64-encoded cursor from an offset
func encodeCursor(offset int) string {
	cursor := PaginationCursor{Offset: offset}
	data, _ := json.Marshal(cursor)
	return base64.StdEncoding.EncodeToString(data)
}

// decodeCursor extracts the offset from a base64-encoded cursor
func decodeCursor(cursorStr string) (int, error) {
	if cursorStr == "" {
		return 0, nil
	}

	data, err := base64.StdEncoding.DecodeString(cursorStr)
	if err != nil {
		return 0, mcperrors.InvalidPagination("Invalid cursor format").
			WithDetail(fmt.Sprintf("Cursor: %s, Error: %v", cursorStr, err))
	}

	var cursor PaginationCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return 0, mcperrors.InvalidPagination("Invalid cursor data").
			WithDetail(fmt.Sprintf("Cursor: %s, Error: %v", cursorStr, err))
	}

	if cursor.Offset < 0 {
		return 0, mcperrors.InvalidPagination("Cursor offset cannot be negative").
			WithDetail(fmt.Sprintf("Offset: %d", cursor.Offset))
	}

	return cursor.Offset, nil
}

// BaseToolsProvider provides a simple implementation of ToolsProvider
type BaseToolsProvider struct {
	mu    sync.RWMutex
	tools map[string]protocol.Tool
}

// NewBaseToolsProvider creates a new BaseToolsProvider
func NewBaseToolsProvider() *BaseToolsProvider {
	return &BaseToolsProvider{
		tools: make(map[string]protocol.Tool),
	}
}

// RegisterTool registers a tool
func (p *BaseToolsProvider) RegisterTool(tool protocol.Tool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tools[tool.Name] = tool
}

// ListTools returns all registered tools
func (p *BaseToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var tools []protocol.Tool

	for _, tool := range p.tools {
		// Filter by category if specified
		if category != "" {
			found := false
			for _, cat := range tool.Categories {
				if cat == category {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		tools = append(tools, tool)
	}

	// Implement proper pagination with cursor support
	total := len(tools)
	limit := 50 // Default limit
	cursor := ""

	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		cursor = pagination.Cursor
	}

	// Decode the cursor to get the starting offset
	start, err := decodeCursor(cursor)
	if err != nil {
		return nil, 0, "", false, mcperrors.InvalidPagination("Invalid pagination cursor for tools list").
			WithDetail(fmt.Sprintf("Cursor: %s, Error: %v", cursor, err))
	}

	// Calculate the end position
	end := start + limit
	if end > total {
		end = total
	}

	// Determine if there are more results
	hasMore := end < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeCursor(end)
	}

	// Return the requested slice
	if start < total {
		return tools[start:end], total, nextCursor, hasMore, nil
	}

	// If start is beyond the total, return empty slice
	return []protocol.Tool{}, total, "", false, nil
}

// CallTool executes a tool and returns the result
func (p *BaseToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
	// In a real implementation, this would execute the tool
	// For now, just return a simple result
	return &protocol.CallToolResult{
		Result: json.RawMessage(`{"message": "Tool execution not implemented"}`),
	}, nil
}

// BaseResourcesProvider provides a simple implementation of ResourcesProvider
type BaseResourcesProvider struct {
	mu          sync.RWMutex
	resources   map[string]protocol.Resource
	templates   map[string]protocol.ResourceTemplate
	subscribers map[string]bool
}

// NewBaseResourcesProvider creates a new BaseResourcesProvider
func NewBaseResourcesProvider() *BaseResourcesProvider {
	return &BaseResourcesProvider{
		resources:   make(map[string]protocol.Resource),
		templates:   make(map[string]protocol.ResourceTemplate),
		subscribers: make(map[string]bool),
	}
}

// RegisterResource registers a resource
func (p *BaseResourcesProvider) RegisterResource(resource protocol.Resource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resources[resource.URI] = resource
}

// RegisterTemplate registers a resource template
func (p *BaseResourcesProvider) RegisterTemplate(template protocol.ResourceTemplate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templates[template.URI] = template
}

// ListResources returns all registered resources
func (p *BaseResourcesProvider) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, int, string, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var resources []protocol.Resource
	var templates []protocol.ResourceTemplate

	// Filter resources by URI prefix if specified
	if uri != "" {
		for _, resource := range p.resources {
			if resource.URI == uri || (recursive && hasPrefix(resource.URI, uri+"/")) {
				resources = append(resources, resource)
			}
		}

		for _, template := range p.templates {
			if template.URI == uri || (recursive && hasPrefix(template.URI, uri+"/")) {
				templates = append(templates, template)
			}
		}
	} else {
		for _, resource := range p.resources {
			resources = append(resources, resource)
		}

		for _, template := range p.templates {
			templates = append(templates, template)
		}
	}

	// Implement proper pagination with cursor support
	totalResources := len(resources)
	totalTemplates := len(templates)
	total := totalResources + totalTemplates

	limit := 50 // Default limit
	cursor := ""

	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		cursor = pagination.Cursor
	}

	// Decode the cursor to get the starting offset
	start, err := decodeCursor(cursor)
	if err != nil {
		return nil, nil, 0, "", false, mcperrors.InvalidPagination("Invalid pagination cursor for resources list").
			WithDetail(fmt.Sprintf("Cursor: %s, Error: %v", cursor, err))
	}

	// Apply pagination to resources first, then templates
	resStart := start
	resEnd := min(totalResources, start+limit)

	templStart := max(0, start-totalResources)
	templEnd := min(totalTemplates, templStart+limit-(resEnd-resStart))

	// Calculate the actual end position across both resources and templates
	actualEnd := resEnd + templEnd
	if resEnd == totalResources && templEnd > 0 {
		actualEnd = totalResources + templEnd
	}

	hasMore := actualEnd < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeCursor(actualEnd)
	}

	// Slice the results
	if resStart < totalResources {
		resources = resources[resStart:resEnd]
	} else {
		resources = []protocol.Resource{}
	}

	if templStart < totalTemplates {
		templates = templates[templStart:templEnd]
	} else {
		templates = []protocol.ResourceTemplate{}
	}

	return resources, templates, total, nextCursor, hasMore, nil
}

// ReadResource reads a resource and returns its contents
func (p *BaseResourcesProvider) ReadResource(ctx context.Context, uri string, templateParams map[string]interface{}, rangeOpt *protocol.ResourceRange) (*protocol.ResourceContents, error) {
	// In a real implementation, this would read the resource content
	// For now, just return a simple result
	return &protocol.ResourceContents{
		URI:     uri,
		Type:    "text/plain",
		Content: json.RawMessage(`"Resource content not implemented"`),
	}, nil
}

// SubscribeResource subscribes to resource changes
func (p *BaseResourcesProvider) SubscribeResource(ctx context.Context, uri string, recursive bool) (bool, error) {
	// Register subscription
	p.subscribers[uri] = recursive
	return true, nil
}

// BasePromptsProvider provides a simple implementation of PromptsProvider
type BasePromptsProvider struct {
	mu      sync.RWMutex
	prompts map[string]protocol.Prompt
}

// NewBasePromptsProvider creates a new BasePromptsProvider
func NewBasePromptsProvider() *BasePromptsProvider {
	return &BasePromptsProvider{
		prompts: make(map[string]protocol.Prompt),
	}
}

// RegisterPrompt registers a prompt
func (p *BasePromptsProvider) RegisterPrompt(prompt protocol.Prompt) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prompts[prompt.ID] = prompt
}

// ListPrompts returns all registered prompts
func (p *BasePromptsProvider) ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, int, string, bool, error) {
	p.mu.RLock()
	var prompts []protocol.Prompt

	// Filter prompts by tag if specified
	if tag != "" {
		for _, prompt := range p.prompts {
			for _, t := range prompt.Tags {
				if t == tag {
					prompts = append(prompts, prompt)
					break
				}
			}
		}
	} else {
		for _, prompt := range p.prompts {
			prompts = append(prompts, prompt)
		}
	}
	p.mu.RUnlock()

	// Implement proper pagination with cursor support
	total := len(prompts)
	limit := 50 // Default limit
	cursor := ""

	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		cursor = pagination.Cursor
	}

	// Decode the cursor to get the starting offset
	start, err := decodeCursor(cursor)
	if err != nil {
		return nil, 0, "", false, mcperrors.InvalidPagination("Invalid pagination cursor for prompts list").
			WithDetail(fmt.Sprintf("Cursor: %s, Error: %v", cursor, err))
	}

	// Calculate the end position
	end := start + limit
	if end > total {
		end = total
	}

	// Determine if there are more results
	hasMore := end < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeCursor(end)
	}

	// Return the requested slice
	if start < total {
		return prompts[start:end], total, nextCursor, hasMore, nil
	}

	// If start is beyond the total, return empty slice
	return []protocol.Prompt{}, total, "", false, nil
}

// GetPrompt returns a specific prompt
func (p *BasePromptsProvider) GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	prompt, ok := p.prompts[id]
	if !ok {
		return nil, errNotFound("prompt", id)
	}

	return &prompt, nil
}

// BaseRootsProvider provides a simple implementation of RootsProvider
type BaseRootsProvider struct {
	mu    sync.RWMutex
	roots map[string]protocol.Root
}

// NewBaseRootsProvider creates a new BaseRootsProvider
func NewBaseRootsProvider() *BaseRootsProvider {
	return &BaseRootsProvider{
		roots: make(map[string]protocol.Root),
	}
}

// RegisterRoot registers a root
func (p *BaseRootsProvider) RegisterRoot(root protocol.Root) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.roots[root.ID] = root
}

// ListRoots returns all registered roots
func (p *BaseRootsProvider) ListRoots(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Root, int, string, bool, error) {
	p.mu.RLock()
	var roots []protocol.Root

	// Filter roots by tag if specified
	if tag != "" {
		for _, root := range p.roots {
			for _, t := range root.Tags {
				if t == tag {
					roots = append(roots, root)
					break
				}
			}
		}
	} else {
		for _, root := range p.roots {
			roots = append(roots, root)
		}
	}
	p.mu.RUnlock()

	// Implement proper pagination with cursor support
	total := len(roots)
	limit := 50 // Default limit
	cursor := ""

	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		cursor = pagination.Cursor
	}

	// Decode the cursor to get the starting offset
	start, err := decodeCursor(cursor)
	if err != nil {
		return nil, 0, "", false, mcperrors.InvalidPagination("Invalid pagination cursor for roots list").
			WithDetail(fmt.Sprintf("Cursor: %s, Error: %v", cursor, err))
	}

	// Calculate the end position
	end := start + limit
	if end > total {
		end = total
	}

	// Determine if there are more results
	hasMore := end < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeCursor(end)
	}

	// Return the requested slice
	if start < total {
		return roots[start:end], total, nextCursor, hasMore, nil
	}

	// If start is beyond the total, return empty slice
	return []protocol.Root{}, total, "", false, nil
}

// Helper functions

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[0:len(prefix)] == prefix
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// errNotFound creates an error for a resource not found
func errNotFound(resourceType, id string) error {
	return &notFoundError{resourceType: resourceType, id: id}
}

// notFoundError represents a not found error
type notFoundError struct {
	resourceType string
	id           string
}

func (e *notFoundError) Error() string {
	return e.resourceType + " not found: " + e.id
}
