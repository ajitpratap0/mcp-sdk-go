# LLM Completion Provider Examples

This example demonstrates how to integrate the MCP Go SDK with various Large Language Model (LLM) completion providers, showcasing production-ready patterns for AI-powered applications.

## 🎯 Overview

Modern applications often need to integrate with multiple LLM providers for text generation, chat completion, and AI-powered features. This example provides enterprise-grade patterns for:

- **Multi-Provider Support**: OpenAI, Anthropic, and extensible architecture for additional providers
- **Rate Limiting**: Token bucket algorithm to respect API limits
- **Retry Logic**: Exponential backoff with jitter for resilient operations
- **Streaming Support**: Real-time streaming completions
- **Context Management**: Proper handling of conversation context and token limits
- **Caching**: Response caching for improved performance and cost efficiency
- **Load Balancing**: Multiple strategies for distributing requests across providers
- **Metrics & Monitoring**: Comprehensive observability for production deployment

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                MCP Server                               │
├─────────────────────────────────────────────────────────┤
│           LLM Completion Provider                       │
│  ┌─────────────────────────────────────────────────────┐│
│  │              LLM Manager                            ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐   ││
│  │  │   OpenAI    │ │  Anthropic  │ │   Custom    │   ││
│  │  │  Provider   │ │  Provider   │ │  Provider   │   ││
│  │  └─────────────┘ └─────────────┘ └─────────────┘   ││
│  └─────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────┤
│              Rate Limiting & Caching                   │
│              Error Handling & Retry                    │
│              Metrics & Monitoring                      │
└─────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. **LLMProvider Interface**
Unified interface for different LLM providers:

```go
type LLMProvider interface {
    Complete(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, request *CompletionRequest, handler StreamHandler) error
    GetModel() string
    GetMaxTokens() int
    ValidateRequest(request *CompletionRequest) error
}
```

#### 2. **Provider Implementations**
- **OpenAIProvider**: Production-ready OpenAI integration
- **AnthropicProvider**: Claude API integration
- **Extensible**: Easy to add new providers

#### 3. **LLMManager**
- Load balancing across providers
- Fallback mechanisms
- Request routing based on model preferences

#### 4. **MCPCompletionProvider**
- MCP tools integration
- Caching layer
- Template management

## 🚀 Quick Start

### Prerequisites

