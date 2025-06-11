# MCP SDK Performance Benchmarks

This directory contains comprehensive performance and load testing tools for the Model Context Protocol (MCP) Go SDK.

## Overview

The benchmark suite includes:

- **Transport Layer Benchmarks**: Performance tests for stdio and HTTP transports
- **Client Operation Benchmarks**: Tests for client-side operations (CallTool, ReadResource, etc.)
- **Server Operation Benchmarks**: Tests for server-side request handling
- **Load Testing Framework**: Configurable load testing with multiple clients
- **Memory Leak Detection**: Tests to detect memory leaks and excessive allocations
- **Stress Testing**: High-load scenarios to test system limits

## Running Benchmarks

### Basic Benchmarks

Run all benchmarks:
```bash
go test -bench=. -benchmem
```

Run specific benchmark categories:
```bash
# Transport benchmarks
go test -bench=BenchmarkStdioTransport -benchmem

# Client benchmarks
go test -bench=BenchmarkClientOperations -benchmem

# Server benchmarks
go test -bench=BenchmarkServerOperations -benchmem

# Memory allocation benchmarks
go test -bench=BenchmarkMemoryAllocation -benchmem
```

### Memory Leak Tests

Run memory leak detection tests:
```bash
# Run all memory tests
go test -run=TestMemoryLeak

# Run with verbose output
go test -run=TestMemoryLeak -v

# Skip in short mode
go test -run=TestMemoryLeak -short
```

### Load Testing

Run the example load test:
```bash
go run example_loadtest.go
```

Or create custom load tests:
```go
config := benchmarks.LoadTestConfig{
    Clients:           50,
    RequestsPerClient: 1000,
    RateLimit:         200, // 200 req/s
    Duration:          60 * time.Second,
    OperationMix: benchmarks.OperationMix{
        CallTool:      40,
        ReadResource:  30,
        ListTools:     20,
        ListResources: 10,
    },
}

tester := benchmarks.NewLoadTester(config)
result, err := tester.Run(context.Background())
if err != nil {
    log.Fatal(err)
}

result.PrintResults()
```

## Benchmark Categories

### 1. Transport Benchmarks (`transport_bench_test.go`)

Tests the performance of the transport layer:

- **SendRequest**: Single request/response performance
- **SendNotification**: Notification sending performance
- **BatchRequest**: Batch processing with various sizes (10, 100, 1000)
- **ConcurrentRequests**: Concurrent request handling
- **Message Serialization/Deserialization**: JSON-RPC message processing

Example results:
```
BenchmarkStdioTransport/SendRequest-8         	    5000	    250125 ns/op	    1024 B/op	      15 allocs/op
BenchmarkStdioTransport/BatchRequest/100-8    	     100	  10234567 ns/op	  102400 B/op	    1500 allocs/op
```

### 2. Client Benchmarks (`client_bench_test.go`)

Tests client-side operation performance:

- **CallTool**: Tool calling performance
- **ReadResource**: Resource reading performance
- **ListTools/ListResources**: List operations with pagination
- **ConcurrentToolCalls**: Concurrent client operations
- **WithCallbacks**: Performance impact of callbacks

### 3. Server Benchmarks (`server_bench_test.go`)

Tests server-side request handling:

- **HandleRequest**: Single request processing
- **HandleBatchRequest**: Batch request processing
- **WithProviders**: Performance with all providers enabled
- **ConcurrentRequests**: Concurrent request handling
- **ResourceSubscriptions**: Subscription management performance

### 4. Load Testing (`loadtest.go`)

Comprehensive load testing framework with:

- **Configurable Client Count**: Scale from 1 to 1000+ clients
- **Rate Limiting**: Control request rate (req/s)
- **Operation Mix**: Configure distribution of operations
- **Ramp-up Period**: Gradual load increase
- **Real-time Reporting**: Progress monitoring
- **Detailed Statistics**: Latency percentiles, throughput, errors

### 5. Memory Testing (`memory_test.go`)

