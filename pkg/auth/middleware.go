package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// AuthMiddleware implements authentication for MCP transports.
// It intercepts requests and validates authentication before forwarding.
type AuthMiddleware struct {
	transport  transport.Transport
	provider   AuthProvider
	config     *AuthConfig
	tokenCache *TokenCache
}

// TokenCache provides in-memory caching for validated tokens
type TokenCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
}

type cacheEntry struct {
	userInfo  *UserInfo
	expiresAt time.Time
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(provider AuthProvider, config *AuthConfig) *AuthMiddleware {
	if config == nil {
		config = &AuthConfig{
			Required:         true,
			TokenExpiry:      1 * time.Hour,
			RefreshThreshold: 5 * time.Minute,
		}
	}

	var cache *TokenCache
	if config.CacheConfig != nil && config.CacheConfig.Enabled {
		cache = &TokenCache{
			entries: make(map[string]*cacheEntry),
			maxSize: config.CacheConfig.MaxSize,
			ttl:     config.CacheConfig.TTL,
		}
		// Start cache cleanup routine
		go cache.cleanup(config.CacheConfig.CleanupInterval)
	}

	return &AuthMiddleware{
		provider:   provider,
		config:     config,
		tokenCache: cache,
	}
}

// Wrap implements the Middleware interface.
func (m *AuthMiddleware) Wrap(transport transport.Transport) transport.Transport {
	m.transport = transport
	return m
}

// SendRequest authenticates the request before forwarding.
func (m *AuthMiddleware) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Extract auth token from context
	token := extractToken(ctx)

	// Validate authentication if required
	if m.config.Required && token == "" && !m.isAnonymousAllowed(method) {
		return nil, NewAuthError(ErrAuthRequired, "authentication required")
	}

	if token != "" {
		// Validate token
		userInfo, err := m.validateToken(ctx, token)
		if err != nil {
			return nil, err
		}

		// Add user info to context
		ctx = ContextWithUserInfo(ctx, userInfo)

		// Check RBAC if enabled
		if m.config.RBAC != nil && m.config.RBAC.Enabled {
			if !m.checkPermission(userInfo, method) {
				return nil, NewAuthError(ErrAccessDenied, "insufficient permissions")
			}
		}
	}

	// Forward the authenticated request
	return m.transport.SendRequest(ctx, method, params)
}

// SendNotification authenticates before sending notification.
func (m *AuthMiddleware) SendNotification(ctx context.Context, method string, params interface{}) error {
	// Extract auth token from context
	token := extractToken(ctx)

	// Validate authentication if required
	if m.config.Required && token == "" && !m.isAnonymousAllowed(method) {
		return NewAuthError(ErrAuthRequired, "authentication required")
	}

	if token != "" {
		// Validate token
		userInfo, err := m.validateToken(ctx, token)
		if err != nil {
			return err
		}

		// Add user info to context
		ctx = ContextWithUserInfo(ctx, userInfo)

		// Check RBAC if enabled
		if m.config.RBAC != nil && m.config.RBAC.Enabled {
			if !m.checkPermission(userInfo, method) {
				return NewAuthError(ErrAccessDenied, "insufficient permissions")
			}
		}
	}

	// Forward the authenticated notification
	return m.transport.SendNotification(ctx, method, params)
}

// HandleRequest validates authentication for incoming requests.
func (m *AuthMiddleware) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	// Extract auth token from request metadata
	token := extractTokenFromRequest(request)

	// Validate authentication if required
	if m.config.Required && token == "" && !m.isAnonymousAllowed(request.Method) {
		return &protocol.Response{
			ID: request.ID,
			Error: &protocol.Error{
				Code:    -32603,
				Message: "authentication required",
			},
		}, nil
	}

	if token != "" {
		// Validate token
		userInfo, err := m.validateToken(ctx, token)
		if err != nil {
			return &protocol.Response{
				ID: request.ID,
				Error: &protocol.Error{
					Code:    -32603,
					Message: err.Error(),
				},
			}, nil
		}

		// Add user info to context
		ctx = ContextWithUserInfo(ctx, userInfo)

		// Check RBAC if enabled
		if m.config.RBAC != nil && m.config.RBAC.Enabled {
			if !m.checkPermission(userInfo, request.Method) {
				return &protocol.Response{
					ID: request.ID,
					Error: &protocol.Error{
						Code:    -32603,
						Message: "insufficient permissions",
					},
				}, nil
			}
		}
	}

	// Forward the authenticated request
	return m.transport.HandleRequest(ctx, request)
}

