.PHONY: \
	build \
	check \
	ci \
	clean \
	coverage \
	dead \
	docs \
	fmt \
	help \
	install \
	install-hooks \
	lint \
	lint-ci \
	test \
	test-race \
	tidy \
	tt \
	vet

BINARY_NAME=tt
INSTALL_DIR=$(HOME)/.local/bin

# Build information
VERSION ?= dev
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/sglavoie/tt/cmd.Version=$(VERSION) -X github.com/sglavoie/tt/cmd.GitCommit=$(GIT_COMMIT) -X github.com/sglavoie/tt/cmd.BuildTime=$(BUILD_TIME)"

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

docs: ## Generate CLI reference for embedding
	@echo "📝 Generating CLI reference..."
	go run ./scripts/gen-api-ref > cmd/cli-reference.md
	@echo "✅ Generated cmd/cli-reference.md"

build: docs ## Build the binary in the current directory
	@echo "🔨 Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "✅ Built $(BINARY_NAME)"

install: build ## Install the binary to ~/.local/bin
	@echo "📦 Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✅ Installed to $(INSTALL_DIR)/$(BINARY_NAME)"

tt: install

test: ## Run tests
	@echo "🧪 Running tests..."
	go test ./...

test-race: ## Run tests with race detector
	@echo "🧪 Running tests with race detector..."
	go test -race ./...

coverage: ## Run tests with coverage report
	@echo "📊 Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

fmt: ## Format Go code
	@echo "🎨 Formatting code..."
	go fmt ./...

tidy: ## Clean up go.mod and go.sum
	go mod tidy

vet: ## Run go vet
	go vet ./...

lint: fmt vet ## Run formatting and vetting

lint-ci: ## Run golangci-lint (requires golangci-lint installed)
	@echo "🔍 Running golangci-lint..."
	golangci-lint run

ci: ## Run all CI checks locally (build, test, coverage, lint)
	@echo "🚀 Running CI checks locally..."
	@echo "📦 Building..."
	go build -v ./...
	@echo "🧪 Running tests with race detector..."
	go test -race -coverprofile=coverage.out ./...
	@echo "📊 Checking coverage threshold..."
	@THRESHOLD=$$(cat .coverage-threshold); \
	COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${COVERAGE}%"; \
	if [ "$$(echo "$$COVERAGE < $$THRESHOLD" | bc)" -eq 1 ]; then \
		echo "❌ Coverage $${COVERAGE}% is below $${THRESHOLD}% threshold"; \
		exit 1; \
	fi; \
	echo "✅ Coverage $${COVERAGE}% meets $${THRESHOLD}% threshold"; \
	MARGIN=5; \
	if [ "$$(echo "$$COVERAGE >= $$THRESHOLD + $$MARGIN" | bc)" -eq 1 ]; then \
		echo "💡 Coverage $${COVERAGE}% exceeds threshold by $${MARGIN}%+. Consider increasing .coverage-threshold."; \
	fi
	@echo "🔍 Running golangci-lint..."
	golangci-lint run
	@echo "✅ All CI checks passed!"

check: ## Run code quality checks (deadcode, vet, fmt, goimports, staticcheck)
	@echo "🔍 Running code quality checks..."
	./scripts/check.sh

dead: ## Find dead code in the codebase
	@echo "💀 Finding dead code..."
	./scripts/dead_code.sh

clean: ## Remove build artifacts
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)
	go clean
	@echo "✅ Clean complete"

install-hooks: ## Install git hooks for pre-push checks
	@echo "🪝 Installing git hooks..."
	@chmod +x scripts/pre-push
	@ln -sf ../../scripts/pre-push .git/hooks/pre-push
	@echo "✅ Pre-push hook installed"
