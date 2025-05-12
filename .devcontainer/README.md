# Development Container for Model Context SDK

[![Go Version](https://img.shields.io/badge/Go-1.24-blue)](https://golang.org/)
[![Dev Container Spec](https://img.shields.io/badge/Dev%20Container-Enabled-blue)](https://containers.dev/)

This directory contains configuration for a development container that provides a consistent environment for working with the Model Context SDK. The Dev Container includes all necessary tools and dependencies required for development, testing, and running the local commit checks.

## Features

- **Go 1.24** - The latest stable version of Go
- **Pre-installed Go tools**:
  - `gopls` - Go language server for IDE features
  - `golangci-lint` - Linting and static analysis
  - `gosec` - Security scanning for Go code
  - `govulncheck` - Vulnerability scanning
  - `goimports` - Import organization
  - `delve` - Go debugger
  - `staticcheck` - Advanced static analysis
- **Git with pre-commit hooks** - Same checks as the GitHub workflows
- **Configured IDE settings** - Works with VS Code out of the box

## Using the Dev Container

### Visual Studio Code

1. Install the [Remote - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension
2. Open the project in VS Code
3. Click on the green icon in the bottom-left corner
4. Select "Reopen in Container"

VS Code will build the container and configure the development environment automatically.

### Other IDEs

For other IDEs that support the Dev Container specification or for command-line usage:

1. Build and start the container:

   ```bash
   docker-compose -f .devcontainer/docker-compose.yml up -d
   ```

2. Attach to the running container:

   ```bash
   docker exec -it $(docker ps -q --filter "name=model-context-sdk_app") bash
   ```

## Developing Transport Implementations

The Dev Container is especially useful for developing and testing transport implementations, such as:

- `HTTPTransport` in `pkg/transport/http.go`
- `StdioTransport` in `pkg/transport/stdio.go`
- Custom transport implementations

The container includes all the necessary tools to validate transport implementations against the `Transport` interface requirements, test JSON-RPC message handling, and verify security considerations.

## Pre-commit Hooks in the Container

The container comes with `pre-commit` installed, which works with the Git hooks in `.git/hooks/`. These checks ensure that any code you commit meets the project's quality standards.

## Building and Testing

Inside the container, you can use the commands from the Makefile:

```bash
# Run all checks
make check

# Run specific checks
make lint
make test
make build
make security
make vulncheck
```

These commands mirror the GitHub workflow checks but run locally in the containerized environment.