Memory leak detection and allocation tracking:

- **Memory Leak Detection**: Tests for client, server, and batch operations
- **Goroutine Leak Detection**: Ensures proper resource cleanup
- **Allocation Benchmarks**: Memory allocation patterns
- **Long-running Tests**: Extended operations to detect gradual leaks

## Performance Targets

The MCP SDK aims to meet these performance targets:

### Latency Targets
- **P50 Latency**: < 5ms for standard operations
- **P90 Latency**: < 10ms for standard operations
- **P99 Latency**: < 50ms for standard operations
- **Max Latency**: < 100ms under normal load

### Throughput Targets
- **Single Client**: 1000+ req/s
- **Concurrent Clients**: 10,000+ req/s aggregate
- **Batch Processing**: 50,000+ operations/s

### Resource Targets
- **Memory Growth**: < 1MB per 10,000 operations
- **Goroutine Leaks**: Zero leaks after operations
- **Connection Pools**: Efficient reuse, no leaks

## Interpreting Results

### Benchmark Output

```
BenchmarkClientCallTool-8    5000    300000 ns/op    2048 B/op    25 allocs/op
                     |        |         |            |         |
                     |        |         |            |         └─ Allocations per operation
                     |        |         |            └─ Bytes allocated per operation
                     |        |         └─ Nanoseconds per operation
                     |        └─ Number of iterations
                     └─ Test name and CPU count
```

### Load Test Output

```
=== Load Test Results ===
Total Duration: 30s
Total Requests: 15000
Successful: 14950 (99.7%)
Failed: 50 (0.3%)
Requests/sec: 500.00

Latency Statistics (ms):
  Min: 0.50
  Avg: 12.34
  P50: 8.20
  P90: 25.40
  P95: 35.60
  P99: 75.20
  Max: 120.30
```

### Memory Test Output

```
Memory growth: 2.45 MB after 10000 operations
Initial heap: 5.23 MB
Final heap: 7.68 MB
Goroutines: initial=8, final=8
```

## Continuous Integration

Add benchmark validation to CI:

```yaml
# .github/workflows/benchmarks.yml
- name: Run Benchmarks
  run: |
    go test -bench=. -benchmem -benchtime=10s ./benchmarks/

- name: Memory Leak Tests
  run: |
    go test -run=TestMemoryLeak -v ./benchmarks/

- name: Performance Regression
  run: |
    go test -bench=BenchmarkStdioTransport/SendRequest -count=5 -benchmem
```

## Troubleshooting

### Poor Performance

1. **Check System Resources**: CPU, Memory, Network
2. **Profile the Application**: Use `go tool pprof`
3. **Review Benchmark Code**: Ensure realistic test scenarios
4. **Check Configuration**: Verify transport and middleware settings

### Memory Leaks

1. **Run with Race Detection**: `go test -race`
2. **Use Memory Profiler**: `go test -memprofile=mem.prof`
3. **Check Goroutine Count**: Look for goroutine leaks
4. **Review Resource Cleanup**: Ensure proper defer statements

### Inconsistent Results

1. **Run Multiple Times**: Use `-count=5` for statistical significance
2. **Disable CPU Frequency Scaling**: For consistent timing
3. **Reduce System Load**: Close other applications
4. **Use Longer Benchmark Time**: `-benchtime=30s`

## Best Practices

1. **Baseline Before Changes**: Establish performance baselines
2. **Test Realistic Scenarios**: Use production-like workloads
3. **Monitor Continuously**: Track performance over time
4. **Test Different Loads**: Light, medium, and stress scenarios
5. **Validate in Production**: Compare with real-world metrics

## Contributing

When adding new benchmarks:

1. Follow the existing naming convention: `BenchmarkOperation`
2. Include memory allocation tracking: `b.ReportAllocs()`
3. Reset timer after setup: `b.ResetTimer()`
4. Test multiple scenarios: concurrent, batch, stress
5. Add documentation for new test categories
