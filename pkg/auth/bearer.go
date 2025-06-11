package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// BearerTokenProvider implements bearer token authentication.
// This provider manages token generation, validation, and revocation
// for bearer token-based authentication schemes.
type BearerTokenProvider struct {
	// Token storage (in production, use Redis or database)
	tokens map[string]*tokenInfo
	mu     sync.RWMutex

	// Configuration
	tokenExpiry      time.Duration
	refreshExpiry    time.Duration
	tokenLength      int
	refreshable      bool
	validateCallback TokenValidationCallback
}

// tokenInfo stores token metadata
type tokenInfo struct {
	Token        string
	RefreshToken string
	UserInfo     *UserInfo
	IssuedAt     time.Time
	ExpiresAt    time.Time
	Revoked      bool
	Scopes       []string
}

// TokenValidationCallback allows custom token validation logic
type TokenValidationCallback func(ctx context.Context, token string) (*UserInfo, error)

// BearerTokenConfig configures the bearer token provider
type BearerTokenConfig struct {
	// TokenExpiry defines access token lifetime (default: 1 hour)
	TokenExpiry time.Duration

	// RefreshExpiry defines refresh token lifetime (default: 7 days)
	RefreshExpiry time.Duration

	// TokenLength in bytes (default: 32)
	TokenLength int

	// Refreshable indicates if refresh tokens should be issued
	Refreshable bool

	// ValidationCallback for external token validation (e.g., JWT verification)
	ValidationCallback TokenValidationCallback
}

// NewBearerTokenProvider creates a new bearer token authentication provider.
func NewBearerTokenProvider(config *BearerTokenConfig) *BearerTokenProvider {
	if config == nil {
		config = &BearerTokenConfig{}
	}

	// Apply defaults
	if config.TokenExpiry == 0 {
		config.TokenExpiry = 1 * time.Hour
	}
	if config.RefreshExpiry == 0 {
		config.RefreshExpiry = 7 * 24 * time.Hour
	}
	if config.TokenLength == 0 {
		config.TokenLength = 32
	}

	return &BearerTokenProvider{
		tokens:           make(map[string]*tokenInfo),
		tokenExpiry:      config.TokenExpiry,
		refreshExpiry:    config.RefreshExpiry,
		tokenLength:      config.TokenLength,
		refreshable:      config.Refreshable,
		validateCallback: config.ValidationCallback,
	}
}

// Type returns the authentication type identifier.
func (p *BearerTokenProvider) Type() string {
	return "bearer"
}

// Authenticate performs bearer token authentication.
// For bearer tokens, this typically validates existing credentials
// and issues new tokens.
func (p *BearerTokenProvider) Authenticate(ctx context.Context, req *AuthRequest) (*AuthResult, error) {
	if req.Type != "bearer" && req.Type != "basic" {
		return nil, NewAuthError(ErrInvalidCredentials, "unsupported authentication type")
	}

	// For basic auth, validate credentials and issue bearer token
	if req.Type == "basic" {
		username, _ := req.Credentials["username"].(string)
		password, _ := req.Credentials["password"].(string)

		// In production, validate against user database
		// This is a placeholder implementation
		if username == "" || password == "" {
			return nil, NewAuthError(ErrInvalidCredentials, "username and password required")
		}

		// Generate tokens
		accessToken, err := p.generateToken()
		if err != nil {
			return nil, err
		}

		var refreshToken string
		if p.refreshable {
			refreshToken, err = p.generateToken()
			if err != nil {
				return nil, err
			}
		}

		// Create user info (in production, fetch from database)
		userInfo := &UserInfo{
			ID:       fmt.Sprintf("user-%s", username),
			Username: username,
			Roles:    []string{"user"}, // Default role
		}

		// Store token info
		now := time.Now()
		info := &tokenInfo{
			Token:        accessToken,
			RefreshToken: refreshToken,
			UserInfo:     userInfo,
			IssuedAt:     now,
			ExpiresAt:    now.Add(p.tokenExpiry),
			Scopes:       req.Scopes,
		}

		p.mu.Lock()
		p.tokens[accessToken] = info
		if refreshToken != "" {
			p.tokens[refreshToken] = info
		}
		p.mu.Unlock()

		return &AuthResult{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    int64(p.tokenExpiry.Seconds()),
			TokenType:    "Bearer",
			Scopes:       req.Scopes,
			UserInfo:     userInfo,
			IssuedAt:     now,
		}, nil
	}

	// For bearer type, validate existing token
	token, _ := req.Credentials["token"].(string)
	userInfo, err := p.Validate(ctx, token)
	if err != nil {
		return nil, err
	}

	// Return existing token info
	p.mu.RLock()
	info := p.tokens[token]
	p.mu.RUnlock()

	if info == nil {
		return nil, NewAuthError(ErrTokenInvalid, "token not found")
	}

	return &AuthResult{
		AccessToken: token,
		ExpiresIn:   int64(time.Until(info.ExpiresAt).Seconds()),
		TokenType:   "Bearer",
		Scopes:      info.Scopes,
		UserInfo:    userInfo,
		IssuedAt:    info.IssuedAt,
	}, nil
}

