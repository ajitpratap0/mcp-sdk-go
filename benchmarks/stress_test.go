// Package benchmarks provides stress testing with connection failures
package benchmarks

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/client"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// StressTestConfig configures stress testing parameters
type StressTestConfig struct {
	// Basic configuration
	Clients           int
	RequestsPerClient int
	Duration          time.Duration

	// Failure injection
	ConnectionFailureRate float64 // 0.0 to 1.0
	NetworkLatency        time.Duration
	NetworkJitter         time.Duration
	PacketLossRate        float64 // 0.0 to 1.0

	// Chaos scenarios
	EnableServerCrash      bool
	ServerCrashInterval    time.Duration
	EnableNetworkPartition bool
	PartitionDuration      time.Duration

	// Recovery testing
	EnableAutoReconnect  bool
	ReconnectDelay       time.Duration
	MaxReconnectAttempts int

	// Resource limits
	MaxMemoryMB        int
	MaxGoroutines      int
	MaxFileDescriptors int
}

// StressTestResult contains stress test results
type StressTestResult struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	ConnectionFailures int64
	TimeoutErrors      int64
	RecoverySuccesses  int64
	RecoveryFailures   int64

	// Performance under stress
	MinLatency time.Duration
	MaxLatency time.Duration
	AvgLatency time.Duration
	P99Latency time.Duration

	// Resource usage
	MaxMemoryUsed      int64
	MaxGoroutines      int
	MaxFileDescriptors int

	// Failure analysis
	FailureTypes    map[string]int64
	RecoveryTimes   []time.Duration
	ConnectionDrops int64
	ServerCrashes   int64
}

