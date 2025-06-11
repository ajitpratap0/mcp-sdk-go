# Multi-Server Client Examples

This example demonstrates how to build resilient, scalable MCP applications that connect to multiple servers. It implements enterprise-grade patterns for load balancing, failover, result aggregation, and health monitoring across multiple MCP server instances.

## 🎯 Overview

Modern applications often need to distribute load across multiple servers or aggregate capabilities from different sources. This example provides production-ready patterns for:

- **Server Pool Management**: Centralized management of multiple MCP server connections
- **Load Balancing**: Distribute requests across servers using various strategies
- **Health Monitoring**: Automatic health checks and failure detection
- **Failover**: Automatic failover to healthy servers
- **Result Aggregation**: Combine results from multiple servers
- **Metrics & Monitoring**: Comprehensive observability

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                Multi-Server Client                      │
├─────────────────────────────────────────────────────────┤
│                  Server Pool                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐        │
│  │   Server 1  │ │   Server 2  │ │   Server 3  │        │
│  │  (Primary)  │ │ (Secondary) │ │  (Backup)   │        │
│  └─────────────┘ └─────────────┘ └─────────────┘        │
├─────────────────────────────────────────────────────────┤
│              Health Monitoring                          │
│           Load Balancing Strategies                     │
│              Failure Detection                          │
└─────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. **ServerPool**
Manages a collection of MCP servers with:
- Connection management and health monitoring
- Load balancing strategy selection
- Automatic server discovery and registration
- Metrics collection per server

#### 2. **HealthChecker**
Monitors server health by:
- Periodic health checks using MCP operations
- Failure count tracking
- Response time monitoring
- Automatic server status updates

#### 3. **MultiServerClient**
High-level client providing:
- Transparent multi-server operations
- Result aggregation strategies
- Automatic failover
- Request routing and load balancing

## 🔀 Load Balancing Strategies

### 1. Round Robin
Distributes requests evenly across all healthy servers.

```go
strategy := LoadBalancingRoundRobin
server, err := pool.GetServer(strategy)
```

**Use Case**: Equal server capabilities, consistent performance

### 2. Weighted
Routes requests based on server weights (capacity-based routing).

```go
serverInfo := ServerInfo{
    Weight: 100, // Higher weight = more requests
}
strategy := LoadBalancingWeighted
```

**Use Case**: Servers with different capacities or performance characteristics

### 3. Least Connections
Routes to the server with the fewest active connections.

```go
strategy := LoadBalancingLeastConn
```

**Use Case**: Long-running operations, variable request complexity

### 4. Priority-Based
Routes to highest priority servers first, with fallback.

```go
serverInfo := ServerInfo{
    Priority: 10, // Higher priority = preferred
}
strategy := LoadBalancingPriority
```

**Use Case**: Primary/secondary server setups, cost optimization

### 5. Random
Randomly selects from healthy servers.

```go
strategy := LoadBalancingRandom
```

**Use Case**: Simple load distribution, stateless operations

## 📊 Result Aggregation Strategies

### 1. First Successful
Returns the first successful response.

```go
config := MultiServerConfig{
    AggregationStrategy: AggregationFirst,
}
```

**Best For**: Fast response times, any valid result acceptable

### 2. Aggregate All
Combines results from all servers.

```go
config := MultiServerConfig{
    AggregationStrategy: AggregationAll,
}
```

**Best For**: Comprehensive tool/resource discovery, data completeness

### 3. Fastest Response
Returns the first response received (race condition).

```go
config := MultiServerConfig{
    AggregationStrategy: AggregationFastest,
}
```

**Best For**: Latency-critical operations, redundant servers

### 4. Majority Consensus
Returns results agreed upon by majority of servers.

```go
config := MultiServerConfig{
    AggregationStrategy: AggregationMajority,
}
```

**Best For**: Data validation, consistency requirements

## 🚀 Usage Examples

### Basic Multi-Server Setup

```go
// Create server pool
config := ServerPoolConfig{
    HealthCheckInterval:   30 * time.Second,
    HealthCheckTimeout:    10 * time.Second,
    MaxFailures:          3,
    LoadBalancingStrategy: LoadBalancingRoundRobin,
}

pool := NewServerPool(config)

// Add servers
servers := []ServerInfo{
    {
        ID:       "server-1",
        Name:     "Primary Server",
        Endpoint: "http://server1:8080/mcp",
        Priority: 10,
        Weight:   100,
    },
    {
        ID:       "server-2",
        Name:     "Secondary Server",
        Endpoint: "http://server2:8080/mcp",
        Priority: 5,
        Weight:   50,
    },
}

for _, info := range servers {
    pool.AddServer(info)
}

// Start the pool
pool.Start(ctx)
```

