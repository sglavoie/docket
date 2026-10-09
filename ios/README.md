# tt for iPhone

A native SwiftUI app over the Go engine. Go keeps task rules, persistence, and
queries; Swift calls them through the versioned bridge in
[`docs/mobile/bridge-contract.md`](../docs/mobile/bridge-contract.md). None of
it runs under `just ci`.

## Layout

| Path | Holds |
| --- | --- |
| `TTEngine/` | Swift package: the bridge models and the `TTEngine` actor |
| `TTEngine/Frameworks/TTBridgeCore.xcframework` | the Go engine bound by gomobile (generated, gitignored) |
| `TTMobile/` | the app target |
| `TTReminders/` | Swift package: the reminder scheduler, its reconciliation, and notification action routing, tested against a fake notification center |
| `TTShared/` | Swift package shared with the extensions: the App Group capture inbox, the widget snapshot and its freshness, and `tt://` links |
| `TTShare/` | the share extension, which saves what was shared to the capture inbox |
| `TTWidgets/` | the Today and Inbox widgets, drawn from the App Group snapshot |
| `Config/` | the app's and extensions' Info.plist additions and App Group entitlements |
| `TTMobileUITests/` | the seeded UI suites (browsing, capture and editing, organizing, rich content, conflicts, share and widgets, accessibility), on a simulator against the real engine, and the reminder delivery suite, on a physical iPhone |
| `TTMobileDeviceTests/` | the responder of the real iCloud exchange, hosted in the app on a physical iPhone |
| `../apple/TTSync/` | Swift package shared with the macOS helper: the exchange transport and its contract |
| `project.yml` | XcodeGen spec for `TTMobile.xcodeproj` (generated, gitignored) |
| `scripts/` | the bridge build, project generation, the suites, and the app automation behind the `ios-*` recipes |

## Prerequisites

Xcode with an iOS 18 or later simulator runtime, and Go. Nothing else is
installed by hand: gomobile and gobind are the versions `go.mod` pins as tool
dependencies, built into `ios/.cache/`, and XcodeGen is the release
`macos/XCODEGEN_VERSION` pins, checksummed and cached under `macos/.cache/`.

## Build and test

| Recipe | Does |
| --- | --- |
| `just ios-bridge` | Bind `mobile/ttbridge` into the device and simulator slices |
| `just ios-project` | The bridge, then `TTMobile.xcodeproj` from `project.yml` |
| `just ios-build` | `tt.app` for the simulator, with its result bundle and build diagnostics |
| `just ios-test [unit\|bridge\|sync\|all]` | The Swift suites that are not UI tests, each on its own |
| `just ios-ui-test [CLASS] [default\|small]` | The simulator UI suite, or one class, on the default or small-screen simulator |
| `just ios-run [FIXTURE] [light\|dark] [TEXT_SIZE] [default\|small]` | Launch the app over a fresh copy of a fixture |
| `just ios-reset` | Reseed the running app's workspace and relaunch it |
| `just ios-screenshot [NAME]` | Save what the simulator shows |
| `just ios-export` | Print the running app's semantic state and save it with its log |
| `just ios-stop` | Stop the app `ios-run` launched |

