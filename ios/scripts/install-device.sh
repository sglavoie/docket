#!/usr/bin/env bash
#
# Build tt for a physical iPhone, signed by your team, and install it over the
# cable. Installing over an earlier build keeps the app's data, so the same
# command is the upgrade. The installed app's path is printed last.
#
#   TT_IOS_DEVELOPMENT_TEAM=TEAMID ios/scripts/install-device.sh
#
# Environment:
#   TT_IOS_DEVELOPMENT_TEAM  signing team (required)
#   TT_IOS_DEVICE            iPhone UDID (default: the first physical iPhone)
#   TT_IOS_DEVICE_DERIVED    derived data (default ios/.cache/QualificationDerivedData)

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

fail() {
	echo "install-device: $*" >&2
	exit 1
}

[ "$(uname -s)" = Darwin ] || fail "needs macOS with Xcode"
team=${TT_IOS_DEVELOPMENT_TEAM:-}
[ -n "$team" ] || fail "set TT_IOS_DEVELOPMENT_TEAM to your signing team"
derived=${TT_IOS_DEVICE_DERIVED:-ios/.cache/QualificationDerivedData}
log=$(mktemp "${TMPDIR:-/tmp}/tt-install-device.XXXXXX")

devices=$(mktemp)
xcrun devicectl list devices --json-output "$devices" >/dev/null
device=${TT_IOS_DEVICE:-$(python3 -I - "$devices" <<'PY'
import json, sys
for d in json.load(open(sys.argv[1]))["result"]["devices"]:
    hw = d["hardwareProperties"]
    if hw.get("reality") == "physical" and hw.get("platform") == "iOS":
        print(hw["udid"])
        break
PY
)}
rm -f "$devices"
[ -n "$device" ] || fail "no physical iPhone is paired"

echo "install-device: building for $device (log: $log)" >&2
ios/scripts/generate-project.sh >>"$log" 2>&1 || fail "project generation failed: $log"
xcodebuild build -project ios/TTMobile.xcodeproj -scheme TTMobile \
	-destination "id=$device" -derivedDataPath "$derived" -allowProvisioningUpdates \
	DEVELOPMENT_TEAM="$team" >>"$log" 2>&1 || fail "the signed build failed: $log"
app="$derived/Build/Products/Debug-iphoneos/tt.app"
[ -d "$app" ] || fail "no app at $app"
codesign --verify --deep --strict "$app" || fail "$app is not validly signed"

echo "install-device: installing (keep the iPhone unlocked)" >&2
xcrun devicectl device install app --device "$device" "$app" >>"$log" 2>&1 || fail "installation failed: $log"
echo "$repo_root/$app"