### Multi-Server Client

```go
// Create multi-server client
clientConfig := MultiServerConfig{
    DefaultStrategy:     LoadBalancingRoundRobin,
    AggregationStrategy: AggregationFirst,
    EnableFallback:      true,
    FallbackOrder:       []string{"server-1", "server-2"},
    RequestTimeout:      30 * time.Second,
    MaxRetries:          3,
}

client := NewMultiServerClient(pool, clientConfig)

// Use normally - multi-server logic is transparent
tools, err := client.ListTools(ctx, "", nil)
if err != nil {
    log.Printf("Failed to list tools: %v", err)
}

result, err := client.CallTool(ctx, "calculator", map[string]interface{}{
    "operation": "add",
    "a": 5,
    "b": 3,
})
```

### Advanced Configuration

```go
// Production configuration with comprehensive settings
productionConfig := ServerPoolConfig{
    // Health monitoring
    HealthCheckInterval:     15 * time.Second,
    HealthCheckTimeout:      5 * time.Second,
    MaxFailures:            2,
    RecoveryTimeout:        60 * time.Second,

    // Performance tuning
    MaxConnectionsPerServer: 20,
    ConnectionTimeout:       30 * time.Second,
    IdleTimeout:            300 * time.Second,

    // Load balancing
    LoadBalancingStrategy:   LoadBalancingLeastConn,
}

// Client with aggregation and fallback
clientConfig := MultiServerConfig{
    DefaultStrategy:     LoadBalancingWeighted,
    AggregationStrategy: AggregationAll,
    EnableFallback:      true,
    FallbackOrder:       []string{"primary", "secondary", "backup"},
    RequestTimeout:      45 * time.Second,
    MaxRetries:          5,
}
```

## 🏥 Health Monitoring

### Health Check Configuration

```go
config := ServerPoolConfig{
    HealthCheckInterval: 30 * time.Second,  // Check every 30 seconds
    HealthCheckTimeout:  10 * time.Second,  // 10 second timeout
    MaxFailures:         3,                 // Unhealthy after 3 failures
    RecoveryTimeout:     60 * time.Second,  // Wait 60s before retry
}
```

### Health Check Process

1. **Periodic Checks**: Automatic health checks using `ListTools` operation
2. **Failure Tracking**: Count consecutive failures per server
3. **Status Updates**: Mark servers healthy/unhealthy based on thresholds
4. **Recovery**: Automatic re-check of failed servers

### Health Metrics

```go
type ServerMetrics struct {
    ServerID     string        `json:"server_id"`
    RequestCount int64         `json:"request_count"`
    ErrorCount   int64         `json:"error_count"`
    ResponseTime time.Duration `json:"response_time"`
    LastUsed     time.Time     `json:"last_used"`
    Connected    bool          `json:"connected"`
    Healthy      bool          `json:"healthy"`
}
```

## 🔄 Failover Patterns

### Automatic Failover

```go
// Client automatically fails over to healthy servers
tools, err := client.ListTools(ctx, "", nil)
// If primary server fails, automatically tries secondary
```

### Explicit Fallback Order

```go
config := MultiServerConfig{
    EnableFallback: true,
    FallbackOrder:  []string{"primary", "secondary", "backup"},
}

// Fallback order:
// 1. Try load-balanced server selection
// 2. If all fail, try servers in fallback order
// 3. Return error only if all servers fail
```

### Circuit Breaker Integration

```go
// Combine with circuit breaker for advanced failure handling
resilientClient := &ResilientMultiServerClient{
    multiClient: multiClient,
    circuitBreaker: NewCircuitBreaker(CircuitBreakerConfig{
        FailureThreshold: 5,
        RecoveryTimeout:  60 * time.Second,
    }),
}
```

## 📈 Performance Optimization

### Connection Pooling

```go
config := ServerPoolConfig{
    MaxConnectionsPerServer: 10,  // Pool size per server
    ConnectionTimeout:       30 * time.Second,
    IdleTimeout:            120 * time.Second,
}
```

### Request Batching

```go
// Batch multiple operations for efficiency
type BatchClient struct {
    multiClient *MultiServerClient
    batchSize   int
    flushDelay  time.Duration
}

func (bc *BatchClient) AddToBatch(operation Operation) {
    // Add to batch and flush when full or after delay
}
```

### Caching Layer

```go
// Add caching for frequently accessed data
type CachedMultiServerClient struct {
    multiClient *MultiServerClient
    cache       map[string]CacheEntry
    cacheTTL    time.Duration
}

func (cmc *CachedMultiServerClient) ListTools(ctx context.Context, category string) ([]protocol.Tool, error) {
    // Check cache first, then fall back to servers
    if cached, ok := cmc.cache[category]; ok && time.Since(cached.Timestamp) < cmc.cacheTTL {
        return cached.Tools, nil
    }

    tools, err := cmc.multiClient.ListTools(ctx, category, nil)
    if err == nil {
        cmc.cache[category] = CacheEntry{Tools: tools, Timestamp: time.Now()}
    }
    return tools, err
}
```