// validateToken validates a token using cache if available.
func (m *AuthMiddleware) validateToken(ctx context.Context, token string) (*UserInfo, error) {
	// Check cache first
	if m.tokenCache != nil {
		if userInfo := m.tokenCache.get(token); userInfo != nil {
			return userInfo, nil
		}
	}

	// Validate with provider
	userInfo, err := m.provider.Validate(ctx, token)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if m.tokenCache != nil {
		m.tokenCache.set(token, userInfo)
	}

	return userInfo, nil
}

// isAnonymousAllowed checks if a method allows anonymous access.
func (m *AuthMiddleware) isAnonymousAllowed(method string) bool {
	if !m.config.AllowAnonymous {
		return false
	}

	// List of methods that allow anonymous access
	// This should be configurable in production
	anonymousMethods := []string{
		"initialize",
		"ping",
		"health",
	}

	for _, allowed := range anonymousMethods {
		if method == allowed {
			return true
		}
	}

	return false
}

// checkPermission validates user permissions for a method.
func (m *AuthMiddleware) checkPermission(userInfo *UserInfo, method string) bool {
	if m.config.RBAC == nil || !m.config.RBAC.Enabled {
		return true
	}

	// Get required permissions for the method
	requiredPerms, ok := m.config.RBAC.ResourcePermissions[method]
	if !ok {
		// No specific permissions required
		return true
	}

	// Check if user has any of the required permissions
	for _, required := range requiredPerms {
		for _, userPerm := range userInfo.Permissions {
			if userPerm == required {
				return true
			}
		}
	}

	// Check role-based permissions
	for _, role := range userInfo.Roles {
		if m.hasRolePermission(role, requiredPerms) {
			return true
		}
	}

	return false
}

// hasRolePermission checks if a role has required permissions.
func (m *AuthMiddleware) hasRolePermission(role string, requiredPerms []string) bool {
	// This is a simplified implementation
	// In production, this would check against a role-permission mapping
	rolePermissions := map[string][]string{
		"admin": {"*"}, // Admin has all permissions
		"user":  {"read", "list"},
		"guest": {"read"},
	}

	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}

	// Check wildcard
	for _, perm := range perms {
		if perm == "*" {
			return true
		}
	}

	// Check specific permissions
	for _, required := range requiredPerms {
		for _, perm := range perms {
			if perm == required {
				return true
			}
		}
	}

	return false
}

// Token cache methods

func (c *TokenCache) get(token string) *UserInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[token]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.userInfo
}

func (c *TokenCache) set(token string, userInfo *UserInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entry if at capacity
	if len(c.entries) >= c.maxSize {
		// Simple eviction - in production use LRU
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[token] = &cacheEntry{
		userInfo:  userInfo,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *TokenCache) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for token, entry := range c.entries {
			if now.After(entry.expiresAt) {
				delete(c.entries, token)
			}
		}
		c.mu.Unlock()
	}
}

// Context helpers

type contextKey string

const (
	contextKeyAuth     contextKey = "mcp_auth_token"
	contextKeyUserInfo contextKey = "mcp_user_info"
)

// ContextWithToken adds an auth token to the context.
func ContextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, contextKeyAuth, token)
}

// ContextWithUserInfo adds user info to the context.
func ContextWithUserInfo(ctx context.Context, userInfo *UserInfo) context.Context {
	return context.WithValue(ctx, contextKeyUserInfo, userInfo)
}

// UserInfoFromContext extracts user info from context.
func UserInfoFromContext(ctx context.Context) (*UserInfo, bool) {
	userInfo, ok := ctx.Value(contextKeyUserInfo).(*UserInfo)
	return userInfo, ok
}

