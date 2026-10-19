"""Rollout procedure evidence, workspace digests, and recovery drills."""

import hashlib
import json
import os
import re
import shutil
import tempfile
from .core import (
    PROCEDURES,
    REPO,
    ROLLOUT_DOC,
    Trial,
    fail,
    note,
    now,
    run,
    sha256_file,
    stamp,
)
from .interaction import (
    ask_yes,
    human,
)


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


def procedure_record(name, checkpoints, checks, performed_by="person"):
    section = PROCEDURES[name][3:]
    if performed_by == "recorder":
        # The recorder ran the section's commands itself; the person did not
        # see them, so they are not asked to vouch for them. The checks are
        # the evidence.
        followed, remarks = True, ""
    else:
        followed = ask_yes(f'Did you follow "{section}" in {ROLLOUT_DOC} as written, and did it match what happened?')
        remarks = "" if followed else human("What differed?").strip()
    return {"validated_at": stamp(), "document": ROLLOUT_DOC, "section": PROCEDURES[name], "performed_by": performed_by,
            "section_sha256": section_digest(PROCEDURES[name]), "followed_as_written": followed,
            "remarks": remarks, "checkpoints": checkpoints, "checks": checks}


def restore_snapshot_contents(snapshot, destination):
    """Copy the workspace bytes, leaving only the root manifest behind."""
    def ignore_manifest(directory, names):
        return {"snapshot.json"} if os.path.realpath(directory) == snapshot and "snapshot.json" in names else set()

    shutil.copytree(snapshot, destination, ignore=ignore_manifest)


