# Plugin Architecture Example

This example demonstrates how to build extensible MCP applications using a comprehensive plugin architecture, enabling dynamic loading, management, and composition of capabilities.

## 🎯 Overview

Modern applications need to be extensible and modular. This plugin architecture provides enterprise-grade patterns for:

- **Dynamic Loading**: Load plugins at runtime from shared libraries
- **Lifecycle Management**: Complete plugin lifecycle with health monitoring
- **Capability Composition**: Aggregate multiple plugins into unified providers
- **Plugin Registry**: Discovery and metadata management
- **Hot Reload**: Dynamic reloading without server restart (configurable)
- **Resource Isolation**: Sandbox plugins with memory and CPU limits
- **Monitoring**: Comprehensive metrics and health checks
- **Management Tools**: MCP tools for plugin administration

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                 MCP Application                         │
├─────────────────────────────────────────────────────────┤
│                Plugin Manager                           │
│  ┌─────────────────────────────────────────────────────┐│
│  │             Plugin Registry                         ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐   ││
│  │  │   Plugin A  │ │   Plugin B  │ │   Plugin C  │   ││
│  │  │   (Tools)   │ │ (Resources) │ │ (Prompts)   │   ││
│  │  └─────────────┘ └─────────────┘ └─────────────┘   ││
│  └─────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────┤
│           Composite Providers                           │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐       │
│  │ Tools       │ │ Resources   │ │ Prompts     │       │
│  │ Aggregator  │ │ Aggregator  │ │ Aggregator  │       │
│  └─────────────┘ └─────────────┘ └─────────────┘       │
├─────────────────────────────────────────────────────────┤
│                  MCP Server                             │
└─────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. **Plugin Interface**
Universal contract for all plugins:

```go
type Plugin interface {
    // Metadata
    GetInfo() PluginInfo
    GetVersion() string
    GetDependencies() []string

    // Lifecycle
    Initialize(ctx context.Context, config map[string]interface{}) error
    Start(ctx context.Context) error
    Stop() error
    Health() PluginHealth

    // Capabilities
    GetToolsProvider() ToolsPluginProvider
    GetResourcesProvider() ResourcesPluginProvider
    GetPromptsProvider() PromptsPluginProvider
    GetMiddleware() MiddlewareProvider
}
```

#### 2. **Plugin Manager**
Central orchestrator for all plugin operations:
- Loading/unloading plugins
- Lifecycle management
- Health monitoring
- Metrics collection
- Resource isolation

#### 3. **Plugin Registry**
Metadata and discovery service:
- Plugin information storage
- Capability-based discovery
- Version management
- Dependency tracking

#### 4. **Composite Providers**
Aggregate multiple plugins into unified MCP providers:
- Route requests to appropriate plugins
- Combine results from multiple sources
- Handle failover and load balancing

## 🚀 Quick Start

### Building and Running

```bash
# Run the example
go run main.go

# The server will start with built-in example plugins
# (Calculator and Time plugins are loaded automatically)
```

### Testing Plugin Management

```bash
# In another terminal, use the MCP client to test plugin management

# List all plugins
curl -X POST http://localhost:8087/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "list_plugins",
      "arguments": {}
    }
  }'

# Get plugin information
curl -X POST http://localhost:8087/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "get_plugin_info",
      "arguments": {"name": "calculator"}
    }
  }'

# Get plugin metrics
curl -X POST http://localhost:8087/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "get_plugin_metrics",
      "arguments": {"detailed": true}
    }
  }'
```

## 🔧 Creating Custom Plugins

### Plugin Structure

A typical plugin implements the `Plugin` interface and provides one or more capabilities:

