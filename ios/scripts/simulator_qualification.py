#!/usr/bin/env python3
"""Unattended simulator release checks, with durable per-stage evidence.

run/start/resume OUTPUT; status OUTPUT. start detaches from SSH. resume reuses
only verified passing stages from exactly the same sources and toolchain.
"""
import argparse
import contextlib
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tarfile
import tempfile

REPO = Path(__file__).resolve().parents[2]
BASELINE = "57032ffc32a6ce813fad3c21af5070cfae5484e8"
OLD_SCHEMA = "08c0cb08fec10154e6d2c9455e32d4c4ffa5f72d"
STAGES = ("swift", "ui", "upgrade-before", "upgrade-after", "compatibility", "backup")
FORMAT = "tt-simulator-qualification/v1"


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def digest(path):
    h = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    temporary.replace(path)


def output(*args, cwd=REPO):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def fingerprint():
    names = subprocess.check_output(["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"], cwd=REPO).split(b"\0")
    h = hashlib.sha256()
    for name in sorted(set(names)):
        if not name:
            continue
        path = REPO / os.fsdecode(name)
        h.update(name + b"\0")
        h.update((digest(path) if path.is_file() else "missing").encode() + b"\0")
    return h.hexdigest()


def checked_summary(path, kind):
    data = json.loads(path.read_text())
    if kind == "swift":
        suites = data.get("suites", [])
        assert {s["suite"] for s in suites} == {"unit-shared", "unit-reminders", "bridge", "app", "sync"}, "incomplete Swift suites"
        for suite in suites:
            assert suite["status"] == "passed" and suite["executed"] > 0 and suite["failed"] == suite["skipped"] == 0, suite
    else:
        assert data.get("result") == "Passed", "XCTest did not pass"
        assert data.get("totalTestCount", 0) > 0, "no tests executed"
        assert data.get("passedTests") == data["totalTestCount"], "not every test passed"
        assert data.get("failedTests") == data.get("skippedTests") == data.get("expectedFailures", 0) == 0, "failed/skipped/expected-failure tests"
    return data


def artifacts_valid(record, root):
    artifacts = record.get("artifacts", {})
    return bool(artifacts) and all((root / name).is_file() and digest(root / name) == sha for name, sha in artifacts.items())


def lock(path):
    handle = open(path, "a+")
    try:
        fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        handle.close()
        raise RuntimeError(f"another qualification worker holds {path}")
    return handle


