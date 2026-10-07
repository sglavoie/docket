import Foundation

/// Where the app keeps its workspaces and how this launch was asked to run.
/// Production reads the app container; a UI test points the same code at a
/// disposable root on the host and may slow the open down to observe loading
/// or hand over an exchange folder the document picker cannot be driven to.
struct LaunchConfiguration: Sendable {
    /// Holds `workspaces/<name>`, `devices/<name>`, and the selection file.
    let root: URL
    /// The pre-workspaces location an earlier build opened, adopted as the
    /// workspace named `workspace` when nothing is selected yet.
    let legacyWorkspace: URL?
    let openDelay: Duration
    let exchangeOverride: URL?
    let deviceIDOverride: String?
    /// A file a UI test hands over in place of the Files picker, which
    /// XCUITest cannot drive. The import pipeline is the same from there on.
    let importSourceOverride: URL?
    /// Stops every attachment import after it is staged and recorded, as a
    /// termination before the commit would, so a UI test can relaunch and
    /// watch the import resume.
    let holdImportsBeforeCommit: Bool

    /// A device UI test's one seeded reminder: a disposable workspace holding
    /// one task whose reminder fires `delay` seconds after launch.
    struct ReminderSeed: Sendable {
        enum Kind: String, Sendable {
            case checkIn = "check_in"
            case deferral
        }

        let kind: Kind
        let title: String
        let delay: TimeInterval
    }

    let reminderSeed: ReminderSeed?

    static let current = LaunchConfiguration(environment: ProcessInfo.processInfo.environment)

    init(environment: [String: String]) {
        if let override = environment["TT_UITEST_ROOT"], !override.isEmpty {
            root = URL(fileURLWithPath: override, isDirectory: true)
            legacyWorkspace = nil
        } else if let scratch = environment["TT_UITEST_SCRATCH"], !scratch.isEmpty {
            // A disposable root inside the app's own container, for UI tests
            // on a physical iPhone, which cannot reach a host directory. The
            // workspaces already on the device are neither read nor written.
            let support = (try? FileManager.default.url(
                for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true
            )) ?? FileManager.default.temporaryDirectory
            root = support.appendingPathComponent("uitest-scratch/\(scratch)", isDirectory: true)
            legacyWorkspace = nil
        } else {
            let support = (try? FileManager.default.url(
                for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true
            )) ?? FileManager.default.temporaryDirectory
            root = support
            legacyWorkspace = support.appendingPathComponent("workspace", isDirectory: true)
        }
        openDelay = .milliseconds(Int(environment["TT_UITEST_OPEN_DELAY_MS"] ?? "") ?? 0)
        exchangeOverride = environment["TT_UITEST_EXCHANGE_DIR"].flatMap { $0.isEmpty ? nil : URL(fileURLWithPath: $0, isDirectory: true) }
        deviceIDOverride = environment["TT_UITEST_DEVICE_ID"].flatMap { $0.isEmpty ? nil : $0 }
        importSourceOverride = environment["TT_UITEST_IMPORT_SOURCE"].flatMap { $0.isEmpty ? nil : URL(fileURLWithPath: $0) }
        holdImportsBeforeCommit = environment["TT_UITEST_HOLD_IMPORTS"] == "1"
        if let kind = environment["TT_UITEST_SEED_REMINDER"].flatMap(ReminderSeed.Kind.init(rawValue:)) {
            reminderSeed = ReminderSeed(
                kind: kind,
                title: environment["TT_UITEST_SEED_TITLE"] ?? "Reminder evidence",
                delay: TimeInterval(environment["TT_UITEST_SEED_DELAY_SECONDS"] ?? "") ?? 20
            )
        } else {
            reminderSeed = nil
        }
    }

    /// Whether this launch runs under a UI test.
    var isUITest: Bool {
        let environment = ProcessInfo.processInfo.environment
        return environment.keys.contains { $0.hasPrefix("TT_UITEST_") }
    }

    private var selectionFile: URL { root.appendingPathComponent("selected-workspace") }

    func location(named name: String) -> WorkspaceLocation {
        if name == WorkspaceLocation.legacyName, let legacyWorkspace {
            return WorkspaceLocation(
                name: name, data: legacyWorkspace,
                runtime: root.appendingPathComponent("device", isDirectory: true)
            )
        }
        return WorkspaceLocation(
            name: name,
            data: root.appendingPathComponent("workspaces/\(name)", isDirectory: true),
            runtime: root.appendingPathComponent("devices/\(name)", isDirectory: true)
        )
    }

    /// Every workspace on this device, by name.
    func existingLocations() -> [WorkspaceLocation] {
        let directory = root.appendingPathComponent("workspaces", isDirectory: true)
        var names = (try? FileManager.default.contentsOfDirectory(atPath: directory.path)) ?? []
        names = names.filter { !$0.hasPrefix(".") }.sorted()
        if let legacyWorkspace, FileManager.default.fileExists(atPath: legacyWorkspace.path) {
            names.insert(WorkspaceLocation.legacyName, at: 0)
        }
        return names.map(location(named:))
    }

    /// The selected workspace, or nil before onboarding. A device an earlier
    /// build already set up keeps its workspace without onboarding again.
    func selectedLocation() -> WorkspaceLocation? {
        if let data = try? Data(contentsOf: selectionFile) {
            let name = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
            return name.isEmpty ? nil : location(named: name)
        }
        if let legacyWorkspace, FileManager.default.fileExists(atPath: legacyWorkspace.path) {
            return location(named: WorkspaceLocation.legacyName)
        }
        return nil
    }

    func select(_ location: WorkspaceLocation?) throws {
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try Data((location?.name ?? "").utf8).write(to: selectionFile, options: .atomic)
    }

    /// A name no workspace on this device uses yet.
    func unusedName(_ base: String) -> String {
        let taken = Set(existingLocations().map(\.name))
        var name = base
        var counter = 2
        while taken.contains(name) || FileManager.default.fileExists(atPath: location(named: name).data.path) {
            name = "\(base)-\(counter)"
            counter += 1
        }
        return name
    }
}

/// One workspace on this device: its data and its machine-local runtime.
struct WorkspaceLocation: Hashable, Sendable, Identifiable {
    static let legacyName = "workspace"

    let name: String
    let data: URL
    let runtime: URL

    var id: String { name }
}