```go
package main

import (
    "context"
    "encoding/json"
    "time"

    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

type MyPlugin struct {
    config    map[string]interface{}
    startTime time.Time
    status    PluginStatus
}

// Plugin must be exported for dynamic loading
var Plugin MyPlugin

func (p *MyPlugin) GetInfo() PluginInfo {
    return PluginInfo{
        Name:        "MyPlugin",
        Description: "Example custom plugin",
        Author:      "Your Name",
        Version:     "1.0.0",
        License:     "MIT",
        Capabilities: []PluginCapability{CapabilityTools},
    }
}

func (p *MyPlugin) Initialize(ctx context.Context, config map[string]interface{}) error {
    p.config = config
    return nil
}

func (p *MyPlugin) Start(ctx context.Context) error {
    p.startTime = time.Now()
    p.status = StatusStarted
    return nil
}

func (p *MyPlugin) Stop() error {
    p.status = StatusStopped
    return nil
}

func (p *MyPlugin) Health() PluginHealth {
    return PluginHealth{
        Status:    p.status,
        Message:   "Plugin running normally",
        LastCheck: time.Now(),
        Uptime:    time.Since(p.startTime),
    }
}

func (p *MyPlugin) GetToolsProvider() ToolsPluginProvider {
    return p // Implement ToolsPluginProvider interface
}

// Implement other capability providers as needed
func (p *MyPlugin) GetResourcesProvider() ResourcesPluginProvider { return nil }
func (p *MyPlugin) GetPromptsProvider() PromptsPluginProvider { return nil }
func (p *MyPlugin) GetMiddleware() MiddlewareProvider { return nil }
```

### Building Plugin Shared Libraries

```bash
# Build as a shared library (.so file)
go build -buildmode=plugin -o myplugin.so myplugin.go

# Place in the plugins directory
mkdir -p plugins
mv myplugin.so plugins/
```

### Plugin Capabilities

#### Tools Provider
Provide custom tools through the MCP interface:

```go
func (p *MyPlugin) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
    tools := []protocol.Tool{
        {
            Name:        "my_tool",
            Description: "Custom tool functionality",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "input": map[string]interface{}{"type": "string"},
                },
                "required": []string{"input"},
            },
        },
    }
    return tools, len(tools), "", false, nil
}

func (p *MyPlugin) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
    switch name {
    case "my_tool":
        // Implement tool logic
        return &protocol.CallToolResult{
            Content: []protocol.TextContent{
                {Type: "text", Text: "Tool result"},
            },
        }, nil
    default:
        return nil, fmt.Errorf("unknown tool: %s", name)
    }
}
```

#### Resources Provider
Expose resources through the MCP interface:

```go
func (p *MyPlugin) ListResources(ctx context.Context, pagination *protocol.PaginationParams) ([]protocol.Resource, int, string, bool, error) {
    resources := []protocol.Resource{
        {
            URI:         "myplugin://data",
            Name:        "Plugin Data",
            Description: "Custom plugin data",
            MimeType:    "application/json",
        },
    }
    return resources, len(resources), "", false, nil
}

func (p *MyPlugin) ReadResource(ctx context.Context, uri string) (*protocol.ResourceContents, error) {
    switch uri {
    case "myplugin://data":
        data := map[string]interface{}{
            "message": "Hello from plugin",
            "timestamp": time.Now(),
        }
        jsonData, _ := json.Marshal(data)

        return &protocol.ResourceContents{
            URI:      uri,
            MimeType: "application/json",
            Text:     string(jsonData),
        }, nil
    default:
        return nil, fmt.Errorf("resource not found: %s", uri)
    }
}
```

#### Middleware Provider
Create custom middleware for request processing:

```go
func (p *MyPlugin) CreateMiddleware(config map[string]interface{}) (transport.Middleware, error) {
    return &MyMiddleware{config: config}, nil
}

type MyMiddleware struct {
    config map[string]interface{}
}

func (m *MyMiddleware) Wrap(next transport.Transport) transport.Transport {
    return &MyTransportWrapper{
        next:   next,
        config: m.config,
    }
}
```

## 📊 Plugin Management

### Available Management Tools

The plugin architecture provides MCP tools for runtime management:

