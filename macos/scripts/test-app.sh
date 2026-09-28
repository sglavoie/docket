#!/usr/bin/env sh
#
# Run the TTAppTests XCTest suite: generate macos/tt.xcodeproj
# (generate-project.sh), then `xcodebuild test`. The tests are hosted in the
# app, so they build it too.
#
# Environment:
#   TT_MACOS_CACHE          cache root (default macos/.cache, gitignored)

set -eu

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
macos_dir=$(dirname -- "$script_dir")

"$script_dir/generate-project.sh"

xcodebuild test \
	-project "$macos_dir/tt.xcodeproj" \
	-scheme tt \
	-destination "platform=macOS,arch=$(uname -m)" \
	-derivedDataPath "$macos_dir/DerivedData" \
	-quiet
