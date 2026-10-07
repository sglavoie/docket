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
| `TTMobileUITests/` | the seeded browsing UI suite, on a simulator against the real engine |
| `TTMobileDeviceTests/` | the responder of the real iCloud exchange, hosted in the app on a physical iPhone |
| `../apple/TTSync/` | Swift package shared with the macOS helper: the exchange transport and its contract |
| `project.yml` | XcodeGen spec for `TTMobile.xcodeproj` (generated, gitignored) |
| `scripts/` | the bridge build, project generation, and the simulator suite |

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
| `just ios-test` | The `TTEngine` simulator suite against the real engine |

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
the bridge to byte-identical capture and editing outcomes.

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

`ios/scripts/record-icloud-exchange.sh` records the same contract between this
Mac and a physical iPhone over iCloud Drive. First install the app on the
iPhone once, open it, and choose `iCloud Drive/tt-sync-evidence` as its exchange
folder. Then run the script with `TT_IOS_DEVELOPMENT_TEAM` set to your signing
team and the iPhone unlocked. Each run uses its own disposable workspace and its
own `runs/<id>` folder. The script rewrites
`docs/mobile/evidence/session-8-icloud-exchange.json`, which
`bash scripts/verify_mobile_session.sh 8` holds to the current sources.
