# MCP Go SDK Examples

This directory contains comprehensive examples demonstrating the full capabilities of the Model Context Protocol Go SDK, from basic usage to enterprise-grade production deployments.

## 🎯 Quick Navigation

### 🏗️ **Core Examples** (Start Here)
- **[Simple Server](simple-server/)** - Basic MCP server with tools, resources, and prompts
- **[Simple Client](simple-client/)** - Basic client connecting to MCP servers
- **[Stdio Client](stdio-client/)** - Standard stdio transport (MCP spec required)
- **[Streamable HTTP Client](streamable-http-client/)** - HTTP+SSE client transport
- **[Streamable HTTP Server](streamable-http-server/)** - HTTP+SSE server implementation

### ⚡ **Advanced Protocol Features**
- **[Batch Processing](batch-processing/)** - High-performance JSON-RPC 2.0 batch operations
- **[Pagination Example](pagination-example/)** - Manual and automatic pagination patterns
- **[Client Callbacks](client-callbacks/)** - Sampling and resource change callbacks
- **[Resource Subscriptions](resource-subscriptions/)** - Real-time resource updates

### 🔐 **Enterprise Security & Authentication**
- **[Authentication](authentication/)** - Bearer tokens, API keys, RBAC integration
- **[Error Recovery](error-recovery/)** - Circuit breakers, retry logic, graceful degradation
- **[Custom Transport](custom-transport/)** - WebSocket transport with middleware integration

### 📊 **Observability & Monitoring**
- **[Observability](observability/)** - OpenTelemetry tracing and Prometheus metrics
- **[Metrics Reporting](metrics-reporting/)** - Comprehensive monitoring and alerting stack

### 🛠️ **Developer Tools**
- **[Development Server](development-server/)** - Hot reload with live dashboard
- **[Code Generator](code-generator/)** - Provider scaffolding with templates
- **[Protocol Validator](protocol-validator/)** - MCP compliance testing framework

### 🏢 **Production & Deployment**
- **[Production Deployment](production-deployment/)** - Docker, Kubernetes, monitoring
- **[Multi-Server Setup](multi-server/)** - Load balancing and failover patterns

### 🤖 **AI & LLM Integration**
- **[LLM Completion](llm-completion/)** - Multi-provider AI integration (OpenAI, Anthropic)
- **[Plugin Architecture](plugin-architecture/)** - Extensible provider system with dynamic loading

### 🧪 **Testing & Validation**
- **[Examples Tests](tests/)** - Comprehensive test suite for all examples

---

## 📋 **Examples by Use Case**

### Getting Started (New to MCP)
1. **[Simple Server](simple-server/)** - Your first MCP server
2. **[Simple Client](simple-client/)** - Connect to any MCP server
3. **[Batch Processing](batch-processing/)** - Scale up with batch operations

### Building Production Services
1. **[Authentication](authentication/)** - Secure your MCP services
2. **[Observability](observability/)** - Monitor and trace your services
3. **[Production Deployment](production-deployment/)** - Deploy to Docker/Kubernetes
4. **[Multi-Server Setup](multi-server/)** - Scale across multiple servers

### Advanced Development
1. **[Development Server](development-server/)** - Develop with hot reload
2. **[Code Generator](code-generator/)** - Generate provider boilerplate
3. **[Protocol Validator](protocol-validator/)** - Validate MCP compliance
4. **[Custom Transport](custom-transport/)** - Build custom transports

### AI & LLM Platforms
1. **[LLM Completion](llm-completion/)** - Integrate AI providers
2. **[Plugin Architecture](plugin-architecture/)** - Build extensible systems
3. **[Error Recovery](error-recovery/)** - Handle AI service failures

---

## 🚀 **Quick Start Recipes**

### Basic MCP Server in 5 Minutes
```bash
cd examples/simple-server
go run main.go
```

### Enterprise MCP Server with Authentication
```bash
cd examples/authentication
go run main.go
```

### Development with Hot Reload
```bash
cd examples/development-server
go run main.go
# Visit http://localhost:3000 for live dashboard
```

### Production Deployment
```bash
cd examples/production-deployment/docker-compose
docker-compose up -d
```

### MCP Compliance Testing
```bash
cd examples/protocol-validator
go run main.go
```

---

## 📊 **Example Complexity Levels**

### 🟢 **Beginner** (New to MCP or Go)
- Simple Server
- Simple Client
- Stdio Client
- Streamable HTTP Client/Server

### 🟡 **Intermediate** (Familiar with MCP basics)
- Batch Processing
- Pagination Example
- Client Callbacks
- Resource Subscriptions
- Authentication
- Observability

### 🔴 **Advanced** (Production deployments)
- Production Deployment
- Multi-Server Setup
- Custom Transport
- Error Recovery
- Plugin Architecture
- LLM Completion
- Metrics Reporting

