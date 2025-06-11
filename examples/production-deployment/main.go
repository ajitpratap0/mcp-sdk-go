// Production-ready MCP server with comprehensive monitoring, health checks, and graceful shutdown
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/examples/shared"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/observability"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/server"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// ProductionConfig holds all configuration for the production server
type ProductionConfig struct {
	// Server configuration
	ServerName    string `json:"server_name"`
	ServerVersion string `json:"server_version"`
	Port          int    `json:"port"`
	Host          string `json:"host"`

	// Health check configuration
	HealthCheckPort int    `json:"health_check_port"`
	HealthPath      string `json:"health_path"`
	ReadinessPath   string `json:"readiness_path"`

	// Metrics configuration
	MetricsPort int    `json:"metrics_port"`
	MetricsPath string `json:"metrics_path"`

	// Observability configuration
	EnableTracing    bool   `json:"enable_tracing"`
	TracingEndpoint  string `json:"tracing_endpoint"`
	EnableMetrics    bool   `json:"enable_metrics"`
	ServiceName      string `json:"service_name"`
	Environment      string `json:"environment"`
	DeploymentRegion string `json:"deployment_region"`
	DeploymentZone   string `json:"deployment_zone"`

	// Security configuration
	AllowedOrigins  []string `json:"allowed_origins"`
	EnableAuth      bool     `json:"enable_auth"`
	AuthTokens      []string `json:"auth_tokens"`
	EnableRateLimit bool     `json:"enable_rate_limit"`
	RateLimit       int      `json:"rate_limit"`

	// Graceful shutdown configuration
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// LoadConfig loads configuration from environment variables with sensible defaults
func LoadConfig() *ProductionConfig {
	config := &ProductionConfig{
		// Defaults
		ServerName:       getEnv("MCP_SERVER_NAME", "production-mcp-server"),
		ServerVersion:    getEnv("MCP_SERVER_VERSION", "1.0.0"),
		Port:             getEnvInt("MCP_PORT", 8080),
		Host:             getEnv("MCP_HOST", "0.0.0.0"),
		HealthCheckPort:  getEnvInt("MCP_HEALTH_PORT", 8081),
		HealthPath:       getEnv("MCP_HEALTH_PATH", "/health"),
		ReadinessPath:    getEnv("MCP_READINESS_PATH", "/ready"),
		MetricsPort:      getEnvInt("MCP_METRICS_PORT", 9090),
		MetricsPath:      getEnv("MCP_METRICS_PATH", "/metrics"),
		EnableTracing:    getEnvBool("MCP_ENABLE_TRACING", true),
		TracingEndpoint:  getEnv("MCP_TRACING_ENDPOINT", "http://jaeger:14268/api/traces"),
		EnableMetrics:    getEnvBool("MCP_ENABLE_METRICS", true),
		ServiceName:      getEnv("MCP_SERVICE_NAME", "mcp-production-server"),
		Environment:      getEnv("MCP_ENVIRONMENT", "production"),
		DeploymentRegion: getEnv("MCP_DEPLOYMENT_REGION", "us-west-2"),
		DeploymentZone:   getEnv("MCP_DEPLOYMENT_ZONE", "us-west-2a"),
		AllowedOrigins:   getEnvStringSlice("MCP_ALLOWED_ORIGINS", []string{"*"}),
		EnableAuth:       getEnvBool("MCP_ENABLE_AUTH", false),
		AuthTokens:       getEnvStringSlice("MCP_AUTH_TOKENS", []string{}),
		EnableRateLimit:  getEnvBool("MCP_ENABLE_RATE_LIMIT", true),
		RateLimit:        getEnvInt("MCP_RATE_LIMIT", 1000),
		ShutdownTimeout:  getEnvDuration("MCP_SHUTDOWN_TIMEOUT", 30*time.Second),
	}

	return config
}

// ProductionServer wraps the MCP server with production concerns
type ProductionServer struct {
	config        *ProductionConfig
	mcpServer     *server.Server
	healthServer  *http.Server
	metricsServer *http.Server
	middleware    transport.Middleware
	ready         bool
	readyMutex    sync.RWMutex
}

// NewProductionServer creates a new production-ready MCP server
func NewProductionServer(config *ProductionConfig) (*ProductionServer, error) {
	ps := &ProductionServer{
		config: config,
		ready:  false,
	}

	// Create observability middleware
	if err := ps.setupObservability(); err != nil {
		return nil, fmt.Errorf("failed to setup observability: %w", err)
	}

	// Create transport with security and performance configurations
	transport, err := ps.createTransport()
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Apply middleware
	if ps.middleware != nil {
		transport = ps.middleware.Wrap(transport)
	}

	// Create MCP server with providers
	ps.mcpServer = server.New(transport,
		server.WithName(config.ServerName),
		server.WithVersion(config.ServerVersion),
		server.WithToolsProvider(shared.CreateToolsProvider()),
		server.WithResourcesProvider(shared.CreateResourcesProvider()),
		server.WithPromptsProvider(shared.CreatePromptsProvider()),
	)

	// Setup health check server
	ps.setupHealthServer()

	// Setup metrics server
	ps.setupMetricsServer()

	return ps, nil
}

// setupObservability configures tracing and metrics
func (ps *ProductionServer) setupObservability() error {
	if !ps.config.EnableTracing && !ps.config.EnableMetrics {
		return nil
	}

	// Configure observability
	obsConfig := observability.ObservabilityConfig{
		EnableTracing: ps.config.EnableTracing,
		EnableMetrics: ps.config.EnableMetrics,
		EnableLogging: true,
		LogLevel:      "info",
	}

	if ps.config.EnableTracing {
		obsConfig.TracingConfig = observability.TracingConfig{
			ServiceName:    ps.config.ServiceName,
			ServiceVersion: ps.config.ServerVersion,
			Environment:    ps.config.Environment,
			ExporterType:   observability.ExporterTypeOTLPHTTP,
			Endpoint:       ps.config.TracingEndpoint,
			Insecure:       true,
			SampleRate:     1.0,
			AlwaysSample:   []string{"callTool", "readResource", "listTools", "listResources"},
			ResourceAttributes: map[string]string{
				"deployment.region": ps.config.DeploymentRegion,
				"deployment.zone":   ps.config.DeploymentZone,
				"service.instance":  fmt.Sprintf("%s-%d", ps.config.ServiceName, ps.config.Port),
			},
		}
	}

	if ps.config.EnableMetrics {
		obsConfig.MetricsConfig = observability.MetricsConfig{
			ServiceName:      ps.config.ServiceName,
			ServiceVersion:   ps.config.ServerVersion,
			Environment:      ps.config.Environment,
			MetricsPath:      ps.config.MetricsPath,
			MetricsPort:      ps.config.MetricsPort,
			Namespace:        "mcp_production",
			HistogramBuckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		}
	}

	// Create observability middleware
	middleware, err := observability.NewEnhancedObservabilityMiddleware(obsConfig)
	if err != nil {
		return fmt.Errorf("failed to create observability middleware: %w", err)
	}

	ps.middleware = middleware
	return nil
}

// createTransport creates a configured transport with security settings
func (ps *ProductionServer) createTransport() (transport.Transport, error) {
	config := transport.DefaultTransportConfig(transport.TransportTypeStreamableHTTP)

	// Configure endpoint
	config.Endpoint = fmt.Sprintf("http://%s:%d/mcp", ps.config.Host, ps.config.Port)

	// Configure security
	config.Security.AllowedOrigins = ps.config.AllowedOrigins
	config.Security.AllowWildcardOrigin = false

	// Configure features
	config.Features.EnableObservability = ps.config.EnableMetrics || ps.config.EnableTracing
	config.Features.EnableAuthentication = ps.config.EnableAuth
	config.Features.EnableReliability = true
	config.Reliability.MaxRetries = 3
	config.Reliability.InitialRetryDelay = time.Second
	config.Reliability.MaxRetryDelay = 10 * time.Second

	// Configure performance
	config.Performance.BufferSize = 32 * 1024
	// Connection timeouts are configured differently in the actual implementation

	return transport.NewTransport(config)
}

// setupHealthServer creates the health check HTTP server
func (ps *ProductionServer) setupHealthServer() {
	mux := http.NewServeMux()

	// Liveness probe - always responds if server is running
	mux.HandleFunc(ps.config.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"service":   ps.config.ServiceName,
			"version":   ps.config.ServerVersion,
		})
	})

	// Readiness probe - checks if server is ready to serve requests
	mux.HandleFunc(ps.config.ReadinessPath, func(w http.ResponseWriter, r *http.Request) {
		ps.readyMutex.RLock()
		ready := ps.ready
		ps.readyMutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if ready {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "ready",
				"timestamp": time.Now().Unix(),
				"service":   ps.config.ServiceName,
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "not_ready",
				"timestamp": time.Now().Unix(),
				"service":   ps.config.ServiceName,
			})
		}
	})

	ps.healthServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", ps.config.HealthCheckPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
}

