// Package benchmarks provides performance and load testing for the MCP SDK
package benchmarks

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// LoadTestConfig configures load testing parameters
type LoadTestConfig struct {
	// Number of concurrent clients
	Clients int

	// Number of requests per client
	RequestsPerClient int

	// Request rate limit (requests per second, 0 = unlimited)
	RateLimit int

	// Test duration (0 = run until all requests complete)
	Duration time.Duration

	// Ramp up period for gradual load increase
	RampUpTime time.Duration

	// Mix of operations to perform
	OperationMix OperationMix

	// Transport configuration
	TransportType transport.TransportType
	Endpoint      string

	// Reporting interval
	ReportInterval time.Duration
}

// OperationMix defines the distribution of different operations
type OperationMix struct {
	CallTool      float64 // Percentage of CallTool operations
	ReadResource  float64 // Percentage of ReadResource operations
	ListTools     float64 // Percentage of ListTools operations
	ListResources float64 // Percentage of ListResources operations
}

// LoadTestResult contains the results of a load test
type LoadTestResult struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalDuration      time.Duration

	// Latency statistics (in milliseconds)
	MinLatency float64
	MaxLatency float64
	AvgLatency float64
	P50Latency float64
	P90Latency float64
	P95Latency float64
	P99Latency float64

	// Throughput
	RequestsPerSecond float64

	// Error breakdown
	ErrorCounts map[string]int64

	// Operation-specific metrics
	OperationMetrics map[string]*OperationMetrics
}

// OperationMetrics tracks metrics for a specific operation type
type OperationMetrics struct {
	Count      int64
	Successful int64
	Failed     int64
	TotalTime  time.Duration
	MinTime    time.Duration
	MaxTime    time.Duration

	mu        sync.Mutex
	latencies []time.Duration
}

