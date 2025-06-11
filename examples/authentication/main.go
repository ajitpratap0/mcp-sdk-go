package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/auth"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// This example demonstrates how to use authentication with MCP transports

func main() {
	fmt.Println("=== MCP Authentication Example ===\n")

	// Demonstrate different authentication scenarios
	fmt.Println("1. Bearer Token Authentication")
	demoBearerTokenAuth()

	fmt.Println("\n2. API Key Authentication")
	demoAPIKeyAuth()

	fmt.Println("\n3. Server with Authentication")
	demoServerWithAuth()
}

func demoBearerTokenAuth() {
	// Create a bearer token provider for testing
	bearerProvider := auth.NewBearerTokenProvider(&auth.BearerTokenConfig{
		TokenExpiry:   10 * time.Minute,
		RefreshExpiry: 1 * time.Hour,
		Refreshable:   true,
	})

	// Simulate user authentication
	authReq := &auth.AuthRequest{
		Type: "basic",
		Credentials: map[string]interface{}{
			"username": "testuser",
			"password": "testpass",
		},
		Scopes: []string{"read", "write"},
	}

	authResult, err := bearerProvider.Authenticate(context.Background(), authReq)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	fmt.Printf("Authentication successful!\n")
	fmt.Printf("- Access Token: %s...\n", authResult.AccessToken[:20])
	fmt.Printf("- Token Type: %s\n", authResult.TokenType)
	fmt.Printf("- Expires In: %d seconds\n", authResult.ExpiresIn)
	fmt.Printf("- User ID: %s\n", authResult.UserInfo.ID)

	// Create transport with authentication
	config := transport.DefaultTransportConfig(transport.TransportTypeStdio)
	config.Features.EnableAuthentication = true
	config.Security.Authentication = &transport.AuthenticationConfig{
		Type:        "bearer",
		Required:    true,
		TokenExpiry: 10 * time.Minute,
		EnableCache: true,
		CacheTTL:    5 * time.Minute,
	}

	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create client with auth token in context
	ctx := auth.ContextWithToken(context.Background(), authResult.AccessToken)
	c := client.New(t)

	fmt.Printf("\nClient created with bearer token authentication\n")

	// Simulate token validation
	userInfo, err := bearerProvider.Validate(ctx, authResult.AccessToken)
	if err != nil {
		log.Fatalf("Token validation failed: %v", err)
	}
	fmt.Printf("Token validated successfully for user: %s\n", userInfo.Username)

	// Demonstrate token refresh
	if authResult.RefreshToken != "" {
		fmt.Printf("\nRefreshing token...\n")
		newAuth, err := bearerProvider.Refresh(ctx, authResult.RefreshToken)
		if err != nil {
			log.Fatalf("Token refresh failed: %v", err)
		}
		fmt.Printf("Token refreshed successfully!\n")
		fmt.Printf("- New Access Token: %s...\n", newAuth.AccessToken[:20])
	}

	_ = c // Avoid unused variable error
}

func demoAPIKeyAuth() {
	// Create an API key provider
	apiKeyProvider := auth.NewAPIKeyProvider(&auth.APIKeyConfig{
		KeyPrefix: "mcp_",
		KeyLength: 32,
	})

	// Create an API key for a user
	userInfo := &auth.UserInfo{
		ID:          "api-user-123",
		Username:    "apiuser",
		Roles:       []string{"service"},
		Permissions: []string{"read", "write"},
	}

	apiKey, err := apiKeyProvider.CreateAPIKey(
		userInfo,
		"Production API Key",
		nil, // No expiration
		[]string{"read", "write"},
	)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	fmt.Printf("API Key created: %s...\n", apiKey[:20])

	// Create transport with API key authentication
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"
	config.Features.EnableAuthentication = true
	config.Security.Authentication = &transport.AuthenticationConfig{
		Type:     "apikey",
		Required: true,
		ProviderConfig: map[string]interface{}{
			"keyPrefix": "mcp_",
		},
	}

	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create client with API key in context
	ctx := auth.ContextWithToken(context.Background(), apiKey)
	c := client.New(t)

	fmt.Printf("Client created with API key authentication\n")

	// Validate API key
	validatedUser, err := apiKeyProvider.Validate(ctx, apiKey)
	if err != nil {
		log.Fatalf("API key validation failed: %v", err)
	}
	fmt.Printf("API key validated for user: %s\n", validatedUser.Username)

	_ = c // Avoid unused variable error
}

