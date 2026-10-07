import Foundation
import Observation
import TTEngine

/// Where the app stands with its workspace. Each phase has its own screen, so
/// a slow open, a first launch, a workspace that went missing, and one this
/// build cannot read never look alike.
enum WorkspacePhase: Equatable {
    case loading
    case onboarding
    case ready
    /// The selected workspace is not where it was left.
    case unavailable(String)
    /// The engine refused the workspace.
    case failed(String)
}

/// The app's one open workspace: chosen at onboarding, opened through the
/// engine, and reread after every commit and every import. `generation`
/// moves whenever the workspace may have changed, which is what every list
/// reloads on.
@MainActor
@Observable
final class WorkspaceModel {
    private(set) var phase: WorkspacePhase = .loading
    private(set) var status: WorkspaceStatus?
    private(set) var location: WorkspaceLocation?
    private(set) var generation = 0
    /// The last mutation refusal, shown until the next one succeeds.
    private(set) var failure: String?
    private(set) var isJoining = false
    private(set) var joinFailure: String?

    let engine = TTEngine()
    let sync = SyncModel()
    let launch = LaunchConfiguration.current

    /// The engine serves one workspace per process, so a hosted test that
    /// opens its own must not race the app for it.
    private static var isHostingTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
    }

    init() {
        sync.onImport = { [weak self] in self?.generation += 1 }
    }

    var existingLocations: [WorkspaceLocation] { launch.existingLocations() }

    func start() async {
        guard !Self.isHostingTests else { return }
        phase = .loading
        if launch.openDelay > .zero {
            try? await Task.sleep(for: launch.openDelay)
        }
        guard let selected = launch.selectedLocation() else {
            phase = .onboarding
            return
        }
        await open(selected, create: false)
    }

    /// Opens a workspace on this device. Without `create`, a missing
    /// workspace is reported unavailable rather than silently recreated empty.
    func open(_ location: WorkspaceLocation, create: Bool) async {
        phase = .loading
        if !create, !FileManager.default.fileExists(atPath: location.data.appendingPathComponent(".tt").path) {
            phase = .unavailable("The workspace “\(location.name)” is no longer on this iPhone.")
            return
        }
        do {
            try FileManager.default.createDirectory(at: location.runtime, withIntermediateDirectories: true)
            let device = deviceID()
            status = try await engine.open(
                dataDirectory: location.data, runtimeDirectory: location.runtime,
                deviceID: device, createIfMissing: create
            )
            try launch.select(location)
            self.location = location
            failure = nil
            sync.attach(location: location, deviceID: device, exchangeOverride: launch.exchangeOverride)
            sync.exchange()
            generation += 1
            phase = .ready
        } catch {
            status = nil
            phase = .failed(Self.describe(error))
        }
    }

    /// Starts a new, empty workspace kept on this iPhone.
    func createWorkspace() async {
        await open(launch.location(named: launch.unusedName("local")), create: true)
    }

    /// Joins the workspace a desktop enrolled in the exchange `folder`: the
    /// transport downloads its history and the engine builds the working copy
    /// from it, then the app opens it like any other.
    func join(folder: URL?) async {
        let access: SyncModel.FolderChoice
        if let folder {
            access = .picked(folder)
        } else if let override = launch.exchangeOverride {
            access = .direct(override)
        } else {
            joinFailure = "Choose the exchange folder the Mac enrolled the workspace in."
            return
        }
        isJoining = true
        joinFailure = nil
        let location = launch.location(named: launch.unusedName("joined"))
        let device = deviceID()
        do {
            try await sync.join(choice: access, location: location, deviceID: device)
            isJoining = false
            await open(location, create: false)
        } catch {
            isJoining = false
            joinFailure = Self.describe(error)
        }
    }

    /// Returns to onboarding without touching any workspace's files.
    func chooseAnotherWorkspace() {
        try? launch.select(nil)
        sync.detach()
        location = nil
        status = nil
        phase = .onboarding
    }

    func retry() async {
        await start()
    }

    /// This workspace's unsaved drafts, or nil before one is open.
    var drafts: DraftStore? { location.map(DraftStore.init(location:)) }

    /// Captures shorthand text. Returns the refusal in a person's words, or
    /// nil once every line committed.
    func capture(text: String, projectID: String?) async -> String? {
        await attempt { _ = try await self.engine.capture(text: text, projectID: projectID) }
    }

    /// Captures one structured task.
    func capture(_ fields: CaptureFields) async -> String? {
        await attempt { _ = try await self.engine.capture(fields) }
    }

    func previewCapture(text: String, projectID: String?) async -> CapturePreview? {
        try? await engine.previewCapture(text: text, projectID: projectID)
    }

    func edit(_ locator: TaskLocator, _ edit: TaskEdit) async -> String? {
        await attempt { _ = try await self.engine.edit(locator, edit) }
    }

    func plan(_ locator: TaskLocator, _ action: PlanAction) async -> String? {
        await attempt { _ = try await self.engine.plan(locator, action) }
    }

    func transition(_ locator: TaskLocator, to state: WorkflowState) async -> String? {
        await attempt { _ = try await self.engine.transition(locator, to: state) }
    }

    /// Runs one mutation for a screen that shows its own refusal, so the
    /// refusal stays beside the draft it refused rather than in the banner.
    private func attempt(_ mutation: @escaping () async throws -> Void) async -> String? {
        do {
            try await mutation()
            generation += 1
            sync.exchange()
            return nil
        } catch {
            return Self.describe(error)
        }
    }

    func complete(_ task: TaskSnapshot) async {
        _ = await perform { _ = try await self.engine.transition(task.locator, to: .done) }
    }

    private func perform(_ mutation: @escaping () async throws -> Void) async -> Bool {
        do {
            try await mutation()
            failure = nil
            generation += 1
            sync.exchange()
            return true
        } catch {
            failure = Self.describe(error)
            return false
        }
    }

    /// Exchanges, then rereads what an import may have changed.
    func refresh() async {
        sync.exchange()
        generation += 1
    }

    static func describe(_ error: Error) -> String {
        if let failure = error as? BridgeError {
            switch failure.code {
            case .schemaTooNew: return "This workspace was written by a newer tt. Update tt on this iPhone to open it. (\(failure.message))"
            case .migrationRequired: return "This workspace needs a migration only the desktop runs. (\(failure.message))"
            case .validation: return failure.explanation
            default: return failure.message
            }
        }
        return String(describing: error)
    }

    /// A stable identity for this installation. The keychain replaces this
    /// once sync needs an identity that survives reinstalling.
    private func deviceID() -> String {
        if let override = launch.deviceIDOverride {
            return override
        }
        let key = "tt.device-id"
        if let existing = UserDefaults.standard.string(forKey: key) {
            return existing
        }
        let minted = UUID().uuidString.lowercased()
        UserDefaults.standard.set(minted, forKey: key)
        return minted
    }
}
