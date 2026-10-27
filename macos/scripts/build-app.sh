#!/usr/bin/env sh
#
# Build Docket.app: generate macos/Docket.xcodeproj (generate-project.sh), build it,
# copy the product to macos/build/Docket.app, bundle docket, and sign (sign.sh).
#
# Usage: build-app.sh [Debug|Release]   (default Debug)
#
# Environment:
#   DOCKET_BINARY               docket binary to bundle at Contents/Resources/bin/docket; required
#                           for Release (`just mac-app Release` builds it with
#                           the `just build` ldflags), optional for Debug, which
#                           otherwise resolves ~/.local/bin/docket then PATH
#   DOCKET_APP_VERSION          CFBundleShortVersionString, e.g. 0.2.0 (default:
#                           project.yml's MARKETING_VERSION)
#   DOCKET_MACOS_CACHE          cache root (default macos/.cache, gitignored)
#   DOCKET_CODESIGN_IDENTITY    see sign.sh (default Apple Development, else ad-hoc)

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

docket_binary="${DOCKET_BINARY:-}"
if [ "$configuration" = Release ] && [ -z "$docket_binary" ]; then
	echo "a Release build bundles docket: set DOCKET_BINARY, or run \`just mac-app Release\`" >&2
	exit 1
fi
if [ -n "$docket_binary" ] && [ ! -x "$docket_binary" ]; then
	echo "DOCKET_BINARY is not an executable file: $docket_binary" >&2
	exit 1
fi

"$script_dir/generate-project.sh"
require_tool ditto

# --- Build -----------------------------------------------------------------------

# A version rides as a trailing build-setting override, which outranks the spec.
if [ -n "${DOCKET_APP_VERSION:-}" ]; then
	set -- "MARKETING_VERSION=$DOCKET_APP_VERSION"
else
	set --
fi

derived_data="$macos_dir/DerivedData"
xcodebuild build \
	-project "$macos_dir/Docket.xcodeproj" \
	-scheme Docket \
	-configuration "$configuration" \
	-destination "platform=macOS,arch=$(uname -m)" \
	-derivedDataPath "$derived_data" \
	-quiet \
	"$@"

product="$derived_data/Build/Products/$configuration/Docket.app"
if [ ! -f "$product/Contents/Info.plist" ]; then
	echo "xcodebuild reported success but $product is missing" >&2
	exit 1
fi

out="$macos_dir/build/Docket.app"
mkdir -p "$macos_dir/build"
rm -rf "$out"
ditto "$product" "$out"

# --- Bundle docket and sign ----------------------------------------------------------
#
# Copying docket in breaks the seal xcodebuild wrote, so the bundle is always
# re-signed afterwards, inside out.

if [ -n "$docket_binary" ]; then
	mkdir -p "$out/Contents/Resources/bin"
	cp "$docket_binary" "$out/Contents/Resources/bin/docket"
	chmod 755 "$out/Contents/Resources/bin/docket"
fi
"$script_dir/sign.sh" "$out"

echo "Built $out ($configuration)"