// StressTester performs stress testing with failure injection
type StressTester struct {
	config StressTestConfig
	server *failureInjectingServer

	// Metrics
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	connectionFailures int64
	timeoutErrors      int64
	recoverySuccesses  int64
	recoveryFailures   int64

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewStressTester creates a new stress tester
func NewStressTester(config StressTestConfig) *StressTester {
	return &StressTester{
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Run executes the stress test
func (st *StressTester) Run(ctx context.Context) (*StressTestResult, error) {
	// Start failure-injecting server
	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	st.server = newFailureInjectingServer(st.config)
	if err := st.server.Start(serverCtx); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	// Start chaos scenarios if enabled
	if st.config.EnableServerCrash {
		go st.runServerCrashScenario(serverCtx)
	}
	if st.config.EnableNetworkPartition {
		go st.runNetworkPartitionScenario(serverCtx)
	}

	// Create and start clients
	clients := make([]*stressClient, st.config.Clients)
	for i := 0; i < st.config.Clients; i++ {
		c := st.createStressClient(i)
		clients[i] = c
		st.wg.Add(1)
		go st.runStressClient(ctx, c)
	}

	// Run for specified duration or until context cancelled
	done := make(chan struct{})
	go func() {
		st.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All clients completed
	case <-time.After(st.config.Duration):
		// Duration expired
		close(st.stopCh)
		st.wg.Wait()
	case <-ctx.Done():
		// Context cancelled
		close(st.stopCh)
		st.wg.Wait()
	}

	// Collect results
	return st.collectResults(), nil
}

// runStressClient runs a single stress test client
func (st *StressTester) runStressClient(ctx context.Context, sc *stressClient) {
	defer st.wg.Done()

	requestCount := 0
	for {
		select {
		case <-st.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			if st.config.RequestsPerClient > 0 && requestCount >= st.config.RequestsPerClient {
				return
			}

			// Inject connection failure
			if st.shouldInjectFailure() {
				atomic.AddInt64(&st.connectionFailures, 1)
				sc.simulateConnectionFailure()

				// Attempt recovery if enabled
				if st.config.EnableAutoReconnect {
					if err := st.attemptRecovery(ctx, sc); err != nil {
						atomic.AddInt64(&st.recoveryFailures, 1)
						log.Printf("Recovery failed: %v", err)
					} else {
						atomic.AddInt64(&st.recoverySuccesses, 1)
					}
				}
				continue
			}

			// Perform normal operation
			atomic.AddInt64(&st.totalRequests, 1)
			err := st.performOperation(ctx, sc)
			if err != nil {
				atomic.AddInt64(&st.failedRequests, 1)
				st.categorizeError(err)
			} else {
				atomic.AddInt64(&st.successfulRequests, 1)
			}

			requestCount++
		}
	}
}

// performOperation performs a random MCP operation
func (st *StressTester) performOperation(ctx context.Context, sc *stressClient) error {
	// Add network latency and jitter
	if st.config.NetworkLatency > 0 {
		jitter := time.Duration(0)
		if st.config.NetworkJitter > 0 {
			jitter = time.Duration(rand.Int63n(int64(st.config.NetworkJitter)))
		}
		time.Sleep(st.config.NetworkLatency + jitter)
	}

	// Simulate packet loss
	if st.config.PacketLossRate > 0 && rand.Float64() < st.config.PacketLossRate {
		return fmt.Errorf("packet loss simulated")
	}

	// Perform random operation
	op := rand.Intn(4)
	switch op {
	case 0:
		return sc.callTool(ctx)
	case 1:
		return sc.readResource(ctx)
	case 2:
		return sc.listTools(ctx)
	case 3:
		return sc.sendBatch(ctx)
	default:
		return sc.callTool(ctx)
	}
}

// shouldInjectFailure determines if a failure should be injected
func (st *StressTester) shouldInjectFailure() bool {
	return rand.Float64() < st.config.ConnectionFailureRate
}

// attemptRecovery attempts to recover from a connection failure
func (st *StressTester) attemptRecovery(ctx context.Context, sc *stressClient) error {
	for attempt := 0; attempt < st.config.MaxReconnectAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(st.config.ReconnectDelay):
			if err := sc.reconnect(ctx); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("max reconnection attempts exceeded")
}

// categorizeError categorizes different error types
func (st *StressTester) categorizeError(err error) {
	if err == nil {
		return
	}

	errStr := err.Error()
	switch {
	case containsString(errStr, "timeout"):
		atomic.AddInt64(&st.timeoutErrors, 1)
	case containsString(errStr, "connection"):
		atomic.AddInt64(&st.connectionFailures, 1)
	}
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// runServerCrashScenario periodically crashes and restarts the server
func (st *StressTester) runServerCrashScenario(ctx context.Context) {
	ticker := time.NewTicker(st.config.ServerCrashInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Simulating server crash...")
			st.server.simulateCrash()
			time.Sleep(time.Second) // Brief downtime
			st.server.restart()
			log.Println("Server restarted")
		}
	}
}

// runNetworkPartitionScenario simulates network partitions
func (st *StressTester) runNetworkPartitionScenario(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(st.config.PartitionDuration * 2):
			log.Println("Simulating network partition...")
			st.server.enableNetworkPartition()
			time.Sleep(st.config.PartitionDuration)
			st.server.disableNetworkPartition()
			log.Println("Network partition resolved")
		}
	}
}

// collectResults collects stress test results
func (st *StressTester) collectResults() *StressTestResult {
	return &StressTestResult{
		TotalRequests:      atomic.LoadInt64(&st.totalRequests),
		SuccessfulRequests: atomic.LoadInt64(&st.successfulRequests),
		FailedRequests:     atomic.LoadInt64(&st.failedRequests),
		ConnectionFailures: atomic.LoadInt64(&st.connectionFailures),
		TimeoutErrors:      atomic.LoadInt64(&st.timeoutErrors),
		RecoverySuccesses:  atomic.LoadInt64(&st.recoverySuccesses),
		RecoveryFailures:   atomic.LoadInt64(&st.recoveryFailures),
		ConnectionDrops:    st.server.getConnectionDrops(),
		ServerCrashes:      st.server.getCrashCount(),
	}
}

// createStressClient creates a client for stress testing
func (st *StressTester) createStressClient(id int) *stressClient {
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = st.server.endpoint
	// Configure retry settings via the connection config
	if st.config.EnableAutoReconnect {
		// MaxRetries would be configured on Reliability config
		config.Reliability.MaxRetries = st.config.MaxReconnectAttempts
	}

	return &stressClient{
		id:     id,
		config: &config,
	}
}

// stressClient represents a client used for stress testing
type stressClient struct {
	id        int
	config    *transport.TransportConfig
	transport transport.Transport
	client    client.Client
	mu        sync.Mutex
}

func (sc *stressClient) initialize(ctx context.Context) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	t, err := transport.NewTransport(*sc.config)
	if err != nil {
		return err
	}

	sc.transport = t
	sc.client = client.New(t,
		client.WithName(fmt.Sprintf("stress-client-%d", sc.id)),
		client.WithVersion("1.0.0"),
	)

	return sc.client.Initialize(ctx)
}

