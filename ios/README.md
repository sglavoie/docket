# tt for iPhone

A native SwiftUI app over the Go engine. Go keeps task rules, persistence, and
queries; Swift calls them through the versioned bridge in
[`docs/mobile/bridge-contract.md`](../docs/mobile/bridge-contract.md). None of
it runs under `just ci`.

Capture offers sigil suggestions as you type, including when opened through
the quick-capture link. Two opaque rows above the keyboard offer every sigil;
tap a symbol to insert it at the cursor, then type or choose a suggestion.
Swipe the suggestion row to see more and tap to insert
at the cursor. Tags and contexts come from the workspace; parent and child
references follow the current line's project, and dependencies can cross
projects. Links offer existing URLs or a URL prefix, and comments offer a
placeholder you can type over. The rest of the draft stays intact.

Capture and task editing keep a submission tied to its draft revision. You can
continue typing while a save finishes; newer text stays open and is retained
for recovery. An edit saved again sends only changes made since the previous
submission. Close and swipe-to-dismiss wait for an in-flight save to finish.
Quick previews always belong to the current draft, including after switching
between Quick and Form. A refused save stays visible above the form and takes
you to the named field or capture token when the engine identifies one.
Long errors have a bounded summary so the editor stays usable; Read full error
opens the complete, scrollable explanation.

Task rows show priority and context in their secondary text, wrapping the full
value. The dominant attention cue remains overdue, review/check-in, due today,
then high priority. Sync leads with its current condition and the relevant
folder-choice or retry action. Zero transfer counts and workspace identifiers
live in expandable details below the main status.

Exchange Now shows progress immediately and retries both upload and download
queues without their automatic retry delay. Waiting to download counts changes
whose files have not arrived; Waiting to apply counts downloaded changes the
engine cannot apply yet and shows the reason. Last completed exchange advances
only when both queues are empty. Rejoining creates a new sync identity while
keeping existing working copies intact. Reused transaction numbers report that
sync history needs repair instead of silently skipping changes.

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
| `TTMobileTests/` | app scheduling and attention tests, including structured edits through the real engine on a simulator |
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
| `just ios-install [TEAMID] [UDID]` | Build, sign, install, and launch on a paired iPhone, keeping its app data |
| `just ios-test [unit\|bridge\|app\|sync\|all]` | Swift package, engine, app, and sync tests, each on its own |
| `just ios-qualify [run\|start\|status\|resume] [OUTPUT]` | Unattended simulator qualification, including sync failures, upgrades, compatibility and backup checks |
| `just ios-ui-test [CLASS] [default\|small]` | The simulator UI suite, or one class, on the default or small-screen simulator |
| `just ios-run [FIXTURE] [light\|dark] [TEXT_SIZE] [default\|small]` | Launch the app over a fresh copy of a fixture |
| `just ios-reset` | Reseed the running app's workspace and relaunch it |
| `just ios-screenshot [NAME]` | Save what the simulator shows |
| `just ios-export` | Print the running app's semantic state and save it with its log |
| `just ios-stop` | Stop the app `ios-run` launched |

