package client

import (
	"context"
	"fmt"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/pagination"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// PaginatedFetchFunc is a function that fetches a single page of results
type PaginatedFetchFunc func(ctx context.Context, params *protocol.PaginationParams) (*protocol.PaginationResult, interface{}, error)

// FetchAllPages collects all pages of results from a paginated operation
// This is a generic utility that can be used with any paginated operation.
func FetchAllPages(ctx context.Context, initialParams *protocol.PaginationParams, fetchFunc PaginatedFetchFunc) ([]interface{}, error) {
	if err := pagination.ValidateParams(initialParams); err != nil {
		return nil, fmt.Errorf("invalid pagination parameters: %w", err)
	}

	// Apply defaults
	params := pagination.ApplyDefaults(initialParams)

	// Create collector
	collector := pagination.NewCollector()
	var allResults []interface{}

	// Fetch pages until done
	for collector.HasMore {
		result, items, err := fetchFunc(ctx, params)
		if err != nil {
			return nil, err
		}

		// Update collector state
		collector.Update(result)

		// Extract items based on the type
		switch typedItems := items.(type) {
		case []interface{}:
			allResults = append(allResults, typedItems...)
		case []protocol.Tool:
			for _, item := range typedItems {
				allResults = append(allResults, item)
			}
		case []protocol.Resource:
			for _, item := range typedItems {
				allResults = append(allResults, item)
			}
		case []protocol.Prompt:
			for _, item := range typedItems {
				allResults = append(allResults, item)
			}
		case []protocol.Root:
			for _, item := range typedItems {
				allResults = append(allResults, item)
			}
		default:
			return nil, fmt.Errorf("unsupported items type: %T", items)
		}

		// Check if we should continue
		if !collector.HasMore {
			break
		}

		// Update parameters for next page
		params = collector.NextParams(params)
	}

	return allResults, nil
}

// ListAllTools retrieves all tools from the server, handling pagination automatically
func (c *Client) ListAllTools(ctx context.Context, category string) ([]protocol.Tool, error) {
	if !c.HasCapability(protocol.CapabilityTools) {
		return nil, fmt.Errorf("server does not support tools")
	}

	// Create fetch function that adapts to the PaginatedFetchFunc signature
	fetchFunc := func(ctx context.Context, params *protocol.PaginationParams) (*protocol.PaginationResult, interface{}, error) {
		tools, pagResult, err := c.ListTools(ctx, category, params)
		return pagResult, tools, err
	}

	results, err := FetchAllPages(ctx, nil, fetchFunc)
	if err != nil {
		return nil, err
	}

	// Convert results to the correct type
	tools := make([]protocol.Tool, 0, len(results))
	for _, result := range results {
		if tool, ok := result.(protocol.Tool); ok {
			tools = append(tools, tool)
		}
	}

	return tools, nil
}

// ListAllResources retrieves all resources from the server, handling pagination automatically
func (c *Client) ListAllResources(ctx context.Context, uri string, recursive bool) ([]protocol.Resource, []protocol.ResourceTemplate, error) {
	if !c.HasCapability(protocol.CapabilityResources) {
		return nil, nil, fmt.Errorf("server does not support resources")
	}

	var allResources []protocol.Resource
	var allTemplates []protocol.ResourceTemplate

	// Create fetch function
	fetchFunc := func(ctx context.Context, params *protocol.PaginationParams) (*protocol.PaginationResult, interface{}, error) {
		resources, templates, pagResult, err := c.ListResources(ctx, uri, recursive, params)

		// Accumulate templates (which aren't part of the normal pagination)
		if err == nil && len(templates) > 0 {
			allTemplates = append(allTemplates, templates...)
		}

		return pagResult, resources, err
	}

	results, err := FetchAllPages(ctx, nil, fetchFunc)
	if err != nil {
		return nil, nil, err
	}

	// Convert results to the correct type
	allResources = make([]protocol.Resource, 0, len(results))
	for _, result := range results {
		if resource, ok := result.(protocol.Resource); ok {
			allResources = append(allResources, resource)
		}
	}

	return allResources, allTemplates, nil
}

// ListAllPrompts retrieves all prompts from the server, handling pagination automatically
func (c *Client) ListAllPrompts(ctx context.Context, tag string) ([]protocol.Prompt, error) {
	if !c.HasCapability(protocol.CapabilityPrompts) {
		return nil, fmt.Errorf("server does not support prompts")
	}

	// Create fetch function
	fetchFunc := func(ctx context.Context, params *protocol.PaginationParams) (*protocol.PaginationResult, interface{}, error) {
		prompts, pagResult, err := c.ListPrompts(ctx, tag, params)
		return pagResult, prompts, err
	}

	results, err := FetchAllPages(ctx, nil, fetchFunc)
	if err != nil {
		return nil, err
	}

	// Convert results to the correct type
	prompts := make([]protocol.Prompt, 0, len(results))
	for _, result := range results {
		if prompt, ok := result.(protocol.Prompt); ok {
			prompts = append(prompts, prompt)
		}
	}

	return prompts, nil
}

// ListAllRoots retrieves all roots from the server, handling pagination automatically
func (c *Client) ListAllRoots(ctx context.Context, tag string) ([]protocol.Root, error) {
	if !c.HasCapability(protocol.CapabilityRoots) {
		return nil, fmt.Errorf("server does not support roots")
	}

	// Create fetch function
	fetchFunc := func(ctx context.Context, params *protocol.PaginationParams) (*protocol.PaginationResult, interface{}, error) {
		roots, pagResult, err := c.ListRoots(ctx, tag, params)
		return pagResult, roots, err
	}

	results, err := FetchAllPages(ctx, nil, fetchFunc)
	if err != nil {
		return nil, err
	}

	// Convert results to the correct type
	roots := make([]protocol.Root, 0, len(results))
	for _, result := range results {
		if root, ok := result.(protocol.Root); ok {
			roots = append(roots, root)
		}
	}

	return roots, nil
}
