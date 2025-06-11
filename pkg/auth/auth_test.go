package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/auth"
)

func TestBearerTokenProvider(t *testing.T) {
	// Create provider
	provider := auth.NewBearerTokenProvider(&auth.BearerTokenConfig{
		TokenExpiry:   1 * time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Refreshable:   true,
	})

	// Test authentication
	authReq := &auth.AuthRequest{
		Type: "basic",
		Credentials: map[string]interface{}{
			"username": "testuser",
			"password": "testpass",
		},
		Scopes: []string{"read", "write"},
	}

	result, err := provider.Authenticate(context.Background(), authReq)
	if err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	if result.AccessToken == "" {
		t.Error("Expected access token, got empty string")
	}
	if result.RefreshToken == "" {
		t.Error("Expected refresh token, got empty string")
	}
	if result.TokenType != "Bearer" {
		t.Errorf("Expected token type Bearer, got %s", result.TokenType)
	}
	if result.UserInfo == nil {
		t.Error("Expected user info, got nil")
	}

	// Test token validation
	userInfo, err := provider.Validate(context.Background(), result.AccessToken)
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}
	if userInfo.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", userInfo.Username)
	}

	// Test refresh
	newResult, err := provider.Refresh(context.Background(), result.RefreshToken)
	if err != nil {
		t.Fatalf("Token refresh failed: %v", err)
	}
	if newResult.AccessToken == result.AccessToken {
		t.Error("Expected new access token after refresh")
	}

	// Test revocation
	err = provider.Revoke(context.Background(), result.AccessToken)
	if err != nil {
		t.Fatalf("Token revocation failed: %v", err)
	}

	// Validate revoked token should fail
	_, err = provider.Validate(context.Background(), result.AccessToken)
	if err == nil {
		t.Error("Expected error validating revoked token")
	}
}

func TestAPIKeyProvider(t *testing.T) {
	// Create provider
	provider := auth.NewAPIKeyProvider(&auth.APIKeyConfig{
		KeyPrefix: "test_",
		KeyLength: 16,
	})

	// Create API key
	userInfo := &auth.UserInfo{
		ID:       "user-123",
		Username: "apiuser",
		Roles:    []string{"service"},
	}

	apiKey, err := provider.CreateAPIKey(userInfo, "Test key", nil, []string{"read"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	if !provider.ValidateAPIKeyFormat(apiKey) {
		t.Error("API key format validation failed")
	}

	// Test validation
	validatedUser, err := provider.Validate(context.Background(), apiKey)
	if err != nil {
		t.Fatalf("API key validation failed: %v", err)
	}
	if validatedUser.ID != userInfo.ID {
		t.Errorf("Expected user ID %s, got %s", userInfo.ID, validatedUser.ID)
	}

	// Test authentication
	authReq := &auth.AuthRequest{
		Type: "apikey",
		Credentials: map[string]interface{}{
			"apiKey": apiKey,
		},
	}

	result, err := provider.Authenticate(context.Background(), authReq)
	if err != nil {
		t.Fatalf("API key authentication failed: %v", err)
	}
	if result.AccessToken != apiKey {
		t.Error("Expected access token to be the API key")
	}

	// Test revocation
	err = provider.Revoke(context.Background(), apiKey)
	if err != nil {
		t.Fatalf("API key revocation failed: %v", err)
	}

	// Validate revoked key should fail
	_, err = provider.Validate(context.Background(), apiKey)
	if err == nil {
		t.Error("Expected error validating revoked API key")
	}
}

func TestRBAC(t *testing.T) {
	// Create RBAC provider
	rbac := auth.NewRBACProvider(&auth.RBACConfig{
		DefaultRole: "guest",
		ResourcePermissions: map[string][]string{
			"admin_resource": {"admin"},
			"user_resource":  {"read", "write"},
		},
	})

	userID := "user-123"

	// Test default role
	roles := rbac.GetUserRoles(userID)
	if len(roles) != 1 || roles[0] != "guest" {
		t.Errorf("Expected default role guest, got %v", roles)
	}

	// Assign user role
	err := rbac.AssignRole(userID, "user")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Test permissions
	if !rbac.HasPermission(userID, "read") {
		t.Error("Expected user to have read permission")
	}
	if rbac.HasPermission(userID, "admin") {
		t.Error("Expected user not to have admin permission")
	}

	// Test resource access
	if !rbac.CanAccessResource(userID, "user_resource") {
		t.Error("Expected user to access user_resource")
	}
	if rbac.CanAccessResource(userID, "admin_resource") {
		t.Error("Expected user not to access admin_resource")
	}

	// Assign admin role
	err = rbac.AssignRole(userID, "admin")
	if err != nil {
		t.Fatalf("Failed to assign admin role: %v", err)
	}

	// Admin should have all permissions
	if !rbac.HasPermission(userID, "anything") {
		t.Error("Expected admin to have all permissions")
	}
	if !rbac.CanAccessResource(userID, "admin_resource") {
		t.Error("Expected admin to access admin_resource")
	}
}

func TestAuthContext(t *testing.T) {
	// Test adding token to context
	ctx := context.Background()
	token := "test-token"
	ctx = auth.ContextWithToken(ctx, token)

	// Test adding user info to context
	userInfo := &auth.UserInfo{
		ID:       "user-123",
		Username: "testuser",
	}
	ctx = auth.ContextWithUserInfo(ctx, userInfo)

	// Test extracting user info
	extractedUser, ok := auth.UserInfoFromContext(ctx)
	if !ok {
		t.Error("Failed to extract user info from context")
	}
	if extractedUser.ID != userInfo.ID {
		t.Errorf("Expected user ID %s, got %s", userInfo.ID, extractedUser.ID)
	}
}

func TestAuthErrors(t *testing.T) {
	// Test error creation
	err := auth.NewAuthError(auth.ErrInvalidCredentials, "Invalid username or password")
	if err.Code != auth.ErrInvalidCredentials {
		t.Errorf("Expected error code %s, got %s", auth.ErrInvalidCredentials, err.Code)
	}

	// Test error with details
	err = auth.NewAuthError(auth.ErrTokenExpired, "Token has expired").
		WithDetail("expired_at", "2024-01-01T00:00:00Z").
		WithDetail("token_id", "123")

	if len(err.Details) != 2 {
		t.Errorf("Expected 2 error details, got %d", len(err.Details))
	}

	// Test error message
	if err.Error() != "Token has expired" {
		t.Errorf("Expected error message 'Token has expired', got %s", err.Error())
	}
}
