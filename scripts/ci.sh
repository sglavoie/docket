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
# No per-package deadline. This sweep fans out over every package at once, so a
# package's wall time is its share of a contended machine rather than its work:
# `internal/tui` measures 257s run alone under -race with coverage, and 663s and
# 1206s inside this command on the same commit. Both trips this deadline has
# ever taken were that contention -- Go's implicit 10m, which is what motivated
# an explicit value, and then the 20m that replaced it -- and it has never once
# caught the hang it exists to catch. Raising it a third time picks another
# number off the same guess. The CI job and the harness that invoke this script
# bound the run instead, as they do for golangci-lint's own disabled deadline.
#
# The cost is the goroutine dump: a genuine deadlock now ends at that outer
# bound with no panic trace. Recover it by rerunning the suspect package alone
# -- `go test -race -timeout=10m ./internal/tui/` -- where 257s is the honest
# baseline and a deadline separates a hang from slowness because nothing else
# is competing for the machine.
go test -race -count=1 -timeout=0 -coverprofile=coverage.out ./...
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
