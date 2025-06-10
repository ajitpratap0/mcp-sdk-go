package transport

import (
	"context"
	"testing"
)

// TestStreamableHTTPTransportOriginValidation tests client-side origin validation
func TestStreamableHTTPTransportOriginValidation(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		allowedOrigins []string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "no origins configured - should pass",
			endpoint:       "http://localhost:8080/mcp",
			allowedOrigins: nil,
			expectError:    false,
		},
		{
			name:           "empty origins configured - should pass",
			endpoint:       "http://localhost:8080/mcp",
			allowedOrigins: []string{},
			expectError:    false,
		},
		{
			name:           "localhost endpoint allowed",
			endpoint:       "http://localhost:8080/mcp",
			allowedOrigins: []string{"http://localhost"},
			expectError:    false,
		},
		{
			name:           "HTTPS localhost endpoint allowed",
			endpoint:       "https://localhost:8443/mcp",
			allowedOrigins: []string{"https://localhost"},
			expectError:    false,
		},
		{
			name:           "localhost with different port allowed",
			endpoint:       "http://localhost:3000/mcp",
			allowedOrigins: []string{"http://localhost"},
			expectError:    false,
		},
		{
			name:           "127.0.0.1 allowed with localhost config",
			endpoint:       "http://127.0.0.1:8080/mcp",
			allowedOrigins: []string{"http://localhost"},
			expectError:    false,
		},
		{
			name:           "IPv6 localhost allowed",
			endpoint:       "http://[::1]:8080/mcp",
			allowedOrigins: []string{"http://localhost"},
			expectError:    false,
		},
		{
			name:           "wildcard allows everything",
			endpoint:       "https://example.com/mcp",
			allowedOrigins: []string{"*"},
			expectError:    false,
		},
		{
			name:           "specific origin allowed",
			endpoint:       "https://api.example.com/mcp",
			allowedOrigins: []string{"https://api.example.com"},
			expectError:    false,
		},
		{
			name:           "unauthorized origin rejected",
			endpoint:       "https://malicious.com/mcp",
			allowedOrigins: []string{"https://trusted.com"},
			expectError:    true,
			errorContains:  "Server origin not allowed",
		},
		{
			name:           "scheme mismatch rejected",
			endpoint:       "http://example.com:8080/mcp",
			allowedOrigins: []string{"https://example.com"},
			expectError:    true,
			errorContains:  "Server origin not allowed",
		},
		{
			name:           "invalid endpoint format",
			endpoint:       "not-a-url",
			allowedOrigins: []string{"http://localhost"},
			expectError:    true,
			errorContains:  "Invalid endpoint URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultTransportConfig(TransportTypeStreamableHTTP)
			config.Endpoint = tt.endpoint
			config.Features.EnableReliability = false   // Disable for test
			config.Features.EnableObservability = false // Disable for test
			baseTransport, _ := NewTransport(config)

			// Cast to StreamableHTTPTransport to access origin methods
			transport, ok := baseTransport.(*StreamableHTTPTransport)
			if !ok {
				t.Fatal("Expected StreamableHTTPTransport")
			}

			if tt.allowedOrigins != nil {
				transport.SetAllowedOrigins(tt.allowedOrigins)
			}

			err := transport.validateOrigin()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestStreamableHTTPTransportOriginMethods tests the origin configuration methods
func TestStreamableHTTPTransportOriginMethods(t *testing.T) {
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	baseTransport, _ := NewTransport(config)

	// Cast to StreamableHTTPTransport to access origin methods
	transport, ok := baseTransport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}

	// Test SetAllowedOrigins
	origins := []string{"http://localhost", "https://example.com"}
	transport.SetAllowedOrigins(origins)

	transport.mu.Lock()
	if len(transport.allowedOrigins) != len(origins) {
		t.Errorf("SetAllowedOrigins: expected %d origins, got %d", len(origins), len(transport.allowedOrigins))
	}
	for i, expected := range origins {
		if transport.allowedOrigins[i] != expected {
			t.Errorf("SetAllowedOrigins: expected origin %q at index %d, got %q", expected, i, transport.allowedOrigins[i])
		}
	}
	transport.mu.Unlock()

	// Test AddAllowedOrigin
	newOrigin := "https://api.trusted.com"
	transport.AddAllowedOrigin(newOrigin)

	transport.mu.Lock()
	expectedCount := len(origins) + 1
	if len(transport.allowedOrigins) != expectedCount {
		t.Errorf("AddAllowedOrigin: expected %d origins, got %d", expectedCount, len(transport.allowedOrigins))
	}
	if transport.allowedOrigins[len(transport.allowedOrigins)-1] != newOrigin {
		t.Errorf("AddAllowedOrigin: expected last origin to be %q, got %q", newOrigin, transport.allowedOrigins[len(transport.allowedOrigins)-1])
	}
	transport.mu.Unlock()
}

