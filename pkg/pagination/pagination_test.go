package pagination

import (
	"testing"

	"github.com/model-context-protocol/go-mcp/pkg/protocol"
)

func TestValidateParams(t *testing.T) {
	// Test nil params
	err := ValidateParams(nil)
	if err != nil {
		t.Errorf("Expected ValidateParams(nil) to succeed, got error: %v", err)
	}

	// Test valid params
	validParams := &protocol.PaginationParams{
		Limit:  50,
		Cursor: "valid-cursor",
	}
	err = ValidateParams(validParams)
	if err != nil {
		t.Errorf("Expected ValidateParams with valid params to succeed, got error: %v", err)
	}

	// Test limit = 0 (should be valid as it will use server defaults)
	zeroLimitParams := &protocol.PaginationParams{
		Limit:  0,
		Cursor: "valid-cursor",
	}
	err = ValidateParams(zeroLimitParams)
	if err != nil {
		t.Errorf("Expected ValidateParams with limit=0 to succeed, got error: %v", err)
	}

	// Test negative limit
	negativeLimitParams := &protocol.PaginationParams{
		Limit:  -10,
		Cursor: "valid-cursor",
	}
	err = ValidateParams(negativeLimitParams)
	if err == nil {
		t.Error("Expected ValidateParams with negative limit to fail")
	}

	// Test limit > MaxLimit
	tooLargeParams := &protocol.PaginationParams{
		Limit:  MaxLimit + 1,
		Cursor: "valid-cursor",
	}
	err = ValidateParams(tooLargeParams)
	if err == nil {
		t.Error("Expected ValidateParams with limit > MaxLimit to fail")
	}

	// The cursor format is considered opaque, so we don't validate it fully here
}

func TestApplyDefaults(t *testing.T) {
	// Test nil params
	params := ApplyDefaults(nil)
	if params == nil {
		t.Fatal("Expected ApplyDefaults(nil) to return non-nil params")
	}
	if params.Limit != DefaultLimit {
		t.Errorf("Expected default limit to be %d, got %d", DefaultLimit, params.Limit)
	}
	if params.Cursor != "" {
		t.Errorf("Expected default cursor to be empty, got %q", params.Cursor)
	}

	// Test with zero limit
	zeroLimitParams := &protocol.PaginationParams{
		Limit:  0,
		Cursor: "test-cursor",
	}
	params = ApplyDefaults(zeroLimitParams)
	if params.Limit != DefaultLimit {
		t.Errorf("Expected limit to be set to default %d, got %d", DefaultLimit, params.Limit)
	}
	if params.Cursor != "test-cursor" {
		t.Errorf("Expected cursor to remain 'test-cursor', got %q", params.Cursor)
	}

	// Test with negative limit
	negativeLimitParams := &protocol.PaginationParams{
		Limit:  -10,
		Cursor: "test-cursor",
	}
	params = ApplyDefaults(negativeLimitParams)
	if params.Limit != DefaultLimit {
		t.Errorf("Expected negative limit to be converted to default %d, got %d", DefaultLimit, params.Limit)
	}

	// Test with limit > MaxLimit
	tooLargeParams := &protocol.PaginationParams{
		Limit:  MaxLimit + 100,
		Cursor: "test-cursor",
	}
	params = ApplyDefaults(tooLargeParams)
	if params.Limit != MaxLimit {
		t.Errorf("Expected limit to be capped at MaxLimit %d, got %d", MaxLimit, params.Limit)
	}

	// Test with valid params (should keep original values)
	validParams := &protocol.PaginationParams{
		Limit:  75,
		Cursor: "test-cursor",
	}
	params = ApplyDefaults(validParams)
	if params.Limit != 75 {
		t.Errorf("Expected limit to remain 75, got %d", params.Limit)
	}
	if params.Cursor != "test-cursor" {
		t.Errorf("Expected cursor to remain 'test-cursor', got %q", params.Cursor)
	}

	// Verify that input params are not modified
	if validParams.Limit != 75 {
		t.Errorf("Expected original params to be unchanged, got limit %d", validParams.Limit)
	}
}

