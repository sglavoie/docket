#!/usr/bin/env sh
#
# Install macos/build/Docket.app (built by `just mac-app Release`) into
# /Applications, replacing an earlier copy. A running Docket.app is left alone:
# replacing a bundle under a live process would end its Docket session, so quit it
# first. An old /Applications/tt.app is removed only when you answer yes.
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

# The app was called tt.app before the rename to Docket. It has another bundle
# ID, so both would stay installed side by side; offer to remove the old one.
legacy="$install_dir/tt.app"
if [ -d "$legacy" ]; then
	if pgrep -f "^$legacy/Contents/MacOS/tt" >/dev/null 2>&1; then
		echo "$legacy is running; quit it (menu bar icon > Quit tt) and rerun" >&2
		exit 1
	fi
	printf '%s is the app before its rename to Docket. Remove it? [y/N] ' "$legacy"
	answer=
	if [ -t 0 ]; then
		read -r answer || answer=
	else
		echo
	fi
	case "$answer" in
	[yY] | [yY][eE][sS])
		rm -rf "$legacy"
		echo "Removed $legacy"
		;;
	*)
		echo "Kept $legacy; remove it yourself once Docket.app works"
		;;
	esac
fi

mkdir -p "$install_dir"
rm -rf "$dest"
ditto "$app" "$dest"
lsregister=/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister
if [ -x "$lsregister" ]; then
	"$lsregister" -f "$dest"
fi
# A widget extension already running keeps drawing with the code it started
# with, so the desktop would show the old widget until the next login. Ending it
# is harmless: macOS starts the installed one the next time the widget draws.
pkill -f "^$dest/Contents/PlugIns/DocketWidgets.appex/" 2>/dev/null || true
echo "Installed $dest"
