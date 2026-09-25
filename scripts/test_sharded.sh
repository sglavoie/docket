#!/usr/bin/env sh
# Run the given packages' tests under -race as many small processes instead of
# one per package.
#
# These packages are the suite's long poles and cannot run their tests in
# parallel in one process: internal/tui's app init calls theme.ApplyTheme, which
# rewrites package-level styles, and internal/config's tests move $HOME, which
# is machine identity. Separate processes share neither, so this compiles each
# test binary once, deals its top-level tests round-robin into TT_SHARD_CHUNKS
# chunks per package (default 24), and runs every chunk of every package from
# one pool of TT_SHARD_JOBS processes (default: two thirds of the CPUs, at
# least 2). Many small chunks pulled from one queue keep every worker busy until
# the end, where a few fixed shards leave the run waiting on whichever drew the
# slowest tests. More jobs than that lose to kernel contention under -race. TestExampleScenariosPass runs in every chunk, and TT_TUI_SCENARIO_SHARD
# keeps each one to its own slice of the corpus.
#
# usage: test_sharded.sh [-coverprofile FILE] PACKAGE...
#
# With -coverprofile, the binaries are built with coverage and the chunks'
# profiles are merged there.

set -eu

# --run-chunk PACKAGE_INDEX CHUNK BINARY DIR is one queued chunk, re-entered by
# xargs below. It records its exit status beside its log and always succeeds, so
# the queue keeps draining and every failure is reported rather than the first.
if [ "${1:-}" = "--run-chunk" ]; then
	package=$2 chunk=$3 binary=$4 dir=$5
	work=$TT_SHARD_WORK
	names=TestExampleScenariosPass
	if [ -f "$work/tests.$package.$chunk" ]; then
		names="$names|$(paste -sd'|' "$work/tests.$package.$chunk")"
	fi
	set -- -test.count=1 -test.timeout=10m -test.run "^($names)\$"
	if [ -n "$TT_SHARD_COVER" ]; then
		set -- "$@" -test.coverprofile="$work/cover.$package.$chunk"
	fi
	status=0
	(cd "$dir" && TT_TUI_SCENARIO_SHARD="$chunk/$TT_SHARD_CHUNK_COUNT" "$binary" "$@") \
		>"$work/log.$package.$chunk" 2>&1 || status=$?
	echo "$status" >"$work/status.$package.$chunk"
	exit 0
fi

coverprofile=
if [ "${1:-}" = "-coverprofile" ]; then
	coverprofile=${2:?"-coverprofile needs a file"}
	shift 2
fi
if [ "$#" -lt 1 ]; then
	echo "usage: $0 [-coverprofile FILE] PACKAGE..." >&2
	exit 2
fi

# The race runtime sleeps atexit_sleep_ms (default 1000) before every test
# process exits, which costs each chunk and each helper process a forked
# cross-machine test starts a full idle second. Races are still reported; only
# that grace period for late reports from exiting goroutines goes.
GORACE="${GORACE:+$GORACE }atexit_sleep_ms=0"
export GORACE

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

positive() {
	case $2 in
		'' | *[!0-9]* | 0)
			echo "$1 must be a positive integer, got '$2'" >&2
			exit 2
			;;
	esac
}
chunks=${TT_SHARD_CHUNKS:-24}
positive TT_SHARD_CHUNKS "$chunks"
jobs=${TT_SHARD_JOBS:-}
if [ -z "$jobs" ]; then
	cpus=$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)
	jobs=$((cpus * 2 / 3))
	[ "$jobs" -lt 2 ] && jobs=2
fi
positive TT_SHARD_JOBS "$jobs"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

scenarios=TestExampleScenariosPass
# A coverage-instrumented binary warns on every run without somewhere to write.
mkdir "$work/listcover"

# One line per chunk: package index, chunk index, test binary, package
# directory. The chunk's test names are in $work/tests.<package>.<chunk>.
: >"$work/jobs"
p=0
for package in "$@"; do
	binary=$work/pkg$p.test
	if [ -n "$coverprofile" ]; then
		go test -race -cover -covermode=atomic -c -o "$binary" "$package"
	else
		go test -race -c -o "$binary" "$package"
	fi
	dir=$(go list -f '{{.Dir}}' "$package")
	(cd "$dir" && GOCOVERDIR=$work/listcover "$binary" -test.list '.*') |
		grep '^Test' | grep -vx "$scenarios" |
		awk -v n="$chunks" -v prefix="$work/tests.$p." '{ print > (prefix ((NR - 1) % n)) }'
	i=0
	while [ "$i" -lt "$chunks" ]; do
		echo "$p $i $binary $dir" >>"$work/jobs"
		i=$((i + 1))
	done
	p=$((p + 1))
done

started=$(date +%s)
export TT_SHARD_WORK="$work" TT_SHARD_COVER="$coverprofile" TT_SHARD_CHUNK_COUNT="$chunks"
xargs -P "$jobs" -L 1 "$root/scripts/test_sharded.sh" --run-chunk <"$work/jobs"

failed=0
p=0
for package in "$@"; do
	i=0
	while [ "$i" -lt "$chunks" ]; do
		status=$(cat "$work/status.$p.$i" 2>/dev/null || echo missing)
		if [ "$status" != 0 ]; then
			failed=1
			echo "--- $package chunk $i/$chunks failed (exit $status):" >&2
			cat "$work/log.$p.$i" >&2
		fi
		i=$((i + 1))
	done
	p=$((p + 1))
done
if [ "$failed" != 0 ]; then
	exit 1
fi

if [ -n "$coverprofile" ]; then
	"$root/scripts/merge_coverage.sh" "$coverprofile" "$work"/cover.*
fi
elapsed=$(($(date +%s) - started))
for package in "$@"; do
	echo "ok  	$package	${elapsed}s ($chunks chunks, $jobs jobs shared)"
done