#### 1. `list_plugins`
List all loaded plugins with optional filtering:

```json
{
    "name": "list_plugins",
    "arguments": {
        "capability": "tools",
        "status": "started"
    }
}
```

#### 2. `get_plugin_info`
Get detailed information about a specific plugin:

```json
{
    "name": "get_plugin_info",
    "arguments": {
        "name": "calculator"
    }
}
```

#### 3. `load_plugin`
Dynamically load a plugin from file:

```json
{
    "name": "load_plugin",
    "arguments": {
        "name": "my_plugin",
        "path": "/path/to/plugin.so",
        "config": {
            "setting1": "value1",
            "setting2": 42
        }
    }
}
```

#### 4. `unload_plugin`
Unload a running plugin:

```json
{
    "name": "unload_plugin",
    "arguments": {
        "name": "my_plugin"
    }
}
```

#### 5. `get_plugin_metrics`
Retrieve plugin system metrics:

```json
{
    "name": "get_plugin_metrics",
    "arguments": {
        "detailed": true
    }
}
```

### Plugin Configuration

Configure the plugin manager through `PluginManagerConfig`:

```go
config := PluginManagerConfig{
    PluginPaths:           []string{"./plugins", "/opt/plugins"},
    AutoLoad:              true,  // Automatically load plugins on startup
    EnableHotReload:       true,  // Enable hot reloading
    HealthCheckInterval:   30 * time.Second,
    EnableSandbox:         true,  // Enable resource isolation
    MaxMemoryPerPlugin:    100 * 1024 * 1024, // 100MB per plugin
    MaxCPUPerPlugin:       0.5,   // 50% CPU per plugin
    LoadTimeout:           30 * time.Second,
    StartTimeout:          10 * time.Second,
    AllowedCapabilities:   []PluginCapability{
        CapabilityTools,
        CapabilityResources,
        CapabilityPrompts,
    },
    RequiredSignature:     true,  // Require signed plugins in production
}
```

## 🔒 Security and Isolation

### Plugin Sandboxing

```go
// Enable resource limits
config.EnableSandbox = true
config.MaxMemoryPerPlugin = 100 * 1024 * 1024 // 100MB
config.MaxCPUPerPlugin = 0.5                   // 50% CPU
```

### Capability Restrictions

```go
// Limit plugin capabilities
config.AllowedCapabilities = []PluginCapability{
    CapabilityTools,     // Allow tools
    CapabilityResources, // Allow resources
    // CapabilityAuth is not allowed
}
```

### Plugin Signing

```go
// Require plugin signatures in production
config.RequiredSignature = true

// Verify plugin integrity before loading
func verifyPluginSignature(path string) error {
    // Implement digital signature verification
    return nil
}
```

## 📈 Monitoring and Metrics

### Plugin Health Monitoring

The system continuously monitors plugin health:

```go
type PluginHealth struct {
    Status     PluginStatus  `json:"status"`     // loaded, started, stopped, error
    Message    string        `json:"message"`    // Status description
    LastCheck  time.Time     `json:"last_check"` // Last health check
    Uptime     time.Duration `json:"uptime"`     // Time since start
    ErrorCount int64         `json:"error_count"`// Error count
}
```

### System Metrics

Track overall plugin system performance:

```go
type PluginMetrics struct {
    TotalPlugins     int                          `json:"total_plugins"`
    RunningPlugins   int                          `json:"running_plugins"`
    FailedPlugins    int                          `json:"failed_plugins"`
    CapabilityCounts map[PluginCapability]int     `json:"capability_counts"`
    PluginStats      map[string]PluginStats       `json:"plugin_stats"`
}

type PluginStats struct {
    RequestCount   int64         `json:"request_count"`
    ErrorCount     int64         `json:"error_count"`
    AverageLatency time.Duration `json:"average_latency"`
    MemoryUsage    int64         `json:"memory_usage"`
    CPUUsage       float64       `json:"cpu_usage"`
}
```

