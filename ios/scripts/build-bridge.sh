#!/usr/bin/env sh
#
# Build ios/TTEngine/Frameworks/TTBridgeCore.xcframework, the Go engine bound
# with gomobile, with a device slice (ios-arm64) and a simulator slice
# (ios-arm64_x86_64-simulator). A no-op while its fingerprint is current.
#
# gomobile and gobind are the versions go.mod pins as tool dependencies, built
# into the cache rather than taken from PATH, so every machine binds with the
# same generator. The fingerprint covers that pin, the Go toolchain, the
# deployment target, this script, go.mod and go.sum, and every in-module
# source file the bound package compiles for iOS; changing any of them
# rebuilds, and nothing else does.
#
# With --fingerprint it prints the fingerprint the current sources would carry
# and builds nothing, so a verifier can compare it with the published stamp.
#
# Environment:
#   DOCKET_IOS_CACHE            cache root (default ios/.cache, gitignored)
#   DOCKET_IOS_FORCE_BRIDGE=1   rebuild even when the fingerprint is current

set -eu

require_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "required tool not found: $1" >&2
		exit 1
	fi
}

script_dir=$(CDPATH="" cd -- "$(dirname -- "$0")" && pwd)
ios_dir=$(dirname -- "$script_dir")
repo_root=$(dirname -- "$ios_dir")

if [ "$(uname -s)" != Darwin ]; then
	echo "the iOS bridge builds on macOS only" >&2
	exit 1
fi

require_tool go
require_tool xcrun
require_tool shasum
require_tool plutil

ios_version=18.0
package=./mobile/ttbridge
framework_dir="$ios_dir/TTEngine/Frameworks"
framework="$framework_dir/TTBridgeCore.xcframework"
stamp="$framework_dir/TTBridgeCore.fingerprint"
cache_root="${DOCKET_IOS_CACHE:-$ios_dir/.cache}"

cd "$repo_root"

mobile_version=$(go list -m -f '{{.Version}}' golang.org/x/mobile)
tool_bin="$cache_root/gomobile-$mobile_version/bin"

fingerprint() {
	{
		echo "x/mobile $mobile_version"
		go version
		echo "iosversion $ios_version"
		echo "xcode $(xcodebuild -version 2>/dev/null | tr '\n' ' ')"
		shasum -a 256 "$script_dir/build-bridge.sh" go.mod go.sum
		# Every in-module file the bound package compiles for iOS, by content.
		# The template's $dir is Go's, not the shell's.
		# shellcheck disable=SC2016
		GOOS=ios GOARCH=arm64 CGO_ENABLED=1 go list -deps \
			-f '{{if .Module}}{{if .Module.Main}}{{$dir := .Dir}}{{range .GoFiles}}{{$dir}}/{{.}}{{"\n"}}{{end}}{{range .CgoFiles}}{{$dir}}/{{.}}{{"\n"}}{{end}}{{range .EmbedFiles}}{{$dir}}/{{.}}{{"\n"}}{{end}}{{end}}{{end}}' \
			"$package" | LC_ALL=C sort | while IFS= read -r file; do
			shasum -a 256 "$file" | sed "s|$repo_root/||"
		done
	} | shasum -a 256 | awk '{print $1}'
}

want=$(fingerprint)
if [ "${1:-}" = --fingerprint ]; then
	echo "$want"
	exit 0
fi
if [ "${DOCKET_IOS_FORCE_BRIDGE:-0}" != 1 ] && [ -d "$framework" ] && [ -f "$stamp" ] &&
	[ "$(cat "$stamp")" = "$want" ]; then
	echo "TTBridgeCore.xcframework is current ($want)"
	exit 0
fi

if [ ! -x "$tool_bin/gomobile" ] || [ ! -x "$tool_bin/gobind" ]; then
	echo "Building gomobile $mobile_version..." >&2
	mkdir -p "$tool_bin"
	go build -o "$tool_bin/gomobile" golang.org/x/mobile/cmd/gomobile
	go build -o "$tool_bin/gobind" golang.org/x/mobile/cmd/gobind
fi

echo "Binding $package for iOS $ios_version (device and simulator)..." >&2
mkdir -p "$framework_dir"
work=$(mktemp -d "$framework_dir/.bind.XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM
PATH="$tool_bin:$PATH" gomobile bind \
	-target=ios,iossimulator \
	-iosversion="$ios_version" \
	-trimpath \
	-o "$work/TTBridgeCore.xcframework" \
	"$package"

# Both slices, or nothing is published.
libraries=$(plutil -extract AvailableLibraries json -o - "$work/TTBridgeCore.xcframework/Info.plist")
for slice in '"ios-arm64"' '"ios-arm64_x86_64-simulator"'; do
	case "$libraries" in
	*"$slice"*) ;;
	*)
		echo "gomobile produced no $slice slice: $libraries" >&2
		exit 1
		;;
	esac
done

rm -rf "$framework" "$stamp"
mv "$work/TTBridgeCore.xcframework" "$framework"
echo "$want" >"$stamp"
echo "Built $framework ($want)"
