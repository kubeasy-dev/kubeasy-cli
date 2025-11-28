# Makefile for kubeasy-cli
.PHONY: help build test lint clean install-tools deps vendor dev release-check build-all

# Variables
BINARY_NAME=kubeasy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=bin
DIST_DIR=dist
LDFLAGS=-s -w \
	-X 'github.com/kubeasy-dev/kubeasy-cli/pkg/constants.Version=$(VERSION)' \
	-X 'github.com/kubeasy-dev/kubeasy-cli/pkg/constants.LogFilePath=/tmp/kubeasy-cli.log' \
	-X 'github.com/kubeasy-dev/kubeasy-cli/pkg/constants.WebsiteURL=https://kubeasy.dev' \
	-X 'github.com/kubeasy-dev/kubeasy-cli/pkg/constants.ExercicesRepoBranch=main'

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

help: ## Display this help message
	@echo "$(BLUE)Kubeasy CLI - Available commands:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

install-tools: ## Install development tools
	@echo "$(YELLOW)Installing development tools...$(NC)"
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest)
	@command -v goreleaser >/dev/null 2>&1 || \
		(echo "Installing goreleaser..." && go install github.com/goreleaser/goreleaser/v2@latest)
	@command -v setup-envtest >/dev/null 2>&1 || \
		(echo "Installing setup-envtest..." && go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)
	@setup-envtest use 1.30.x --bin-dir ./bin/k8s
	@echo "$(GREEN)✓ Tools installed$(NC)"

deps: ## Download and tidy Go dependencies
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

vendor: ## Generate vendor directory
	@echo "$(YELLOW)Generating vendor directory...$(NC)"
	@go mod vendor
	@echo "$(GREEN)✓ Vendor directory created$(NC)"

build: ## Build the binary for current platform
	@echo "$(YELLOW)Building $(BINARY_NAME) $(VERSION)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "$(GREEN)✓ Binary built: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

build-all: clean ## Build binaries for all platforms
	@echo "$(YELLOW)Building for all platforms...$(NC)"
	@mkdir -p $(DIST_DIR)
	@echo "  → Linux amd64..."
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "  → Linux arm64..."
	@GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "  → Darwin amd64..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "  → Darwin arm64..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "  → Windows amd64..."
	@GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "  → Windows arm64..."
	@GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-arm64.exe .
	@echo "$(GREEN)✓ All binaries built in $(DIST_DIR)/$(NC)"

test: test-all ## Run all tests (unit + integration)

test-unit: ## Run unit tests only
	@echo "$(YELLOW)Running unit tests...$(NC)"
	@go test -v -race -coverprofile=coverage-unit.out -covermode=atomic \
		$$(go list ./... | grep -v /test/integration)
	@go tool cover -func=coverage-unit.out | tail -1
	@echo "$(GREEN)✓ Unit tests passed$(NC)"

test-integration: setup-envtest ## Run integration tests
	@echo "$(YELLOW)Running integration tests...$(NC)"
	@echo "$(BLUE)ℹ  This will start a local Kubernetes API server$(NC)"
	@KUBEBUILDER_ASSETS=$$(setup-envtest use -p path 1.30.x) \
		go test -v -tags=integration -coverprofile=coverage-integration.out \
		-covermode=atomic ./test/integration/... -timeout 10m
	@echo "$(GREEN)✓ Integration tests passed$(NC)"

test-all: test-unit test-integration ## Run all tests (unit + integration)
	@echo "$(GREEN)✓ All tests passed$(NC)"

test-verbose: setup-envtest ## Run integration tests with verbose output
	@echo "$(YELLOW)Running integration tests (verbose)...$(NC)"
	@KUBEBUILDER_ASSETS=$$(setup-envtest use -p path 1.30.x) \
		go test -v -tags=integration ./test/integration/... -timeout 10m -v

test-coverage: test-all ## Generate combined coverage report
	@echo "$(YELLOW)Generating combined coverage report...$(NC)"
	@mkdir -p coverage
	@echo "mode: atomic" > coverage.out
	@test -f coverage-unit.out && tail -q -n +2 coverage-unit.out >> coverage.out || true
	@test -f coverage-integration.out && tail -q -n +2 coverage-integration.out >> coverage.out || true
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

setup-envtest: ## Install and setup controller-runtime envtest binaries
	@echo "$(YELLOW)Setting up envtest...$(NC)"
	@which setup-envtest > /dev/null || \
		(echo "Installing setup-envtest..." && \
		go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)
	@setup-envtest use 1.30.x --bin-dir ./bin/k8s
	@echo "$(GREEN)✓ EnvTest setup complete$(NC)"

