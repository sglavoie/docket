[← Mobile contracts](README.md)

# Personal rollout: Mac and iPhone

This is how to install the signed iPhone app, enroll a workspace, keep backups,
recover, troubleshoot, and upgrade. It also covers the qualification trial that
must pass before you enroll your real workspace. Every procedure below was
written to be followed as is: the trial records each section's text, and
`bash scripts/verify_mobile_session.sh 18` refuses the evidence once a
validated section changes.

## Before you start

You need a Mac with Xcode, an iPhone paired with it over a cable, the same
Apple Account signed into iCloud Drive on both, and an Apple Developer team
that can sign apps with the App Group `group.io.github.sglavoie.docket`. Your team
ID appears under Membership details in the developer account. A free personal
team's signature expires after seven days, so with one, reinstall before then
(see [Upgrade](#upgrade)); the app's data stays put.

Install the app on the iPhone, unlocked and connected:

```sh
DOCKET_IOS_DEVELOPMENT_TEAM=TEAMID ios/scripts/install-device.sh
```

The script builds the app for that iPhone, checks the signature, and installs
it with `devicectl`. The first time, the iPhone may ask you to trust the
developer under Settings > General > VPN & Device Management.

On the Mac, build and install `tt` with `just install`.

## Qualification trial

For routine implementation checks, run the [unattended simulator
qualification](simulator-qualification.md). It requires no phone interaction or
Apple Account and is the default development gate. The physical-device trial
below is a separately requested check of actual iCloud and device behavior;
simulator results never stand in for its evidence.

Run this once per build you plan to rely on, before enrolling your real
workspace. It uses a disposable copy of the representative fixture, a trial
directory under `~/.tt-qualification/`, an exchange folder of its own in
`iCloud Drive/tt-qualification/`, and on the iPhone a joined workspace beside
any other. Nothing you already have is read or written. On the Mac you edit the
trial with the `tt-trial` command the script writes into the trial directory.

```sh
export DOCKET_IOS_DEVELOPMENT_TEAM=TEAMID
ios/scripts/record-device-qualification.sh start          # signed install, enrollment, join
ios/scripts/record-device-qualification.sh scenario airplane_mode
ios/scripts/record-device-qualification.sh scenario simultaneous_edits
ios/scripts/record-device-qualification.sh scenario termination
ios/scripts/record-device-qualification.sh scenario delayed_delivery
ios/scripts/record-device-qualification.sh scenario revoked_access
ios/scripts/record-device-qualification.sh scenario large_attachments
DOCKET_QUALIFICATION_UPGRADE_FROM=<earlier commit> \
  ios/scripts/record-device-qualification.sh scenario upgrade
ios/scripts/record-device-qualification.sh procedure backup
ios/scripts/record-device-qualification.sh procedure recovery
ios/scripts/record-device-qualification.sh procedure troubleshooting
ios/scripts/record-device-qualification.sh day             # each day, for at least 7 days
ios/scripts/record-device-qualification.sh status
ios/scripts/record-device-qualification.sh finish          # writes the evidence
```

Each step tells you what to do on each device. Then it asks you to export
diagnostics on the iPhone and reads them over the cable. It records both
devices' digests of their synced files, unpublished work, open conflicts, the
build the iPhone ran, and how many copies of each step's marker task the Mac
holds. A checkpoint that does not hold can be retried, because iCloud may take
minutes. If you decline a retry, the checkpoint is recorded as failed.

For the daily use, work in the trial on both devices every day for at least a
week, then run `day`. It asks whether an edit went missing or the devices
showed a difference you could not explain. A yes fails the trial.

`finish` writes `docs/mobile/evidence/session-18-qualification.json`. The
evidence is tied to the Swift sources, the bridge, and `tt` it was recorded
with, so keep the tree unchanged for the whole trial. Remove the trial
afterwards: on the iPhone, choose another workspace (the trial's stays
listed), and on the Mac delete its directory under `~/.tt-qualification/` and
its folder in `iCloud Drive/tt-qualification/`.

## Enrollment

Enroll on the Mac first. Quit `tt`, then:

```sh
tt sync enroll --exchange ~/Library/Mobile\ Documents/com~apple~CloudDocs/tt-exchange --working-copy ~/tt-synced
```

This saves a verified snapshot of your workspace under `~/.tt/enrollments/`,
builds the working copy at `--working-copy` (outside iCloud Drive), selects
it, and publishes its history to the exchange folder. The workspace you
enrolled from is left as it was. If the command is interrupted, run it again;
it finishes without copying anything twice. `tt sync status` should now say the
workspace is enrolled.

On the iPhone, open tt. If it already shows a workspace, open the sync status
button and tap **Choose Another Workspace…**; the workspace you leave is kept.
Tap **Join from an Exchange Folder…** and pick the same exchange folder. The
phone downloads the history and opens the joined workspace. When the sync
sheet says **Exchange reachable** with nothing waiting, both devices hold the
same workspace.

A folder the Mac just created may not have reached the iPhone yet. If the join
says the folder holds no tt exchange, or that the exchange has not delivered the
workspace yet, open the Files app, long-press the exchange folder, and choose
**Keep Downloaded**. Wait until its files lose their cloud icons, then join
again.

A second Mac joins with
`tt sync join --exchange DIR --working-copy DIR`.

Keep `tt` running on the Mac to exchange automatically: it syncs at launch,
on focus, and every five seconds, including while Docket.app's window is hidden.
The iPhone exchanges at launch, on foreground, after saves, and on refresh.
An offline folder is retried automatically on the Mac. iCloud delivery can
take longer; `tt sync status` shows what is still waiting, and `tt sync exchange`
remains available for a manual pass while `tt` is closed.

## Backup

The working copy on the Mac is the copy to back up. The exchange folder holds
history, not a workspace you can open. Take a backup right after an exchange
leaves nothing waiting, with `tt` quit so no commit is in flight:

```sh
tt sync exchange && tt sync status
tt workspace backup ~/Backups/tt
```

`tt sync status` should show nothing outstanding. Backup prints its timestamped
snapshot path and a `tt workspace verify-backup PATH` command. Run that command
again after copying the snapshot to another drive: it checks every file against
the hash manifest without touching the live workspace. A backup is only meant
to be restored at the working copy's own path: its undo history records that
path. Hash verification proves the copy is intact; a restore rehearsal checks
that the workspace also opens and syncs correctly. The snapshot under
`~/.tt/enrollments/` is the workspace as it was before enrolling; keep it until
you no longer need to go back.

The iPhone needs no separate backup for work it has published, because joining
again rebuilds it from the exchange. Work it has not published yet, shown as
waiting in the sync sheet, exists only on the phone and in the phone's own
iCloud or computer backup. Let it publish before you erase or replace the
phone.

## Recovery

**An interrupted commit or exchange.** You do not need to do anything. Each
device finishes or rolls back an interrupted commit when it next opens the
workspace, and an exchange stopped midway resumes on the next pass without
importing anything twice.

**A replacement Mac, or a Mac whose working copy is lost.** On a new Mac (or a
new macOS user account, which is a different device to `tt`), install `tt` and
join from the exchange folder:

```sh
tt sync join --exchange ~/Library/Mobile\ Documents/com~apple~CloudDocs/tt-exchange --working-copy ~/tt-synced
```

Then run `tt sync exchange` once: joining builds the working copy from the
history it reads in one pass, and the first exchange brings in the rest. The
new working copy then holds every task every device had published. It may
store some tasks in a different order than your other devices, so its files are
not byte-for-byte identical to theirs. Work the old Mac never published is lost. `tt sync
join` refuses on the same Mac and user account, because that device already
publishes to the folder and each device keeps one working copy.

**The same Mac, from a backup.** Restoring a backup over the working copy is
safe only if the Mac committed nothing after the backup was taken. Its history
then matches what it already published. Quit `tt`, move the damaged copy
aside, copy the verified snapshot's contents back to that path while excluding
its root `snapshot.json` manifest, and run `tt sync exchange` to fetch what
the other devices published since. If the Mac did commit after the backup, do
not restore it: the restored history would number new changes the way already
published ones are numbered. Keep the damaged copy and the exchange folder as
they are, and use a replacement device as above.

**The iPhone.** If the app's data is lost, install it again, then join from
the exchange folder as in [Enrollment](#enrollment). If the joined workspace
misbehaves, export diagnostics first (see [Troubleshooting](#troubleshooting)),
then choose another workspace and join again. The old one stays listed until
you remove the app.

## Troubleshooting

Start with what each device says:

- On the Mac, the Outliner's status strip shows a `sync:` line while anything
  is waiting. `tt sync status` gives the details, and `tt sync doctor` checks the
  working copy's placement, the exchange folder, the history, and which device
  needs an update.
- On the iPhone, the sync status button opens the sync sheet. Its status line
  reads:
  - **Exchange reachable**: working.
  - **iCloud is unavailable; changes stay on this device**: sign in to iCloud,
    or turn iCloud Drive back on.
  - **Choose the exchange folder again**: the app lost access to the folder.
    Tap **Choose Exchange Folder…**, pick the same folder, and the queued work
    publishes. This always happens after Sync this iPhone (Settings > your
    name > iCloud > Drive) has been off; **Retry** cannot renew the access.
  - **Waiting for the exchange folder to download**: iCloud has not delivered
    it yet. Leave tt open on Wi-Fi. In the Files app, long-press the exchange
    folder and choose **Keep Downloaded** so iCloud keeps every file on the
    phone.
  - **That folder belongs to another workspace**: you picked the wrong folder.

  While an exchange fails, a banner says everything is saved on the iPhone and
  offers **Retry**. Nothing is lost while access is missing; it waits.

  If tt asks you to choose a workspace after you change iCloud settings, pick
  the workspace you already have from the list. Do not tap **Join from an
  Exchange Folder…** again: that builds a second copy beside it from whatever
  iCloud has downloaded so far, and while iCloud is still catching up that copy
  can be missing tasks. Your existing workspace keeps everything, including
  work not yet published, and catches up by itself.

A file iCloud has not downloaded yet is reported as unavailable and fetched on a
later pass. A missing file never deletes anything. A change from a newer build
is held, not dropped, until the device that has to read it is upgraded; see
[Upgrade](#upgrade).

When both devices changed the same thing while apart, both versions are kept as
a sync conflict. On the Mac, `gc` lists conflicts, `tt sync conflicts ID` shows
one, and `tt sync resolve ID --version N` keeps a version. On the iPhone, open
the conflict in the sync sheet and choose. Resolving on one device reaches the
other at the next exchange.

To report a problem, export diagnostics from both devices. On the Mac, run
`tt sync diagnostics --file FILE`. On the iPhone, open the sync sheet and tap
**Export Diagnostics** under Recovery, then **Share Diagnostics**. Both name
files, devices, and digests, but no task or note text. When both devices show
the same `workspace_digest`, they hold the same synced files.

## Upgrade

Upgrade either device first. A change written by the newer build is held on the
older device until it is upgraded too, and `tt sync doctor` names the device
that needs an update.

- **iPhone.** Connect and unlock it, then run
  `DOCKET_IOS_DEVELOPMENT_TEAM=TEAMID ios/scripts/install-device.sh`. Installing
  over the earlier build keeps the app's data, including work not yet
  published, which publishes once the new build opens.
- **Mac.** Quit `tt`, run `just install` from the new checkout, and start `tt`.

After both are upgraded, the sync sheet and `tt sync status` should show nothing
waiting and nothing held. A new build may raise the workspace's schema, and
then the earlier build refuses to open it rather than write around it, so
upgrade every device that opens the workspace.

## Production enrollment

Enroll your real workspace only after both of these pass on the tree you
are about to install:

```sh
just ci
bash scripts/verify_mobile_session.sh 18
```

The second command runs the mobile automated suites and holds the recorded
trial and the earlier device evidence to the current builds. Then install from
that tree as described in [Before you start](#before-you-start), and follow
[Enrollment](#enrollment) with your real workspace and a new exchange folder.
Do not reuse the trial's folder.
