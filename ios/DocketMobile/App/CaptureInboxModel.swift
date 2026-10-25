import Foundation
import Observation
import DocketEngine
import DocketShared
import WidgetKit

/// The app's half of the App Group: it imports what the share extension and
/// shortcuts saved, shows what is still waiting, and keeps the widgets'
/// snapshot current. Extensions only ever write captures; the workspace
/// changes here, through the engine.
@MainActor
@Observable
final class CaptureInboxModel {
    /// Saved captures the engine has not imported yet, oldest first.
    private(set) var pending: [PendingCapture] = []

    /// The App Group container, or nil when this build has no entitlement
    /// (an unsigned simulator build outside a UI test).
    let container: URL?
    private let holdImports: Bool
    @ObservationIgnored private var publishing: Task<Void, Never>?

    init(container: URL?, holdImports: Bool) {
        self.container = container
        self.holdImports = holdImports
    }

    var inbox: CaptureInbox? { container.map(CaptureInbox.init(container:)) }
    private var store: WidgetSnapshotStore? { container.map(WidgetSnapshotStore.init(container:)) }

    func refreshPending() {
        pending = inbox?.pending() ?? []
    }

    /// Imports every saved capture. Answers whether the workspace changed.
    func importPending(engine: DocketEngine) async -> Bool {
        refreshPending()
        guard let inbox, !holdImports, !pending.isEmpty else { return false }
        do {
            let report = try await engine.importCaptures(from: inbox.directory)
            refreshPending()
            return report.changedWorkspace
        } catch {
            // Every capture stays pending; the next pass retries it.
            refreshPending()
            return false
        }
    }

    func discard(_ capture: PendingCapture) {
        try? inbox?.discard(id: capture.id)
        refreshPending()
    }

    /// Rewrites a refused capture's text so the next import tries it again.
    func revise(_ capture: PendingCapture, text: String) {
        let content: CaptureContent = capture.envelope.text != nil
            ? .text(text) : .literal(title: text, note: capture.envelope.note ?? "")
        try? inbox?.revise(id: capture.id, content: content)
        refreshPending()
    }

    /// Writes the widgets' snapshot from the engine, reloading the widgets
    /// when it changed. A request waits briefly first and yields to a newer
    /// one, so a burst of commits publishes once and the screens' own reads
    /// of a commit reach the engine before the snapshot's does.
    func publishSnapshot(engine: DocketEngine) {
        guard let store else { return }
        requested += 1
        let request = requested
        let previous = publishing
        publishing = Task {
            await previous?.value
            try? await Task.sleep(for: Self.publishDelay)
            guard request == requested else { return }
            do {
                let value = try await engine.widgetSnapshot()
                let data = try JSONEncoder().encode(value)
                let snapshot = try WidgetSnapshot.decode(from: data)
                if try store.write(snapshot) {
                    WidgetCenter.shared.reloadAllTimelines()
                }
            } catch {
                // The widgets keep the last snapshot and say how old it is.
            }
        }
    }

    private static let publishDelay: Duration = .milliseconds(750)
    @ObservationIgnored private var requested = 0

    /// Waits for the snapshot publications already asked for.
    func settle() async {
        await publishing?.value
    }

    /// Removes the snapshot of a workspace this device no longer has open.
    func clearSnapshot() {
        store?.clear()
        WidgetCenter.shared.reloadAllTimelines()
    }
}
