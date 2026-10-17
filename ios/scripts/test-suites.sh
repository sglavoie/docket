#!/usr/bin/env sh
#
# Run the iPhone app's Swift suites that are not UI tests, each on its own,
# and write one structured summary of what ran.
#
#   ios/scripts/test-suites.sh SUITE OUTPUT_DIR
#
# SUITE is one of:
#   unit     the SwiftPM packages' own tests: TTShared (the App Group capture
#            inbox, widget snapshot, links, and its contract with a real
#            bridgehost engine) and TTReminders (scheduling and actions over
#            a fake notification center), on this Mac
#   bridge   the TTEngine suite on an iOS simulator against the
#            gomobile-bound engine (test-engine.sh)
#   sync     the exchange transport scenarios in apple/TTSync: two or more
#            bridgehost engines exchanging over a fake iCloud provider and a
#            real coordinated folder, and joining from an enrolled exchange
#   app      hosted app scheduling and presentation tests on a simulator
#   all      unit, bridge, app, then sync
#
# Each suite writes OUTPUT_DIR/<suite>/ (its log, and for simulator suites the result
# bundle), and OUTPUT_DIR/summary.json lists every suite run with its status
# and its executed, failed, and skipped counts. A suite that executes no test
# or skips one fails, so an empty or skipped run is never reported as passing.
#
# Environment: TT_IOS_SIMULATOR and TT_IOS_CACHE as test-engine.sh reads them.

set -eu

if [ "$#" -ne 2 ]; then
	echo "usage: $0 unit|bridge|app|sync|all OUTPUT_DIR" >&2
	exit 2
fi

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
ios_dir=$(dirname -- "$script_dir")
repo_root=$(dirname -- "$ios_dir")
suite=$1
mkdir -p "$2"
output=$(CDPATH="" cd -- "$2" && pwd)
fixture="$repo_root/internal/compat/testdata/workspaces/representative"
records="$output/.records"
rm -f "$records" "$output/summary.json"

# record NAME STATUS EXECUTED FAILED SKIPPED LOG
record() {
	printf '{"suite": "%s", "status": "%s", "executed": %s, "failed": %s, "skipped": %s, "log": "%s"}\n' \
		"$1" "$2" "$3" "$4" "$5" "$6" >>"$records"
}

failures=0

# swift_package NAME PACKAGE_DIR: runs `swift test` and reads XCTest's counts.
swift_package() {
	name=$1 package=$2
	log="$output/$name.log"
	echo "Testing $name (swift test in ${package#"$repo_root"/})..." >&2
	status=passed
	(cd "$package" && swift test) >"$log" 2>&1 || status=failed
	# The last "Executed N tests, with F failures" line is the whole run's.
	counts=$(grep -Eo 'Executed [0-9]+ tests?, with [0-9]+ (tests? skipped and [0-9]+ )?failures?' "$log" | tail -1)
	executed=$(printf '%s' "$counts" | awk '{ print $2 + 0 }')
	failed=$(printf '%s' "$counts" | awk '{ print $(NF - 1) + 0 }')
	skipped=$(printf '%s' "$counts" | sed -n 's/.*with \([0-9]*\) tests\{0,1\} skipped.*/\1/p')
	skipped=${skipped:-0}
	[ "${executed:-0}" -gt 0 ] || status=failed
	[ "$skipped" -eq 0 ] || status=failed
	record "$name" "$status" "${executed:-0}" "${failed:-0}" "$skipped" "$log"
	if [ "$status" != passed ]; then
		grep -E 'error:|failed|skipped' "$log" | head -20 >&2 || tail -20 "$log" >&2
		echo "$name: failed (log at $log)" >&2
		failures=$((failures + 1))
	fi
}

bridgehost() {
	if [ ! -x "$output/bridgehost" ]; then
		(cd "$repo_root" && go build -trimpath -o "$output/bridgehost" ./mobile/bridgehost)
	fi
	echo "$output/bridgehost"
}

run_unit() {
	host=$(bridgehost)
	TT_BRIDGEHOST="$host" TT_CAPTURE_FIXTURE="$fixture" swift_package unit-shared "$ios_dir/TTShared"
	swift_package unit-reminders "$ios_dir/TTReminders"
}

run_simulator_suite() {
	name=$1 script=$2 bundle_name=$3
	log="$output/$name/test.log"
	mkdir -p "$output/$name"
	echo "Testing $name on a simulator..." >&2
	status=passed
	"$script_dir/$script" "$output/$name" >"$log" 2>&1 || status=failed
	bundle="$output/$name/$bundle_name.xcresult"
	executed=0 failed=0 skipped=0
	if [ -d "$bundle" ]; then
		xcrun xcresulttool get test-results summary --path "$bundle" >"$output/$name/summary.json" 2>/dev/null || true
		executed=$(plutil -extract totalTestCount raw -o - "$output/$name/summary.json" 2>/dev/null || echo 0)
		failed=$(plutil -extract failedTests raw -o - "$output/$name/summary.json" 2>/dev/null || echo 0)
		skipped=$(plutil -extract skippedTests raw -o - "$output/$name/summary.json" 2>/dev/null || echo 0)
	fi
	[ "$executed" -gt 0 ] || status=failed
	[ "$skipped" -eq 0 ] || status=failed
	[ "$failed" -eq 0 ] || status=failed
	record "$name" "$status" "$executed" "$failed" "$skipped" "$log"
	if [ "$status" != passed ]; then
		tail -20 "$log" >&2
		echo "$name: failed (log at $log)" >&2
		failures=$((failures + 1))
	fi
}

# The sync scenarios' join test reads an exchange a desktop enrollment of the
# fixture published, made here with the tt this tree builds.
run_sync() {
	host=$(bridgehost)
	enroll="$output/sync-enrollment"
	rm -rf "$enroll"
	mkdir -p "$enroll/exchange"
	(cd "$repo_root" && go build -o "$enroll/tt" .)
	cp -R "$fixture" "$enroll/original"
	HOME="$enroll/home" "$enroll/tt" --data-dir "$enroll/original" sync enroll \
		--exchange "$enroll/exchange" --working-copy "$enroll/desk" --force >"$enroll/enroll.log" 2>&1 || {
		cat "$enroll/enroll.log" >&2
		echo "enrolling the fixture failed" >&2
		exit 1
	}
	TT_BRIDGEHOST="$host" TT_ENROLLED_EXCHANGE="$enroll/exchange" swift_package sync "$repo_root/apple/TTSync"
}

case "$suite" in
unit) run_unit ;;
bridge) run_simulator_suite bridge test-engine.sh TTEngine ;;
app) run_simulator_suite app test-app.sh TTMobileTests ;;
sync) run_sync ;;
all)
	run_unit
	run_simulator_suite bridge test-engine.sh TTEngine
	run_simulator_suite app test-app.sh TTMobileTests
	run_sync
	;;
*)
	echo "unknown suite $suite (unit, bridge, app, sync, or all)" >&2
	exit 2
	;;
esac

{
	echo '{"suites": ['
	sed '$!s/$/,/' "$records"
	echo ']}'
} >"$output/summary.json"
rm -f "$records"
cat "$output/summary.json"
[ "$failures" -eq 0 ] || exit 1
