import Foundation

/// What one exchange pass did and what it left waiting, reported apart so a
/// status surface never implies delivery from a local write.
public struct ExchangeReport: Codable, Equatable, Sendable {
    public var condition: ExchangeCondition
    /// Outgoing transactions written, read back intact, and acknowledged.
    public var published: [String] = []
    /// Incoming transactions the engine applied.
    public var imported: [String] = []
    /// Outgoing transactions still queued.
    public var outstandingUploads = 0
    /// Incoming transactions still queued: not downloaded, incomplete, or held.
    public var outstandingDownloads = 0
    /// Workspace paths whose incoming content has not arrived. The local files
    /// stay as they are until it does.
    public var unavailable: [String] = []
    /// Incoming transactions this engine holds until it is upgraded.
    public var held: [String] = []
    public var errors: [String] = []
    public var lastSuccessfulExchange: Date?
}

/// Moves one device's sync traffic between its engine and an exchange folder.
/// Uploads write every blob before the manifest that names them and confirm
/// each by reading it back before the engine is told it was published.
/// Downloads stage a manifest into the local inbox only after every blob it
/// names was read and verified, and the engine imports only what it can
/// verify again. Both queues are durable, so a failure, a lapsed grant, an
/// unavailable account, or a restart resumes exactly where it stopped, and
/// nothing is ever deleted from the exchange or the workspace because a file
/// is missing.
public final class ExchangeTransport: @unchecked Sendable {
    public static let root = "tt-sync/v1"

    public let provider: any ExchangeProvider
    public let engine: any SyncEngine
    public let store: TransferStore
    public let inbox: URL
    private let now: @Sendable () -> Date
    private let retry: RetryPolicy
    private let lock = NSLock()

    public init(
        provider: any ExchangeProvider,
        engine: any SyncEngine,
        stateDirectory: URL,
        now: @escaping @Sendable () -> Date = { Date() },
        retry: RetryPolicy = .standard
    ) {
        self.provider = provider
        self.engine = engine
        self.store = TransferStore(file: stateDirectory.appendingPathComponent("transport.json"))
        self.inbox = stateDirectory.appendingPathComponent("inbox", isDirectory: true)
        self.now = now
        self.retry = retry
    }

    /// The durable queues and status as last saved.
    public func state() throws -> TransferState {
        try store.load()
    }

    /// Runs one exchange pass. It throws only when its own local state cannot
    /// be read or written; everything about the exchange is reported.
    @discardableResult
    public func exchange() throws -> ExchangeReport {
        lock.lock()
        defer { lock.unlock() }
        var state = try store.load()
        if state.workspaceID == nil {
            state.workspaceID = engine.workspaceID
        }
        let started = now()
        state.lastAttempt = started
        var report = ExchangeReport(condition: .reachable)
        func finish() throws -> ExchangeReport {
            report.condition = state.condition ?? report.condition
            report.outstandingUploads = state.uploads.count
            report.outstandingDownloads = state.downloads.count
            report.unavailable = Array(Set(state.downloads.flatMap(\.unavailable))).sorted()
            report.held = state.downloads.filter { $0.outcome == "held" }.map(\.txnID)
            report.lastSuccessfulExchange = state.lastSuccessfulExchange
            try store.save(state)
            return report
        }

        var blocked = reachExchange(&report)
        if blocked == nil {
            blocked = try checkIdentity(&report)
        }
        if let condition = blocked {
            state.condition = condition
            state.lastError = report.errors.last
            return try finish()
        }
        try upload(&state, &report)
        try download(&state, &report)
        state.condition = .reachable
        if report.errors.isEmpty {
            state.lastSuccessfulExchange = started
            state.lastError = nil
        } else {
            state.lastError = report.errors.last
        }
        return try finish()
    }

