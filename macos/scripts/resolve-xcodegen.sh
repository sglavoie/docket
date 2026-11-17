#!/usr/bin/env sh
#
# Print the path of the pinned XcodeGen, downloading it into the cache first
# when neither PATH nor the cache holds that version. macOS and iOS project
# generation both resolve it here, so one pin and one cache serve both.
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

require_tool curl
require_tool shasum
require_tool unzip

# shellcheck disable=SC1091
. "$macos_dir/XCODEGEN_VERSION"
: "${XCODEGEN_VERSION:?missing in XCODEGEN_VERSION}"
: "${XCODEGEN_SHA256:?missing in XCODEGEN_VERSION}"

cache_root="${DOCKET_MACOS_CACHE:-$macos_dir/.cache}"
mkdir -p "$cache_root"

# --- XcodeGen ------------------------------------------------------------------
#
# A PATH copy is used only when it reports the pinned version; otherwise the
# checksummed release zip is unpacked into the cache. It keeps its share/
# directory beside bin/, which XcodeGen reads its templates from.

xcodegen_matches() {
	[ -n "$1" ] && [ "$("$1" --version 2>/dev/null || true)" = "Version: $XCODEGEN_VERSION" ]
}

resolve_xcodegen() {
	if xcodegen_matches "$(command -v xcodegen 2>/dev/null || true)"; then
		command -v xcodegen
		return
	fi
	xcodegen_dir="$cache_root/xcodegen-$XCODEGEN_VERSION"
	if xcodegen_matches "$xcodegen_dir/bin/xcodegen"; then
		echo "$xcodegen_dir/bin/xcodegen"
		return
	fi
	archive="$cache_root/xcodegen-$XCODEGEN_VERSION.zip"
	echo "Downloading XcodeGen $XCODEGEN_VERSION..." >&2
	curl -fL --retry 3 -o "$archive.part" \
		"https://github.com/yonaskolb/XcodeGen/releases/download/$XCODEGEN_VERSION/xcodegen.zip" >&2
	actual=$(shasum -a 256 "$archive.part" | awk '{print $1}')
	if [ "$actual" != "$XCODEGEN_SHA256" ]; then
		rm -f "$archive.part"
		echo "xcodegen checksum mismatch: expected $XCODEGEN_SHA256, got $actual" >&2
		exit 1
	fi
	unpack="$cache_root/xcodegen-unpack"
	rm -rf "$unpack" "$xcodegen_dir"
	unzip -q "$archive.part" -d "$unpack"
	mv "$unpack/xcodegen" "$xcodegen_dir"
	rm -rf "$unpack" "$archive.part"
	if ! xcodegen_matches "$xcodegen_dir/bin/xcodegen"; then
		echo "unpacked xcodegen at $xcodegen_dir does not report $XCODEGEN_VERSION" >&2
		exit 1
	fi
	echo "$xcodegen_dir/bin/xcodegen"
}

resolve_xcodegen