func demoServerWithAuth() {
	// Create a custom auth provider that validates against a simple user store
	// Note: In a real implementation, you would configure this in the transport
	// For demo purposes, we'll just show the provider structure
	_ = &customAuthProvider{
		users: map[string]*auth.UserInfo{
			"admin-token": {
				ID:          "admin-1",
				Username:    "admin",
				Roles:       []string{"admin"},
				Permissions: []string{"*"},
			},
			"user-token": {
				ID:          "user-1",
				Username:    "user",
				Roles:       []string{"user"},
				Permissions: []string{"read", "list"},
			},
		},
	}

	// Create server transport with authentication
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = "http://localhost:8080/mcp"
	config.Features.EnableAuthentication = true
	config.Security.Authentication = &transport.AuthenticationConfig{
		Type:           "custom",
		Required:       true,
		AllowAnonymous: false,
		EnableCache:    true,
		CacheTTL:       5 * time.Minute,
	}

	// For this example, we'll use stdio transport instead
	config.Type = transport.TransportTypeStdio

	t, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create server with auth-aware tools provider
	s := server.New(t,
		server.WithName("auth-demo-server"),
		server.WithVersion("1.0.0"),
		server.WithToolsProvider(&authAwareToolsProvider{}),
	)

	fmt.Printf("Server created with authentication enabled\n")
	fmt.Printf("- Authentication required: true\n")
	fmt.Printf("- Anonymous access: false\n")
	fmt.Printf("- Token cache enabled: true\n")

	// The server would validate tokens on incoming requests
	// and enforce permissions based on user roles

	_ = s // Avoid unused variable error
}

// Custom auth provider for demonstration
type customAuthProvider struct {
	users map[string]*auth.UserInfo
}

func (p *customAuthProvider) Type() string {
	return "custom"
}

func (p *customAuthProvider) Authenticate(ctx context.Context, req *auth.AuthRequest) (*auth.AuthResult, error) {
	// Simple token-based auth
	token, _ := req.Credentials["token"].(string)
	userInfo, ok := p.users[token]
	if !ok {
		return nil, auth.NewAuthError(auth.ErrInvalidCredentials, "invalid token")
	}

	return &auth.AuthResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		UserInfo:    userInfo,
		IssuedAt:    time.Now(),
	}, nil
}

func (p *customAuthProvider) Validate(ctx context.Context, token string) (*auth.UserInfo, error) {
	userInfo, ok := p.users[token]
	if !ok {
		return nil, auth.NewAuthError(auth.ErrTokenInvalid, "invalid token")
	}
	return userInfo, nil
}

func (p *customAuthProvider) Refresh(ctx context.Context, refreshToken string) (*auth.AuthResult, error) {
	return nil, auth.NewAuthError(auth.ErrInvalidCredentials, "refresh not supported")
}

func (p *customAuthProvider) Revoke(ctx context.Context, token string) error {
	delete(p.users, token)
	return nil
}

// Auth-aware tools provider that checks permissions
type authAwareToolsProvider struct{}

func (p *authAwareToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	// Check if user is authenticated
	userInfo, ok := auth.UserInfoFromContext(ctx)
	if !ok {
		return nil, 0, "", false, fmt.Errorf("authentication required")
	}

	// Filter tools based on user permissions
	allTools := []protocol.Tool{
		{
			Name:        "read_data",
			Description: "Read data (requires 'read' permission)",
		},
		{
			Name:        "write_data",
			Description: "Write data (requires 'write' permission)",
		},
		{
			Name:        "admin_tool",
			Description: "Admin tool (requires 'admin' role)",
		},
	}

	// Filter based on permissions
	var filteredTools []protocol.Tool
	for _, tool := range allTools {
		if p.canAccessTool(userInfo, tool.Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	return filteredTools, len(filteredTools), "", false, nil
}

func (p *authAwareToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
	// Check authentication
	userInfo, ok := auth.UserInfoFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("authentication required")
	}

	// Check permissions
	if !p.canAccessTool(userInfo, name) {
		return nil, fmt.Errorf("permission denied for tool: %s", name)
	}

	// Execute tool
	result := map[string]interface{}{
		"status": "success",
		"user":   userInfo.Username,
		"tool":   name,
	}

	resultJSON, _ := json.Marshal(result)
	return &protocol.CallToolResult{
		Result: resultJSON,
	}, nil
}

func (p *authAwareToolsProvider) canAccessTool(userInfo *auth.UserInfo, toolName string) bool {
	// Check admin role
	for _, role := range userInfo.Roles {
		if role == "admin" {
			return true
		}
	}

	// Check specific permissions
	switch toolName {
	case "read_data":
		return hasPermission(userInfo, "read")
	case "write_data":
		return hasPermission(userInfo, "write")
	case "admin_tool":
		return false // Only admins
	}

	return false
}

func hasPermission(userInfo *auth.UserInfo, perm string) bool {
	for _, p := range userInfo.Permissions {
		if p == perm || p == "*" {
			return true
		}
	}
	return false
}
