// Package pagination provides utilities for handling paginated requests and responses
// in the Model Context Protocol.
package pagination

import (
	"errors"
	"fmt"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

const (
	// DefaultLimit is the recommended default page size for paginated results
	DefaultLimit = 50

	// MaxLimit is the maximum allowed page size for paginated results
	MaxLimit = 200
)

var (
	// ErrInvalidLimit is returned when the pagination limit is invalid
	ErrInvalidLimit = errors.New("pagination limit must be greater than 0 and less than or equal to MaxLimit")

	// ErrInvalidCursor is returned when a pagination cursor is invalid
	ErrInvalidCursor = errors.New("invalid pagination cursor format")
)

// ValidateParams validates pagination parameters
func ValidateParams(params *protocol.PaginationParams) error {
	if params == nil {
		return nil // nil params are valid (will use server defaults)
	}

	// Validate limit if specified
	if params.Limit < 0 {
		return fmt.Errorf("%w: got %d", ErrInvalidLimit, params.Limit)
	}

	if params.Limit > MaxLimit {
		return fmt.Errorf("%w: got %d, max is %d", ErrInvalidLimit, params.Limit, MaxLimit)
	}

	// Note: We can't fully validate the cursor format here as it's opaque and
	// implementation-specific. The server will validate it fully.

	return nil
}

// ApplyDefaults ensures pagination parameters have sensible defaults
func ApplyDefaults(params *protocol.PaginationParams) *protocol.PaginationParams {
	if params == nil {
		// Create default params
		return &protocol.PaginationParams{
			Limit: DefaultLimit,
		}
	}

	// Create a copy to avoid modifying the original
	result := &protocol.PaginationParams{
		Cursor: params.Cursor,
		Limit:  params.Limit,
	}

	// Apply default limit if not specified
	if result.Limit <= 0 {
		result.Limit = DefaultLimit
	}

	// If limit exceeds maximum, cap it
	if result.Limit > MaxLimit {
		result.Limit = MaxLimit
	}

	return result
}

// HasNextPage checks if there are more pages to fetch
func HasNextPage(result *protocol.PaginationResult) bool {
	if result == nil {
		return false
	}
	return result.HasMore && result.NextCursor != ""
}

// Collector is a utility for collecting all pages of paginated results
type Collector struct {
	// NextCursor holds the pagination cursor for the next page
	NextCursor string
	// HasMore indicates if there are more pages to fetch
	HasMore bool
	// TotalItems is the total number of items collected so far
	TotalItems int
}

// NewCollector creates a new pagination collector
func NewCollector() *Collector {
	return &Collector{
		NextCursor: "",
		HasMore:    true,
		TotalItems: 0,
	}
}

// Update updates the collector with pagination results
func (c *Collector) Update(result *protocol.PaginationResult) {
	if result == nil {
		c.HasMore = false
		return
	}

	c.NextCursor = result.NextCursor
	c.HasMore = result.HasMore && result.NextCursor != ""
	if len(result.Items) > 0 {
		c.TotalItems += len(result.Items)
	}
}

// NextParams returns pagination parameters for the next page
func (c *Collector) NextParams(baseParams *protocol.PaginationParams) *protocol.PaginationParams {
	params := ApplyDefaults(baseParams)
	params.Cursor = c.NextCursor
	return params
}
