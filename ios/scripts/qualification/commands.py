"""Trial lifecycle, enrollment, daily use, and evidence export commands."""

import datetime
import json
import os
import shutil
import sys
from .core import (
    FIXTURE,
    FORMAT,
    PROCEDURES,
    REPO,
    ROLLOUT_DOC,
    SCENARIOS,
    Trial,
    fail,
    note,
    now,
    qualification_root,
    run,
    stamp,
)
from .mac import (
    mac_capture,
)
from .interaction import (
    ask_yes,
    human,
    say,
    wait_enter,
)
from .checkpoints import (
    checkpoint,
)
from .builds import (
    app_executables,
    build_mac,
    device_info,
    record_install,
    signing,
)
from .procedures import (
    digest_tree,
    procedure_record,
)


def cmd_start(_args):
    if sys.platform != "darwin":
        fail("a device qualification needs macOS")
    team = os.environ.get("DOCKET_IOS_DEVELOPMENT_TEAM")
    if not team:
        fail("set DOCKET_IOS_DEVELOPMENT_TEAM to the signing team")
    root = qualification_root()
    pointer = os.path.join(root, "current")
    if os.path.exists(pointer) and not os.environ.get("DOCKET_QUALIFICATION_RESTART"):
        fail(f"a trial is already running ({open(pointer).read().strip()}); finish it, or set DOCKET_QUALIFICATION_RESTART=1")
    run_id = now().strftime("%Y%m%dT%H%M%SZ")
    directory = os.path.join(root, run_id)
    exchange_root = os.environ.get("DOCKET_QUALIFICATION_EXCHANGE") or os.path.expanduser(
        "~/Library/Mobile Documents/com~apple~CloudDocs/tt-qualification")
    if "/Library/Mobile Documents/" not in exchange_root:
        fail(f"{exchange_root} is not inside iCloud Drive")
    exchange = os.path.join(exchange_root, run_id)
    iphone = device_info(os.environ.get("DOCKET_IOS_DEVICE"))
    os.makedirs(os.path.join(directory, "bin"))
    os.makedirs(os.path.join(directory, "home"))
    log = os.path.join(directory, "build.log")

    note(f"trial {run_id} in {directory}")
    note("building tt for the Mac")
    tt_sha = build_mac(REPO, os.path.join(directory, "bin", "tt"))
    note("building, signing, and installing the iPhone app as the rollout guide does")
    app = run(["ios/scripts/install-device.sh"], env={"DOCKET_IOS_DEVICE": iphone["udid"]}, log=log).stdout.strip()
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
            "installs": [],
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
    record_install(trial, executables, "start", at=signed["installed_at"])

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


def cmd_enrollment(_args):
    trial = Trial.current()
    say(f"""
The trial workspace should already be joined on the iPhone, from iCloud Drive/tt-qualification/{trial.run_id}.
If tt says the exchange has not delivered the workspace yet, long-press that folder in Files,
choose Keep Downloaded, and join again.""")
    record_enrollment(trial)


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
    evidence = os.environ.get("DOCKET_QUALIFICATION_EVIDENCE") or os.path.join(REPO, "docs/mobile/evidence/session-18-qualification.json")
    os.makedirs(os.path.dirname(evidence), exist_ok=True)
    with open(evidence, "w") as handle:
        json.dump(state, handle, indent=2, sort_keys=True)
        handle.write("\n")
    note(f"wrote {evidence}; bash scripts/verify_mobile_session.sh 18 validates it")
    if os.environ.get("DOCKET_QUALIFICATION_KEEP") != "1":
        os.remove(os.path.join(qualification_root(), "current"))
