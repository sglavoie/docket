"""Runner integrity checks; no simulator, UI or network required."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import simulator_qualification as q


class QualificationRunnerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

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
