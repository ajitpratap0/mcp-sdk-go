# IDE Configuration for Model Context SDK

This project includes version-controlled IDE configuration for Visual Studio Code and IntelliJ IDEA to provide a consistent development environment. These settings are designed to work with the commit checks and maintain code quality standards.

## Visual Studio Code Features

The `.vscode` directory contains configurations that provide:

- Go language server (gopls) setup
- Integration with golangci-lint
- Code formatting on save
- Debug configurations for tests and Go programs
- Import organization
- Tasks integration with the Makefile

### Recommended Extensions

For the best experience, install these VS Code extensions:

- Go - The Go extension by the Go Team at Google (`golang.go`)
- Go Test Explorer - For easy test running and visualization
- Error Lens - For better error visualization
- Task Explorer - For visualizing and running Makefile tasks

### Using Tasks

Access the configured tasks by:
1. Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on macOS)
2. Type "Tasks: Run Task"
3. Select from available tasks like "Check All", "Run Tests", "Lint Code", etc.

## IntelliJ IDEA Features

The `.idea` directory contains configurations that provide:

- Code style settings for Go
- Run configurations for tests
- Run configurations for code formatting and linting tools

### Required Plugin

Make sure to install the Go plugin for IntelliJ:
- Go to Preferences/Settings > Plugins > Marketplace
- Search for "Go" and install the plugin by JetBrains
- Restart IntelliJ IDEA

## Required Go Tools

These IDE configurations work best with the following Go tools installed:

```bash
# Install Go tools
go install golang.org/x/tools/gopls@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install golang.org/x/tools/cmd/goimports@latest
```

## Project-Specific Optimizations

The IDE configurations are optimized for the Model Context SDK codebase, with special attention to:

- The transport package (`pkg/transport/`) with its various implementations (HTTPTransport, StdioTransport)
- The client package (`pkg/client/`) and its configuration
- Protocol definitions and handlers
- Security-related code patterns

## Integration with Pre-commit Hooks

These IDE configurations work seamlessly with the pre-commit hooks set up in this project. They use the same tools and follow the same standards to ensure consistency between your IDE environment and the commit validation process.
