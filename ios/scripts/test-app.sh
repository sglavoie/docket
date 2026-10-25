#!/usr/bin/env sh
# Hosted app tests, including structured schedules through the real Go engine.
set -eu
if [ "$#" -ne 1 ]; then
    echo "usage: $0 OUTPUT_DIR" >&2
    exit 2
fi
script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
ios_dir=$(dirname -- "$script_dir")
mkdir -p "$1"
output=$(CDPATH="" cd -- "$1" && pwd)
if [ -d "$output/DocketMobileTests.xcresult" ]; then
    rm -r "$output/DocketMobileTests.xcresult"
fi
"$script_dir/generate-project.sh"
simulator=$("$script_dir/simulator.sh" default)
xcrun simctl boot "$simulator" >/dev/null 2>&1 || true
xcrun simctl bootstatus "$simulator" -b >/dev/null
xcodebuild test -project "$ios_dir/DocketMobile.xcodeproj" -scheme DocketMobile \
    -only-testing:DocketMobileTests \
    -destination "platform=iOS Simulator,id=$simulator" \
    -derivedDataPath "${DOCKET_IOS_CACHE:-$ios_dir/.cache}/UIDerivedData" \
    -resultBundlePath "$output/DocketMobileTests.xcresult" \
    CODE_SIGNING_ALLOWED=NO
