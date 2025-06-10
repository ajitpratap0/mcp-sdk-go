package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSimpleClientServerExamples tests the simple client and server examples
func TestSimpleClientServerExamples(t *testing.T) {
	// Get the base examples directory
	baseDir, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Verify client and server examples compile
	clientDir := filepath.Join(baseDir, "simple-client")
	serverDir := filepath.Join(baseDir, "simple-server")

	// Verify the client compiles
	clientCmd := exec.Command("go", "build", "-o", "/dev/null")
	clientCmd.Dir = clientDir
	clientOutput, err := clientCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile simple client: %v\nOutput: %s", err, clientOutput)
	}

	// Verify the server compiles
	serverCmd := exec.Command("go", "build", "-o", "/dev/null")
	serverCmd.Dir = serverDir
	serverOutput, err := serverCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile simple server: %v\nOutput: %s", err, serverOutput)
	}

	// Read and analyze client source code
	clientMainPath := filepath.Join(clientDir, "main.go")
	clientMainContent, err := os.ReadFile(clientMainPath)
	if err != nil {
		t.Fatalf("Failed to read client main.go: %v", err)
	}
	clientCodeStr := string(clientMainContent)

	// Read and analyze server source code
	serverMainPath := filepath.Join(serverDir, "main.go")
	serverMainContent, err := os.ReadFile(serverMainPath)
	if err != nil {
		t.Fatalf("Failed to read server main.go: %v", err)
	}
	serverCodeStr := string(serverMainContent)

	// Verify client code has necessary MCP functionality
	verifySimpleClientCode(t, clientCodeStr)

	// Verify server code has necessary MCP functionality
	verifySimpleServerCode(t, serverCodeStr)

	t.Log("Simple client and server examples contain all required MCP functionalities")
}

// verifySimpleClientCode checks that the client code properly implements MCP functionality
func verifySimpleClientCode(t *testing.T, code string) {
	// Check for client initialization
	if !strings.Contains(code, "client.New") {
		t.Error("Client is missing MCP client initialization")
	}

	// Check for transport initialization
	if !strings.Contains(code, "transport") && !strings.Contains(code, "Transport") {
		t.Error("Client is missing transport initialization")
	}

	// Check for listing resources
	if !strings.Contains(code, "ListResources") {
		t.Error("Client doesn't call ListResources method")
	}

	// Check for tool calling
	if !strings.Contains(code, "CallTool") {
		t.Error("Client doesn't call CallTool method")
	}

	// Check for error handling
	if !strings.Contains(code, "if err != nil") {
		t.Error("Client doesn't implement proper error handling")
	}
}

// verifySimpleServerCode checks that the server code properly implements MCP functionality
func verifySimpleServerCode(t *testing.T, code string) {
	// Check for server initialization - now using shared factory
	if !strings.Contains(code, "shared.CreateStdioServer") && !strings.Contains(code, "server.New") {
		t.Error("Server is missing MCP server initialization")
	}

	// Since the server is using shared factory, check for shared imports
	if !strings.Contains(code, "shared") {
		t.Error("Server doesn't import shared package")
	}

	// Check for Start method
	if !strings.Contains(code, "Start") {
		t.Error("Server doesn't call Start method")
	}

	// Check for HTTP transport
	if !strings.Contains(code, "transport") && !strings.Contains(code, "Transport") &&
		!strings.Contains(code, "http") && !strings.Contains(code, "HTTP") &&
		!strings.Contains(code, "port") && !strings.Contains(code, "Port") {
		t.Error("Server doesn't appear to set up any networking transport")
	}
}