1. **API Keys**: You'll need API keys for the providers you want to use:
   - OpenAI: Get from [OpenAI Platform](https://platform.openai.com/api-keys)
   - Anthropic: Get from [Anthropic Console](https://console.anthropic.com/)

2. **Environment Setup**:
   ```bash
   # Copy the example environment file
   cp .env.example .env

   # Edit .env with your API keys
   vim .env
   ```

### Running the Example

```bash
# Set your API keys
export OPENAI_API_KEY="your-openai-api-key"
export ANTHROPIC_API_KEY="your-anthropic-api-key"

# Run the example
go run main.go
```

## 🔧 Configuration

### Provider Configuration

#### OpenAI Provider
```go
openaiProvider := NewOpenAIProvider(apiKey)
openaiProvider.Model = "gpt-4o-mini"
openaiProvider.MaxTokens = 4096
openaiProvider.Temperature = 0.7

// Rate limiting: 100 requests/minute (OpenAI standard)
// Automatic retry with exponential backoff
```

#### Anthropic Provider
```go
anthropicProvider := NewAnthropicProvider(apiKey)
anthropicProvider.Model = "claude-3-haiku-20240307"
anthropicProvider.MaxTokens = 4096
anthropicProvider.Temperature = 0.7

// Rate limiting: 60 requests/minute
// Automatic format conversion between OpenAI and Anthropic APIs
```

### Load Balancing Strategies

#### Round Robin
```go
manager := NewLLMManager()
manager.loadBalance = LoadBalanceRoundRobin
// Distributes requests evenly across providers
```

#### Fallback (Recommended)
```go
manager := NewLLMManager()
manager.loadBalance = LoadBalanceFallback
// Uses primary provider, falls back to secondary on failure
```

#### Weighted
```go
manager := NewLLMManager()
manager.loadBalance = LoadBalanceWeighted
// Routes based on provider capabilities and performance
```

### Caching Configuration

```go
cache := &CompletionCache{
    ttl: 5 * time.Minute,  // Cache responses for 5 minutes
}
// Automatically caches identical requests to reduce costs
```

## 🛠️ Available MCP Tools

The integration provides these MCP tools for use by clients:

### 1. `complete_text`
Simple text completion from a prompt.

```json
{
    "name": "complete_text",
    "input": {
        "prompt": "Explain quantum computing in simple terms",
        "model": "gpt-4o-mini",
        "max_tokens": 200,
        "temperature": 0.7,
        "stream": false
    }
}
```

### 2. `complete_chat`
Chat completion with conversation context.

```json
{
    "name": "complete_chat",
    "input": {
        "messages": [
            {"role": "system", "content": "You are a helpful assistant"},
            {"role": "user", "content": "What is the Model Context Protocol?"}
        ],
        "max_tokens": 300,
        "temperature": 0.5
    }
}
```

### 3. `list_providers`
Get information about available LLM providers.

```json
{
    "name": "list_providers",
    "input": {
        "include_metrics": true
    }
}
```

### 4. `get_metrics`
Retrieve provider metrics and usage statistics.

```json
{
    "name": "get_metrics",
    "input": {
        "provider": "openai"  // Optional: specific provider
    }
}
```

## 📊 Monitoring and Metrics

### Provider Metrics

Each provider tracks:
- **Request Count**: Total requests processed
- **Error Count**: Failed requests
- **Stream Count**: Streaming requests
- **Token Count**: Total tokens used
- **Response Times**: Latency metrics

### Manager Metrics

The LLM manager provides:
- **Success/Failure Rates**: Overall success metrics
- **Provider Usage**: Request distribution across providers
- **Fallback Statistics**: When fallback providers are used

### Example Metrics Output

```json
{
    "total_requests": 1250,
    "success_requests": 1198,
    "failed_requests": 52,
    "provider_usage": {
        "openai": 1100,
        "anthropic": 150
    },
    "provider_errors": {
        "openai": 45,
        "anthropic": 7
    },
    "provider_metrics": {
        "openai": {
            "request_count": 1100,
            "error_count": 45,
            "stream_count": 230,
            "token_count": 125000
        }
    }
}
```

## 🔒 Security and Best Practices

### API Key Management
```bash
# Never hardcode API keys
export OPENAI_API_KEY="your-key"
export ANTHROPIC_API_KEY="your-key"

# Use environment-specific configurations
# Development: .env.development
# Production: Use secrets management (HashiCorp Vault, AWS Secrets Manager)
```

### Rate Limiting
- **Automatic**: Built-in rate limiting respects provider limits
- **Configurable**: Adjust limits based on your API quotas
- **Token Bucket**: Smooth request distribution prevents bursts

### Error Handling
```go
// Comprehensive error classification
func isRetryableError(err error) bool {
    // Network errors, timeouts, rate limits
    return strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "429") ||
           strings.Contains(err.Error(), "502")
}
```

### Cost Management
- **Caching**: Reduces duplicate API calls
- **Token Limits**: Configurable per request
- **Provider Selection**: Route to cost-effective providers

## 🧪 Testing

### Unit Tests
```bash
# Test individual providers
go test -run TestOpenAIProvider -v
go test -run TestAnthropicProvider -v

# Test manager logic
go test -run TestLLMManager -v
```

### Integration Tests
```bash
# Test with real APIs (requires API keys)
go test -run TestIntegration -v

# Mock tests for CI/CD
go test -run TestMock -v
```

### Load Testing
```bash
# Stress test with multiple concurrent requests
go test -run TestConcurrentRequests -v -count=10
```

## 🚀 Production Deployment

### Docker Configuration
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o llm-server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/llm-server .
EXPOSE 8085
CMD ["./llm-server"]
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llm-completion-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: llm-completion-server
  template:
    spec:
      containers:
      - name: server
        image: mcp-llm-server:latest
        ports:
        - containerPort: 8085
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: llm-secrets
              key: openai-api-key
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: llm-secrets
              key: anthropic-api-key
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### Health Checks
```go
// Add health check endpoint
func healthCheck(manager *LLMManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        metrics := manager.GetMetrics()
        status := "healthy"

        // Check if any providers are responding
        if metrics.TotalRequests > 0 {
            errorRate := float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
            if errorRate > 0.5 {
                status = "degraded"
                w.WriteHeader(http.StatusServiceUnavailable)
            }
        }

        json.NewEncoder(w).Encode(map[string]string{
            "status": status,
            "timestamp": time.Now().Format(time.RFC3339),
        })
    }
}
```

## 📈 Performance Optimization

### Connection Pooling
```go
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 60 * time.Second,
}
```

### Request Optimization
- **Batch Processing**: Group multiple completions when possible
- **Context Reuse**: Maintain conversation context efficiently
- **Token Management**: Optimize prompt engineering for token efficiency

### Caching Strategies
- **Response Caching**: Cache identical requests
- **Partial Caching**: Cache common prompt prefixes
- **TTL Management**: Balance freshness vs. performance

## 🔧 Extending the Example

### Adding New Providers

1. **Implement LLMProvider Interface**:
   ```go
   type MyCustomProvider struct {
       apiKey string
       baseURL string
   }

   func (p *MyCustomProvider) Complete(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
       // Implementation
   }
   ```

2. **Register with Manager**:
   ```go
   manager.AddProvider("custom", &MyCustomProvider{})
   ```

### Custom Tools

```go
// Add custom tools to the MCP provider
func (mcp *MCPCompletionProvider) addCustomTool() {
    // Register new tool with specific completion logic
}
```

### Advanced Features

- **Function Calling**: Integrate with OpenAI function calling
- **Embeddings**: Add vector similarity search
- **Fine-tuning**: Support for custom models
- **Multi-modal**: Image and text completion

## 🐛 Troubleshooting

### Common Issues

#### API Key Errors
```bash
# Check environment variables
echo $OPENAI_API_KEY
echo $ANTHROPIC_API_KEY

# Verify key format
# OpenAI: sk-...
# Anthropic: sk-ant-...
```

#### Rate Limiting
```bash
# Monitor rate limit headers in logs
grep "rate limit" app.log

# Adjust rate limiting configuration
```

#### High Latency
```bash
# Check provider response times
curl http://localhost:8085/metrics | grep response_time

# Enable streaming for long responses
```

### Debug Mode
```go
// Enable debug logging
log.SetLevel(log.DebugLevel)

// Trace API requests
transport.EnableDebugLogging()
```

## 📚 Related Examples

- **[Multi-Server Examples](../multi-server/)**: Load balancing across multiple MCP servers
- **[Error Recovery](../error-recovery/)**: Advanced resilience patterns
- **[Observability](../observability/)**: Monitoring and tracing integration
- **[Authentication](../authentication/)**: Secure API access

## ⚡ Next Steps

1. **Set up API Keys**: Configure your LLM provider credentials
2. **Run Examples**: Test the integration with your use case
3. **Customize Providers**: Add your preferred LLM providers
4. **Deploy**: Use the production deployment patterns
5. **Monitor**: Implement comprehensive monitoring
6. **Scale**: Add load balancing and caching as needed

This LLM completion integration provides a robust foundation for building AI-powered applications with the Model Context Protocol, ensuring production readiness, cost efficiency, and excellent developer experience.