// TestStreamableHTTPTransportIsLocalhostOrigin tests the localhost detection
func TestStreamableHTTPTransportIsLocalhostOrigin(t *testing.T) {
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	baseTransport, _ := NewTransport(config)

	// Cast to StreamableHTTPTransport to access origin methods
	transport, ok := baseTransport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}

	localhostOrigins := []string{
		"http://localhost",
		"https://localhost",
		"http://localhost:3000",
		"https://localhost:8080",
		"http://127.0.0.1",
		"http://127.0.0.1:5000",
		"http://::1",
		"https://::1:4000",
	}

	for _, origin := range localhostOrigins {
		if !transport.isLocalhostOrigin(origin) {
			t.Errorf("isLocalhostOrigin(%q) should return true", origin)
		}
	}

	nonLocalhostOrigins := []string{
		"https://example.com",
		"http://192.168.1.1",
		"https://192.168.1.1:8080",
		"http://10.0.0.1",
	}

	for _, origin := range nonLocalhostOrigins {
		if transport.isLocalhostOrigin(origin) {
			t.Errorf("isLocalhostOrigin(%q) should return false", origin)
		}
	}
}

// TestStreamableHTTPTransportInitializeWithOriginValidation tests that Initialize calls origin validation
func TestStreamableHTTPTransportInitializeWithOriginValidation(t *testing.T) {
	// Test case where origin validation should fail
	config := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config.Endpoint = "https://malicious.com/mcp"
	config.Features.EnableReliability = false   // Disable for test
	config.Features.EnableObservability = false // Disable for test
	baseTransport, _ := NewTransport(config)

	// Cast to StreamableHTTPTransport to access origin methods
	transport, ok := baseTransport.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}
	transport.SetAllowedOrigins([]string{"https://trusted.com"})

	err := transport.Initialize(context.Background())
	if err == nil {
		t.Error("Expected Initialize to fail due to origin validation, but it succeeded")
	}
	if !containsString(err.Error(), "Server origin not allowed") {
		t.Errorf("Expected error about origin not allowed, got: %v", err)
	}

	// Test case where origin validation should pass
	config2 := DefaultTransportConfig(TransportTypeStreamableHTTP)
	config2.Endpoint = "https://trusted.com/mcp"
	config2.Features.EnableReliability = false   // Disable for test
	config2.Features.EnableObservability = false // Disable for test
	baseTransport2, _ := NewTransport(config2)

	// Cast to StreamableHTTPTransport to access origin methods
	transport2, ok := baseTransport2.(*StreamableHTTPTransport)
	if !ok {
		t.Fatal("Expected StreamableHTTPTransport")
	}
	transport2.SetAllowedOrigins([]string{"https://trusted.com"})

	// We expect this to fail at the network level (since there's no actual server),
	// but it should pass origin validation first
	err2 := transport2.Initialize(context.Background())
	// The error should be about network connection, not origin validation
	if err2 != nil && containsString(err2.Error(), "Server origin not allowed") {
		t.Errorf("Origin validation should have passed, got origin error: %v", err2)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findString(s, substr))
}

// Simple string search function
func findString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
