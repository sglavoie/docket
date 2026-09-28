#!/usr/bin/env sh
#
# Sign tt.app inside out: the bundled tt, then the app. Ad-hoc by default,
# which runs on this Mac only. With a Developer ID the signature carries the
# hardened runtime and a secure timestamp, which notarization requires, and a
# notarytool keychain profile then notarizes and staples the app.
#
# Usage: sign.sh APP
#
# Environment:
#   TT_CODESIGN_IDENTITY    codesign identity, e.g.
#                           "Developer ID Application: Name (TEAMID)";
#                           unset or "-" signs ad-hoc
#   TT_NOTARY_PROFILE       `xcrun notarytool store-credentials` profile name;
#                           requires a Developer ID identity

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

identity="${TT_CODESIGN_IDENTITY:--}"
notary_profile="${TT_NOTARY_PROFILE:-}"

if [ "$identity" = - ]; then
	if [ -n "$notary_profile" ]; then
		echo "TT_NOTARY_PROFILE needs a Developer ID TT_CODESIGN_IDENTITY; ad-hoc apps cannot be notarized" >&2
		exit 1
	fi
	set -- --force --sign -
else
	set -- --force --sign "$identity" --options runtime --timestamp
fi

bundled="$app/Contents/Resources/bin/tt"
if [ -f "$bundled" ]; then
	codesign "$@" --identifier io.github.sglavoie.tt.cli "$bundled"
fi
codesign "$@" "$app"
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
archive=$(mktemp -d)/tt.zip
trap 'rm -rf "$(dirname -- "$archive")"' EXIT HUP INT TERM
ditto -c -k --keepParent "$app" "$archive"
echo "Submitting $app for notarization..."
xcrun notarytool submit "$archive" --keychain-profile "$notary_profile" --wait
xcrun stapler staple "$app"
xcrun stapler validate "$app"
echo "Notarized and stapled $app"
