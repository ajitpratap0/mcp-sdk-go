# MCP Protocol Validation Tool

This tool provides comprehensive validation of MCP (Model Context Protocol) implementations against the official specification, ensuring compliance, interoperability, and performance standards.

## 🎯 Overview

The MCP Protocol Validation Tool is essential for developers building MCP servers, clients, and tools. It provides automated testing to ensure your implementation follows the MCP specification correctly and performs well under various conditions.

## 🔍 What It Validates

### Protocol Compliance
- **JSON-RPC 2.0 Compliance**: Validates message format and request-response patterns
- **MCP Initialization**: Tests proper handshake and capability exchange
- **Message Formats**: Verifies all MCP message types conform to specification
- **Error Handling**: Ensures proper error responses and error codes
- **Pagination Support**: Tests pagination implementation for list operations
- **Notification Handling**: Validates notification message processing

### Capability Testing
- **Tools Capability**: Tests tool listing, calling, and metadata
- **Resources Capability**: Tests resource listing, reading, and subscriptions
- **Prompts Capability**: Tests prompt listing and retrieval
- **Transport Layer**: Validates transport-specific implementations

### Performance Characteristics
- **Response Time Benchmarks**: Measures operation latency
- **Throughput Testing**: Tests sustained operation rates
- **Concurrency Testing**: Validates concurrent request handling
- **Memory Usage Analysis**: Monitors resource consumption patterns

### Security and Reliability
- **Input Validation**: Tests handling of malformed requests
- **Error Recovery**: Validates graceful error handling
- **Resource Limits**: Tests behavior under resource constraints

## 🚀 Quick Start

### Running the Validator

```bash
# Run the protocol validation demo
go run main.go

# The validator will:
# 1. Start a test MCP server
# 2. Run comprehensive protocol tests
# 3. Generate a detailed validation report
```

### Output Example

```
=== MCP Protocol Validation Demo ===
Setting up Protocol Compliance test suite
Running test suite: Protocol Compliance
Running test: initialization_sequence
  ✓ initialization_sequence: Initialization sequence completed successfully (45ms)
Running test: capabilities_exchange
  ✓ capabilities_exchange: Capabilities exchange is compliant (12ms)
Running test: jsonrpc_compliance
  ✓ jsonrpc_compliance: JSON-RPC communication successful (23ms)
Running test: message_format_validation
  ✓ message_format_validation: Message formats are compliant (18ms)
Running test: error_handling
  ✓ error_handling: Error handling is compliant (34ms)
Running test: pagination_support
  ⚠ pagination_support: Pagination appears to work correctly (15ms)

================================================================================
MCP PROTOCOL VALIDATION REPORT
================================================================================
Server: http://localhost:8084/mcp
Total Tests: 12
Passed: 10
Failed: 0
Warnings: 2
Errors: 0
Skipped: 0
================================================================================

OVERALL STATUS: PASSED WITH WARNINGS
```

## 🧪 Test Suites

### 1. Protocol Compliance Test Suite

Tests core MCP protocol specification compliance:

#### Initialization Sequence Test
- Validates proper MCP handshake
- Checks capability exchange
- Verifies client-server negotiation

#### Capabilities Exchange Test
- Tests capability advertisement
- Validates capability structure
- Checks required vs optional capabilities

#### JSON-RPC Compliance Test
- Validates JSON-RPC 2.0 message format
- Tests request-response patterns
- Verifies proper ID handling

#### Message Format Test
- Validates MCP-specific message structures
- Tests tool, resource, and prompt formats
- Checks required fields and data types

#### Error Handling Test
- Tests error response formats
- Validates error codes
- Checks error message structure

#### Pagination Test
- Tests list operation pagination
- Validates cursor-based pagination
- Checks limit parameter handling

### 2. Performance Test Suite

Benchmarks and performance characteristics:

#### Response Time Test
```go
// Measures response times for various operations
result := validator.measureResponseTimes(operations)
```

#### Throughput Test
```go
// Tests sustained operation rates
throughput := validator.measureThroughput(duration)
```

#### Concurrency Test
```go
// Tests concurrent request handling
result := validator.testConcurrency(workers, operations)
```