### 🟠 **Expert** (SDK development and tooling)
- Development Server
- Code Generator
- Protocol Validator

---

## 🔧 **Running Examples**

### Prerequisites
```bash
# Install Go 1.21+
go version

# Clone the repository
git clone https://github.com/ajitpratap0/mcp-sdk-go.git
cd mcp-sdk-go

# Install development tools (optional but recommended)
make install-tools
```

### Running Individual Examples
```bash
# Navigate to any example directory
cd examples/simple-server

# Run the example
go run main.go

# Or build and run
go build -o server main.go
./server
```

### Running with Custom Configuration
Many examples support configuration via environment variables or config files:

```bash
# Example: Custom ports
MCP_PORT=8081 DASHBOARD_PORT=3001 go run main.go

# Example: With configuration file
CONFIG_FILE=./custom-config.json go run main.go
```

### Testing Examples
```bash
# Test all examples
go test ./examples/tests/...

# Test specific example
go test ./examples/tests/ -run TestSimpleServer
```

---

## 🏗️ **Architecture Patterns Demonstrated**

### Transport Layer Patterns
- **[Stdio Transport](stdio-client/)** - Standard MCP transport
- **[HTTP Transport](streamable-http-server/)** - Production HTTP+SSE
- **[Custom Transport](custom-transport/)** - WebSocket implementation
- **[Batch Processing](batch-processing/)** - High-throughput operations

### Middleware Patterns
- **[Authentication](authentication/)** - Security middleware
- **[Observability](observability/)** - Monitoring middleware
- **[Error Recovery](error-recovery/)** - Reliability middleware

### Provider Patterns
- **[Simple Server](simple-server/)** - Basic provider implementation
- **[Plugin Architecture](plugin-architecture/)** - Dynamic provider loading
- **[LLM Completion](llm-completion/)** - Multi-provider aggregation

### Configuration Patterns
- **[Development Server](development-server/)** - Hot reload configuration
- **[Production Deployment](production-deployment/)** - Production configuration
- **[Authentication](authentication/)** - Security configuration

### Client Patterns
- **[Simple Client](simple-client/)** - Basic client usage
- **[Client Callbacks](client-callbacks/)** - Server-initiated callbacks
- **[Multi-Server Setup](multi-server/)** - Load balancing clients

---

## 📚 **Learning Path**

### Week 1: MCP Fundamentals
1. Read the [MCP Specification](https://modelcontextprotocol.io/)
2. Run **[Simple Server](simple-server/)** and **[Simple Client](simple-client/)**
3. Try **[Stdio Client](stdio-client/)** for standard transport
4. Experiment with **[Batch Processing](batch-processing/)**

### Week 2: Production Features
1. Set up **[Authentication](authentication/)** with bearer tokens
2. Add **[Observability](observability/)** for monitoring
3. Try **[Error Recovery](error-recovery/)** patterns
4. Deploy with **[Production Deployment](production-deployment/)**

### Week 3: Advanced Development
1. Use **[Development Server](development-server/)** for rapid iteration
2. Generate providers with **[Code Generator](code-generator/)**
3. Validate compliance with **[Protocol Validator](protocol-validator/)**
4. Build a **[Custom Transport](custom-transport/)**

### Week 4: Specialized Use Cases
1. Integrate AI with **[LLM Completion](llm-completion/)**
2. Build extensible systems with **[Plugin Architecture](plugin-architecture/)**
3. Scale with **[Multi-Server Setup](multi-server/)**
4. Monitor everything with **[Metrics Reporting](metrics-reporting/)**

---

## 🤝 **Contributing Examples**

We welcome new examples! To contribute:

1. **Create Example Directory**: `examples/your-example/`
2. **Add README.md**: Document the example thoroughly
3. **Include main.go**: Complete, runnable implementation
4. **Add Tests**: Create tests in `examples/tests/`
5. **Update This Index**: Add your example to this README

### Example Template
```
examples/your-example/
├── README.md           # Comprehensive documentation
├── main.go            # Complete implementation
├── config.json        # Configuration examples (if needed)
├── Dockerfile         # Container support (if applicable)
└── docker-compose.yml # Deployment example (if applicable)
```

---

## 🔗 **External Resources**

- **[MCP Specification](https://modelcontextprotocol.io/)** - Official protocol specification
- **[Go Documentation](https://pkg.go.dev/github.com/ajitpratap0/mcp-sdk-go)** - API reference
- **[Contributing Guide](../CONTRIBUTING.md)** - How to contribute
- **[Development Guide](../CLAUDE.md)** - Development best practices
- **[Architecture Guide](../PLAN.md)** - Project roadmap and architecture

---

**Need Help?** Check the individual example READMEs for detailed instructions, or refer to the main project documentation for comprehensive guidance.

*The examples in this directory represent the most comprehensive collection of MCP implementation patterns available for Go developers.*