    /// Asks for content now rather than on the queue's own schedule: every
    /// queued incoming transaction whose unavailable workspace paths include
    /// one of `paths` (every queued one when nil) has its retry delay cleared
    /// and each manifest or blob it still misses requested from the provider.
    /// Run an exchange afterwards to stage and import what arrived. Returns how
    /// many queued transactions were asked for.
    @discardableResult
    public func requestDownloads(paths: [String]? = nil) throws -> Int {
        lock.lock()
        defer { lock.unlock() }
        var state = try store.load()
        let wanted = paths.map(Set.init)
        var requested = 0
        for index in state.downloads.indices {
            var entry = state.downloads[index]
            if let wanted, wanted.isDisjoint(with: entry.unavailable) {
                continue
            }
            entry.nextAttempt = nil
            entry.attempts = 0
            for missing in entry.missing {
                let path = missing == entry.path ? missing : Self.blobPath(missing)
                try? provider.requestDownload("\(Self.root)/\(path)")
            }
            if entry.missing.isEmpty, !entry.manifestStaged {
                try? provider.requestDownload("\(Self.root)/\(entry.path)")
            }
            state.downloads[index] = entry
            requested += 1
        }
        try store.save(state)
        return requested
    }

    // MARK: Access

    /// Confirms the account and the grant, renewing an expired grant once.
    private func reachExchange(_ report: inout ExchangeReport) -> ExchangeCondition? {
        do {
            try provider.checkAccess()
            return nil
        } catch ExchangeProviderError.accountUnavailable(let detail) {
            report.errors.append("iCloud account unavailable: \(detail)")
            return .accountUnavailable
        } catch ExchangeProviderError.accessExpired {
            do {
                try provider.renewAccess()
                try provider.checkAccess()
                return nil
            } catch ExchangeProviderError.accountUnavailable(let detail) {
                report.errors.append("iCloud account unavailable: \(detail)")
                return .accountUnavailable
            } catch {
                report.errors.append("exchange folder access could not be renewed: \(error)")
                return .needsFolderAccess
            }
        } catch {
            report.errors.append("exchange folder unreachable: \(error)")
            return .needsFolderAccess
        }
    }

    private struct ExchangeIdentity: Codable {
        var format = "tt-sync-exchange"
        var formatVersion = 1
        var workspaceID: String

        enum CodingKeys: String, CodingKey {
            case format
            case formatVersion = "format_version"
            case workspaceID = "workspace_id"
        }
    }

    /// Creates the exchange identity, or confirms the folder is this
    /// workspace's.
    private func checkIdentity(_ report: inout ExchangeReport) throws -> ExchangeCondition? {
        let path = "\(Self.root)/workspace.json"
        do {
            let data = try provider.read(path)
            let identity = try JSONDecoder().decode(ExchangeIdentity.self, from: data)
            if identity.workspaceID != engine.workspaceID {
                report.errors.append("the exchange folder belongs to workspace \(identity.workspaceID)")
                return .foreignExchange
            }
            return nil
        } catch ExchangeProviderError.notFound {
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
            do {
                try provider.write(path, data: encoder.encode(ExchangeIdentity(workspaceID: engine.workspaceID)))
                return nil
            } catch {
                report.errors.append("creating the exchange identity: \(error)")
                return .waitingForExchange
            }
        } catch ExchangeProviderError.notDownloaded {
            try? provider.requestDownload(path)
            return .waitingForExchange
        } catch let error as DecodingError {
            report.errors.append("the exchange identity is unreadable: \(error)")
            return .waitingForExchange
        } catch {
            report.errors.append("reading the exchange identity: \(error)")
            return .waitingForExchange
        }
    }

    // MARK: Upload

