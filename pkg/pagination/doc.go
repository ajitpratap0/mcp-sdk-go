// Package pagination provides utilities for handling paginated results in MCP.
//
// This package includes functions and types to help with implementing pagination
// in MCP servers and clients. It provides:
//
//   - Parameter validation: Ensure pagination parameters are valid
//   - Default application: Apply sensible defaults to pagination parameters
//   - Cursor handling: Encode and decode pagination cursors
//
// # Using Pagination in a Server
//
// To implement pagination in an MCP server:
//
//	import (
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/pagination"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
//	)
//
//	func (s *Server) ListTools(ctx context.Context, category string, paginationParams *protocol.PaginationParams) ([]protocol.Tool, *protocol.PaginationResult, error) {
//	    // Validate pagination parameters
//	    if err := pagination.ValidateParams(paginationParams); err != nil {
//	        return nil, nil, err
//	    }
//
//	    // Apply default values
//	    params := pagination.ApplyDefaults(paginationParams)
//
//	    // Get all tools matching the category
//	    allTools := s.getToolsByCategory(category)
//
//	    // Apply pagination
//	    startIndex := params.Offset
//	    endIndex := startIndex + params.Limit
//	    if endIndex > len(allTools) {
//	        endIndex = len(allTools)
//	    }
//
//	    // Create result with pagination info
//	    paginationResult := &protocol.PaginationResult{
//	        TotalCount: len(allTools),
//	        HasMore:    endIndex < len(allTools),
//	    }
//
//	    // If there are more results, create a cursor for the next page
//	    if paginationResult.HasMore {
//	        paginationResult.NextCursor = pagination.EncodeCursor(endIndex)
//	    }
//
//	    return allTools[startIndex:endIndex], paginationResult, nil
//	}
//
// # Using Pagination in a Client
//
// To handle pagination in an MCP client:
//
//	import (
//	    "context"
//	    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
//	)
//
//	// Function to get all tools by automatically paginating
//	func (c *Client) ListAllTools(ctx context.Context, category string) ([]protocol.Tool, error) {
//	    var allTools []protocol.Tool
//	    var cursor string
//
//	    for {
//	        // Create pagination params with cursor
//	        params := &protocol.PaginationParams{
//	            Limit: 50,
//	            After: cursor,
//	        }
//
//	        // Get a page of tools
//	        tools, pagination, err := c.ListTools(ctx, category, params)
//	        if err != nil {
//	            return nil, err
//	        }
//
//	        // Add tools to the result
//	        allTools = append(allTools, tools...)
//
//	        // If there are no more results, stop paginating
//	        if !pagination.HasMore {
//	            break
//	        }
//
//	        // Use the next cursor for the next page
//	        cursor = pagination.NextCursor
//	    }
//
//	    return allTools, nil
//	}
package pagination
