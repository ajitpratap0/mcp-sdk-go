package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
)

// TestLogger tests the basic logger functionality
func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())
	logger.SetLevel(DebugLevel) // Enable debug logging

	// Test different log levels
	logger.Debug("Debug message", String("key", "value"))
	logger.Info("Info message", Int("count", 42))
	logger.Warn("Warning message", Bool("flag", true))
	logger.Error("Error message", ErrorField(errors.New("test error")))

	output := buf.String()

	// Check that all messages were logged
	if !strings.Contains(output, "Debug message") {
		t.Error("Expected debug message in output")
	}
	if !strings.Contains(output, "Info message") {
		t.Error("Expected info message in output")
	}
	if !strings.Contains(output, "Warning message") {
		t.Error("Expected warning message in output")
	}
	if !strings.Contains(output, "Error message") {
		t.Error("Expected error message in output")
	}

	// Check fields
	if !strings.Contains(output, "key=value") {
		t.Error("Expected key=value in output")
	}
	if !strings.Contains(output, "count=42") {
		t.Error("Expected count=42 in output")
	}
	if !strings.Contains(output, "flag=true") {
		t.Error("Expected flag=true in output")
	}
	if !strings.Contains(output, "error=test error") {
		t.Error("Expected error=test error in output")
	}
}

// TestLogLevels tests log level filtering
func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())

	// Set level to warn
	logger.SetLevel(WarnLevel)

	// Log at different levels
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")

	output := buf.String()

	// Debug and info should be filtered out
	if strings.Contains(output, "Debug message") {
		t.Error("Debug message should be filtered out")
	}
	if strings.Contains(output, "Info message") {
		t.Error("Info message should be filtered out")
	}

	// Warn and error should be present
	if !strings.Contains(output, "Warning message") {
		t.Error("Warning message should be present")
	}
	if !strings.Contains(output, "Error message") {
		t.Error("Error message should be present")
	}
}

// TestWithFields tests field inheritance
func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())

	// Create logger with base fields
	logger = logger.WithFields(
		String("service", "test-service"),
		String("version", "1.0.0"),
	)

	// Log a message
	logger.Info("Test message", String("operation", "test"))

	output := buf.String()

	// Check all fields are present
	if !strings.Contains(output, "service=test-service") {
		t.Error("Expected service field")
	}
	if !strings.Contains(output, "version=1.0.0") {
		t.Error("Expected version field")
	}
	if !strings.Contains(output, "operation=test") {
		t.Error("Expected operation field")
	}
}

// TestWithContext tests context integration
func TestWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())

	// Create context with request ID
	ctx := ContextWithRequestID(context.Background(), "test-request-123")

	// Create logger with context
	logger = logger.WithContext(ctx)

	// Log a message
	logger.Info("Test message")

	output := buf.String()

	// Check request ID is present
	if !strings.Contains(output, "[test-request-123]") {
		t.Error("Expected request ID in output")
	}
}

// TestWithError tests error context integration
func TestWithError(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())

	// Create MCP error with context
	mcpErr := mcperrors.InvalidParameter("test_param", "invalid_value", "string").
		WithContext(&mcperrors.Context{
			RequestID: "req-123",
			Component: "TestComponent",
			Operation: "TestOperation",
		})

	// Create logger with error
	logger = logger.WithError(mcpErr)

	// Log a message
	logger.Error("Operation failed")

	output := buf.String()

	// Check error details are present
	if !strings.Contains(output, "error=") {
		t.Error("Expected error field")
	}
	if !strings.Contains(output, "error_code=-32752") {
		t.Error("Expected error_code field")
	}
	if !strings.Contains(output, "error_category=validation") {
		t.Error("Expected error_category field")
	}
	if !strings.Contains(output, "[req-123]") {
		t.Error("Expected request ID from error context")
	}
	// Component and operation are shown in the special formatting section, not as fields
	if !strings.Contains(output, "TestComponent/TestOperation:") {
		t.Error("Expected component and operation in message formatting")
	}
}

// TestJSONFormatter tests JSON output formatting
func TestJSONFormatter(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewJSONFormatter())

	// Log a message with fields
	logger.Info("Test message",
		String("key", "value"),
		Int("count", 42),
		Bool("flag", true),
	)

	// Parse JSON output
	var entry map[string]interface{}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Check fields
	if entry["level"] != "INFO" {
		t.Errorf("Expected level INFO, got %v", entry["level"])
	}
	if entry["message"] != "Test message" {
		t.Errorf("Expected message 'Test message', got %v", entry["message"])
	}
	if entry["key"] != "value" {
		t.Errorf("Expected key='value', got %v", entry["key"])
	}
	if entry["count"] != float64(42) { // JSON numbers are float64
		t.Errorf("Expected count=42, got %v", entry["count"])
	}
	if entry["flag"] != true {
		t.Errorf("Expected flag=true, got %v", entry["flag"])
	}

	// Check timestamp exists
	if _, ok := entry["timestamp"]; !ok {
		t.Error("Expected timestamp field")
	}
}