    private func upload(_ state: inout TransferState, _ report: inout ExchangeReport) throws {
        let outbox: ExportedOutbox
        do {
            outbox = try engine.exportPending()
        } catch {
            report.errors.append("listing outgoing transactions: \(error)")
            return
        }
        let pending = Set(outbox.pending.map(\.txnID))
        state.uploads.removeAll { !pending.contains($0.transaction.txnID) }
        for transaction in outbox.pending where !state.uploads.contains(where: { $0.transaction.txnID == transaction.txnID }) {
            state.uploads.append(UploadEntry(transaction: transaction))
        }
        state.uploads.sort { $0.transaction.deviceSeq < $1.transaction.deviceSeq }
        try store.save(state)

        let index = 0 // the head: a published entry is removed, a failed one stops the pass
        while index < state.uploads.count {
            var entry = state.uploads[index]
            if let next = entry.nextAttempt, next > now() {
                break
            }
            do {
                for file in entry.transaction.blobs + [entry.transaction.manifest] where !entry.uploaded.contains(file.path) {
                    try publish(file)
                    entry.uploaded.append(file.path)
                    state.uploads[index] = entry
                    try store.save(state)
                }
                try engine.acknowledge([entry.transaction.txnID])
                report.published.append(entry.transaction.txnID)
                state.uploads.remove(at: index)
                try store.save(state)
            } catch {
                entry.attempts += 1
                entry.nextAttempt = now().addingTimeInterval(retry.delay(afterAttempts: entry.attempts))
                entry.lastError = String(describing: error)
                state.uploads[index] = entry
                try store.save(state)
                report.errors.append("publishing \(entry.transaction.txnID): \(error)")
                // Later transactions wait: a peer could not apply them before
                // this one anyway.
                break
            }
        }
    }

    /// Writes one file unless an intact copy is already there, then confirms
    /// it by reading it back.
    private func publish(_ file: OutgoingFile) throws {
        let local = try Data(contentsOf: URL(fileURLWithPath: file.source))
        guard contentDigest(local) == file.digest else {
            throw ExchangeProviderError.transient("local copy of \(file.path) does not match \(file.digest)")
        }
        let path = "\(Self.root)/\(file.path)"
        if let existing = try? provider.read(path), contentDigest(existing) == file.digest {
            return
        }
        try provider.write(path, data: local)
        let back = try provider.read(path)
        guard contentDigest(back) == file.digest else {
            throw ExchangeProviderError.transient("\(file.path) did not read back intact")
        }
    }

    // MARK: Download

    private static let manifestPattern = try! NSRegularExpression(pattern: "^txns/([A-Za-z0-9][A-Za-z0-9._-]{0,63})/([0-9]{10})-([0-9a-f]{64})\\.json$")

