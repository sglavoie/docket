// swift-tools-version: 6.0
//
// DocketShared is what the app shares with its share extension, its widgets, and
// its shortcuts through the App Group container: the durable capture inbox
// (docs/mobile/capture-inbox.md) an extension saves into and only the app's
// engine imports from, the widget snapshot the app writes and the widgets
// read with its freshness, and the deep links both sides spell. None of it
// links the engine, so an extension never touches the workspace.
import PackageDescription

let package = Package(
    name: "DocketShared",
    platforms: [.iOS(.v18), .macOS(.v14)],
    products: [
        .library(name: "DocketShared", targets: ["DocketShared"]),
    ],
    targets: [
        .target(name: "DocketShared"),
        .testTarget(name: "DocketSharedTests", dependencies: ["DocketShared"]),
    ]
)
