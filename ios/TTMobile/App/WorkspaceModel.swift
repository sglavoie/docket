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

    func start() async {
        do {
            let support = try FileManager.default.url(
                for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true
            )
            try await engine.open(
                dataDirectory: support.appendingPathComponent("workspace", isDirectory: true),
                runtimeDirectory: support.appendingPathComponent("device", isDirectory: true),
                deviceID: Self.deviceID(),
                createIfMissing: true
            )
            isOpen = true
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
        } catch {
            failure = String(describing: error)
        }
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
