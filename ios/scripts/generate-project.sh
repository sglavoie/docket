#!/usr/bin/env sh
#
# Prepare ios/DocketMobile.xcodeproj: build the Go bridge XCFramework (a no-op while
# its fingerprint is current), resolve the pinned XcodeGen, and generate the
# project from ios/project.yml.
#
# Environment:
#   DOCKET_IOS_CACHE            bridge tool cache (default ios/.cache, gitignored)
#   DOCKET_MACOS_CACHE          XcodeGen cache (default macos/.cache, gitignored)

set -eu

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
ios_dir=$(dirname -- "$script_dir")
repo_root=$(dirname -- "$ios_dir")

"$script_dir/build-bridge.sh"

xcodegen=$("$repo_root/macos/scripts/resolve-xcodegen.sh")
"$xcodegen" generate --quiet --spec "$ios_dir/project.yml" --project "$ios_dir"
