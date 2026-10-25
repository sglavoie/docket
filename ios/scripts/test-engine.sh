#!/usr/bin/env sh
#
# Run the TTEngine simulator suite against the real gomobile-bound engine.
# Builds the bridge (a no-op while current), copies the representative
# compatibility fixture into OUTPUT_DIR/workspace, and runs `xcodebuild test`
# on an iOS simulator with OUTPUT_DIR/evidence as the suite's evidence
# directory. The result bundle is OUTPUT_DIR/TTEngine.xcresult and the log
# OUTPUT_DIR/xcodebuild-test.log.
#
#   ios/scripts/test-engine.sh OUTPUT_DIR
#
# Environment:
#   DOCKET_IOS_SIMULATOR        simulator UDID (default: the first available
#                           iPhone on an iOS 18 or later runtime)
#   DOCKET_IOS_CACHE            cache root (default ios/.cache, gitignored)

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

"$script_dir/build-bridge.sh"

# The first available iPhone under an "-- iOS N.M --" heading with N >= 18.
simulator=${DOCKET_IOS_SIMULATOR:-$(xcrun simctl list devices available | awk '
	/^-- iOS [0-9]+/ { split($3, version, "."); eligible = (version[1] + 0 >= 18); next }
	/^-- / { eligible = 0; next }
	eligible && /iPhone/ && match($0, /\([0-9A-F-]{36}\)/) {
		print substr($0, RSTART + 1, RLENGTH - 2); exit
	}')}
if [ -z "$simulator" ]; then
	echo "no available iPhone simulator on iOS 18 or later; set DOCKET_IOS_SIMULATOR" >&2
	exit 1
fi

rm -rf "$output/workspace" "$output/evidence" "$output/TTEngine.xcresult"
cp -R "$fixture" "$output/workspace"
mkdir -p "$output/evidence"

echo "Testing TTEngine on simulator $simulator..." >&2
cd "$ios_dir/TTEngine"
TEST_RUNNER_DOCKET_BRIDGE_FIXTURE_DIR="$output/workspace" \
	TEST_RUNNER_DOCKET_BRIDGE_EVIDENCE_DIR="$output/evidence" \
	xcodebuild test \
	-scheme TTEngine \
	-destination "platform=iOS Simulator,id=$simulator" \
	-derivedDataPath "$cache_root/DerivedData" \
	-resultBundlePath "$output/TTEngine.xcresult" \
	>"$output/xcodebuild-test.log" 2>&1 || {
	tail -50 "$output/xcodebuild-test.log" >&2
	echo "xcodebuild test failed; full log at $output/xcodebuild-test.log" >&2
	exit 1
}
echo "TTEngine suite passed; evidence in $output/evidence" >&2
