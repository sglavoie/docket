#!/usr/bin/env sh
#
# Sign Docket.app inside out: the bundled docket, the widget and share
# extensions, then the app, each extension and the app with its entitlements
# file, which carries the App Group the widget reads the pulse from. By default with this
# Mac's first Apple Development identity, else ad-hoc; either runs on this Mac
# only. A certificate gives the app a designated requirement that survives a
# rebuild, so privacy grants (Documents, notifications) are asked for once
# rather than after every install; an ad-hoc signature is its cdhash, which
# macOS treats as a new app each build. With a Developer ID the signature
# carries the hardened runtime and a secure timestamp, which notarization
# requires, and a notarytool keychain profile then notarizes and staples the app.
#
# Usage: sign.sh APP
#
# Environment:
#   DOCKET_CODESIGN_IDENTITY    codesign identity, e.g.
#                           "Developer ID Application: Name (TEAMID)";
#                           unset picks the first Apple Development
#                           identity, else ad-hoc; "-" forces ad-hoc
#   DOCKET_NOTARY_PROFILE       `xcrun notarytool store-credentials` profile name;
#                           requires a Developer ID identity
#   DOCKET_NOTARY_KEYCHAIN      keychain holding that profile (default: the
#                           keychain search list)

set -eu

require_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required tool not found: $1" >&2
		exit 1
	fi
}

if [ "$#" -ne 1 ] || [ ! -d "$1/Contents" ]; then
	echo "usage: sign.sh APP" >&2
	exit 1
fi
app=$1

require_tool codesign

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
macos_dir=$(dirname -- "$script_dir")

notary_profile="${DOCKET_NOTARY_PROFILE:-}"
if [ -n "${DOCKET_CODESIGN_IDENTITY:-}" ]; then
	identity=$DOCKET_CODESIGN_IDENTITY
	development=
else
	# The SHA-1 rather than the name, which an expired twin may share.
	identity=$(security find-identity -v -p codesigning 2>/dev/null |
		sed -n 's/^ *[0-9][0-9]*) \([0-9A-F]\{40\}\) "Apple Development: .*/\1/p' | head -n 1)
	identity=${identity:--}
	development=$identity
fi

if [ "$identity" = - ] || [ -n "$development" ]; then
	if [ -n "$notary_profile" ]; then
		echo "DOCKET_NOTARY_PROFILE needs a Developer ID DOCKET_CODESIGN_IDENTITY" >&2
		exit 1
	fi
	set -- --force --sign "$identity"
else
	set -- --force --sign "$identity" --options runtime --timestamp
fi

bundled="$app/Contents/Resources/bin/docket"
if [ -f "$bundled" ]; then
	codesign "$@" --identifier io.github.sglavoie.docket.cli "$bundled"
fi
for extension in "$app"/Contents/PlugIns/*.appex; do
	[ -d "$extension" ] || continue
	name=$(basename "$extension" .appex)
	codesign "$@" --entitlements "$macos_dir/$name/$name.entitlements" "$extension"
done
codesign "$@" --entitlements "$macos_dir/DocketApp/Resources/Docket.entitlements" "$app"
codesign --verify --strict --verbose=1 "$app"

if [ "$identity" = - ]; then
	echo "Signed $app ad-hoc"
	exit 0
fi
echo "Signed $app as $identity"

if [ -z "$notary_profile" ]; then
	exit 0
fi

require_tool xcrun
require_tool ditto
archive=$(mktemp -d)/docket.zip
trap 'rm -rf "$(dirname -- "$archive")"' EXIT HUP INT TERM
ditto -c -k --keepParent "$app" "$archive"
set -- --keychain-profile "$notary_profile"
if [ -n "${DOCKET_NOTARY_KEYCHAIN:-}" ]; then
	set -- "$@" --keychain "$DOCKET_NOTARY_KEYCHAIN"
fi
echo "Submitting $app for notarization..."
result=$(xcrun notarytool submit "$archive" "$@" --wait --output-format json)
status=$(printf '%s' "$result" | plutil -extract status raw -o - -)
if [ "$status" != Accepted ]; then
	echo "notarization ended $status" >&2
	submission=$(printf '%s' "$result" | plutil -extract id raw -o - - 2>/dev/null || true)
	if [ -n "$submission" ]; then
		xcrun notarytool log "$submission" "$@" >&2 || true
	fi
	exit 1
fi
xcrun stapler staple "$app"
xcrun stapler validate "$app"
echo "Notarized and stapled $app"
