# Contributing to Go Model Context Protocol SDK

Thank you for your interest in contributing to the Go Model Context Protocol SDK! This document provides guidelines and instructions for contributing to ensure the SDK maintains a high quality standard and remains compliant with the [Model Context Protocol specification](https://modelcontextprotocol.io/).

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** to your local machine
3. **Set up your development environment**:

   ```bash
   git clone https://github.com/your-username/mcp-sdk-go.git
   cd mcp-sdk-go
   go mod tidy
   ```

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

- Run linters to ensure code quality:

  ```bash
  golangci-lint run
  ```

### Commit Messages

Follow conventional commit messages format:

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for adding missing tests
- `chore:` for maintenance tasks

## Pull Request Process

1. **Create a branch** with a descriptive name
2. **Make your changes** and commit them
3. **Run tests** to ensure everything works
4. **Update documentation** if necessary
5. **Check specification compliance** to ensure your changes adhere to the MCP specification
6. **Submit a pull request** to the main repository
7. **Describe your changes** in the pull request with sufficient detail
8. **Respond to feedback** during the review process

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

By contributing, you agree that your contributions will be licensed under the project's [MIT License](LICENSE).
