// Multi-Server Client Examples
// This example demonstrates how to manage connections to multiple MCP servers,
// implement load balancing, failover, and aggregation patterns.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/examples/shared"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// Multi-Server Management Patterns

// 1. Server Pool Management
// Manages a pool of MCP servers with health monitoring

type ServerInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Endpoint     string                 `json:"endpoint"`
	Priority     int                    `json:"priority"` // Higher = more preferred
	Weight       int                    `json:"weight"`   // For weighted load balancing
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`

	// Health status
	Healthy         bool          `json:"healthy"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	FailureCount    int64         `json:"failure_count"`
	ResponseTime    time.Duration `json:"response_time"`
}

type ServerPool struct {
	servers map[string]*ManagedServer
	config  ServerPoolConfig
	mutex   sync.RWMutex

	// Health monitoring
	healthChecker *HealthChecker

	// Metrics
	totalRequests  int64
	failedRequests int64

	ctx    context.Context
	cancel context.CancelFunc
}

type ManagedServer struct {
	Info      ServerInfo
	Client    client.Client
	Transport transport.Transport

	// Connection management
	connected     bool
	connecting    bool
	lastConnected time.Time
	connectMutex  sync.Mutex

	// Metrics
	requestCount int64
	errorCount   int64
	lastUsed     time.Time
}

