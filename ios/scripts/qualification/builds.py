"""Physical device discovery, builds, signing, and installation records."""

import json
import os
import plistlib
import re
import tempfile
from .core import (
    Trial,
    fail,
    note,
    run,
    sha256_file,
    stamp,
)


def device_info(udid):
    with tempfile.NamedTemporaryFile(suffix=".json") as out:
        run(["xcrun", "devicectl", "list", "devices", "--json-output", out.name])
        with open(out.name) as handle:
            devices = json.load(handle)["result"]["devices"]
    for device in devices:
        properties = device.get("properties", {})
        hw = properties.get("hardware", device.get("hardwareProperties", {}))
        legacy = device.get("deviceProperties", {})
        state = properties.get("state", legacy)
        version = properties.get("software", {}).get("osVersionNumber", legacy.get("osVersionNumber"))
        if isinstance(version, dict):
            version = version.get("stringValue")
        if hw.get("platform") != "iOS" or hw.get("reality") != "physical":
            continue
        if udid and hw.get("udid") != udid:
            continue
        return {"udid": hw["udid"], "name": state.get("name"), "model": hw.get("productType"),
                "os_version": version, "reality": hw["reality"]}
    fail("no physical iPhone is paired" + (f" with UDID {udid}" if udid else ""))


def build_mac(source, out):
    run(["go", "build", "-trimpath", "-buildvcs=false", "-o", out, "."], cwd=source)
    return sha256_file(out).removeprefix("sha256:")


def build_app(source, udid, team, derived, log):
    run(["ios/scripts/generate-project.sh"], cwd=source, log=log)
    run(["xcodebuild", "build", "-project", "ios/TTMobile.xcodeproj", "-scheme", "TTMobile",
         "-destination", f"id={udid}", "-derivedDataPath", derived, "-allowProvisioningUpdates",
         f"DEVELOPMENT_TEAM={team}"], cwd=source, log=log)
    app = os.path.join(source, derived, "Build", "Products", "Debug-iphoneos", "tt.app")
    if not os.path.isdir(app):
        fail(f"no app at {app}")
    return app, app_executables(app)


def app_executables(app):
    return {name: sha256_file(os.path.join(app, name))
            for name in ("tt", "tt.debug.dylib") if os.path.exists(os.path.join(app, name))}


def signing(app):
    details = run(["codesign", "-dvvv", app]).stderr
    authorities = re.findall(r"^Authority=(.+)$", details, re.M)
    team = (re.findall(r"^TeamIdentifier=(.+)$", details, re.M) or [""])[0]
    entitlements = plistlib.loads(run(["codesign", "-d", "--entitlements", "-", "--xml", app]).stdout.encode())
    run(["codesign", "--verify", "--deep", "--strict", app])
    return {"team_identifier": team, "authorities": authorities, "verified": True,
            "application_groups": entitlements.get("com.apple.security.application-groups", []),
            "application_identifier": entitlements.get("application-identifier", "")}


def current_install(trial):
    """The latest install the trial recorded; a trial from before installs
    were recorded falls back to its starting build."""
    installs = trial.state["builds"].get("installs") or []
    if installs:
        return installs[-1]
    return {"app_executables": trial.state["builds"]["app_executables"]}


def record_install(trial, executables, reason, at=None):
    """Records one install of the trial's own app. A rebuild re-signs the app,
    so its executable digests change from install to install; what binds each
    install to the trial is the source and bridge fingerprints it was built
    from, which must still be the trial's."""
    source = run(["ios/scripts/qualification-fingerprint.sh"]).stdout.strip()
    bridge = run(["ios/scripts/build-bridge.sh", "--fingerprint"]).stdout.strip()
    builds = trial.state["builds"]
    if source != builds["source_fingerprint"] or bridge != builds["bridge_fingerprint"]:
        fail(f"the tree changed since the trial started (sources {source}, bridge {bridge}): this install would not be the trial's app")
    builds.setdefault("installs", []).append({"at": at or stamp(), "reason": reason, "app_executables": executables,
                                              "source_fingerprint": source, "bridge_fingerprint": bridge})
    trial.save()


def install_current(trial, reason):
    """Builds, signs, and installs the trial's app again through the rollout
    guide's script, and records the install."""
    log = os.path.join(trial.dir, "build.log")
    app = run(["ios/scripts/install-device.sh"], env={"DOCKET_IOS_DEVICE": trial.state["iphone"]["udid"]}, log=log).stdout.strip()
    record_install(trial, app_executables(app), reason)
    note(f"installed the trial's app again ({reason})")


def cmd_reinstall(args):
    """reinstall REASON: put the trial's own app back on the iPhone, for
    example after another recorder installed a different build."""
    install_current(Trial.current(), " ".join(args) or "reinstall")


def install(trial, app):
    run(["xcrun", "devicectl", "device", "install", "app", "--device", trial.state["iphone"]["udid"], app],
        log=os.path.join(trial.dir, "devicectl.log"))
