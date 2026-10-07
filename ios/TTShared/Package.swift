// swift-tools-version: 6.0
//
// TTShared is what the app shares with its share extension, its widgets, and
// its shortcuts through the App Group container: the durable capture inbox
// (docs/mobile/capture-inbox.md) an extension saves into and only the app's
// engine imports from, the widget snapshot the app writes and the widgets
// read with its freshness, and the deep links both sides spell. None of it
// links the engine, so an extension never touches the workspace.
import PackageDescription

let package = Package(
    name: "TTShared",
    platforms: [.iOS(.v18), .macOS(.v14)],
    products: [
        .library(name: "TTShared", targets: ["TTShared"]),
    ],
    targets: [
        .target(name: "TTShared"),
        .testTarget(name: "TTSharedTests", dependencies: ["TTShared"]),
    ]
)
