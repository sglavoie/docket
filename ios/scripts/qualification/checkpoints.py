"""Device checkpoints and validation of their raw observations."""

import datetime
import json
import tempfile
from .core import (
    LARGE_MIN_BYTES,
    note,
    now,
    run,
    stamp,
)
from .interaction import (
    ask_yes,
    say,
    wait_enter,
)
from .builds import (
    current_install,
)


EXPORT = ("On the iPhone, in tt: pull down to refresh, then tap the sync status button and "
          "Export Diagnostics under Recovery. Keep the iPhone unlocked and connected by cable.")


def checkpoint(trial, scenario, label, markers=(), expect=None, attachments_min=None, rounds_hint=""):
    """Exchanges the Mac, asks the person to refresh and export on the iPhone,
    and records both devices until the expectation holds or the person stops
    retrying. Returns the recorded checkpoint."""
    expect = expect or {}
    markers = list(markers)
    previous = None
    while True:
        # An export made after the previous round's prompt also counts: a
        # person often exports first and answers the retry afterwards.
        started = previous or now() - datetime.timedelta(seconds=5)
        previous = now() - datetime.timedelta(seconds=5)
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


def device_reachable(trial):
    """Whether CoreDevice lists the trial's iPhone as connected right now."""
    with tempfile.NamedTemporaryFile(suffix=".json") as out:
        if run(["xcrun", "devicectl", "list", "devices", "--json-output", out.name], check=False).returncode != 0:
            return False
        with open(out.name) as handle:
            devices = json.load(handle)["result"]["devices"]
    for device in devices:
        properties = device.get("properties", {})
        hardware = properties.get("hardware", device.get("hardwareProperties", {}))
        if hardware.get("udid") == trial.state["iphone"]["udid"]:
            connection = properties.get("connection", device.get("connectionProperties", {}))
            state = connection.get("state", connection.get("tunnelState"))
            return state == "connected"
    return False


def checkpoint_problems(trial, record, expect):
    """The same rules the Go validator applies, so a person sees a failure
    while there is still time to retry."""
    problems = []
    mac, iphone = record["mac"], record["iphone"]
    if iphone is None:
        if not device_reachable(trial):
            return ["the Mac cannot reach the iPhone over the cable (it shows as unavailable): unlock it, "
                    "plug the cable in, tap Trust if asked, then retry"]
        return ["no iPhone diagnostics exported since this checkpoint began"]
    if iphone["workspace_id"] != trial.state["workspace"]["workspace_id"]:
        problems.append(f"the iPhone exported workspace {iphone['workspace_id']}, not the trial's")
    builds = expect.get("iphone_build", current_install(trial)["app_executables"])
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
    if "attachments" in record and expect.get("mac_attachment"):
        sent = expect["mac_attachment"]
        mine = [a for a in record["attachments"] if a["sha256"] == sent]
        theirs = [a for a in record["attachments"] if a["sha256"] != sent]
        if not mine:
            problems.append("the Mac's own large attachment is no longer in the Mac workspace: was it removed on a device?")
        if not theirs:
            problems.append(f"no iPhone file of at least {LARGE_MIN_BYTES >> 20} MB has reached the Mac")
    if expect.get("earlier_build") and (iphone.get("app_executables") or iphone.get("digest")):
        problems.append("the iPhone export carries the qualification fields, so it came from the trial's build, not the earlier one")
    if expect.get("iphone_unreachable"):
        transport = iphone["transport"]
        if transport["condition"] == "reachable" and not transport["last_error"]:
            problems.append("the iPhone still reaches the exchange folder")
    return problems