#### Memory Usage Test
```go
// Monitors memory consumption
usage := validator.monitorMemoryUsage(operations)
```

## 🔧 Configuration

### Validator Configuration

```go
config := ValidatorConfig{
    // Server connection
    ServerEndpoint: "http://localhost:8080/mcp",
    TransportType:  "streamable_http", // or "stdio"

    // Test execution
    TestTimeout:           30 * time.Second,
    MaxConcurrentTests:    5,
    StrictMode:            false,
    EnablePerformanceTests: true,

    // Output configuration
    OutputFormat: "text", // "json", "junit", "text"
    OutputFile:   "validation-report.json",

    // Test selection
    TestSuites: []string{"Protocol Compliance", "Performance Tests"},
    SkipTests:  []string{"memory_usage_test"},
    FailFast:   false,
    Verbose:    true,
}
```

### Transport Types

#### HTTP Transport
```go
config := ValidatorConfig{
    ServerEndpoint: "http://localhost:8080/mcp",
    TransportType:  "streamable_http",
}
```

#### Stdio Transport
```go
config := ValidatorConfig{
    ServerEndpoint: "", // Not used for stdio
    TransportType:  "stdio",
}
```

## 📊 Test Categories and Severity

### Test Categories
- `protocol`: Core protocol compliance
- `capabilities`: Capability-specific tests
- `tools`: Tools capability validation
- `resources`: Resources capability validation
- `prompts`: Prompts capability validation
- `transport`: Transport layer validation
- `performance`: Performance and benchmarking
- `security`: Security-related tests

### Test Severity Levels
- `critical`: Must pass for basic compliance
- `error`: Should pass for full compliance
- `warning`: Best practices and optional features
- `info`: Informational and performance data

## 📋 Custom Test Development

### Creating Custom Test Suites

```go
type CustomTestSuite struct {
    name        string
    description string
}

func (s *CustomTestSuite) GetSuiteName() string {
    return s.name
}

func (s *CustomTestSuite) GetTestCases() []TestCase {
    return []TestCase{
        &CustomTest{},
    }
}

func (s *CustomTestSuite) Setup(ctx context.Context, validator *ProtocolValidator) error {
    // Initialize test suite
    return nil
}

func (s *CustomTestSuite) Teardown(ctx context.Context, validator *ProtocolValidator) error {
    // Cleanup test suite
    return nil
}
```

### Creating Custom Test Cases

```go
type CustomTest struct{}

func (t *CustomTest) GetTestName() string {
    return "custom_validation_test"
}

func (t *CustomTest) GetDescription() string {
    return "Validates custom functionality"
}

func (t *CustomTest) GetCategory() TestCategory {
    return CategoryProtocol
}

func (t *CustomTest) GetSeverity() TestSeverity {
    return SeverityError
}

func (t *CustomTest) Run(ctx context.Context, validator *ProtocolValidator) TestResult {
    startTime := time.Now()

    result := TestResult{
        TestName:         t.GetTestName(),
        Category:         t.GetCategory(),
        Severity:         t.GetSeverity(),
        StartTime:        startTime,
        SpecificationRef: "Custom Specification Section",
        ComplianceLevel:  "REQUIRED",
        Metadata:         make(map[string]interface{}),
    }

    // Perform custom validation logic
    if err := performCustomValidation(ctx, validator); err != nil {
        result.Status = StatusFailed
        result.Message = "Custom validation failed"
        result.Details = err.Error()
    } else {
        result.Status = StatusPassed
        result.Message = "Custom validation successful"
        result.Details = "All custom checks passed"
    }

    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)

    return result
}

func performCustomValidation(ctx context.Context, validator *ProtocolValidator) error {
    // Implement custom validation logic
    return nil
}
```

### Registering Custom Tests

```go
validator := NewProtocolValidator(config)

// Register built-in test suites
validator.RegisterTestSuite(NewProtocolComplianceTestSuite())
validator.RegisterTestSuite(NewPerformanceTestSuite())

// Register custom test suite
validator.RegisterTestSuite(&CustomTestSuite{
    name: "Custom Tests",
    description: "Application-specific validation tests",
})
```

## 📈 Report Formats

### Text Report (Default)

