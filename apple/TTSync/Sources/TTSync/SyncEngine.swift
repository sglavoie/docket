import Foundation

/// One immutable file the engine asks the transport to publish.
public struct OutgoingFile: Codable, Equatable, Sendable {
    /// The place under the exchange's tt-sync/v1 root.
    public let path: String
    /// The local file holding the bytes.
    public let source: String
    public let digest: String
    public let size: Int64
}

/// One outgoing transaction: blobs first, manifest last.
public struct OutgoingTransaction: Codable, Equatable, Sendable {
    public let txnID: String
    public let deviceSeq: UInt64
    public let manifest: OutgoingFile
    public let blobs: [OutgoingFile]

    enum CodingKeys: String, CodingKey {
        case txnID = "txn_id"
        case deviceSeq = "device_seq"
        case manifest, blobs
    }
}

public struct ExportedOutbox: Decodable, Sendable {
    public let deviceID: String
    public let pending: [OutgoingTransaction]

    enum CodingKeys: String, CodingKey {
        case deviceID = "device_id"
        case pending
    }
}

/// The engine's verdict on one staged manifest (internal/syncexchange).
public struct ImportResult: Codable, Equatable, Sendable {
    public let path: String
    public let txnID: String
    public let deviceID: String
    public let deviceSeq: UInt64
    public let outcome: String
    public let reason: String?
    public let localTxnID: String?
    public let missing: [String]?
    public let unavailable: [String]?

    enum CodingKeys: String, CodingKey {
        case path, outcome, reason, missing, unavailable
        case txnID = "txn_id"
        case deviceID = "device_id"
        case deviceSeq = "device_seq"
        case localTxnID = "local_txn_id"
    }
}

public struct ImportReport: Decodable, Sendable {
    public let results: [ImportResult]
}

public struct EngineSyncStatus: Decodable, Sendable {
    public struct OutboxEntry: Decodable, Sendable {
        public let txnID: String
        public let deviceSeq: UInt64
        enum CodingKeys: String, CodingKey {
            case txnID = "txn_id"
            case deviceSeq = "device_seq"
        }
    }

    public let deviceID: String
    public let heads: [String]
    public let outbox: [OutboxEntry]
    public let received: [String: UInt64]
    public let conflicts: [String]

    enum CodingKeys: String, CodingKey {
        case deviceID = "device_id"
        case heads, outbox, received, conflicts
    }
}

/// The engine operations the transport drives: the sync.* ops of the bridge
/// contract.
public protocol SyncEngine: Sendable {
    var workspaceID: String { get }
    func exportPending() throws -> ExportedOutbox
    func importInbox(_ directory: URL) throws -> ImportReport
    func acknowledge(_ txnIDs: [String]) throws
    func status() throws -> EngineSyncStatus
}

/// A refusal the engine answered with.
public struct BridgeFailure: Error, CustomStringConvertible, Sendable {
    public let op: String
    public let code: String
    public let message: String
    public var description: String { "\(op): \(code): \(message)" }
}

/// Speaks the bridge's JSON protocol over any `(request) -> response`
/// function: the gomobile-bound engine on iOS, or a bridgehost process on the
/// Mac. It addresses the one workspace it opened.
public final class BridgeClient: @unchecked Sendable {
    public typealias Call = @Sendable (String) -> String

    private let call: Call
    private let lock = NSLock()
    private var counter = 0
    public private(set) var workspace: String?

    public init(call: @escaping Call) {
        self.call = call
    }

    /// Opens a workspace and addresses every later request to it.
    @discardableResult
    public func open(dataDirectory: URL, runtimeDirectory: URL, deviceID: String, createIfMissing: Bool = false) throws -> [String: Any] {
        let result = try send("workspace.open", payload: [
            "data_dir": dataDirectory.path,
            "runtime_dir": runtimeDirectory.path,
            "device_id": deviceID,
            "create_if_missing": createIfMissing,
        ], addressed: false)
        guard let object = try JSONSerialization.jsonObject(with: result) as? [String: Any],
              let workspace = object["workspace"] as? String
        else {
            throw BridgeFailure(op: "workspace.open", code: "malformed", message: "no workspace in the reply")
        }
        lock.lock()
        self.workspace = workspace
        lock.unlock()
        return object
    }

    /// Sends one op and returns its raw result JSON.
    public func send(_ op: String, payload: [String: Any]? = nil, addressed: Bool = true) throws -> Data {
        lock.lock()
        counter += 1
        let requestID = "ttsync-\(UUID().uuidString.lowercased())-\(counter)"
        let workspace = self.workspace
        lock.unlock()
        var envelope: [String: Any] = ["bridge_version": 1, "request_id": requestID, "op": op]
        if addressed {
            guard let workspace else { throw BridgeFailure(op: op, code: "not_open", message: "no workspace is open") }
            envelope["workspace"] = workspace
        }
        if let payload {
            envelope["payload"] = payload
        }
        let request = try JSONSerialization.data(withJSONObject: envelope)
        let reply = call(String(decoding: request, as: UTF8.self))
        guard let object = try? JSONSerialization.jsonObject(with: Data(reply.utf8)) as? [String: Any] else {
            throw BridgeFailure(op: op, code: "malformed", message: "reply is not JSON: \(reply.prefix(200))")
        }
        if let error = object["error"] as? [String: Any] {
            throw BridgeFailure(op: op, code: error["code"] as? String ?? "unknown", message: error["message"] as? String ?? "")
        }
        guard object["ok"] as? Bool == true, object["request_id"] as? String == requestID, let result = object["result"] else {
            throw BridgeFailure(op: op, code: "malformed", message: "reply does not answer \(requestID)")
        }
        return try JSONSerialization.data(withJSONObject: result)
    }

    public func decode<T: Decodable>(_ type: T.Type, _ op: String, payload: [String: Any]? = nil) throws -> T {
        try JSONDecoder().decode(type, from: send(op, payload: payload))
    }
}

/// The transport's engine over the bridge's sync ops.
public struct BridgeSyncEngine: SyncEngine {
    public let client: BridgeClient
    public let workspaceID: String

    public init(client: BridgeClient) throws {
        guard let workspace = client.workspace else {
            throw BridgeFailure(op: "sync", code: "not_open", message: "open a workspace first")
        }
        self.client = client
        self.workspaceID = workspace
    }

    public func exportPending() throws -> ExportedOutbox {
        try client.decode(ExportedOutbox.self, "sync.export_pending")
    }

    public func importInbox(_ directory: URL) throws -> ImportReport {
        try client.decode(ImportReport.self, "sync.import", payload: ["inbox_dir": directory.path])
    }

    public func acknowledge(_ txnIDs: [String]) throws {
        _ = try client.send("sync.acknowledge", payload: ["txn_ids": txnIDs])
    }

    public func status() throws -> EngineSyncStatus {
        try client.decode(EngineSyncStatus.self, "sync.status")
    }
}