None of them needs Xcode open; see [Automation](#automation).

The app icon and launch artwork reuse the desktop icon's renderer. Regenerate
the committed PNGs with `swift macos/scripts/make-icons.swift --ios` after
changing the artwork. The iPhone app icon fills an opaque 1024-pixel square;
iOS supplies the corner mask and smaller icon sizes. `LaunchIcon` also appears
while the workspace opens, on a background that follows light and dark mode.

`ios/scripts/build-bridge.sh` publishes the XCFramework with a fingerprint of
everything that decides its bytes: the gomobile pin, the Go toolchain, Xcode,
the deployment target, `go.mod` and `go.sum`, the script, and every in-module
source file the bound package compiles for iOS. A run whose fingerprint
matches the published one builds nothing; any change to those inputs rebuilds.
`--fingerprint` prints the current one without building, and
`TT_IOS_FORCE_BRIDGE=1` rebuilds regardless.

`ios/scripts/test-engine.sh OUTPUT_DIR` copies the representative compatibility
fixture into `OUTPUT_DIR`, then opens it, captures and completes a task, and
reopens it on a simulator. `TT_IOS_SIMULATOR` picks the simulator by UDID.
`bash scripts/verify_mobile_session.sh 3` runs that suite and then checks the
resulting Markdown through Go.

`ios/scripts/test-ui.sh OUTPUT_DIR` runs the browsing UI suite. It enrolls a
disposable copy of the fixture with the `tt` this tree builds, so the join test
has an exchange folder to read. Each test then seeds its own root under
`OUTPUT_DIR/work`, and the app reads that root from `TT_UITEST_ROOT` in place
of its container. `bash scripts/verify_mobile_session.sh 10` runs it with the
engine's browsing tests and the transport's join tests. `TT_UITEST_ONLY` names
one test class to run alone; `bash scripts/verify_mobile_session.sh 11` runs
`CaptureEditingUITests` that way, beside the Go tests that hold the desktop and
the bridge to byte-identical capture and editing outcomes, and
`bash scripts/verify_mobile_session.sh 12` runs `OrganizingUITests`, beside the
bridge's single-device and two-device organizing tests, and
`bash scripts/verify_mobile_session.sh 13` runs `RichContentUITests`, beside the
bridge's rich-content tests on one and two devices, and
`bash scripts/verify_mobile_session.sh 14` runs `ConflictRecoveryUITests`. For
it, `test-ui.sh` also enrolls a second fixture copy as two Mac devices (each
under its own `HOME`) that change one task title and one note line while apart,
so `OUTPUT_DIR/conflict/exchange` holds a coupled conflict for the phone to join;
the verifier then applies what the phone published on both Macs and compares
their files with the phone's. Two launch variables stand
in for what XCUITest cannot drive: `TT_UITEST_IMPORT_SOURCE` names the file the
Files picker would have returned, and `TT_UITEST_HOLD_IMPORTS=1` stops each
import after it is staged, as a termination before the commit would.

## Automation

Every recipe writes under `ios/.cache/` (gitignored): results under
`results/`, or `TT_IOS_RESULTS`, and the running app's session under `run/`,
or `TT_IOS_RUN`.

**Suites.** `just ios-test` runs four separately reported suites through
`ios/scripts/test-suites.sh`: `unit` is `swift test` in `TTShared` and
`TTReminders` on the Mac; `bridge` is the `TTEngine` suite on a simulator
against the gomobile-bound engine (`test-engine.sh`); `sync` is the
`apple/TTSync` exchange and join scenarios between real `bridgehost` engines.
`results/<suite>/summary.json` lists each with its status and its executed,
failed, and skipped counts; a suite that executes nothing or skips a test
fails. `just ios-ui-test` runs the UI suite (`test-ui.sh`) and leaves, under
`results/ui-<device>/`, the result bundle, `summary.json` and `tests.json`
(the bundle's structured results, failures included), and every screenshot
and recording under `attachments/` with their `manifest.json`. `small` runs
on a simulator named `tt-small`, an iPhone SE (3rd generation) that
`ios/scripts/simulator.sh` creates on the newest runtime the first time.

**Accessibility.** `AccessibilityUITests` holds the main workflows to the
largest Dynamic Type size, VoiceOver labels on every control, dark mode, a
small screen, a hardware keyboard, and drafts that survive a relaunch under
all of them, with the system's accessibility audits on each screen. Each test
reads the app's semantic state to prove its setting took effect and keeps a
screenshot of what it checked. Issues in system-drawn chrome (the navigation
toolbar, the search field, and content under the translucent tab bar) are
attached rather than failed. On a hardware keyboard, Command-1, -2, and -3
select Today, Inbox, and Browse, Command-N opens capture, Command-S saves it,
and Command-W closes it keeping the draft.

**Running the app.** `just ios-run` builds, boots the simulator, sets its
appearance and text size (`simctl ui`; `accessibility-extra-extra-extra-large`
is the largest), installs the app, and launches it over `run/root`, a fresh
copy of the representative fixture without its one wall-clock deferral (or no
workspace at all, for `empty`), so every run starts from the same state. The
app never touches its real container: `TT_UITEST_ROOT` points it at that root,
and `TT_UITEST_STATE_EXPORT` has it keep `run/state.json` current. That
document (`tt-mobile-state/v1`, described in
`TTMobile/App/AutomationState.swift`) names the phase, the workspace, the tab
and sheet showing, the appearance and text size actually drawn, sync status,
pending captures, unsaved drafts, and the rows Today and Inbox show.
`just ios-export` prints it and saves it under `run/export/` beside the app's
recent unified log and a summary of the session; `just ios-screenshot NAME`
saves `run/screenshots/NAME.png`; `just ios-reset` reseeds and relaunches with
the same options, so a workflow repeats from the same start.

## The app

The first launch offers two choices: start a workspace kept on the iPhone, or
join the one the Mac enrolled. Joining is `tt sync enroll` on the Mac, then
picking that exchange folder. Each workspace lives under
`Application Support/workspaces/<name>`, and the selection survives
relaunches. A workspace an earlier build kept at `Application
Support/workspace` is adopted as `workspace`. **Choose Another Workspace…** in
the sync sheet returns to that choice without touching any files.

Today, Inbox, and Browse are the tabs, and the capture button floats over all
three. Browse holds Next, Due, All, Backlog, and Recent, every project by
population (active, parked, archived), and the saved lenses. Every list is one
engine query with search, filters, and a sort. A project's list follows the
project's outline. A task's detail links its ancestors, subtasks,
dependencies, and dependents.

A task's detail also edits its checklist in place (tick, add, edit, reorder,
delete, promote to a subtask), schedules and acknowledges its check-in, adds
a subtask, indents, outdents, reorders it among its siblings, and moves it to
another project. **Select** on any list picks several tasks for one action:
Today, Next, Backlog, Start, Done, a tag, or a project. A project list's
settings rename, describe, and tag the project, park and archive it after a
confirmation, and delete it after a confirmation that names everything the
delete takes. **New Project** sits in Browse.

Capture takes shorthand in the desktop's capture grammar, one task per line,
previewed as it is typed and committed in one transaction, or a structured
form. A task's detail edits its daily fields (title, description, resume note,
priority, tags, context, pin, review, due, and deferral) and changes its plan
and status, the two halves of the desktop's workflow chord. The engine judges
every save, and a refusal is explained beside the draft it refused. Unsaved
capture and edit drafts are kept per workspace under
`devices/<name>/drafts`, never synced, and come back after a refused save, a
dismissed sheet, or a relaunch. Only a committed save or **Cancel** removes
one.