// Validate verifies a bearer token and returns user information.
func (p *BearerTokenProvider) Validate(ctx context.Context, token string) (*UserInfo, error) {
	// Check custom validation callback first
	if p.validateCallback != nil {
		return p.validateCallback(ctx, token)
	}

	// Check internal token storage
	p.mu.RLock()
	info, exists := p.tokens[token]
	p.mu.RUnlock()

	if !exists {
		return nil, NewAuthError(ErrTokenInvalid, "token not found")
	}

	if info.Revoked {
		return nil, NewAuthError(ErrTokenRevoked, "token has been revoked")
	}

	if time.Now().After(info.ExpiresAt) {
		return nil, NewAuthError(ErrTokenExpired, "token has expired")
	}

	return info.UserInfo, nil
}

// Refresh exchanges a refresh token for new access credentials.
func (p *BearerTokenProvider) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	if !p.refreshable {
		return nil, NewAuthError(ErrInvalidCredentials, "refresh not supported")
	}

	p.mu.RLock()
	info, exists := p.tokens[refreshToken]
	p.mu.RUnlock()

	if !exists || info.RefreshToken != refreshToken {
		return nil, NewAuthError(ErrTokenInvalid, "invalid refresh token")
	}

	if info.Revoked {
		return nil, NewAuthError(ErrTokenRevoked, "refresh token has been revoked")
	}

	// Check refresh token expiry (typically longer than access token)
	refreshExpiresAt := info.IssuedAt.Add(p.refreshExpiry)
	if time.Now().After(refreshExpiresAt) {
		return nil, NewAuthError(ErrTokenExpired, "refresh token has expired")
	}

	// Generate new access token
	newAccessToken, err := p.generateToken()
	if err != nil {
		return nil, err
	}

	// Create new token info
	now := time.Now()
	newInfo := &tokenInfo{
		Token:        newAccessToken,
		RefreshToken: refreshToken, // Keep same refresh token
		UserInfo:     info.UserInfo,
		IssuedAt:     now,
		ExpiresAt:    now.Add(p.tokenExpiry),
		Scopes:       info.Scopes,
	}

	// Store new token
	p.mu.Lock()
	p.tokens[newAccessToken] = newInfo
	// Revoke old access token
	if info.Token != "" {
		if oldInfo, exists := p.tokens[info.Token]; exists {
			oldInfo.Revoked = true
		}
	}
	// Update refresh token to point to new access token
	info.Token = newAccessToken
	p.mu.Unlock()

	return &AuthResult{
		AccessToken:  newAccessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(p.tokenExpiry.Seconds()),
		TokenType:    "Bearer",
		Scopes:       info.Scopes,
		UserInfo:     info.UserInfo,
		IssuedAt:     now,
	}, nil
}

// Revoke invalidates a token.
func (p *BearerTokenProvider) Revoke(ctx context.Context, token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, exists := p.tokens[token]
	if !exists {
		return NewAuthError(ErrTokenInvalid, "token not found")
	}

	info.Revoked = true

	// Also revoke associated tokens
	if info.RefreshToken != "" && info.RefreshToken != token {
		if refreshInfo, exists := p.tokens[info.RefreshToken]; exists {
			refreshInfo.Revoked = true
		}
	}
	if info.Token != "" && info.Token != token {
		if accessInfo, exists := p.tokens[info.Token]; exists {
			accessInfo.Revoked = true
		}
	}

	return nil
}

// generateToken creates a cryptographically secure random token.
func (p *BearerTokenProvider) generateToken() (string, error) {
	bytes := make([]byte, p.tokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", NewAuthError(ErrTokenInvalid, "failed to generate token").
			WithDetail("error", err.Error())
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// CleanupExpired removes expired tokens from storage.
// This should be called periodically in production systems.
func (p *BearerTokenProvider) CleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for token, info := range p.tokens {
		// Remove if expired or revoked
		if info.Revoked || now.After(info.ExpiresAt.Add(p.refreshExpiry)) {
			delete(p.tokens, token)
		}
	}
}
