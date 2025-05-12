# Local Commit Checks

This document describes the local commit checks that can be run before pushing changes to the repository. These checks mirror the GitHub workflows that run in CI.

## Pre-commit Hook

A pre-commit hook has been set up to run the same checks as the GitHub workflows. This will prevent commits that would fail the CI checks.

The pre-commit hook will run:
1. `go mod tidy` check
2. Code formatting verification with `gofmt`
3. Linting with `golangci-lint`
4. Security scanning with `gosec`
5. Vulnerability checking with `govulncheck`
6. Running tests with race detection
7. Verifying that the code builds

### Installing Required Tools

Some of the checks require external tools. You can install them with:

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Install govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## Makefile Commands

A Makefile has been added to run the checks individually:

```bash
# Run all checks
make check

# Run specific checks
make lint
make test
make build
make format
make tidy
make security
make vulncheck

# Show available commands
make help
```

## Bypassing Pre-commit Hooks

In case you need to commit changes without running the pre-commit hook (not recommended), you can use:

```bash
git commit --no-verify -m "Your commit message"
```