### Health Check Integration

```go
// Set up health monitoring
config.HealthCheckInterval = 30 * time.Second

// Monitor plugin health
func (pm *PluginManager) performHealthChecks() {
    for _, instance := range pm.plugins {
        health := instance.Plugin.Health()
        if health.Status == StatusError {
            log.Printf("Plugin %s unhealthy: %s",
                      instance.Info.Name, health.Message)
            // Take corrective action
        }
    }
}
```

## 🚀 Production Deployment

### Docker Configuration

```dockerfile
FROM golang:1.21-alpine AS builder

# Build main application
WORKDIR /app
COPY . .
RUN go build -o plugin-server main.go

# Build example plugins
RUN go build -buildmode=plugin -o plugins/calculator.so examples/calculator/
RUN go build -buildmode=plugin -o plugins/time.so examples/time/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy application and plugins
COPY --from=builder /app/plugin-server .
COPY --from=builder /app/plugins ./plugins/

# Create plugin directories
RUN mkdir -p /opt/plugins /var/lib/plugins

EXPOSE 8087
CMD ["./plugin-server"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: plugin-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: plugin-server
  template:
    spec:
      containers:
      - name: server
        image: plugin-server:latest
        ports:
        - containerPort: 8087
        env:
        - name: PLUGIN_PATH
          value: "/opt/plugins:/var/lib/plugins"
        - name: ENABLE_HOT_RELOAD
          value: "true"
        - name: HEALTH_CHECK_INTERVAL
          value: "30s"
        volumeMounts:
        - name: plugin-storage
          mountPath: /var/lib/plugins
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8087
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8087
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: plugin-storage
        persistentVolumeClaim:
          claimName: plugin-storage-pvc
```

### Plugin Distribution

```bash
# Create plugin package
tar -czf myplugin-1.0.0.tar.gz myplugin.so config.json README.md

# Install plugin
kubectl create configmap myplugin-config --from-file=myplugin.so
kubectl label configmap myplugin-config plugin=myplugin version=1.0.0
```

## 🧪 Testing Plugins

### Unit Testing

```go
func TestMyPlugin(t *testing.T) {
    plugin := &MyPlugin{}

    // Test initialization
    err := plugin.Initialize(context.Background(), map[string]interface{}{})
    assert.NoError(t, err)

    // Test start
    err = plugin.Start(context.Background())
    assert.NoError(t, err)

    // Test health
    health := plugin.Health()
    assert.Equal(t, StatusStarted, health.Status)

    // Test tools provider
    if provider := plugin.GetToolsProvider(); provider != nil {
        tools, _, _, _, err := provider.ListTools(context.Background(), "", nil)
        assert.NoError(t, err)
        assert.Greater(t, len(tools), 0)
    }

    // Test cleanup
    err = plugin.Stop()
    assert.NoError(t, err)
}
```

### Integration Testing

```go
func TestPluginIntegration(t *testing.T) {
    // Create plugin manager
    manager := NewPluginManager(PluginManagerConfig{
        LoadTimeout:  10 * time.Second,
        StartTimeout: 5 * time.Second,
    })

    // Load plugin
    err := manager.LoadPlugin(context.Background(), "test", "/path/to/plugin.so", nil)
    assert.NoError(t, err)

    // Start manager
    err = manager.Start(context.Background())
    assert.NoError(t, err)
    defer manager.Stop()

    // Test plugin functionality
    instance, exists := manager.GetPlugin("test")
    assert.True(t, exists)
    assert.Equal(t, StatusStarted, instance.Status)

    // Test metrics
    metrics := manager.GetMetrics()
    assert.Equal(t, 1, metrics.RunningPlugins)
}
```

### Load Testing

