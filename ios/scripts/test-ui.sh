#!/usr/bin/env sh
#
# Run the TTMobile browsing UI tests on an iOS simulator against the real
# gomobile-bound engine. Prepares, under OUTPUT_DIR:
#   exchange/   an exchange folder a desktop `tt sync enroll` of the
#               representative fixture published (the join test's source)
#   conflict/   a second enrollment whose two Mac devices (studio/ and
#               laptop/, each with its own HOME) changed one task title and
#               one note line while apart, so conflict/exchange/ holds a
#               coupled conflict across two files for the conflict tests
#   work/       the host directory each test makes its disposable root under
# then generates the project and runs `xcodebuild test` for TTMobileUITests.
# The result bundle is OUTPUT_DIR/TTMobileUITests.xcresult and the log
# OUTPUT_DIR/xcodebuild-ui-test.log.
#
#   ios/scripts/test-ui.sh OUTPUT_DIR
#
# Environment:
#   TT_IOS_SIMULATOR        simulator UDID (default: the first available
#                           iPhone on an iOS 18 or later runtime)
#   TT_IOS_CACHE            cache root (default ios/.cache, gitignored)
#   TT_UITEST_ONLY          one UI test class to run (default: every class)

set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 OUTPUT_DIR" >&2
	exit 2
fi

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
ios_dir=$(dirname -- "$script_dir")
repo_root=$(dirname -- "$ios_dir")
cache_root="${TT_IOS_CACHE:-$ios_dir/.cache}"

mkdir -p "$1"
output=$(CDPATH="" cd -- "$1" && pwd)
fixture="$repo_root/internal/compat/testdata/workspaces/representative"

"$script_dir/generate-project.sh"

simulator=${TT_IOS_SIMULATOR:-$(xcrun simctl list devices available | awk '
	/^-- iOS [0-9]+/ { split($3, version, "."); eligible = (version[1] + 0 >= 18); next }
	/^-- / { eligible = 0; next }
	eligible && /iPhone/ && match($0, /\([0-9A-F-]{36}\)/) {
		print substr($0, RSTART + 1, RLENGTH - 2); exit
	}')}
if [ -z "$simulator" ]; then
	echo "no available iPhone simulator on iOS 18 or later; set TT_IOS_SIMULATOR" >&2
	exit 1
fi

# The desktop half of the join test: enroll a disposable copy of the fixture
# with the tt this tree builds, under a private HOME.
rm -rf "$output/exchange" "$output/enroll" "$output/work" "$output/TTMobileUITests.xcresult"
mkdir -p "$output/exchange" "$output/enroll" "$output/work"
(cd "$repo_root" && go build -o "$output/enroll/tt" .)
cp -R "$fixture" "$output/enroll/original"
HOME="$output/enroll/home" "$output/enroll/tt" --data-dir "$output/enroll/original" sync enroll \
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
# workspace edit the same task title and the same note line outside tt while
# apart, then exchange, so both hold the same two coupled conflicts.
conflict="$output/conflict"
rm -rf "$conflict"
mkdir -p "$conflict/exchange"
cp -R "$fixture" "$conflict/original"
tt_as() {
	who=$1
	shift
	HOME="$conflict/$who-home" "$output/enroll/tt" "$@" >>"$conflict/$who.log" 2>&1 || {
		cat "$conflict/$who.log" >&2
		echo "the $who device failed: tt $*" >&2
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
tt_as studio --data-dir "$conflict/original" sync enroll --exchange "$conflict/exchange" --working-copy "$conflict/studio" --force
tt_as laptop sync join --exchange "$conflict/exchange" --working-copy "$conflict/laptop"
edit_apart studio
tt_as studio --data-dir "$conflict/studio" sync exchange
edit_apart laptop
tt_as laptop --data-dir "$conflict/laptop" sync exchange
tt_as studio --data-dir "$conflict/studio" sync exchange
HOME="$conflict/studio-home" "$output/enroll/tt" --data-dir "$conflict/studio" sync conflicts --json >"$conflict/conflicts.json"
[ "$(grep -c '"replaceable":' "$conflict/conflicts.json")" -eq 2 ] || {
	cat "$conflict/conflicts.json" >&2
	echo "the two Macs did not leave the coupled conflict" >&2
	exit 1
}

echo "Testing TTMobileUITests on simulator $simulator..." >&2
TEST_RUNNER_TT_UITEST_FIXTURE="$fixture" \
	TEST_RUNNER_TT_UITEST_CONFLICT_EXCHANGE="$conflict/exchange" \
	TEST_RUNNER_TT_UITEST_EXCHANGE="$output/exchange" \
	TEST_RUNNER_TT_UITEST_WORK_DIR="$output/work" \
	xcodebuild test \
	-project "$ios_dir/TTMobile.xcodeproj" \
	-scheme TTMobile \
	-only-testing:"TTMobileUITests${TT_UITEST_ONLY:+/$TT_UITEST_ONLY}" \
	-destination "platform=iOS Simulator,id=$simulator" \
	-derivedDataPath "$cache_root/UIDerivedData" \
	-resultBundlePath "$output/TTMobileUITests.xcresult" \
	CODE_SIGNING_ALLOWED=NO \
	>"$output/xcodebuild-ui-test.log" 2>&1 || {
	tail -60 "$output/xcodebuild-ui-test.log" >&2
	echo "xcodebuild test failed; full log at $output/xcodebuild-ui-test.log" >&2
	exit 1
}
echo "TTMobileUITests passed; result bundle at $output/TTMobileUITests.xcresult" >&2
