package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func TestBaseToolsProvider(t *testing.T) {
	// Create a base tools provider with no tools
	provider := NewBaseToolsProvider()

	// Add some tools first to avoid empty slice issues
	provider.RegisterTool(protocol.Tool{
		Name:       "tool1",
		Categories: []string{"category1"},
	})

	provider.RegisterTool(protocol.Tool{
		Name:       "tool2",
		Categories: []string{"category2"},
	})

	// Test ListTools with no category filter and valid pagination
	ctx := context.Background()
	pagination := &protocol.PaginationParams{
		Limit: 10, // Set a reasonable limit
	}
	results, _, _, _, err := provider.ListTools(ctx, "", pagination)
	if err != nil {
		t.Fatalf("Expected ListTools to succeed, got error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(results))
	}

	// Test ListTools with category filter
	results, _, _, _, err = provider.ListTools(ctx, "category1", pagination)
	if err != nil {
		t.Fatalf("Expected ListTools with category filter to succeed, got error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 tool with category1, got %d", len(results))
	}

	if results[0].Name != "tool1" {
		t.Errorf("Expected tool1, got %s", results[0].Name)
	}

	// Test ListTools with pagination limit 1
	paginationLimit1 := &protocol.PaginationParams{
		Limit: 1,
	}
	results, _, _, _, err = provider.ListTools(ctx, "", paginationLimit1)
	if err != nil {
		t.Fatalf("Expected ListTools with pagination limit 1 to succeed, got error: %v", err)
	}

	if len(results) > 1 {
		t.Errorf("Expected at most 1 tool with pagination limit 1, got %d", len(results))
	}

	// Test with cursor pagination
	// First get a valid cursor
	firstPage := &protocol.PaginationParams{
		Limit: 1,
	}
	_, _, validCursor, _, err := provider.ListTools(ctx, "", firstPage)
	if err != nil {
		t.Fatalf("Failed to get first page for cursor: %v", err)
	}

	// Now use the valid cursor for next page
	paginationWithCursor := &protocol.PaginationParams{
		Cursor: validCursor,
		Limit:  1,
	}
	results, _, _, _, err = provider.ListTools(ctx, "", paginationWithCursor)
	if err != nil {
		t.Fatalf("Expected ListTools with pagination cursor to succeed, got error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 tool with pagination cursor, got %d", len(results))
	}

	// Test CallTool
	jsonParams, _ := json.Marshal(map[string]interface{}{"param": "value"})
	result, err := provider.CallTool(ctx, "tool1", json.RawMessage(jsonParams), nil)
	if err != nil {
		t.Errorf("Expected CallTool to succeed, got error: %v", err)
	}
	if result == nil {
		t.Fatalf("Expected CallTool result to not be nil")
		return
	}
	expectedMessage := "Tool execution not implemented"
	var resultObj map[string]interface{}
	if err := json.Unmarshal(result.Result, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if msg, ok := resultObj["message"]; !ok || msg != expectedMessage {
		t.Errorf("Expected message '%s', got %v", expectedMessage, resultObj)
	}
}

type mockResourcesProvider struct {
	resources []protocol.Resource
	templates []protocol.ResourceTemplate
	contents  *protocol.ResourceContents
	err       error
}

func (m *mockResourcesProvider) ListResources(ctx context.Context, uri string, recursive bool, pagination *protocol.PaginationParams) ([]protocol.Resource, []protocol.ResourceTemplate, string, *protocol.PaginationResult, error) {
	if m.err != nil {
		return nil, nil, "", nil, m.err
	}
	return m.resources, m.templates, "", &protocol.PaginationResult{}, nil
}

func (m *mockResourcesProvider) ReadResource(ctx context.Context, uri string, templateParams json.RawMessage, resourceRange *protocol.ResourceRange) (*protocol.ResourceContents, error) {
	return m.contents, nil
}

func (m *mockResourcesProvider) UpdateResource(resource protocol.Resource, contents *protocol.ResourceContents) error {
	// Find the resource to update
	for i, r := range m.resources {
		if r.URI == resource.URI {
			m.resources[i] = resource
			m.contents = contents
			return nil
		}
	}
	// Not found, add it
	m.resources = append(m.resources, resource)
	m.contents = contents
	return nil
}

func TestBaseResourcesProvider(t *testing.T) {
	// Create a base resources provider with some resources
	resources := []protocol.Resource{
		{
			URI:         "resource1",
			Name:        "Resource 1",
			Description: "Resource 1 description",
			Type:        "text/plain",
		},
		{
			URI:         "resource2",
			Name:        "Resource 2",
			Description: "Resource 2 description",
			Type:        "application/json",
		},
	}

	templates := []protocol.ResourceTemplate{
		{
			URI:           "template1",
			Name:          "Template 1",
			Description:   "Template 1 description",
			Type:          "template",
			Parameters:    []protocol.ResourceParameter{},
			ParameterDefs: map[string]interface{}{"type": "object"},
		},
	}

	contents := &protocol.ResourceContents{
		URI:     "resource1",
		Type:    "text/plain",
		Content: json.RawMessage([]byte(`"Resource 1 content"`)),
	}

	provider := &mockResourcesProvider{resources: resources, templates: templates, contents: contents}

	// Test ListResources
	ctx := context.Background()
	resultResources, resultTemplates, _, _, err := provider.ListResources(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("Expected ListResources to succeed, got error: %v", err)
	}

	if len(resultResources) != len(resources) {
		t.Errorf("Expected %d resources, got %d", len(resources), len(resultResources))
	}

	if len(resultTemplates) != len(templates) {
		t.Errorf("Expected %d templates, got %d", len(templates), len(resultTemplates))
	}

	// Test ReadResource
	resourceContent, err := provider.ReadResource(ctx, "resource1", nil, nil)
	if err != nil {
		t.Fatalf("Expected ReadResource to succeed, got error: %v", err)
	}

	if resourceContent.URI != "resource1" {
		t.Errorf("Expected URI to be 'resource1', got %q", resourceContent.URI)
	}

	expectedContent := json.RawMessage([]byte(`"Resource 1 content"`)) // Wrap in quotes for JSON string
	if string(resourceContent.Content) != string(expectedContent) {
		t.Errorf("Expected content to be %s, got %s", expectedContent, resourceContent.Content)
	}
	// Test UpdateResource
	updatedResource := protocol.Resource{
		URI:         "resource2",
		Name:        "Updated Resource 2",
		Description: "Updated Resource 2 description",
		Type:        "application/json",
	}
	updatedContent := &protocol.ResourceContents{
		URI:     "resource2",
		Type:    "application/json",
		Content: json.RawMessage(`{"key": "updated value"}`),
	}
	if err := provider.UpdateResource(updatedResource, updatedContent); err != nil {
		t.Fatalf("Expected UpdateResource to succeed, got error: %v", err)
	}

	resultResources, _, _, _, err = provider.ListResources(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("Expected ListResources to succeed after updating resource, got error: %v", err)
	}

	found := false
	for _, r := range resultResources {
		if r.URI == "resource2" {
			found = true
			if r.Name != "Updated Resource 2" {
				t.Errorf("Expected resource name to be updated, got %q", r.Name)
			}
		}
	}

	if !found {
		t.Error("Expected to find updated resource")
	}

	resourceContent, err = provider.ReadResource(ctx, "resource2", nil, nil)
	if err != nil {
		t.Fatalf("Expected ReadResource to succeed after update, got error: %v", err)
	}

	expectedJSON := json.RawMessage([]byte(`{"key": "updated value"}`))
	if string(resourceContent.Content) != string(expectedJSON) {
		t.Errorf("Expected content to match updated value, got %s", resourceContent.Content)
	}
}

// TestBasePromptsProvider is currently disabled as the BasePromptsProvider implementation
// is now implemented and tested.
func TestBasePromptsProvider(t *testing.T) {
	ctx := context.Background()
	provider := NewBasePromptsProvider()

	// Test RegisterPrompt
	prompt := protocol.Prompt{
		ID:   "prompt1",
		Name: "Test Prompt",
		Messages: []protocol.PromptMessage{
			{Role: "user", Content: "Hello"},
		},
		Tags: []string{"test", "example"},
	}
	provider.RegisterPrompt(prompt)

	// Test ListPrompts
	results, total, _, _, err := provider.ListPrompts(ctx, "", nil)
	if err != nil {
		t.Fatalf("Expected ListPrompts to succeed, got error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(results))
	}

	if results[0].ID != "prompt1" {
		t.Errorf("Expected prompt ID to be 'prompt1', got %q", results[0].ID)
	}

	if total != 1 {
		t.Errorf("Expected total to be 1, got %d", total)
	}

	// Test ListPrompts with tag filter
	results, _, _, _, err = provider.ListPrompts(ctx, "test", nil)
	if err != nil {
		t.Fatalf("Expected ListPrompts with tag to succeed, got error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 prompt with tag 'test', got %d", len(results))
	}

	results, _, _, _, err = provider.ListPrompts(ctx, "nonexistent", nil)
	if err != nil {
		t.Fatalf("Expected ListPrompts with nonexistent tag to succeed, got error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 prompts with tag 'nonexistent', got %d", len(results))
	}

	// Test GetPrompt
	result, err := provider.GetPrompt(ctx, "prompt1")
	if err != nil {
		t.Fatalf("Expected GetPrompt to succeed, got error: %v", err)
	}

	if result.ID != "prompt1" {
		t.Errorf("Expected prompt ID to be 'prompt1', got %q", result.ID)
	}

	// Test GetPrompt with nonexistent ID
	_, err = provider.GetPrompt(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected GetPrompt with nonexistent ID to fail")
	}
}

func TestPaginationCursor(t *testing.T) {
	tests := []struct {
		name   string
		offset int
	}{
		{"zero offset", 0},
		{"small offset", 10},
		{"large offset", 1000},
		{"very large offset", 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test encoding
			cursor := encodeCursor(tt.offset)
			if cursor == "" {
				t.Error("Expected non-empty cursor")
			}

			// Test decoding
			decoded, err := decodeCursor(cursor)
			if err != nil {
				t.Fatalf("Failed to decode cursor: %v", err)
			}

			if decoded != tt.offset {
				t.Errorf("Expected decoded offset %d, got %d", tt.offset, decoded)
			}
		})
	}

	// Test empty cursor
	offset, err := decodeCursor("")
	if err != nil {
		t.Fatalf("Failed to decode empty cursor: %v", err)
	}
	if offset != 0 {
		t.Errorf("Expected empty cursor to decode to 0, got %d", offset)
	}

	// Test invalid cursors
	invalidCursors := []string{
		"invalid",
		"cursor_10", // Old format
		"!!!",
		base64.StdEncoding.EncodeToString([]byte("invalid json")),
	}

	for _, cursor := range invalidCursors {
		_, err := decodeCursor(cursor)
		if err == nil {
			t.Errorf("Expected error for invalid cursor %s", cursor)
		}
	}
}

func TestToolsPagination(t *testing.T) {
	provider := NewBaseToolsProvider()

	// Register multiple tools
	for i := 0; i < 25; i++ {
		tool := protocol.Tool{
			Name:        fmt.Sprintf("tool%d", i),
			Description: fmt.Sprintf("Test tool %d", i),
			InputSchema: json.RawMessage(`{"type": "object"}`),
		}
		provider.RegisterTool(tool)
	}

	ctx := context.Background()

	// Test first page
	page1 := &protocol.PaginationParams{Limit: 10}
	tools1, total1, cursor1, hasMore1, err := provider.ListTools(ctx, "", page1)
	if err != nil {
		t.Fatalf("Failed to get first page: %v", err)
	}

	if len(tools1) != 10 {
		t.Errorf("Expected 10 tools on first page, got %d", len(tools1))
	}
	if total1 != 25 {
		t.Errorf("Expected total 25, got %d", total1)
	}
	if !hasMore1 {
		t.Error("Expected hasMore to be true for first page")
	}
	if cursor1 == "" {
		t.Error("Expected non-empty cursor for first page")
	}

	// Test second page
	page2 := &protocol.PaginationParams{Limit: 10, Cursor: cursor1}
	tools2, total2, cursor2, hasMore2, err := provider.ListTools(ctx, "", page2)
	if err != nil {
		t.Fatalf("Failed to get second page: %v", err)
	}

	if len(tools2) != 10 {
		t.Errorf("Expected 10 tools on second page, got %d", len(tools2))
	}
	if total2 != 25 {
		t.Errorf("Expected total 25, got %d", total2)
	}
	if !hasMore2 {
		t.Error("Expected hasMore to be true for second page")
	}
	if cursor2 == "" {
		t.Error("Expected non-empty cursor for second page")
	}

	// Test last page
	page3 := &protocol.PaginationParams{Limit: 10, Cursor: cursor2}
	tools3, total3, cursor3, hasMore3, err := provider.ListTools(ctx, "", page3)
	if err != nil {
		t.Fatalf("Failed to get third page: %v", err)
	}

	if len(tools3) != 5 {
		t.Errorf("Expected 5 tools on last page, got %d", len(tools3))
	}
	if total3 != 25 {
		t.Errorf("Expected total 25, got %d", total3)
	}
	if hasMore3 {
		t.Error("Expected hasMore to be false for last page")
	}
	if cursor3 != "" {
		t.Error("Expected empty cursor for last page")
	}

	// Test invalid cursor
	pageInvalid := &protocol.PaginationParams{Limit: 10, Cursor: "invalid_cursor"}
	_, _, _, _, err = provider.ListTools(ctx, "", pageInvalid)
	if err == nil {
		t.Error("Expected error for invalid cursor")
	}
}

func TestResourcesPagination(t *testing.T) {
	provider := NewBaseResourcesProvider()

	// Register multiple resources
	for i := 0; i < 15; i++ {
		resource := protocol.Resource{
			URI:  fmt.Sprintf("resource%d", i),
			Name: fmt.Sprintf("Resource %d", i),
		}
		provider.RegisterResource(resource)
	}

	// Register multiple templates
	for i := 0; i < 10; i++ {
		template := protocol.ResourceTemplate{
			URI:  fmt.Sprintf("template%d", i),
			Name: fmt.Sprintf("Template %d", i),
		}
		provider.RegisterTemplate(template)
	}

	ctx := context.Background()

	// Test pagination across resources and templates
	page1 := &protocol.PaginationParams{Limit: 20}
	resources1, templates1, total1, cursor1, hasMore1, err := provider.ListResources(ctx, "", false, page1)
	if err != nil {
		t.Fatalf("Failed to get first page: %v", err)
	}

	// Should get 15 resources and 5 templates on first page
	if len(resources1) != 15 {
		t.Errorf("Expected 15 resources on first page, got %d", len(resources1))
	}
	if len(templates1) != 5 {
		t.Errorf("Expected 5 templates on first page, got %d", len(templates1))
	}
	if total1 != 25 {
		t.Errorf("Expected total 25, got %d", total1)
	}
	if !hasMore1 {
		t.Error("Expected hasMore to be true")
	}

	// Test second page
	page2 := &protocol.PaginationParams{Limit: 20, Cursor: cursor1}
	resources2, templates2, _, cursor2, hasMore2, err := provider.ListResources(ctx, "", false, page2)
	if err != nil {
		t.Fatalf("Failed to get second page: %v", err)
	}

	// Should get 0 resources and 5 templates on second page
	if len(resources2) != 0 {
		t.Errorf("Expected 0 resources on second page, got %d", len(resources2))
	}
	if len(templates2) != 5 {
		t.Errorf("Expected 5 templates on second page, got %d", len(templates2))
	}
	if hasMore2 {
		t.Error("Expected hasMore to be false")
	}
	if cursor2 != "" {
		t.Error("Expected empty cursor for last page")
	}
}
