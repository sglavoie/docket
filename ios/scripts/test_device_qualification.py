"""Recorder regression checks without building, UI interaction, or a device.

Run: python3 -I -m unittest discover -s ios/scripts -p test_device_qualification.py
"""

import contextlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

from qualification import builds, checkpoints, core, interaction, procedures, scenarios


SCRIPT = Path(__file__).with_name("device_qualification.py").resolve()


class DeviceQualificationTests(unittest.TestCase):
    def setUp(self):
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.root = Path(scratch.name)
        self.directory = self.root / "trial"
        self.directory.mkdir()
        self.state = {
            "run_id": "trial",
            "checkpoints": [],
            "scenarios": {},
            "procedures": {},
            "daily_use": [],
        }
        self.state_path = self.directory / "trial.json"
        self.state_path.write_text(json.dumps(self.state))
        (self.root / "current").write_text("trial\n")
        self.env = {k: v for k, v in os.environ.items()
                    if not k.startswith("DOCKET_QUALIFICATION_")}
        self.env["DOCKET_QUALIFICATION_ROOT"] = str(self.root)

    def invoke(self, *args):
        return subprocess.run(
            [sys.executable, "-I", str(SCRIPT), *args],
            cwd=self.root, env=self.env, capture_output=True, text=True,
        )

    def test_isolated_cli_rejects_unknown_commands(self):
        for args in ((), ("unknown",)):
            with self.subTest(args=args):
                result = self.invoke(*args)
                self.assertEqual(result.returncode, 1)
                self.assertIn("record-device-qualification: usage:", result.stderr)
                self.assertNotIn("Traceback", result.stderr)

    def test_isolated_cli_reads_status_from_another_directory(self):
        result = self.invoke("status")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(f"trial trial in {self.directory}", result.stdout)
        for name in core.SCENARIOS + list(core.PROCEDURES):
            self.assertIn(name, result.stdout)
        self.assertIn("daily use: 0 days", result.stdout)

    def test_finish_preserves_evidence_and_honors_keep(self):
        evidence = self.root / "evidence" / "qualification.json"
        self.env["DOCKET_QUALIFICATION_EVIDENCE"] = str(evidence)
        self.env["DOCKET_QUALIFICATION_KEEP"] = "1"
        result = self.invoke("finish")
        self.assertEqual(result.returncode, 0, result.stderr)
        recorded = json.loads(evidence.read_text())
        core.parse_time(recorded.pop("finished_at"))
        self.assertEqual(recorded, self.state)
        self.assertEqual(json.loads(self.state_path.read_text()), self.state)
        self.assertTrue((self.root / "current").exists())

        del self.env["DOCKET_QUALIFICATION_KEEP"]
        result = self.invoke("finish")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.root / "current").exists())

    def test_rollout_sections_resolve_from_repository_root(self):
        self.assertEqual(Path(core.REPO), SCRIPT.parents[2])
        for heading in core.PROCEDURES.values():
            self.assertRegex(procedures.section_digest(heading), r"^sha256:[0-9a-f]{64}$")

    def test_device_discovery_accepts_both_coredevice_json_layouts(self):
        trial = core.Trial(str(self.directory))
        trial.state["iphone"] = {"udid": "PHONE_A"}
        hardware = {"udid": "PHONE_A", "platform": "iOS", "reality": "physical"}

        def list_devices(device):
            def run(args, **_kwargs):
                Path(args[args.index("--json-output") + 1]).write_text(
                    json.dumps({"result": {"devices": [device]}}))
                return subprocess.CompletedProcess(args, 0, "", "")
            return run

        for device in (
            {"hardwareProperties": hardware, "deviceProperties": {"name": "iPhone", "osVersionNumber": "27.0.1"},
             "connectionProperties": {"tunnelState": "connected"}},
            {"properties": {"hardware": hardware, "state": {"name": "iPhone"},
                            "software": {"osVersionNumber": {"stringValue": "27.0.1"}},
                            "connection": {"state": "connected"}}},
        ):
            with self.subTest(device=device), \
                 patch.object(builds, "run", side_effect=list_devices(device)), \
                 patch.object(checkpoints, "run", side_effect=list_devices(device)):
                phone = builds.device_info(None)
                self.assertEqual(phone["udid"], "PHONE_A")
                self.assertEqual(phone["name"], "iPhone")
                self.assertEqual(phone["os_version"], "27.0.1")
                self.assertTrue(checkpoints.device_reachable(trial))
                connection = device.get("properties", {}).get("connection", device.get("connectionProperties", {}))
                state_key = "state" if "properties" in device else "tunnelState"
                for state in ("disconnected", "unavailable", "connecting", ""):
                    connection[state_key] = state
                    self.assertFalse(checkpoints.device_reachable(trial), state)
                connection.clear()
                self.assertFalse(checkpoints.device_reachable(trial))

    def test_backup_procedure_uses_and_verifies_workspace_snapshot(self):
        trial = core.Trial(str(self.directory))
        trial.state["mac"] = {"data_directory": str(self.directory / "working")}
        trial.state["workspace"] = {"workspace_id": "workspace-1"}
        working = Path(trial.working)
        snapshot = self.directory / "backups" / "workspace-1" / "20261019T120000Z"
        for directory in (working, snapshot):
            (directory / "projects" / "unfiled").mkdir(parents=True)
            (directory / "projects" / "unfiled" / "tasks.md").write_text("same tasks\n")
        (snapshot / "snapshot.json").write_text("manifest\n")
        calls = []

        def tt(*args, **kwargs):
            calls.append(args)
            if args[:2] == ("workspace", "backup"):
                return subprocess.CompletedProcess(args, 0, f"Backup verified: {snapshot} (1 files)\n", "")
            return subprocess.CompletedProcess(args, 0, "Backup verified\n", "")

        with contextlib.ExitStack() as stack:
            stack.enter_context(patch.object(core.Trial, "current", return_value=trial))
            stack.enter_context(patch.object(trial, "mac_exchange", return_value=True))
            stack.enter_context(patch.object(trial, "mac_observation", return_value={
                "outstanding_up": 0, "outstanding_down": 0, "local_transactions": 7, "held": 0,
            }))
            stack.enter_context(patch.object(trial, "tt", side_effect=tt))
            stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
            procedures.cmd_procedure(["backup"])

        self.assertEqual(calls, [
            ("workspace", "backup", str(self.directory / "backups"), "--data-dir", str(working)),
            ("workspace", "verify-backup", os.path.realpath(snapshot)),
        ])
        checks = json.loads(self.state_path.read_text())["procedures"]["backup"]["checks"]
        self.assertTrue(checks["verified_after_creation"])
        self.assertTrue(checks["matches_working_copy"])

    def test_backup_refuses_outstanding_or_held_sync_work(self):
        trial = core.Trial(str(self.directory))
        for pending in ("outstanding_up", "outstanding_down", "held"):
            observation = {"outstanding_up": 0, "outstanding_down": 0,
                           "local_transactions": 7, "held": 0}
            observation[pending] = 1
            with self.subTest(pending=pending), contextlib.ExitStack() as stack:
                stack.enter_context(patch.object(core.Trial, "current", return_value=trial))
                stack.enter_context(patch.object(trial, "mac_exchange", return_value=True))
                stack.enter_context(patch.object(trial, "mac_observation", return_value=observation))
                tt = stack.enter_context(patch.object(trial, "tt"))
                stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
                with self.assertRaises(SystemExit):
                    procedures.cmd_procedure(["backup"])
                tt.assert_not_called()
                self.assertNotIn("backup", json.loads(self.state_path.read_text())["procedures"])

    def test_restore_snapshot_preserves_nested_user_file_named_snapshot_json(self):
        snapshot = self.directory / "snapshot"
        nested = snapshot / "projects" / "unfiled"
        nested.mkdir(parents=True)
        (snapshot / "snapshot.json").write_text("backup manifest\n")
        (nested / "snapshot.json").write_text("user content\n")
        restored = self.directory / "restored"
        procedures.restore_snapshot_contents(os.path.realpath(snapshot), str(restored))
        self.assertFalse((restored / "snapshot.json").exists())
        self.assertEqual((restored / "projects" / "unfiled" / "snapshot.json").read_text(), "user content\n")

    def test_resumed_scenario_records_attempts_and_supersedes_evidence(self):
        trial = core.Trial(str(self.directory))
        trial.attempts = {}
        trial.mac_attempts = {}
        old = {"scenario": "delayed_delivery", "label": "previous"}
        trial.state["checkpoints"] = [old]
        record = {"scenario": "delayed_delivery", "label": "delivered"}

        def runner(current):
            self.assertTrue(scenarios.RESUME_AFTER_IPHONE)
            self.assertTrue(scenarios.RESUME_INSTALLED_OLD)
            self.assertEqual(current.marker("delayed_delivery", "iphone-1"),
                             "q18 trial delayed-delivery iphone-1 a3")
            self.assertEqual(current.marker("delayed_delivery", "mac-1"),
                             "q18 trial delayed-delivery mac-1 a2")
            return [record], {}

        with contextlib.ExitStack() as stack:
            stack.enter_context(patch.object(core.Trial, "current", return_value=trial))
            stack.enter_context(patch.dict(scenarios.SCENARIO_RUNNERS, delayed_delivery=runner))
            stack.enter_context(patch.object(scenarios, "human", return_value="none"))
            stack.enter_context(patch.object(scenarios, "RESUME_AFTER_IPHONE", False))
            stack.enter_context(patch.object(scenarios, "RESUME_INSTALLED_OLD", False))
            stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
            scenarios.cmd_scenario([
                "delayed_delivery", "--attempt=3", "--mac-attempt=2",
                "--resume-after-iphone", "--resume-installed-old",
            ])

        saved = json.loads(self.state_path.read_text())
        self.assertEqual(saved["checkpoints"], [record])
        self.assertEqual(saved["superseded"][0]["checkpoints"], [old])
        result = saved["scenarios"]["delayed_delivery"]
        self.assertEqual(result["observations"], {"attempt": 3, "mac_attempt": 2})
        self.assertEqual(result["remarks"], "")

    def test_airplane_waits_for_offline_confirmation_before_mac_capture(self):
        trial = core.Trial(str(self.directory))
        trial.attempts = {}
        trial.mac_attempts = {}
        events = []
        with contextlib.ExitStack() as stack:
            stack.enter_context(patch.object(scenarios, "say"))
            stack.enter_context(patch.object(scenarios, "wait_enter", side_effect=lambda: events.append("confirmed")))
            stack.enter_context(patch.object(trial, "marker_counts", return_value={trial.marker("airplane_mode", "mac-1"): 0}))
            stack.enter_context(patch.object(scenarios, "mac_capture", side_effect=lambda *_: events.append("captured")))
            stack.enter_context(patch.object(scenarios, "checkpoint", side_effect=lambda *_: events.append("checkpoint")))
            scenarios.scenario_airplane(trial)
        self.assertEqual(events, ["confirmed", "captured", "checkpoint", "confirmed", "checkpoint"])

    def test_delayed_resume_clears_shared_mailbox_prompt(self):
        trial = core.Trial(str(self.directory))
        trial.attempts = {}
        trial.mac_attempts = {}
        interaction._pending.clear()
        self.addCleanup(interaction._pending.clear)
        with contextlib.ExitStack() as stack:
            stack.enter_context(patch.object(interaction, "MAILBOX", "plan"))
            stack.enter_context(patch.object(scenarios, "RESUME_AFTER_IPHONE", True))
            stack.enter_context(patch.object(scenarios, "DELAY_MIN_SECONDS", 0))
            stack.enter_context(patch.object(scenarios, "mac_capture"))
            stack.enter_context(patch.object(trial, "mac_exchange", return_value=True))
            stack.enter_context(patch.object(scenarios, "checkpoint", return_value={}))
            wait = stack.enter_context(patch.object(scenarios, "wait_enter"))
            stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
            stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
            scenarios.scenario_delayed(trial)
        wait.assert_called_once_with()
        self.assertEqual(interaction._pending,
                         ["Turn Airplane Mode off on the iPhone and open tt."])


if __name__ == "__main__":
    unittest.main()
