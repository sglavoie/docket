// swift-tools-version: 6.0
//
// TTReminders turns the engine's reminder plan (the bridge's query.reminders,
// docs/mobile/bridge-contract.md) into local notifications and routes their
// actions back to engine commands. The engine decides which tasks raise a
// reminder and when, under the shared attention rules; this package only
// schedules, replaces, and cancels what the plan names, behind a
// ReminderNotificationCenter protocol the tests replace with a fake.
import PackageDescription

let package = Package(
    name: "TTReminders",
    platforms: [.iOS(.v18), .macOS(.v14)],
    products: [
        .library(name: "TTReminders", targets: ["TTReminders"]),
    ],
    targets: [
        .target(name: "TTReminders"),
        .testTarget(name: "TTRemindersTests", dependencies: ["TTReminders"]),
    ]
)