A task's detail lists its comments and attachments, and a project's settings
list the project's comments and open its notes. **All Notes** in Browse lists
the global notes and every available project's. A comment or a note keeps a
draft the same way, and each change names what was read: a comment edited on
another device refuses the change, and a note saved over another device's edit
is refused, with **Save as New Note** keeping the phone's text beside it.
Attachments import from Files or Photos. The item is copied into
`devices/<name>/attachment-imports` while its grant is valid, recorded with its
digest, and committed by the engine with the task's metadata in one
transaction; no provider URL or staging path is ever stored, and an import a
termination interrupted resumes at the next launch. A file opens in Quick Look
and shares through the system sheet; a link opens and shares; a code reference
is kept for the desktop. A file this iPhone does not hold says so, one still
arriving says **Waiting to download**, and **Download** asks iCloud for it now.

## Reminders

**Reminders** in the sync sheet turns on local notifications. They are off
until turned on, and turning them on is the only thing that asks for
notification permission. The engine plans them (`query.reminders`) under the
same attention rules the desktop's review pass uses: a check-in on its day at
the check-in time chosen there, a deadline that carries a time of day when it
is due, and a deferral when it ends. The app only schedules, replaces, and
cancels what the plan names, after the workspace opens, every commit, every
import of another device's changes, each return to the foreground, a
time-zone or clock change, and a settings change. Closing or switching the
workspace cancels every pending reminder.

A notification offers **Acknowledge** (when the task owes a review),
**Defer 1 Hour**, and **Defer to Tomorrow**, which commit in the background
through the same commands the task's detail uses; tapping it opens the task.
A refused action writes nothing and is listed under **Recent** with the
reason. Denying permission is harmless: nothing is scheduled, the status says
why and links to Settings, and allowing it later schedules at the next
foreground. Times are explicit about zones: check-ins and timed deadlines keep
their local time when the iPhone changes time zone, while a deferral ends at
the same instant everywhere. Settings and the recent-action list are kept
under `Application Support/reminders/` and never synced.

`cd ios/TTReminders && swift test` runs the scheduling and action suite on the
Mac against a fake notification center. Delivery itself is proven on a
physical iPhone: `TT_IOS_DEVELOPMENT_TEAM=… ios/scripts/record-reminder-evidence.sh`
builds the signed app, runs `ReminderDeliveryUITests` there (each test seeds a
disposable workspace in the app's own container through `TT_UITEST_SCRATCH`
and `TT_UITEST_SEED_REMINDER`, waits for the real notification, and acts on it
through SpringBoard), and writes `docs/mobile/evidence/session-15-reminders.json`
naming the source fingerprint, the bridge fingerprint, and the executable
digests the running app reported. `bash scripts/verify_mobile_session.sh 15`
runs the engine and scheduling suites and holds that evidence to the current
tree; a simulator run does not count.

