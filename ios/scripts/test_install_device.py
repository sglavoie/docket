"""Exercise deployment failure boundaries without building or touching a phone.

Run with: python3 -I -m unittest discover -s ios/scripts -p test_install_device.py
"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


class InstallDeviceTests(unittest.TestCase):
    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory()
        self.addCleanup(self.scratch.cleanup)
        self.root = Path(self.scratch.name) / "repo with spaces"
        scripts = self.root / "ios/scripts"
        scripts.mkdir(parents=True)
        shutil.copy(Path(__file__).with_name("install-device.sh"), scripts)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        stub = self.bin / "stub"
        stub.write_text(f"#!{sys.executable}\n" + r'''
import json, os, pathlib, sys
tool, args = pathlib.Path(sys.argv[0]).name, sys.argv[1:]
with open(os.environ["TRACE"], "a") as trace:
    trace.write(json.dumps([tool] + args) + "\n")
def value(flag):
    return args[args.index(flag) + 1]
stage = tool
if tool == "uname":
    print("Darwin")
elif tool == "codesign":
    stage = "verify" if "--verify" in args else "signing-team"
    if stage == "signing-team":
        print("TeamIdentifier=PREVIOUS_TEAM", file=sys.stderr)
elif tool == "xcrun":
    if args[:3] == ["devicectl", "list", "devices"]:
        stage = "devices"
        pathlib.Path(value("--json-output")).write_text(os.environ["DEVICES"])
    elif args[:4] == ["devicectl", "device", "install", "app"]:
        stage = "install"
    elif args[:4] == ["devicectl", "device", "process", "launch"]:
        stage = "launch"
    else:
        sys.exit("unexpected device command")
elif tool == "xcodebuild":
    stage = "build"
    (pathlib.Path(value("-derivedDataPath")) / "Build/Products/Debug-iphoneos/Docket.app").mkdir(parents=True, exist_ok=True)
if os.environ.get("FAIL_AT") == stage:
    sys.exit("injected " + stage + " failure")
''')
        stub.chmod(0o755)
        for name in ["uname", "codesign", "xcrun", "xcodebuild", "generate"]:
            (self.bin / name).symlink_to(stub)
        generation = scripts / "generate-project.sh"
        generation.write_text("#!/bin/sh\nexec generate\n")
        generation.chmod(0o755)
        self.trace = self.root / "trace.jsonl"
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("DOCKET_IOS_")}
        self.env.update(
            PATH=str(self.bin) + os.pathsep + os.environ["PATH"],
            TRACE=str(self.trace),
            DOCKET_IOS_DEVELOPMENT_TEAM="EXPLICIT_TEAM",
        )
        self.devices("PHONE_A")

    def devices(self, *ids, modern=True):
        key = "properties" if modern else "hardwareProperties"
        devices = []
        for udid in ids:
            hardware = dict(udid=udid, reality="physical", platform="iOS", deviceType="iPhone")
            devices.append({key: {"hardware": hardware} if modern else hardware})
        self.env["DEVICES"] = json.dumps({"result": {"devices": devices}})

    def run_install(self, launch=True):
        return subprocess.run(
            ["bash", str(self.root / "ios/scripts/install-device.sh")] + (["--launch"] if launch else []),
            cwd=self.root, env=self.env, text=True, capture_output=True, check=False,
        )

    def calls(self):
        return [json.loads(line) for line in self.trace.read_text().splitlines()]

    def test_upgrade_builds_verifies_installs_and_launches_in_order(self):
        result = self.run_install()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        self.assertEqual([call[0] for call in calls], ["uname", "xcrun", "generate", "xcodebuild", "codesign", "xcrun", "xcrun"])
        self.assertIn("DEVELOPMENT_TEAM=EXPLICIT_TEAM", calls[3])
        self.assertIn("--verify", calls[4])
        self.assertEqual(calls[5][1:5], ["devicectl", "device", "install", "app"])
        self.assertIn("--terminate-existing", calls[6])
        self.assertTrue(Path(result.stdout.strip()).is_dir())
        self.assertEqual(len(result.stdout.splitlines()), 1)

    def test_reuses_previous_team_and_accepts_absolute_derived_path(self):
        self.env.pop("DOCKET_IOS_DEVELOPMENT_TEAM")
        derived = self.root / "custom derived"
        self.env["DOCKET_IOS_DEVICE_DERIVED"] = str(derived)
        app = derived / "Build/Products/Debug-iphoneos/Docket.app"
        app.mkdir(parents=True)
        result = self.run_install(launch=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), str(app))
        self.assertTrue(any("DEVELOPMENT_TEAM=PREVIOUS_TEAM" in c for c in self.calls()))
        self.assertFalse(any("launch" in c for c in self.calls()))

    def test_first_install_explains_required_team(self):
        self.env.pop("DOCKET_IOS_DEVELOPMENT_TEAM")
        result = self.run_install()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("just ios-install TEAMID", result.stderr)
        self.assertFalse(any(c[0] == "xcodebuild" for c in self.calls()))

    @unittest.skipUnless(shutil.which("just"), "just is needed to exercise the recipe")
    def test_just_recipe_forwards_team_and_phone(self):
        shutil.copy(Path(__file__).resolve().parents[2] / "justfile", self.root)
        self.devices("PHONE_A", "PHONE_B")
        result = subprocess.run(
            ["just", "ios-install", "RECIPE_TEAM", "PHONE_B"],
            cwd=self.root, env=self.env, text=True, capture_output=True, check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any("DEVELOPMENT_TEAM=RECIPE_TEAM" in c for c in self.calls()))
        self.assertTrue(any("id=PHONE_B" in c for c in self.calls()))
        self.assertTrue(any("launch" in c for c in self.calls()))

    def test_ambiguous_or_missing_phone_never_builds(self):
        for phones in [(), ("PHONE_A", "PHONE_B")]:
            with self.subTest(phones=phones):
                self.devices(*phones)
                result = self.run_install()
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(c[0] == "xcodebuild" for c in self.calls()))

    def test_explicit_phone_and_legacy_device_schema(self):
        self.devices("PHONE_A", "PHONE_B", modern=False)
        self.env["DOCKET_IOS_DEVICE"] = "PHONE_B"
        result = self.run_install()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any("id=PHONE_B" in c for c in self.calls()))
        for call in self.calls():
            if "--device" in call:
                self.assertEqual(call[call.index("--device") + 1], "PHONE_B")

    def test_failures_never_continue_to_install_or_launch(self):
        for stage in ["generate", "build", "verify", "install", "launch"]:
            with self.subTest(stage=stage):
                self.trace.unlink(missing_ok=True)
                self.env["FAIL_AT"] = stage
                result = self.run_install()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("full log:", result.stderr)
                calls = self.calls()
                if stage in ["generate", "build", "verify"]:
                    self.assertFalse(any("install" in c for c in calls))
                if stage != "launch":
                    self.assertFalse(any("launch" in c for c in calls))
                else:
                    self.assertIn("the update is installed, but launch failed", result.stderr)


if __name__ == "__main__":
    unittest.main()
