#!/usr/bin/env bash
#
# Record a disposable real iCloud exchange between this Mac and a physical
# iPhone, and write it as evidence the session-8 verifier validates.
#
# The Mac runs the originator of the exchange contract through docket-sync-helper
# and a bridgehost engine; the iPhone runs the responder as the hosted
# DocketMobileDeviceTests suite inside the signed app. Both use a fresh copy of the
# representative compatibility fixture, seeded through the exchange folder
# into runs/RUN_ID, so no real workspace is read or written. The folder is an
# iCloud Drive folder: the Mac reaches it by path, and the iPhone through the
# bookmark the app's document picker left (open Docket on the iPhone, Sync >
# Choose Exchange Folder…, and pick the same folder once).
#
#   ios/scripts/record-icloud-exchange.sh
#
# Environment:
#   DOCKET_IOS_DEVELOPMENT_TEAM    signing team for the device build (required)
#   DOCKET_IOS_DEVICE              iPhone UDID (default: the first physical iPhone)
#   DOCKET_ICLOUD_EXCHANGE_FOLDER  exchange folder (default: iCloud Drive/tt-sync-evidence)
#   DOCKET_EXCHANGE_TIMEOUT        seconds each side waits for the other (default 1500)
#   DOCKET_ICLOUD_EVIDENCE         output (default docs/mobile/evidence/session-8-icloud-exchange.json)

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

fail() {
	echo "record-icloud-exchange: $*" >&2
	exit 1
}

note() {
	echo "record-icloud-exchange: $*" >&2
}

[ "$(uname -s)" = Darwin ] || fail "a real iCloud exchange needs macOS"
team=${DOCKET_IOS_DEVELOPMENT_TEAM:-}
[ -n "$team" ] || fail "set DOCKET_IOS_DEVELOPMENT_TEAM to the signing team"
folder=${DOCKET_ICLOUD_EXCHANGE_FOLDER:-"$HOME/Library/Mobile Documents/com~apple~CloudDocs/tt-sync-evidence"}
case "$folder" in
*"/Library/Mobile Documents/"*) ;;
*) fail "$folder is not inside iCloud Drive" ;;
esac
timeout=${DOCKET_EXCHANGE_TIMEOUT:-1500}
evidence=${DOCKET_ICLOUD_EVIDENCE:-docs/mobile/evidence/session-8-icloud-exchange.json}
run=$(date -u +%Y%m%dT%H%M%SZ)
out=$(mktemp -d "${TMPDIR:-/tmp}/tt-icloud-exchange.XXXXXX")
note "run $run, scratch $out"

# The iPhone: physical only, never a simulator.
xcrun devicectl list devices --json-output "$out/devices.json" >/dev/null
device=${DOCKET_IOS_DEVICE:-$(python3 -I - "$out/devices.json" <<'PY'
import json, sys
for d in json.load(open(sys.argv[1]))["result"]["devices"]:
    hw = d["hardwareProperties"]
    if hw.get("reality") == "physical" and hw.get("platform") == "iOS":
        print(hw["udid"])
        break
PY
)}
[ -n "$device" ] || fail "no physical iPhone is paired"
python3 -I - "$out/devices.json" "$device" >"$out/iphone.json" <<'PY' || fail "device $device is not a physical iPhone"
import json, sys
for d in json.load(open(sys.argv[1]))["result"]["devices"]:
    hw, props = d["hardwareProperties"], d["deviceProperties"]
    if hw.get("udid") == sys.argv[2]:
        assert hw.get("reality") == "physical", "simulated"
        json.dump({"udid": hw["udid"], "name": props.get("name"), "model": hw.get("productType"),
                   "os_version": props.get("osVersionNumber"), "reality": hw["reality"]}, sys.stdout)
        sys.exit(0)
sys.exit(1)
PY

mkdir -p "$folder"

note "building the Mac side"
go build -trimpath -o "$out/bridgehost" ./mobile/bridgehost
(cd apple/DocketSync && swift build -c release --product docket-sync-helper >"$out/helper-build.log" 2>&1) ||
	fail "building docket-sync-helper failed: $out/helper-build.log"
helper=apple/DocketSync/.build/release/docket-sync-helper

note "building the signed iPhone app and its device suite"
ios/scripts/generate-project.sh >"$out/generate.log" 2>&1 || fail "project generation failed: $out/generate.log"
derived=ios/.cache/DeviceDerivedData
xcodebuild build-for-testing -project ios/DocketMobile.xcodeproj -scheme DocketMobile \
	-destination "id=$device" -derivedDataPath "$derived" -allowProvisioningUpdates \
	DEVELOPMENT_TEAM="$team" >"$out/build.log" 2>&1 || fail "device build failed: $out/build.log"
app="$derived/Build/Products/Debug-iphoneos/Docket.app"
[ -d "$app" ] || fail "no app at $app"

