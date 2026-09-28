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

# Run tests (with race detector; internal/tui split across processes)
test:
    @echo "Running tests..."
    ./scripts/test_race.sh

# Run tests outside internal/tui without the race detector (fast inner loop)
test-fast:
    go test $(go list ./... | grep -v /internal/tui)

# Run one package's tests with the race detector, e.g. `just test-pkg ./cmd`
test-pkg PKG:
    GORACE="atexit_sleep_ms=0" go test -race {{ PKG }}

# Run tests with coverage report and enforce the coverage thresholds
coverage:
    @echo "Running tests with coverage..."
    ./scripts/test_race.sh coverage.out
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"
    ./scripts/check_coverage.sh

# Format Go code (goimports when installed, else go fmt)
fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Formatting code..."
    if command -v goimports &>/dev/null; then
        goimports -w -local github.com/sglavoie/tt .
    else
        echo "goimports not found; falling back to go fmt"
        go fmt ./...
    fi

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

# Run the two-machine hazard gate (outside `just ci`; see TT_OLD_BINARY_REF)
cross-machine:
    ./scripts/cross_machine_gate.sh

# Build and vendor the pinned GhosttyKit.xcframework for the macOS app (outside `just ci`)
mac-ghosttykit:
    ./macos/scripts/build-ghosttykit.sh

# Run read-only code quality checks (gofmt, goimports, vet, golangci-lint, deadcode)
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
    (cd dist && shasum -a 256 tt-* > SHA256SUMS)
    echo "Release binaries and SHA256SUMS written to dist/"

# Remove build artifacts
clean:
    @echo "Cleaning..."
    rm -f {{ binary_name }} coverage.out coverage.html
    rm -rf dist
    go clean
    @echo "Clean complete"

# Install git hooks (points core.hooksPath at scripts/hooks)
install-hooks:
    git config core.hooksPath scripts/hooks
    @echo "Git hooks installed from scripts/hooks"