func (sc *stressClient) simulateConnectionFailure() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.transport != nil {
		sc.transport.Stop(context.Background())
		sc.transport = nil
		sc.client = nil
	}
}

func (sc *stressClient) reconnect(ctx context.Context) error {
	return sc.initialize(ctx)
}

func (sc *stressClient) callTool(ctx context.Context) error {
	sc.mu.Lock()
	c := sc.client
	sc.mu.Unlock()

	if c == nil {
		return fmt.Errorf("client not initialized")
	}

	_, err := c.CallTool(ctx, "stress_tool", map[string]interface{}{
		"input": fmt.Sprintf("stress-%d", time.Now().UnixNano()),
	}, nil)
	return err
}

func (sc *stressClient) readResource(ctx context.Context) error {
	sc.mu.Lock()
	c := sc.client
	sc.mu.Unlock()

	if c == nil {
		return fmt.Errorf("client not initialized")
	}

	_, err := c.ReadResource(ctx, "stress://resource/1", nil, nil)
	return err
}

func (sc *stressClient) listTools(ctx context.Context) error {
	sc.mu.Lock()
	c := sc.client
	sc.mu.Unlock()

	if c == nil {
		return fmt.Errorf("client not initialized")
	}

	_, _, err := c.ListTools(ctx, "", &protocol.PaginationParams{Limit: 10})
	return err
}

func (sc *stressClient) sendBatch(ctx context.Context) error {
	sc.mu.Lock()
	t := sc.transport
	sc.mu.Unlock()

	if t == nil {
		return fmt.Errorf("transport not initialized")
	}

	batch := &protocol.JSONRPCBatchRequest{}
	for i := 0; i < 5; i++ {
		req, _ := protocol.NewRequest(fmt.Sprintf("stress-%d", i), "listTools", nil)
		*batch = append(*batch, req)
	}

	_, err := t.SendBatchRequest(ctx, batch)
	return err
}

// failureInjectingServer simulates a server with various failure modes
type failureInjectingServer struct {
	endpoint        string
	server          *server.Server
	transport       transport.Transport
	crashed         atomic.Bool
	partitioned     atomic.Bool
	connectionDrops int64
	crashCount      int64
	mu              sync.Mutex
}

func newFailureInjectingServer(config StressTestConfig) *failureInjectingServer {
	return &failureInjectingServer{
		endpoint: "http://localhost:8089/mcp",
	}
}

func (s *failureInjectingServer) Start(ctx context.Context) error {
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)
	config.Endpoint = s.endpoint

	t, err := transport.NewTransport(config)
	if err != nil {
		return err
	}

	// Wrap with failure injection
	s.transport = &failureInjectingTransport{
		Transport: t,
		server:    s,
	}

	s.server = server.New(s.transport)
	return s.server.Start(ctx)
}

func (s *failureInjectingServer) simulateCrash() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.crashed.Store(true)
	atomic.AddInt64(&s.crashCount, 1)
}

func (s *failureInjectingServer) restart() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.crashed.Store(false)
}

func (s *failureInjectingServer) enableNetworkPartition() {
	s.partitioned.Store(true)
}

func (s *failureInjectingServer) disableNetworkPartition() {
	s.partitioned.Store(false)
}

func (s *failureInjectingServer) getConnectionDrops() int64 {
	return atomic.LoadInt64(&s.connectionDrops)
}

func (s *failureInjectingServer) getCrashCount() int64 {
	return atomic.LoadInt64(&s.crashCount)
}

// failureInjectingTransport wraps a transport to inject failures
type failureInjectingTransport struct {
	transport.Transport
	server *failureInjectingServer
}

