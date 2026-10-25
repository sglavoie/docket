"""Trial state, workspace observations, and shared recorder utilities."""

import datetime
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile


REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
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
    return os.environ.get("DOCKET_QUALIFICATION_ROOT") or os.path.expanduser("~/.tt-qualification")


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

    # A scenario rerun after an interrupted attempt left its markers behind
    # uses fresh ones, suffixed "aN"; the attempt is recorded so the
    # validator derives the same titles.
    attempts = {}
    mac_attempts = {}

    def marker(self, scenario, tag):
        title = f"q18 {self.run_id} {scenario.replace('_', '-')} {tag}"
        attempt = self.attempts.get(scenario, 1)
        if tag.startswith("mac"):
            attempt = self.mac_attempts.get(scenario, attempt)
        return f"{title} a{attempt}" if attempt > 1 else title

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
