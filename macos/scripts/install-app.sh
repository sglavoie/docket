#!/usr/bin/env sh
#
# Install macos/build/tt.app (built by `just mac-app Release`) into
# /Applications, replacing an earlier copy. A running tt.app is left alone:
# replacing a bundle under a live process would end its tt session, so quit it
# first.
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

app="$macos_dir/build/tt.app"
if [ ! -x "$app/Contents/Resources/bin/tt" ]; then
	echo "$app has no bundled tt; build it with \`just mac-app Release\`" >&2
	exit 1
fi
codesign --verify --strict "$app"

install_dir="${DOCKET_APP_INSTALL_DIR:-/Applications}"
dest="$install_dir/tt.app"
if pgrep -f "^$dest/Contents/MacOS/tt" >/dev/null 2>&1; then
	echo "$dest is running; quit it (menu bar icon > Quit tt) and rerun" >&2
	exit 1
fi

mkdir -p "$install_dir"
rm -rf "$dest"
ditto "$app" "$dest"
lsregister=/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister
if [ -x "$lsregister" ]; then
	"$lsregister" -f "$dest"
fi
echo "Installed $dest"
