# Local Commit Checks

[![CI Status](https://github.com/ajitpratap0/mcp-sdk-go/workflows/CI/badge.svg)](https://github.com/ajitpratap0/mcp-sdk-go/actions?query=workflow%3ACI)
[![codecov](https://codecov.io/gh/ajitpratap0/mcp-sdk-go/branch/main/graph/badge.svg)](https://codecov.io/gh/ajitpratap0/mcp-sdk-go)

This document describes the local commit checks that you can run before pushing changes to the repository. These checks mirror the GitHub workflows that run in CI, ensuring your contributions will pass pipeline validation.

## Pre-commit Hook

A pre-commit hook has been set up to run the same checks as the GitHub workflows. This will prevent commits that would fail the CI checks.

The pre-commit hook will run:

1. `go mod tidy` check - Ensures module files are clean and up-to-date
2. Code formatting verification with `gofmt` - Enforces Go code style standards
3. Linting with `golangci-lint` - Identifies code quality issues
4. Security scanning with `gosec` - Finds potential security vulnerabilities
5. Vulnerability checking with `govulncheck` - Detects known vulnerabilities in dependencies
6. Running tests with race detection - Validates functionality and thread safety
7. Verifying that the code builds - Ensures code compiles correctly

## Installing Required Tools

You can install all development tools at once with:

```bash
make install-tools
```

This will install:

- pre-commit hook manager
- gopls (Go language server)
- golangci-lint
- gosec
- govulncheck
- goimports

And set up the pre-commit hooks in your local repository.

## Makefile Commands

This project includes a comprehensive Makefile to run checks individually or together:

```bash
# Run all standard checks (same as GitHub workflow)
make check

# Run pre-commit checks on all files
make precommit

# Run specific checks
make lint            # Run golangci-lint
make test            # Run tests with race detection
make build           # Verify build
make format          # Check code formatting
make tidy            # Check if go.mod and go.sum are tidy
make security        # Run gosec security scan
make vulncheck       # Run vulnerability check

# Run transport-specific checks
make transport-check # Test transport implementations specifically

# Show available commands
make help
```

### Transport Implementation Checks

The `transport-check` target runs specialized tests for the transport implementations, which is particularly important for verifying:

- StdioTransport with BaseTransport embedding
- HTTPTransport functionality including error handling and security aspects

These tests ensure that all transport mechanisms properly implement the Transport interface and handle edge cases correctly.

## CI Pipeline

The GitHub Actions CI workflow runs the following jobs:

1. **Lint**: Uses golangci-lint to check code quality
2. **Test**: Runs tests with race detection and uploads coverage to Codecov
3. **Build**: Verifies the code builds successfully
4. **Verify-Formatting**: Ensures code is properly formatted with gofmt

The test job runs on multiple OS platforms (Linux, macOS, Windows) and Go versions to ensure broad compatibility.

## Bypassing Pre-commit Hooks

In case you need to commit changes without running the pre-commit hook (not recommended), you can use:

```bash
git commit --no-verify -m "Your commit message"
```

**Note**: Bypassing checks should only be done in exceptional circumstances as it may lead to failing CI builds.
