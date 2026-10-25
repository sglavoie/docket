import Foundation

/// Why a join could not start.
public enum JoinError: Error, Equatable, Sendable, CustomStringConvertible {
    /// The folder holds no Docket exchange: no device has enrolled a workspace in it.
    case notAnExchange
    /// The exchange identity is still a placeholder iCloud has not downloaded.
    case identityNotDownloaded
    /// The exchange identity could not be read.
    case unreadableIdentity(String)

    public var description: String {
        switch self {
        case .notAnExchange: "this folder holds no Docket exchange; enroll the workspace on the Mac first"
        case .identityNotDownloaded: "the exchange folder is still downloading; try again shortly"
        case .unreadableIdentity(let detail): "the exchange identity is unreadable: \(detail)"
        }
    }
}

/// The engine side of a device joining an enrolled workspace through the
/// ordinary transport. Until the join, it publishes nothing and has received
/// nothing, so one exchange pass downloads every transaction the exchange
/// holds and stages it; its first import is `workspace.join`, which builds and
/// opens the working copy from that history alone. After that it is the open
/// workspace's ordinary sync engine. A join that delivers no workspace removes
/// the directory it was given, provided it did not exist before, so the next
/// attempt starts clean.
public final class JoiningSyncEngine: SyncEngine, @unchecked Sendable {
    public let client: BridgeClient
    public let workspaceID: String
    public let dataDirectory: URL
    public let runtimeDirectory: URL
    public let deviceID: String
    public let exchange: String
    private let ownsDirectory: Bool
    private let lock = NSLock()
    private var joinedWorkspace = false

    public init(
        client: BridgeClient, workspaceID: String, dataDirectory: URL,
        runtimeDirectory: URL, deviceID: String, exchange: String
    ) {
        self.client = client
        self.workspaceID = workspaceID
        self.dataDirectory = dataDirectory
        self.runtimeDirectory = runtimeDirectory
        self.deviceID = deviceID
        self.exchange = exchange
        ownsDirectory = !FileManager.default.fileExists(atPath: dataDirectory.path)
    }

    /// Whether the working copy exists and is open.
    public var joined: Bool {
        lock.lock()
        defer { lock.unlock() }
        return joinedWorkspace
    }

    public var isJoining: Bool { !joined }

    /// The workspace an exchange folder declares.
    public static func workspaceID(in provider: any ExchangeProvider) throws -> String {
        struct Identity: Decodable {
            var workspaceID: String
            enum CodingKeys: String, CodingKey { case workspaceID = "workspace_id" }
        }
        let path = "\(ExchangeTransport.root)/workspace.json"
        do {
            return try JSONDecoder().decode(Identity.self, from: provider.read(path)).workspaceID
        } catch ExchangeProviderError.notFound {
            throw JoinError.notAnExchange
        } catch ExchangeProviderError.notDownloaded {
            try? provider.requestDownload(path)
            throw JoinError.identityNotDownloaded
        } catch let error as DecodingError {
            throw JoinError.unreadableIdentity(String(describing: error))
        }
    }

    public func exportPending() throws -> ExportedOutbox {
        guard joined else { return ExportedOutbox(deviceID: deviceID, pending: []) }
        return try client.decode(ExportedOutbox.self, "sync.export_pending")
    }

    public func importInbox(_ directory: URL) throws -> ImportReport {
        if joined {
            return try client.decode(ImportReport.self, "sync.import", payload: ["inbox_dir": directory.path])
        }
        do {
            let reply = try client.join(
                dataDirectory: dataDirectory, runtimeDirectory: runtimeDirectory, deviceID: deviceID,
                workspaceID: workspaceID, exchange: exchange, inbox: directory
            )
            lock.lock()
            joinedWorkspace = true
            lock.unlock()
            let report = reply["import"] ?? ["results": []]
            return try JSONDecoder().decode(ImportReport.self, from: JSONSerialization.data(withJSONObject: report))
        } catch {
            if ownsDirectory {
                try? FileManager.default.removeItem(at: dataDirectory)
            }
            throw error
        }
    }

    public func acknowledge(_ txnIDs: [String]) throws {
        guard joined else { return }
        _ = try client.send("sync.acknowledge", payload: ["txn_ids": txnIDs])
    }

    public func status() throws -> EngineSyncStatus {
        guard joined else { return EngineSyncStatus(deviceID: deviceID) }
        return try client.decode(EngineSyncStatus.self, "sync.status")
    }
}
