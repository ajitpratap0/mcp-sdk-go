// Package auth provides authentication and authorization functionality for the MCP SDK.
// It supports pluggable authentication providers and integrates with the transport layer
// through middleware for securing MCP communications.
package auth

import (
	"context"
	"time"
)

// AuthProvider defines the interface for authentication providers.
// Implementations can provide various authentication mechanisms such as
// bearer tokens, API keys, OAuth2, OIDC, etc.
type AuthProvider interface {
	// Authenticate performs initial authentication with the provided credentials.
	// Returns an AuthResult containing tokens and user information on success.
	Authenticate(ctx context.Context, req *AuthRequest) (*AuthResult, error)

	// Validate verifies a token's validity and returns associated user information.
	// This is typically used for request-time validation.
	Validate(ctx context.Context, token string) (*UserInfo, error)

	// Refresh exchanges a refresh token for new access credentials.
	// Returns updated tokens and expiration information.
	Refresh(ctx context.Context, refreshToken string) (*AuthResult, error)

	// Revoke invalidates the specified token, preventing further use.
	// This is used for logout or security incident response.
	Revoke(ctx context.Context, token string) error

	// Type returns the authentication type identifier (e.g., "bearer", "apikey", "oauth2").
	Type() string
}

// AuthRequest represents an authentication request containing credentials.
type AuthRequest struct {
	// Type specifies the authentication type (e.g., "bearer", "apikey", "basic")
	Type string `json:"type"`

	// Credentials contains the authentication credentials.
	// The structure depends on the authentication type:
	// - For basic auth: {"username": "...", "password": "..."}
	// - For API key: {"apiKey": "..."}
	// - For OAuth2: {"clientId": "...", "clientSecret": "..."}
	Credentials map[string]interface{} `json:"credentials"`

	// Metadata contains additional request context (e.g., IP address, user agent)
	Metadata map[string]string `json:"metadata,omitempty"`

	// Scopes requested for the authentication session
	Scopes []string `json:"scopes,omitempty"`
}

// AuthResult represents the result of a successful authentication.
type AuthResult struct {
	// AccessToken is the token to be used for authenticated requests
	AccessToken string `json:"accessToken"`

	// RefreshToken is used to obtain new access tokens (optional)
	RefreshToken string `json:"refreshToken,omitempty"`

	// ExpiresIn indicates the access token lifetime in seconds
	ExpiresIn int64 `json:"expiresIn"`

	// TokenType describes the token type (e.g., "Bearer")
	TokenType string `json:"tokenType"`

	// Scopes granted for this authentication session
	Scopes []string `json:"scopes,omitempty"`

	// UserInfo contains information about the authenticated user
	UserInfo *UserInfo `json:"userInfo"`

	// IssuedAt timestamp for token issuance
	IssuedAt time.Time `json:"issuedAt"`
}

// UserInfo represents authenticated user information.
type UserInfo struct {
	// ID is the unique user identifier
	ID string `json:"id"`

	// Username is the user's login name
	Username string `json:"username"`

	// Email address (optional)
	Email string `json:"email,omitempty"`

	// Roles assigned to the user for RBAC
	Roles []string `json:"roles,omitempty"`

	// Permissions granted to the user
	Permissions []string `json:"permissions,omitempty"`

	// Attributes contains additional user metadata
	Attributes map[string]interface{} `json:"attributes,omitempty"`

	// ExpiresAt indicates when the user session expires
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// AuthConfig configures authentication for the MCP SDK.
type AuthConfig struct {
	// Provider is the authentication provider implementation
	Provider AuthProvider

	// Required indicates if authentication is mandatory for all requests
	Required bool

	// AllowAnonymous permits unauthenticated access for specific operations
	AllowAnonymous bool

	// TokenExpiry defines the default token lifetime
	TokenExpiry time.Duration

	// RefreshThreshold indicates when to attempt token refresh (e.g., 5 minutes before expiry)
	RefreshThreshold time.Duration

	// Cache configuration for validated tokens
	CacheConfig *TokenCacheConfig

	// RBAC configuration
	RBAC *RBACConfig
}

// TokenCacheConfig configures token validation caching to improve performance.
type TokenCacheConfig struct {
	// Enabled indicates if token caching is active
	Enabled bool

	// MaxSize is the maximum number of tokens to cache
	MaxSize int

	// TTL is the cache entry lifetime
	TTL time.Duration

	// CleanupInterval for expired entries
	CleanupInterval time.Duration
}

// RBACConfig configures role-based access control.
type RBACConfig struct {
	// Enabled indicates if RBAC is active
	Enabled bool

	// DefaultRole assigned to authenticated users without explicit roles
	DefaultRole string

	// RoleHierarchy defines role inheritance relationships
	RoleHierarchy map[string][]string

	// ResourcePermissions maps resources to required permissions
	ResourcePermissions map[string][]string
}

// AuthError represents authentication and authorization errors.
type AuthError struct {
	// Code is the error code (e.g., "invalid_credentials", "token_expired")
	Code string

	// Message provides human-readable error details
	Message string

	// Details contains additional error context
	Details map[string]interface{}
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	return e.Message
}

// Common authentication error codes
const (
	ErrInvalidCredentials = "invalid_credentials"
	ErrTokenExpired       = "token_expired"
	ErrTokenInvalid       = "token_invalid"
	ErrTokenRevoked       = "token_revoked"
	ErrInsufficientScope  = "insufficient_scope"
	ErrAccessDenied       = "access_denied"
	ErrAuthRequired       = "authentication_required"
)

// NewAuthError creates a new authentication error.
func NewAuthError(code, message string) *AuthError {
	return &AuthError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the authentication error.
func (e *AuthError) WithDetail(key string, value interface{}) *AuthError {
	e.Details[key] = value
	return e
}