Human-readable console output with color coding and detailed results:

```
================================================================================
MCP PROTOCOL VALIDATION REPORT
================================================================================
Server: http://localhost:8080/mcp
Total Tests: 15
Passed: 12
Failed: 1
Warnings: 2
Errors: 0
Skipped: 0
================================================================================

PROTOCOL TESTS
----------------------------------------
  ✓ initialization_sequence        Initialization sequence completed successfully (45ms)
  ✓ capabilities_exchange          Capabilities exchange is compliant (12ms)
  ✓ jsonrpc_compliance             JSON-RPC communication successful (23ms)
  ✗ message_format_validation      Message format validation failed (18ms)
    Details: Tool 'invalid_tool' missing required 'name' field
  ⚠ error_handling                 Error handling has minor issues (34ms)
    Details: Some error responses missing specification-compliant codes

PERFORMANCE TESTS
----------------------------------------
  ✓ response_time_benchmark        Response time measurements completed (2.1s)
    Details: Avg: 25ms, Min: 15ms, Max: 45ms (100 samples)
  ✓ throughput_benchmark           Throughput test completed (10.2s)
    Details: 85.3 ops/sec, 2.1% error rate (853 ops in 10s)

================================================================================
OVERALL STATUS: FAILED
================================================================================
```

### JSON Report

Machine-readable JSON format for integration with CI/CD systems:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "server": "http://localhost:8080/mcp",
  "validator_info": {
    "name": "MCP Protocol Validator",
    "version": "1.0.0"
  },
  "server_info": {
    "name": "My MCP Server",
    "version": "2.1.0"
  },
  "server_capabilities": {
    "tools": {
      "listChanged": true
    },
    "resources": {
      "subscribe": true,
      "listChanged": true
    }
  },
  "test_results": [
    {
      "test_name": "initialization_sequence",
      "category": "protocol",
      "severity": "critical",
      "status": "passed",
      "message": "Initialization sequence completed successfully",
      "details": "Client successfully initialized and capabilities exchanged",
      "duration": "45ms",
      "start_time": "2024-01-15T10:30:01Z",
      "end_time": "2024-01-15T10:30:01Z",
      "specification_ref": "MCP Specification Section 2.1 - Initialization",
      "compliance_level": "REQUIRED",
      "metadata": {
        "server_capabilities": {...},
        "client_capabilities": {...}
      }
    }
  ],
  "summary": {
    "total_tests": 15,
    "passed": 12,
    "failed": 1,
    "warnings": 2,
    "errors": 0,
    "skipped": 0
  }
}
```

### JUnit XML Report

JUnit-compatible XML format for integration with testing frameworks:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="MCP Protocol Validation" tests="15" failures="1" time="12.450">
  <testcase name="initialization_sequence" classname="protocol" time="0.045"/>
  <testcase name="capabilities_exchange" classname="protocol" time="0.012"/>
  <testcase name="message_format_validation" classname="protocol" time="0.018">
    <failure message="Message format validation failed">Tool 'invalid_tool' missing required 'name' field</failure>
  </testcase>
  <testcase name="error_handling" classname="protocol" time="0.034"/>
  <testcase name="response_time_benchmark" classname="performance" time="2.100"/>
</testsuite>
```

## 🏗️ Integration with CI/CD

### GitHub Actions

```yaml
name: MCP Protocol Validation

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  validate-protocol:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21

    - name: Start MCP Server
      run: |
        go run ./cmd/server &
        sleep 5

    - name: Run Protocol Validation
      run: |
        cd examples/protocol-validator
        go run main.go \
          --server-endpoint "http://localhost:8080/mcp" \
          --output-format "junit" \
          --output-file "validation-results.xml" \
          --fail-fast

    - name: Publish Test Results
      uses: mikepenz/action-junit-report@v3
      if: always()
      with:
        report_paths: 'examples/protocol-validator/validation-results.xml'
        check_name: 'MCP Protocol Validation'
```

### Docker Integration

```dockerfile
FROM golang:1.21-alpine AS validator

WORKDIR /app
COPY . .
RUN go build -o protocol-validator examples/protocol-validator/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates curl
WORKDIR /root/

COPY --from=validator /app/protocol-validator .

# Validation script
COPY validate.sh .
RUN chmod +x validate.sh

CMD ["./validate.sh"]
```

