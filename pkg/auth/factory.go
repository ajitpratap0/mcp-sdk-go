package auth

import (
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// init registers the auth middleware factory with the transport package
func init() {
	transport.RegisterAuthMiddlewareFactory(CreateAuthMiddleware)
	transport.RegisterRateLimitMiddlewareFactory(CreateRateLimitMiddleware)
}

// CreateAuthMiddleware creates authentication middleware from transport config
func CreateAuthMiddleware(config *transport.AuthenticationConfig) transport.Middleware {
	if config == nil {
		return nil
	}

	// Create auth provider based on type
	var provider AuthProvider

	switch config.Type {
	case "bearer":
		bearerConfig := &BearerTokenConfig{
			TokenExpiry:   config.TokenExpiry,
			RefreshExpiry: config.TokenExpiry * 7, // 7x longer for refresh tokens
			Refreshable:   true,
		}
		// Extract additional config if provided
		if tokenLength, ok := config.ProviderConfig["tokenLength"].(int); ok {
			bearerConfig.TokenLength = tokenLength
		}
		provider = NewBearerTokenProvider(bearerConfig)

	case "apikey":
		apiKeyConfig := &APIKeyConfig{}
		// Extract additional config if provided
		if prefix, ok := config.ProviderConfig["keyPrefix"].(string); ok {
			apiKeyConfig.KeyPrefix = prefix
		}
		if keyLength, ok := config.ProviderConfig["keyLength"].(int); ok {
			apiKeyConfig.KeyLength = keyLength
		}
		provider = NewAPIKeyProvider(apiKeyConfig)

	default:
		// Unsupported auth type, return nil
		return nil
	}

	// Create auth config
	authConfig := &AuthConfig{
		Provider:         provider,
		Required:         config.Required,
		AllowAnonymous:   config.AllowAnonymous,
		TokenExpiry:      config.TokenExpiry,
		RefreshThreshold: config.RefreshThreshold,
	}

	// Configure cache if enabled
	if config.EnableCache {
		authConfig.CacheConfig = &TokenCacheConfig{
			Enabled:         true,
			MaxSize:         1000, // Default
			TTL:             config.CacheTTL,
			CleanupInterval: config.CacheTTL / 2,
		}

		// Apply defaults if not specified
		if authConfig.CacheConfig.TTL == 0 {
			authConfig.CacheConfig.TTL = 5 * time.Minute
		}
		if authConfig.CacheConfig.CleanupInterval == 0 {
			authConfig.CacheConfig.CleanupInterval = authConfig.CacheConfig.TTL / 2
		}
	}

	return NewAuthMiddleware(provider, authConfig)
}

// CreateRateLimitMiddleware creates rate limit middleware from transport config
func CreateRateLimitMiddleware(config *transport.RateLimitConfig) transport.Middleware {
	if config == nil {
		return nil
	}

	return NewRateLimitMiddleware(config)
}