@contextlib.contextmanager
def keep_awake():
    process = None
    if shutil.which("caffeinate"):
        process = subprocess.Popen(["caffeinate", "-i", "-w", str(os.getpid())],
                                   stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        yield
    finally:
        if process:
            process.terminate()
            process.wait()


def invoke(command, cwd, env, logfile, timeout=5400):
    with open(logfile, "w") as log:
        log.write(json.dumps({"command": command, "cwd": str(cwd), "started_at": now()}) + "\n")
        log.flush()
        process = subprocess.Popen(command, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                   stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            result = process.wait(timeout=timeout)
        except BaseException:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            raise
        if result:
            raise RuntimeError(f"command exited {result}; see {logfile}")


def baseline_source(root):
    destination = root / "baseline-source"
    if (destination / ".qualification-ready").exists():
        return destination
    if destination.exists():
        shutil.rmtree(destination)
    destination.mkdir()
    archive = root / "baseline.tar"
    with open(archive, "wb") as handle:
        subprocess.run(["git", "archive", BASELINE], cwd=REPO, stdout=handle, check=True)
    with tarfile.open(archive) as tar:
        # Archive is a fixed, repository-owned commit; no user-supplied tar.
        if sys.version_info >= (3, 12):
            tar.extractall(destination, filter="data")
        else:
            tar.extractall(destination)
    archive.unlink()
    # Add only the current upgrade TEST harness and its target. All app/engine
    # sources remain exactly those of BASELINE, including the old UI behavior.
    shutil.copytree(REPO / "ios/TTMobileUpgradeUITests", destination / "ios/TTMobileUpgradeUITests")
    spec = destination / "ios/project.yml"
    text = spec.read_text()
    assert "        - TTMobileUITests\n" in text
    text = text.replace("        - TTMobileUITests\n", "        - TTMobileUITests\n        - TTMobileUpgradeUITests\n", 1)
    current = (REPO / "ios/project.yml").read_text()
    text += "\n  TTMobileUpgradeUITests:\n" + current.split("  TTMobileUpgradeUITests:\n", 1)[1]
    spec.write_text(text)
    # The script only selects the test bundle and exports XCTest artifacts.
    shutil.copy2(REPO / "ios/scripts/test-ui.sh", destination / "ios/scripts/test-ui.sh")
    (destination / ".qualification-ready").write_text(BASELINE + "\n")
    return destination


def run_stage(name, directory, state, root, env):
    cwd = REPO
    artifacts = []
    log = directory / "stage.log"
    if name == "swift":
        command = ["ios/scripts/test-suites.sh", "all", str(directory)]
    elif name == "ui":
        command = ["ios/scripts/test-ui.sh", str(directory)]
    elif name.startswith("upgrade-"):
        if name == "upgrade-before":
            cwd = baseline_source(root)
            state["upgrade_root"] = str(directory / "retained-app-data")
        env["TEST_RUNNER_TT_UITEST_UPGRADE_ROOT"] = state["upgrade_root"]
        env["TT_UITEST_TARGET"] = "TTMobileUpgradeUITests"
        env["TT_UITEST_ONLY"] = "SimulatorUpgradeUITests/" + ("testSeedUpgrade" if name == "upgrade-before" else "testVerifyUpgrade")
        env["TT_IOS_CACHE"] = str(cwd / "ios/.cache")
        command = ["ios/scripts/test-ui.sh", str(directory)]
    elif name == "compatibility":
        env.update(TT_OLD_BINARY_REF=OLD_SCHEMA, TT_REQUIRE_NO_SKIPS="1")
        command = ["sh", "scripts/cross_machine_gate.sh"]
    else:
        command = ["go", "test", "-count=1", "-json", "./internal/config", "-run", "^TestWorkspaceBackup"]
    invoke(command, cwd, env, log)
    if name in ("swift", "ui", "upgrade-before", "upgrade-after"):
        summary = directory / "summary.json"
        checked_summary(summary, "swift" if name == "swift" else "xctest")
        artifacts.append(summary)
        if name != "swift":
            tests = directory / "tests.json"
            tree = tests.read_text()
            if name == "ui":
                for method in ("testOfflineCapturesSurviveTerminationAndConverge", "testAccountAndFolderAccessFailuresKeepLocalSaves",
                               "testWithheldBlobsWaitThenArriveWithoutDuplicates", "testLargeAttachmentResumesAndReachesPeerIntact",
                               "testKeepingAVersionResolvesTheWholeChangeAndPublishesIt"):
                    assert method in tree, f"required UI case missing: {method}"
            artifacts.append(tests)
            if name.startswith("upgrade-"):
                app = Path(env["TT_IOS_CACHE"]) / "UIDerivedData/Build/Products/Debug-iphonesimulator/tt.app"
                executables = {p.name: digest(p) for p in (app / "tt", app / "tt.debug.dylib") if p.is_file()}
                assert "tt" in executables, "upgrade app executable missing"
                build = directory / "app-build.json"
                write_json(build, {"source_commit": BASELINE if name == "upgrade-before" else output("git", "rev-parse", "HEAD"),
                                   "executables": executables})
                artifacts.append(build)
                if name == "upgrade-after":
                    before = root / state["stages"]["upgrade-before"]["directory"] / "app-build.json"
                    assert json.loads(before.read_text())["executables"] != executables, "upgrade must install a different build"
    elif name == "compatibility":
        text = log.read_text()
        assert "cross-machine gate passed." in text and "SKIP:" not in text and "FAIL:" not in text
    else:
        events = [json.loads(line) for line in log.read_text().splitlines() if line.startswith("{\"Time\"")]
        passed = [e.get("Test") for e in events if e.get("Action") == "pass" and e.get("Test", "").startswith("TestWorkspaceBackup")]
        assert len(passed) >= 4 and not any(e.get("Action") in ("fail", "skip") for e in events), "backup tests incomplete"
    artifacts.append(log)
    return {str(path.relative_to(root)): digest(path) for path in artifacts}


def run(root):
    if root == REPO or REPO in root.parents:
        relative = root.relative_to(REPO)
        ignored = subprocess.run(["git", "check-ignore", "-q", str(relative / "qualification.json")], cwd=REPO).returncode == 0
        assert relative.parts and ".git" not in relative.parts and ignored, "output inside the checkout must be gitignored (use ios/.cache/qualification)"
    root.mkdir(parents=True, exist_ok=True)
    # Local to this checkout, and across checkouts using the same user/simulator.
    with lock(root / ".lock"), lock(Path(tempfile.gettempdir()) / f"tt-simulator-qualification-{os.getuid()}.lock"), keep_awake():
        state_file = root / "qualification.json"
        current = fingerprint()
        simulator = output("ios/scripts/simulator.sh", "default")
        tools = {"xcode": output("xcodebuild", "-version"), "go": output("go", "version"),
                 "simulator": simulator, "baseline": BASELINE, "old_schema": OLD_SCHEMA}
        if state_file.exists():
            state = json.loads(state_file.read_text())
            assert state["source_fingerprint"] == current and state["tools"] == tools, "sources/toolchain changed; start a new output directory"
        else:
            state = {"format": FORMAT, "evidence_kind": "simulator-controlled-delivery", "source_fingerprint": current,
                     "tools": tools, "created_at": now(), "stages": {}, "history": [],
                     "limitations": ["Does not certify Apple iCloud delivery, device signing, or physical-device notification delivery.",
                                     "Controlled delay is released by the test; no ten-minute or seven-day real-world duration is claimed."]}
        env = dict(os.environ)
        # A caller's focused-test variables must never shrink the gate silently.
        for key in list(env):
            if key.startswith(("TT_UITEST_", "TEST_RUNNER_TT_UITEST_")):
                env.pop(key)
        env["TT_IOS_SIMULATOR"] = simulator
        state.update(status="running", pid=os.getpid(), updated_at=now())
        write_json(state_file, state)
        # Verifying an upgrade consumes its saved draft. Retry both halves.
        after = state["stages"].get("upgrade-after", {})
        before = state["stages"].get("upgrade-before", {})
        if after and any(record.get("status") != "passed" or not artifacts_valid(record, root) for record in (before, after)):
            for name in ("upgrade-before", "upgrade-after"):
                if name in state["stages"]:
                    state["history"].append({"stage": name, **state["stages"].pop(name)})
        try:
            for name in STAGES:
                previous = state["stages"].get(name, {})
                if previous.get("status") == "passed" and artifacts_valid(previous, root):
                    print(f"{name}: reusing verified result", flush=True)
                    continue
                if previous:
                    state["history"].append({"stage": name, **previous})
                attempt = 1 + sum(r["stage"] == name for r in state["history"])
                directory = root / name / f"attempt-{attempt}"
                directory.mkdir(parents=True)
                record = {"status": "running", "started_at": now(), "directory": str(directory.relative_to(root))}
                state["stages"][name] = record
                state["current_stage"] = name
                write_json(state_file, state)
                print(f"{name}: running (logs in {directory})", flush=True)
                try:
                    record["artifacts"] = run_stage(name, directory, state, root, dict(env))
                    assert fingerprint() == current, "sources changed while testing"
                    record["status"] = "passed"
                except BaseException as error:
                    record.update(status="failed", error=str(error))
                    raise
                finally:
                    record["finished_at"] = now()
                    state["updated_at"] = now()
                    write_json(state_file, state)
            state["status"] = "passed"
        except BaseException as error:
            state.update(status="failed", error=str(error))
            raise
        finally:
            state["updated_at"] = now()
            write_json(state_file, state)
        print(f"Simulator qualification passed: {state_file}", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("run", "start", "resume", "status"))
    parser.add_argument("directory", type=Path)
    args = parser.parse_args()
    root = args.directory.expanduser().resolve()
    if args.command == "status":
        if not (root / "qualification.json").exists():
            print(json.dumps({"status": "not_initialized", "log": str(root / "worker.log"),
                              "detail": "The worker is starting or failed preflight; inspect worker.log."}))
            return
        state = json.loads((root / "qualification.json").read_text())
        try:
            with lock(root / ".lock"):
                if state["status"] == "running":
                    state["status"] = "interrupted"
        except RuntimeError:
            pass
        print(json.dumps(state, indent=2))
    elif args.command == "start":
        root.mkdir(parents=True, exist_ok=True)
        with open(root / "worker.log", "a") as log:
            process = subprocess.Popen([sys.executable, "-I", str(Path(__file__).resolve()), "run", str(root)],
                                       cwd=REPO, stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        print(json.dumps({"pid": process.pid, "output": str(root), "log": str(root / "worker.log")}))
    else:
        if args.command == "resume" and not (root / "qualification.json").exists():
            parser.error("resume needs an existing qualification.json")
        run(root)


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, lambda *_: sys.exit("qualification interrupted"))
    try:
        main()
    except (RuntimeError, AssertionError, OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"simulator-qualification: {error}", file=sys.stderr)
        sys.exit(1)
