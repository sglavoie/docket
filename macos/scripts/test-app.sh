#!/usr/bin/env sh
#
# Run the TTAppTests XCTest suite: generate macos/tt.xcodeproj
# (generate-project.sh), then `xcodebuild test`. The tests are hosted in the
# app, so they build it too.
#
# Environment:
#   TT_MACOS_CACHE          cache root (default macos/.cache, gitignored)
#   TT_MACOS_RESULTS        optional directory for a unique XCTest result bundle

set -eu

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
macos_dir=$(dirname -- "$script_dir")

"$script_dir/generate-project.sh"

# A unique bundle also makes repeated local runs safe; xcodebuild refuses an
# existing result path. Keep failed runs too for CI artifact collection.
set --
if [ -n "${TT_MACOS_RESULTS:-}" ]; then
	mkdir -p "$TT_MACOS_RESULTS"
	result_dir=$(mktemp -d "$TT_MACOS_RESULTS/desktop.XXXXXX")
	set -- -resultBundlePath "$result_dir/tests.xcresult"
fi

xcodebuild test "$@" \
	-project "$macos_dir/tt.xcodeproj" \
	-scheme tt \
	-destination "platform=macOS,arch=$(uname -m)" \
	-derivedDataPath "$macos_dir/DerivedData" \
	-quiet
