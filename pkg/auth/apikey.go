package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// APIKeyProvider implements API key authentication.
// API keys are long-lived credentials typically used for service-to-service
// authentication or programmatic access.
type APIKeyProvider struct {
	// Key storage (in production, use database)
	keys map[string]*apiKeyInfo
	mu   sync.RWMutex

	// Configuration
	keyPrefix        string
	keyLength        int
	validateCallback APIKeyValidationCallback
}

// apiKeyInfo stores API key metadata
type apiKeyInfo struct {
	Key         string
	UserInfo    *UserInfo
	CreatedAt   time.Time
	LastUsedAt  time.Time
	ExpiresAt   *time.Time
	Revoked     bool
	Scopes      []string
	RateLimits  *RateLimitConfig
	Description string
}

// APIKeyValidationCallback allows custom API key validation
type APIKeyValidationCallback func(ctx context.Context, apiKey string) (*UserInfo, error)

// RateLimitConfig defines rate limiting for API keys
type RateLimitConfig struct {
	RequestsPerMinute int
	RequestsPerHour   int
	RequestsPerDay    int
}

// APIKeyConfig configures the API key provider
type APIKeyConfig struct {
	// KeyPrefix added to all generated keys (e.g., "mcp_")
	KeyPrefix string

	// KeyLength in bytes (default: 32)
	KeyLength int

	// ValidationCallback for external key validation
	ValidationCallback APIKeyValidationCallback
}

// NewAPIKeyProvider creates a new API key authentication provider.
func NewAPIKeyProvider(config *APIKeyConfig) *APIKeyProvider {
	if config == nil {
		config = &APIKeyConfig{}
	}

	// Apply defaults
	if config.KeyPrefix == "" {
		config.KeyPrefix = "mcp_"
	}
	if config.KeyLength == 0 {
		config.KeyLength = 32
	}

	return &APIKeyProvider{
		keys:             make(map[string]*apiKeyInfo),
		keyPrefix:        config.KeyPrefix,
		keyLength:        config.KeyLength,
		validateCallback: config.ValidationCallback,
	}
}

// Type returns the authentication type identifier.
func (p *APIKeyProvider) Type() string {
	return "apikey"
}

// Authenticate validates API key credentials.
func (p *APIKeyProvider) Authenticate(ctx context.Context, req *AuthRequest) (*AuthResult, error) {
	if req.Type != "apikey" {
		return nil, NewAuthError(ErrInvalidCredentials, "unsupported authentication type")
	}

	// Extract API key from credentials
	apiKey, ok := req.Credentials["apiKey"].(string)
	if !ok || apiKey == "" {
		// Also check "key" field for compatibility
		apiKey, ok = req.Credentials["key"].(string)
		if !ok || apiKey == "" {
			return nil, NewAuthError(ErrInvalidCredentials, "API key required")
		}
	}

	// Validate the API key
	userInfo, err := p.Validate(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	// Update last used timestamp
	p.mu.Lock()
	if info, exists := p.keys[apiKey]; exists {
		info.LastUsedAt = time.Now()
	}
	p.mu.Unlock()

	// API keys don't expire like bearer tokens, but we return a large expiry
	return &AuthResult{
		AccessToken: apiKey,
		TokenType:   "APIKey",
		ExpiresIn:   int64(365 * 24 * time.Hour / time.Second), // 1 year
		UserInfo:    userInfo,
		IssuedAt:    time.Now(),
		Scopes:      req.Scopes,
	}, nil
}

// Validate verifies an API key and returns user information.
func (p *APIKeyProvider) Validate(ctx context.Context, apiKey string) (*UserInfo, error) {
	// Check custom validation callback first
	if p.validateCallback != nil {
		return p.validateCallback(ctx, apiKey)
	}

	// Normalize key (trim whitespace)
	apiKey = strings.TrimSpace(apiKey)

	// Check prefix if required
	if p.keyPrefix != "" && !strings.HasPrefix(apiKey, p.keyPrefix) {
		return nil, NewAuthError(ErrTokenInvalid, "invalid API key format")
	}

	// Look up key
	p.mu.RLock()
	info, exists := p.keys[apiKey]
	p.mu.RUnlock()

	if !exists {
		return nil, NewAuthError(ErrTokenInvalid, "API key not found")
	}

	if info.Revoked {
		return nil, NewAuthError(ErrTokenRevoked, "API key has been revoked")
	}

	// Check expiration if set
	if info.ExpiresAt != nil && time.Now().After(*info.ExpiresAt) {
		return nil, NewAuthError(ErrTokenExpired, "API key has expired")
	}

	return info.UserInfo, nil
}

// Refresh is not supported for API keys.
func (p *APIKeyProvider) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	return nil, NewAuthError(ErrInvalidCredentials, "API keys do not support refresh")
}

