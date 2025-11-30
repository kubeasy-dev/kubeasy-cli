# Makefile for kubeasy-cli
.PHONY: help build test lint clean install-tools deps vendor dev release-check build-all

# Setup Go PATH
export GOPATH ?= $(shell go env GOPATH)
export PATH := $(PATH):$(GOPATH)/bin

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
# Extract Kubernetes version from go.mod (e.g., v0.34.2 -> 1.34.2)
K8S_GO_VERSION=$(shell grep 'k8s.io/client-go' go.mod | grep -v '//' | awk '{print $$2}' | sed 's/v0\.\([0-9]*\)\.\([0-9]*\)/1.\1.\2/')
KUBERNETES_VERSION=$(or $(K8S_GO_VERSION),1.34.2)

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
	@command -v gocovmerge >/dev/null 2>&1 || \
		(echo "Installing gocovmerge..." && go install github.com/wadey/gocovmerge@latest)
	@echo "$(GREEN)✓ Tools installed$(NC)"

setup-envtest: install-tools ## Setup envtest Kubernetes assets
	@echo "$(YELLOW)Setting up envtest assets...$(NC)"
	@$(GOPATH)/bin/setup-envtest use $(KUBERNETES_VERSION) --bin-dir ./bin/k8s
	@echo "$(GREEN)✓ Envtest assets ready$(NC)"

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
		-coverpkg=./pkg/... $$(go list ./... | grep -v /test/integration)
	@if [ -s coverage-unit.out ]; then \
		go tool cover -func=coverage-unit.out | tail -1; \
	else \
		echo "$(YELLOW)ℹ  No unit test coverage data$(NC)"; \
	fi
	@echo "$(GREEN)✓ Unit tests passed$(NC)"

test-integration: setup-envtest ## Run integration tests
	@echo "$(YELLOW)Running integration tests...$(NC)"
	@echo "$(BLUE)ℹ  This will start a local Kubernetes API server$(NC)"
	@KUBEBUILDER_ASSETS=$$($(GOPATH)/bin/setup-envtest use -p path $(KUBERNETES_VERSION)) \
		go test -v -tags=integration -coverprofile=coverage-integration.out \
		-covermode=atomic -coverpkg=./pkg/... ./test/integration/... -timeout 15m
	@if [ -s coverage-integration.out ]; then \
		echo "$(GREEN)✓ Integration tests passed$(NC)"; \
		go tool cover -func=coverage-integration.out | tail -1; \
	else \
		echo "$(GREEN)✓ Integration tests passed$(NC)"; \
		echo "$(YELLOW)ℹ  No coverage data generated (normal for external-only tests)$(NC)"; \
	fi

test-all: test-unit test-integration ## Run all tests (unit + integration)
	@echo "$(GREEN)✓ All tests passed$(NC)"

test-verbose: setup-envtest ## Run integration tests with verbose output
	@echo "$(YELLOW)Running integration tests (verbose)...$(NC)"
	@KUBEBUILDER_ASSETS=$$(setup-envtest use -p path $(KUBERNETES_VERSION)) \
		go test -v -tags=integration ./test/integration/... -timeout 15m -v

test-coverage: test-all ## Generate combined coverage report
	@echo "$(YELLOW)Generating combined coverage report...$(NC)"
	@mkdir -p coverage
	@if [ -s coverage-unit.out ] && [ -s coverage-integration.out ]; then \
		echo "$(BLUE)ℹ  Merging unit and integration coverage...$(NC)"; \
		$(GOPATH)/bin/gocovmerge coverage-unit.out coverage-integration.out > coverage.out; \
	elif [ -s coverage-integration.out ]; then \
		echo "$(BLUE)ℹ  Using integration coverage only$(NC)"; \
		cp coverage-integration.out coverage.out; \
	elif [ -s coverage-unit.out ]; then \
		echo "$(BLUE)ℹ  Using unit coverage only$(NC)"; \
		cp coverage-unit.out coverage.out; \
	else \
		echo "$(RED)✗ No coverage data found$(NC)"; \
		exit 1; \
	fi
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

ci-test: setup-envtest ## Run tests in CI environment
	@echo "$(YELLOW)Running CI tests...$(NC)"
	@# Unit tests
	@go test -v -race -coverprofile=coverage-unit.out -covermode=atomic \
		$$(go list ./... | grep -v /test/integration)
	@# Integration tests
	@KUBEBUILDER_ASSETS=$$(setup-envtest use -p path $(KUBERNETES_VERSION) --bin-dir ./bin/k8s) \
		go test -v -tags=integration -coverprofile=coverage-integration.out \
		-covermode=atomic ./test/integration/... -timeout 15m
	@# Combine coverage using gocovmerge for reliable merging
	@mkdir -p coverage
	@if [ -s coverage-unit.out ] && [ -s coverage-integration.out ]; then \
		echo "$(BLUE)ℹ  Merging unit and integration coverage...$(NC)"; \
		$(GOPATH)/bin/gocovmerge coverage-unit.out coverage-integration.out > coverage.out; \
	elif [ -s coverage-integration.out ]; then \
		echo "$(BLUE)ℹ  Using integration coverage only...$(NC)"; \
		cp coverage-integration.out coverage.out; \
	elif [ -s coverage-unit.out ]; then \
		echo "$(BLUE)ℹ  Using unit coverage only...$(NC)"; \
		cp coverage-unit.out coverage.out; \
	fi
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

.DEFAULT_GOAL := help