func TestHasNextPage(t *testing.T) {
	// Test nil result
	if HasNextPage(nil) {
		t.Error("Expected HasNextPage(nil) to return false")
	}

	// Test result with no more pages
	noMoreResult := &protocol.PaginationResult{
		HasMore:    false,
		NextCursor: "",
		Items:      []interface{}{},
	}
	if HasNextPage(noMoreResult) {
		t.Error("Expected HasNextPage with no more pages to return false")
	}

	// Test result with more pages but no cursor (invalid state, should return false)
	invalidResult := &protocol.PaginationResult{
		HasMore:    true,
		NextCursor: "",
		Items:      []interface{}{},
	}
	if HasNextPage(invalidResult) {
		t.Error("Expected HasNextPage with more pages but no cursor to return false")
	}

	// Test result with cursor but HasMore=false (invalid state, should return false)
	invalidResult2 := &protocol.PaginationResult{
		HasMore:    false,
		NextCursor: "next-cursor",
		Items:      []interface{}{},
	}
	if HasNextPage(invalidResult2) {
		t.Error("Expected HasNextPage with cursor but HasMore=false to return false")
	}

	// Test result with more pages and valid cursor
	validResult := &protocol.PaginationResult{
		HasMore:    true,
		NextCursor: "next-cursor",
		Items:      []interface{}{},
	}
	if !HasNextPage(validResult) {
		t.Error("Expected HasNextPage with more pages and valid cursor to return true")
	}
}

func TestCollector(t *testing.T) {
	// Test new collector
	collector := NewCollector()
	if collector == nil {
		t.Fatal("Expected NewCollector to return non-nil collector")
	}
	if !collector.HasMore {
		t.Error("Expected new collector to have HasMore=true")
	}
	if collector.NextCursor != "" {
		t.Errorf("Expected new collector to have empty NextCursor, got %q", collector.NextCursor)
	}
	if collector.TotalItems != 0 {
		t.Errorf("Expected new collector to have TotalItems=0, got %d", collector.TotalItems)
	}

	// Test updating collector with nil result
	collector.Update(nil)
	if collector.HasMore {
		t.Error("Expected collector.Update(nil) to set HasMore=false")
	}

	// Test updating collector with valid result
	collector = NewCollector() // Reset collector
	result1 := &protocol.PaginationResult{
		HasMore:    true,
		NextCursor: "page-2",
		Items:      []interface{}{1, 2, 3},
	}
	collector.Update(result1)
	if !collector.HasMore {
		t.Error("Expected collector to have HasMore=true after update")
	}
	if collector.NextCursor != "page-2" {
		t.Errorf("Expected collector to have NextCursor='page-2', got %q", collector.NextCursor)
	}
	if collector.TotalItems != 3 {
		t.Errorf("Expected collector to have TotalItems=3, got %d", collector.TotalItems)
	}

	// Test updating collector with second page
	result2 := &protocol.PaginationResult{
		HasMore:    true,
		NextCursor: "page-3",
		Items:      []interface{}{4, 5, 6, 7},
	}
	collector.Update(result2)
	if collector.NextCursor != "page-3" {
		t.Errorf("Expected collector to have NextCursor='page-3', got %q", collector.NextCursor)
	}
	if collector.TotalItems != 7 {
		t.Errorf("Expected collector to have TotalItems=7, got %d", collector.TotalItems)
	}

	// Test updating collector with final page
	result3 := &protocol.PaginationResult{
		HasMore:    false,
		NextCursor: "",
		Items:      []interface{}{8, 9},
	}
	collector.Update(result3)
	if collector.HasMore {
		t.Error("Expected collector to have HasMore=false after final page")
	}
	if collector.NextCursor != "" {
		t.Errorf("Expected collector to have empty NextCursor, got %q", collector.NextCursor)
	}
	if collector.TotalItems != 9 {
		t.Errorf("Expected collector to have TotalItems=9, got %d", collector.TotalItems)
	}

	// Test NextParams
	collector = NewCollector() // Reset collector
	collector.NextCursor = "test-cursor"
	
	// With nil base params
	params := collector.NextParams(nil)
	if params.Cursor != "test-cursor" {
		t.Errorf("Expected NextParams to set cursor='test-cursor', got %q", params.Cursor)
	}
	if params.Limit != DefaultLimit {
		t.Errorf("Expected NextParams to use DefaultLimit, got %d", params.Limit)
	}
	
	// With custom base params
	baseParams := &protocol.PaginationParams{
		Limit: 25,
	}
	params = collector.NextParams(baseParams)
	if params.Cursor != "test-cursor" {
		t.Errorf("Expected NextParams to set cursor='test-cursor', got %q", params.Cursor)
	}
	if params.Limit != 25 {
		t.Errorf("Expected NextParams to keep limit=25, got %d", params.Limit)
	}
}
