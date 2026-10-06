import Foundation

/// Whether the exchange could be used on the last attempt.
public enum ExchangeCondition: String, Codable, Sendable {
    /// The exchange was reached and every due transfer was attempted.
    case reachable
    /// No iCloud account is available; local work continues and waits.
    case accountUnavailable
    /// The folder grant lapsed and could not be renewed: pick it again.
    case needsFolderAccess
    /// The folder is another workspace's exchange.
    case foreignExchange
    /// The exchange identity file has not downloaded yet.
    case waitingForExchange
}

/// One outgoing transaction on its way to the exchange. Uploaded records the
/// exchange paths already written and read back intact, so a retry or a
/// restart resumes rather than starts over.
public struct UploadEntry: Codable, Equatable, Sendable {
    public var transaction: OutgoingTransaction
    public var uploaded: [String] = []
    public var attempts = 0
    public var nextAttempt: Date?
    public var lastError: String?
}

/// One remote transaction on its way into the engine. Staged records the
/// blobs already copied into the local inbox and verified; the manifest is
/// staged last, once nothing it names is missing.
public struct DownloadEntry: Codable, Equatable, Sendable {
    public var path: String
    public var txnID: String
    public var deviceID: String
    public var deviceSeq: UInt64
    public var staged: [String] = []
    public var manifestStaged = false
    /// Required content not yet delivered or not yet intact.
    public var missing: [String] = []
    /// Workspace paths whose new content is missing: shown as unavailable.
    public var unavailable: [String] = []
    public var attempts = 0
    public var nextAttempt: Date?
    public var lastError: String?
    /// The engine's last verdict once staged, such as "incomplete" or "held".
    public var outcome: String?
}

/// The transport's durable local queues and status, kept beside the engine's
/// runtime directory and rewritten atomically after every step.
public struct TransferState: Codable, Equatable, Sendable {
    public var format = "tt-sync-transport"
    public var formatVersion = 1
    public var workspaceID: String?
    public var condition: ExchangeCondition?
    public var uploads: [UploadEntry] = []
    public var downloads: [DownloadEntry] = []
    public var lastAttempt: Date?
    public var lastSuccessfulExchange: Date?
    public var lastError: String?

    public init() {}
}

/// Loads and saves the transfer state file.
public struct TransferStore: Sendable {
    public let file: URL

    public init(file: URL) {
        self.file = file
    }

    private static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }()

    private static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }()

    public func load() throws -> TransferState {
        guard FileManager.default.fileExists(atPath: file.path) else {
            return TransferState()
        }
        return try Self.decoder.decode(TransferState.self, from: Data(contentsOf: file))
    }

    public func save(_ state: TransferState) throws {
        try FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Self.encoder.encode(state).write(to: file, options: .atomic)
    }
}

/// Exponential backoff for a transfer that failed: the first retry after
/// base, doubling up to cap.
public struct RetryPolicy: Sendable {
    public var base: TimeInterval
    public var cap: TimeInterval

    public init(base: TimeInterval, cap: TimeInterval) {
        self.base = base
        self.cap = cap
    }

    public static let standard = RetryPolicy(base: 2, cap: 15 * 60)

    public func delay(afterAttempts attempts: Int) -> TimeInterval {
        min(cap, base * pow(2, Double(max(0, attempts - 1))))
    }
}
