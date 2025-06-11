package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// RateLimitMiddleware implements rate limiting for MCP transports
type RateLimitMiddleware struct {
	transport transport.Transport
	limiter   *RateLimiter
	config    *transport.RateLimitConfig
}

// RateLimiter tracks request rates per key
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	config  *transport.RateLimitConfig
}

// tokenBucket implements token bucket algorithm
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(config *transport.RateLimitConfig) *RateLimitMiddleware {
	if config == nil {
		config = &transport.RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 60,
			BurstSize:         10,
		}
	}

	return &RateLimitMiddleware{
		limiter: &RateLimiter{
			buckets: make(map[string]*tokenBucket),
			config:  config,
		},
		config: config,
	}
}

// Wrap implements the Middleware interface
func (m *RateLimitMiddleware) Wrap(transport transport.Transport) transport.Transport {
	m.transport = transport
	// Start cleanup routine
	go m.limiter.cleanup()
	return m
}

// SendRequest applies rate limiting before forwarding request
func (m *RateLimitMiddleware) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if !m.config.Enabled {
		return m.transport.SendRequest(ctx, method, params)
	}

	// Get rate limit key from context
	key := m.getRateLimitKey(ctx)

	// Check rate limit
	allowed, err := m.limiter.Allow(key)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, NewAuthError(ErrAccessDenied, "rate limit exceeded")
	}

	return m.transport.SendRequest(ctx, method, params)
}

// getRateLimitKey extracts the rate limit key from context
func (m *RateLimitMiddleware) getRateLimitKey(ctx context.Context) string {
	// Try to get user info for per-user rate limiting
	if userInfo, ok := UserInfoFromContext(ctx); ok {
		return fmt.Sprintf("user:%s", userInfo.ID)
	}

	// Try to get token for per-token rate limiting
	if token := extractToken(ctx); token != "" {
		// Use first 8 chars of token as key (don't expose full token)
		if len(token) > 8 {
			return fmt.Sprintf("token:%s", token[:8])
		}
		return fmt.Sprintf("token:%s", token)
	}

	// Default to global rate limiting
	return "global"
}

// Allow checks if a request is allowed under rate limits
func (l *RateLimiter) Allow(key string) (bool, error) {
	l.mu.Lock()
	bucket, exists := l.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(l.config.BurstSize),
			lastRefill: time.Now(),
		}
		l.buckets[key] = bucket
	}
	l.mu.Unlock()

	return bucket.consume(l.config)
}

// consume attempts to consume a token from the bucket
func (b *tokenBucket) consume(config *transport.RateLimitConfig) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	tokensToAdd := elapsed.Seconds() * (float64(config.RequestsPerMinute) / 60.0)

	b.tokens = min(b.tokens+tokensToAdd, float64(config.BurstSize))
	b.lastRefill = now

	// Try to consume a token
	if b.tokens >= 1.0 {
		b.tokens--
		return true, nil
	}

	return false, nil
}

// cleanup periodically removes old buckets
func (l *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, bucket := range l.buckets {
			bucket.mu.Lock()
			// Remove buckets that haven't been used in 10 minutes
			if now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(l.buckets, key)
			}
			bucket.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

// GetRemainingTokens returns the number of tokens remaining for a key
func (l *RateLimiter) GetRemainingTokens(key string) float64 {
	l.mu.RLock()
	bucket, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		return float64(l.config.BurstSize)
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	return bucket.tokens
}

// Reset resets the rate limit for a key
func (l *RateLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.buckets, key)
	l.mu.Unlock()
}

// Delegate all other Transport methods

func (m *RateLimitMiddleware) Initialize(ctx context.Context) error {
	return m.transport.Initialize(ctx)
}

func (m *RateLimitMiddleware) Start(ctx context.Context) error {
	return m.transport.Start(ctx)
}

func (m *RateLimitMiddleware) Stop(ctx context.Context) error {
	return m.transport.Stop(ctx)
}

func (m *RateLimitMiddleware) SendNotification(ctx context.Context, method string, params interface{}) error {
	if !m.config.Enabled {
		return m.transport.SendNotification(ctx, method, params)
	}

	key := m.getRateLimitKey(ctx)
	allowed, _ := m.limiter.Allow(key)
	if !allowed {
		return NewAuthError(ErrAccessDenied, "rate limit exceeded")
	}

	return m.transport.SendNotification(ctx, method, params)
}

func (m *RateLimitMiddleware) RegisterRequestHandler(method string, handler transport.RequestHandler) {
	m.transport.RegisterRequestHandler(method, handler)
}

func (m *RateLimitMiddleware) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	m.transport.RegisterNotificationHandler(method, handler)
}

func (m *RateLimitMiddleware) RegisterProgressHandler(id interface{}, handler transport.ProgressHandler) {
	m.transport.RegisterProgressHandler(id, handler)
}

func (m *RateLimitMiddleware) UnregisterProgressHandler(id interface{}) {
	m.transport.UnregisterProgressHandler(id)
}

func (m *RateLimitMiddleware) HandleResponse(response *protocol.Response) {
	m.transport.HandleResponse(response)
}

func (m *RateLimitMiddleware) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	if !m.config.Enabled {
		return m.transport.HandleRequest(ctx, request)
	}

	// For incoming requests, extract user info from request
	// This is simplified - in production, extract from auth headers
	key := "global"
	if userInfo, ok := UserInfoFromContext(ctx); ok {
		key = fmt.Sprintf("user:%s", userInfo.ID)
	}

	allowed, _ := m.limiter.Allow(key)
	if !allowed {
		return &protocol.Response{
			ID: request.ID,
			Error: &protocol.Error{
				Code:    protocol.InternalError,
				Message: "rate limit exceeded",
			},
		}, nil
	}

	return m.transport.HandleRequest(ctx, request)
}

func (m *RateLimitMiddleware) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	return m.transport.HandleNotification(ctx, notification)
}

func (m *RateLimitMiddleware) GenerateID() string {
	return m.transport.GenerateID()
}

func (m *RateLimitMiddleware) GetRequestIDPrefix() string {
	return m.transport.GetRequestIDPrefix()
}

func (m *RateLimitMiddleware) GetNextID() int64 {
	return m.transport.GetNextID()
}

func (m *RateLimitMiddleware) Cleanup() {
	m.transport.Cleanup()
}

func (m *RateLimitMiddleware) SendBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	if !m.config.Enabled {
		return m.transport.SendBatchRequest(ctx, batch)
	}

	// For batch requests, count as multiple requests
	key := m.getRateLimitKey(ctx)
	requestCount := batch.Len()

	// Check if we have enough tokens for the batch
	remaining := m.limiter.GetRemainingTokens(key)
	if remaining < float64(requestCount) {
		return nil, NewAuthError(ErrAccessDenied,
			fmt.Sprintf("rate limit exceeded: batch requires %d tokens, only %.0f available",
				requestCount, remaining))
	}

	// Consume tokens for each request in batch
	for i := 0; i < requestCount; i++ {
		allowed, _ := m.limiter.Allow(key)
		if !allowed {
			return nil, NewAuthError(ErrAccessDenied, "rate limit exceeded during batch processing")
		}
	}

	return m.transport.SendBatchRequest(ctx, batch)
}

func (m *RateLimitMiddleware) HandleBatchRequest(ctx context.Context, batch *protocol.JSONRPCBatchRequest) (*protocol.JSONRPCBatchResponse, error) {
	return m.transport.HandleBatchRequest(ctx, batch)
}

// Helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
