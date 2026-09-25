#!/usr/bin/env sh
# Run internal/tui's tests under -race as several processes instead of one.
#
# That package is the suite's long pole and cannot run its tests in parallel in
# one process: every app init calls theme.ApplyTheme, which rewrites
# package-level styles. Separate processes share no globals, so this compiles
# the test binary once and deals its top-level tests round-robin across
# TT_TUI_SHARDS processes (default: a third of the CPUs, clamped to 2..8).
# TestExampleScenariosPass runs in every shard, and TT_TUI_SCENARIO_SHARD keeps
# each one to its own slice of the corpus.
#
# usage: test_tui_sharded.sh [COVERPROFILE]
#
# With COVERPROFILE, the binary is built with coverage and the shards' profiles
# are merged there.

set -eu

if [ "$#" -gt 1 ]; then
	echo "usage: $0 [COVERPROFILE]" >&2
	exit 2
fi
coverprofile=${1:-}

root=$(cd "$(dirname "$0")/.." && pwd)
package_dir=$root/internal/tui

shards=${TT_TUI_SHARDS:-}
if [ -z "$shards" ]; then
	cpus=$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)
	shards=$((cpus / 3))
	[ "$shards" -lt 2 ] && shards=2
	[ "$shards" -gt 8 ] && shards=8
fi
case $shards in
	'' | *[!0-9]* | 0)
		echo "TT_TUI_SHARDS must be a positive integer, got '$shards'" >&2
		exit 2
		;;
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

binary=$work/tui.test
if [ -n "$coverprofile" ]; then
	(cd "$root" && go test -race -cover -covermode=atomic -c -o "$binary" ./internal/tui)
else
	(cd "$root" && go test -race -c -o "$binary" ./internal/tui)
fi

scenarios=TestExampleScenariosPass
# A coverage-instrumented binary warns on every run without somewhere to write.
mkdir "$work/listcover"
(cd "$package_dir" && GOCOVERDIR=$work/listcover "$binary" -test.list '.*') |
	grep '^Test' | grep -vx "$scenarios" |
	awk -v n="$shards" -v dir="$work" '{ print > (dir "/tests." ((NR - 1) % n)) }'

started=$(date +%s)
i=0
while [ "$i" -lt "$shards" ]; do
	names=$scenarios
	if [ -f "$work/tests.$i" ]; then
		names="$names|$(paste -sd'|' "$work/tests.$i")"
	fi
	set -- -test.count=1 -test.timeout=10m -test.run "^($names)\$"
	if [ -n "$coverprofile" ]; then
		set -- "$@" -test.coverprofile="$work/cover.$i"
	fi
	(
		cd "$package_dir"
		status=0
		TT_TUI_SCENARIO_SHARD="$i/$shards" "$binary" "$@" >"$work/log.$i" 2>&1 || status=$?
		echo "$status" >"$work/status.$i"
	) &
	i=$((i + 1))
done
wait

failed=0
i=0
while [ "$i" -lt "$shards" ]; do
	status=$(cat "$work/status.$i" 2>/dev/null || echo missing)
	if [ "$status" != 0 ]; then
		failed=1
		echo "--- internal/tui shard $i/$shards failed (exit $status):" >&2
		cat "$work/log.$i" >&2
	fi
	i=$((i + 1))
done
if [ "$failed" != 0 ]; then
	exit 1
fi

if [ -n "$coverprofile" ]; then
	"$root/scripts/merge_coverage.sh" "$coverprofile" "$work"/cover.*
fi
echo "ok  	github.com/sglavoie/tt/internal/tui	$(($(date +%s) - started))s ($shards shards)"