```bash
#!/bin/sh
# validate.sh

# Wait for server to be ready
until curl -f http://server:8080/health; do
  echo "Waiting for server..."
  sleep 2
done

# Run validation
./protocol-validator \
  --server-endpoint "http://server:8080/mcp" \
  --output-format "json" \
  --output-file "/reports/validation.json"
```

## 🔧 Advanced Configuration

### Environment Variables

```bash
# Server configuration
export MCP_SERVER_ENDPOINT="http://localhost:8080/mcp"
export MCP_TRANSPORT_TYPE="streamable_http"

# Test configuration
export MCP_TEST_TIMEOUT="30s"
export MCP_ENABLE_PERFORMANCE_TESTS="true"
export MCP_STRICT_MODE="false"

# Output configuration
export MCP_OUTPUT_FORMAT="json"
export MCP_OUTPUT_FILE="validation-report.json"
export MCP_VERBOSE="true"

# Run validator
go run main.go
```

### Configuration File

```yaml
# protocol-validator-config.yaml
server:
  endpoint: "http://localhost:8080/mcp"
  transport_type: "streamable_http"
  timeout: "30s"

testing:
  strict_mode: false
  enable_performance_tests: true
  max_concurrent_tests: 5
  fail_fast: false

test_suites:
  - "Protocol Compliance"
  - "Performance Tests"

skip_tests:
  - "memory_usage_test"

output:
  format: "json"  # text, json, junit
  file: "validation-report.json"
  verbose: true
```

```go
// Load configuration from file
config, err := LoadValidatorConfig("protocol-validator-config.yaml")
if err != nil {
    log.Fatal(err)
}

validator := NewProtocolValidator(config)
```

## 📚 Best Practices

### Pre-Deployment Validation

1. **Run Full Test Suite**: Always run all tests before deployment
2. **Performance Baselines**: Establish performance baselines and validate against them
3. **Error Scenario Testing**: Ensure proper error handling in all scenarios
4. **Capability Coverage**: Test all advertised capabilities thoroughly

### Continuous Validation

1. **Automated Testing**: Integrate validation into CI/CD pipelines
2. **Regression Testing**: Run validation after any protocol-related changes
3. **Performance Monitoring**: Track performance trends over time
4. **Compliance Monitoring**: Regular validation in production environments

### Custom Validation

1. **Domain-Specific Tests**: Add tests for application-specific requirements
2. **Integration Tests**: Validate entire workflows, not just individual operations
3. **Load Testing**: Test realistic load scenarios
4. **Security Testing**: Add security-focused validation tests

## 🐛 Troubleshooting

### Common Issues

#### Connection Failures
```bash
# Check server is running
curl -f http://localhost:8080/health

# Check endpoint configuration
echo $MCP_SERVER_ENDPOINT

# Verify transport type
# stdio: No endpoint needed
# http: Must include full URL with /mcp path
```

#### Test Failures
```bash
# Run in verbose mode for detailed output
go run main.go --verbose

# Run specific test suite only
go run main.go --test-suites="Protocol Compliance"

# Skip problematic tests
go run main.go --skip-tests="memory_usage_test,concurrency_test"
```

#### Performance Issues
```bash
# Disable performance tests if causing issues
go run main.go --enable-performance-tests=false

# Reduce concurrent test limit
go run main.go --max-concurrent-tests=1

# Increase test timeout
go run main.go --test-timeout=60s
```

### Debug Mode

```go
// Enable debug logging
config.Verbose = true
config.Debug = true

// Add custom logging
validator.SetLogger(customLogger)
```

## ⚡ Next Steps

1. **Set Up Validation**: Configure validator for your MCP implementation
2. **Run Initial Tests**: Execute full validation suite
3. **Fix Issues**: Address any compliance or performance issues
4. **Automate Testing**: Integrate into your development workflow
5. **Monitor Continuously**: Set up regular validation checks
6. **Extend Tests**: Add custom tests for your specific use cases

This protocol validation tool ensures your MCP implementation meets specification requirements and performs optimally in production environments.
