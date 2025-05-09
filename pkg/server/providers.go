package server

import (
	"context"
	"encoding/json"

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

// BaseToolsProvider provides a simple implementation of ToolsProvider
type BaseToolsProvider struct {
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
	p.tools[tool.Name] = tool
}

// ListTools returns all registered tools
func (p *BaseToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
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

	// Simple pagination (in a real implementation, this would be more sophisticated)
	total := len(tools)
	limit := pagination.Limit
	if limit <= 0 {
		limit = 50 // Default limit
	}

	start := 0
	// In a real implementation, we would parse pagination.Cursor to get the start index
	// For now we always start at index 0

	end := start + limit
	if end > total {
		end = total
	}

	hasMore := end < total
	nextCursor := ""
	if hasMore {
		nextCursor = "cursor_" + string(rune(end)) // Simple string cursor
	}

	if start < total {
		return tools[start:end], total, nextCursor, hasMore, nil
	}

	return []protocol.Tool{}, total, nextCursor, hasMore, nil
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
	p.resources[resource.URI] = resource
}

// RegisterTemplate registers a resource template
func (p *BaseResourcesProvider) RegisterTemplate(template protocol.ResourceTemplate) {
	p.templates[template.URI] = template
}

// ListResources returns all registered resources
func (p *BaseResourcesProvider) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, int, string, bool, error) {
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

	// Simple pagination (in a real implementation, this would be more sophisticated)
	totalResources := len(resources)
	totalTemplates := len(templates)
	total := totalResources + totalTemplates

	limit := pagination.Limit
	if limit <= 0 {
		limit = 50 // Default limit
	}

	start := 0
	// In a real implementation, we would parse pagination.Cursor to get the start index
	// For now we always start at index 0

	// Apply pagination to resources first, then templates
	resStart := start
	resEnd := min(totalResources, start+limit)

	templStart := max(0, resEnd-start)
	templEnd := min(totalTemplates, templStart+limit-(resEnd-resStart))

	hasMore := resEnd < totalResources || templEnd < totalTemplates
	nextCursor := ""
	if hasMore {
		nextCursor = "cursor_" + string(rune(resEnd)) + "_" + string(rune(templEnd)) // Simple string cursor
	}

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
	p.prompts[prompt.ID] = prompt
}

// ListPrompts returns all registered prompts
func (p *BasePromptsProvider) ListPrompts(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Prompt, int, string, bool, error) {
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

	// Simple pagination (in a real implementation, this would be more sophisticated)
	total := len(prompts)
	limit := 50 // Default limit
	start := 0
	// In a real implementation, we would parse pagination.Cursor to get the start index
	// For now we always start at index 0

	end := start + limit
	if end > total {
		end = total
	}

	hasMore := end < total
	nextCursor := ""
	if hasMore {
		nextCursor = "cursor_" + string(rune(end)) // Simple string cursor
	}

	if start < total {
		return prompts[start:end], total, nextCursor, hasMore, nil
	}

	return []protocol.Prompt{}, total, nextCursor, hasMore, nil
}

// GetPrompt returns a specific prompt
func (p *BasePromptsProvider) GetPrompt(ctx context.Context, id string) (*protocol.Prompt, error) {
	prompt, ok := p.prompts[id]
	if !ok {
		return nil, errNotFound("prompt", id)
	}

	return &prompt, nil
}

// BaseRootsProvider provides a simple implementation of RootsProvider
type BaseRootsProvider struct {
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
	p.roots[root.ID] = root
}

// ListRoots returns all registered roots
func (p *BaseRootsProvider) ListRoots(ctx context.Context, tag string, pagination *protocol.PaginationParams) ([]protocol.Root, int, string, bool, error) {
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

	// Simple pagination (in a real implementation, this would be more sophisticated)
	total := len(roots)
	limit := pagination.Limit
	if limit <= 0 {
		limit = 50 // Default limit
	}

	start := 0
	// In a real implementation, we would parse pagination.Cursor to get the start index
	// For now we always start at index 0

	end := start + limit
	if end > total {
		end = total
	}

	hasMore := end < total
	nextCursor := ""
	if hasMore {
		nextCursor = "cursor_" + string(rune(end)) // Simple string cursor
	}

	if start < total {
		return roots[start:end], total, nextCursor, hasMore, nil
	}

	return []protocol.Root{}, total, nextCursor, hasMore, nil
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