ci-test: setup-envtest ## Run tests in CI environment
	@echo "$(YELLOW)Running CI tests...$(NC)"
	@# Unit tests
	@go test -v -race -coverprofile=coverage-unit.out -covermode=atomic \
		$$(go list ./... | grep -v /test/integration)
	@# Integration tests
	@KUBEBUILDER_ASSETS=$$(setup-envtest use -p path 1.30.x --bin-dir ./bin/k8s) \
		go test -v -tags=integration -coverprofile=coverage-integration.out \
		-covermode=atomic ./test/integration/... -timeout 15m
	@# Combine coverage
	@mkdir -p coverage
	@echo "mode: atomic" > coverage.out
	@tail -q -n +2 coverage-unit.out >> coverage.out || true
	@tail -q -n +2 coverage-integration.out >> coverage.out || true
	@echo "$(GREEN)✓ CI tests complete$(NC)"

lint: ## Run golangci-lint
	@echo "$(YELLOW)Running linters...$(NC)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(RED)✗ golangci-lint not installed$(NC)"; \
		echo "$(YELLOW)ℹ  Install it with: make install-tools$(NC)"; \
		exit 1; \
	fi
	@if [ ! -f .github/linters/.golangci.yml ]; then \
		echo "$(RED)✗ .github/linters/.golangci.yml not found$(NC)"; \
		exit 1; \
	fi
	@golangci-lint run --config .github/linters/.golangci.yml
	@echo "$(GREEN)✓ Linting passed$(NC)"

lint-fix: ## Run golangci-lint with auto-fix
	@echo "$(YELLOW)Running linters with auto-fix...$(NC)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(RED)✗ golangci-lint not installed$(NC)"; \
		echo "$(YELLOW)ℹ  Install it with: make install-tools$(NC)"; \
		exit 1; \
	fi
	@golangci-lint run --config .github/linters/.golangci.yml --fix
	@gofmt -w $$(find . -name "*.go" -not -path "./vendor/*")
	@echo "$(GREEN)✓ Linting fixed$(NC)"

fmt: ## Format Go code
	@echo "$(YELLOW)Formatting code...$(NC)"
	@gofmt -w $$(find . -name "*.go" -not -path "./vendor/*")
	@echo "$(GREEN)✓ Code formatted$(NC)"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -rf $(BUILD_DIR) $(DIST_DIR) vendor coverage*.out coverage.html coverage/
	@go clean -testcache
	@echo "$(GREEN)✓ Cleaned$(NC)"

dev: build ## Build and run in development mode
	@echo "$(BLUE)Running in development mode...$(NC)"
	@./$(BUILD_DIR)/$(BINARY_NAME)

release-check: ## Pre-release validation checks
	@echo "$(BLUE)========================================$(NC)"
	@echo "$(BLUE)  Pre-Release Validation$(NC)"
	@echo "$(BLUE)========================================$(NC)"
	@echo ""
	@echo "$(YELLOW)1. Checking git status...$(NC)"
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "$(RED)✗ Uncommitted changes detected$(NC)"; \
		git status --short; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Working directory clean$(NC)"
	@echo ""
	@echo "$(YELLOW)2. Checking current branch...$(NC)"
	@if [ "$$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then \
		echo "$(RED)✗ You must be on main branch$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ On main branch$(NC)"
	@echo ""
	@echo "$(YELLOW)3. Checking branch is up to date...$(NC)"
	@git fetch origin main --quiet
	@if [ $$(git rev-parse HEAD) != $$(git rev-parse @{u}) ]; then \
		echo "$(RED)✗ Branch is not up to date with origin/main$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Branch up to date$(NC)"
	@echo ""
	@echo "$(YELLOW)4. Running tests...$(NC)"
	@$(MAKE) test --no-print-directory
	@echo ""
	@echo "$(YELLOW)5. Running linters...$(NC)"
	@$(MAKE) lint --no-print-directory
	@echo ""
	@echo "$(YELLOW)6. Testing build...$(NC)"
	@$(MAKE) build --no-print-directory
	@echo ""
	@echo "$(GREEN)========================================$(NC)"
	@echo "$(GREEN)  ✓ All checks passed!$(NC)"
	@echo "$(GREEN)  Ready for release$(NC)"
	@echo "$(GREEN)========================================$(NC)"

release-local: clean ## Test release process locally (snapshot mode)
	@echo "$(YELLOW)Testing release locally...$(NC)"
	@goreleaser release --snapshot --clean --skip=publish
	@echo "$(GREEN)✓ Local release test completed$(NC)"
	@echo "$(BLUE)ℹ  Check dist/ directory for artifacts$(NC)"

.DEFAULT_GOAL := help
