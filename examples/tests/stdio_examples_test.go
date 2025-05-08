package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestStdioClientExample tests the stdio client example functionality
func TestStdioClientExample(t *testing.T) {
	// Get the base examples directory
	baseDir, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	
	// Set up stdio client directory
	stdioDir := filepath.Join(baseDir, "stdio-client")
	
	// Verify the example compiles
	buildCmd := exec.Command("go", "build", "-o", "/dev/null")
	buildCmd.Dir = stdioDir
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile stdio client: %v\nOutput: %s", err, output)
	}
	
	// Read and analyze source code
	mainFilePath := filepath.Join(stdioDir, "main.go")
	mainFileContent, err := os.ReadFile(mainFilePath)
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}
	codeStr := string(mainFileContent)
	
	// Verify stdio client functionality
	verifyStdioClientCode(t, codeStr)
	
	t.Log("Stdio client example contains all required functionality")
}

// verifyStdioClientCode checks that the stdio client code has the expected functionality
func verifyStdioClientCode(t *testing.T, code string) {
	// Check for stdio client/transport functionality
	if !strings.Contains(code, "stdio") && !strings.Contains(code, "Stdio") {
		t.Error("Stdio client is missing stdio transport functionality")
	}
	
	// Check for client initialization with capabilities
	if !strings.Contains(code, "client.New") {
		t.Error("Stdio client is missing client initialization")
	}
	
	// Check for standard I/O usage
	if !strings.Contains(code, "os.") {
		t.Error("Stdio client doesn't use os package for standard I/O")
	}
	
	// Check for signal handling
	if !strings.Contains(code, "signal") {
		t.Error("Stdio client doesn't implement signal handling")
	}
	
	// Check for context usage
	if !strings.Contains(code, "context") {
		t.Error("Stdio client doesn't use context")
	}
	
	// Check for transport starting
	if !strings.Contains(code, "Start") {
		t.Error("Stdio client doesn't start the transport")
	}
}
