# Production Deployment Examples

This directory contains comprehensive examples for deploying the MCP Go SDK server in production environments using Docker, Docker Compose, and Kubernetes.

## 🏗️ Architecture Overview

### Production Components
- **MCP Server**: High-performance Model Context Protocol server
- **Load Balancer**: HAProxy for distributing traffic across multiple server instances
- **Observability Stack**:
  - Prometheus for metrics collection
  - Jaeger for distributed tracing
  - Grafana for visualization and dashboards
- **Health Checks**: Kubernetes-style health and readiness probes
- **Security**: Non-root containers, security contexts, and network policies
- **Auto-scaling**: Horizontal Pod Autoscaler for Kubernetes deployments

## 🐳 Docker Deployment

### Single Container
```bash
# Build the Docker image
docker build -f docker/Dockerfile -t mcp-server:latest .

# Run with basic configuration
docker run -p 8080:8080 -p 8081:8081 -p 9090:9090 \
  -e MCP_ENABLE_TRACING=true \
  -e MCP_ENABLE_METRICS=true \
  mcp-server:latest
```

### Configuration via Environment Variables
```bash
docker run -p 8080:8080 \
  -e MCP_SERVER_NAME="my-mcp-server" \
  -e MCP_ENVIRONMENT="production" \
  -e MCP_ENABLE_TRACING=true \
  -e MCP_TRACING_ENDPOINT="http://jaeger:14268/api/traces" \
  -e MCP_ENABLE_METRICS=true \
  -e MCP_ALLOWED_ORIGINS='["https://myapp.com"]' \
  -e MCP_ENABLE_RATE_LIMIT=true \
  -e MCP_RATE_LIMIT=1000 \
  mcp-server:latest
```

## 🐳 Docker Compose Deployment

### Quick Start
```bash
cd docker-compose/
docker-compose up -d
```

### Full Observability Stack
The Docker Compose setup includes:
- **2x MCP Server instances** (load balanced)
- **HAProxy Load Balancer** (port 80, stats on 8404)
- **Prometheus** (metrics collection on port 9090)
- **Jaeger** (tracing UI on port 16686)
- **Grafana** (dashboards on port 3000, admin/admin)
- **Redis** (optional caching and session storage)

### Access Points
- **MCP Server (Load Balanced)**: http://localhost/mcp
- **HAProxy Stats**: http://localhost:8404/stats
- **Prometheus**: http://localhost:9090
- **Jaeger UI**: http://localhost:16686
- **Grafana**: http://localhost:3000 (admin/admin)

### Monitoring Setup
```bash
# View server logs
docker-compose logs -f mcp-server-1 mcp-server-2

# Check health status
curl http://localhost:8081/health
curl http://localhost:8083/health

# View metrics
curl http://localhost:9090/metrics
curl http://localhost:9091/metrics
```

## ☸️ Kubernetes Deployment

### Prerequisites
- Kubernetes cluster (1.20+)
- kubectl configured
- Kustomize (optional, for overlays)

### Base Deployment
```bash
# Deploy base configuration
kubectl apply -k kubernetes/base/

# Check deployment status
kubectl get pods -l app=mcp-server
kubectl get svc mcp-server
```

### Production Overlay
```bash
# Create production namespace
kubectl create namespace mcp-production

# Deploy production configuration
kubectl apply -k kubernetes/overlays/production/

# Monitor deployment
kubectl get pods -n mcp-production -l app=mcp-server
kubectl get hpa -n mcp-production
```

### Kubernetes Features
- **Security**: Non-root containers, read-only filesystems, security contexts
- **High Availability**: Pod anti-affinity, pod disruption budgets
- **Auto-scaling**: Horizontal Pod Autoscaler based on CPU, memory, and custom metrics
- **Health Checks**: Liveness, readiness, and startup probes
- **Resource Management**: Requests and limits for CPU and memory
- **Network Security**: Network policies (in production overlay)
- **Configuration Management**: ConfigMaps and Secrets

### Ingress Configuration
```yaml
# Example ingress (add to production overlay)
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mcp-server-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - mcp.example.com
    secretName: mcp-tls
  rules:
  - host: mcp.example.com
    http:
      paths:
      - path: /mcp
        pathType: Prefix
        backend:
          service:
            name: mcp-server
            port:
              number: 80
```

## 🔧 Configuration

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_SERVER_NAME` | `production-mcp-server` | Server identification name |
| `MCP_PORT` | `8080` | Main server port |
| `MCP_HOST` | `0.0.0.0` | Server bind address |
| `MCP_HEALTH_PORT` | `8081` | Health check port |
| `MCP_METRICS_PORT` | `9090` | Metrics export port |
| `MCP_ENABLE_TRACING` | `true` | Enable OpenTelemetry tracing |
| `MCP_TRACING_ENDPOINT` | `http://jaeger:14268/api/traces` | Tracing collector endpoint |
| `MCP_ENABLE_METRICS` | `true` | Enable Prometheus metrics |
| `MCP_SERVICE_NAME` | `mcp-production-server` | Service name for observability |
| `MCP_ENVIRONMENT` | `production` | Environment label |
| `MCP_ALLOWED_ORIGINS` | `["*"]` | CORS allowed origins (JSON array) |
| `MCP_ENABLE_AUTH` | `false` | Enable authentication |
| `MCP_ENABLE_RATE_LIMIT` | `true` | Enable rate limiting |
| `MCP_RATE_LIMIT` | `1000` | Requests per second limit |
| `MCP_SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |

### Security Configuration
```bash
# Enable authentication
export MCP_ENABLE_AUTH=true
export MCP_AUTH_TOKENS='["token1", "token2"]'

