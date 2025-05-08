package tests

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExamplesCompile verifies that all examples compile successfully
func TestExamplesCompile(t *testing.T) {
	// Get the base examples directory
	baseDir, err := filepath.Abs("../")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// List of example directories to test
	exampleDirs := []string{
		"pagination-example",
		"simple-client",
		"simple-server",
		"stdio-client",
		"streamable-http-client",
		"streamable-http-server",
	}

	for _, dir := range exampleDirs {
		exampleDir := filepath.Join(baseDir, dir)
		t.Run(dir, func(t *testing.T) {
			// Attempt to compile the example
			cmd := exec.Command("go", "build", "-o", "/dev/null")
			cmd.Dir = exampleDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("Failed to compile %s: %v\nOutput: %s", dir, err, output)
			} else {
				t.Logf("Successfully compiled %s", dir)
			}
		})
	}
}

// Note: All tests that required a real server have been replaced with mock-based tests
// in their respective test files.
