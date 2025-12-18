.PHONY: \
	build \
	clean \
	fmt \
	help \
	install \
	lint \
	test \
	tidy \
	vet

BINARY_NAME=tt
INSTALL_DIR=$(HOME)/.local/bin

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary in the current directory
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .
	@echo "✅ Built $(BINARY_NAME)"

install: build ## Install the binary to ~/.local/bin
	@echo "📦 Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✅ Installed to $(INSTALL_DIR)/$(BINARY_NAME)"

test: ## Run tests
	@echo "🧪 Running tests..."
	go test ./...

fmt: ## Format Go code
	@echo "🎨 Formatting code..."
	go fmt ./...

tidy: ## Clean up go.mod and go.sum
	go mod tidy

vet: ## Run go vet
	go vet ./...

lint: fmt vet ## Run formatting and vetting

clean: ## Remove build artifacts
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)
	go clean
	@echo "✅ Clean complete"