// LoadTester performs load testing on MCP servers
type LoadTester struct {
	config LoadTestConfig
	// server *server.Server // Reserved for future use

	// Metrics
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	errorCounts        sync.Map
	operationMetrics   sync.Map

	// Control
	startTime time.Time
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewLoadTester creates a new load tester
func NewLoadTester(config LoadTestConfig) *LoadTester {
	// Set defaults
	if config.ReportInterval == 0 {
		config.ReportInterval = 5 * time.Second
	}

	// Normalize operation mix
	total := config.OperationMix.CallTool + config.OperationMix.ReadResource +
		config.OperationMix.ListTools + config.OperationMix.ListResources
	if total == 0 {
		// Default mix
		config.OperationMix = OperationMix{
			CallTool:      40,
			ReadResource:  30,
			ListTools:     20,
			ListResources: 10,
		}
		total = 100
	}

	// Normalize to percentages
	config.OperationMix.CallTool /= total
	config.OperationMix.ReadResource /= total
	config.OperationMix.ListTools /= total
	config.OperationMix.ListResources /= total

	return &LoadTester{
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Run executes the load test
func (lt *LoadTester) Run(ctx context.Context) (*LoadTestResult, error) {
	lt.startTime = time.Now()

	// Start reporting goroutine
	go lt.reportProgress()

	// Create clients
	clients := make([]client.Client, lt.config.Clients)
	for i := 0; i < lt.config.Clients; i++ {
		c, err := lt.createClient(i)
		if err != nil {
			return nil, fmt.Errorf("failed to create client %d: %w", i, err)
		}
		clients[i] = c

		// Initialize client
		if err := c.Initialize(ctx); err != nil {
			return nil, fmt.Errorf("failed to initialize client %d: %w", i, err)
		}
	}

	// Start load generation
	rateLimiter := lt.createRateLimiter()

	for i, c := range clients {
		lt.wg.Add(1)
		go lt.runClient(ctx, i, c, rateLimiter)

		// Ramp up delay
		if lt.config.RampUpTime > 0 && i < len(clients)-1 {
			time.Sleep(lt.config.RampUpTime / time.Duration(len(clients)-1))
		}
	}

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		lt.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All clients completed
	case <-time.After(lt.config.Duration):
		// Duration expired
		close(lt.stopCh)
		lt.wg.Wait()
	case <-ctx.Done():
		// Context cancelled
		close(lt.stopCh)
		lt.wg.Wait()
	}

	// Calculate results
	return lt.calculateResults(), nil
}

// runClient runs a single client's workload
func (lt *LoadTester) runClient(ctx context.Context, id int, c client.Client, rateLimiter <-chan struct{}) {
	defer lt.wg.Done()

	requestCount := 0
	for {
		select {
		case <-lt.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// Check if we've reached the request limit
			if lt.config.RequestsPerClient > 0 && requestCount >= lt.config.RequestsPerClient {
				return
			}

			// Rate limiting
			if rateLimiter != nil {
				<-rateLimiter
			}

			// Select operation based on mix
			op := lt.selectOperation()
			lt.executeOperation(ctx, c, op)

			requestCount++
		}
	}
}

// selectOperation chooses an operation based on the configured mix
func (lt *LoadTester) selectOperation() string {
	r := randomFloat()

	if r < lt.config.OperationMix.CallTool {
		return "CallTool"
	} else if r < lt.config.OperationMix.CallTool+lt.config.OperationMix.ReadResource {
		return "ReadResource"
	} else if r < lt.config.OperationMix.CallTool+lt.config.OperationMix.ReadResource+lt.config.OperationMix.ListTools {
		return "ListTools"
	} else {
		return "ListResources"
	}
}

// executeOperation performs a single operation and records metrics
func (lt *LoadTester) executeOperation(ctx context.Context, c client.Client, operation string) {
	start := time.Now()
	var err error

	atomic.AddInt64(&lt.totalRequests, 1)

	switch operation {
	case "CallTool":
		_, err = c.CallTool(ctx, "test_tool", map[string]interface{}{
			"input": fmt.Sprintf("test-%d", time.Now().UnixNano()),
		}, nil)

	case "ReadResource":
		_, err = c.ReadResource(ctx, "test://resource/1", nil, nil)

	case "ListTools":
		_, _, err = c.ListTools(ctx, "", &protocol.PaginationParams{Limit: 10})

	case "ListResources":
		_, _, _, err = c.ListResources(ctx, "", false, &protocol.PaginationParams{Limit: 10})
	}

	duration := time.Since(start)

	// Update metrics
	metrics := lt.getOperationMetrics(operation)
	metrics.recordOperation(duration, err)

	if err != nil {
		atomic.AddInt64(&lt.failedRequests, 1)
		lt.recordError(err)
	} else {
		atomic.AddInt64(&lt.successfulRequests, 1)
	}
}

// getOperationMetrics returns metrics for a specific operation
func (lt *LoadTester) getOperationMetrics(operation string) *OperationMetrics {
	v, _ := lt.operationMetrics.LoadOrStore(operation, &OperationMetrics{})
	metrics, _ := v.(*OperationMetrics)
	return metrics
}

// recordOperation records a single operation's metrics
func (m *OperationMetrics) recordOperation(duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Count++
	m.TotalTime += duration

	if err != nil {
		m.Failed++
	} else {
		m.Successful++
	}

	if m.MinTime == 0 || duration < m.MinTime {
		m.MinTime = duration
	}
	if duration > m.MaxTime {
		m.MaxTime = duration
	}

	m.latencies = append(m.latencies, duration)
}

// recordError records an error occurrence
func (lt *LoadTester) recordError(err error) {
	errStr := err.Error()
	if v, loaded := lt.errorCounts.LoadOrStore(errStr, int64(1)); loaded {
		count, _ := v.(int64)
		lt.errorCounts.Store(errStr, count+1)
	}
}

// createRateLimiter creates a rate limiter channel
func (lt *LoadTester) createRateLimiter() <-chan struct{} {
	if lt.config.RateLimit <= 0 {
		return nil
	}

	ch := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(lt.config.RateLimit))
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				select {
				case ch <- struct{}{}:
				case <-lt.stopCh:
					return
				}
			case <-lt.stopCh:
				return
			}
		}
	}()

	return ch
}

// reportProgress periodically reports test progress
func (lt *LoadTester) reportProgress() {
	ticker := time.NewTicker(lt.config.ReportInterval)
	defer ticker.Stop()

	lastRequests := int64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ticker.C:
			currentRequests := atomic.LoadInt64(&lt.totalRequests)
			currentTime := time.Now()

			elapsed := currentTime.Sub(lastTime).Seconds()
			rps := float64(currentRequests-lastRequests) / elapsed

			successful := atomic.LoadInt64(&lt.successfulRequests)
			failed := atomic.LoadInt64(&lt.failedRequests)

			log.Printf("Progress: %d requests (%.1f req/s), %d successful, %d failed",
				currentRequests, rps, successful, failed)

			lastRequests = currentRequests
			lastTime = currentTime

		case <-lt.stopCh:
			return
		}
	}
}

