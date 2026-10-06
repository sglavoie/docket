import Foundation
import Observation
import TTEngine
import TTSync

/// The app's exchange with the folder its user picked. Local saves never wait
/// for it: each pass runs off the main actor, and what it reports — the local
/// save state, outstanding transfers, the last successful exchange, content
/// still unavailable, and open conflicts — is shown apart, so nothing implies
/// a remote delivery from a local write.
@MainActor
@Observable
final class SyncModel {
    private(set) var hasFolder = false
    private(set) var isExchanging = false
    private(set) var condition: ExchangeCondition?
    private(set) var outstandingUploads = 0
    private(set) var outstandingDownloads = 0
    private(set) var unavailable: [String] = []
    private(set) var conflicts = 0
    private(set) var lastSuccessfulExchange: Date?
    private(set) var lastError: String?

    private var transport: ExchangeTransport?
    private var pending = false

    /// Connects the transport to the open workspace and restores the picked
    /// folder, if any.
    func attach(dataDirectory: URL, runtimeDirectory: URL, deviceID: String) {
        do {
            let client = BridgeClient(call: { GoBridgeTransport().call($0) })
            try client.open(dataDirectory: dataDirectory, runtimeDirectory: runtimeDirectory, deviceID: deviceID)
            let bookmark = try ExchangeFolderBookmark.defaultURL()
            hasFolder = FileManager.default.fileExists(atPath: bookmark.path)
            transport = ExchangeTransport(
                provider: CoordinatedFolderProvider(access: BookmarkFolderAccess(bookmarkFile: bookmark)),
                engine: try BridgeSyncEngine(client: client),
                stateDirectory: runtimeDirectory.appendingPathComponent("sync-transport", isDirectory: true)
            )
            if let state = try? transport?.state() {
                apply(state)
            }
        } catch {
            lastError = String(describing: error)
        }
    }

    /// Records the folder the document picker returned.
    func choose(folder: URL) {
        do {
            try BookmarkFolderAccess.save(folder: folder, to: ExchangeFolderBookmark.defaultURL())
            hasFolder = true
            lastError = nil
            exchange()
        } catch {
            lastError = "the folder could not be kept: \(error.localizedDescription)"
        }
    }

    /// Runs one exchange pass in the background; a request made during a pass
    /// runs once more after it.
    func exchange() {
        guard hasFolder, let transport else { return }
        if isExchanging {
            pending = true
            return
        }
        isExchanging = true
        Task.detached(priority: .utility) {
            let outcome = Result { try transport.exchange() }
            let conflicts = (try? transport.engine.status().conflicts.count) ?? 0
            await MainActor.run {
                self.finish(outcome, conflicts: conflicts)
            }
        }
    }

    private func finish(_ outcome: Result<ExchangeReport, Error>, conflicts: Int) {
        isExchanging = false
        self.conflicts = conflicts
        switch outcome {
        case .success(let report):
            condition = report.condition
            outstandingUploads = report.outstandingUploads
            outstandingDownloads = report.outstandingDownloads
            unavailable = report.unavailable
            lastSuccessfulExchange = report.lastSuccessfulExchange
            lastError = report.errors.last
        case .failure(let error):
            lastError = String(describing: error)
        }
        if pending {
            pending = false
            exchange()
        }
    }

    private func apply(_ state: TransferState) {
        condition = state.condition
        outstandingUploads = state.uploads.count
        outstandingDownloads = state.downloads.count
        unavailable = Array(Set(state.downloads.flatMap(\.unavailable))).sorted()
        lastSuccessfulExchange = state.lastSuccessfulExchange
        lastError = state.lastError
    }
}
