"""Guided real-device qualification of the Mac and iPhone sync, recorded as
evidence the session-18 verifier validates.

Run through ios/scripts/record-device-qualification.sh, which documents the
commands. Everything happens in one disposable trial: a copy of the
representative fixture enrolled under its own HOME, a working copy under the
trial directory, an exchange folder of its own in iCloud Drive, and on the
iPhone a joined workspace beside any other. No real workspace is read or
written.

A person performs the iPhone steps and the Mac edits in the trial's own tt;
this script prompts for them, exchanges the Mac, reads the iPhone's exported
diagnostics over the cable, and records raw observations: both devices'
synced-file digests, outstanding work, open conflicts, transport condition,
the build each device ran, and how many times each scenario's marker task
appears on the Mac. The verifier recomputes every verdict from those
observations; nothing here is trusted as a bare "passed".
"""

import datetime
import hashlib
import json
import os
import plistlib
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
FIXTURE = "internal/compat/testdata/workspaces/representative"
BUNDLE_ID = "io.github.sglavoie.tt.mobile"
SUPPORT = "Library/Application Support"
ROLLOUT_DOC = "docs/mobile/personal-rollout.md"
FORMAT = "tt-device-qualification-evidence"
LARGE_MAC_BYTES = 64 << 20
LARGE_MIN_BYTES = 25 << 20
DELAY_MIN_SECONDS = 600
SCENARIOS = ["airplane_mode", "simultaneous_edits", "termination", "delayed_delivery",
             "revoked_access", "large_attachments", "upgrade"]
PROCEDURES = {"enrollment": "## Enrollment", "backup": "## Backup", "recovery": "## Recovery",
              "troubleshooting": "## Troubleshooting", "upgrade": "## Upgrade"}


def fail(message):
    print(f"record-device-qualification: {message}", file=sys.stderr)
    sys.exit(1)


def note(message):
    print(f"record-device-qualification: {message}", file=sys.stderr)


def now():
    return datetime.datetime.now(datetime.timezone.utc)


def stamp(moment=None):
    return (moment or now()).strftime("%Y-%m-%dT%H:%M:%SZ")


def parse_time(text):
    return datetime.datetime.fromisoformat(text.replace("Z", "+00:00"))


def run(argv, env=None, cwd=REPO, check=True, log=None):
    merged = dict(os.environ)
    merged.update(env or {})
    result = subprocess.run(argv, cwd=cwd, env=merged, capture_output=True, text=True)
    if log:
        with open(log, "a") as handle:
            handle.write(f"$ {' '.join(argv)}\n{result.stdout}{result.stderr}\n")
    if check and result.returncode != 0:
        sys.stderr.write(result.stdout[-4000:] + result.stderr[-4000:])
        fail(f"{' '.join(argv[:4])} failed")
    return result


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return "sha256:" + digest.hexdigest()


def qualification_root():
    return os.environ.get("TT_QUALIFICATION_ROOT") or os.path.expanduser("~/.tt-qualification")