// Revoke invalidates an API key.
func (p *APIKeyProvider) Revoke(ctx context.Context, apiKey string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, exists := p.keys[apiKey]
	if !exists {
		return NewAuthError(ErrTokenInvalid, "API key not found")
	}

	info.Revoked = true
	return nil
}

// CreateAPIKey generates a new API key for a user.
func (p *APIKeyProvider) CreateAPIKey(userInfo *UserInfo, description string, expiresAt *time.Time, scopes []string) (string, error) {
	// Generate random key
	bytes := make([]byte, p.keyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", NewAuthError(ErrTokenInvalid, "failed to generate API key").
			WithDetail("error", err.Error())
	}

	// Create key with prefix
	apiKey := p.keyPrefix + hex.EncodeToString(bytes)

	// Store key info
	now := time.Now()
	info := &apiKeyInfo{
		Key:         apiKey,
		UserInfo:    userInfo,
		CreatedAt:   now,
		LastUsedAt:  now,
		ExpiresAt:   expiresAt,
		Scopes:      scopes,
		Description: description,
	}

	p.mu.Lock()
	p.keys[apiKey] = info
	p.mu.Unlock()

	return apiKey, nil
}

// ListAPIKeys returns all API keys for a user.
func (p *APIKeyProvider) ListAPIKeys(userID string) ([]*APIKeyInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var keys []*APIKeyInfo
	for _, info := range p.keys {
		if info.UserInfo.ID == userID && !info.Revoked {
			keys = append(keys, &APIKeyInfo{
				KeyPrefix:   p.keyPrefix + info.Key[:8] + "...", // Show partial key
				Description: info.Description,
				CreatedAt:   info.CreatedAt,
				LastUsedAt:  info.LastUsedAt,
				ExpiresAt:   info.ExpiresAt,
				Scopes:      info.Scopes,
			})
		}
	}

	return keys, nil
}

// APIKeyInfo represents public API key information (without the actual key).
type APIKeyInfo struct {
	KeyPrefix   string     // Partial key for identification
	Description string     // User-provided description
	CreatedAt   time.Time  // Creation timestamp
	LastUsedAt  time.Time  // Last usage timestamp
	ExpiresAt   *time.Time // Optional expiration
	Scopes      []string   // Granted scopes
}

// ValidateAPIKeyFormat checks if a string matches the expected API key format.
func (p *APIKeyProvider) ValidateAPIKeyFormat(apiKey string) bool {
	if p.keyPrefix != "" && !strings.HasPrefix(apiKey, p.keyPrefix) {
		return false
	}

	// Check length (prefix + hex encoded bytes)
	expectedLength := len(p.keyPrefix) + (p.keyLength * 2)
	return len(apiKey) == expectedLength
}

// CompareAPIKeys performs constant-time comparison of API keys.
func CompareAPIKeys(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// CleanupExpired removes expired API keys from storage.
func (p *APIKeyProvider) CleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, info := range p.keys {
		if info.Revoked || (info.ExpiresAt != nil && now.After(*info.ExpiresAt)) {
			delete(p.keys, key)
		}
	}
}

// GetRateLimits returns rate limit configuration for an API key.
func (p *APIKeyProvider) GetRateLimits(apiKey string) (*RateLimitConfig, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info, exists := p.keys[apiKey]
	if !exists {
		return nil, NewAuthError(ErrTokenInvalid, "API key not found")
	}

	if info.RateLimits == nil {
		// Return default limits
		return &RateLimitConfig{
			RequestsPerMinute: 60,
			RequestsPerHour:   1000,
			RequestsPerDay:    10000,
		}, nil
	}

	return info.RateLimits, nil
}

// SetRateLimits updates rate limit configuration for an API key.
func (p *APIKeyProvider) SetRateLimits(apiKey string, limits *RateLimitConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, exists := p.keys[apiKey]
	if !exists {
		return NewAuthError(ErrTokenInvalid, "API key not found")
	}

	info.RateLimits = limits
	return nil
}
