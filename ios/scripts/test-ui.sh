#!/usr/bin/env sh
#
# Run the DocketMobile browsing UI tests on an iOS simulator against the real
# gomobile-bound engine. Prepares, under OUTPUT_DIR:
#   exchange/   an exchange folder a desktop `docket sync enroll` of the
#               representative fixture published (the join test's source)
#   conflict/   a second enrollment whose two Mac devices (studio/ and
#               laptop/, each with its own HOME) changed one task title and
#               one note line while apart, so conflict/exchange/ holds a
#               coupled conflict across two files for the conflict tests
#   work/       the host directory each test makes its disposable root under
# then generates the project and runs `xcodebuild test` for DocketMobileUITests.
# The result bundle is OUTPUT_DIR/DocketMobileUITests.xcresult and the log
# OUTPUT_DIR/xcodebuild-ui-test.log. Pass or fail, the bundle's structured
# results are written to OUTPUT_DIR/summary.json (counts, devices, and each
# failure) and OUTPUT_DIR/tests.json (every test case and its result), and its
# screenshots and other attachments to OUTPUT_DIR/attachments with their
# manifest.json.
#
#   ios/scripts/test-ui.sh OUTPUT_DIR
#
# Environment:
#   DOCKET_IOS_DEVICE           default or small, the simulator simulator.sh picks
#   DOCKET_IOS_SIMULATOR        simulator UDID for default (default: the first
#                           available iPhone on an iOS 18 or later runtime)
#   DOCKET_UITEST_MAX_WIDTH     the widest window the small-screen test accepts
#   DOCKET_IOS_CACHE            cache root (default ios/.cache, gitignored)
#   DOCKET_UITEST_ONLY          UI test classes or methods, comma-separated
#                           (Class or Class/testMethod; default: every class)
#   DOCKET_UITEST_WORKERS       simulator clones the classes are spread across
#                           (default 6; 1 runs serially on the simulator itself)
#   DOCKET_UITEST_TARGET        UI test bundle (default: DocketMobileUITests)

set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 OUTPUT_DIR" >&2
	exit 2
fi

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
ios_dir=$(dirname -- "$script_dir")
repo_root=$(dirname -- "$ios_dir")
cache_root="${DOCKET_IOS_CACHE:-$ios_dir/.cache}"

mkdir -p "$1"
output=$(CDPATH="" cd -- "$1" && pwd)
fixture="$repo_root/internal/compat/testdata/workspaces/representative"

"$script_dir/generate-project.sh"

simulator=$("$script_dir/simulator.sh" "${DOCKET_IOS_DEVICE:-default}")
# The tests read the system's appearance and text size unless they set their
# own, and `just ios-run` changes both, so every run starts from the defaults.
xcrun simctl boot "$simulator" >/dev/null 2>&1 || true
xcrun simctl bootstatus "$simulator" -b >/dev/null
xcrun simctl ui "$simulator" appearance light
xcrun simctl ui "$simulator" content_size large

# The desktop half of the join test: enroll a disposable copy of the fixture
# with the Docket this tree builds, under a private HOME.
rm -rf "$output/exchange" "$output/enroll" "$output/work" "$output/DocketMobileUITests.xcresult" \
	"$output/attachments" "$output/summary.json" "$output/tests.json"
mkdir -p "$output/exchange" "$output/enroll" "$output/work"
(cd "$repo_root" && go build -o "$output/enroll/docket" .)
cp -R "$fixture" "$output/enroll/original"
HOME="$output/enroll/home" "$output/enroll/docket" --data-dir "$output/enroll/original" sync enroll \
	--exchange "$output/exchange" --working-copy "$output/enroll/desk" --force >"$output/enroll/enroll.log" 2>&1 || {
	cat "$output/enroll/enroll.log" >&2
	echo "enrolling the fixture failed" >&2
	exit 1
}
[ -s "$output/exchange/tt-sync/v1/workspace.json" ] || {
	echo "the enrollment published no exchange identity" >&2
	exit 1
}

