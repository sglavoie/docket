[← Mobile contracts](README.md)

# Unattended simulator qualification

This is the default gate for iPhone implementation work. It runs the real app
and Go engine on an iOS simulator, requires no Apple Account or signing identity,
and asks no questions. Use the MacBook Pro over SSH for UI runs, following the
repository's UI-testing permission rule. Existing permission for the same work
still applies.

## Run and resume

From the repository on the MacBook, with Xcode, an iOS simulator runtime, Go,
Python 3 and `just` available:

```sh
just ios-qualify

# Start a durable worker before disconnecting SSH.
just ios-qualify start ios/.cache/qualification
just ios-qualify status ios/.cache/qualification

# Resume after a failed/interrupted worker, from the same source tree.
just ios-qualify resume ios/.cache/qualification
```

`run` waits and returns a failing exit code on any failed stage. `start` returns
when it launches the worker; it is not a passing result. Follow `worker.log`
and `qualification.json` in the output directory. `status` reports an
interrupted worker when its lock is no longer held. One qualification worker
may use this user's simulators at a time.

Use an ignored directory (such as `ios/.cache/qualification`) or a directory
outside the checkout. After changing source or toolchain, give the next run a
new output directory. Resume only reuses passing stages whose recorded artifact
hashes still match. Attempts retain separate logs and result bundles. A retried
upgrade repeats both halves because verification consumes the saved draft.

The runner prevents idle sleep while active. Keep the Mac logged in and
reachable when the agent polls it; closing the laptop lid can still suspend it.
An SSH disconnect does not terminate a detached worker. A reboot interrupts the
current stage; resume reruns that stage. No task completion depends on somebody
replying to a terminal prompt.

## What runs

| Stage | Evidence |
| --- | --- |
| Swift suites | Shared capture/reminders, simulator bridge and hosted app tests, transport and join contracts |
| App UI suite | Browsing, editing, conflicts, rich content, accessibility and delivered notification actions, plus the simulator qualification cases |
| Previous app | A pinned previous revision saves a task and an unfinished draft through its UI |
| Current app | Installs the current build, reopens the same disposable data, verifies both values and saves the draft exactly once |
| Cross-machine compatibility | Pinned older CLI, migration, refusal and concurrent-write cases, with no skips allowed |
| Backup verification | Snapshot integrity, source changes, tampering and unsafe destinations |

The new app cases exercise offline saves across termination, unavailable
accounts, revoked folder access, withheld blobs, repeated delivery, and a
64 MiB attachment interrupted after staging. A separate real engine in the UI
runner acts as the peer. Both processes use the production bridge, transport,
transactions and conflict logic. Diagnostics compare workspace digests and
check task counts, queues and conflicts. Existing conflict UI tests choose a
version, publish it, and test stale-resolution refusal.

The upgrade baseline is pinned in `ios/scripts/simulator_qualification.py`.
Only the upgrade test target and its harness are added to that archived
revision. Its app and engine sources remain unchanged. Both executable hashes
are recorded, and the upgrade fails if the installed builds are identical.
The current app opens the same selected workspace and draft files produced by
the previous app; the test never manufactures those expected writes.

## Controlled failures and results

A Debug simulator build can wrap its real coordinated-folder provider when
`TT_UITEST_ROOT` and `TT_UITEST_SYNC_CONTROL=1` are supplied. The test atomically
writes `exchange-control.json` under that isolated root. It controls access
failure and withheld reads; it cannot fabricate a successful engine operation.
Invalid controls fail the exchange. The wrapper is absent from Release and
physical-device builds.

The runner writes `qualification.json` with source and toolchain fingerprints,
stage status and artifact hashes. UI stages retain `.xcresult` bundles,
`summary.json`, `tests.json`, screenshots and accessibility trees. Disposable
workspaces and diagnostics remain under each stage's output for investigation.
Missing, empty, skipped or partially passed test runs fail the gate. Sources
changing during a run invalidate it.

These are **simulator results with controlled delivery**. They do not certify
Apple's iCloud service, physical-device signing, notification delivery on real
hardware, or a week of daily use. Withheld files are released by the test;
no ten-minute real-world wait is claimed. Real-device evidence remains in the
separate, explicitly requested [physical trial](personal-rollout.md#qualification-trial).
