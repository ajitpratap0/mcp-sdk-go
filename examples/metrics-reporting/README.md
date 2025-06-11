# Metrics Aggregation and Reporting

This example demonstrates comprehensive metrics collection, aggregation, and reporting for MCP applications, providing enterprise-grade observability and monitoring capabilities.

## 🎯 Overview

Modern distributed applications require robust monitoring and observability infrastructure. This metrics aggregation system provides production-ready patterns for:

- **Multi-Source Collection**: Aggregate metrics from multiple MCP servers, clients, and custom sources
- **Time Series Storage**: Store and query historical metrics data with configurable retention
- **Real-Time Alerting**: Configurable alerting rules with multiple notification channels
- **Web Dashboard**: Live metrics visualization and monitoring interface
- **Automated Reporting**: Scheduled report generation in multiple formats
- **Extensible Architecture**: Easy integration with existing monitoring infrastructure
- **Production Patterns**: Battle-tested observability patterns for high-scale deployments

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Dashboard Server                      │
│                 (Web Interface)                        │
├─────────────────────────────────────────────────────────┤
│               Metrics Aggregator                       │
│  ┌─────────────────────────────────────────────────────┐│
│  │            Alert Manager                            ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐   ││
│  │  │Console      │ │    Email    │ │   Webhook   │   ││
│  │  │ Handler     │ │   Handler   │ │   Handler   │   ││
│  │  └─────────────┘ └─────────────┘ └─────────────┘   ││
│  └─────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────┐│
│  │           Time Series Storage                       ││
│  │     (In-Memory / InfluxDB / Prometheus)             ││
│  └─────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────┤
│                Metrics Sources                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐       │
│  │ MCP Server  │ │ MCP Client  │ │   Custom    │       │
│  │   Source    │ │   Source    │ │   Source    │       │
│  └─────────────┘ └─────────────┘ └─────────────┘       │
└─────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. **Metrics Aggregator**
Central coordination of metrics collection, storage, and processing:
- Source management and registration
- Periodic metrics collection
- Data validation and normalization
- Alert rule evaluation
- Storage coordination

#### 2. **Time Series Storage**
Scalable storage for historical metrics data:
- Configurable retention policies
- Efficient time-based queries
- Data aggregation and downsampling
- Multiple backend support (Memory, InfluxDB, Prometheus)

#### 3. **Alert Manager**
Real-time alerting with configurable rules:
- Threshold-based alerting
- Multiple notification channels
- Alert state management
- Escalation policies

#### 4. **Dashboard Server**
Web-based monitoring interface:
- Real-time metrics visualization
- Historical trend analysis
- Alert status monitoring
- Source health overview

## 🚀 Quick Start

### Running the Example

```bash
# Run the metrics aggregation demo
go run main.go

# The system will start:
# - Metrics collection every 5 seconds
# - Web dashboard on http://localhost:8089
# - MCP server on http://localhost:8088/mcp
# - Alert monitoring with console output
```

### Accessing the Dashboard

