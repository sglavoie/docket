#!/usr/bin/env sh
# Run every package's tests under -race: internal/tui through
# test_tui_sharded.sh, and every other package in one `go test` alongside it.
#
# usage: test_race.sh [COVERPROFILE]
#
# With COVERPROFILE, both halves collect coverage and it is merged there.

set -eu

if [ "$#" -gt 1 ]; then
	echo "usage: $0 [COVERPROFILE]" >&2
	exit 2
fi
coverprofile=${1:-}

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

packages=$(go list ./... | grep -v '/internal/tui$')

# With internal/tui split off, no package here runs longer than a minute under
# -race, so a 10m deadline separates a hang from slowness again and a hang ends
# with its goroutine dump. For internal/tui, each shard carries its own.
if [ -n "$coverprofile" ]; then
	# shellcheck disable=SC2086
	go test -race -count=1 -timeout=10m -coverprofile="$work/rest.out" $packages &
else
	# shellcheck disable=SC2086
	go test -race -count=1 -timeout=10m $packages &
fi
rest=$!

tui_status=0
if [ -n "$coverprofile" ]; then
	./scripts/test_tui_sharded.sh "$work/tui.out" || tui_status=$?
else
	./scripts/test_tui_sharded.sh || tui_status=$?
fi
rest_status=0
wait "$rest" || rest_status=$?

if [ "$tui_status" != 0 ] || [ "$rest_status" != 0 ]; then
	exit 1
fi

if [ -n "$coverprofile" ]; then
	./scripts/merge_coverage.sh "$coverprofile" "$work/rest.out" "$work/tui.out"
fi