## 📊 Monitoring and Metrics

### Server Pool Metrics

```go
metrics := pool.GetMetrics()

log.Printf("Total Servers: %d", metrics.TotalServers)
log.Printf("Healthy Servers: %d", metrics.HealthyServers)
log.Printf("Total Requests: %d", metrics.TotalRequests)
log.Printf("Failed Requests: %d", metrics.FailedRequests)

for _, sm := range metrics.ServerMetrics {
    log.Printf("Server %s: %d requests, %d errors, %v response time",
        sm.ServerID, sm.RequestCount, sm.ErrorCount, sm.ResponseTime)
}
```

### Client Metrics

```go
clientMetrics := multiClient.GetMetrics()

successRate := float64(clientMetrics.SuccessfulRequests) /
               float64(clientMetrics.TotalRequests) * 100

log.Printf("Success Rate: %.2f%%", successRate)
log.Printf("Average Latency: %v", clientMetrics.AverageLatency)
log.Printf("Fallback Usage: %d", clientMetrics.FallbackUsed)

// Strategy usage breakdown
for strategy, count := range clientMetrics.StrategyUsage {
    log.Printf("%s strategy used %d times", strategy, count)
}
```

### Alerting Integration

```go
// Set up alerting based on metrics
func (pool *ServerPool) checkAlerts() {
    metrics := pool.GetMetrics()

    // Alert on low server availability
    if metrics.HealthyServers < 2 {
        sendAlert("Low server availability",
                 fmt.Sprintf("Only %d servers healthy", metrics.HealthyServers))
    }

    // Alert on high error rate
    if metrics.TotalRequests > 0 {
        errorRate := float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
        if errorRate > 0.05 { // 5% error rate threshold
            sendAlert("High error rate",
                     fmt.Sprintf("Error rate: %.2f%%", errorRate*100))
        }
    }
}
```

## 🧪 Testing Patterns

### Load Testing

```go
func TestMultiServerLoadBalancing(t *testing.T) {
    // Create test servers
    servers := createTestServers(3)
    defer stopTestServers(servers)

    // Create pool and client
    pool := NewServerPool(ServerPoolConfig{
        LoadBalancingStrategy: LoadBalancingRoundRobin,
    })

    for i, server := range servers {
        pool.AddServer(ServerInfo{
            ID:       fmt.Sprintf("server-%d", i),
            Endpoint: server.URL,
        })
    }

    client := NewMultiServerClient(pool, MultiServerConfig{})

    // Generate load
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, err := client.ListTools(context.Background(), "", nil)
            assert.NoError(t, err)
        }()
    }
    wg.Wait()

    // Verify load distribution
    metrics := pool.GetMetrics()
    for _, sm := range metrics.ServerMetrics {
        assert.Greater(t, sm.RequestCount, int64(25)) // Roughly even distribution
        assert.Less(t, sm.RequestCount, int64(40))
    }
}
```

### Failover Testing

```go
func TestFailoverBehavior(t *testing.T) {
    servers := createTestServers(3)
    pool := setupServerPool(servers)
    client := NewMultiServerClient(pool, MultiServerConfig{
        EnableFallback: true,
        FallbackOrder:  []string{"server-1", "server-2", "server-3"},
    })

    // Verify normal operation
    tools, err := client.ListTools(context.Background(), "", nil)
    assert.NoError(t, err)
    assert.NotEmpty(t, tools)

    // Simulate server failure
    servers[0].Close()

    // Verify failover works
    tools, err = client.ListTools(context.Background(), "", nil)
    assert.NoError(t, err)
    assert.NotEmpty(t, tools)

    // Check metrics show fallback usage
    metrics := client.GetMetrics()
    assert.Greater(t, metrics.FallbackUsed, int64(0))
}
```

### Health Check Testing

```go
func TestHealthMonitoring(t *testing.T) {
    servers := createTestServers(2)
    pool := NewServerPool(ServerPoolConfig{
        HealthCheckInterval: 1 * time.Second,
        MaxFailures:        2,
    })

    // Add servers and start monitoring
    addServersToPool(pool, servers)
    pool.Start(context.Background())

    // Verify initial health
    time.Sleep(2 * time.Second)
    metrics := pool.GetMetrics()
    assert.Equal(t, 2, metrics.HealthyServers)

    // Simulate server failure
    servers[0].Close()

    // Wait for health check to detect failure
    time.Sleep(3 * time.Second)
    metrics = pool.GetMetrics()
    assert.Equal(t, 1, metrics.HealthyServers)
}
```