class Trial:
    def __init__(self, directory):
        self.dir = directory
        self.state_path = os.path.join(directory, "trial.json")
        with open(self.state_path) as handle:
            self.state = json.load(handle)

    @classmethod
    def current(cls):
        root = qualification_root()
        pointer = os.path.join(root, "current")
        if not os.path.exists(pointer):
            fail("no trial is running: start one with `record-device-qualification.sh start`")
        with open(pointer) as handle:
            return cls(os.path.join(root, handle.read().strip()))

    def save(self):
        tmp = self.state_path + ".tmp"
        with open(tmp, "w") as handle:
            json.dump(self.state, handle, indent=2, sort_keys=True)
            handle.write("\n")
        os.replace(tmp, self.state_path)

    @property
    def run_id(self):
        return self.state["run_id"]

    @property
    def working(self):
        return self.state["mac"]["data_directory"]

    @property
    def home(self):
        return self.state["mac"]["home"]

    @property
    def exchange(self):
        return self.state["exchange"]["folder"]

    def tt(self, *args, binary=None, check=True):
        return run([binary or os.path.join(self.dir, "bin", "tt"), *args],
                   env={"HOME": self.home}, check=check, log=os.path.join(self.dir, "tt.log"))

    def marker(self, scenario, tag):
        return f"q18 {self.run_id} {scenario.replace('_', '-')} {tag}"

    # The Mac side.

    def mac_exchange(self, binary=None):
        return self.tt("sync", "exchange", "--data-dir", self.working, binary=binary, check=False).returncode == 0

    def mac_observation(self, binary=None):
        report = json.loads(self.tt("sync", "diagnostics", "--data-dir", self.working, binary=binary).stdout)
        status = report.get("status") or {}
        return {
            "digest": report.get("workspace_digest", ""),
            "device_id": report.get("device_id", ""),
            "workspace_id": status.get("workspace_id", ""),
            "outstanding_up": status.get("outstanding_up", 0),
            "outstanding_down": status.get("outstanding_down", 0),
            "local_transactions": status.get("local_transactions", 0),
            "held": len(status.get("held") or []),
            "conflicts": len(report.get("conflicts") or []),
        }

    def marker_counts(self, titles):
        counts = {title: 0 for title in titles}
        projects = os.path.join(self.working, "projects")
        for slug in sorted(os.listdir(projects)):
            path = os.path.join(projects, slug, "tasks.md")
            if not os.path.exists(path):
                continue
            with open(path, encoding="utf-8") as handle:
                for line in handle:
                    if line.startswith("## Task: "):
                        title = line[len("## Task: "):].rstrip("\n")
                        if title in counts:
                            counts[title] += 1
        return counts

    def attachments(self, minimum):
        found = []
        projects = os.path.join(self.working, "projects")
        for base, _, files in os.walk(projects):
            if f"{os.sep}.attachments" not in base:
                continue
            for name in files:
                path = os.path.join(base, name)
                size = os.path.getsize(path)
                if size >= minimum:
                    found.append({"path": os.path.relpath(path, self.working), "size": size,
                                  "sha256": sha256_file(path)})
        return sorted(found, key=lambda a: a["path"])

    # The iPhone side, over the cable.

    def copy_from_iphone(self, source, destination):
        result = run(["xcrun", "devicectl", "device", "copy", "from", "--device", self.state["iphone"]["udid"],
                      "--domain-type", "appDataContainer", "--domain-identifier", BUNDLE_ID,
                      "--source", source, "--destination", destination], check=False,
                     log=os.path.join(self.dir, "devicectl.log"))
        return result.returncode == 0

    def iphone_observation(self, since):
        """The newest diagnostics export of the trial workspace written after
        since, or None. The location name comes from the app's selection."""
        scratch = tempfile.mkdtemp(dir=self.dir, prefix="pull.")
        try:
            selected = os.path.join(scratch, "selected")
            if not self.copy_from_iphone(f"{SUPPORT}/selected-workspace", selected):
                note("could not read the app's selection: is the iPhone connected and unlocked?")
                return None
            with open(selected) as handle:
                name = handle.read().strip()
            if not name:
                note("tt on the iPhone has no workspace selected")
                return None
            pulled = os.path.join(scratch, "diagnostics")
            if not self.copy_from_iphone(f"{SUPPORT}/devices/{name}/diagnostics", pulled):
                note(f"no diagnostics exported yet for the iPhone workspace {name}")
                return None
            newest = None
            for base, _, files in os.walk(pulled):
                for file in files:
                    if not file.endswith(".json"):
                        continue
                    with open(os.path.join(base, file)) as handle:
                        document = json.load(handle)
                    exported = parse_time(document["exported_at"])
                    if exported < since:
                        continue
                    if newest is None or exported > parse_time(newest["exported_at"]):
                        newest = document
            if newest is None:
                return None
            engine = newest.get("engine") or {}
            status = engine.get("status") or {}
            transport = newest.get("transport") or {}
            archive = os.path.join(self.dir, "iphone-diagnostics")
            os.makedirs(archive, exist_ok=True)
            with open(os.path.join(archive, newest["exported_at"].replace(":", "") + ".json"), "w") as handle:
                json.dump(newest, handle, indent=2, sort_keys=True)
            return {
                "location": name,
                "exported_at": newest["exported_at"],
                "app_executables": newest.get("app_executables") or {},
                "digest": engine.get("workspace_digest", ""),
                "device_id": engine.get("device_id", ""),
                "workspace_id": status.get("workspace_id", ""),
                "outstanding_up": status.get("outstanding_up", 0),
                "local_transactions": status.get("local_transactions", 0),
                "received": status.get("received") or {},
                "conflicts": len(engine.get("conflicts") or []),
                "engine_error": newest.get("engine_error"),
                "transport": {
                    "has_folder": transport.get("has_folder", False),
                    "condition": transport.get("condition"),
                    "last_error": transport.get("last_error"),
                    "outstanding_uploads": transport.get("outstanding_uploads", 0),
                },
            }
        finally:
            shutil.rmtree(scratch, ignore_errors=True)