note "starting the Mac originator"
"$helper" contract-originate --bridgehost "$out/bridgehost" \
	--fixture internal/compat/testdata/workspaces/representative \
	--folder "$folder" --run "$run" --work "$out/mac" --out "$out/contract.json" \
	--timeout "$timeout" >"$out/helper.log" 2>&1 &
helper_pid=$!

note "running the iPhone responder (keep the iPhone unlocked)"
device_status=0
TEST_RUNNER_DOCKET_EXCHANGE_RUN_ID="$run" TEST_RUNNER_DOCKET_EXCHANGE_TIMEOUT="$timeout" \
	xcodebuild test-without-building -project ios/DocketMobile.xcodeproj -scheme DocketMobile \
	-destination "id=$device" -derivedDataPath "$derived" \
	-only-testing:DocketMobileDeviceTests/ICloudExchangeDeviceTests/testRespondsToTheMacOverICloudDrive \
	-resultBundlePath "$out/device.xcresult" >"$out/device-test.log" 2>&1 || device_status=$?
helper_status=0
wait "$helper_pid" || helper_status=$?
[ "$device_status" -eq 0 ] || { tail -40 "$out/device-test.log" >&2; fail "the iPhone responder failed ($out/device-test.log)"; }
[ "$helper_status" -eq 0 ] || { cat "$out/helper.log" >&2; fail "the Mac originator failed ($out/helper.log)"; }

xcrun xcresulttool get test-results summary --path "$out/device.xcresult" --format json >"$out/summary.json"
xcrun xcresulttool get test-results tests --path "$out/device.xcresult" --format json >"$out/tests.json"

mkdir -p "$(dirname "$evidence")"
python3 -I - "$out" "$evidence" "$run" "$folder" "$app" \
	"$(ios/scripts/icloud-exchange-fingerprint.sh)" \
	"$(ios/scripts/build-bridge.sh --fingerprint)" \
	"$(shasum -a 256 "$out/bridgehost" | cut -d' ' -f1)" \
	"$(shasum -a 256 "$helper" | cut -d' ' -f1)" \
	"$(sw_vers -productVersion)" "$(sysctl -n hw.model)" <<'PY'
import datetime, hashlib, json, os, sys

(out, evidence, run, folder, app, source_fp, bridge_fp, bridgehost_sha, helper_sha,
 mac_os, mac_model) = sys.argv[1:]
contract = json.load(open(os.path.join(out, "contract.json")))
summary = json.load(open(os.path.join(out, "summary.json")))
tests = json.load(open(os.path.join(out, "tests.json")))
iphone = json.load(open(os.path.join(out, "iphone.json")))
responder_device = json.loads(contract["responder_device"])

def digest(path):
    return "sha256:" + hashlib.sha256(open(path, "rb").read()).hexdigest()

app_executables = {}
for name in ("Docket", "Docket.debug.dylib"):
    path = os.path.join(app, name)
    if os.path.exists(path):
        app_executables[name] = digest(path)

def cases(node):
    if node.get("nodeType") == "Test Case":
        yield {"name": node.get("name"), "result": node.get("result")}
    for child in node.get("children", []):
        yield from cases(child)

ran = [case for node in tests.get("testNodes", []) for case in cases(node)]
devices = [d.get("deviceId") for d in summary.get("devicesAndConfigurations", [])
           for d in [d.get("device", {})]]

iphone.update({
    "model_identifier": responder_device["model"],
    "system_version": responder_device["system_version"],
    "app_executables": responder_device["app_executables"],
    "data_directory": responder_device["data_directory"],
    "folder": responder_device["folder"],
})
record = {
    "format": "tt-sync-icloud-exchange-evidence",
    "format_version": 1,
    "run_id": run,
    "recorded_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "builds": {
        "source_fingerprint": source_fp,
        "bridge_fingerprint": bridge_fp,
        "bridgehost_sha256": bridgehost_sha,
        "helper_sha256": helper_sha,
        "app_executables": app_executables,
    },
    "mac": {"model": mac_model, "os_version": mac_os, "data_directory": contract["mac_data_directory"]},
    "iphone": iphone,
    "exchange": {"provider": "iCloud Drive", "folder": folder, "run_folder": "runs/" + run},
    "workspace": {
        "workspace_id": contract["seed"]["workspace_id"],
        "fixture": "internal/compat/testdata/workspaces/representative",
        "seed_files": len(contract["seed"]["files"]),
    },
    "originator": contract["originator"],
    "responder": contract["responder"],
    "xcresult": {
        "result": summary.get("result"),
        "passed": summary.get("passedTests"),
        "failed": summary.get("failedTests"),
        "skipped": summary.get("skippedTests"),
        "devices": [d for d in devices if d],
        "tests": ran,
    },
}
with open(evidence, "w") as handle:
    json.dump(record, handle, indent=2, sort_keys=True)
    handle.write("\n")
PY
note "wrote $evidence"
