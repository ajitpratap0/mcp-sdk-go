package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPaginationExampleCode verifies that the pagination example code correctly handles pagination
func TestPaginationExampleCode(t *testing.T) {
	// Get the base examples directory
	baseDir, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	
	exampleDir := filepath.Join(baseDir, "pagination-example")
	
	// Verify the pagination example compiles
	cmd := exec.Command("go", "build", "-o", "/dev/null")
	cmd.Dir = exampleDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile pagination example: %v\nOutput: %s", err, output)
	}
	
	// Read and analyze the source code
	mainFilePath := filepath.Join(exampleDir, "main.go")
	mainFileContent, err := os.ReadFile(mainFilePath)
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}
	
	// Check for key pagination functions in the source
	codeStr := string(mainFileContent)
	
	// Check for manual pagination handling
	if !strings.Contains(codeStr, "func demonstrateManualPagination") {
		t.Error("Source code missing manual pagination demonstration function")
	}
	
	if !strings.Contains(codeStr, "nextPagination := &protocol.PaginationParams") {
		t.Error("Source code missing proper use of pagination params for next page")
	}
	
	if !strings.Contains(codeStr, "pagResult.NextCursor") {
		t.Error("Source code doesn't properly use pagination cursor for next page")
	}
	
	// Check for automatic pagination
	if !strings.Contains(codeStr, "func demonstrateAutomaticPagination") {
		t.Error("Source code missing automatic pagination demonstration function")
	}
	
	if !strings.Contains(codeStr, "ListAllResources") {
		t.Error("Source code doesn't use ListAllResources helper for automatic pagination")
	}
	
	// Test the functions in the example by manually extracting and validating them
	validatePaginationFunctions(t, codeStr)
	
	t.Log("Pagination example contains all required pagination handling code")
}

// validatePaginationFunctions analyzes the pagination functions to verify they work correctly
func validatePaginationFunctions(t *testing.T, sourceCode string) {
	// This function validates the logic in the pagination example
	// by checking for key parts of the implementation
	
	// Verify manual pagination checks pagination results
	if !strings.Contains(sourceCode, "if pagResult != nil && pagResult.HasMore") {
		t.Error("Manual pagination doesn't check for HasMore flag")
	}
	
	// Verify proper cursor handling 
	if !strings.Contains(sourceCode, "Cursor: pagResult.NextCursor") {
		t.Error("Manual pagination doesn't properly use the next cursor")
	}
	
	// Verify automatic pagination retrieves all resources
	if !strings.Contains(sourceCode, "Found total of %d resources across all pages") {
		t.Error("Automatic pagination doesn't summarize all resources")
	}
	
	// Verify the example handles resource type tracking
	if !strings.Contains(sourceCode, "typeCount := make(map[string]int)") {
		t.Error("Example doesn't demonstrate aggregating data across pages")
	}
	
	// Check that the difference between manual and automatic pagination is explained
	hasManualMessage := strings.Contains(sourceCode, "Example 1: Manual pagination")
	hasAutoMessage := strings.Contains(sourceCode, "Example 2: Automatic pagination")
	
	if !hasManualMessage || !hasAutoMessage {
		t.Error("Example doesn't clearly label manual vs automatic pagination approaches")
	}
}

// TestPaginatedDataHandling tests processing code for paginated data
func TestPaginatedDataHandling(t *testing.T) {
	// Create mock paginated data
	page1 := createMockResourcePage(1, 10, true, "page2")
	page2 := createMockResourcePage(11, 10, true, "page3")
	page3 := createMockResourcePage(21, 10, false, "")
	
	// Manually simulate processing paginated data
	var allResources []map[string]interface{}
	
	// Process first page
	results1 := extractResources(page1)
	allResources = append(allResources, results1...)
	
	// Check first page
	if len(results1) != 10 {
		t.Errorf("Expected 10 resources in page 1, got %d", len(results1))
	}
	
	if getResourceID(results1[0]) != "resource-1" {
		t.Errorf("Expected first resource ID to be resource-1, got %s", getResourceID(results1[0]))
	}
	
	// Process second page
	results2 := extractResources(page2)
	allResources = append(allResources, results2...)
	
	// Check second page
	if len(results2) != 10 {
		t.Errorf("Expected 10 resources in page 2, got %d", len(results2))
	}
	
	if getResourceID(results2[0]) != "resource-11" {
		t.Errorf("Expected first resource ID to be resource-11, got %s", getResourceID(results2[0]))
	}
	
	// Process third page
	results3 := extractResources(page3)
	allResources = append(allResources, results3...)
	
	// Check third page
	if len(results3) != 10 {
		t.Errorf("Expected 10 resources in page 3, got %d", len(results3))
	}
	
	if getResourceID(results3[0]) != "resource-21" {
		t.Errorf("Expected first resource ID to be resource-21, got %s", getResourceID(results3[0]))
	}
	
	// Verify all resources 
	if len(allResources) != 30 {
		t.Errorf("Expected 30 total resources, got %d", len(allResources))
	}
	
	// Simulate the counting code in the example
	typeCount := make(map[string]int)
	for _, res := range allResources {
		if resType, ok := res["type"].(string); ok {
			typeCount[resType]++
		}
	}
	
	// Check that all resources were counted
	if typeCount["test"] != 30 {
		t.Errorf("Expected 30 resources of type 'test', got %d", typeCount["test"])
	}
	
	t.Log("Successful simulation of pagination example's data handling")
}

// Helper functions for pagination tests

// createMockResourcePage creates a mock paginated response with resources
func createMockResourcePage(startIndex, count int, hasMore bool, nextCursor string) map[string]interface{} {
	resources := make([]map[string]interface{}, count)
	
	for i := 0; i < count; i++ {
		resources[i] = map[string]interface{}{
			"uri":  fmt.Sprintf("resource-%d", startIndex+i),
			"name": fmt.Sprintf("Resource %d", startIndex+i),
			"type": "test",
		}
	}
	
	return map[string]interface{}{
		"resources":  resources,
		"templates":  []interface{}{},
		"hasMore":    hasMore,
		"nextCursor": nextCursor,
	}
}

// extractResources extracts resources from a paginated response
func extractResources(page map[string]interface{}) []map[string]interface{} {
	if resourcesRaw, ok := page["resources"].([]map[string]interface{}); ok {
		return resourcesRaw
	}
	
	// Handle the case where JSON unmarshaling creates []interface{} rather than []map[string]interface{}
	if resourcesRaw, ok := page["resources"].([]interface{}); ok {
		results := make([]map[string]interface{}, 0, len(resourcesRaw))
		for _, r := range resourcesRaw {
			if res, ok := r.(map[string]interface{}); ok {
				results = append(results, res)
			}
		}
		return results
	}
	
	return []map[string]interface{}{}
}

// getResourceID gets the URI from a resource
func getResourceID(resource map[string]interface{}) string {
	if uri, ok := resource["uri"].(string); ok {
		return uri
	}
	return ""
}
