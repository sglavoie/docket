import CryptoKit
import Foundation

/// Where one file of the exchange stands on this device.
public enum ExchangeItemState: String, Codable, Sendable {
    /// The bytes are local and readable.
    case current
    /// Only a placeholder is local; the content has to be downloaded first.
    case notDownloaded
    /// A download is in progress.
    case downloading
}

/// One file of the exchange as a listing reports it.
public struct ExchangeItem: Equatable, Sendable {
    /// The path relative to the provider's root, '/'-separated.
    public let path: String
    public let size: Int64?
    public let state: ExchangeItemState

    public init(path: String, size: Int64?, state: ExchangeItemState) {
        self.path = path
        self.size = size
        self.state = state
    }
}

/// Why the exchange could not be reached or read. None of them means a file
/// was deleted: a missing or undownloaded file is not yet delivered.
public enum ExchangeProviderError: Error, Equatable, Sendable, CustomStringConvertible {
    /// No iCloud account is signed in, or iCloud Drive is off.
    case accountUnavailable(String)
    /// The folder grant lapsed or the folder moved out of reach.
    case accessExpired(String)
    /// The file exists but only as a placeholder.
    case notDownloaded(String)
    /// The file does not exist (yet).
    case notFound(String)
    /// A failure worth retrying later.
    case transient(String)

    public var description: String {
        switch self {
        case .accountUnavailable(let detail): "iCloud account unavailable: \(detail)"
        case .accessExpired(let detail): "exchange folder access expired: \(detail)"
        case .notDownloaded(let path): "\(path) is not downloaded yet"
        case .notFound(let path): "\(path) does not exist"
        case .transient(let detail): detail
        }
    }
}

/// The file operations the transport needs from an exchange folder. Every
/// path is relative to the provider's root. A provider never deletes: the
/// exchange is an append-only log of immutable files.
public protocol ExchangeProvider: Sendable {
    /// Confirms the account and the folder grant, throwing accountUnavailable
    /// or accessExpired.
    func checkAccess() throws
    /// Renews the folder grant after accessExpired.
    func renewAccess() throws
    /// Every file beneath directory, recursively. A missing directory lists
    /// nothing.
    func list(directory: String) throws -> [ExchangeItem]
    /// Reads a file whose content is local, throwing notDownloaded for a
    /// placeholder and notFound for a missing file.
    func read(_ path: String) throws -> Data
    /// Writes a whole file, replacing whatever an interrupted attempt left.
    func write(_ path: String, data: Data) throws
    /// Asks for a placeholder's content without waiting for it.
    func requestDownload(_ path: String) throws
}

/// A provider rooted at a subdirectory of another, so one disposable run can
/// use its own folder inside the folder the user picked.
public struct ScopedProvider: ExchangeProvider {
    public let base: any ExchangeProvider
    public let prefix: String

    public init(base: any ExchangeProvider, prefix: String) {
        self.base = base
        self.prefix = prefix.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
    }

    private func full(_ path: String) -> String { path.isEmpty ? prefix : "\(prefix)/\(path)" }

    public func checkAccess() throws { try base.checkAccess() }
    public func renewAccess() throws { try base.renewAccess() }
    public func list(directory: String) throws -> [ExchangeItem] {
        try base.list(directory: full(directory)).compactMap { item in
            guard item.path.hasPrefix(prefix + "/") else { return nil }
            return ExchangeItem(path: String(item.path.dropFirst(prefix.count + 1)), size: item.size, state: item.state)
        }
    }
    public func read(_ path: String) throws -> Data { try base.read(full(path)) }
    public func write(_ path: String, data: Data) throws { try base.write(full(path), data: data) }
    public func requestDownload(_ path: String) throws { try base.requestDownload(full(path)) }
}

/// The content address the engine names blobs by: "sha256:" and the hex
/// digest.
public func contentDigest(_ data: Data) -> String {
    "sha256:" + SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
}
