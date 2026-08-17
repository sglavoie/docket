#!/usr/bin/env sh

set -eu

require_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required CI tool not found: $1" >&2
		exit 1
	fi
}

require_tool golangci-lint
require_tool deadcode

echo "Checking documentation..."
./scripts/check_docs.sh

echo "Running race tests and collecting coverage..."
go test -race -count=1 -coverprofile=coverage.out ./...
./scripts/check_coverage.sh coverage.out

echo "Running committed TUI scenarios..."
./scripts/run_tui_scenarios.sh

echo "Running vet..."
go vet ./...

echo "Running golangci-lint..."
golangci-lint run

echo "Checking module metadata..."
go mod tidy -diff

echo "Checking dead code..."
./scripts/dead_code.sh

echo "Building all packages..."
go build -v ./...

echo "All CI checks passed."
