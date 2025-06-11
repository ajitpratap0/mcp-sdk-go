# MCP Development Server with Hot Reload

This development server provides hot reloading, live configuration updates, and comprehensive development tools for MCP providers, enabling rapid development and testing workflows.

## 🚀 Overview

The MCP Development Server is designed to streamline the development process for MCP (Model Context Protocol) implementations by providing:

- **Hot Reload**: Automatic server restart when code changes are detected
- **Live Dashboard**: Real-time web interface for monitoring and debugging
- **File Watching**: Intelligent monitoring of source files and configuration
- **Development Tools**: Request logging, metrics, and debugging utilities
- **Zero Downtime**: Graceful reloads without losing connections
- **Multi-Provider Support**: Independent reloading of different provider types

## 🎯 Key Features

### Hot Reload System
- **File Watching**: Monitors Go source files, YAML/JSON configs
- **Smart Debouncing**: Prevents rapid successive reloads
- **Selective Watching**: Configurable paths and ignore patterns
- **Graceful Restart**: Maintains existing connections during reload

### Development Dashboard
- **Real-time Monitoring**: Live server status and metrics
- **Provider Management**: View loaded providers and their status
- **Log Streaming**: Real-time log output with filtering
- **Metrics Visualization**: Request counts, response times, error rates
- **WebSocket Updates**: Instant dashboard updates without refresh

### Advanced Development Tools
- **Request Logging**: Detailed HTTP request/response logging
- **CORS Support**: Configurable CORS for web development
- **Performance Metrics**: Response time tracking and analysis
- **Error Tracking**: Automatic error counting and reporting
- **Memory Monitoring**: Runtime memory usage tracking

## 🚀 Quick Start

### Basic Usage

```bash
# Run the development server
go run main.go

# The server will start with default configuration:
# - MCP server: http://localhost:8080/mcp
# - Dashboard: http://localhost:3000
# - Hot reload: Enabled
```

### Custom Configuration

```bash
# Create a configuration file
cat > dev-config.json << EOF
{
  "mcp_port": 8080,
  "dashboard_port": 3000,
  "watch_paths": [".", "./examples", "./pkg"],
  "enable_hot_reload": true,
  "reload_delay": "500ms",
  "enable_request_logging": true,
  "enable_metrics": true,
  "enable_cors": true,
  "providers_config": [
    {
      "name": "my_tools",
      "type": "tools",
      "source_path": "./my-provider",
      "enabled": true
    }
  ],
  "verbose_logging": true
}
EOF

# Run with custom config
CONFIG_FILE=dev-config.json go run main.go
```

## 📊 Development Dashboard

The development dashboard provides a comprehensive view of your MCP server:

### Server Information Panel
- **Status**: Current server state (Running/Stopped)
- **Uptime**: How long the server has been running
- **Last Reload**: When the last hot reload occurred
- **Endpoints**: MCP and dashboard URLs
- **Hot Reload Status**: Whether automatic reloading is enabled

### Metrics Panel
- **Request Count**: Total number of requests processed
- **Error Count**: Number of failed requests
- **Average Response Time**: Mean response time across all requests
- **Memory Usage**: Current memory consumption
- **Goroutine Count**: Number of active goroutines

### Providers Panel
- **Provider List**: All configured providers with status
- **Load Time**: How long each provider took to load
- **Error Messages**: Any loading errors for failed providers
- **Configuration**: Provider type and source paths

### Logs Panel
- **Real-time Logs**: Live streaming of server logs
- **Log Levels**: INFO, WARN, ERROR with color coding
- **Provider Context**: Which provider generated each log entry
- **Timestamp**: Precise timing for each log entry

### Watched Paths Panel
- **File Monitoring**: List of directories being watched
- **Pattern Matching**: Which file types trigger reloads
- **Ignore Patterns**: Files and directories being ignored

## ⚙️ Configuration

### Complete Configuration Example

