.PHONY: \
	build \
	clean \
	coverage \
	fmt \
	help \
	install \
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

build: ## Build the binary in the current directory
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

clean: ## Remove build artifacts
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)
	go clean
	@echo "✅ Clean complete"