None of them needs Xcode open; see [Automation](#automation).

Use [unattended simulator qualification](../docs/mobile/simulator-qualification.md)
for routine implementation work. `start` survives an SSH disconnect and
`resume` reuses verified results from the same sources. No Apple Account,
signing identity, or physical iPhone is needed. Physical-device trials are
separate checks and should only be started when explicitly requested.

### Redeploy to your iPhone

Connect and unlock your paired iPhone, then run from the repository:

```sh
just ios-install
```

This rebuilds the Go bridge when its sources changed, regenerates the Xcode
project, builds and verifies the signed app, installs it over the existing
copy, and launches it. Workspaces, drafts, and settings stay in the app's
container. It uses the signing team from the previous device build.

For the first install on this Mac, use `just ios-install TEAMID` (or export
`TT_IOS_DEVELOPMENT_TEAM`). Xcode must have your signing account configured,
and the iPhone must trust this Mac and have Developer Mode enabled. If more
than one iPhone is paired, use `just ios-install TEAMID UDID`; run
`xcrun devicectl list devices` to find its UDID. You can also select it with
`TT_IOS_DEVICE`. The recipe refuses to choose an arbitrary phone.

Build and device logs are kept in `ios/.cache/results/device-install.*/`
(or beneath `TT_IOS_RESULTS`); failures print the log path and its last lines.
An installation failure stops before launch. If installation succeeds but
launch fails, the message says so and you can open tt on the phone.

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

**Suites.** `just ios-test` runs five separately reported suites through
`ios/scripts/test-suites.sh`: `unit` is `swift test` in `TTShared` and
`TTReminders` on the Mac; `bridge` is the `TTEngine` suite on a simulator
against the gomobile-bound engine (`test-engine.sh`); `app` runs the hosted
`TTMobileTests`, including schedule round trips through that engine
(`test-app.sh`); `sync` is the
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
attached rather than failed. On a hardware keyboard, Command-1 through -4
select Today, Inbox, Notes, and Browse, Command-N opens capture, Command-S saves it,
and Command-W closes it keeping the draft.

The 2026-10-17 polish verification passed all six hosted app tests and fourteen
focused UI tests on iOS 27 simulators (iPhone 18 Pro and 375-point iPhone SE).
These cover schedule preservation, Close and confirmed discard, independent
search/filter recovery, empty-view navigation, capture links, every autocomplete
sigil, and all six accessibility workflows. Native screenshots were reviewed
for timed deadlines, task rows, empty Today, dark mode, and the largest text size.
The capture placeholder now has stronger contrast; accessibility audits exclude
content fully hidden behind the keyboard accessory while still checking its
shortcut buttons.

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
In **On this iPhone**, tap the trash button beside an unused workspace, then
confirm **Delete from This iPhone**. This permanently removes its local tasks,
notes, attachments, and unsaved drafts, including changes that have not synced.
The exchange folder and copies on other devices are kept. Cancel leaves the
workspace intact; deleting the last workspace returns you to the start/join
choices.

Today, Inbox, Notes, and Browse are the tabs, and the capture button floats over all
four. Browse holds Next, Due, All, Backlog, and Recent, every project by
population (active, parked, archived), and the saved lenses. Every list is one
engine query with search, filters, and a sort. Search offers tappable
completions for tags, contexts, projects, states, priorities, dates, flags,
attachment presence, computed properties, and dependency references as you type,
including their negated forms. Tags, contexts, projects, dates, and dependency
references come from the current list; selecting a completion preserves the
preceding search and appends a space. A project's list follows the
project's outline. A task's detail links its ancestors, subtasks,
dependencies, and dependents.

Rows lead with the title and one attention cue: overdue, review or check-in,
due today, then high priority. Completed work stays quiet. Supporting state,
plan, pin, and project information is compact; task details retain every tag,
context, and priority, and put the resume note beside the title. VoiceOver
still reads the complete metadata. Empty searches name the query and active
filters and let you clear them independently. An empty Today offers Next;
an empty Next offers Backlog, and an empty Inbox or Due offers Today, keeping
the current project scope.

Long-press a task and drag it to reorder. Today and Next save a workspace-wide
order; Inbox and project lists reorder siblings within the same project, keeping
subtasks with their parent. These lists use manual order by default. All and
Backlog offer **Manual order** in Sort. Clear searches and filters and leave
selection mode before dragging; field-sorted lists, Due, Recent, and saved lenses
keep their query order. Reordering is saved on the iPhone and synced normally.

Swipe left on a task for **Done**, **Start** (or **Pause** while doing), and
**Edit**; completed and canceled tasks offer **Reopen** and **Edit**. Swipe
right for **Today** and **Next**, or remove it from the plan it already has.
The same right swipe offers **Organize**, including on completed and canceled
tasks: choose **Indent**, **Outdent**, **Move Up**, or **Move Down** for that
row. Outdent is disabled for a top-level task. Up and Down change sibling order
in the project's outline; a sorted or filtered list may not show that order.
These four controls live on task rows; task details keep **Add Subtask** and
**Move to Project**.
Swipes reveal buttons without committing an action until tapped. These
shortcuts are available outside selection mode, and an action the engine
refuses is explained above the list. Successful swipe actions name the task
and result in a brief confirmation, announced to VoiceOver, with **Undo** for
that exact transaction. Task detail actions, task edits, and bulk actions use the
same confirmation and Undo; bulk confirmations name the number of tasks affected.
Undoing a project move while reading its task returns the detail to the original
project. A newer action or an intervening edit can make Undo
unavailable; it never reverses a different action instead.

Search names its scope in the field before typing: **Search in Today**, for
example. It stays within that list, unlike the terminal’s ranked search across
modes. An empty scoped or filtered search offers **Search All with this query**:
it opens All across active projects with the same query and without the list's
filters. Back returns to the original search and its position. A transient refresh failure keeps the last successful list and reading
position, names that the results are from before the failure, and offers
**Retry**. Invalid searches and unavailable workspaces replace the old results.
Task details likewise retain the same task after a temporary refresh failure,
with **Retry** above the content. A project picker that cannot load explains the
failure and offers **Retry**, separately from having no destinations.

A task's detail also edits its checklist in place (tick, add, edit, reorder,
delete, promote to a subtask), schedules and acknowledges its check-in, adds
a subtask, indents, outdents, reorders it among its siblings, and moves it to
another project. **Select** on any list picks several tasks for one action:
Today, Next, Backlog, Start, Done, a tag, or a project. A project list's
settings rename, describe, and tag the project, park and archive it after a
confirmation, and delete it after a confirmation that names everything the
delete takes. **New Project** sits in Browse.