// setupMetricsServer creates the metrics HTTP server (if not handled by observability middleware)
func (ps *ProductionServer) setupMetricsServer() {
	if !ps.config.EnableMetrics {
		return
	}

	// The observability middleware will handle metrics on its own port
	// This is just a placeholder if we need additional metrics endpoints
}

// SetReady marks the server as ready to serve requests
func (ps *ProductionServer) SetReady(ready bool) {
	ps.readyMutex.Lock()
	ps.ready = ready
	ps.readyMutex.Unlock()
}

// Start starts all server components
func (ps *ProductionServer) Start(ctx context.Context) error {
	log.Printf("Starting production MCP server %s v%s", ps.config.ServiceName, ps.config.ServerVersion)

	// Start health check server
	go func() {
		log.Printf("Health check server listening on :%d", ps.config.HealthCheckPort)
		if err := ps.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	// Start MCP server
	go func() {
		log.Printf("MCP server listening on %s:%d", ps.config.Host, ps.config.Port)
		if err := ps.mcpServer.Start(ctx); err != nil {
			log.Printf("MCP server error: %v", err)
		}
	}()

	// Wait a moment for server to initialize, then mark as ready
	time.Sleep(2 * time.Second)
	ps.SetReady(true)
	log.Printf("Server is ready and accepting connections")

	return nil
}

// Stop gracefully shuts down all server components
func (ps *ProductionServer) Stop() error {
	log.Printf("Shutting down production MCP server...")

	// Mark as not ready
	ps.SetReady(false)

	ctx, cancel := context.WithTimeout(context.Background(), ps.config.ShutdownTimeout)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	// Stop health server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ps.healthServer.Shutdown(ctx); err != nil {
			errChan <- fmt.Errorf("health server shutdown error: %w", err)
		}
	}()

	// Stop MCP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		ps.mcpServer.Stop()
	}()

	// Wait for all shutdowns or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("All servers shut down gracefully")
	case <-ctx.Done():
		log.Printf("Shutdown timeout exceeded")
		return fmt.Errorf("shutdown timeout exceeded")
	}

	// Collect any errors
	close(errChan)
	for err := range errChan {
		log.Printf("Shutdown error: %v", err)
	}

	return nil
}

func main() {
	// Load configuration
	config := LoadConfig()

	// Log startup configuration (without sensitive data)
	log.Printf("Starting with configuration:")
	log.Printf("  Service: %s v%s", config.ServiceName, config.ServerVersion)
	log.Printf("  Environment: %s", config.Environment)
	log.Printf("  MCP Port: %d", config.Port)
	log.Printf("  Health Port: %d", config.HealthCheckPort)
	log.Printf("  Metrics Port: %d", config.MetricsPort)
	log.Printf("  Tracing: %t", config.EnableTracing)
	log.Printf("  Metrics: %t", config.EnableMetrics)

	// Create production server
	server, err := NewProductionServer(config)
	if err != nil {
		log.Fatalf("Failed to create production server: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)
	cancel()

	// Shutdown server
	if err := server.Stop(); err != nil {
		log.Printf("Server shutdown error: %v", err)
		os.Exit(1)
	}

	log.Printf("Server shutdown complete")
}

// Utility functions for environment variable handling

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		var slice []string
		if err := json.Unmarshal([]byte(value), &slice); err == nil {
			return slice
		}
	}
	return defaultValue
}
