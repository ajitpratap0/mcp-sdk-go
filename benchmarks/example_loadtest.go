//go:build ignore
// +build ignore

// Example load test runner
// Run with: go run example_loadtest.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/benchmarks"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

func main() {
	fmt.Println("=== MCP SDK Load Test Example ===")

	// Example 1: Light load test
	fmt.Println("\n1. Running light load test (10 clients, 100 requests each)...")
	runLightLoadTest()

	// Example 2: Medium load test
	fmt.Println("\n2. Running medium load test (50 clients, 500 requests each)...")
	runMediumLoadTest()

	// Example 3: Stress test
	fmt.Println("\n3. Running stress test (100 clients, unlimited rate)...")
	runStressTest()

	// Example 4: Rate-limited test
	fmt.Println("\n4. Running rate-limited test (25 clients, 100 req/s)...")
	runRateLimitedTest()
}

func runLightLoadTest() {
	config := benchmarks.LoadTestConfig{
		Clients:           10,
		RequestsPerClient: 100,
		RateLimit:         0, // Unlimited
		Duration:          30 * time.Second,
		RampUpTime:        2 * time.Second,
		OperationMix: benchmarks.OperationMix{
			CallTool:      50, // 50%
			ReadResource:  30, // 30%
			ListTools:     15, // 15%
			ListResources: 5,  // 5%
		},
		TransportType:  transport.TransportTypeStdio,
		ReportInterval: 2 * time.Second,
	}

	tester := benchmarks.NewLoadTester(config)
	result, err := tester.Run(context.Background())
	if err != nil {
		log.Fatalf("Load test failed: %v", err)
	}

	result.PrintResults()
}

func runMediumLoadTest() {
	config := benchmarks.LoadTestConfig{
		Clients:           50,
		RequestsPerClient: 500,
		RateLimit:         0,
		Duration:          60 * time.Second,
		RampUpTime:        5 * time.Second,
		OperationMix: benchmarks.OperationMix{
			CallTool:      40,
			ReadResource:  35,
			ListTools:     20,
			ListResources: 5,
		},
		TransportType:  transport.TransportTypeStdio,
		ReportInterval: 5 * time.Second,
	}

	tester := benchmarks.NewLoadTester(config)
	result, err := tester.Run(context.Background())
	if err != nil {
		log.Fatalf("Load test failed: %v", err)
	}

	result.PrintResults()
}

func runStressTest() {
	config := benchmarks.LoadTestConfig{
		Clients:           100,
		RequestsPerClient: 0, // Unlimited
		RateLimit:         0, // Unlimited
		Duration:          30 * time.Second,
		RampUpTime:        10 * time.Second,
		OperationMix: benchmarks.OperationMix{
			CallTool:      60,
			ReadResource:  25,
			ListTools:     10,
			ListResources: 5,
		},
		TransportType:  transport.TransportTypeStdio,
		ReportInterval: 3 * time.Second,
	}

	tester := benchmarks.NewLoadTester(config)
	result, err := tester.Run(context.Background())
	if err != nil {
		log.Fatalf("Stress test failed: %v", err)
	}

	result.PrintResults()

	// Check if we achieved good performance under stress
	if result.RequestsPerSecond < 1000 {
		fmt.Printf("WARNING: Low throughput under stress: %.2f req/s\n", result.RequestsPerSecond)
	}
	if result.P95Latency > 100 {
		fmt.Printf("WARNING: High P95 latency under stress: %.2fms\n", result.P95Latency)
	}
}

func runRateLimitedTest() {
	config := benchmarks.LoadTestConfig{
		Clients:           25,
		RequestsPerClient: 200,
		RateLimit:         100, // 100 requests per second
		Duration:          60 * time.Second,
		RampUpTime:        3 * time.Second,
		OperationMix: benchmarks.OperationMix{
			CallTool:      45,
			ReadResource:  35,
			ListTools:     15,
			ListResources: 5,
		},
		TransportType:  transport.TransportTypeStdio,
		ReportInterval: 5 * time.Second,
	}

	tester := benchmarks.NewLoadTester(config)
	result, err := tester.Run(context.Background())
	if err != nil {
		log.Fatalf("Rate-limited test failed: %v", err)
	}

	result.PrintResults()

	// Verify rate limiting worked
	expectedRPS := float64(config.RateLimit)
	tolerance := 0.1 // 10% tolerance
	if result.RequestsPerSecond > expectedRPS*(1+tolerance) {
		fmt.Printf("WARNING: Rate limiting may not be working correctly. Expected ~%.0f req/s, got %.2f req/s\n",
			expectedRPS, result.RequestsPerSecond)
	}
}
