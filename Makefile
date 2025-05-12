.PHONY: all check lint test build format tidy security vulncheck help precommit install-tools transport-check

GO = go
GOFMT = gofmt
GOLINT = golangci-lint
GOSEC = gosec
GOVULNCHECK = govulncheck
PRECOMMIT = pre-commit

all: check

help:
	@echo "Available commands:"
	@echo "  make check          - Run all checks (same as GitHub workflow)"
	@echo "  make precommit      - Run pre-commit checks on all files"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make test           - Run tests with race detection"
	@echo "  make build          - Verify build"
	@echo "  make format         - Check code formatting"
	@echo "  make tidy           - Check if go.mod and go.sum are tidy"
	@echo "  make security       - Run gosec security scan"
	@echo "  make vulncheck      - Run vulnerability check"
	@echo "  make transport-check - Test transport implementations"
	@echo "  make install-tools  - Install all development tools"

# Main check target - runs basic checks that the GitHub workflow runs
check: tidy format lint security vulncheck test build
	@echo "All checks passed!"

# Run all pre-commit checks (recommended instead of individual checks)
precommit:
	@command -v $(PRECOMMIT) >/dev/null 2>&1 || { echo "Error: pre-commit not installed. Run 'make install-tools'"; exit 1; }
	$(PRECOMMIT) run --all-files

# Transport-specific checks - crucial for testing BaseTransport embedding
transport-check:
	@echo "Running transport implementation tests..."
	$(GO) test -v ./pkg/transport/...
	@echo "Testing StdioTransport with BaseTransport embedding..."
	$(GO) test -run TestStdioTransport ./pkg/transport/...
	@echo "Testing HTTPTransport with custom HTTP client..."
	$(GO) test -run TestHTTPTransport ./pkg/transport/...
	@echo "Transport implementation tests complete!"

# Install all development tools
install-tools:
	@echo "Installing development tools..."
	pip install pre-commit
	go install golang.org/x/tools/gopls@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install golang.org/x/tools/cmd/goimports@latest
	$(PRECOMMIT) install
	@echo "All development tools installed!"

lint:
	@command -v $(GOLINT) >/dev/null 2>&1 || { echo "Error: golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	$(GOLINT) run ./...

test:
	$(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

build:
	$(GO) build -v ./...

format:
	@unformatted=$$($(GOFMT) -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Error: The following files are not formatted properly:"; \
		echo "$$unformatted"; \
		echo "Run 'gofmt -w .' to fix the formatting."; \
		exit 1; \
	else \
		echo "Code formatting verified"; \
	fi

tidy:
	@cp go.mod go.mod.bak && cp go.sum go.sum.bak
	@$(GO) mod tidy
	@if ! diff -q go.mod go.mod.bak >/dev/null || ! diff -q go.sum go.sum.bak >/dev/null; then \
		echo "Error: go.mod or go.sum is not tidy. Please run 'go mod tidy' and commit the changes."; \
		mv go.mod.bak go.mod && mv go.sum.bak go.sum; \
		exit 1; \
	else \
		echo "go.mod and go.sum are tidy"; \
		rm go.mod.bak go.sum.bak; \
	fi

security:
	@command -v $(GOSEC) >/dev/null 2>&1 || { echo "Warning: gosec not installed. Run: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	$(GOSEC) ./...

vulncheck:
	@command -v $(GOVULNCHECK) >/dev/null 2>&1 || { echo "Warning: govulncheck not installed. Run: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; }
	$(GOVULNCHECK) ./...
