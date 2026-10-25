// swift-tools-version: 6.0
//
// DocketSync is the Apple transport for the sync exchange folder
// (docs/mobile/sync-envelope.md), shared by the iPhone app and the macOS
// helper. It moves immutable manifests and blobs between a device's engine
// and an exchange folder — coordinated file access, iCloud downloads, folder
// access renewal, account checks, retries, and durable transfer queues — while
// the Go engine, reached through the bridge, owns verification and
// reconciliation. DocketSyncContract is the two-device exchange contract the
// fake-provider suite and the real Mac/iPhone iCloud exchange both run.
import PackageDescription

let package = Package(
    name: "DocketSync",
    platforms: [.iOS(.v18), .macOS(.v14)],
    products: [
        .library(name: "DocketSync", targets: ["DocketSync"]),
        .library(name: "DocketSyncContract", targets: ["DocketSyncContract"]),
        .executable(name: "docket-sync-helper", targets: ["docket-sync-helper"]),
    ],
    targets: [
        .target(name: "DocketSync"),
        .target(name: "DocketSyncContract", dependencies: ["DocketSync"]),
        .executableTarget(name: "docket-sync-helper", dependencies: ["DocketSync", "DocketSyncContract"]),
        .testTarget(name: "DocketSyncTests", dependencies: ["DocketSync", "DocketSyncContract"]),
    ]
)
