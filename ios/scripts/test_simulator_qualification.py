"""Runner integrity checks; no simulator, UI or network required."""
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import simulator_qualification as q


class QualificationRunnerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def test_baseline_wrapper_adapts_legacy_names_without_rewriting_the_shipped_app(self):
        repo = self.root / "repo"
        harness = repo / "ios/DocketMobileUpgradeUITests"
        harness.mkdir(parents=True)
        (harness / "Upgrade.swift").write_text('let key = "DOCKET_UITEST_ROOT"\n')
        (repo / "ios/project.yml").write_text("  DocketMobileUpgradeUITests:\n    TEST_TARGET_NAME: DocketMobile\n")
        scripts = repo / "ios/scripts"
        scripts.mkdir()
        (scripts / "test-ui.sh").write_text("DocketMobile.xcodeproj DOCKET_IOS_CACHE DocketMobileUpgradeUITests\n")
        production = b'let literal = "Docket DOCKET docket must stay unchanged"\n'
        for mobile in ("TTMobile", "DocketMobile"):
            with self.subTest(mobile=mobile):
                archive = io.BytesIO()
                with tarfile.open(fileobj=archive, mode="w") as tar:
                    files = {
                        "ios/project.yml": f"name: {mobile}\n        - {mobile}UITests\n".encode(),
                        f"ios/{mobile}/App.swift": production,
                        "ios/scripts/test-ui.sh": b"#!/bin/sh\n",
                    }
                    for name, data in files.items():
                        member = tarfile.TarInfo(name)
                        member.size = len(data)
                        member.mode = 0o755 if name.endswith(".sh") else 0o644
                        tar.addfile(member, io.BytesIO(data))

                def archived(*args, **kwargs):
                    kwargs["stdout"].write(archive.getvalue())

                root = self.root / mobile
                root.mkdir()
                with patch.object(q, "REPO", repo), patch.object(q.subprocess, "run", side_effect=archived):
                    baseline = q.baseline_source(root)
                self.assertEqual((baseline / f"ios/{mobile}/App.swift").read_bytes(), production)
                spec = (baseline / "ios/project.yml").read_text()
                self.assertIn(f"        - {mobile}UpgradeUITests\n", spec)
                self.assertIn(f"TEST_TARGET_NAME: {mobile}", spec)
                copied = baseline / f"ios/{mobile}UpgradeUITests/Upgrade.swift"
                self.assertIn('"TT_UITEST_ROOT"' if mobile == "TTMobile" else '"DOCKET_UITEST_ROOT"', copied.read_text())
                self.assertIn(mobile + ".xcodeproj", (baseline / "ios/scripts/test-ui.sh").read_text())
                self.assertTrue((baseline / "ios/scripts/test-ui.sh").stat().st_mode & 0o111)

    def test_upgrade_requires_a_successful_desktop_migration_before_installing_current_app(self):
        for event in (
            {"Time": "now", "Action": "pass", "Test": "TestCrossMachineHelperOpensWorkspace"},
            {"Time": "now", "Action": "skip", "Test": "TestCrossMachineHelperOpensWorkspace"},
            {"Time": "now", "Action": "pass"},
        ):
            with self.subTest(event=event):
                calls = []

                def invoke(command, cwd, env, logfile, timeout=5400):
                    calls.append(command)
                    if command[0] == "go":
                        self.assertEqual(env["DOCKET_HELPER_DATA_DIR"], str((self.root / "data/workspaces/upgrade").resolve()))
                        logfile.write_text(json.dumps(event) + "\n")
                        return
                    raise RuntimeError("reached current app installation")

                state = {"upgrade_root": str(self.root / "data")}
                with patch.object(q, "invoke", side_effect=invoke):
                    if event.get("Test") and event["Action"] == "pass":
                        with self.assertRaisesRegex(RuntimeError, "reached current app installation"):
                            q.run_stage("upgrade-after", self.root, state, self.root, {})
                        self.assertEqual(len(calls), 2)
                    else:
                        with self.assertRaises(AssertionError):
                            q.run_stage("upgrade-after", self.root, state, self.root, {})
                        self.assertEqual(len(calls), 1)

    def test_upgrade_refuses_migration_outside_its_disposable_data(self):
        outside = self.root / "other-workspace"
        directory = self.root / "qualification"
        directory.mkdir()
        state = {"upgrade_root": str(outside)}
        with patch.object(q, "invoke") as invoke, self.assertRaisesRegex(AssertionError, "disposable qualification data"):
            q.run_stage("upgrade-after", directory, state, directory, {})
        invoke.assert_not_called()

    def test_summary_refuses_empty_skipped_and_partial_runs(self):
        path = self.root / "summary.json"
        good = dict(result="Passed", totalTestCount=4, passedTests=4, failedTests=0, skippedTests=0, expectedFailures=0)
        q.write_json(path, good)
        q.checked_summary(path, "xctest")
        for changes in (dict(totalTestCount=0, passedTests=0), dict(skippedTests=1), dict(passedTests=3), dict(failedTests=1), dict(expectedFailures=1)):
            q.write_json(path, {**good, **changes})
            with self.subTest(changes=changes), self.assertRaises(AssertionError):
                q.checked_summary(path, "xctest")

    def test_summary_requires_every_swift_suite(self):
        path = self.root / "summary.json"
        q.write_json(path, {"suites": [{"suite": "sync", "status": "passed", "executed": 1, "failed": 0, "skipped": 0}]})
        with self.assertRaises(AssertionError):
            q.checked_summary(path, "swift")

    def test_resume_does_not_trust_missing_or_changed_artifacts(self):
        path = self.root / "result"
        path.write_text("passed")
        record = {"artifacts": {"result": q.digest(path)}}
        self.assertTrue(q.artifacts_valid(record, self.root))
        path.write_text("modified")
        self.assertFalse(q.artifacts_valid(record, self.root))
        path.unlink()
        self.assertFalse(q.artifacts_valid(record, self.root))
        self.assertFalse(q.artifacts_valid({}, self.root))

    def test_lock_refuses_a_second_worker(self):
        with q.lock(self.root / "lock"):
            with self.assertRaises(RuntimeError):
                q.lock(self.root / "lock")

    def test_resume_preserves_passes_and_retries_interrupted_stage(self):
        calls = []
        def stage(name, directory, state, root, env):
            calls.append(name)
            if name == "ui" and calls.count("ui") == 1:
                raise RuntimeError("simulated interruption")
            artifact = directory / "result"
            artifact.write_text("passed")
            return {str(artifact.relative_to(root)): q.digest(artifact)}
        with patch.object(q, "STAGES", ("swift", "ui")), patch.object(q, "fingerprint", return_value="source"), \
             patch.object(q, "output", return_value="tool"), patch.object(q, "run_stage", side_effect=stage):
            with self.assertRaises(RuntimeError):
                q.run(self.root)
            self.assertEqual(json.loads((self.root / "qualification.json").read_text())["status"], "failed")
            q.run(self.root)
            self.assertEqual(calls, ["swift", "ui", "ui"])
            self.assertEqual(json.loads((self.root / "qualification.json").read_text())["status"], "passed")
            with patch.object(q, "fingerprint", return_value="changed"):
                with self.assertRaises(AssertionError):
                    q.run(self.root)

    def test_changed_upgrade_seed_invalidates_both_upgrade_results(self):
        calls = []
        def stage(name, directory, state, root, env):
            calls.append(name)
            artifact = directory / "result"
            artifact.write_text("passed")
            return {str(artifact.relative_to(root)): q.digest(artifact)}
        with patch.object(q, "STAGES", ("upgrade-before", "upgrade-after")), \
             patch.object(q, "fingerprint", return_value="source"), patch.object(q, "output", return_value="tool"), \
             patch.object(q, "run_stage", side_effect=stage):
            q.run(self.root)
            (self.root / "upgrade-before/attempt-1/result").write_text("changed")
            q.run(self.root)
        self.assertEqual(calls, ["upgrade-before", "upgrade-after", "upgrade-before", "upgrade-after"])


if __name__ == "__main__":
    unittest.main()
