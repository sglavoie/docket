"""Guided sync scenarios and resumable scenario recording."""

import datetime
import os
import signal
import subprocess
import time
from .core import (
    DELAY_MIN_SECONDS,
    LARGE_MAC_BYTES,
    LARGE_MIN_BYTES,
    ROLLOUT_DOC,
    SCENARIOS,
    Trial,
    fail,
    note,
    now,
    run,
    sha256_file,
    stamp,
)
from .mac import (
    mac_capture,
    mac_rename,
)
from .interaction import (
    _pending,
    ask_yes,
    human,
    say,
    wait_enter,
)
from .checkpoints import (
    checkpoint,
)
from .builds import (
    build_app,
    build_mac,
    install,
    install_current,
)
from .procedures import (
    procedure_record,
)


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
and complete one existing task. Once you confirm, the Mac captures its own task.""")
    # The TUI exchanges automatically: wait until the phone is offline before
    # creating the Mac transaction used to prove isolation.
    wait_enter()
    if trial.marker_counts(mac)[mac[0]] == 0:
        mac_capture(trial, mac[0])
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
    # Small exchanges can finish in less than 200 ms on a fast Mac.
    for delay in (0.02, 0.05, 0.1, 0.2, 0.5, 1.0):
        process = subprocess.Popen([os.path.join(trial.dir, "bin", "tt"), "sync", "exchange", "--data-dir", trial.working],
                                   env={**os.environ, "HOME": trial.home}, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(delay)
        if process.poll() is None:
            process.send_signal(signal.SIGKILL)
        if process.wait() == -signal.SIGKILL:
            interrupted += 1
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
    if RESUME_AFTER_IPHONE:
        # The person already did this step for an earlier, interrupted run;
        # the delivered checkpoint still has to find the task exactly once.
        _pending.clear()
        note(f"resuming: {phone!r} already captured on the iPhone with Airplane Mode on")
    else:
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
Revoked access, on the iPhone:
  1. Settings > your name > iCloud > Drive, and turn off Sync this iPhone.
  2. Open tt and capture a task titled exactly {phone!r}.
  3. Read the banner tt shows. Keep Sync this iPhone off for now.""")
    wait_enter()
    if trial.marker_counts([mac])[mac] == 0:
        mac_capture(trial, mac)
    revoked = checkpoint(trial, s, "revoked", [phone, mac],
                         {"converged": False, "present": [mac], "absent": [phone], "isolated": True,
                          "iphone_unreachable": True})
    banner = ask_yes("Did tt say your work is saved on the iPhone and offer Retry?")
    say(f"""
Restore access on the iPhone:
  1. Settings > your name > iCloud > Drive, and turn Sync this iPhone back on.
  2. In Files, long-press iCloud Drive > tt-qualification > {trial.run_id} and choose Keep Downloaded.
  3. Open tt. It should reopen the trial workspace by itself. If it asks you to choose a
     workspace, pick the existing one from the list (not Join from an Exchange Folder…).
  4. Turning sync off removed tt's permission to the folder, and Retry cannot renew it: tap the
     sync status button, then Choose Exchange Folder…, pick iCloud Drive > tt-qualification >
     {trial.run_id}, tap Open, then Exchange Now.""")
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
open that attachment there. Leave both attachments in place: don't remove either.""")
    mac_capture(trial, f"{mac} &{blob}")
    wait_enter()
    record = checkpoint(trial, s, "delivered", [mac, phone], {"mac_attachment": sha256_file(blob)},
                        attachments_min=LARGE_MIN_BYTES)
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
    old_tt = os.path.join(trial.dir, "bin", "tt-old")
    if RESUME_INSTALLED_OLD:
        # An interrupted run already installed the earlier app on the iPhone
        # and built the earlier tt. Its signed digests were not kept and a
        # rebuild re-signs, so they are not recreated: the earlier app proves
        # itself by its export, which predates the qualification fields.
        if not os.path.exists(old_tt):
            fail(f"no earlier tt at {old_tt} to resume with")
        built = run([old_tt, "--version"], check=False).stdout.strip()
        note(f"resuming with the earlier app already on the iPhone and {old_tt} ({built or 'no version'})")
        old_executables = {}
    else:
        source = os.path.join(trial.dir, "old-source")
        if os.path.exists(source):
            run(["git", "worktree", "remove", "--force", source], check=False)
        run(["git", "worktree", "add", "--detach", source, commit])
        try:
            log = os.path.join(trial.dir, "build-old.log")
            note(f"building tt and the app at {commit[:12]}")
            build_mac(source, old_tt)
            app, old_executables = build_app(source, trial.state["iphone"]["udid"], team, "ios/.cache/QualificationDerivedData", log)
            install(trial, app)
        finally:
            run(["git", "worktree", "remove", "--force", source], check=False)
        if any(old_executables == i["app_executables"] for i in trial.state["builds"].get("installs", [])):
            fail(f"{ref} builds the same app as the trial: pick the commit of an earlier build")
    wrapper = os.path.join(trial.dir, "bin", "tt-trial-old")
    with open(wrapper, "w") as handle:
        handle.write(f"#!/bin/sh\nHOME='{trial.home}' exec '{old_tt}' --data-dir '{trial.working}' \"$@\"\n")
    os.chmod(wrapper, 0o755)
    old_phone, old_mac = trial.marker(s, "iphone-old"), trial.marker(s, "mac-old")
    pending, new_mac = trial.marker(s, "iphone-pending"), trial.marker(s, "mac-new")
    say(f"""
{"(Already relayed: finish those steps, then reply done.)" if RESUME_INSTALLED_OLD else ""}The earlier build is installed on the iPhone. In this order:
  1. Turn Airplane Mode OFF (Wi-Fi on), open tt, and capture {old_phone!r}.
  2. Turn Airplane Mode ON, then capture {pending!r}, so work is waiting
     to publish across the upgrade.
  3. Leave Airplane Mode on.""")
    mac_capture(trial, old_mac, binary=old_tt)
    wait_enter()
    before = checkpoint(trial, s, "before", [old_phone, old_mac, pending],
                        {"converged": False, "present": [old_mac], "absent": [pending],
                         "iphone_build": None, "mac_binary": old_tt, "earlier_build": True})
    note("installing the current build over it")
    install_current(trial, "upgrade")
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