def cmd_procedure(args):
    trial = Trial.current()
    name = args[0] if args else ""
    earlier = trial.state["procedures"].get(name)
    if earlier:
        trial.state.setdefault("superseded_procedures", []).append({"procedure": name, "superseded_at": stamp(), "record": earlier})
    if name == "backup":
        backup_root = os.path.join(trial.dir, "backups")
        # The recorder's tt closes after every step, so no commit is in flight.
        if not trial.mac_exchange():
            fail("Mac exchange failed before the backup")
        observation = trial.mac_observation()
        # local_transactions is the full authored history, including genesis;
        # only outstanding work and held imports prevent a settled backup.
        if (observation["outstanding_up"] or observation["outstanding_down"] or
                observation["held"]):
            fail("Mac still has sync work waiting; finish the exchange before backing up")
        created = trial.tt("workspace", "backup", backup_root, "--data-dir", trial.working)
        match = re.search(r"^Backup verified: (.+) \(\d+ files\)$", created.stdout, re.M)
        if not match:
            fail("tt workspace backup did not report its snapshot path")
        backup = os.path.realpath(match.group(1))
        expected_root = os.path.realpath(os.path.join(backup_root, trial.state["workspace"]["workspace_id"]))
        if os.path.dirname(backup) != expected_root:
            fail("tt workspace backup reported a snapshot outside the trial backup root")
        verified = trial.tt("workspace", "verify-backup", backup, check=False)
        checks = {"backup": os.path.relpath(backup, os.path.realpath(trial.dir)),
                  "verified_after_creation": verified.returncode == 0,
                  "matches_working_copy": digest_tree(backup) == digest_tree(trial.working)}
        trial.state["procedures"]["backup"] = procedure_record("backup", [], checks, performed_by="recorder")
    elif name == "recovery":
        backup_record = trial.state["procedures"].get("backup", {})
        backup_rel = backup_record.get("checks", {}).get("backup")
        if not backup_rel:
            fail("record the backup procedure first")
        trial_dir = os.path.realpath(trial.dir)
        backup = os.path.realpath(os.path.join(trial_dir, backup_rel))
        if os.path.commonpath([trial_dir, backup]) != trial_dir:
            fail("recorded backup is outside the qualification trial")
        verified = trial.tt("workspace", "verify-backup", backup, check=False)
        if verified.returncode != 0:
            fail("recorded backup failed verification before the restore drill")
        if not trial.mac_exchange():
            fail("Mac exchange failed before the recovery drill")
        mac_digest = trial.mac_observation()["digest"]
        # A replacement Mac: a new HOME is a new device to tt. Join, then
        # exchange until nothing more arrives, as the guide says.
        rejoined = os.path.join(trial.dir, "recovered", now().strftime("%Y%m%dT%H%M%SZ"))
        home = rejoined + "-home"
        os.makedirs(home)
        tt = os.path.join(trial.dir, "bin", "tt")
        joined = run([tt, "sync", "join", "--exchange", trial.exchange, "--working-copy", rejoined],
                     env={"HOME": home}, check=False, log=os.path.join(trial.dir, "tt.log"))
        for _ in range(5):
            passed = run([tt, "sync", "exchange", "--data-dir", rejoined, "--json"], env={"HOME": home}, check=False,
                         log=os.path.join(trial.dir, "tt.log"))
            if passed.returncode != 0 or not json.loads(passed.stdout or "{}").get("imported"):
                break
        rejoined_digest = synced_digest(trial, rejoined, home)
        # The same Mac, from the backup: move the working copy aside, put the
        # backup at its path, check it there, then put the working copy back
        # and prove it is unchanged.
        aside = trial.working + ".recovery-aside"
        before = trial.mac_observation()["digest"]
        if digest_tree(backup) != digest_tree(trial.working):
            fail("working copy changed since the backup; record a fresh backup before the restore drill")
        os.rename(trial.working, aside)
        try:
            restore_snapshot_contents(backup, trial.working)
            restored_doctor = trial.tt("doctor", "--data-dir", trial.working, check=False)
            restored_digest = synced_digest(trial, trial.working)
        finally:
            shutil.rmtree(trial.working, ignore_errors=True)
            os.rename(aside, trial.working)
        checks = {"rejoined_from_exchange": joined.returncode == 0,
                  "rejoined_same_tasks": task_records(rejoined) == task_records(trial.working),
                  "rejoined_byte_identical": rejoined_digest == mac_digest,
                  "backup_verified_before_restore": verified.returncode == 0,
                  "restored_backup_passes_doctor_in_place": restored_doctor.returncode == 0 and "escapes approved roots" not in restored_doctor.stdout,
                  "restored_backup_digest": restored_digest,
                  "working_copy_unchanged_after_drill": trial.mac_observation()["digest"] == before}
        trial.state["procedures"]["recovery"] = procedure_record("recovery", [], checks, performed_by="recorder")
    elif name == "troubleshooting":
        doctor = trial.tt("sync", "doctor", "--data-dir", trial.working, "--json", check=False)
        diagnostics = trial.tt("sync", "diagnostics", "--data-dir", trial.working, check=False)
        checks = {"sync_doctor_passes": doctor.returncode == 0,
                  "diagnostics_export": diagnostics.returncode == 0 and '"workspace_digest"' in diagnostics.stdout,
                  "revoked_access_recorded": "revoked_access" in trial.state["scenarios"]}
        trial.state["procedures"]["troubleshooting"] = procedure_record("troubleshooting", [], checks, performed_by="recorder")
    elif name in ("enrollment", "upgrade"):
        fail(f"{name} is recorded by `start` and `scenario upgrade`")
    else:
        fail("usage: procedure backup|recovery|troubleshooting")
    trial.save()
    note(f"procedure {name} recorded: {trial.state['procedures'][name]['checks']}")


def synced_digest(trial, directory, home=None):
    """The synced-file digest tt reports for an enrolled directory, read under
    a HOME of its own (or the device's own HOME) so this machine's identity
    and selection stay as they are."""
    scratch = None if home else tempfile.mkdtemp(dir=trial.dir, prefix="digest-home.")
    try:
        result = run([os.path.join(trial.dir, "bin", "tt"), "sync", "diagnostics", "--data-dir", directory],
                     env={"HOME": home or scratch}, check=False, log=os.path.join(trial.dir, "tt.log"))
        return json.loads(result.stdout).get("workspace_digest") if result.returncode == 0 else None
    finally:
        if scratch:
            shutil.rmtree(scratch, ignore_errors=True)


def task_records(directory):
    """Every task heading with its metadata line, per project, ignoring the
    order tasks are stored in: what two copies must agree on even when their
    stored order differs."""
    out = {}
    projects = os.path.join(directory, "projects")
    for slug in sorted(os.listdir(projects)):
        path = os.path.join(projects, slug, "tasks.md")
        if not os.path.exists(path):
            continue
        lines = open(path, encoding="utf-8").read().splitlines()
        out[slug] = sorted(lines[i] + "\n" + lines[i + 1] for i in range(len(lines) - 1) if lines[i].startswith("## Task: "))
    return out
