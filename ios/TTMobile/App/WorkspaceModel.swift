import Foundation
import Observation
import TTEngine

/// The app's one local workspace: opened from the app container at launch,
/// read through the engine, and refreshed after every commit.
@MainActor
@Observable
final class WorkspaceModel {
    private(set) var inbox: [TaskSnapshot] = []
    private(set) var failure: String?
    private(set) var isOpen = false

    private let engine = TTEngine()
    let sync = SyncModel()

    /// The engine serves one workspace per process, so a hosted test that
    /// opens its own must not race the app for it.
    private static var isHostingTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
    }

    func start() async {
        guard !Self.isHostingTests else { return }
        do {
            let support = try FileManager.default.url(
                for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true
            )
            let data = support.appendingPathComponent("workspace", isDirectory: true)
            let runtime = support.appendingPathComponent("device", isDirectory: true)
            let device = Self.deviceID()
            try await engine.open(dataDirectory: data, runtimeDirectory: runtime, deviceID: device, createIfMissing: true)
            isOpen = true
            sync.attach(dataDirectory: data, runtimeDirectory: runtime, deviceID: device)
            sync.exchange()
            await reload()
        } catch {
            failure = String(describing: error)
        }
    }

    func capture(_ title: String) async {
        await perform { _ = try await self.engine.capture(title: title) }
    }

    func complete(_ task: TaskSnapshot) async {
        await perform { _ = try await self.engine.transition(task.locator, to: .done) }
    }

    private func perform(_ mutation: @escaping () async throws -> Void) async {
        do {
            try await mutation()
            failure = nil
            sync.exchange()
        } catch {
            failure = String(describing: error)
        }
        await reload()
    }

    /// Exchanges, then rereads what an import may have changed.
    func refresh() async {
        sync.exchange()
        await reload()
    }

    private func reload() async {
        do {
            inbox = try await engine.tasks(mode: "inbox").items.map(\.task)
        } catch {
            failure = String(describing: error)
        }
    }

    /// A stable identity for this installation. The keychain replaces this
    /// once sync needs an identity that survives reinstalling.
    private static func deviceID() -> String {
        let key = "tt.device-id"
        if let existing = UserDefaults.standard.string(forKey: key) {
            return existing
        }
        let minted = UUID().uuidString.lowercased()
        UserDefaults.standard.set(minted, forKey: key)
        return minted
    }
}