type ServerPoolConfig struct {
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`
	MaxFailures         int           `json:"max_failures"`
	RecoveryTimeout     time.Duration `json:"recovery_timeout"`

	// Load balancing
	LoadBalancingStrategy LoadBalancingStrategy `json:"load_balancing_strategy"`

	// Connection management
	MaxConnectionsPerServer int           `json:"max_connections_per_server"`
	ConnectionTimeout       time.Duration `json:"connection_timeout"`
	IdleTimeout             time.Duration `json:"idle_timeout"`
}

type LoadBalancingStrategy string

const (
	LoadBalancingRoundRobin LoadBalancingStrategy = "round_robin"
	LoadBalancingWeighted   LoadBalancingStrategy = "weighted"
	LoadBalancingLeastConn  LoadBalancingStrategy = "least_connections"
	LoadBalancingPriority   LoadBalancingStrategy = "priority"
	LoadBalancingRandom     LoadBalancingStrategy = "random"
)

func NewServerPool(config ServerPoolConfig) *ServerPool {
	// Set defaults
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = 10 * time.Second
	}
	if config.MaxFailures == 0 {
		config.MaxFailures = 3
	}
	if config.RecoveryTimeout == 0 {
		config.RecoveryTimeout = 60 * time.Second
	}
	if config.LoadBalancingStrategy == "" {
		config.LoadBalancingStrategy = LoadBalancingRoundRobin
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &ServerPool{
		servers: make(map[string]*ManagedServer),
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Initialize health checker
	pool.healthChecker = NewHealthChecker(pool, config.HealthCheckInterval, config.HealthCheckTimeout)

	return pool
}

func (sp *ServerPool) AddServer(info ServerInfo) error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	// Create transport based on endpoint
	transportConfig := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	transportConfig.Endpoint = info.Endpoint

	t, err := transport.NewTransport(transportConfig)
	if err != nil {
		return fmt.Errorf("failed to create transport for server %s: %w", info.ID, err)
	}

	// Create client
	c := client.New(t,
		client.WithName(fmt.Sprintf("multi-client-%s", info.ID)),
		client.WithVersion("1.0.0"),
	)

	server := &ManagedServer{
		Info:      info,
		Client:    c,
		Transport: t,
		connected: false,
	}

	sp.servers[info.ID] = server

	log.Printf("Added server %s (%s) to pool", info.ID, info.Name)
	return nil
}

func (sp *ServerPool) RemoveServer(serverID string) error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	server, exists := sp.servers[serverID]
	if !exists {
		return fmt.Errorf("server %s not found", serverID)
	}

	// Disconnect if connected
	if server.connected {
		server.Client.Close()
	}

	delete(sp.servers, serverID)
	log.Printf("Removed server %s from pool", serverID)
	return nil
}

func (sp *ServerPool) GetServer(strategy LoadBalancingStrategy) (*ManagedServer, error) {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	healthyServers := make([]*ManagedServer, 0)
	for _, server := range sp.servers {
		if server.Info.Healthy && server.connected {
			healthyServers = append(healthyServers, server)
		}
	}

	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available")
	}

	switch strategy {
	case LoadBalancingRoundRobin:
		return sp.roundRobinSelect(healthyServers), nil
	case LoadBalancingWeighted:
		return sp.weightedSelect(healthyServers), nil
	case LoadBalancingLeastConn:
		return sp.leastConnectionsSelect(healthyServers), nil
	case LoadBalancingPriority:
		return sp.prioritySelect(healthyServers), nil
	case LoadBalancingRandom:
		return sp.randomSelect(healthyServers), nil
	default:
		return sp.roundRobinSelect(healthyServers), nil
	}
}

var roundRobinCounter int64

func (sp *ServerPool) roundRobinSelect(servers []*ManagedServer) *ManagedServer {
	index := atomic.AddInt64(&roundRobinCounter, 1) % int64(len(servers))
	return servers[index]
}

func (sp *ServerPool) weightedSelect(servers []*ManagedServer) *ManagedServer {
	totalWeight := 0
	for _, server := range servers {
		totalWeight += server.Info.Weight
	}

	if totalWeight == 0 {
		return sp.randomSelect(servers)
	}

	target := rand.Intn(totalWeight)
	current := 0

	for _, server := range servers {
		current += server.Info.Weight
		if current > target {
			return server
		}
	}

	return servers[len(servers)-1]
}

func (sp *ServerPool) leastConnectionsSelect(servers []*ManagedServer) *ManagedServer {
	minConnections := int64(-1)
	var selected *ManagedServer

	for _, server := range servers {
		connections := atomic.LoadInt64(&server.requestCount) - atomic.LoadInt64(&server.errorCount)
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selected = server
		}
	}

	return selected
}

func (sp *ServerPool) prioritySelect(servers []*ManagedServer) *ManagedServer {
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Info.Priority > servers[j].Info.Priority
	})
	return servers[0]
}

func (sp *ServerPool) randomSelect(servers []*ManagedServer) *ManagedServer {
	return servers[rand.Intn(len(servers))]
}

func (sp *ServerPool) Start(ctx context.Context) error {
	// Connect to all servers
	sp.mutex.RLock()
	var wg sync.WaitGroup
	for _, server := range sp.servers {
		wg.Add(1)
		go func(s *ManagedServer) {
			defer wg.Done()
			if err := sp.connectServer(ctx, s); err != nil {
				log.Printf("Failed to connect to server %s: %v", s.Info.ID, err)
			}
		}(server)
	}
	sp.mutex.RUnlock()

	wg.Wait()

	// Start health checker
	sp.healthChecker.Start(ctx)

	return nil
}

func (sp *ServerPool) Stop() {
	sp.cancel()

	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	for _, server := range sp.servers {
		if server.connected {
			server.Client.Close()
		}
	}

	sp.healthChecker.Stop()
}

func (sp *ServerPool) connectServer(ctx context.Context, server *ManagedServer) error {
	server.connectMutex.Lock()
	defer server.connectMutex.Unlock()

	if server.connecting || server.connected {
		return nil
	}

	server.connecting = true
	defer func() { server.connecting = false }()

	// Start transport
	if err := server.Transport.Start(ctx); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// Initialize client
	if err := server.Client.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	server.connected = true
	server.lastConnected = time.Now()
	server.Info.Healthy = true

	log.Printf("Connected to server %s (%s)", server.Info.ID, server.Info.Name)
	return nil
}

func (sp *ServerPool) GetHealthyServers() []ServerInfo {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	healthy := make([]ServerInfo, 0)
	for _, server := range sp.servers {
		if server.Info.Healthy {
			healthy = append(healthy, server.Info)
		}
	}
	return healthy
}

func (sp *ServerPool) GetMetrics() ServerPoolMetrics {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	metrics := ServerPoolMetrics{
		TotalServers:   len(sp.servers),
		HealthyServers: 0,
		TotalRequests:  atomic.LoadInt64(&sp.totalRequests),
		FailedRequests: atomic.LoadInt64(&sp.failedRequests),
		ServerMetrics:  make([]ServerMetrics, 0, len(sp.servers)),
	}

	for _, server := range sp.servers {
		if server.Info.Healthy {
			metrics.HealthyServers++
		}

		serverMetrics := ServerMetrics{
			ServerID:     server.Info.ID,
			RequestCount: atomic.LoadInt64(&server.requestCount),
			ErrorCount:   atomic.LoadInt64(&server.errorCount),
			ResponseTime: server.Info.ResponseTime,
			LastUsed:     server.lastUsed,
			Connected:    server.connected,
			Healthy:      server.Info.Healthy,
		}
		metrics.ServerMetrics = append(metrics.ServerMetrics, serverMetrics)
	}

	return metrics
}

type ServerPoolMetrics struct {
	TotalServers   int             `json:"total_servers"`
	HealthyServers int             `json:"healthy_servers"`
	TotalRequests  int64           `json:"total_requests"`
	FailedRequests int64           `json:"failed_requests"`
	ServerMetrics  []ServerMetrics `json:"server_metrics"`
}

type ServerMetrics struct {
	ServerID     string        `json:"server_id"`
	RequestCount int64         `json:"request_count"`
	ErrorCount   int64         `json:"error_count"`
	ResponseTime time.Duration `json:"response_time"`
	LastUsed     time.Time     `json:"last_used"`
	Connected    bool          `json:"connected"`
	Healthy      bool          `json:"healthy"`
}

// 2. Health Checker
// Monitors server health and updates status

type HealthChecker struct {
	pool     *ServerPool
	interval time.Duration
	timeout  time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewHealthChecker(pool *ServerPool, interval, timeout time.Duration) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthChecker{
		pool:     pool,
		interval: interval,
		timeout:  timeout,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (hc *HealthChecker) Start(ctx context.Context) {
	hc.wg.Add(1)
	go hc.healthCheckLoop()
}

func (hc *HealthChecker) Stop() {
	hc.cancel()
	hc.wg.Wait()
}

func (hc *HealthChecker) healthCheckLoop() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.performHealthChecks()
		case <-hc.ctx.Done():
			return
		}
	}
}

func (hc *HealthChecker) performHealthChecks() {
	hc.pool.mutex.RLock()
	servers := make([]*ManagedServer, 0, len(hc.pool.servers))
	for _, server := range hc.pool.servers {
		servers = append(servers, server)
	}
	hc.pool.mutex.RUnlock()

	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(s *ManagedServer) {
			defer wg.Done()
			hc.checkServerHealth(s)
		}(server)
	}
	wg.Wait()
}

func (hc *HealthChecker) checkServerHealth(server *ManagedServer) {
	ctx, cancel := context.WithTimeout(hc.ctx, hc.timeout)
	defer cancel()

	start := time.Now()

	// Try to list tools as a health check
	_, _, err := server.Client.ListTools(ctx, "", &protocol.PaginationParams{Limit: 1})

	duration := time.Since(start)
	server.Info.ResponseTime = duration
	server.Info.LastHealthCheck = time.Now()

	if err != nil {
		atomic.AddInt64(&server.Info.FailureCount, 1)
		log.Printf("Health check failed for server %s: %v", server.Info.ID, err)

		// Mark as unhealthy if too many failures
		if server.Info.FailureCount >= int64(hc.pool.config.MaxFailures) {
			server.Info.Healthy = false
			log.Printf("Server %s marked as unhealthy", server.Info.ID)
		}
	} else {
		// Reset failure count on success
		atomic.StoreInt64(&server.Info.FailureCount, 0)
		server.Info.Healthy = true
		server.lastUsed = time.Now()
	}
}

// 3. Multi-Server Client
// High-level client that manages multiple servers

type MultiServerClient struct {
	pool   *ServerPool
	config MultiServerConfig

	// Aggregation strategies
	aggregationStrategy AggregationStrategy

	// Fallback behavior
	enableFallback bool
	fallbackOrder  []string

	metrics *MultiServerMetrics
	mutex   sync.RWMutex
}

type MultiServerConfig struct {
	DefaultStrategy     LoadBalancingStrategy `json:"default_strategy"`
	AggregationStrategy AggregationStrategy   `json:"aggregation_strategy"`
	EnableFallback      bool                  `json:"enable_fallback"`
	FallbackOrder       []string              `json:"fallback_order"`
	RequestTimeout      time.Duration         `json:"request_timeout"`
	MaxRetries          int                   `json:"max_retries"`
}

type AggregationStrategy string

const (
	AggregationFirst    AggregationStrategy = "first"    // Return first successful result
	AggregationAll      AggregationStrategy = "all"      // Aggregate all results
	AggregationMajority AggregationStrategy = "majority" // Return majority consensus
	AggregationFastest  AggregationStrategy = "fastest"  // Return fastest response
)

type MultiServerMetrics struct {
	TotalRequests      int64                           `json:"total_requests"`
	SuccessfulRequests int64                           `json:"successful_requests"`
	FailedRequests     int64                           `json:"failed_requests"`
	AggregatedRequests int64                           `json:"aggregated_requests"`
	FallbackUsed       int64                           `json:"fallback_used"`
	AverageLatency     time.Duration                   `json:"average_latency"`
	StrategyUsage      map[LoadBalancingStrategy]int64 `json:"strategy_usage"`
}

func NewMultiServerClient(pool *ServerPool, config MultiServerConfig) *MultiServerClient {
	// Set defaults
	if config.DefaultStrategy == "" {
		config.DefaultStrategy = LoadBalancingRoundRobin
	}
	if config.AggregationStrategy == "" {
		config.AggregationStrategy = AggregationFirst
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &MultiServerClient{
		pool:                pool,
		config:              config,
		aggregationStrategy: config.AggregationStrategy,
		enableFallback:      config.EnableFallback,
		fallbackOrder:       config.FallbackOrder,
		metrics: &MultiServerMetrics{
			StrategyUsage: make(map[LoadBalancingStrategy]int64),
		},
	}
}

func (msc *MultiServerClient) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, error) {
	atomic.AddInt64(&msc.metrics.TotalRequests, 1)
	startTime := time.Now()

	defer func() {
		duration := time.Since(startTime)
		msc.updateLatencyMetrics(duration)
	}()

	switch msc.aggregationStrategy {
	case AggregationFirst:
		return msc.listToolsFirst(ctx, category, pagination)
	case AggregationAll:
		return msc.listToolsAggregated(ctx, category, pagination)
	case AggregationFastest:
		return msc.listToolsFastest(ctx, category, pagination)
	default:
		return msc.listToolsFirst(ctx, category, pagination)
	}
}

func (msc *MultiServerClient) listToolsFirst(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, error) {
	strategy := msc.config.DefaultStrategy
	msc.incrementStrategyUsage(strategy)

	for attempt := 0; attempt < msc.config.MaxRetries; attempt++ {
		server, err := msc.pool.GetServer(strategy)
		if err != nil {
			if msc.enableFallback {
				return msc.fallbackListTools(ctx, category, pagination)
			}
			return nil, err
		}

		tools, _, err := server.Client.ListTools(ctx, category, pagination)
		if err != nil {
			atomic.AddInt64(&server.errorCount, 1)
			log.Printf("Request failed on server %s (attempt %d): %v", server.Info.ID, attempt+1, err)
			continue
		}

		atomic.AddInt64(&server.requestCount, 1)
		server.lastUsed = time.Now()
		atomic.AddInt64(&msc.metrics.SuccessfulRequests, 1)
		return tools, nil
	}

	atomic.AddInt64(&msc.metrics.FailedRequests, 1)
	return nil, fmt.Errorf("all attempts failed")
}

func (msc *MultiServerClient) listToolsAggregated(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, error) {
	atomic.AddInt64(&msc.metrics.AggregatedRequests, 1)

	healthyServers := msc.pool.GetHealthyServers()
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available")
	}

	type result struct {
		tools    []protocol.Tool
		err      error
		serverID string
	}

	results := make(chan result, len(healthyServers))
	var wg sync.WaitGroup

	// Query all servers concurrently
	for _, serverInfo := range healthyServers {
		wg.Add(1)
		go func(info ServerInfo) {
			defer wg.Done()

			msc.pool.mutex.RLock()
			server, exists := msc.pool.servers[info.ID]
			msc.pool.mutex.RUnlock()

			if !exists || !server.connected {
				results <- result{err: fmt.Errorf("server not available"), serverID: info.ID}
				return
			}

			tools, _, err := server.Client.ListTools(ctx, category, pagination)
			results <- result{tools: tools, err: err, serverID: info.ID}
		}(serverInfo)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate results
	allTools := make([]protocol.Tool, 0)
	toolsMap := make(map[string]protocol.Tool)
	successCount := 0

	for res := range results {
		if res.err != nil {
			log.Printf("Error from server %s: %v", res.serverID, res.err)
			continue
		}

		successCount++
		for _, tool := range res.tools {
			if _, exists := toolsMap[tool.Name]; !exists {
				toolsMap[tool.Name] = tool
				allTools = append(allTools, tool)
			}
		}
	}

	if successCount == 0 {
		atomic.AddInt64(&msc.metrics.FailedRequests, 1)
		return nil, fmt.Errorf("all servers failed")
	}

	atomic.AddInt64(&msc.metrics.SuccessfulRequests, 1)
	return allTools, nil
}

func (msc *MultiServerClient) listToolsFastest(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, error) {
	healthyServers := msc.pool.GetHealthyServers()
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available")
	}

	type result struct {
		tools    []protocol.Tool
		err      error
		serverID string
		duration time.Duration
	}

	results := make(chan result, len(healthyServers))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start requests to all servers
	for _, serverInfo := range healthyServers {
		go func(info ServerInfo) {
			start := time.Now()

			msc.pool.mutex.RLock()
			server, exists := msc.pool.servers[info.ID]
			msc.pool.mutex.RUnlock()

			if !exists || !server.connected {
				results <- result{err: fmt.Errorf("server not available"), serverID: info.ID, duration: time.Since(start)}
				return
			}

			tools, _, err := server.Client.ListTools(ctx, category, pagination)
			duration := time.Since(start)
			results <- result{tools: tools, err: err, serverID: info.ID, duration: duration}
		}(serverInfo)
	}

	// Return first successful result
	for i := 0; i < len(healthyServers); i++ {
		select {
		case res := <-results:
			if res.err == nil {
				atomic.AddInt64(&msc.metrics.SuccessfulRequests, 1)
				log.Printf("Fastest response from server %s in %v", res.serverID, res.duration)
				return res.tools, nil
			}
		case <-ctx.Done():
			atomic.AddInt64(&msc.metrics.FailedRequests, 1)
			return nil, ctx.Err()
		}
	}

	atomic.AddInt64(&msc.metrics.FailedRequests, 1)
	return nil, fmt.Errorf("all servers failed")
}

func (msc *MultiServerClient) fallbackListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, error) {
	atomic.AddInt64(&msc.metrics.FallbackUsed, 1)

	for _, serverID := range msc.fallbackOrder {
		msc.pool.mutex.RLock()
		server, exists := msc.pool.servers[serverID]
		msc.pool.mutex.RUnlock()

		if !exists || !server.connected {
			continue
		}

		tools, _, err := server.Client.ListTools(ctx, category, pagination)
		if err == nil {
			atomic.AddInt64(&msc.metrics.SuccessfulRequests, 1)
			log.Printf("Fallback succeeded using server %s", serverID)
			return tools, nil
		}

		log.Printf("Fallback failed on server %s: %v", serverID, err)
	}

	atomic.AddInt64(&msc.metrics.FailedRequests, 1)
	return nil, fmt.Errorf("all fallback servers failed")
}

func (msc *MultiServerClient) CallTool(ctx context.Context, name string, input interface{}) (*protocol.CallToolResult, error) {
	atomic.AddInt64(&msc.metrics.TotalRequests, 1)
	startTime := time.Now()

	defer func() {
		duration := time.Since(startTime)
		msc.updateLatencyMetrics(duration)
	}()

	strategy := msc.config.DefaultStrategy
	msc.incrementStrategyUsage(strategy)

	for attempt := 0; attempt < msc.config.MaxRetries; attempt++ {
		server, err := msc.pool.GetServer(strategy)
		if err != nil {
			if msc.enableFallback {
				return msc.fallbackCallTool(ctx, name, input)
			}
			return nil, err
		}

		result, err := server.Client.CallTool(ctx, name, input, nil)
		if err != nil {
			atomic.AddInt64(&server.errorCount, 1)
			log.Printf("Tool call failed on server %s (attempt %d): %v", server.Info.ID, attempt+1, err)
			continue
		}

		atomic.AddInt64(&server.requestCount, 1)
		server.lastUsed = time.Now()
		atomic.AddInt64(&msc.metrics.SuccessfulRequests, 1)
		return result, nil
	}

	atomic.AddInt64(&msc.metrics.FailedRequests, 1)
	return nil, fmt.Errorf("all attempts failed")
}

func (msc *MultiServerClient) fallbackCallTool(ctx context.Context, name string, input interface{}) (*protocol.CallToolResult, error) {
	atomic.AddInt64(&msc.metrics.FallbackUsed, 1)

	for _, serverID := range msc.fallbackOrder {
		msc.pool.mutex.RLock()
		server, exists := msc.pool.servers[serverID]
		msc.pool.mutex.RUnlock()

		if !exists || !server.connected {
			continue
		}

		result, err := server.Client.CallTool(ctx, name, input, nil)
		if err == nil {
			atomic.AddInt64(&msc.metrics.SuccessfulRequests, 1)
			log.Printf("Fallback tool call succeeded using server %s", serverID)
			return result, nil
		}

		log.Printf("Fallback tool call failed on server %s: %v", serverID, err)
	}

	atomic.AddInt64(&msc.metrics.FailedRequests, 1)
	return nil, fmt.Errorf("all fallback servers failed")
}

func (msc *MultiServerClient) updateLatencyMetrics(duration time.Duration) {
	msc.mutex.Lock()
	defer msc.mutex.Unlock()

	// Simple moving average for latency
	if msc.metrics.AverageLatency == 0 {
		msc.metrics.AverageLatency = duration
	} else {
		msc.metrics.AverageLatency = (msc.metrics.AverageLatency + duration) / 2
	}
}

func (msc *MultiServerClient) incrementStrategyUsage(strategy LoadBalancingStrategy) {
	msc.mutex.Lock()
	defer msc.mutex.Unlock()
	msc.metrics.StrategyUsage[strategy]++
}

func (msc *MultiServerClient) GetMetrics() MultiServerMetrics {
	msc.mutex.RLock()
	defer msc.mutex.RUnlock()

	return MultiServerMetrics{
		TotalRequests:      atomic.LoadInt64(&msc.metrics.TotalRequests),
		SuccessfulRequests: atomic.LoadInt64(&msc.metrics.SuccessfulRequests),
		FailedRequests:     atomic.LoadInt64(&msc.metrics.FailedRequests),
		AggregatedRequests: atomic.LoadInt64(&msc.metrics.AggregatedRequests),
		FallbackUsed:       atomic.LoadInt64(&msc.metrics.FallbackUsed),
		AverageLatency:     msc.metrics.AverageLatency,
		StrategyUsage:      copyStrategyUsage(msc.metrics.StrategyUsage),
	}
}

func copyStrategyUsage(source map[LoadBalancingStrategy]int64) map[LoadBalancingStrategy]int64 {
	result := make(map[LoadBalancingStrategy]int64)
	for k, v := range source {
		result[k] = v
	}
	return result
}

// 4. Demo Functions

func createMockServers(count int) ([]*server.Server, []ServerInfo, error) {
	servers := make([]*server.Server, count)
	serverInfos := make([]ServerInfo, count)

	for i := 0; i < count; i++ {
		// Create transport for each server
		config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
		config.Endpoint = fmt.Sprintf("http://localhost:%d/mcp", 8090+i)

		t, err := transport.NewTransport(config)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create transport for server %d: %w", i, err)
		}

		// Create server
		s := server.New(t,
			server.WithName(fmt.Sprintf("Mock Server %d", i+1)),
			server.WithVersion("1.0.0"),
			server.WithToolsProvider(shared.CreateToolsProvider()),
			server.WithResourcesProvider(shared.CreateResourcesProvider()),
			server.WithPromptsProvider(shared.CreatePromptsProvider()),
		)

		servers[i] = s

		serverInfos[i] = ServerInfo{
			ID:           fmt.Sprintf("server-%d", i+1),
			Name:         fmt.Sprintf("Mock Server %d", i+1),
			Endpoint:     fmt.Sprintf("http://localhost:%d/mcp", 8090+i),
			Priority:     10 - i,       // Higher priority for lower numbered servers
			Weight:       (i + 1) * 10, // Higher weight for higher numbered servers
			Capabilities: []string{"tools", "resources", "prompts"},
			Metadata: map[string]interface{}{
				"region": fmt.Sprintf("region-%d", (i%3)+1),
				"zone":   fmt.Sprintf("zone-%c", 'a'+(i%3)),
			},
			Healthy: true,
		}
	}

	return servers, serverInfos, nil
}

func demonstrateMultiServerPatterns() {
	log.Println("=== Multi-Server Client Patterns Demo ===")

	// Create mock servers
	servers, serverInfos, err := createMockServers(3)
	if err != nil {
		log.Fatalf("Failed to create mock servers: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start all servers
	for i, s := range servers {
		go func(server *server.Server, index int) {
			if err := server.Start(ctx); err != nil {
				log.Printf("Server %d error: %v", index+1, err)
			}
		}(s, i)
	}

	// Wait for servers to start
	time.Sleep(3 * time.Second)

	// Create server pool
	poolConfig := ServerPoolConfig{
		HealthCheckInterval:     10 * time.Second,
		HealthCheckTimeout:      5 * time.Second,
		MaxFailures:             2,
		RecoveryTimeout:         30 * time.Second,
		LoadBalancingStrategy:   LoadBalancingRoundRobin,
		MaxConnectionsPerServer: 10,
		ConnectionTimeout:       15 * time.Second,
	}

	pool := NewServerPool(poolConfig)
	defer pool.Stop()

	// Add servers to pool
	for _, info := range serverInfos {
		if err := pool.AddServer(info); err != nil {
			log.Printf("Failed to add server %s: %v", info.ID, err)
		}
	}

	// Start the pool
	if err := pool.Start(ctx); err != nil {
		log.Printf("Failed to start server pool: %v", err)
		return
	}

	// Wait for connections
	time.Sleep(2 * time.Second)

	// Create multi-server client
	clientConfig := MultiServerConfig{
		DefaultStrategy:     LoadBalancingRoundRobin,
		AggregationStrategy: AggregationFirst,
		EnableFallback:      true,
		FallbackOrder:       []string{"server-1", "server-2", "server-3"},
		RequestTimeout:      10 * time.Second,
		MaxRetries:          3,
	}

	multiClient := NewMultiServerClient(pool, clientConfig)

	// Demonstrate different patterns
	log.Println("\n1. Testing Round Robin Load Balancing")
	testLoadBalancing(ctx, multiClient, LoadBalancingRoundRobin)

	log.Println("\n2. Testing Weighted Load Balancing")
	testLoadBalancing(ctx, multiClient, LoadBalancingWeighted)

	log.Println("\n3. Testing Result Aggregation")
	testAggregation(ctx, multiClient)

	log.Println("\n4. Testing Failover")
	testFailover(ctx, multiClient, pool)

	log.Println("\n5. Final Metrics")
	printMultiServerMetrics(multiClient, pool)

	// Stop servers
	for _, s := range servers {
		s.Stop()
	}
}

func testLoadBalancing(ctx context.Context, client *MultiServerClient, strategy LoadBalancingStrategy) {
	// Temporarily change strategy
	originalStrategy := client.config.DefaultStrategy
	client.config.DefaultStrategy = strategy
	defer func() { client.config.DefaultStrategy = originalStrategy }()

	log.Printf("Testing %s strategy", strategy)

	for i := 0; i < 6; i++ {
		tools, err := client.ListTools(ctx, "", &protocol.PaginationParams{Limit: 5})
		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
		} else {
			log.Printf("Request %d: Got %d tools", i+1, len(tools))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func testAggregation(ctx context.Context, client *MultiServerClient) {
	// Test aggregation strategy
	originalStrategy := client.aggregationStrategy
	client.aggregationStrategy = AggregationAll
	defer func() { client.aggregationStrategy = originalStrategy }()

	log.Println("Testing result aggregation from all servers")

	tools, err := client.ListTools(ctx, "", &protocol.PaginationParams{Limit: 10})
	if err != nil {
		log.Printf("Aggregation failed: %v", err)
	} else {
		log.Printf("Aggregated %d unique tools from all servers", len(tools))
	}
}

func testFailover(ctx context.Context, client *MultiServerClient, pool *ServerPool) {
	log.Println("Testing failover by removing a server")

	// Remove a server to simulate failure
	if err := pool.RemoveServer("server-2"); err != nil {
		log.Printf("Failed to remove server: %v", err)
	}

	// Test requests continue working
	for i := 0; i < 3; i++ {
		tools, err := client.ListTools(ctx, "", &protocol.PaginationParams{Limit: 3})
		if err != nil {
			log.Printf("Failover request %d failed: %v", i+1, err)
		} else {
			log.Printf("Failover request %d: Got %d tools", i+1, len(tools))
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Println("Failover test completed")
}

func printMultiServerMetrics(client *MultiServerClient, pool *ServerPool) {
	clientMetrics := client.GetMetrics()
	poolMetrics := pool.GetMetrics()

	log.Println("=== Multi-Server Client Metrics ===")
	log.Printf("Total Requests: %d", clientMetrics.TotalRequests)
	log.Printf("Successful Requests: %d", clientMetrics.SuccessfulRequests)
	log.Printf("Failed Requests: %d", clientMetrics.FailedRequests)
	log.Printf("Aggregated Requests: %d", clientMetrics.AggregatedRequests)
	log.Printf("Fallback Used: %d", clientMetrics.FallbackUsed)
	log.Printf("Average Latency: %v", clientMetrics.AverageLatency)

	if clientMetrics.TotalRequests > 0 {
		successRate := float64(clientMetrics.SuccessfulRequests) / float64(clientMetrics.TotalRequests) * 100
		log.Printf("Success Rate: %.2f%%", successRate)
	}

	log.Println("\n=== Load Balancing Strategy Usage ===")
	for strategy, count := range clientMetrics.StrategyUsage {
		log.Printf("%s: %d requests", strategy, count)
	}

	log.Println("\n=== Server Pool Metrics ===")
	log.Printf("Total Servers: %d", poolMetrics.TotalServers)
	log.Printf("Healthy Servers: %d", poolMetrics.HealthyServers)
	log.Printf("Pool Total Requests: %d", poolMetrics.TotalRequests)
	log.Printf("Pool Failed Requests: %d", poolMetrics.FailedRequests)

	log.Println("\n=== Individual Server Metrics ===")
	for _, sm := range poolMetrics.ServerMetrics {
		log.Printf("Server %s:", sm.ServerID)
		log.Printf("  Requests: %d, Errors: %d", sm.RequestCount, sm.ErrorCount)
		log.Printf("  Response Time: %v", sm.ResponseTime)
		log.Printf("  Connected: %t, Healthy: %t", sm.Connected, sm.Healthy)
		log.Printf("  Last Used: %v", sm.LastUsed.Format("15:04:05"))
	}
}

func main() {
	log.Println("Multi-Server Client Examples")
	log.Println("===========================")

	// Handle graceful shutdown
	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nReceived shutdown signal...")
		cancel()
	}()

	// Run demonstrations
	demonstrateMultiServerPatterns()

	// Wait for potential cleanup
	select {
	case <-mainCtx.Done():
		log.Println("Shutdown completed")
	default:
		// Continue normally
	}

	log.Println("\n=== Multi-Server Examples Complete ===")
	log.Println("Patterns demonstrated:")
	log.Println("- Server Pool Management with health monitoring")
	log.Println("- Load Balancing (Round Robin, Weighted, Priority, etc.)")
	log.Println("- Result Aggregation from multiple servers")
	log.Println("- Automatic Failover and recovery")
	log.Println("- Comprehensive metrics and monitoring")
}
