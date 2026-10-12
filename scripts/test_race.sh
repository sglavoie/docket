#!/usr/bin/env sh
# Run every package's tests under -race: the long poles through
# test_sharded.sh, and every other package in one `go test` alongside it.
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

# The race runtime sleeps atexit_sleep_ms (default 1000) before every test
# process exits, which costs each package, each shard, and each helper process a
# forked cross-machine test starts a full idle second. Races are still reported;
# only that grace period for late reports from exiting goroutines goes.
GORACE="${GORACE:+$GORACE }atexit_sleep_ms=0"
export GORACE

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

# The packages whose serial runtime would otherwise set the suite's wall time.
sharded="github.com/sglavoie/tt/internal/tui github.com/sglavoie/tt/internal/config"
packages=$(go list ./... | grep -v -e '/internal/tui$' -e '/internal/config$')

# With those split off, the long poles left here are internal/storage,
# internal/syncmerge and internal/bridge, whose sync convergence suites run seven
# to ten minutes each under -race. A 20m deadline still ends a hang with its
# goroutine dump without failing one of them for slowness. For the sharded
# packages, each chunk carries its own deadline.
if [ -n "$coverprofile" ]; then
	# shellcheck disable=SC2086
	go test -race -count=1 -timeout=20m -coverprofile="$work/rest.out" $packages &
else
	# shellcheck disable=SC2086
	go test -race -count=1 -timeout=20m $packages &
fi
rest=$!

sharded_status=0
if [ -n "$coverprofile" ]; then
	# shellcheck disable=SC2086
	./scripts/test_sharded.sh -coverprofile "$work/sharded.out" $sharded || sharded_status=$?
else
	# shellcheck disable=SC2086
	./scripts/test_sharded.sh $sharded || sharded_status=$?
fi
rest_status=0
wait "$rest" || rest_status=$?

if [ "$sharded_status" != 0 ] || [ "$rest_status" != 0 ]; then
	exit 1
fi

if [ -n "$coverprofile" ]; then
	./scripts/merge_coverage.sh "$coverprofile" "$work/rest.out" "$work/sharded.out"
fi
