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

from qualification import core, interaction, procedures, scenarios


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
                    if not k.startswith("TT_QUALIFICATION_")}
        self.env["TT_QUALIFICATION_ROOT"] = str(self.root)

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
        self.env["TT_QUALIFICATION_EVIDENCE"] = str(evidence)
        self.env["TT_QUALIFICATION_KEEP"] = "1"
        result = self.invoke("finish")
        self.assertEqual(result.returncode, 0, result.stderr)
        recorded = json.loads(evidence.read_text())
        core.parse_time(recorded.pop("finished_at"))
        self.assertEqual(recorded, self.state)
        self.assertEqual(json.loads(self.state_path.read_text()), self.state)
        self.assertTrue((self.root / "current").exists())

        del self.env["TT_QUALIFICATION_KEEP"]
        result = self.invoke("finish")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.root / "current").exists())

    def test_rollout_sections_resolve_from_repository_root(self):
        self.assertEqual(Path(core.REPO), SCRIPT.parents[2])
        for heading in core.PROCEDURES.values():
            self.assertRegex(procedures.section_digest(heading), r"^sha256:[0-9a-f]{64}$")

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
