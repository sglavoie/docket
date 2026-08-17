# tt - CLI task manager

binary_name := "tt"
install_dir := env("HOME") / ".local/bin"

# Build information
version := env("VERSION", "dev")
git_commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_time := `date -u '+%Y-%m-%dT%H:%M:%SZ'`
ldflags := "-ldflags \"-X github.com/sglavoie/tt/cmd.Version=" + version + " -X github.com/sglavoie/tt/cmd.GitCommit=" + git_commit + " -X github.com/sglavoie/tt/cmd.BuildTime=" + build_time + "\""

# Show available recipes
help:
    @just --list

# Build the binary in the current directory
build:
    @echo "Building {{ binary_name }}..."
    go build {{ ldflags }} -o {{ binary_name }} .
    @echo "Built {{ binary_name }}"

# Install the binary to ~/.local/bin
install: build
    @echo "Installing {{ binary_name }} to {{ install_dir }}..."
    @mkdir -p {{ install_dir }}
    rm -f {{ install_dir }}/{{ binary_name }}
    cp {{ binary_name }} {{ install_dir }}/{{ binary_name }}
    @echo "Installed to {{ install_dir }}/{{ binary_name }}"

# Alias for install
tt: install

# Run tests (with race detector)
test:
    @echo "Running tests..."
    go test -race ./...

# Removed: test-race alias (use `just test` instead — race detector is always enabled)

# Run tests with coverage report
coverage:
    @echo "Running tests with coverage..."
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# Format Go code
fmt:
    @echo "Formatting code..."
    go fmt ./...

# Clean up go.mod and go.sum
tidy:
    go mod tidy

# Run go vet
vet:
    go vet ./...

# Run go fix
fix:
    go fix ./...

# Run formatting and vetting
lint: fmt vet

# Run golangci-lint (requires golangci-lint installed)
lint-ci:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! command -v golangci-lint &>/dev/null; then
        echo "Error: golangci-lint not found. Install it from https://golangci-lint.run/welcome/install/"
        exit 1
    fi
    echo "Running golangci-lint..."
    golangci-lint run

# Run the exact platform-independent gate invoked by GitHub CI.
ci:
    ./scripts/ci.sh

# Replay every committed TUI scenario with one command.
tui-scenarios:
    ./scripts/run_tui_scenarios.sh

# Run code quality checks (deadcode, vet, fmt, goimports, staticcheck)
check:
    @echo "Running code quality checks..."
    ./scripts/check.sh

# Find dead code in the codebase
dead:
    @echo "Finding dead code..."
    ./scripts/dead_code.sh

# Build release binaries for darwin/linux × amd64/arm64
release:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p dist
    for os in darwin linux; do
        for arch in amd64 arm64; do
            out="dist/tt-${os}-${arch}"
            echo "Building ${out}..."
            GOOS=${os} GOARCH=${arch} go build {{ ldflags }} -o "${out}" .
        done
    done
    echo "Release binaries written to dist/"

# Remove build artifacts
clean:
    @echo "Cleaning..."
    rm -f {{ binary_name }}
    go clean
    @echo "Clean complete"

# Install git hooks for pre-push checks
install-hooks:
    @echo "Installing git hooks..."
    @chmod +x scripts/pre-push
    @ln -sf ../../scripts/pre-push .git/hooks/pre-push
    @echo "Pre-push hook installed"
