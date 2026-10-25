#!/usr/bin/env sh
#
# Prepare macos/tt.xcodeproj: vendor GhosttyKit (a no-op when current), ensure
# the pinned XcodeGen, and generate the project from macos/project.yml.
# build-app.sh and test-app.sh both start here.
#
# Environment:
#   DOCKET_MACOS_CACHE          cache root (default macos/.cache, gitignored)

set -eu

require_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required tool not found: $1" >&2
		exit 1
	fi
}

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
macos_dir=$(dirname -- "$script_dir")

if [ "$(uname -s)" != Darwin ]; then
	echo "tt.app builds on macOS only" >&2
	exit 1
fi

require_tool xcodebuild

"$script_dir/build-ghosttykit.sh"

xcodegen=$("$script_dir/resolve-xcodegen.sh")
"$xcodegen" generate --quiet --spec "$macos_dir/project.yml" --project "$macos_dir"