    private func download(_ state: inout TransferState, _ report: inout ExchangeReport) throws {
        let status: EngineSyncStatus
        let manifests: [ExchangeItem]
        var blobs: [String: ExchangeItem] = [:]
        do {
            status = try engine.status()
            manifests = try provider.list(directory: "\(Self.root)/txns")
            for item in try provider.list(directory: "\(Self.root)/blobs") {
                blobs[relative(item.path)] = item
            }
        } catch {
            report.errors.append("listing the exchange: \(error)")
            return
        }
        var listed: [String: ExchangeItem] = [:]
        for item in manifests {
            let path = relative(item.path)
            guard let (device, seq, txnID) = Self.parseManifestPath(path) else { continue }
            listed[path] = item
            if device == status.deviceID || seq <= status.received[device, default: 0] {
                continue
            }
            if !state.downloads.contains(where: { $0.path == path }) {
                state.downloads.append(DownloadEntry(path: path, txnID: txnID, deviceID: device, deviceSeq: seq))
            }
        }
        // What the engine already received needs nothing more.
        for entry in state.downloads where entry.deviceSeq <= status.received[entry.deviceID, default: 0] {
            try? FileManager.default.removeItem(at: inboxURL(entry.path))
        }
        state.downloads.removeAll { $0.deviceSeq <= status.received[$0.deviceID, default: 0] }
        state.downloads.sort { ($0.deviceSeq, $0.deviceID) < ($1.deviceSeq, $1.deviceID) }
        try store.save(state)

        var stagedAny = false
        for index in state.downloads.indices {
            var entry = state.downloads[index]
            if entry.manifestStaged {
                stagedAny = true
                continue
            }
            if let next = entry.nextAttempt, next > now() {
                continue
            }
            stage(&entry, listed: listed[entry.path], blobs: blobs, report: &report)
            stagedAny = stagedAny || entry.manifestStaged
            state.downloads[index] = entry
            try store.save(state)
        }
        guard stagedAny else { return }

        let imported: ImportReport
        do {
            imported = try engine.importInbox(inbox)
        } catch {
            report.errors.append("importing staged transactions: \(error)")
            return
        }
        for result in imported.results {
            guard let index = state.downloads.firstIndex(where: { $0.path == result.path }) else {
                // A staged copy nothing queues any more is spent.
                if result.outcome == "imported" || result.outcome == "known" {
                    try? FileManager.default.removeItem(at: inboxURL(result.path))
                }
                continue
            }
            var entry = state.downloads[index]
            entry.outcome = result.outcome
            switch result.outcome {
            case "imported", "known":
                if result.outcome == "imported" {
                    report.imported.append(result.txnID)
                }
                try? FileManager.default.removeItem(at: inboxURL(result.path))
                entry.manifestStaged = false
                entry.staged = []
            case "invalid":
                // The copy is not the transaction its name promises: fetch it
                // again later rather than trust it.
                try? FileManager.default.removeItem(at: inboxURL(result.path))
                entry.manifestStaged = false
                entry.attempts += 1
                entry.nextAttempt = now().addingTimeInterval(retry.delay(afterAttempts: entry.attempts))
                entry.lastError = result.reason
                report.errors.append("\(result.path) is invalid: \(result.reason ?? "")")
            case "incomplete", "held":
                entry.lastError = result.reason
                entry.unavailable = result.unavailable ?? entry.unavailable
            default:
                entry.attempts += 1
                entry.nextAttempt = now().addingTimeInterval(retry.delay(afterAttempts: entry.attempts))
                entry.lastError = result.reason
                report.errors.append("importing \(result.path): \(result.outcome): \(result.reason ?? "")")
            }
            state.downloads[index] = entry
        }
        state.downloads.removeAll { $0.outcome == "imported" || $0.outcome == "known" }
        try store.save(state)
        collectInboxBlobs(state)
    }

    /// Copies a manifest's content into the inbox, verifying every blob, and
    /// stages the manifest itself only when nothing it names is missing.
    private func stage(_ entry: inout DownloadEntry, listed: ExchangeItem?, blobs: [String: ExchangeItem], report: inout ExchangeReport) {
        let manifestPath = "\(Self.root)/\(entry.path)"
        guard let listed else {
            entry.missing = [entry.path]
            return
        }
        if listed.state != .current {
            try? provider.requestDownload(manifestPath)
            entry.missing = [entry.path]
            return
        }
        let manifestData: Data
        let effects: [(digests: [String], path: String, newBlob: String?)]
        do {
            manifestData = try provider.read(manifestPath)
            effects = try Self.effects(of: manifestData)
        } catch ExchangeProviderError.notDownloaded {
            try? provider.requestDownload(manifestPath)
            entry.missing = [entry.path]
            return
        } catch {
            // A manifest still arriving can be truncated; it is retried.
            entry.attempts += 1
            entry.nextAttempt = now().addingTimeInterval(retry.delay(afterAttempts: entry.attempts))
            entry.lastError = "manifest not readable yet: \(error)"
            entry.missing = [entry.path]
            return
        }
        var missing: [String] = []
        for digest in Set(effects.flatMap(\.digests)).sorted() where !entry.staged.contains(digest) {
            let path = Self.blobPath(digest)
            guard let item = blobs[path] else {
                missing.append(digest)
                continue
            }
            if item.state != .current {
                try? provider.requestDownload("\(Self.root)/\(path)")
                missing.append(digest)
                continue
            }
            do {
                let data = try provider.read("\(Self.root)/\(path)")
                guard contentDigest(data) == digest else {
                    // An incomplete transfer: never content, retried later.
                    missing.append(digest)
                    continue
                }
                try writeInbox(path, data: data)
                entry.staged.append(digest)
            } catch ExchangeProviderError.notDownloaded {
                try? provider.requestDownload("\(Self.root)/\(path)")
                missing.append(digest)
            } catch {
                missing.append(digest)
                entry.lastError = "\(path): \(error)"
            }
        }
        entry.missing = missing
        let absent = Set(missing)
        entry.unavailable = effects.compactMap { effect in
            guard let blob = effect.newBlob, absent.contains(blob) else { return nil }
            return effect.path
        }.sorted()
        if missing.isEmpty {
            do {
                try writeInbox(entry.path, data: manifestData)
                entry.manifestStaged = true
                entry.lastError = nil
            } catch {
                entry.lastError = "staging \(entry.path): \(error)"
                report.errors.append(entry.lastError!)
            }
        }
    }