```go
func TestPluginLoad(t *testing.T) {
    manager := setupPluginManager()
    defer manager.Stop()

    // Generate concurrent load
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            // Test plugin operations
            provider := manager.plugins["test"].Plugin.GetToolsProvider()
            _, err := provider.CallTool(context.Background(), "test_tool",
                                      json.RawMessage(`{}`), nil)
            assert.NoError(t, err)
        }()
    }

    wg.Wait()

    // Verify system stability
    metrics := manager.GetMetrics()
    assert.Equal(t, 1, metrics.RunningPlugins)
}
```

## 🔧 Advanced Features

### Plugin Dependencies

```go
func (p *MyPlugin) GetDependencies() []string {
    return []string{
        "base-plugin>=1.0.0",
        "utils-plugin^2.0.0",
    }
}

// Dependency resolution
func (pm *PluginManager) resolveDependencies(plugin Plugin) error {
    deps := plugin.GetDependencies()
    for _, dep := range deps {
        if !pm.isDependencySatisfied(dep) {
            return fmt.Errorf("dependency not satisfied: %s", dep)
        }
    }
    return nil
}
```

### Plugin Versioning

```go
type VersionConstraint struct {
    Name       string
    Constraint string // >=1.0.0, ^2.0.0, ~1.2.0
}

func (pm *PluginManager) checkVersionCompatibility(plugin Plugin) error {
    version := plugin.GetVersion()
    // Implement semantic version checking
    return nil
}
```

### Hot Reload

```go
// Enable hot reload
config.EnableHotReload = true

// Watch for file changes
func (pm *PluginManager) hotReloadLoop(ctx context.Context) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return
    }
    defer watcher.Close()

    for _, path := range pm.config.PluginPaths {
        watcher.Add(path)
    }

    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                pm.reloadPlugin(event.Name)
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### Plugin Communication

```go
// Inter-plugin messaging
type PluginMessage struct {
    From    string                 `json:"from"`
    To      string                 `json:"to"`
    Type    string                 `json:"type"`
    Payload map[string]interface{} `json:"payload"`
}

func (pm *PluginManager) SendMessage(msg PluginMessage) error {
    target, exists := pm.plugins[msg.To]
    if !exists {
        return fmt.Errorf("target plugin not found: %s", msg.To)
    }

    // Deliver message to plugin
    if handler, ok := target.Plugin.(MessageHandler); ok {
        return handler.HandleMessage(msg)
    }

    return fmt.Errorf("plugin does not support messaging")
}
```

## 📚 Example Plugins

This repository includes several example plugins:

### Calculator Plugin
- **Capability**: Tools
- **Functions**: add, subtract, multiply, divide
- **Use Case**: Mathematical operations

### Time Plugin
- **Capabilities**: Tools, Resources
- **Functions**: current_time, format_time
- **Resources**: time://current, time://zones
- **Use Case**: Time and date utilities

### Custom Plugin Template
- **Location**: `examples/plugin-template/`
- **Use Case**: Starting point for new plugins

## 🤝 Contributing Plugins

### Plugin Development Guidelines

1. **Follow Interface**: Implement all required interface methods
2. **Error Handling**: Provide clear error messages and proper error types
3. **Documentation**: Include comprehensive documentation and examples
4. **Testing**: Write unit and integration tests
5. **Versioning**: Use semantic versioning
6. **Security**: Validate all inputs and handle sensitive data properly

### Plugin Submission

1. **Create Plugin**: Develop following the guidelines
2. **Test Thoroughly**: Ensure compatibility with latest SDK version
3. **Document**: Provide README and usage examples
4. **Submit**: Create pull request with plugin in `examples/plugins/`

## ⚡ Next Steps

1. **Build Custom Plugin**: Use the provided template
2. **Test Integration**: Load your plugin into the example server
3. **Deploy**: Use the production deployment patterns
4. **Monitor**: Set up comprehensive monitoring
5. **Scale**: Add load balancing and clustering
6. **Extend**: Add more sophisticated plugin features

This plugin architecture provides a robust foundation for building extensible MCP applications that can grow and adapt to changing requirements while maintaining stability and performance.