# The desktop half of the conflict tests: two Mac devices of one enrolled
# workspace edit the same task title and the same note line outside Docket while
# apart, then exchange, so both hold the same two coupled conflicts.
conflict="$output/conflict"
rm -rf "$conflict"
mkdir -p "$conflict/exchange"
cp -R "$fixture" "$conflict/original"
docket_as() {
	who=$1
	shift
	HOME="$conflict/$who-home" "$output/enroll/docket" "$@" >>"$conflict/$who.log" 2>&1 || {
		cat "$conflict/$who.log" >&2
		echo "the $who device failed: docket $*" >&2
		exit 1
	}
}
edit_apart() {
	who=$1
	tasks="$conflict/$who/projects/atlas-launch/tasks.md"
	note="$conflict/$who/projects/atlas-launch/notes/2026-09-03-090000_0a1b2c3d-release-runbook.md"
	sed "s/^## Task: Ship beta build\$/## Task: Ship beta build ($who)/" "$tasks" >"$tasks.edit" && mv "$tasks.edit" "$tasks"
	sed "s/^3\. Upload\$/3. Upload from the $who/" "$note" >"$note.edit" && mv "$note.edit" "$note"
	grep -q "Ship beta build ($who)" "$tasks" && grep -q "Upload from the $who" "$note" || {
		echo "the $who edit did not apply" >&2
		exit 1
	}
}
docket_as studio --data-dir "$conflict/original" sync enroll --exchange "$conflict/exchange" --working-copy "$conflict/studio" --force
docket_as laptop sync join --exchange "$conflict/exchange" --working-copy "$conflict/laptop"
edit_apart studio
docket_as studio --data-dir "$conflict/studio" sync exchange
edit_apart laptop
docket_as laptop --data-dir "$conflict/laptop" sync exchange
docket_as studio --data-dir "$conflict/studio" sync exchange
HOME="$conflict/studio-home" "$output/enroll/docket" --data-dir "$conflict/studio" sync conflicts --json >"$conflict/conflicts.json"
[ "$(grep -c '"replaceable":' "$conflict/conflicts.json")" -eq 2 ] || {
	cat "$conflict/conflicts.json" >&2
	echo "the two Macs did not leave the coupled conflict" >&2
	exit 1
}

# Each test copies what it changes into its own root under work/, so classes
# can run side by side on clones of the simulator; a class stays on one clone.
target="${DOCKET_UITEST_TARGET:-DocketMobileUITests}"
set --
if [ -n "${DOCKET_UITEST_ONLY:-}" ]; then
	old_ifs=$IFS
	IFS=,
	for only in $DOCKET_UITEST_ONLY; do
		set -- "$@" "-only-testing:$target/$only"
	done
	IFS=$old_ifs
else
	set -- "-only-testing:$target"
fi
workers="${DOCKET_UITEST_WORKERS:-6}"
if [ "$workers" -gt 1 ]; then
	set -- "$@" -parallel-testing-enabled YES -parallel-testing-worker-count "$workers"
else
	set -- "$@" -parallel-testing-enabled NO
fi
# A UI test can wedge inside XCTest itself (a typeText that never returns), and
# without a cap it holds its worker, and the whole run, indefinitely. The
# slowest test takes about three minutes under parallel load, so six fails a
# wedged one quickly and leaves every honest one room.
allowance="${DOCKET_UITEST_ALLOWANCE:-360}"
set -- "$@" -test-timeouts-enabled YES \
	-default-test-execution-time-allowance "$allowance" \
	-maximum-test-execution-time-allowance "$allowance"

echo "Testing DocketMobileUITests on simulator $simulator ($workers worker(s))..." >&2
status=0
TEST_RUNNER_DOCKET_UITEST_FIXTURE="$fixture" \
	TEST_RUNNER_DOCKET_UITEST_CONFLICT_EXCHANGE="$conflict/exchange" \
	TEST_RUNNER_DOCKET_UITEST_EXCHANGE="$output/exchange" \
	TEST_RUNNER_DOCKET_UITEST_WORK_DIR="$output/work" \
	TEST_RUNNER_DOCKET_UITEST_MAX_WIDTH="${DOCKET_UITEST_MAX_WIDTH:-}" \
	xcodebuild test \
	-project "$ios_dir/DocketMobile.xcodeproj" \
	-scheme DocketMobile \
	"$@" \
	-destination "platform=iOS Simulator,id=$simulator" \
	-derivedDataPath "$cache_root/UIDerivedData" \
	-resultBundlePath "$output/DocketMobileUITests.xcresult" \
	CODE_SIGNING_ALLOWED=NO \
	>"$output/xcodebuild-ui-test.log" 2>&1 || status=$?
if [ -d "$output/DocketMobileUITests.xcresult" ]; then
	xcrun xcresulttool get test-results summary --path "$output/DocketMobileUITests.xcresult" >"$output/summary.json" 2>/dev/null || true
	xcrun xcresulttool get test-results tests --path "$output/DocketMobileUITests.xcresult" >"$output/tests.json" 2>/dev/null || true
	xcrun xcresulttool export attachments --path "$output/DocketMobileUITests.xcresult" \
		--output-path "$output/attachments" >/dev/null 2>&1 || true
fi
if [ "$status" -ne 0 ]; then
	tail -60 "$output/xcodebuild-ui-test.log" >&2
	echo "xcodebuild test failed; full log at $output/xcodebuild-ui-test.log, results in $output/summary.json" >&2
	exit 1
fi
echo "DocketMobileUITests passed; result bundle at $output/DocketMobileUITests.xcresult" >&2