    /// Removes staged blobs that no queued transaction still needs.
    private func collectInboxBlobs(_ state: TransferState) {
        let needed = Set(state.downloads.flatMap(\.staged).map(Self.blobPath))
        let blobsDir = inbox.appendingPathComponent("blobs", isDirectory: true)
        guard let enumerator = FileManager.default.enumerator(at: blobsDir, includingPropertiesForKeys: [.isRegularFileKey]) else { return }
        for case let file as URL in enumerator {
            let hex = file.lastPathComponent
            guard hex.count == 64 else { continue }
            if !needed.contains(Self.blobPath("sha256:" + hex)) {
                try? FileManager.default.removeItem(at: file)
            }
        }
    }

    // MARK: Paths

    private func relative(_ path: String) -> String {
        let prefix = Self.root + "/"
        return path.hasPrefix(prefix) ? String(path.dropFirst(prefix.count)) : path
    }

    private func inboxURL(_ path: String) -> URL {
        path.split(separator: "/").reduce(inbox) { $0.appendingPathComponent(String($1)) }
    }

    private func writeInbox(_ path: String, data: Data) throws {
        let url = inboxURL(path)
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try data.write(to: url, options: .atomic)
    }

    public static func blobPath(_ digest: String) -> String {
        let hex = digest.hasPrefix("sha256:") ? String(digest.dropFirst(7)) : digest
        return "blobs/sha256/\(hex.prefix(2))/\(hex)"
    }

    static func parseManifestPath(_ path: String) -> (String, UInt64, String)? {
        let range = NSRange(path.startIndex..., in: path)
        guard let match = manifestPattern.firstMatch(in: path, range: range),
              let device = Range(match.range(at: 1), in: path),
              let seq = Range(match.range(at: 2), in: path),
              let hex = Range(match.range(at: 3), in: path),
              let number = UInt64(path[seq]), number > 0
        else { return nil }
        return (String(path[device]), number, "sha256:" + path[hex])
    }

    /// Each effect's required digests, the workspace path its new content
    /// lands at, and that content's digest.
    static func effects(of manifest: Data) throws -> [(digests: [String], path: String, newBlob: String?)] {
        guard let object = try JSONSerialization.jsonObject(with: manifest) as? [String: Any],
              let effects = object["effects"] as? [[String: Any]]
        else {
            throw ExchangeProviderError.transient("manifest has no effects")
        }
        return effects.map { effect in
            let base = effect["base_blob"] as? String
            let blob = effect["blob"] as? String
            let target = (effect["target_path"] as? String).flatMap { $0.isEmpty ? nil : $0 }
            return ([base, blob].compactMap { $0 }, target ?? (effect["path"] as? String ?? ""), blob)
        }
    }
}
