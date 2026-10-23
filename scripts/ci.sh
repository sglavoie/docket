#!/usr/bin/env sh
# The repository gate. GitHub CI runs `ci.sh checks` once per platform, which
# is everything but the race tests, and runs test_race.sh split across runners
# through TT_SHARD_INDEX and TT_SHARD_COUNT, merging their coverage profiles
# before check_coverage.sh judges the whole.
#
# usage: ci.sh [checks]

set -eu

phase=${1:-all}
case $phase in
	all | checks) ;;
	*)
		echo "usage: $0 [checks]" >&2
		exit 2
		;;
esac

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

if [ "$phase" = all ]; then
	# This pass already replays the committed TUI scenarios and the focused
	# migration/recovery tests; run_tui_scenarios.sh only re-selects them for
	# standalone use. golangci-lint runs govet with enable-all, so `go vet` here
	# would be a third pass over the same analyzers.
	echo "Running race tests and collecting coverage..."
	./scripts/test_race.sh coverage.out
	./scripts/check_coverage.sh coverage.out
fi

echo "Checking search allocation budgets..."
./scripts/check_search_budget.sh

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
