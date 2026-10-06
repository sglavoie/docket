// swift-tools-version: 6.0
//
// TTSync is the Apple transport for the sync exchange folder
// (docs/mobile/sync-envelope.md), shared by the iPhone app and the macOS
// helper. It moves immutable manifests and blobs between a device's engine
// and an exchange folder — coordinated file access, iCloud downloads, folder
// access renewal, account checks, retries, and durable transfer queues — while
// the Go engine, reached through the bridge, owns verification and
// reconciliation. TTSyncContract is the two-device exchange contract the
// fake-provider suite and the real Mac/iPhone iCloud exchange both run.
import PackageDescription

let package = Package(
    name: "TTSync",
    platforms: [.iOS(.v18), .macOS(.v14)],
    products: [
        .library(name: "TTSync", targets: ["TTSync"]),
        .library(name: "TTSyncContract", targets: ["TTSyncContract"]),
        .executable(name: "tt-sync-helper", targets: ["tt-sync-helper"]),
    ],
    targets: [
        .target(name: "TTSync"),
        .target(name: "TTSyncContract", dependencies: ["TTSync"]),
        .executableTarget(name: "tt-sync-helper", dependencies: ["TTSync", "TTSyncContract"]),
        .testTarget(name: "TTSyncTests", dependencies: ["TTSync", "TTSyncContract"]),
    ]
)