// TestLegacyAdapter tests backward compatibility
func TestLegacyAdapter(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())
	logger.SetLevel(DebugLevel) // Enable debug logging
	adapter := NewLegacyAdapter(logger)

	// Use legacy interface
	adapter.Debug("Debug: %s %d", "test", 123)
	adapter.Info("Info: %s", "message")
	adapter.Warn("Warning: %v", errors.New("test warning"))
	adapter.Error("Error: %v", errors.New("test error"))

	output := buf.String()

	// Check messages
	if !strings.Contains(output, "Debug: test 123") {
		t.Error("Expected debug message")
	}
	if !strings.Contains(output, "Info: message") {
		t.Error("Expected info message")
	}
	if !strings.Contains(output, "Warning: test warning") {
		t.Error("Expected warning message")
	}
	if !strings.Contains(output, "Error: test error") {
		t.Error("Expected error message")
	}
}

// TestTransportAdapter tests transport logging adapter
func TestTransportAdapter(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())
	logger.SetLevel(DebugLevel) // Enable debug logging
	adapter := NewTransportAdapter(logger, "TestTransport")

	// Use transport interface
	adapter.Logf("Starting connection to %s:%d", "localhost", 8080)
	adapter.Logf("ERROR: Connection failed: %v", errors.New("connection refused"))
	adapter.Logf("DEBUG: Retry attempt %d", 1)

	output := buf.String()

	// Check component is displayed in header
	if !strings.Contains(output, "TestTransport:") {
		t.Error("Expected component in message header")
	}

	// Check messages and levels
	if !strings.Contains(output, "[INFO]") || !strings.Contains(output, "Starting connection") {
		t.Error("Expected info level for regular message")
	}
	if !strings.Contains(output, "[ERROR]") || !strings.Contains(output, "Connection failed") {
		t.Error("Expected error level for ERROR message")
	}
	if !strings.Contains(output, "[DEBUG]") || !strings.Contains(output, "Retry attempt") {
		t.Error("Expected debug level for DEBUG message")
	}
}

// TestFieldTypes tests different field types
func TestFieldTypes(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewJSONFormatter())

	now := time.Now()
	duration := 5 * time.Second

	logger.Info("Test fields",
		String("string", "value"),
		Int("int", 42),
		Bool("bool", true),
		Duration("duration", duration),
		Time("time", now),
		Any("any", map[string]int{"a": 1, "b": 2}),
		ErrorField(errors.New("test error")),
	)

	// Parse JSON output
	var entry map[string]interface{}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Check fields
	if entry["string"] != "value" {
		t.Error("Expected string field")
	}
	if entry["int"] != float64(42) {
		t.Error("Expected int field")
	}
	if entry["bool"] != true {
		t.Error("Expected bool field")
	}
	if entry["error"] != "test error" {
		t.Error("Expected error field")
	}

	// Duration should be in nanoseconds
	if _, ok := entry["duration"].(float64); !ok {
		t.Error("Expected duration as number")
	}

	// Time should be formatted
	if _, ok := entry["time"].(string); !ok {
		t.Error("Expected time as string")
	}

	// Any should preserve structure
	if anyVal, ok := entry["any"].(map[string]interface{}); ok {
		if anyVal["a"] != float64(1) || anyVal["b"] != float64(2) {
			t.Error("Expected any field to preserve map structure")
		}
	} else {
		t.Error("Expected any field as map")
	}
}

// TestGlobalLogger tests the global logger functions
func TestGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, NewTextFormatter())
	logger.SetLevel(DebugLevel) // Enable debug logging
	SetGlobalLogger(logger)

	// Use global functions
	Debug("Debug message", String("key", "value"))
	Info("Info message")
	Warn("Warning message")
	LogError("Error message")

	output := buf.String()

	// Check all messages
	if !strings.Contains(output, "Debug message") {
		t.Error("Expected debug message")
	}
	if !strings.Contains(output, "Info message") {
		t.Error("Expected info message")
	}
	if !strings.Contains(output, "Warning message") {
		t.Error("Expected warning message")
	}
	if !strings.Contains(output, "Error message") {
		t.Error("Expected error message")
	}
}
