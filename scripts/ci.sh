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

# This pass already replays the committed TUI scenarios and the focused
# migration/recovery tests; run_tui_scenarios.sh only re-selects them for
# standalone use. golangci-lint runs govet with enable-all, so `go vet` here
# would be a third pass over the same analyzers.
echo "Running race tests and collecting coverage..."
# The race-enabled TUI package can exceed Go's implicit 10-minute package
# deadline on shared CI runners even when every test passes. Keep the package
# deadline inside this script's outer CI budget instead of inheriting that
# environment-sensitive default.
go test -race -count=1 -timeout=20m -coverprofile=coverage.out ./...
./scripts/check_coverage.sh coverage.out

echo "Running golangci-lint..."
# golangci-lint otherwise takes a machine-wide lock at $TMPDIR/golangci-lint.lock
# and exits 3 with "parallel golangci-lint is running" when any other checkout on
# the same machine happens to be linting. That lock guards this run against
# nothing it owns, so an unrelated repository must not be able to fail this gate.
golangci-lint run --allow-parallel-runners

echo "Checking module metadata..."
go mod tidy -diff

echo "Checking dead code..."
./scripts/dead_code.sh

echo "Building all packages..."
go build -v ./...

echo "All CI checks passed."