// extractToken extracts auth token from context.
func extractToken(ctx context.Context) string {
	token, _ := ctx.Value(contextKeyAuth).(string)
	return token
}

// extractTokenFromRequest extracts auth token from request metadata.
func extractTokenFromRequest(request *protocol.Request) string {
	// Check for auth in params (if params is a map)
	if request.Params != nil {
		// Try to unmarshal params to check for auth
		var params map[string]interface{}
		if err := json.Unmarshal(request.Params, &params); err == nil {
			// Check for authorization header
			if auth, ok := params["_authorization"].(string); ok {
				// Extract bearer token
				if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
					return strings.TrimSpace(auth[7:])
				}
				// API key format
				if strings.HasPrefix(strings.ToLower(auth), "apikey ") {
					return strings.TrimSpace(auth[7:])
				}
				// Just the token
				return auth
			}
			// Check for token directly
			if token, ok := params["_token"].(string); ok {
				return token
			}
		}
	}

	return ""
}

// Delegate all other Transport methods to the wrapped transport

func (m *AuthMiddleware) Initialize(ctx context.Context) error {
	return m.transport.Initialize(ctx)
}

func (m *AuthMiddleware) Start(ctx context.Context) error {
	return m.transport.Start(ctx)
}

func (m *AuthMiddleware) Stop(ctx context.Context) error {
	return m.transport.Stop(ctx)
}

func (m *AuthMiddleware) RegisterRequestHandler(method string, handler transport.RequestHandler) {
	m.transport.RegisterRequestHandler(method, handler)
}

func (m *AuthMiddleware) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	m.transport.RegisterNotificationHandler(method, handler)
}

func (m *AuthMiddleware) RegisterProgressHandler(id interface{}, handler transport.ProgressHandler) {
	m.transport.RegisterProgressHandler(id, handler)
}

func (m *AuthMiddleware) UnregisterProgressHandler(id interface{}) {
	m.transport.UnregisterProgressHandler(id)
}

func (m *AuthMiddleware) HandleResponse(response *protocol.Response) {
	m.transport.HandleResponse(response)
}

func (m *AuthMiddleware) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	// For incoming notifications, we might want to validate auth as well
	// For now, just forward
	return m.transport.HandleNotification(ctx, notification)
}

func (m *AuthMiddleware) GenerateID() string {
	return m.transport.GenerateID()
}

func (m *AuthMiddleware) GetRequestIDPrefix() string {
	return m.transport.GetRequestIDPrefix()
}

func (m *AuthMiddleware) GetNextID() int64 {
	return m.transport.GetNextID()
}

func (m *AuthMiddleware) Cleanup() {
	m.transport.Cleanup()
}

func (m *AuthMiddleware) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	// Extract auth token from context
	token := extractToken(ctx)

	// Validate authentication if required
	if m.config.Required && token == "" {
		return nil, NewAuthError(ErrAuthRequired, "authentication required")
	}

	if token != "" {
		// Validate token
		userInfo, err := m.validateToken(ctx, token)
		if err != nil {
			return nil, err
		}

		// Add user info to context
		ctx = ContextWithUserInfo(ctx, userInfo)

		// For batch requests, we should check permissions for each method
		// This is a simplified implementation
		if m.config.RBAC != nil && m.config.RBAC.Enabled {
			for _, item := range *batch {
				if req, ok := item.(*protocol.Request); ok {
					if !m.checkPermission(userInfo, req.Method) {
						return nil, NewAuthError(ErrAccessDenied,
							fmt.Sprintf("insufficient permissions for method %s", req.Method))
					}
				} else if notif, ok := item.(*protocol.Notification); ok {
					if !m.checkPermission(userInfo, notif.Method) {
						return nil, NewAuthError(ErrAccessDenied,
							fmt.Sprintf("insufficient permissions for notification %s", notif.Method))
					}
				}
			}
		}
	}

	// Forward the authenticated batch
	return m.transport.SendBatchRequest(ctx, batch)
}

func (m *AuthMiddleware) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	// For incoming batch requests, authentication should be handled per-request
	// This is delegated to HandleRequest which is called for each item
	return m.transport.HandleBatchRequest(ctx, batch)
}