Once running, open your browser to [http://localhost:8089](http://localhost:8089) to view:
- Real-time metrics overview
- Active alerts
- Source status
- Historical data trends

## 📊 Metrics Sources

### Built-in Sources

#### MCP Server Metrics
Automatically collected from MCP server instances:

```go
serverSource := NewMCPServerMetricsSource(mcpServer)
aggregator.AddSource("my-server", serverSource)
```

**Collected Metrics:**
- `requests_total`: Total requests processed
- `errors_total`: Total errors encountered
- `response_time_ms`: Average response time
- `active_connections`: Current active connections

#### MCP Client Metrics
Collected from MCP client connections:

```go
clientSource := NewMCPClientMetricsSource(mcpClient)
aggregator.AddSource("my-client", clientSource)
```

**Collected Metrics:**
- `requests_sent`: Total requests sent
- `responses_received`: Total responses received
- `connection_errors`: Connection error count
- `avg_latency_ms`: Average request latency

### Custom Metrics Sources

Create custom sources for application-specific metrics:

```go
type CustomMetricsSource struct {
    id   string
    name string
}

func (s *CustomMetricsSource) GetMetrics(ctx context.Context) (MetricsSnapshot, error) {
    return MetricsSnapshot{
        Timestamp: time.Now(),
        SourceID:  s.id,
        Metrics: map[string]interface{}{
            "custom_metric_1": 42.0,
            "custom_metric_2": 100.0,
        },
        Labels: map[string]string{
            "component": "custom",
            "version":   "1.0.0",
        },
    }, nil
}

func (s *CustomMetricsSource) GetSourceInfo() SourceInfo {
    return SourceInfo{
        ID:          s.id,
        Name:        s.name,
        Type:        "custom",
        Description: "Custom application metrics",
    }
}

// Register the source
aggregator.AddSource("custom-app", &CustomMetricsSource{
    id:   "custom-app",
    name: "My Application",
})
```

## 🚨 Alert Management

### Alert Rules Configuration

Define custom alert rules for proactive monitoring:

```go
// High error rate alert
alertManager.AddRule(AlertRule{
    ID:          "high-error-rate",
    Name:        "High Error Rate",
    Description: "Error rate exceeds 5%",
    Metric:      "errors_total",
    Condition:   ConditionGreaterThan,
    Threshold:   5.0,
    Duration:    time.Minute,
    Severity:    SeverityWarning,
    Enabled:     true,
    Labels: map[string]string{
        "team": "backend",
    },
})

// Response time alert
alertManager.AddRule(AlertRule{
    ID:          "slow-responses",
    Name:        "Slow Response Times",
    Description: "Average response time exceeds 100ms",
    Metric:      "response_time_ms",
    Condition:   ConditionGreaterThan,
    Threshold:   100.0,
    Duration:    time.Minute * 2,
    Severity:    SeverityError,
    Enabled:     true,
})
```

### Alert Conditions

Supported alert conditions:
- `ConditionGreaterThan`: Value > threshold
- `ConditionLessThan`: Value < threshold
- `ConditionEquals`: Value == threshold
- `ConditionNotEquals`: Value != threshold
- `ConditionGreaterOrEqual`: Value >= threshold
- `ConditionLessOrEqual`: Value <= threshold

### Alert Severity Levels

- `SeverityInfo`: Informational alerts
- `SeverityWarning`: Warning-level issues
- `SeverityError`: Error conditions
- `SeverityCritical`: Critical system issues

### Custom Alert Handlers

Implement custom notification channels:

```go
type SlackAlertHandler struct {
    WebhookURL string
    Channel    string
}

func (h *SlackAlertHandler) HandleAlert(alert Alert) error {
    message := fmt.Sprintf("🚨 *%s*\n%s\nValue: %.2f | Threshold: %.2f",
        alert.Rule.Name,
        alert.Message,
        alert.Value,
        alert.Rule.Threshold,
    )

    // Send to Slack webhook
    return h.sendSlackMessage(message)
}

func (h *SlackAlertHandler) GetHandlerInfo() HandlerInfo {
    return HandlerInfo{
        Name:        "Slack",
        Type:        "webhook",
        Description: "Send alerts to Slack channel",
    }
}

// Register the handler
alertManager.AddHandler(&SlackAlertHandler{
    WebhookURL: "https://hooks.slack.com/services/...",
    Channel:    "#alerts",
})
```

## 📈 Time Series Queries

### Query Historical Data

Retrieve historical metrics with flexible queries:

```go
query := TimeSeriesQuery{
    Metric:     "requests_total",
    Sources:    []string{"server", "client"},
    StartTime:  time.Now().Add(-time.Hour),
    EndTime:    time.Now(),
    Interval:   time.Minute * 5,
    Aggregator: "avg",
    Labels: map[string]string{
        "component": "server",
    },
}

result, err := storage.GetTimeSeriesData(ctx, query)
if err != nil {
    log.Printf("Query failed: %v", err)
    return
}

// Process results
for _, point := range result.Points {
    fmt.Printf("%s: %.2f\n",
        point.Timestamp.Format("15:04:05"),
        point.Value)
}
```

### Aggregation Functions

- `sum`: Sum all values in the interval
- `avg`: Average of all values
- `max`: Maximum value in the interval
- `min`: Minimum value in the interval

### Query Performance

For optimal query performance:
- Use specific time ranges
- Filter by relevant sources
- Choose appropriate intervals
- Apply labels for filtering

## 🖥️ Dashboard API

### REST Endpoints

#### GET /api/metrics
Get current metrics from all sources:

```bash
curl http://localhost:8089/api/metrics
```

#### GET /api/sources
List all registered metrics sources:

```bash
curl http://localhost:8089/api/sources
```

#### GET /api/timeseries?metric=requests_total
Get time series data for a specific metric:

```bash
curl "http://localhost:8089/api/timeseries?metric=requests_total"
```

#### GET /api/alerts
Get current alert states:

```bash
curl http://localhost:8089/api/alerts
```

### Custom Dashboard Integration

Integrate with existing dashboards using the API:

```javascript
// Fetch current metrics
async function fetchMetrics() {
    const response = await fetch('/api/metrics');
    const metrics = await response.json();

    // Update dashboard charts
    updateCharts(metrics);
}

// Fetch time series data
async function fetchTimeSeries(metric) {
    const response = await fetch(`/api/timeseries?metric=${metric}`);
    const data = await response.json();

    // Render trend chart
    renderChart(data.points);
}

// Auto-refresh every 5 seconds
setInterval(fetchMetrics, 5000);
```

## 📋 Report Generation

### Automated Reports

Configure automated report generation:

```go
reportGenerator := NewReportGenerator(aggregator)

// Add daily summary report
template := ReportTemplate{
    Name:        "Daily Summary",
    Description: "Daily metrics summary report",
    Format:      FormatHTML,
    Schedule:    "0 9 * * *", // 9 AM daily
    Sections: []ReportSection{
        {
            Title: "Metrics Overview",
            Type:  SectionMetricsSummary,
            Config: map[string]interface{}{
                "period": "24h",
            },
        },
        {
            Title: "Alerts Summary",
            Type:  SectionAlerts,
            Config: map[string]interface{}{
                "show_resolved": true,
            },
        },
    },
}

reportGenerator.AddTemplate(template)
```

### Report Formats

- **HTML**: Rich formatted reports with charts
- **JSON**: Machine-readable data export
- **CSV**: Spreadsheet-compatible format

### Custom Report Sections

Create custom report sections:

```go
type CustomReportSection struct {
    aggregator *MetricsAggregator
}

func (s *CustomReportSection) Generate(config map[string]interface{}) (ReportData, error) {
    // Generate custom report data
    return ReportData{
        Title:   "Custom Section",
        Content: "Custom report content...",
        Charts:  []ChartData{},
    }, nil
}
```

## 🔧 Configuration

### Aggregator Configuration

```go
config := AggregatorConfig{
    // Collection frequency
    CollectionInterval: 10 * time.Second,

    // Data retention period
    RetentionPeriod: 7 * 24 * time.Hour,

    // Storage backend
    StorageType: "influxdb",
    StorageConfig: map[string]interface{}{
        "url":      "http://localhost:8086",
        "database": "mcp_metrics",
        "username": "admin",
        "password": "secret",
    },

    // Dashboard settings
    DashboardPort: 8080,

    // Alert configuration
    AlertsEnabled: true,

    // Report settings
    ReportsEnabled: true,
    ReportSchedule: "0 */6 * * *", // Every 6 hours
}
```

### Storage Backends

#### InfluxDB Backend
For production time series storage:

```go
type InfluxDBStorage struct {
    client   influxdb.Client
    database string
}

func NewInfluxDBStorage(url, database, username, password string) *InfluxDBStorage {
    client, err := influxdb.NewHTTPClient(influxdb.HTTPConfig{
        Addr:     url,
        Username: username,
        Password: password,
    })
    if err != nil {
        log.Fatal(err)
    }

    return &InfluxDBStorage{
        client:   client,
        database: database,
    }
}
```

#### Prometheus Backend
For integration with Prometheus:

```go
type PrometheusStorage struct {
    registry *prometheus.Registry
    gateway  *push.Gateway
}

func NewPrometheusStorage(gatewayURL string) *PrometheusStorage {
    registry := prometheus.NewRegistry()
    gateway := push.New(gatewayURL, "mcp_metrics").Gatherer(registry)

    return &PrometheusStorage{
        registry: registry,
        gateway:  gateway,
    }
}
```

## 🚀 Production Deployment

### Docker Configuration

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o metrics-server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/metrics-server .

# Configuration
ENV COLLECTION_INTERVAL=10s
ENV RETENTION_PERIOD=168h
ENV DASHBOARD_PORT=8080
ENV ALERTS_ENABLED=true

EXPOSE 8080
CMD ["./metrics-server"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-metrics-aggregator
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mcp-metrics-aggregator
  template:
    spec:
      containers:
      - name: aggregator
        image: mcp-metrics-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: COLLECTION_INTERVAL
          value: "10s"
        - name: RETENTION_PERIOD
          value: "168h"
        - name: STORAGE_TYPE
          value: "influxdb"
        - name: INFLUXDB_URL
          value: "http://influxdb:8086"
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Production Monitoring

#### Health Checks

```go
func (ma *MetricsAggregator) HealthCheck() map[string]interface{} {
    ma.mutex.RLock()
    sourceCount := len(ma.sources)
    ma.mutex.RUnlock()

    return map[string]interface{}{
        "status":        "healthy",
        "sources":       sourceCount,
        "last_collection": ma.lastCollection,
        "storage_health": ma.storage.HealthCheck(),
    }
}
```

#### Prometheus Integration

```go
// Export metrics to Prometheus
func (ma *MetricsAggregator) RegisterPrometheusMetrics() {
    sourceGauge := prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_metrics_sources_total",
            Help: "Number of registered metrics sources",
        },
        []string{"type"},
    )

    prometheus.MustRegister(sourceGauge)

    // Update metrics
    go func() {
        for {
            ma.updatePrometheusMetrics(sourceGauge)
            time.Sleep(30 * time.Second)
        }
    }()
}
```

## 🔍 Troubleshooting

### Common Issues

#### High Memory Usage
```bash
# Check metrics retention settings
# Reduce retention period or increase cleanup frequency
config.RetentionPeriod = 24 * time.Hour
```

#### Missing Metrics
```bash
# Verify source registration
curl http://localhost:8089/api/sources

# Check collection logs
grep "Failed to collect" /var/log/metrics-aggregator.log
```

#### Alert Spam
```bash
# Adjust alert thresholds
# Add alert dampening/duration requirements
rule.Duration = 5 * time.Minute
```

### Debug Mode

Enable debug logging for troubleshooting:

```go
log.SetLevel(log.DebugLevel)

// Enable detailed collection logging
config.Debug = true
config.VerboseLogging = true
```

### Performance Tuning

#### Collection Optimization
- Increase collection interval for less critical metrics
- Use batch collection for multiple sources
- Implement source health checks

#### Storage Optimization
- Use appropriate time series databases for scale
- Configure data downsampling for long-term storage
- Implement data compression

#### Alert Optimization
- Use alert grouping to reduce noise
- Implement alert dampening
- Configure escalation policies

## 📚 Integration Examples

### Grafana Integration

```yaml
# Grafana dashboard configuration
dashboard:
  title: "MCP Metrics"
  panels:
    - title: "Request Rate"
      type: "graph"
      targets:
        - expr: "rate(mcp_requests_total[5m])"
          legendFormat: "{{source}}"

    - title: "Error Rate"
      type: "stat"
      targets:
        - expr: "rate(mcp_errors_total[5m]) / rate(mcp_requests_total[5m])"
```

### ELK Stack Integration

```go
// Logstash output handler
type LogstashAlertHandler struct {
    endpoint string
}

func (h *LogstashAlertHandler) HandleAlert(alert Alert) error {
    logEntry := map[string]interface{}{
        "@timestamp": alert.Timestamp,
        "alert_id":   alert.ID,
        "rule_name":  alert.Rule.Name,
        "severity":   alert.Rule.Severity,
        "value":      alert.Value,
        "threshold":  alert.Rule.Threshold,
        "status":     alert.Status,
    }

    return h.sendToLogstash(logEntry)
}
```

## ⚡ Next Steps

1. **Deploy Infrastructure**: Set up time series database (InfluxDB/Prometheus)
2. **Configure Sources**: Register all MCP components as metrics sources
3. **Set Up Alerts**: Define appropriate alert rules for your environment
4. **Create Dashboard**: Build custom dashboards for your specific metrics
5. **Automate Reports**: Schedule regular metrics reports
6. **Integrate Monitoring**: Connect with existing monitoring infrastructure
7. **Scale**: Add horizontal scaling and high availability

This comprehensive metrics aggregation system provides the foundation for enterprise-grade observability of MCP applications, ensuring you have complete visibility into system performance and health.
