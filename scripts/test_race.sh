#!/usr/bin/env sh
# Run every package's tests under -race: the long poles through
# test_sharded.sh, and every other package in one `go test` alongside it.
#
# usage: test_race.sh [COVERPROFILE]
#
# With COVERPROFILE, both halves collect coverage and it is merged there.
#
# DOCKET_SHARD_INDEX and DOCKET_SHARD_COUNT (default 0 of 1) run one CI runner's slice:
# test_sharded.sh takes its slice of the long poles' chunks, and this shard
# takes every DOCKET_SHARD_COUNT-th of the other packages. Every shard's profile
# merges into the whole suite's with merge_coverage.sh.

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

# Every worker here competes for the same cores and memory, which skews the
# guards that compare timings; ci.sh checks runs those alone instead.
DOCKET_DEFER_TIMING_GUARDS=1
export DOCKET_DEFER_TIMING_GUARDS

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

# The packages whose serial runtime would otherwise set the suite's wall time:
# internal/tui and internal/config, and the sync convergence suites of
# internal/storage, internal/bridge, internal/syncmerge and internal/synclog,
# which run one to seven minutes each under -race as one process.
sharded_names="tui config storage bridge syncmerge synclog"
sharded=
pattern=
for name in $sharded_names; do
	sharded="$sharded github.com/sglavoie/docket/internal/$name"
	pattern="$pattern -e /internal/$name\$"
done
shard_index=${DOCKET_SHARD_INDEX:-0}
shard_count=${DOCKET_SHARD_COUNT:-1}
# shellcheck disable=SC2086
packages=$(go list ./... | grep -v $pattern |
	awk -v i="$shard_index" -v n="$shard_count" '(NR - 1) % n == i')

# A 20m deadline still ends a hang with its goroutine dump. For the sharded
# packages, each chunk carries its own deadline.
# A bare `go test` would test the current directory, so an empty slice runs
# nothing and leaves an empty profile.
if [ -z "$packages" ]; then
	: >"$work/rest.out"
	true &
elif [ -n "$coverprofile" ]; then
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
