# Contributing to Go Model Context Protocol SDK

[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://makeapullrequest.com)
[![Contributors](https://img.shields.io/github/contributors/ajitpratap0/mcp-sdk-go)](https://github.com/ajitpratap0/mcp-sdk-go/graphs/contributors)
[![Last Commit](https://img.shields.io/github/last-commit/ajitpratap0/mcp-sdk-go)](https://github.com/ajitpratap0/mcp-sdk-go/commits/main)

Thank you for your interest in contributing to the Go Model Context Protocol SDK! This document provides guidelines and instructions for contributing to ensure the SDK maintains a high quality standard and remains compliant with the [Model Context Protocol specification](https://modelcontextprotocol.io/).

## Ways to Contribute

There are many ways to contribute to this project, and all contributions are valued:

- **Code Contributions**: Add features, fix bugs, or improve performance
- **Documentation**: Improve or correct the documentation
- **Testing**: Add test cases or improve existing tests
- **Issue Triage**: Help process issues, reproduce bugs
- **Bug Reports**: Report bugs or unexpected behavior
- **Feature Requests**: Suggest new features or improvements
- **Community Support**: Help answer questions in discussions

### First-Time Contributors

New to the project or to open source? We're happy to help you get started! Look for issues labeled [`good first issue`](https://github.com/ajitpratap0/mcp-sdk-go/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) or [`help wanted`](https://github.com/ajitpratap0/mcp-sdk-go/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22).

For your first contribution, we recommend:

1. Start small: Fix a typo, improve documentation, or add a small test
2. Get familiar with the codebase by running the examples
3. Ask questions if you're unsure - we're here to help
4. Follow the PR process described below - it ensures quality and consistency

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** to your local machine
3. **Set up your development environment**:

   ```bash
   git clone https://github.com/your-username/mcp-sdk-go.git
   cd mcp-sdk-go
   go mod tidy
   ```

### Development Environment Setup

#### Development Options

This project provides multiple options for setting up your development environment:

1. **Dev Container**: For the fastest setup with zero local configuration, use the provided Development Container - a pre-configured environment with all necessary tools and dependencies. See [.devcontainer/README.md](.devcontainer/README.md) for details.

2. **Local Setup**: If you prefer local development, you'll need the following tools:

   - **Go**: Version 1.24.2 or later (required)
   - **golangci-lint**: For code quality checks and automated validation

#### IDE Configuration

This project includes version-controlled IDE configurations for Visual Studio Code and IntelliJ IDEA to provide a consistent development experience.

For detailed instructions on setting up your IDE with recommended extensions, configurations, and tools, please refer to [README.ide.md](README.ide.md).

#### Helpful Workflows

**Running tests**:

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/transport

# Run tests with coverage report
go test -cover ./...
```

**Viewing examples**:
Review the examples in the `examples/` directory to understand how the SDK is used.

## Development Guidelines

### Code Style

This project follows standard Go coding conventions:

- Use `gofmt` or `goimports` to format your code
- Follow [Effective Go](https://golang.org/doc/effective_go) practices
- Add comments to exported functions, types, and constants
- Keep functions short and focused
- Write descriptive variable and function names
- Structure packages according to domain responsibilities
- Ensure error handling is comprehensive and propagates useful information
- Use context.Context properly for cancellation and timeouts

### Testing

- All new features should include appropriate tests
- Maintain or improve test coverage with your changes
- Include both unit tests and integration tests where appropriate
- Test edge cases and error conditions
- Run tests before submitting a PR:

  ```bash
  go test ./...
  ```

### Code Quality and Checks

- Run code quality checks before submitting a PR
- The project includes comprehensive checks for security, linting, and formatting
- Use the provided Makefile commands for consistent validation

  ```bash
  # Run all checks at once
  make check

  # Or run individual checks
  make lint
  make security
  ```

- For transport implementations, ensure you run the transport-specific checks:

  ```bash
  make transport-check
  ```

- See [CHECKS.md](CHECKS.md) for detailed information on all available checks, CI integration, and required tools

### Commit Messages

Follow conventional commit messages format:

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for adding missing tests
- `chore:` for maintenance tasks

## Pull Request Process

### Preparation

1. **Create a branch** with a descriptive name related to your changes (e.g., `feature/add-custom-transport` or `fix/http-timeout-issue`)
2. **Make your changes** and commit them using the commit message format described above
3. **Run tests** to ensure everything works: `go test ./...`
4. **Run linting** to ensure code quality: `golangci-lint run`
5. **Update documentation** if necessary, including godoc comments and README.md
6. **Check specification compliance** to ensure your changes adhere to the MCP specification

### Submitting Your PR

1. **Pull and rebase** from the main branch to ensure your code is up-to-date
2. **Submit a pull request** to the main repository
3. **Fill out the PR template** with all relevant information
4. **Link related issues** in the PR description (e.g., "Fixes #123")
5. **Request reviews** from appropriate team members

### PR Description

Your PR description should include:

- **What** is being changed
- **Why** it's being changed (link to issues where possible)
- **How** it was implemented
- **Testing** that was performed
- **Breaking changes** if any
- **Screenshots** for UI changes (if applicable)

### Review Process

1. **Automated checks** must pass (tests, linting, CI builds)
2. **Code reviews** will be conducted by at least one maintainer
3. **Address feedback** promptly and make requested changes
4. **Reviewers will approve** once all requirements are met
5. **Maintainers will merge** the PR when ready

Pull requests are typically reviewed within 1-3 business days. Complex changes may take longer.

## Bug Reports

When reporting bugs, please include:

- A clear and descriptive title
- Steps to reproduce the issue
- Expected behavior
- Actual behavior
- Go version and environment details
- Any relevant logs or error messages
- MCP server being used, if applicable
- Transport mechanism involved (stdio, HTTP, etc.)

## Feature Requests

Feature requests are welcome! Please include:

- A clear and descriptive title
- Detailed description of the proposed feature
- Explanation of why this feature would be useful
- Examples of how the feature would be used
- References to the MCP specification if implementing a specified feature
- Performance implications, if any

### Specification Support

When implementing features from the MCP specification:

- Clearly indicate which part of the specification your contribution addresses
- Follow the naming conventions used in the specification
- Ensure proper validation and error handling as described in the spec
- Add examples demonstrating the feature

## Implementation Guidelines

### Transports

When implementing or modifying transport mechanisms:

- The stdio transport should be maintained as the primary recommended transport
- Ensure proper buffering and flushing of messages
- Implement proper error handling and recovery
- Document transport-specific behaviors and limitations

### Pagination

When working with paginated operations:

- Use the pagination utilities in the `pagination` package
- Apply consistent validation across all paginated operations
- Follow the cursor-based pagination model from the specification
- Provide options for automatic pagination for users when appropriate

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help maintain a welcoming community
- Respect the MCP specification and its goals
- Collaborate with other implementers

## License

By contributing, you agree that your contributions will be licensed under the project's [MIT License](LICENSE) and will become part of the work maintained by the Go Model Context Protocol SDK Contributors.
