#!/usr/bin/env sh
#
# Install macos/build/Docket.app (built by `just mac-app Release`) into
# /Applications, replacing an earlier copy. A running Docket.app is left alone:
# replacing a bundle under a live process would end its Docket session, so quit it
# first. It replaces v1's Docket.app the same way: both share a bundle ID, so
# Settings carry over.
#
# Environment:
#   DOCKET_APP_INSTALL_DIR      destination directory (default /Applications)

set -eu

require_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required tool not found: $1" >&2
		exit 1
	fi
}

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
macos_dir=$(dirname -- "$script_dir")

require_tool ditto
require_tool codesign

app="$macos_dir/build/Docket.app"
if [ ! -x "$app/Contents/Resources/bin/docket" ]; then
	echo "$app has no bundled docket; build it with \`just mac-app Release\`" >&2
	exit 1
fi
codesign --verify --strict "$app"

install_dir="${DOCKET_APP_INSTALL_DIR:-/Applications}"
dest="$install_dir/Docket.app"
if pgrep -f "^$dest/Contents/MacOS/Docket" >/dev/null 2>&1; then
	echo "$dest is running; quit it (menu bar icon > Quit Docket) and rerun" >&2
	exit 1
fi

mkdir -p "$install_dir"
rm -rf "$dest"
ditto "$app" "$dest"
lsregister=/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister
if [ -x "$lsregister" ]; then
	"$lsregister" -f "$dest"
fi
# v1's widget extension goes with v1's bundle; one still running would keep
# drawing until the next login.
pkill -f "/Docket.app/Contents/PlugIns/DocketWidgets.appex/" 2>/dev/null || true
echo "Installed $dest"