RESUME_AFTER_IPHONE = False
RESUME_INSTALLED_OLD = False


def cmd_scenario(args):
    global RESUME_AFTER_IPHONE
    global RESUME_INSTALLED_OLD
    RESUME_AFTER_IPHONE = "--resume-after-iphone" in args
    RESUME_INSTALLED_OLD = "--resume-installed-old" in args
    attempt, mac_attempt = 1, None
    for a in args:
        if a.startswith("--attempt="):
            attempt = int(a.split("=", 1)[1])
        if a.startswith("--mac-attempt="):
            mac_attempt = int(a.split("=", 1)[1])
    args = [a for a in args if not a.startswith("--")]
    if not args or args[0] not in SCENARIO_RUNNERS:
        fail("usage: scenario " + "|".join(SCENARIOS) + " [--attempt=N] [--resume-after-iphone]")
    trial = Trial.current()
    name = args[0]
    if attempt > 1:
        trial.attempts[name] = attempt
    if mac_attempt:
        trial.mac_attempts[name] = mac_attempt
    records, observations = SCENARIO_RUNNERS[name](trial)
    if attempt > 1:
        observations["attempt"] = attempt
    if mac_attempt:
        observations["mac_attempt"] = mac_attempt
    remarks = human("Anything unexpected in this scenario? Reply 'none' if nothing.").strip()
    if remarks.lower().rstrip(".") in ("none", "nothing", "no"):
        remarks = ""
    # A rerun supersedes the earlier attempt's records but keeps them, so the
    # evidence shows every attempt and why it was repeated.
    earlier = [c for c in trial.state["checkpoints"] if c["scenario"] == name]
    if earlier or name in trial.state["scenarios"]:
        trial.state.setdefault("superseded", []).append(
            {"scenario": name, "superseded_at": stamp(), "record": trial.state["scenarios"].get(name), "checkpoints": earlier})
    trial.state["checkpoints"] = [c for c in trial.state["checkpoints"] if c["scenario"] != name] + records
    trial.state["scenarios"][name] = {"recorded_at": stamp(), "checkpoints": [r["label"] for r in records],
                                      "observations": observations, "remarks": remarks}
    trial.save()
    failed = [r["label"] for r in records if r.get("problems")]
    if failed:
        fail(f"{name} recorded with failed checkpoints {failed}; rerun it after fixing the cause")
    note(f"{name} recorded")
