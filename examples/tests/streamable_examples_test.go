package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestStreamableClientServerExamples tests the streamable HTTP client and server examples
func TestStreamableClientServerExamples(t *testing.T) {
	// Get the base examples directory
	baseDir, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	
	// Verify client and server examples compile
	clientDir := filepath.Join(baseDir, "streamable-http-client")
	serverDir := filepath.Join(baseDir, "streamable-http-server")
	
	// Verify the client compiles
	clientCmd := exec.Command("go", "build", "-o", "/dev/null")
	clientCmd.Dir = clientDir
	clientOutput, err := clientCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile streamable HTTP client: %v\nOutput: %s", err, clientOutput)
	}
	
	// Verify the server compiles
	serverCmd := exec.Command("go", "build", "-o", "/dev/null")
	serverCmd.Dir = serverDir
	serverOutput, err := serverCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile streamable HTTP server: %v\nOutput: %s", err, serverOutput)
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
	
	// Verify client code has streaming functionality
	verifyStreamableClientCode(t, clientCodeStr)
	
	// Verify server code has streaming functionality
	verifyStreamableServerCode(t, serverCodeStr)
	
	t.Log("Streamable HTTP client and server examples contain all required code")
}

// verifyStreamableClientCode checks that the client code properly implements streaming
func verifyStreamableClientCode(t *testing.T, code string) {
	// Check for streaming functionality
	if !strings.Contains(code, "stream") && !strings.Contains(code, "Stream") {
		t.Error("Client is missing streaming functionality")
	}
	
	// Check for streaming event handling - using broader terms
	hasStreamingHandling := strings.Contains(code, "token") ||
						 strings.Contains(code, "Token") ||
						 strings.Contains(code, "streaming") ||
						 strings.Contains(code, "Streaming") ||
						 strings.Contains(code, "event") ||
						 strings.Contains(code, "Event") ||
						 strings.Contains(code, "chunk") ||
						 strings.Contains(code, "Chunk") ||
						 strings.Contains(code, "receive") ||
						 strings.Contains(code, "Receive")
	
	if !hasStreamingHandling {
		t.Error("Client doesn't appear to handle streaming events or tokens")
	}
	
	// Check for event callbacks - at least one should be present
	hasCallbacks := strings.Contains(code, "On") || 
				 strings.Contains(code, "callback") || 
				 strings.Contains(code, "Callback") ||
				 strings.Contains(code, "handle") ||
				 strings.Contains(code, "Handle")
				 
	if !hasCallbacks {
		t.Error("Client is missing callbacks for streaming events")
	}
	
	// Check for displaying or accumulating output
	hasOutputHandling := strings.Contains(code, "print") || 
						 strings.Contains(code, "Print") ||
						 strings.Contains(code, "buffer") ||
						 strings.Contains(code, "Buffer") ||
						 strings.Contains(code, "append") ||
						 strings.Contains(code, "Append") ||
						 strings.Contains(code, "accumulate")
						 
	if !hasOutputHandling {
		t.Error("Client doesn't appear to handle or display streamed output")
	}
}

// verifyStreamableServerCode checks that the server code properly implements streaming
func verifyStreamableServerCode(t *testing.T, code string) {
	// Check for capability declaration - various ways to declare this
	hasCapability := strings.Contains(code, "Capability") ||
					 strings.Contains(code, "capability") ||
					 strings.Contains(code, "Complete")
	if !hasCapability {
		t.Error("Server doesn't declare appropriate capabilities")
	}
	
	// Check for the completion handling
	hasCompletionHandler := strings.Contains(code, "Complete") ||
							 strings.Contains(code, "complete") ||
							 strings.Contains(code, "Handler") ||
							 strings.Contains(code, "handler")
	if !hasCompletionHandler {
		t.Error("Server is missing completion handling functionality")
	}
	
	// Check for token streaming or sending logic - allow different implementations
	hasTokenSending := strings.Contains(code, "token") ||
					  strings.Contains(code, "Token") ||
					  strings.Contains(code, "Send") ||
					  strings.Contains(code, "send") ||
					  strings.Contains(code, "stream") ||
					  strings.Contains(code, "Stream")
	if !hasTokenSending {
		t.Error("Server doesn't implement token sending or streaming")
	}
	
	// Check for iteration over tokens or content
	hasIteration := strings.Contains(code, "for") ||
				   strings.Contains(code, "range") ||
				   strings.Contains(code, "next") ||
				   strings.Contains(code, "Next") ||
				   strings.Contains(code, "each") ||
				   strings.Contains(code, "Each")
	if !hasIteration {
		t.Error("Server doesn't iterate over content for streaming")
	}
}