## ⚙️ Configuration Examples

### Development Environment

```go
devConfig := ServerPoolConfig{
    HealthCheckInterval:     5 * time.Second,  // Fast feedback
    HealthCheckTimeout:      2 * time.Second,
    MaxFailures:            1,                 // Fail fast
    RecoveryTimeout:        10 * time.Second,  // Quick recovery
    LoadBalancingStrategy:   LoadBalancingRoundRobin,
    ConnectionTimeout:       5 * time.Second,
}

devClientConfig := MultiServerConfig{
    DefaultStrategy:     LoadBalancingRoundRobin,
    AggregationStrategy: AggregationFirst,
    EnableFallback:      false, // Fail fast for debugging
    RequestTimeout:      10 * time.Second,
    MaxRetries:          1,
}
```

### Production Environment

```go
prodConfig := ServerPoolConfig{
    HealthCheckInterval:     30 * time.Second,
    HealthCheckTimeout:      10 * time.Second,
    MaxFailures:            3,
    RecoveryTimeout:        120 * time.Second,
    LoadBalancingStrategy:   LoadBalancingLeastConn,
    MaxConnectionsPerServer: 50,
    ConnectionTimeout:       30 * time.Second,
    IdleTimeout:            300 * time.Second,
}

prodClientConfig := MultiServerConfig{
    DefaultStrategy:     LoadBalancingWeighted,
    AggregationStrategy: AggregationFirst,
    EnableFallback:      true,
    FallbackOrder:       []string{"primary", "secondary", "backup"},
    RequestTimeout:      60 * time.Second,
    MaxRetries:          5,
}
```

### High Availability Setup

```go
haConfig := ServerPoolConfig{
    HealthCheckInterval:     15 * time.Second,
    HealthCheckTimeout:      5 * time.Second,
    MaxFailures:            2,
    RecoveryTimeout:        60 * time.Second,
    LoadBalancingStrategy:   LoadBalancingPriority,
}

// Multi-region server setup
servers := []ServerInfo{
    {ID: "us-west-1", Priority: 10, Weight: 100},   // Primary region
    {ID: "us-west-2", Priority: 9, Weight: 100},    // Same region backup
    {ID: "us-east-1", Priority: 5, Weight: 50},     // Cross-region backup
    {ID: "eu-west-1", Priority: 1, Weight: 25},     // International backup
}
```

## 🔍 Troubleshooting

### Common Issues

#### High Latency
```bash
# Check server response times
curl http://localhost:8080/metrics | grep mcp_response_time

# Monitor load distribution
grep "Load balancing" app.log | tail -20
```

**Solutions**:
- Switch to `LoadBalancingLeastConn` strategy
- Increase connection pool sizes
- Add more server instances

#### Uneven Load Distribution
```bash
# Check request counts per server
curl http://localhost:8080/metrics | grep mcp_requests_total
```

**Solutions**:
- Verify server weights are set correctly
- Check for server health issues
- Consider `LoadBalancingWeighted` strategy

#### Failover Not Working
```bash
# Check health check logs
grep "Health check" app.log | tail -20

# Verify fallback configuration
grep "Fallback" app.log
```

**Solutions**:
- Reduce `MaxFailures` threshold
- Decrease `HealthCheckInterval`
- Verify fallback order is correct

## 🚀 Quick Start

```bash
# Run the example
go run main.go

# Monitor in another terminal
curl http://localhost:8080/metrics
tail -f app.log | grep -E "(Load|Health|Failover)"
```

## 📚 Integration Examples

### With Kubernetes

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mcp-servers
spec:
  selector:
    app: mcp-server
  ports:
  - port: 8080
    targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-client
spec:
  template:
    spec:
      containers:
      - name: client
        env:
        - name: MCP_SERVERS
          value: "mcp-server-1:8080,mcp-server-2:8080,mcp-server-3:8080"
        - name: LOAD_BALANCING_STRATEGY
          value: "least_connections"
```

### With Service Discovery

```go
// Integrate with service discovery (Consul, etcd, etc.)
type ServiceDiscoveryServerPool struct {
    *ServerPool
    discovery ServiceDiscovery
}

func (sdsp *ServiceDiscoveryServerPool) watchServices() {
    for {
        services, err := sdsp.discovery.DiscoverServices("mcp-server")
        if err != nil {
            log.Printf("Service discovery error: %v", err)
            continue
        }

        // Update server pool based on discovered services
        sdsp.updateServers(services)
        time.Sleep(30 * time.Second)
    }
}
```

This comprehensive multi-server implementation provides the foundation for building highly available, scalable MCP applications that can handle production workloads with grace and resilience.
