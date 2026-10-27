#!/usr/bin/env sh
#
# Package the release Docket.app the Homebrew cask installs: join the two darwin
# docket builds into one universal binary, build the Release app around it
# (build-app.sh, which signs and, with DOCKET_NOTARY_PROFILE, notarizes and
# staples), check both are universal and accepted by Gatekeeper, then zip the app
# beside the shell completions the cask links.
#
# Usage: package-release.sh VERSION ARM64_DOCKET AMD64_DOCKET COMPLETIONS_DIR OUT_ZIP
#
# VERSION has no leading v. The zip holds Docket.app and completions/ at its root.
#
# Environment:
#   DOCKET_CODESIGN_IDENTITY    see sign.sh; a release passes its Developer ID
#   DOCKET_NOTARY_PROFILE       see sign.sh; required unless the next is set
#   DOCKET_ALLOW_UNNOTARIZED=1  package without notarization, for a local dry run

set -eu

require_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required tool not found: $1" >&2
		exit 1
	fi
}

if [ "$#" -ne 5 ]; then
	echo "usage: package-release.sh VERSION ARM64_DOCKET AMD64_DOCKET COMPLETIONS_DIR OUT_ZIP" >&2
	exit 1
fi
version=$1 arm64=$2 amd64=$3 completions=$4 out=$5

case "$version" in
v*)
	echo "VERSION has no leading v: $version" >&2
	exit 1
	;;
esac
if [ -z "${DOCKET_NOTARY_PROFILE:-}" ] && [ "${DOCKET_ALLOW_UNNOTARIZED:-}" != 1 ]; then
	echo "a release app is notarized: set DOCKET_NOTARY_PROFILE (or DOCKET_ALLOW_UNNOTARIZED=1 for a dry run)" >&2
	exit 1
fi
for f in "$arm64" "$amd64"; do
	if [ ! -x "$f" ]; then
		echo "not an executable file: $f" >&2
		exit 1
	fi
done
if [ ! -f "$completions/docket.bash" ]; then
	echo "$completions holds no docket.bash" >&2
	exit 1
fi

require_tool lipo
require_tool ditto
require_tool codesign

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
macos_dir=$(dirname -- "$script_dir")

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

# Both slices, or neither: an Intel Mac must not get an app whose docket only
# runs on Apple silicon.
require_universal() {
	archs=" $(lipo -archs "$1") "
	for arch in arm64 x86_64; do
		case "$archs" in
		*" $arch "*) ;;
		*)
			echo "$1 is not universal:$archs" >&2
			exit 1
			;;
		esac
	done
}

lipo -create -output "$work/docket" "$arm64" "$amd64"
chmod 755 "$work/docket"
require_universal "$work/docket"

DOCKET_BINARY="$work/docket" DOCKET_APP_VERSION="$version" "$script_dir/build-app.sh" Release

app="$macos_dir/build/Docket.app"
require_universal "$app/Contents/MacOS/Docket"
require_universal "$app/Contents/Resources/bin/docket"
codesign --verify --strict --deep "$app"
shipped=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")
if [ "$shipped" != "$version" ]; then
	echo "Docket.app reports version $shipped, want $version" >&2
	exit 1
fi
if [ -n "${DOCKET_NOTARY_PROFILE:-}" ]; then
	spctl --assess --type execute --verbose=2 "$app"
fi

stage="$work/stage"
mkdir -p "$stage/completions"
ditto "$app" "$stage/Docket.app"
cp "$completions"/* "$stage/completions/"
mkdir -p "$(dirname -- "$out")"
rm -f "$out"
ditto -c -k --norsrc --noextattr --noacl "$stage" "$out"
echo "Packaged $out"