## Share, Shortcuts, and widgets

The app shares the App Group `group.io.github.sglavoie.tt` with its share
extension and widgets ([capture-inbox.md](../docs/mobile/capture-inbox.md)).
**Share → tt** reads the shared words, links, and files, proposes a title and
a note, and saves them to the capture inbox in the container. The **Capture a
Task** shortcut saves its text the same way, read as capture shorthand. Neither
opens the workspace. The app imports every saved capture through the engine
(`capture.import`) when it opens a workspace, returns to the foreground, or
refreshes, and right after a shortcut runs if it can open the workspace then.
Each capture lands exactly once, however often it was delivered or wherever an
import was interrupted. Until a capture is imported, the Inbox lists it under
**Saved, waiting to be added**, and the Inbox tab carries a badge. A capture
the engine refused shows its reason there and can be corrected or discarded.

The **Today** and **Inbox** widgets draw the snapshot the app writes after
every change (`query.widget_snapshot`). They say how old it is, mark it as
possibly out of date after three hours, and stop presenting it as current at
local midnight. Each row opens its task. **Open tt** opens Today, the Inbox, or
capture, and so do the links `tt://view/today`, `tt://view/inbox`,
`tt://capture`, and `tt://task/<workspace>/<project>/<task>`. A task link for
another workspace is refused.

`cd ios/TTShared && swift test` runs the package suite. With `TT_BRIDGEHOST` (a
built `mobile/bridgehost`) and `TT_CAPTURE_FIXTURE` (the representative
fixture), it also runs the contract between the package's writer and the real
engine's importer. `TT_UITEST_ONLY=ShareCaptureUITests ios/scripts/test-ui.sh
OUT` runs the simulator suite. The unsigned UI test build holds no App Group,
so `TT_UITEST_SHARED_CONTAINER` stands one in on the host, and
`TT_UITEST_HOLD_CAPTURES=1` leaves saved captures pending.
`bash scripts/verify_mobile_session.sh 16` runs all of it.

## Sync exchange

The app exchanges through a folder the user picks from the sync status button,
under **Choose Exchange Folder…**. It exchanges at launch, when it returns to the foreground,
after every commit, and on pull to refresh. The transport is
[`apple/TTSync`](../apple/TTSync/Package.swift). Its contract suite runs on the
Mac against the real engine:

```sh
go build -trimpath -o /tmp/bridgehost ./mobile/bridgehost
(cd apple/TTSync && TT_BRIDGEHOST=/tmp/bridgehost swift test)
```

The sync sheet lists the open conflicts from the phone's own history, so they
read with the exchange unreachable. A conflict shows what differs, the version
every side started from, and each competing version with the device and change
that wrote it. **Keep This Version** keeps that side of what conflicted and the
rest of what both devices changed, plus the same change's side of any other
file it touched; **Keep What This iPhone Has Now** and, for tasks and notes,
**Write a Replacement…** resolve it otherwise. A conflict that changed after it
was opened is refused without writing and read again. While the exchange
fails, a banner says everything is saved on the iPhone and offers **Retry**;
the sheet's Recovery section repeats the retry and exports a diagnostics file
(under `devices/<name>/diagnostics/`) to share.

`ios/scripts/record-icloud-exchange.sh` records the same contract between this
Mac and a physical iPhone over iCloud Drive. First install the app on the
iPhone once, open it, and choose `iCloud Drive/tt-sync-evidence` as its exchange
folder. Then run the script with `TT_IOS_DEVELOPMENT_TEAM` set to your signing
team and the iPhone unlocked. Each run uses its own disposable workspace and its
own `runs/<id>` folder. The script rewrites
`docs/mobile/evidence/session-8-icloud-exchange.json`, which
`bash scripts/verify_mobile_session.sh 8` holds to the current sources.

## Device qualification and rollout

`ios/scripts/install-device.sh` builds the app for a physical iPhone, signed by
`TT_IOS_DEVELOPMENT_TEAM`, and installs it over the cable. Installing over an
earlier build keeps the app's data. `ios/scripts/record-device-qualification.sh`
runs the guided real-device trial (airplane mode, simultaneous edits,
termination, delayed delivery, revoked folder access, large attachments, an
upgrade, the rollout procedures, and at least a week of daily use) in a
disposable workspace. It writes `docs/mobile/evidence/session-18-qualification.json`,
which `bash scripts/verify_mobile_session.sh 18` holds to the current builds.
[docs/mobile/personal-rollout.md](../docs/mobile/personal-rollout.md) is the
guide, including when to enroll a real workspace.