func (t *failureInjectingTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if t.server.crashed.Load() {
		return nil, fmt.Errorf("server crashed")
	}
	if t.server.partitioned.Load() {
		return nil, fmt.Errorf("network partition")
	}
	return t.Transport.SendRequest(ctx, method, params)
}

func (t *failureInjectingTransport) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	if t.server.crashed.Load() {
		return nil, fmt.Errorf("server crashed")
	}
	if t.server.partitioned.Load() {
		atomic.AddInt64(&t.server.connectionDrops, 1)
		return nil, fmt.Errorf("network partition")
	}
	return t.Transport.HandleRequest(ctx, request)
}

// Stress test functions

func TestStressWithConnectionFailures(t *testing.T) {
	config := StressTestConfig{
		Clients:           10,
		RequestsPerClient: 100,
		Duration:          30 * time.Second,

		// Failure injection
		ConnectionFailureRate: 0.1, // 10% failure rate
		NetworkLatency:        50 * time.Millisecond,
		NetworkJitter:         20 * time.Millisecond,
		PacketLossRate:        0.05, // 5% packet loss

		// Recovery
		EnableAutoReconnect:  true,
		ReconnectDelay:       time.Second,
		MaxReconnectAttempts: 3,
	}

	tester := NewStressTester(config)
	ctx := context.Background()

	result, err := tester.Run(ctx)
	if err != nil {
		t.Fatalf("Stress test failed: %v", err)
	}

	// Analyze results
	successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests)
	recoveryRate := float64(result.RecoverySuccesses) / float64(result.RecoverySuccesses+result.RecoveryFailures)

	t.Logf("Stress Test Results:")
	t.Logf("  Total Requests: %d", result.TotalRequests)
	t.Logf("  Success Rate: %.2f%%", successRate*100)
	t.Logf("  Connection Failures: %d", result.ConnectionFailures)
	t.Logf("  Recovery Rate: %.2f%%", recoveryRate*100)

	// Ensure reasonable success rate under stress
	if successRate < 0.8 {
		t.Errorf("Success rate too low under stress: %.2f%%", successRate*100)
	}
}

func TestStressWithServerCrashes(t *testing.T) {
	config := StressTestConfig{
		Clients:  5,
		Duration: 30 * time.Second,

		// Chaos scenarios
		EnableServerCrash:   true,
		ServerCrashInterval: 5 * time.Second,

		// Recovery
		EnableAutoReconnect:  true,
		ReconnectDelay:       500 * time.Millisecond,
		MaxReconnectAttempts: 5,
	}

	tester := NewStressTester(config)
	ctx := context.Background()

	result, err := tester.Run(ctx)
	if err != nil {
		t.Fatalf("Stress test failed: %v", err)
	}

	t.Logf("Server Crash Test Results:")
	t.Logf("  Server Crashes: %d", result.ServerCrashes)
	t.Logf("  Recovery Successes: %d", result.RecoverySuccesses)
	t.Logf("  Recovery Failures: %d", result.RecoveryFailures)

	// Ensure clients can recover from crashes
	if result.RecoverySuccesses == 0 {
		t.Error("No successful recoveries from server crashes")
	}
}

func TestStressWithNetworkPartitions(t *testing.T) {
	config := StressTestConfig{
		Clients:  5,
		Duration: 30 * time.Second,

		// Network partition
		EnableNetworkPartition: true,
		PartitionDuration:      3 * time.Second,

		// High latency and jitter
		NetworkLatency: 100 * time.Millisecond,
		NetworkJitter:  50 * time.Millisecond,

		// Recovery
		EnableAutoReconnect:  true,
		ReconnectDelay:       time.Second,
		MaxReconnectAttempts: 3,
	}

	tester := NewStressTester(config)
	ctx := context.Background()

	result, err := tester.Run(ctx)
	if err != nil {
		t.Fatalf("Stress test failed: %v", err)
	}

	t.Logf("Network Partition Test Results:")
	t.Logf("  Connection Drops: %d", result.ConnectionDrops)
	t.Logf("  Timeout Errors: %d", result.TimeoutErrors)
	t.Logf("  Recovery Rate: %.2f%%",
		float64(result.RecoverySuccesses)/float64(result.RecoverySuccesses+result.RecoveryFailures)*100)
}