def drive_tui(trial, keys, binary=None):
    """Runs the Mac's trial tt in a pseudo-terminal, as a person at the
    keyboard would, sends each key string in turn, then Ctrl+C."""
    import pty
    import select
    import struct
    import fcntl
    import termios
    env = {**os.environ, "HOME": trial.home, "TERM": "xterm-256color"}
    pid, fd = pty.fork()
    if pid == 0:
        os.execve(binary or os.path.join(trial.dir, "bin", "tt"),
                  [binary or "tt", "--data-dir", trial.working], env)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 140, 0, 0))

    def drain(seconds):
        end = time.time() + seconds
        while time.time() < end:
            ready, _, _ = select.select([fd], [], [], 0.1)
            if ready:
                try:
                    os.read(fd, 65536)
                except OSError:
                    return

    drain(3.0)
    for key in list(keys) + ["\x03"]:
        os.write(fd, key.encode())
        drain(1.5)
    try:
        os.kill(pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    os.waitpid(pid, 0)
    os.close(fd)


def mac_capture(trial, line, binary=None):
    """Captures one line on the Mac (`a`, the line, Enter) and checks that its
    title now appears exactly once more."""
    title = line.split(" &", 1)[0]
    before = trial.marker_counts([title])[title]
    drive_tui(trial, ["a", line, "\r"], binary)
    after = trial.marker_counts([title])[title]
    if after != before + 1:
        fail(f"the Mac capture of {title!r} did not land: {before} -> {after} copies")
    note(f"Mac: captured {title!r}")


def mac_rename(trial, old, new, binary=None):
    """Renames a task on the Mac: `/` search, Enter onto the result, `e` to
    edit inline, clear the line, type the new title, Enter."""
    drive_tui(trial, ["/", old, "\r", "e", "\x15", new, "\r"], binary)
    counts = trial.marker_counts([old, new])
    if counts[new] != 1 or counts[old] != 0:
        fail(f"the Mac rename of {old!r} to {new!r} did not land: {counts}")
    note(f"Mac: renamed {old!r} to {new!r}")


def cmd_mac_capture(args):
    """mac-capture LINE: one Mac capture in the trial, verified by count."""
    if not args:
        fail("usage: mac-capture LINE")
    mac_capture(Trial.current(), " ".join(args))


# Interaction.

# With TT_QUALIFICATION_MAILBOX set to an ASL plan directory, every prompt is
# asked through that plan's mailbox (asl ask --wait) and the person's reply is
# the answer; otherwise the prompts read the terminal.
MAILBOX = os.environ.get("TT_QUALIFICATION_MAILBOX")
MAILBOX_WAIT_ROUNDS = 12  # 30 minutes each
_pending = []


def say(text):
    print("\n" + text.strip() + "\n")
    if MAILBOX:
        _pending.append(text.strip())


def human(question):
    """One answer from the person."""
    if not MAILBOX:
        return input(question + " ")
    body = "\n\n".join(_pending + [question])
    _pending.clear()
    argv = ["asl", "ask", MAILBOX, body, "--wait", "--wait-timeout", "1800", "--json"]
    for _ in range(MAILBOX_WAIT_ROUNDS):
        result = json.loads(run(argv, check=False).stdout or "{}")
        if result.get("reply"):
            return result["reply"].get("text", "")
        if result.get("resolution"):
            fail(f"the mailbox question was resolved without an answer: {result['resolution']}")
        if not result.get("timed_out") or not result.get("question_id"):
            fail(f"asl ask failed: {result}")
        argv = ["asl", "ask", MAILBOX, "--question-id", result["question_id"], "--wait", "--wait-timeout", "1800", "--json"]
    fail("no answer arrived through the mailbox")


def wait_enter(prompt="Press Enter when done."):
    human(prompt if not MAILBOX else "Reply 'done' when you have finished these steps.")


def ask_yes(question):
    while True:
        answer = human(question + " [y/n]").strip().lower()
        if answer.startswith("y"):
            return True
        if answer.startswith("n"):
            return False


EXPORT = ("On the iPhone, in tt: pull down to refresh, then tap the sync status button and "
          "Export Diagnostics under Recovery. Keep the iPhone unlocked and connected by cable.")


def checkpoint(trial, scenario, label, markers=(), expect=None, attachments_min=None, rounds_hint=""):
    """Exchanges the Mac, asks the person to refresh and export on the iPhone,
    and records both devices until the expectation holds or the person stops
    retrying. Returns the recorded checkpoint."""
    expect = expect or {}
    markers = list(markers)
    while True:
        started = now() - datetime.timedelta(seconds=5)
        trial.mac_exchange(expect.get("mac_binary"))
        say(f"[{scenario} / {label}] {rounds_hint}\n{EXPORT}")
        wait_enter()
        trial.mac_exchange(expect.get("mac_binary"))
        iphone = trial.iphone_observation(started)
        mac = trial.mac_observation(expect.get("mac_binary"))
        counts = trial.marker_counts(markers)
        record = {"scenario": scenario, "label": label, "at": stamp(), "mac": mac, "iphone": iphone,
                  "markers": counts}
        if attachments_min:
            record["attachments"] = trial.attachments(attachments_min)
        problems = checkpoint_problems(trial, record, expect)
        if not problems:
            note(f"{scenario} / {label}: recorded")
            return record
        say("Not there yet:\n  - " + "\n  - ".join(problems))
        if not ask_yes("Retry this checkpoint (iCloud can take minutes)? Answering n records it as failed."):
            record["problems"] = problems
            return record


def checkpoint_problems(trial, record, expect):
    """The same rules the Go validator applies, so a person sees a failure
    while there is still time to retry."""
    problems = []
    mac, iphone = record["mac"], record["iphone"]
    if iphone is None:
        return ["no iPhone diagnostics exported since this checkpoint began"]
    if iphone["workspace_id"] != trial.state["workspace"]["workspace_id"]:
        problems.append(f"the iPhone exported workspace {iphone['workspace_id']}, not the trial's")
    builds = expect.get("iphone_build", trial.state["builds"]["app_executables"])
    if builds is not None and iphone["app_executables"] != builds:
        problems.append("the iPhone is running a different build than the one installed for this step")
    present = expect.get("present", list(record["markers"]))
    for title in present:
        if record["markers"].get(title) != 1:
            problems.append(f"the Mac holds {record['markers'].get(title, 0)} copies of {title!r}, want 1")
    for title in expect.get("absent", []):
        if record["markers"].get(title):
            problems.append(f"{title!r} reached the Mac before it should have")
    if expect.get("converged", True):
        if not mac["digest"] or mac["digest"] != iphone["digest"]:
            problems.append(f"the devices differ: Mac {mac['digest']}, iPhone {iphone['digest']}")
        if mac["outstanding_up"] or iphone["outstanding_up"]:
            problems.append(f"unpublished work remains: Mac {mac['outstanding_up']}, iPhone {iphone['outstanding_up']}")
    conflicts = expect.get("conflicts", "none")
    if conflicts == "none" and (mac["conflicts"] or iphone["conflicts"]):
        problems.append(f"open conflicts: Mac {mac['conflicts']}, iPhone {iphone['conflicts']}")
    if conflicts == "both" and not (mac["conflicts"] and iphone["conflicts"]):
        problems.append(f"the genuine conflict is not preserved on both: Mac {mac['conflicts']}, iPhone {iphone['conflicts']}")
    if expect.get("isolated"):
        # iOS lets the app write its exchange files into the local iCloud
        # Drive copy while offline, so unpublished work proves nothing.
        # Isolation is what neither side could get: the Mac exchanged and
        # still lacks the iPhone's markers (checked as absent), and the
        # iPhone has not applied the Mac's latest transaction.
        got = iphone.get("received", {}).get(mac["device_id"], 0)
        if got >= mac.get("local_transactions", 0):
            problems.append(f"the iPhone applied the Mac's transaction {got} of {mac.get('local_transactions', 0)}, so it was not cut off")
    if expect.get("iphone_unreachable"):
        transport = iphone["transport"]
        if transport["condition"] == "reachable" and not transport["last_error"]:
            problems.append("the iPhone still reaches the exchange folder")
    return problems


# Builds.

def device_info(udid):
    with tempfile.NamedTemporaryFile(suffix=".json") as out:
        run(["xcrun", "devicectl", "list", "devices", "--json-output", out.name])
        devices = json.load(open(out.name))["result"]["devices"]
    for device in devices:
        hw, props = device["hardwareProperties"], device["deviceProperties"]
        if hw.get("platform") != "iOS" or hw.get("reality") != "physical":
            continue
        if udid and hw.get("udid") != udid:
            continue
        return {"udid": hw["udid"], "name": props.get("name"), "model": hw.get("productType"),
                "os_version": props.get("osVersionNumber"), "reality": hw["reality"]}
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


def install(trial, app):
    run(["xcrun", "devicectl", "device", "install", "app", "--device", trial.state["iphone"]["udid"], app],
        log=os.path.join(trial.dir, "devicectl.log"))


# Commands.

def cmd_start(_args):
    if sys.platform != "darwin":
        fail("a device qualification needs macOS")
    team = os.environ.get("TT_IOS_DEVELOPMENT_TEAM")
    if not team:
        fail("set TT_IOS_DEVELOPMENT_TEAM to the signing team")
    root = qualification_root()
    pointer = os.path.join(root, "current")
    if os.path.exists(pointer) and not os.environ.get("TT_QUALIFICATION_RESTART"):
        fail(f"a trial is already running ({open(pointer).read().strip()}); finish it, or set TT_QUALIFICATION_RESTART=1")
    run_id = now().strftime("%Y%m%dT%H%M%SZ")
    directory = os.path.join(root, run_id)
    exchange_root = os.environ.get("TT_QUALIFICATION_EXCHANGE") or os.path.expanduser(
        "~/Library/Mobile Documents/com~apple~CloudDocs/tt-qualification")
    if "/Library/Mobile Documents/" not in exchange_root:
        fail(f"{exchange_root} is not inside iCloud Drive")
    exchange = os.path.join(exchange_root, run_id)
    iphone = device_info(os.environ.get("TT_IOS_DEVICE"))
    os.makedirs(os.path.join(directory, "bin"))
    os.makedirs(os.path.join(directory, "home"))
    log = os.path.join(directory, "build.log")

    note(f"trial {run_id} in {directory}")
    note("building tt for the Mac")
    tt_sha = build_mac(REPO, os.path.join(directory, "bin", "tt"))
    note("building, signing, and installing the iPhone app as the rollout guide does")
    app = run(["ios/scripts/install-device.sh"], env={"TT_IOS_DEVICE": iphone["udid"]}, log=log).stdout.strip()
    executables = app_executables(app)
    signed = signing(app)
    signed["installed_at"] = stamp()
    signed["installed_with"] = "devicectl"

    state = {
        "format": FORMAT, "format_version": 1, "run_id": run_id, "started_at": stamp(),
        "builds": {
            "source_fingerprint": run(["ios/scripts/qualification-fingerprint.sh"]).stdout.strip(),
            "bridge_fingerprint": run(["ios/scripts/build-bridge.sh", "--fingerprint"]).stdout.strip(),
            "tt_sha256": tt_sha,
            "app_executables": executables,
        },
        "signing": signed,
        "mac": {"model": run(["sysctl", "-n", "hw.model"]).stdout.strip(),
                "os_version": run(["sw_vers", "-productVersion"]).stdout.strip(),
                "home": os.path.join(directory, "home"),
                "original": os.path.join(directory, "original"),
                "data_directory": os.path.join(directory, "working")},
        "iphone": iphone,
        "exchange": {"provider": "iCloud Drive", "folder": exchange},
        "workspace": {"fixture": FIXTURE},
        "checkpoints": [], "scenarios": {}, "daily_use": [], "procedures": {}, "failures": [],
    }
    with open(os.path.join(directory, "trial.json"), "w") as handle:
        json.dump(state, handle, indent=2, sort_keys=True)
    trial = Trial(directory)

    shutil.copytree(os.path.join(REPO, FIXTURE), trial.state["mac"]["original"])
    os.makedirs(exchange, exist_ok=True)
    note("enrolling the disposable workspace")
    trial.tt("sync", "enroll", "--data-dir", trial.state["mac"]["original"], "--exchange", exchange,
             "--working-copy", trial.working, "--force")
    mac = trial.mac_observation()
    trial.state["workspace"]["workspace_id"] = mac["workspace_id"]
    trial.state["mac"]["device_id"] = mac["device_id"]
    wrapper = os.path.join(directory, "bin", "tt-trial")
    with open(wrapper, "w") as handle:
        handle.write(f"#!/bin/sh\nHOME='{trial.home}' exec '{directory}/bin/tt' --data-dir '{trial.working}' \"$@\"\n")
    os.chmod(wrapper, 0o755)
    trial.save()
    os.makedirs(root, exist_ok=True)
    with open(pointer, "w") as handle:
        handle.write(run_id + "\n")

    say(f"""
Join the trial on the iPhone, following "Enrollment" in {ROLLOUT_DOC}:
  1. Open tt (the build just installed). If a workspace is open, use Choose
     Another Workspace so the one you have is left alone.
  2. Tap Join from an Exchange Folder… and pick iCloud Drive/{os.path.relpath(exchange, os.path.dirname(exchange_root))}.
The Mac's trial tt is {wrapper}; open it any time to read or edit the trial.""")
    record_enrollment(trial)


def record_enrollment(trial):
    """The join checkpoint and the enrollment procedure; `enrollment` reruns
    it for the current trial when a pause interrupted it."""
    record = checkpoint(trial, "enrollment", "joined")
    trial.state["checkpoints"] = [c for c in trial.state["checkpoints"] if c["scenario"] != "enrollment"] + [record]
    snapshot_root = os.path.join(trial.home, ".tt", "enrollments")
    trial.state["procedures"]["enrollment"] = procedure_record(
        "enrollment", [record["label"]],
        {"snapshot_kept": os.path.isdir(snapshot_root) and bool(os.listdir(snapshot_root)),
         "original_content_untouched": digest_tree(trial.state["mac"]["original"]) == digest_tree(os.path.join(REPO, FIXTURE))})
    trial.save()
    if record.get("problems"):
        fail(f"the join checkpoint was recorded as failed: {record['problems']}; rerun `enrollment` after fixing the cause")
    note("enrollment recorded; run the scenarios next (record-device-qualification.sh status lists them)")


def digest_tree(directory):
    """Paths and contents of the synced files under directory, as
    synclog.Classify splits synced from device-local: opening a workspace adds
    locks, journals, and history beside them and changes none of them."""
    lines = []
    for base, _, files in os.walk(directory):
        for name in files:
            path = os.path.join(base, name)
            rel = os.path.relpath(path, directory).replace(os.sep, "/")
            if is_synced(rel):
                lines.append(f"{rel}\t{sha256_file(path)}\n")
    return "sha256:" + hashlib.sha256("".join(sorted(lines)).encode()).hexdigest()


def is_synced(rel):
    parts = rel.split("/")
    name = parts[-1]
    if name.endswith((".bak", ".lock")) or name.startswith(".tt-stage-"):
        return False
    if parts[0] == ".tt":
        return (len(parts) == 2 and name in ("workspace.json", "planning.json", "lenses.json")) or \
            (len(parts) >= 3 and parts[1] == "global-notes")
    return parts[0] == "projects" and len(parts) >= 3 and parts[1] != ""


def section_digest(heading):
    with open(os.path.join(REPO, ROLLOUT_DOC), encoding="utf-8") as handle:
        text = handle.read()
    match = re.search(r"^" + re.escape(heading) + r"\n(.*?)(?=^## |\Z)", text, re.M | re.S)
    if not match:
        fail(f"{ROLLOUT_DOC} has no section {heading!r}")
    return "sha256:" + hashlib.sha256((heading + "\n" + match.group(1)).encode()).hexdigest()


def procedure_record(name, checkpoints, checks):
    followed = ask_yes(f'Did you follow "{PROCEDURES[name][3:]}" in {ROLLOUT_DOC} as written, and did it match what happened?')
    remarks = "" if followed else human("What differed?").strip()
    return {"validated_at": stamp(), "document": ROLLOUT_DOC, "section": PROCEDURES[name],
            "section_sha256": section_digest(PROCEDURES[name]), "followed_as_written": followed,
            "remarks": remarks, "checkpoints": checkpoints, "checks": checks}


def scenario_airplane(trial):
    s = "airplane_mode"
    phone = [trial.marker(s, "iphone-1"), trial.marker(s, "iphone-2")]
    mac = [trial.marker(s, "mac-1")]
    say(f"""
Turn on Airplane Mode on the iPhone (Wi-Fi off too). Keep the cable connected.
Skip any of these you have already done in this trial.
In tt on the iPhone capture two tasks titled exactly:
  {phone[0]}
  {phone[1]}
and complete one existing task. The Mac captures its own task meanwhile.""")
    if trial.marker_counts(mac)[mac[0]] == 0:
        mac_capture(trial, mac[0])
    wait_enter()
    offline = checkpoint(trial, s, "offline", phone + mac,
                         {"converged": False, "present": mac, "absent": phone, "isolated": True})
    say("Turn Airplane Mode off on the iPhone and open tt.")
    wait_enter()
    online = checkpoint(trial, s, "online", phone + mac)
    return [offline, online], {}


def scenario_simultaneous(trial):
    s = "simultaneous_edits"
    shared = trial.marker(s, "shared")
    phone_title, mac_title = trial.marker(s, "renamed-on-iphone"), trial.marker(s, "renamed-on-mac")
    phone_own, mac_own = trial.marker(s, "iphone-only"), trial.marker(s, "mac-only")
    mac_capture(trial, shared)
    base = checkpoint(trial, s, "base", [shared], rounds_hint=f"Pull down to refresh until the task {shared!r} appears on the iPhone; don't rename it yet.")
    say(f"""
Now edit the same task on the iPhone while it cannot exchange:
  1. Turn on Airplane Mode on the iPhone.
  2. In tt on the iPhone, rename {shared!r} to {phone_title!r}.
  3. Capture a task titled {phone_own!r}.
Leave Airplane Mode on for now; the Mac makes its own edit next.""")
    wait_enter()
    mac_rename(trial, shared, mac_title)
    mac_capture(trial, mac_own)
    say("Now turn Airplane Mode off on the iPhone and open tt.")
    wait_enter()
    both = [phone_own, mac_own]
    conflicted = checkpoint(trial, s, "conflicted", both + [phone_title, mac_title],
                            {"present": both, "converged": False, "conflicts": "both"},
                            rounds_hint="Both devices should now show one sync conflict. Do not resolve it yet.")
    say(f"""
Resolve the conflict on the iPhone, following "Troubleshooting" in {ROLLOUT_DOC}:
tap the sync status button, open the conflict, and keep one of the two titles.""")
    wait_enter()
    resolved = checkpoint(trial, s, "resolved", both + [phone_title, mac_title, shared], {"present": both})
    kept = [t for t in (phone_title, mac_title) if resolved["markers"].get(t) == 1]
    return [base, conflicted, resolved], {"kept_title": kept[0] if len(kept) == 1 else None}


def scenario_termination(trial):
    s = "termination"
    phone = [trial.marker(s, "iphone-1"), trial.marker(s, "iphone-2")]
    mac = trial.marker(s, "mac-1")
    say(f"""
On the iPhone, capture {phone[0]!r} and immediately swipe tt away in the App
Switcher. Reopen tt, capture {phone[1]!r}, and swipe it away again right after
saving, while it exchanges. Reopen tt once more.""")
    wait_enter()
    mac_capture(trial, mac)
    interrupted = 0
    for delay in (0.2, 0.5, 1.0):
        process = subprocess.Popen([os.path.join(trial.dir, "bin", "tt"), "sync", "exchange", "--data-dir", trial.working],
                                   env={**os.environ, "HOME": trial.home}, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(delay)
        if process.poll() is None:
            process.send_signal(signal.SIGKILL)
            interrupted += 1
        process.wait()
    note(f"killed {interrupted} Mac exchanges mid-pass")
    record = checkpoint(trial, s, "relaunched", phone + [mac])
    return [record], {"mac_exchanges_killed": interrupted}


def scenario_delayed(trial):
    s = "delayed_delivery"
    mac, phone = trial.marker(s, "mac-1"), trial.marker(s, "iphone-1")
    say(f"""
Turn on Airplane Mode on the iPhone, then capture {phone!r} in tt.
Keep Airplane Mode on: the Mac publishes next, and the iPhone has to stay
offline for at least {DELAY_MIN_SECONDS // 60} minutes after that. You'll be told when.""")
    wait_enter()
    mac_capture(trial, mac)
    if not trial.mac_exchange():
        fail("the Mac could not publish")
    published = now()
    note(f"the Mac published at {stamp(published)}; the iPhone must stay offline until "
         f"{stamp(published + datetime.timedelta(seconds=DELAY_MIN_SECONDS))}")
    while (now() - published).total_seconds() < DELAY_MIN_SECONDS:
        time.sleep(15)
    say("Turn Airplane Mode off on the iPhone and open tt.")
    wait_enter()
    record = checkpoint(trial, s, "delivered", [mac, phone])
    return [record], {"mac_published_at": stamp(published), "online_after_seconds": int((now() - published).total_seconds())}


def scenario_revoked(trial):
    s = "revoked_access"
    phone, mac = trial.marker(s, "iphone-1"), trial.marker(s, "mac-1")
    say(f"""
Revoke tt's access to the exchange folder on the iPhone, as "Troubleshooting"
in {ROLLOUT_DOC} describes: Settings > your name > iCloud > iCloud Drive, turn
off Sync this iPhone (or sign out of iCloud). Then open tt, capture {phone!r},
and read the banner.""")
    wait_enter()
    if trial.marker_counts([mac])[mac] == 0:
        mac_capture(trial, mac)
    revoked = checkpoint(trial, s, "revoked", [phone, mac],
                         {"converged": False, "present": [mac], "absent": [phone], "isolated": True,
                          "iphone_unreachable": True})
    banner = ask_yes("Did tt say your work is saved on the iPhone and offer Retry?")
    say("""
Restore access: turn iCloud Drive syncing back on, open tt, and if it asks,
Choose Exchange Folder… and pick the trial folder again, then Retry.""")
    wait_enter()
    restored = checkpoint(trial, s, "restored", [phone, mac])
    return [revoked, restored], {"saved_locally_banner": banner}


def mac_opens(trial, title):
    """Whether the iPhone's large attachment reached the Mac whole: a file of
    at least the minimum size under the task's attachments, other than the
    Mac's own."""
    own = f"q18-{trial.run_id}-mac.bin"
    return any(not a["path"].endswith(own) for a in trial.attachments(LARGE_MIN_BYTES))


def scenario_large(trial):
    s = "large_attachments"
    blob = os.path.join(trial.dir, "attachments", f"q18-{trial.run_id}-mac.bin")
    os.makedirs(os.path.dirname(blob), exist_ok=True)
    with open(blob, "wb") as handle:
        handle.write(os.urandom(LARGE_MAC_BYTES))
    mac, phone = trial.marker(s, "mac"), trial.marker(s, "iphone")
    say(f"""
On the iPhone, capture {phone!r} and attach a file of at least
{LARGE_MIN_BYTES >> 20} MB to it (a video from Files or Photos works). The Mac
attaches its own 64 MiB file to {mac!r}. Once it arrives on the iPhone,
open that attachment there.""")
    mac_capture(trial, f"{mac} &{blob}")
    wait_enter()
    record = checkpoint(trial, s, "delivered", [mac, phone], attachments_min=LARGE_MIN_BYTES)
    opened = ask_yes("Did the Mac's 64 MiB attachment open on the iPhone?") and mac_opens(trial, phone)
    return [record], {"mac_attachment_sha256": sha256_file(blob), "mac_attachment_size": LARGE_MAC_BYTES,
                      "opened_on_peer": opened}


def scenario_upgrade(trial):
    s = "upgrade"
    ref = os.environ.get("TT_QUALIFICATION_UPGRADE_FROM")
    team = os.environ.get("TT_IOS_DEVELOPMENT_TEAM")
    if not ref or not team:
        fail("set TT_QUALIFICATION_UPGRADE_FROM to the commit of the build being upgraded from, and TT_IOS_DEVELOPMENT_TEAM")
    commit = run(["git", "rev-parse", ref + "^{commit}"]).stdout.strip()
    source = os.path.join(trial.dir, "old-source")
    if os.path.exists(source):
        run(["git", "worktree", "remove", "--force", source], check=False)
    run(["git", "worktree", "add", "--detach", source, commit])
    try:
        log = os.path.join(trial.dir, "build-old.log")
        old_tt = os.path.join(trial.dir, "bin", "tt-old")
        note(f"building tt and the app at {commit[:12]}")
        build_mac(source, old_tt)
        app, old_executables = build_app(source, trial.state["iphone"]["udid"], team, "ios/.cache/QualificationDerivedData", log)
        install(trial, app)
    finally:
        run(["git", "worktree", "remove", "--force", source], check=False)
    if old_executables == trial.state["builds"]["app_executables"]:
        fail(f"{ref} builds the same app as the tree: pick the commit of an earlier build")
    wrapper = os.path.join(trial.dir, "bin", "tt-trial-old")
    with open(wrapper, "w") as handle:
        handle.write(f"#!/bin/sh\nHOME='{trial.home}' exec '{old_tt}' --data-dir '{trial.working}' \"$@\"\n")
    os.chmod(wrapper, 0o755)
    old_phone, old_mac = trial.marker(s, "iphone-old"), trial.marker(s, "mac-old")
    pending, new_mac = trial.marker(s, "iphone-pending"), trial.marker(s, "mac-new")
    say(f"""
The earlier build is installed on the iPhone. Open tt and capture {old_phone!r}.
Then turn on Airplane Mode and capture {pending!r}, so work is waiting to
publish across the upgrade. Leave Airplane Mode on.""")
    mac_capture(trial, old_mac, binary=old_tt)
    wait_enter()
    before = checkpoint(trial, s, "before", [old_phone, old_mac, pending],
                        {"converged": False, "present": [old_mac], "absent": [pending],
                         "iphone_build": None, "mac_binary": old_tt})
    note("installing the current build over it")
    app = run(["ios/scripts/install-device.sh"], env={"TT_IOS_DEVICE": trial.state["iphone"]["udid"]}, log=log).stdout.strip()
    if app_executables(app) != trial.state["builds"]["app_executables"]:
        fail("the tree no longer builds the app this trial started with")
    say(f"""
Upgraded, following "Upgrade" in {ROLLOUT_DOC}. Open tt on the iPhone: it must
still show {pending!r}. Then turn Airplane Mode off.""")
    mac_capture(trial, new_mac)
    wait_enter()
    after = checkpoint(trial, s, "after", [old_phone, old_mac, pending, new_mac])
    trial.state["procedures"]["upgrade"] = procedure_record("upgrade", ["before", "after"], {"from_commit": commit})
    return [before, after], {"from_commit": commit, "from_app_executables": old_executables,
                             "from_tt_sha256": sha256_file(old_tt).removeprefix("sha256:")}


SCENARIO_RUNNERS = {
    "airplane_mode": scenario_airplane, "simultaneous_edits": scenario_simultaneous,
    "termination": scenario_termination, "delayed_delivery": scenario_delayed,
    "revoked_access": scenario_revoked, "large_attachments": scenario_large, "upgrade": scenario_upgrade,
}


def cmd_enrollment(_args):
    trial = Trial.current()
    say(f"""
The trial workspace should already be joined on the iPhone, from iCloud Drive/tt-qualification/{trial.run_id}.
If tt says the exchange has not delivered the workspace yet, long-press that folder in Files,
choose Keep Downloaded, and join again.""")
    record_enrollment(trial)


def cmd_scenario(args):
    if not args or args[0] not in SCENARIO_RUNNERS:
        fail("usage: scenario " + "|".join(SCENARIOS))
    trial = Trial.current()
    name = args[0]
    records, observations = SCENARIO_RUNNERS[name](trial)
    remarks = human("Anything unexpected in this scenario? Reply 'none' if nothing.").strip()
    if remarks.lower().rstrip(".") in ("none", "nothing", "no"):
        remarks = ""
    trial.state["checkpoints"] = [c for c in trial.state["checkpoints"] if c["scenario"] != name] + records
    trial.state["scenarios"][name] = {"recorded_at": stamp(), "checkpoints": [r["label"] for r in records],
                                      "observations": observations, "remarks": remarks}
    trial.save()
    failed = [r["label"] for r in records if r.get("problems")]
    if failed:
        fail(f"{name} recorded with failed checkpoints {failed}; rerun it after fixing the cause")
    note(f"{name} recorded")


def cmd_day(_args):
    trial = Trial.current()
    today = datetime.date.today().isoformat()
    mac, phone = trial.marker("day", today + " mac"), trial.marker("day", today + " iphone")
    say(f"""
Daily-use checkpoint for {today}. Use tt on the iPhone in the trial workspace
for real work today: capture, complete, edit, plan. Also capture {phone!r} on
the iPhone. The Mac adds its own marker.""")
    mac_capture(trial, mac)
    wait_enter()
    record = checkpoint(trial, "daily_use", today, [mac, phone])
    lost = ask_yes("Did any edit you made today go missing on either device?")
    diverged = ask_yes("Did the devices ever show something different that you could not explain?")
    remarks = human("Describe what happened.").strip() if lost or diverged else ""
    trial.state["checkpoints"] = [c for c in trial.state["checkpoints"]
                                  if not (c["scenario"] == "daily_use" and c["label"] == today)] + [record]
    trial.state["daily_use"] = [d for d in trial.state["daily_use"] if d["date"] != today] + [
        {"date": today, "lost_edits": lost, "unexplained_divergence": diverged, "remarks": remarks}]
    trial.save()
    note(f"day {today} recorded; {len(trial.state['daily_use'])} days so far")


def cmd_procedure(args):
    trial = Trial.current()
    name = args[0] if args else ""
    if name == "backup":
        backup = os.path.join(trial.dir, "backups", now().strftime("%Y%m%dT%H%M%SZ"))
        # The recorder's tt closes after every Mac step, so no commit is in
        # flight; the backup follows "Backup" in the rollout guide.
        trial.mac_exchange()
        run(["ditto", trial.working, os.path.join(backup, "working")])
        doctor = trial.tt("doctor", "--data-dir", os.path.join(backup, "working"), check=False)
        checks = {"backup": os.path.relpath(backup, trial.dir),
                  "matches_working_copy": synced_digest(trial, os.path.join(backup, "working")) == trial.mac_observation()["digest"],
                  "doctor_passes": doctor.returncode == 0}
        trial.state["procedures"]["backup"] = procedure_record("backup", [], checks)
    elif name == "recovery":
        backups = sorted(os.listdir(os.path.join(trial.dir, "backups"))) if os.path.isdir(os.path.join(trial.dir, "backups")) else []
        if not backups:
            fail("record the backup procedure first")
        trial.mac_exchange()
        rejoined = os.path.join(trial.dir, "recovered", now().strftime("%Y%m%dT%H%M%SZ"))
        home = rejoined + "-home"
        os.makedirs(home)
        joined = run([os.path.join(trial.dir, "bin", "tt"), "sync", "join", "--exchange", trial.exchange,
                      "--working-copy", rejoined], env={"HOME": home}, check=False, log=os.path.join(trial.dir, "tt.log"))
        restored = os.path.join(rejoined + "-restored")
        run(["ditto", os.path.join(trial.dir, "backups", backups[-1], "working"), restored])
        checks = {"rejoined_from_exchange": joined.returncode == 0 and synced_digest(trial, rejoined) == trial.mac_observation()["digest"],
                  "restored_from_backup_opens": trial.tt("doctor", "--data-dir", restored, check=False).returncode == 0}
        trial.state["procedures"]["recovery"] = procedure_record("recovery", [], checks)
    elif name == "troubleshooting":
        doctor = trial.tt("sync", "doctor", "--data-dir", trial.working, "--json", check=False)
        diagnostics = trial.tt("sync", "diagnostics", "--data-dir", trial.working, check=False)
        checks = {"sync_doctor_passes": doctor.returncode == 0,
                  "diagnostics_export": diagnostics.returncode == 0 and '"workspace_digest"' in diagnostics.stdout,
                  "revoked_access_recorded": "revoked_access" in trial.state["scenarios"]}
        trial.state["procedures"]["troubleshooting"] = procedure_record("troubleshooting", [], checks)
    elif name in ("enrollment", "upgrade"):
        fail(f"{name} is recorded by `start` and `scenario upgrade`")
    else:
        fail("usage: procedure backup|recovery|troubleshooting")
    trial.save()
    note(f"procedure {name} recorded: {trial.state['procedures'][name]['checks']}")


def synced_digest(trial, directory):
    """The synced-file digest tt reports for an enrolled directory, read under
    a HOME of its own so this machine's identity and selection stay as they
    are."""
    home = tempfile.mkdtemp(dir=trial.dir, prefix="digest-home.")
    try:
        result = run([os.path.join(trial.dir, "bin", "tt"), "sync", "diagnostics", "--data-dir", directory],
                     env={"HOME": home}, check=False, log=os.path.join(trial.dir, "tt.log"))
        return json.loads(result.stdout).get("workspace_digest") if result.returncode == 0 else None
    finally:
        shutil.rmtree(home, ignore_errors=True)


def cmd_status(_args):
    trial = Trial.current()
    state = trial.state
    print(f"trial {state['run_id']} in {trial.dir}")
    for name in SCENARIOS:
        print(f"  scenario {name:20} {'recorded' if name in state['scenarios'] else 'pending'}")
    for name in PROCEDURES:
        print(f"  procedure {name:19} {'recorded' if name in state['procedures'] else 'pending'}")
    days = sorted(d["date"] for d in state["daily_use"])
    print(f"  daily use: {len(days)} days" + (f" ({days[0]} .. {days[-1]})" if days else ""))


def cmd_finish(_args):
    trial = Trial.current()
    state = dict(trial.state)
    state["finished_at"] = stamp()
    evidence = os.environ.get("TT_QUALIFICATION_EVIDENCE") or os.path.join(REPO, "docs/mobile/evidence/session-18-qualification.json")
    os.makedirs(os.path.dirname(evidence), exist_ok=True)
    with open(evidence, "w") as handle:
        json.dump(state, handle, indent=2, sort_keys=True)
        handle.write("\n")
    note(f"wrote {evidence}; bash scripts/verify_mobile_session.sh 18 validates it")
    if os.environ.get("TT_QUALIFICATION_KEEP") != "1":
        os.remove(os.path.join(qualification_root(), "current"))


COMMANDS = {"start": cmd_start, "enrollment": cmd_enrollment, "mac-capture": cmd_mac_capture, "scenario": cmd_scenario, "day": cmd_day, "procedure": cmd_procedure,
            "status": cmd_status, "finish": cmd_finish}

if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        fail("usage: record-device-qualification.sh start|enrollment|scenario NAME|day|procedure NAME|status|finish")
    COMMANDS[sys.argv[1]](sys.argv[2:])