# Restrict origins
export MCP_ALLOWED_ORIGINS='["https://app.example.com", "https://api.example.com"]'

# Configure rate limiting
export MCP_ENABLE_RATE_LIMIT=true
export MCP_RATE_LIMIT=2000
```

## 📊 Monitoring & Observability

### Health Checks
- **Liveness**: `GET /health` - Server is running
- **Readiness**: `GET /ready` - Server is ready to serve requests

### Metrics
The server exports Prometheus metrics on `/metrics`:
- `mcp_production_requests_total` - Total requests counter
- `mcp_production_request_duration_seconds` - Request latency histogram
- `mcp_production_active_connections` - Active connections gauge
- `mcp_production_errors_total` - Error counter by type

### Tracing
Distributed tracing with OpenTelemetry:
- Automatic span creation for all MCP operations
- Trace correlation across service boundaries
- Integration with Jaeger for trace visualization

### Grafana Dashboards
Pre-configured dashboards include:
- **MCP Production Overview**: Request rate, latency, success rate, connections
- **Error Analysis**: Error rates and types
- **Performance Metrics**: Resource utilization and throughput

## 🚀 Performance Tuning

### Resource Allocation
```yaml
# Kubernetes resource configuration
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### Connection Tuning
```bash
# High-performance configuration
export MCP_RATE_LIMIT=5000
export MCP_CONNECTION_POOL_SIZE=100
export MCP_BUFFER_SIZE=65536
```

### Auto-scaling Configuration
```yaml
# HPA configuration for high traffic
spec:
  minReplicas: 5
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        averageUtilization: 70
  - type: Pods
    pods:
      metric:
        name: mcp_production_requests_per_second
      target:
        averageValue: "100"
```

## 🔒 Security Best Practices

### Container Security
- Non-root user execution (UID 65534)
- Read-only root filesystem
- Minimal base image (scratch)
- No privilege escalation
- Dropped capabilities

### Network Security
- Network policies to restrict traffic
- TLS termination at ingress
- Internal service mesh (optional)

### Authentication & Authorization
```bash
# Enable authentication
export MCP_ENABLE_AUTH=true
export MCP_AUTH_TOKENS='["secure-token-1", "secure-token-2"]'

# Configure CORS properly
export MCP_ALLOWED_ORIGINS='["https://trusted-app.com"]'
```

## 🛠️ Troubleshooting

### Common Issues

#### Container Won't Start
```bash
# Check logs
docker logs mcp-server
kubectl logs -l app=mcp-server

# Verify configuration
docker exec -it mcp-server env | grep MCP_
```

#### Health Check Failures
```bash
# Test health endpoint directly
curl http://localhost:8081/health
curl http://localhost:8081/ready

# Check server logs for errors
kubectl logs -f deployment/mcp-server
```

#### Performance Issues
```bash
# Monitor metrics
curl http://localhost:9090/metrics | grep mcp_production

# Check resource usage
kubectl top pods -l app=mcp-server
```

#### Tracing Not Working
```bash
# Verify Jaeger connectivity
curl http://jaeger:14268/api/traces

# Check trace export
kubectl logs -l app=mcp-server | grep -i trace
```

### Debug Mode
```bash
# Enable debug logging
export MCP_LOG_LEVEL=debug
export MCP_ENABLE_DEBUG=true
```

## 📈 Scaling Guidelines

### Horizontal Scaling
- **Light Load**: 3 replicas minimum
- **Medium Load**: 5-10 replicas
- **High Load**: 10-20 replicas with HPA

### Vertical Scaling
- **Memory**: 128Mi-512Mi per replica
- **CPU**: 100m-500m per replica
- **Connections**: 1000-5000 per replica

### Load Testing
```bash
# Test with Docker Compose
cd examples/production-deployment/
docker-compose up -d

# Run load test
cd ../../../benchmarks/
go test -run=TestLoadTest -v
```

## 🔄 CI/CD Integration

### Docker Build Pipeline
```yaml
# GitHub Actions example
- name: Build Docker Image
  run: |
    docker build -f examples/production-deployment/docker/Dockerfile \
      -t mcp-server:${{ github.sha }} .
    docker tag mcp-server:${{ github.sha }} mcp-server:latest
```

### Kubernetes Deployment
```yaml
# Deploy with Kustomize
- name: Deploy to Production
  run: |
    kubectl apply -k examples/production-deployment/kubernetes/overlays/production/
```

## 📚 Additional Resources

- [MCP Specification](https://spec.modelcontextprotocol.io/)
- [Performance Benchmarking](../../../benchmarks/README.md)
- [Observability Guide](../observability/README.md)
- [Security Configuration](../authentication/README.md)

## 🤝 Support

For production deployment questions:
1. Check the troubleshooting section above
2. Review logs and metrics
3. Consult the main project documentation
4. Open an issue with deployment details and error logs