**Change Check-In** opens with the saved day and repeat interval. Leave the
interval blank for a one-off, or enter 1–3650 days. Saving an unchanged schedule
leaves it untouched; a refused save keeps the sheet and its fields open.

Capture takes shorthand in the desktop's capture grammar, one task per line,
previewed as it is typed and committed in one transaction, or a structured
form. A task's detail edits its daily fields (title, description, resume note,
priority, tags, context, pin, review, due, and deferral) and changes its plan
and status, the two halves of the desktop's workflow chord. The engine judges
every save, and a refusal is explained beside the draft it refused. Unsaved
capture and edit drafts are kept per workspace under
`devices/<name>/drafts`, never synced, and come back after a refused save, a
dismissed sheet, or a relaunch. **Close**, a swipe down, and the hardware-keyboard
close shortcuts all preserve the draft. **Draft actions → Discard Draft…**
asks before deleting it; **Keep Editing** retains every field. Only a committed
save or a confirmed discard removes the draft.

If recovery storage fails, the editor explains that its newest text is held
only for this session and offers **Retry Saving Drafts**. Retry before quitting;
it saves the recovery draft without submitting a task, comment, or note.
Unreadable earlier drafts are preserved before replacement, with **Export
Unreadable Draft** available from the notice.

The structured form's **Deadline time** toggle adds an optional local clock to
the due date. Changing the day preserves that clock; midnight is date-only.
Rows, task details, and VoiceOver show timed deadlines. Deferrals have a date
and time and retain the exact instant across devices. Time-only edits save;
editing an unrelated field preserves the existing schedule. Older saved form
drafts keep their original date-only meaning.

A task's detail lists its comments and attachments, and a project's settings
list the project's comments and open its notes. A project's task list also shows
its notes in a separate **Notes** section, newest first, including when it has no
tasks. Tap a note to open it; search, task filters, and archived projects omit
this section. The **Notes** tab lists
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
read with the exchange unreachable. Loading and read failures are distinct from
an empty successful read; **Retry** rereads after a failure. “No conflicts found
on this device” describes the local history, not delivery from every device.
A conflict shows what differs, the version
every side started from, and each competing version with the device and change
that wrote it. A version reads as a diff against that starting version, removed
lines in red and added lines in green under `@@` hunk headers, and **Show Whole
File** swaps in its full text. **Keep This Version** keeps that side of what conflicted and the
rest of what both devices changed, plus the same change's side of any other
file it touched; **Keep What This iPhone Has Now** and, for tasks and notes,
**Write a Replacement…** resolve it otherwise. A conflict that changed after it
was opened is refused without writing and read again.

Replacement drafts are saved locally as you edit. **Close** keeps the draft;
**Discard…** asks before deleting it, and starting from another version asks
before replacing edited text. Reopening or relaunching restores the draft without
submitting it. If the conflict has changed, review its updated versions and
explicitly choose **Use Draft for These Versions** before saving. Failed draft
writes use the shared recovery warning and Retry, and unreadable drafts are
preserved before replacement.

While the exchange fails, a banner says everything is saved on the iPhone and
offers **Retry**; the sheet's Recovery section repeats the retry and exports a
diagnostics file (under `devices/<name>/diagnostics/`) to share.

`ios/scripts/record-icloud-exchange.sh` records the same contract between this
Mac and a physical iPhone over iCloud Drive. First install the app on the
iPhone once, open it, and choose `iCloud Drive/tt-sync-evidence` as its exchange
folder. Then run the script with `TT_IOS_DEVELOPMENT_TEAM` set to your signing
team and the iPhone unlocked. Each run uses its own disposable workspace and its
own `runs/<id>` folder. The script rewrites
`docs/mobile/evidence/session-8-icloud-exchange.json`, which
`bash scripts/verify_mobile_session.sh 8` holds to the current sources.

## Device qualification and rollout

`just ios-install` uses `ios/scripts/install-device.sh` to build and sign the
app, install it over the earlier copy, and launch it, keeping the app's data.
The signing team comes from `TT_IOS_DEVELOPMENT_TEAM`, the recipe's optional
argument, or the previous device build. `ios/scripts/record-device-qualification.sh`
runs the guided real-device trial (airplane mode, simultaneous edits,
termination, delayed delivery, revoked folder access, large attachments, an
upgrade, the rollout procedures, and at least a week of daily use) in a
disposable workspace. It writes `docs/mobile/evidence/session-18-qualification.json`,
which `bash scripts/verify_mobile_session.sh 18` holds to the current builds.
[docs/mobile/personal-rollout.md](../docs/mobile/personal-rollout.md) is the
guide, including when to enroll a real workspace.
