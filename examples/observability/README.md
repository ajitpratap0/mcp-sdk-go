# MCP Observability Example

This example demonstrates comprehensive observability for MCP services using:
- **OpenTelemetry** for distributed tracing
- **Prometheus** for metrics collection and monitoring
- **Structured logging** for debugging

## Features

### OpenTelemetry Tracing
- Distributed trace propagation across client/server boundaries
- Automatic span creation for all MCP operations
- Custom span attributes and events
- Error recording and status tracking
- Support for OTLP, Jaeger, and other exporters

### Prometheus Metrics
- Request/response latency histograms
- Operation success/failure counters
- Active connection gauges
- Batch operation size tracking
- Custom business metrics
- MCP-specific metric dimensions

### Enhanced Logging
- Structured logging with context
- Request/response correlation
- Performance metrics in logs
- Error categorization

## Prerequisites

1. **Jaeger** for trace visualization (optional):
```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest
```

2. **Prometheus** for metrics (optional):
```bash
docker run -d --name prometheus \
  -p 9091:9090 \
  -v prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

Example `prometheus.yml`:
```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'mcp-example'
    static_configs:
      - targets: ['host.docker.internal:9090']
```

## Running the Example

```bash
go run main.go
```

The example will:
1. Start an MCP server with full observability on `http://localhost:8080/mcp`
2. Expose Prometheus metrics on `http://localhost:9090/metrics`
3. Send traces to Jaeger at `localhost:4317`
4. Run a client that performs various operations
5. Generate metrics and traces for analysis

## Viewing Results

### Traces in Jaeger
1. Open Jaeger UI: http://localhost:16686
2. Select service: `mcp-example`
3. View distributed traces showing:
   - Client → Server request flow
   - Operation timings
   - Error details
   - Custom attributes

### Metrics in Prometheus
1. View raw metrics: http://localhost:9090/metrics
2. Prometheus UI: http://localhost:9091
3. Example queries:
   ```promql
   # Request rate
   rate(mcp_example_request_total[5m])

   # P99 latency
   histogram_quantile(0.99, rate(mcp_example_request_duration_milliseconds_bucket[5m]))

   # Error rate
   rate(mcp_example_request_total{status="error"}[5m])
   ```

## Configuration Options

### Tracing Configuration
```go
tracingConfig := observability.TracingConfig{
    ServiceName:    "my-service",
    ServiceVersion: "1.0.0",
    Environment:    "production",

    // Exporter options
    ExporterType: observability.ExporterTypeOTLPGRPC,
    Endpoint:     "otel-collector:4317",

    // Sampling
    SampleRate:   0.1,  // Sample 10% of traces
    AlwaysSample: []string{"critical-operation"},
    NeverSample:  []string{"health-check"},
}
```

### Metrics Configuration
```go
metricsConfig := observability.MetricsConfig{
    ServiceName:    "my-service",
    MetricsPath:    "/metrics",
    MetricsPort:    9090,

    // Custom buckets for latency (milliseconds)
    HistogramBuckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},

    // Constant labels
    ConstLabels: prometheus.Labels{
        "region": "us-east-1",
        "team":   "platform",
    },
}
```

## Production Considerations

1. **Sampling**: Use appropriate sampling rates to control data volume
2. **Security**: Use TLS for OTLP endpoints in production
3. **Performance**: Disable payload capture in production
4. **Storage**: Configure trace and metric retention policies
5. **Alerting**: Set up Prometheus alerts for SLI violations

## Metrics Reference

### Request Metrics
- `mcp_example_request_duration_milliseconds`: Request latency histogram
- `mcp_example_request_total`: Request counter by method and status
- `mcp_example_notification_duration_milliseconds`: Notification latency
- `mcp_example_batch_request_size`: Batch request size distribution

### Transport Metrics
- `mcp_example_active_connections`: Current active connections
- `mcp_example_connection_state`: Connection state (connected/disconnected)
- `mcp_example_transport_event_duration_milliseconds`: Transport operations

### Resource Metrics
- `mcp_example_tool_call_duration_milliseconds`: Tool execution time
- `mcp_example_resource_operation_duration_milliseconds`: Resource operations

## Troubleshooting

### No traces appearing
1. Check Jaeger is running: `docker ps`
2. Verify OTLP endpoint is accessible
3. Check for errors in application logs
4. Ensure sampling rate > 0

### No metrics
1. Access metrics endpoint: `curl http://localhost:9090/metrics`
2. Check Prometheus scrape configuration
3. Verify metrics are being recorded

### High memory usage
1. Reduce sampling rate
2. Disable payload capture
3. Adjust batch export settings
