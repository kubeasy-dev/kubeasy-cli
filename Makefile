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
		(echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@command -v goreleaser >/dev/null 2>&1 || \
		(echo "Installing goreleaser..." && go install github.com/goreleaser/goreleaser@latest)
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

test: ## Run tests with coverage
	@echo "$(YELLOW)Running tests...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1
	@echo "$(GREEN)✓ Tests passed$(NC)"
	@echo "$(BLUE)ℹ  Coverage report: coverage.out$(NC)"

test-coverage: test ## Generate HTML coverage report
	@echo "$(YELLOW)Generating HTML coverage report...$(NC)"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report: coverage.html$(NC)"

lint: ## Run golangci-lint
	@echo "$(YELLOW)Running linters...$(NC)"
	@if [ ! -f .github/linters/.golangci.yml ]; then \
		echo "$(RED)✗ .github/linters/.golangci.yml not found$(NC)"; \
		exit 1; \
	fi
	@golangci-lint run --config .github/linters/.golangci.yml
	@echo "$(GREEN)✓ Linting passed$(NC)"

lint-fix: ## Run golangci-lint with auto-fix
	@echo "$(YELLOW)Running linters with auto-fix...$(NC)"
	@golangci-lint run --config .github/linters/.golangci.yml --fix
	@gofmt -w $$(find . -name "*.go" -not -path "./vendor/*")
	@echo "$(GREEN)✓ Linting fixed$(NC)"

fmt: ## Format Go code
	@echo "$(YELLOW)Formatting code...$(NC)"
	@gofmt -w $$(find . -name "*.go" -not -path "./vendor/*")
	@echo "$(GREEN)✓ Code formatted$(NC)"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -rf $(BUILD_DIR) $(DIST_DIR) vendor coverage.out coverage.html
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