```json
{
  "mcp_port": 8080,
  "dashboard_port": 3000,
  "watch_paths": [
    ".",
    "./examples",
    "./pkg",
    "./internal"
  ],
  "enable_hot_reload": true,
  "reload_delay": "500ms",
  "ignore_patterns": [
    ".git/",
    "node_modules/",
    "*.tmp",
    "*.log",
    ".DS_Store"
  ],
  "enable_request_logging": true,
  "enable_metrics": true,
  "enable_cors": true,
  "providers_config": [
    {
      "name": "calculation_tools",
      "type": "tools",
      "config_path": "./configs/calc-tools.yaml",
      "source_path": "./providers/calculator",
      "enabled": true,
      "parameters": {
        "precision": 10,
        "max_operations": 1000
      }
    },
    {
      "name": "file_resources",
      "type": "resources",
      "config_path": "./configs/file-resources.yaml",
      "source_path": "./providers/files",
      "enabled": true,
      "parameters": {
        "base_path": "/tmp/mcp-files",
        "max_file_size": "10MB"
      }
    },
    {
      "name": "ai_prompts",
      "type": "prompts",
      "config_path": "./configs/ai-prompts.yaml",
      "source_path": "./providers/prompts",
      "enabled": true,
      "parameters": {
        "template_dir": "./templates",
        "cache_templates": true
      }
    }
  ],
  "verbose_logging": true,
  "profile_enabled": false
}
```

### Environment Variables

You can override configuration using environment variables:

```bash
# Server ports
export MCP_PORT=8080
export DASHBOARD_PORT=3000

# Hot reload settings
export ENABLE_HOT_RELOAD=true
export RELOAD_DELAY=500ms

# Development features
export ENABLE_REQUEST_LOGGING=true
export ENABLE_METRICS=true
export ENABLE_CORS=true
export VERBOSE_LOGGING=true

# File watching
export WATCH_PATHS=".,./examples,./pkg"
export IGNORE_PATTERNS=".git/,node_modules/,*.tmp"

# Run server
go run main.go
```

## 🔧 Provider Development Workflow

### 1. Create a New Provider

```go
// my-provider/tools.go
package myprovider

import (
    "context"
    "encoding/json"
    "github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

type MyToolsProvider struct {
    config MyConfig
}

func (p *MyToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
    tools := []protocol.Tool{
        {
            Name: "my_tool",
            Description: "My custom tool",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "input": map[string]interface{}{
                        "type": "string",
                        "description": "Tool input",
                    },
                },
                "required": []string{"input"},
            },
        },
    }
    return tools, len(tools), "", false, nil
}

func (p *MyToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
    // Tool implementation
    return &protocol.CallToolResult{
        Content: []protocol.TextContent{
            {
                Type: "text",
                Text: "Tool executed successfully",
            },
        },
    }, nil
}
```

### 2. Add Provider to Configuration

```json
{
  "providers_config": [
    {
      "name": "my_custom_tools",
      "type": "tools",
      "source_path": "./my-provider",
      "enabled": true,
      "parameters": {
        "custom_setting": "value"
      }
    }
  ]
}
```

### 3. Start Development Server

```bash
# The server will automatically detect and load your provider
go run main.go

# Visit the dashboard to see your provider status
open http://localhost:3000
```

### 4. Iterate with Hot Reload

1. **Edit your provider code** in `./my-provider/tools.go`
2. **Save the file** - the development server automatically detects changes
3. **Watch the dashboard** - you'll see the reload status update in real-time
4. **Test immediately** - the server restarts with your changes loaded

## 📈 Development Best Practices

### File Organization

```
my-mcp-project/
├── cmd/
│   └── server/
│       └── main.go              # Production server
├── internal/
│   ├── providers/
│   │   ├── tools/              # Tools providers
│   │   ├── resources/          # Resources providers
│   │   └── prompts/            # Prompts providers
│   └── config/
│       └── config.go           # Configuration management
├── configs/
│   ├── development.json        # Development configuration
│   ├── staging.json           # Staging configuration
│   └── production.json        # Production configuration
├── examples/
│   └── development-server/
│       └── main.go            # This development server
└── go.mod
```

### Hot Reload Optimization

1. **Watch Specific Paths**: Only monitor directories that contain your code
   ```json
   {
     "watch_paths": [
       "./internal/providers",
       "./configs"
     ]
   }
   ```

2. **Ignore Build Artifacts**: Exclude generated files and dependencies
   ```json
   {
     "ignore_patterns": [
       ".git/",
       "vendor/",
       "*.tmp",
       "build/",
       "dist/"
     ]
   }
   ```

3. **Optimize Reload Delay**: Balance responsiveness vs. avoiding excessive reloads
   ```json
   {
     "reload_delay": "1s"  // Wait 1 second for additional changes
   }
   ```

