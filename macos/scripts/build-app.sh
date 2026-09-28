#!/usr/bin/env sh
#
# Build tt.app: generate macos/tt.xcodeproj (generate-project.sh), build it,
# copy the product to macos/build/tt.app, bundle tt, and sign (sign.sh).
#
# Usage: build-app.sh [Debug|Release]   (default Debug)
#
# Environment:
#   TT_BINARY               tt to bundle at Contents/Resources/bin/tt; required
#                           for Release (`just mac-app Release` builds it with
#                           the `just build` ldflags), optional for Debug, which
#                           otherwise resolves ~/.local/bin/tt then PATH
#   TT_MACOS_CACHE          cache root (default macos/.cache, gitignored)
#   TT_CODESIGN_IDENTITY    see sign.sh (default ad-hoc)

set -eu

require_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required tool not found: $1" >&2
		exit 1
	fi
}

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
macos_dir=$(dirname -- "$script_dir")

configuration="${1:-Debug}"
case "$configuration" in
Debug | Release) ;;
*)
	echo "configuration must be Debug or Release, not $configuration" >&2
	exit 1
	;;
esac

tt_binary="${TT_BINARY:-}"
if [ "$configuration" = Release ] && [ -z "$tt_binary" ]; then
	echo "a Release build bundles tt: set TT_BINARY, or run \`just mac-app Release\`" >&2
	exit 1
fi
if [ -n "$tt_binary" ] && [ ! -x "$tt_binary" ]; then
	echo "TT_BINARY is not an executable file: $tt_binary" >&2
	exit 1
fi

"$script_dir/generate-project.sh"
require_tool ditto

# --- Build -----------------------------------------------------------------------

derived_data="$macos_dir/DerivedData"
xcodebuild build \
	-project "$macos_dir/tt.xcodeproj" \
	-scheme tt \
	-configuration "$configuration" \
	-destination "platform=macOS,arch=$(uname -m)" \
	-derivedDataPath "$derived_data" \
	-quiet

product="$derived_data/Build/Products/$configuration/tt.app"
if [ ! -f "$product/Contents/Info.plist" ]; then
	echo "xcodebuild reported success but $product is missing" >&2
	exit 1
fi

out="$macos_dir/build/tt.app"
mkdir -p "$macos_dir/build"
rm -rf "$out"
ditto "$product" "$out"

# --- Bundle tt and sign ----------------------------------------------------------
#
# Copying tt in breaks the seal xcodebuild wrote, so the bundle is always
# re-signed afterwards, inside out.

if [ -n "$tt_binary" ]; then
	mkdir -p "$out/Contents/Resources/bin"
	cp "$tt_binary" "$out/Contents/Resources/bin/tt"
	chmod 755 "$out/Contents/Resources/bin/tt"
fi
"$script_dir/sign.sh" "$out"

echo "Built $out ($configuration)"
