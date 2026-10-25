import Foundation
import TTShared

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
    /// A simulator test releases opening only after it has observed loading.
    let openGate: URL?
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

    /// The App Group container shared with the share extension, the widgets,
    /// and shortcuts. A UI test, whose unsigned build holds no App Group,
    /// names a host directory instead.
    let sharedContainer: URL?
    /// Leaves saved captures in the inbox, so a UI test can see them pending
    /// before a relaunch imports them.
    let holdCaptureImports: Bool

    /// Where the app writes its semantic state for automation, rewritten as
    /// it changes (see AutomationState.swift); nil leaves nothing written.
    let stateExport: URL?
    /// A forced appearance, standing in for the system setting a UI test
    /// cannot reach; nil follows the system.
    let appearance: Appearance?

    enum Appearance: String, Sendable {
        case light, dark
    }

    static let current = LaunchConfiguration(environment: ProcessInfo.processInfo.environment)

    init(environment: [String: String]) {
        if let override = environment["DOCKET_UITEST_ROOT"], !override.isEmpty {
            root = URL(fileURLWithPath: override, isDirectory: true)
            legacyWorkspace = nil
        } else if let scratch = environment["DOCKET_UITEST_SCRATCH"], !scratch.isEmpty {
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
        openDelay = .milliseconds(Int(environment["DOCKET_UITEST_OPEN_DELAY_MS"] ?? "") ?? 0)
        #if DEBUG && targetEnvironment(simulator)
        openGate = environment["DOCKET_UITEST_HOLD_OPEN"] == "1" && environment["DOCKET_UITEST_ROOT"]?.isEmpty == false
            ? root.appendingPathComponent("hold-open") : nil
        #else
        openGate = nil
        #endif
        exchangeOverride = environment["DOCKET_UITEST_EXCHANGE_DIR"].flatMap { $0.isEmpty ? nil : URL(fileURLWithPath: $0, isDirectory: true) }
        deviceIDOverride = environment["DOCKET_UITEST_DEVICE_ID"].flatMap { $0.isEmpty ? nil : $0 }
        importSourceOverride = environment["DOCKET_UITEST_IMPORT_SOURCE"].flatMap { $0.isEmpty ? nil : URL(fileURLWithPath: $0) }
        holdImportsBeforeCommit = environment["DOCKET_UITEST_HOLD_IMPORTS"] == "1"
        if let shared = environment["DOCKET_UITEST_SHARED_CONTAINER"], !shared.isEmpty {
            sharedContainer = URL(fileURLWithPath: shared, isDirectory: true)
        } else {
            sharedContainer = SharedContainer.root(environment: [:])
        }
        holdCaptureImports = environment["DOCKET_UITEST_HOLD_CAPTURES"] == "1"
        stateExport = environment["DOCKET_UITEST_STATE_EXPORT"].flatMap { $0.isEmpty ? nil : URL(fileURLWithPath: $0) }
        appearance = environment["DOCKET_UITEST_APPEARANCE"].flatMap(Appearance.init(rawValue:))
        if let kind = environment["DOCKET_UITEST_SEED_REMINDER"].flatMap(ReminderSeed.Kind.init(rawValue:)) {
            reminderSeed = ReminderSeed(
                kind: kind,
                title: environment["DOCKET_UITEST_SEED_TITLE"] ?? "Reminder evidence",
                delay: TimeInterval(environment["DOCKET_UITEST_SEED_DELAY_SECONDS"] ?? "") ?? 20
            )
        } else {
            reminderSeed = nil
        }
    }

    /// Whether this launch runs under a UI test.
    var isUITest: Bool {
        let environment = ProcessInfo.processInfo.environment
        return environment.keys.contains { $0.hasPrefix("DOCKET_UITEST_") }
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

    /// Removes only an app-owned local copy and its device state. The picked
    /// exchange folder is deliberately outside this operation.
    func delete(_ workspace: WorkspaceLocation) throws {
        let manager = FileManager.default
        let ownedRoot = root.resolvingSymlinksInPath().standardizedFileURL
        let expectedData = ownedRoot.appendingPathComponent(
            workspace.name == WorkspaceLocation.legacyName && legacyWorkspace != nil
                ? "workspace" : "workspaces/\(workspace.name)"
        ).standardizedFileURL.path
        let expectedRuntime = ownedRoot.appendingPathComponent(
            workspace.name == WorkspaceLocation.legacyName && legacyWorkspace != nil
                ? "device" : "devices/\(workspace.name)"
        ).standardizedFileURL.path
        guard existingLocations().contains(workspace),
              workspace.name != ".", workspace.name != "..", !workspace.name.contains("/"),
              workspace.data.resolvingSymlinksInPath().standardizedFileURL.path == expectedData,
              workspace.runtime.resolvingSymlinksInPath().standardizedFileURL.path == expectedRuntime else {
            throw CocoaError(.fileWriteNoPermission)
        }
        if selectedLocation() == workspace { try select(nil) }
        // Keep the workspace listed if removing its device state fails, so
        // the person can retry instead of leaving invisible drafts behind.
        if manager.fileExists(atPath: workspace.runtime.path) {
            try manager.removeItem(at: workspace.runtime)
        }
        try manager.removeItem(at: workspace.data)
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
