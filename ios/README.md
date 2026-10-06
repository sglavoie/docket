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
