#!/usr/bin/env sh
#
# Run the TTMobile browsing UI tests on an iOS simulator against the real
# gomobile-bound engine. Prepares, under OUTPUT_DIR:
#   exchange/   an exchange folder a desktop `tt sync enroll` of the
#               representative fixture published (the join test's source)
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

echo "Testing TTMobileUITests on simulator $simulator..." >&2
TEST_RUNNER_TT_UITEST_FIXTURE="$fixture" \
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
