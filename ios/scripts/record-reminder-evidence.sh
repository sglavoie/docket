#!/usr/bin/env bash
#
# Record real reminder delivery and notification actions on a physical
# iPhone, and write them as evidence the session-15 verifier validates.
#
# Builds the signed app and its UI tests for the iPhone, then runs
# ReminderDeliveryUITests there: each test seeds a disposable workspace inside
# the app's own container (no workspace already on the iPhone is read or
# written), lets the system deliver a real local notification while tt is in
# the background, and acts on it through SpringBoard — Acknowledge on a
# check-in, Defer 1 Hour on a deferral, and a tap that opens the task. The
# running app must report the executable digests of the build this script
# installed, and the evidence names those digests, the Swift source
# fingerprint, and the bridge fingerprint, so it is tied to the tested build.
#
#   ios/scripts/record-reminder-evidence.sh
#
# Keep the iPhone unlocked, awake, and on the home screen during the run; the
# first test answers the notification permission prompt if it appears. UI
# Automation must be enabled in Settings > Developer.
#
# Environment:
#   TT_IOS_DEVELOPMENT_TEAM   signing team for the device build (required)
#   TT_IOS_DEVICE             iPhone UDID (default: the first physical iPhone)
#   TT_REMINDER_EVIDENCE      output (default docs/mobile/evidence/session-15-reminders.json)

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

fail() {
	echo "record-reminder-evidence: $*" >&2
	exit 1
}

note() {
	echo "record-reminder-evidence: $*" >&2
}

[ "$(uname -s)" = Darwin ] || fail "a physical iPhone run needs macOS"
team=${TT_IOS_DEVELOPMENT_TEAM:-}
[ -n "$team" ] || fail "set TT_IOS_DEVELOPMENT_TEAM to the signing team"
evidence=${TT_REMINDER_EVIDENCE:-docs/mobile/evidence/session-15-reminders.json}
run=$(date -u +%Y%m%dT%H%M%SZ)
out=$(mktemp -d "${TMPDIR:-/tmp}/tt-reminder-evidence.XXXXXX")
note "run $run, scratch $out"

# The iPhone: physical only, never a simulator.
xcrun devicectl list devices --json-output "$out/devices.json" >/dev/null
device=${TT_IOS_DEVICE:-$(python3 -I - "$out/devices.json" <<'PY'
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
        json.dump({"udid": hw["udid"], "name": props.get("name"), "model": hw.get("marketingName") or hw.get("productType"),
                   "model_identifier": hw.get("productType"), "os_version": props.get("osVersionNumber"),
                   "reality": hw["reality"]}, sys.stdout)
        sys.exit(0)
sys.exit(1)
PY

source_fp=$(ios/scripts/reminder-evidence-fingerprint.sh)
bridge_fp=$(ios/scripts/build-bridge.sh --fingerprint)

note "building the signed app and its UI tests for the iPhone"
ios/scripts/generate-project.sh >"$out/generate.log" 2>&1 || fail "project generation failed: $out/generate.log"
derived=ios/.cache/DeviceDerivedData
xcodebuild build-for-testing -project ios/TTMobile.xcodeproj -scheme TTMobile \
	-destination "id=$device" -derivedDataPath "$derived" -allowProvisioningUpdates \
	DEVELOPMENT_TEAM="$team" >"$out/build.log" 2>&1 || fail "device build failed: $out/build.log"
app="$derived/Build/Products/Debug-iphoneos/tt.app"
[ -d "$app" ] || fail "no app at $app"
[ "$(ios/scripts/reminder-evidence-fingerprint.sh)" = "$source_fp" ] || fail "the Swift sources changed during the build"

# The digests the running app must report, in the app's own spelling.
executables=$(python3 -I - "$app" <<'PY'
import hashlib, os, sys
parts = []
for name in ("tt", "tt.debug.dylib"):
    path = os.path.join(sys.argv[1], name)
    if os.path.exists(path):
        parts.append(name + "=sha256:" + hashlib.sha256(open(path, "rb").read()).hexdigest())
print(";".join(parts))
PY
)
[ -n "$executables" ] || fail "the app has no executable"

note "running ReminderDeliveryUITests on the iPhone (keep it unlocked, about five minutes)"
test_status=0
TEST_RUNNER_TT_EXPECTED_APP_EXECUTABLES="$executables" \
	xcodebuild test-without-building -project ios/TTMobile.xcodeproj -scheme TTMobile \
	-destination "id=$device" -derivedDataPath "$derived" \
	-only-testing:TTMobileUITests/ReminderDeliveryUITests \
	-resultBundlePath "$out/device.xcresult" >"$out/device-test.log" 2>&1 || test_status=$?
[ "$test_status" -eq 0 ] || { grep -E "error:|Test Case" "$out/device-test.log" | tail -40 >&2; fail "the iPhone run failed ($out/device-test.log)"; }

xcrun xcresulttool get test-results summary --path "$out/device.xcresult" --format json >"$out/summary.json"
xcrun xcresulttool get test-results tests --path "$out/device.xcresult" --format json >"$out/tests.json"

mkdir -p "$(dirname "$evidence")"
python3 -I - "$out" "$evidence" "$run" "$source_fp" "$bridge_fp" "$executables" \
	"$(sw_vers -productVersion)" "$(sysctl -n hw.model)" "$(xcodebuild -version | tr '\n' ' ')" <<'PY'
import datetime, json, os, sys

out, evidence, run, source_fp, bridge_fp, executables, mac_os, mac_model, xcode = sys.argv[1:]
summary = json.load(open(os.path.join(out, "summary.json")))
tests = json.load(open(os.path.join(out, "tests.json")))
iphone = json.load(open(os.path.join(out, "iphone.json")))

def cases(node):
    if node.get("nodeType") == "Test Case":
        yield {"name": node.get("name"), "result": node.get("result"),
               "duration": node.get("durationInSeconds") or node.get("duration")}
    for child in node.get("children", []):
        yield from cases(child)

ran = [case for node in tests.get("testNodes", []) for case in cases(node)]
devices = [d.get("device", {}).get("deviceId") for d in summary.get("devicesAndConfigurations", [])]
record = {
    "format": "tt-mobile-reminder-evidence",
    "format_version": 1,
    "run_id": run,
    "recorded_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "builds": {
        "source_fingerprint": source_fp,
        "bridge_fingerprint": bridge_fp,
        "app_executables": executables,
        "xcode": xcode.strip(),
    },
    "mac": {"model": mac_model, "os_version": mac_os},
    "iphone": iphone,
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