### Development vs. Production

| Feature | Development Server | Production Server |
|---------|-------------------|-------------------|
| Hot Reload | ✅ Enabled | ❌ Disabled |
| Dashboard | ✅ Full Featured | ⚠️ Limited/None |
| Request Logging | ✅ Verbose | ⚠️ Essential Only |
| CORS | ✅ Permissive | ⚠️ Restrictive |
| Metrics | ✅ Detailed | ✅ Aggregated |
| File Watching | ✅ Active | ❌ None |

## 🐛 Debugging and Troubleshooting

### Common Issues

#### Hot Reload Not Working

```bash
# Check file permissions
ls -la ./my-provider/

# Verify watch paths are correct
curl http://localhost:3000/api/status | jq '.watched_paths'

# Enable verbose logging
export VERBOSE_LOGGING=true
go run main.go
```

#### Provider Load Errors

```bash
# Check provider status via API
curl http://localhost:3000/api/providers | jq

# Check dashboard for error messages
open http://localhost:3000

# Review server logs for detailed error information
```

#### Dashboard Not Accessible

```bash
# Verify dashboard port
netstat -an | grep :3000

# Check for port conflicts
lsof -i :3000

# Try alternative port
export DASHBOARD_PORT=3001
go run main.go
```

### Debug Mode

Enable comprehensive debugging:

```json
{
  "verbose_logging": true,
  "profile_enabled": true,
  "enable_request_logging": true
}
```

This provides:
- **Detailed Logs**: Every file change, reload trigger, and provider load
- **Request Tracing**: Complete HTTP request/response logging
- **Performance Profiling**: Memory and CPU usage tracking

### WebSocket Debugging

Monitor real-time updates:

```javascript
// Open browser console on dashboard page
const ws = new WebSocket('ws://localhost:3000/ws');
ws.onmessage = (event) => {
    console.log('Dashboard update:', JSON.parse(event.data));
};
```

## 🔌 API Reference

### REST Endpoints

#### Get Server Status
```bash
curl http://localhost:3000/api/status
```

#### Trigger Manual Reload
```bash
curl -X POST http://localhost:3000/api/reload
```

#### Get Provider Information
```bash
curl http://localhost:3000/api/providers
```

#### Get Recent Logs
```bash
curl http://localhost:3000/api/logs
```

### WebSocket Messages

#### Client to Server
```json
{
  "type": "get_dashboard_data"
}
```

#### Server to Client
```json
{
  "type": "dashboard_update",
  "dashboard": {
    "server_info": {...},
    "providers": {...},
    "metrics": {...},
    "recent_logs": [...],
    "watched_paths": [...]
  }
}
```

```json
{
  "type": "reload_started",
  "message": "Server reload initiated..."
}
```

```json
{
  "type": "reload_completed",
  "message": "Server reload completed successfully"
}
```

## 📦 Dependencies

### Required Dependencies

```bash
# File system notifications
go get github.com/fsnotify/fsnotify

# WebSocket support
go get github.com/gorilla/websocket

# MCP SDK
go get github.com/ajitpratap0/mcp-sdk-go
```

### Development Dependencies

```bash
# For advanced development features
go get github.com/pkg/profile    # Performance profiling
go get github.com/sirupsen/logrus # Structured logging
```

## 🚢 Production Deployment

While this is a development server, you can adapt it for production use:

### Production Configuration

```json
{
  "enable_hot_reload": false,
  "enable_request_logging": false,
  "verbose_logging": false,
  "enable_cors": false,
  "dashboard_port": 0,  // Disable dashboard
  "enable_metrics": true
}
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o dev-server examples/development-server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/dev-server .
COPY --from=builder /app/configs ./configs

# Production mode - no hot reload
ENV ENABLE_HOT_RELOAD=false
ENV DASHBOARD_PORT=0
ENV VERBOSE_LOGGING=false

EXPOSE 8080
CMD ["./dev-server"]
```

## 🎯 Next Steps

1. **Explore Examples**: Check other examples in the `/examples` directory
2. **Read MCP Specification**: Understand the full protocol capabilities
3. **Build Custom Providers**: Create providers specific to your use case
4. **Performance Testing**: Use the included benchmarking tools
5. **Production Deployment**: Adapt for your production environment

This development server provides a solid foundation for rapid MCP provider development with modern development tools and workflows.