// calculateResults computes the final test results
func (lt *LoadTester) calculateResults() *LoadTestResult {
	duration := time.Since(lt.startTime)

	result := &LoadTestResult{
		TotalRequests:      atomic.LoadInt64(&lt.totalRequests),
		SuccessfulRequests: atomic.LoadInt64(&lt.successfulRequests),
		FailedRequests:     atomic.LoadInt64(&lt.failedRequests),
		TotalDuration:      duration,
		RequestsPerSecond:  float64(atomic.LoadInt64(&lt.totalRequests)) / duration.Seconds(),
		ErrorCounts:        make(map[string]int64),
		OperationMetrics:   make(map[string]*OperationMetrics),
	}

	// Collect error counts
	lt.errorCounts.Range(func(key, value interface{}) bool {
		errStr, _ := key.(string)
		count, _ := value.(int64)
		result.ErrorCounts[errStr] = count
		return true
	})

	// Collect operation metrics and calculate overall latencies
	var allLatencies []time.Duration
	lt.operationMetrics.Range(func(key, value interface{}) bool {
		opName, _ := key.(string)
		metrics, _ := value.(*OperationMetrics)

		result.OperationMetrics[opName] = metrics
		allLatencies = append(allLatencies, metrics.latencies...)

		return true
	})

	// Calculate latency statistics
	if len(allLatencies) > 0 {
		result.MinLatency = float64(minDuration(allLatencies).Milliseconds())
		result.MaxLatency = float64(maxDuration(allLatencies).Milliseconds())
		result.AvgLatency = float64(avgDuration(allLatencies).Milliseconds())

		sortDurations(allLatencies)
		result.P50Latency = float64(percentileDuration(allLatencies, 50).Milliseconds())
		result.P90Latency = float64(percentileDuration(allLatencies, 90).Milliseconds())
		result.P95Latency = float64(percentileDuration(allLatencies, 95).Milliseconds())
		result.P99Latency = float64(percentileDuration(allLatencies, 99).Milliseconds())
	}

	return result
}

// createClient creates a test client
func (lt *LoadTester) createClient(id int) (client.Client, error) {
	config := transport.DefaultTransportConfig(lt.config.TransportType)
	if lt.config.Endpoint != "" {
		config.Endpoint = lt.config.Endpoint
	}

	t, err := transport.NewTransport(config)
	if err != nil {
		return nil, err
	}

	return client.New(t,
		client.WithName(fmt.Sprintf("load-test-client-%d", id)),
		client.WithVersion("1.0.0"),
	), nil
}

// Helper functions for statistics

func minDuration(durations []time.Duration) time.Duration {
	min := durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
	}
	return min
}

func maxDuration(durations []time.Duration) time.Duration {
	max := durations[0]
	for _, d := range durations[1:] {
		if d > max {
			max = d
		}
	}
	return max
}

func avgDuration(durations []time.Duration) time.Duration {
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func sortDurations(durations []time.Duration) {
	// Simple insertion sort for demonstration
	for i := 1; i < len(durations); i++ {
		key := durations[i]
		j := i - 1
		for j >= 0 && durations[j] > key {
			durations[j+1] = durations[j]
			j--
		}
		durations[j+1] = key
	}
}

func percentileDuration(sortedDurations []time.Duration, percentile float64) time.Duration {
	index := int(math.Ceil(float64(len(sortedDurations))*percentile/100.0)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sortedDurations) {
		index = len(sortedDurations) - 1
	}
	return sortedDurations[index]
}

func randomFloat() float64 {
	return float64(time.Now().UnixNano()%1000000) / 1000000.0
}

// PrintResults prints load test results in a readable format
func (r *LoadTestResult) PrintResults() {
	fmt.Println("\n=== Load Test Results ===")
	fmt.Printf("Total Duration: %s\n", r.TotalDuration)
	fmt.Printf("Total Requests: %d\n", r.TotalRequests)
	fmt.Printf("Successful: %d (%.1f%%)\n", r.SuccessfulRequests,
		float64(r.SuccessfulRequests)/float64(r.TotalRequests)*100)
	fmt.Printf("Failed: %d (%.1f%%)\n", r.FailedRequests,
		float64(r.FailedRequests)/float64(r.TotalRequests)*100)
	fmt.Printf("Requests/sec: %.2f\n", r.RequestsPerSecond)

	fmt.Println("\nLatency Statistics (ms):")
	fmt.Printf("  Min: %.2f\n", r.MinLatency)
	fmt.Printf("  Avg: %.2f\n", r.AvgLatency)
	fmt.Printf("  P50: %.2f\n", r.P50Latency)
	fmt.Printf("  P90: %.2f\n", r.P90Latency)
	fmt.Printf("  P95: %.2f\n", r.P95Latency)
	fmt.Printf("  P99: %.2f\n", r.P99Latency)
	fmt.Printf("  Max: %.2f\n", r.MaxLatency)

	if len(r.OperationMetrics) > 0 {
		fmt.Println("\nOperation Breakdown:")
		for op, metrics := range r.OperationMetrics {
			fmt.Printf("  %s:\n", op)
			fmt.Printf("    Count: %d\n", metrics.Count)
			fmt.Printf("    Success Rate: %.1f%%\n",
				float64(metrics.Successful)/float64(metrics.Count)*100)
			fmt.Printf("    Avg Time: %.2fms\n",
				float64(metrics.TotalTime.Milliseconds())/float64(metrics.Count))
		}
	}

	if len(r.ErrorCounts) > 0 {
		fmt.Println("\nError Summary:")
		for err, count := range r.ErrorCounts {
			fmt.Printf("  %s: %d\n", err, count)
		}
	}
}
