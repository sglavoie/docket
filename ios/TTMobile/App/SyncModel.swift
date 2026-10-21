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
    /// How the exchange folder is reached: the document picker's bookmark, or
    /// a folder this process may always read (a UI test's host folder).
    enum FolderChoice: Sendable {
        case picked(URL)
        case direct(URL)
    }

    struct JoinFailure: Error, CustomStringConvertible {
        let description: String
    }

    private(set) var hasFolder = false
    private(set) var isExchanging = false
    private(set) var condition: ExchangeCondition?
    private(set) var outstandingUploads = 0
    private(set) var outstandingDownloads = 0
    private(set) var waitingForDownload = 0
    private(set) var waitingToApply = 0
    private(set) var blockedReason: String?
    private(set) var unavailable: [String] = []
    private(set) var held: [String] = []
    private(set) var conflicts = 0
    private(set) var lastSuccessfulExchange: Date?
    private(set) var lastError: String?
    /// Names other devices advertise in the exchange, by device id, read on
    /// each pass so a conflict names who wrote each version.
    private(set) var deviceNames: [String: String] = [:]
    /// When the last pass ended, successful or not.
    private(set) var lastAttempt: Date?

    /// Whether the last pass failed to reach the exchange or finished with an
    /// error. Local saves are unaffected: everything stays on this iPhone and
    /// is published on a later pass.
    var isFailing: Bool {
        guard hasFolder else { return false }
        if let condition, condition != .reachable { return true }
        return lastError != nil
    }

    /// Called on the main actor after a pass imported anything.
    var onImport: (() -> Void)?

    private var transport: ExchangeTransport?
    private var pending: ExchangeRequest?
    private var attachment = UUID()
    private var exchangeTasks: [UUID: Task<Void, Never>] = [:]

    /// Detaching hides old results immediately, but a pass can still be
    /// writing files. Workspace deletion waits until those writers finish.
    func waitForExchanges() async {
        for task in Array(exchangeTasks.values) { await task.value }
    }

    init(transport: ExchangeTransport? = nil) {
        self.transport = transport
        hasFolder = transport != nil
    }

    private enum ExchangeRequest: Sendable {
        case automatic, retry, download([String]?)

        func merging(_ other: ExchangeRequest) -> ExchangeRequest {
            switch (self, other) {
            case (.retry, _), (_, .retry): return .retry
            case (.download(nil), _), (_, .download(nil)): return .download(nil)
            case (.download(let left?), .download(let right?)): return .download(Array(Set(left + right)).sorted())
            case (.automatic, _): return other
            default: return self
            }
        }
    }

    /// Connects the transport to the open workspace and restores the picked
    /// folder, if any.
    func attach(location: WorkspaceLocation, deviceID: String, exchangeOverride: URL?) {
        detach()
        do {
            let client = BridgeClient(call: { GoBridgeTransport().call($0) })
            try client.open(dataDirectory: location.data, runtimeDirectory: location.runtime, deviceID: deviceID)
            let access: any FolderAccess
            if let exchangeOverride {
                access = DirectFolderAccess(folder: exchangeOverride)
                hasFolder = true
            } else {
                let bookmark = try ExchangeFolderBookmark.defaultURL()
                access = BookmarkFolderAccess(bookmarkFile: bookmark)
                hasFolder = FileManager.default.fileExists(atPath: bookmark.path)
            }
            transport = ExchangeTransport(
                provider: Self.provider(access: access),
                engine: try BridgeSyncEngine(client: client),
                stateDirectory: location.runtime.appendingPathComponent("sync-transport", isDirectory: true)
            )
            if let state = try? transport?.state() {
                apply(state)
            }
        } catch {
            lastError = String(describing: error)
        }
    }

    /// Forgets the transport; the workspace and the queues stay on disk.
    func detach() {
        attachment = UUID()
        transport = nil
        isExchanging = false
        pending = nil
        hasFolder = false
        condition = nil
        outstandingUploads = 0
        outstandingDownloads = 0
        waitingForDownload = 0
        waitingToApply = 0
        blockedReason = nil
        unavailable = []
        held = []
        conflicts = 0
        lastSuccessfulExchange = nil
        lastError = nil
        deviceNames = [:]
        lastAttempt = nil
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

    /// Builds `location` from the history an enrolled exchange holds. One
    /// transport pass downloads and stages it; the engine's first import is
    /// the join. The pass's queues are kept, so the opened workspace resumes
    /// them rather than downloading again.
    func join(choice: FolderChoice, location: WorkspaceLocation, deviceID: String) async throws {
        let access: any FolderAccess
        switch choice {
        case .picked(let folder):
            let bookmark = try ExchangeFolderBookmark.defaultURL()
            try BookmarkFolderAccess.save(folder: folder, to: bookmark)
            access = BookmarkFolderAccess(bookmarkFile: bookmark)
        case .direct(let folder):
            access = DirectFolderAccess(folder: folder)
        }
        try await Task.detached(priority: .userInitiated) {
            let provider = Self.provider(access: access)
            try provider.checkAccess()
            let workspaceID = try JoiningSyncEngine.workspaceID(in: provider)
            try FileManager.default.createDirectory(at: location.runtime, withIntermediateDirectories: true)
            let engine = JoiningSyncEngine(
                client: BridgeClient(call: { GoBridgeTransport().call($0) }),
                workspaceID: workspaceID, dataDirectory: location.data, runtimeDirectory: location.runtime,
                deviceID: deviceID, exchange: try access.url().path
            )
            let transport = ExchangeTransport(
                provider: provider, engine: engine,
                stateDirectory: location.runtime.appendingPathComponent("sync-transport", isDirectory: true)
            )
            let report = try transport.exchange()
            guard engine.joined else {
                throw JoinFailure(description: report.errors.last
                    ?? "The exchange has not delivered the workspace yet; iCloud may still be downloading it.")
            }
        }.value
    }

    /// Runs one exchange pass in the background; a request made during a pass
    /// runs once more after it.
    func exchange() {
        exchange(.automatic)
    }

    private func exchange(_ request: ExchangeRequest) {
        guard hasFolder, let transport else { return }
        if isExchanging {
            pending = pending?.merging(request) ?? request
            return
        }
        // Publish progress before any download requests, locks, or folder I/O.
        isExchanging = true
        let attachment = self.attachment
        let priority: TaskPriority
        if case .automatic = request { priority = .utility } else { priority = .userInitiated }
        let taskID = UUID()
        exchangeTasks[taskID] = Task.detached(priority: priority) {
            let outcome = Result {
                switch request {
                case .automatic: return try transport.exchange()
                case .retry: return try transport.exchange(retryNow: true)
                case .download(let paths):
                    try transport.requestDownloads(paths: paths)
                    return try transport.exchange()
                }
            }
            let conflicts = (try? transport.engine.status().conflicts.count) ?? 0
            let names = transport.deviceNames()
            await MainActor.run {
                self.exchangeTasks[taskID] = nil
                guard self.attachment == attachment else { return }
                if !names.isEmpty { self.deviceNames = names }
                self.finish(outcome, conflicts: conflicts)
            }
        }
    }

    nonisolated private static func provider(access: any FolderAccess) -> any ExchangeProvider {
        let base = CoordinatedFolderProvider(access: access)
        #if DEBUG && targetEnvironment(simulator)
        let environment = ProcessInfo.processInfo.environment
        if let root = environment["TT_UITEST_ROOT"], !root.isEmpty,
           environment["TT_UITEST_SYNC_CONTROL"] == "1" {
            return SimulatorExchangeProvider(
                base: base, control: URL(fileURLWithPath: root).appendingPathComponent("exchange-control.json")
            )
        }
        #endif
        return base
    }

    /// Retries now: every queued transfer skips its backoff and the exchange
    /// runs once more. Nothing local waits on it.
    func retry() {
        exchange(.retry)
    }

    /// Records the conflict count a resolution reported, before the next pass
    /// confirms it.
    func noteConflicts(_ count: Int) {
        conflicts = count
    }

    /// The transport's half of a diagnostics export: the durable queues and
    /// condition as last saved, plus what this session observed.
    func diagnostics() -> TransportDiagnostics {
        TransportDiagnostics(
            hasFolder: hasFolder,
            condition: condition?.rawValue,
            lastError: lastError,
            lastAttempt: lastAttempt,
            lastSuccessfulExchange: lastSuccessfulExchange,
            outstandingUploads: outstandingUploads,
            outstandingDownloads: outstandingDownloads,
            waitingForDownload: waitingForDownload,
            waitingToApply: waitingToApply,
            blockedReason: blockedReason,
            unavailable: unavailable,
            held: held,
            deviceNames: deviceNames,
            state: try? transport?.state()
        )
    }

    /// Asks for content a person is waiting on now: the queued transfers
    /// whose unavailable paths include one of `paths` (every queued one when
    /// nil) skip their retry delay and request their missing files from
    /// iCloud, then an exchange runs.
    func download(paths: [String]?) {
        exchange(.download(paths))
    }

    /// Whether content for `path` is known to be on its way.
    func isArriving(_ path: String) -> Bool {
        unavailable.contains(path)
    }

    private func finish(_ outcome: Result<ExchangeReport, Error>, conflicts: Int) {
        isExchanging = false
        lastAttempt = Date()
        self.conflicts = conflicts
        switch outcome {
        case .success(let report):
            condition = report.condition
            outstandingUploads = report.outstandingUploads
            outstandingDownloads = report.outstandingDownloads
            waitingForDownload = report.waitingForDownload ?? report.outstandingDownloads
            waitingToApply = report.waitingToApply ?? 0
            blockedReason = report.blockedReason
            unavailable = report.unavailable
            held = report.held
            lastSuccessfulExchange = report.lastSuccessfulExchange
            lastError = report.errors.last
            if !report.imported.isEmpty {
                onImport?()
            }
        case .failure(let error):
            lastError = String(describing: error)
        }
        if let pending {
            self.pending = nil
            exchange(pending)
        }
    }

    private func apply(_ state: TransferState) {
        condition = state.condition
        outstandingUploads = state.uploads.count
        outstandingDownloads = state.downloads.count
        waitingForDownload = state.waitingForDownload
        waitingToApply = state.waitingToApply
        blockedReason = state.blockedReason
        unavailable = Array(Set(state.downloads.flatMap(\.unavailable))).sorted()
        held = state.downloads.filter { $0.outcome == "held" }.map(\.txnID)
        lastSuccessfulExchange = state.lastSuccessfulExchange
        lastError = state.lastError
    }
}

/// What the transport knew when a diagnostics export was made.
struct TransportDiagnostics: Encodable {
    var hasFolder: Bool
    var condition: String?
    var lastError: String?
    var lastAttempt: Date?
    var lastSuccessfulExchange: Date?
    var outstandingUploads: Int
    var outstandingDownloads: Int
    var waitingForDownload: Int
    var waitingToApply: Int
    var blockedReason: String?
    var unavailable: [String]
    var held: [String]
    var deviceNames: [String: String]
    var state: TransferState?

    enum CodingKeys: String, CodingKey {
        case condition, unavailable, held, state
        case hasFolder = "has_folder"
        case lastError = "last_error"
        case lastAttempt = "last_attempt"
        case lastSuccessfulExchange = "last_successful_exchange"
        case outstandingUploads = "outstanding_uploads"
        case outstandingDownloads = "outstanding_downloads"
        case waitingForDownload = "waiting_for_download"
        case waitingToApply = "waiting_to_apply"
        case blockedReason = "blocked_reason"
        case deviceNames = "device_names"
    }
}
