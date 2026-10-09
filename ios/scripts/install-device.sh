#!/usr/bin/env bash
#
# Build tt for a physical iPhone, signed by your team, and install it over the
# cable. Installing over an earlier build keeps the app's data, so the same
# command is the upgrade. --launch also opens the updated app. The installed
# app's absolute path is the only stdout output (qualification consumes it).
#
#   TT_IOS_DEVELOPMENT_TEAM=TEAMID ios/scripts/install-device.sh
#
# Environment:
#   TT_IOS_DEVELOPMENT_TEAM  signing team (default: the previous device build's team)
#   TT_IOS_DEVICE            iPhone UDID (required when more than one is paired)
#   TT_IOS_DEVICE_DERIVED    derived data (default ios/.cache/QualificationDerivedData)
#   TT_IOS_RESULTS           log root (default ios/.cache/results)

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

fail() {
	echo "install-device: $*" >&2
	if [ -n "${log:-}" ] && [ -s "$log" ]; then
		tail -40 "$log" >&2
		echo "install-device: full log: $log" >&2
	fi
	exit 1
}

launch=false
if [ "${1:-}" = --launch ]; then launch=true; shift; fi
[ "$#" -eq 0 ] || fail "usage: $0 [--launch]"
[ "$(uname -s)" = Darwin ] || fail "needs macOS with Xcode"
for tool in xcrun xcodebuild codesign python3; do
	command -v "$tool" >/dev/null || fail "required tool not found: $tool"
done
derived=${TT_IOS_DEVICE_DERIVED:-ios/.cache/QualificationDerivedData}
case "$derived" in /*) ;; *) derived="$repo_root/$derived" ;; esac
app="$derived/Build/Products/Debug-iphoneos/tt.app"
team=${TT_IOS_DEVELOPMENT_TEAM:-}
if [ -z "$team" ] && [ -d "$app" ]; then
	team=$(codesign -d --verbose=4 "$app" 2>&1 | awk -F= '/^TeamIdentifier=/ && $2 != "not set" { print $2 }') || true
fi
[ -n "$team" ] || fail "no previous signing team; run just ios-install TEAMID once, or set TT_IOS_DEVELOPMENT_TEAM"

results=${TT_IOS_RESULTS:-ios/.cache/results}
mkdir -p "$results"
output=$(mktemp -d "$results/device-install.XXXXXX")
output=$(cd "$output" && pwd)
log="$output/install.log"
devices="$output/devices.json"
xcrun devicectl list devices --timeout 30 --json-output "$devices" >"$log" 2>&1 || fail "could not list paired devices"
device=$(python3 -I - "$devices" "${TT_IOS_DEVICE:-}" <<'PY'
import json, sys
phones = []
for d in json.load(open(sys.argv[1]))["result"]["devices"]:
    hw = d.get("properties", {}).get("hardware", d.get("hardwareProperties", {}))
    if hw.get("reality") == "physical" and hw.get("platform") == "iOS" and hw.get("deviceType") == "iPhone":
        phones.append(hw["udid"])
requested = sys.argv[2]
if requested:
    if requested not in phones:
        sys.exit(f"iPhone {requested} is not paired; connect it, unlock it, and trust this Mac")
    print(requested)
elif len(phones) == 1:
    print(phones[0])
elif not phones:
    sys.exit("no physical iPhone is paired; connect it, unlock it, and trust this Mac")
else:
    sys.exit("multiple iPhones are paired; select one with just ios-install TEAMID UDID: " + ", ".join(phones))
PY
) || fail "could not select an iPhone"

echo "install-device: building for $device with team $team (log: $log)" >&2
ios/scripts/generate-project.sh >>"$log" 2>&1 || fail "project generation failed: $log"
xcodebuild build -project ios/TTMobile.xcodeproj -scheme TTMobile \
	-configuration Debug -destination "id=$device" -destination-timeout 60 \
	-derivedDataPath "$derived" -allowProvisioningUpdates \
	DEVELOPMENT_TEAM="$team" >>"$log" 2>&1 || fail "the signed build failed: $log"
[ -d "$app" ] || fail "no app at $app"
codesign --verify --deep --strict "$app" >>"$log" 2>&1 || fail "$app is not validly signed"

echo "install-device: installing (keep the iPhone unlocked)" >&2
xcrun devicectl device install app --device "$device" --timeout 120 \
	--json-output "$output/install.json" "$app" >>"$log" 2>&1 || fail "installation failed; connect and unlock the iPhone, then rerun"
if "$launch"; then
	echo "install-device: launching tt" >&2
	xcrun devicectl device process launch --device "$device" --timeout 60 \
		--terminate-existing --json-output "$output/launch.json" io.github.sglavoie.tt.mobile \
		>>"$log" 2>&1 || fail "the update is installed, but launch failed; unlock the iPhone and open tt"
fi
echo "install-device: installed successfully (log: $log)" >&2
echo "$app"
