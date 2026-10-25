import Foundation

/// How the transport reaches its exchange folder's URL.
public protocol FolderAccess: Sendable {
    /// The folder, with any security scope it needs already entered.
    func url() throws -> URL
    /// Re-establishes the grant after it lapsed.
    func renew() throws
}

/// A folder this process may always reach, such as one a non-sandboxed macOS
/// helper was given by path.
public struct DirectFolderAccess: FolderAccess {
    public let folder: URL

    public init(folder: URL) {
        self.folder = folder
    }

    public func url() throws -> URL { folder }
    public func renew() throws {}
}

/// A folder the user picked through the document picker, retained across
/// launches as a bookmark. Resolving a stale bookmark rewrites it from the
/// resolved URL, which is what renews access before it is lost; a bookmark
/// that no longer resolves reports accessExpired, so the app asks the user to
/// pick the folder again rather than guessing.
public final class BookmarkFolderAccess: FolderAccess, @unchecked Sendable {
    public let bookmarkFile: URL
    private let lock = NSLock()
    private var resolved: URL?
    private var scoped = false

    public init(bookmarkFile: URL) {
        self.bookmarkFile = bookmarkFile
    }

    deinit {
        if scoped {
            resolved?.stopAccessingSecurityScopedResource()
        }
    }

    /// Persists access to a folder the document picker returned.
    public static func save(folder: URL, to bookmarkFile: URL) throws {
        let entered = folder.startAccessingSecurityScopedResource()
        defer {
            if entered { folder.stopAccessingSecurityScopedResource() }
        }
        let data = try folder.bookmarkData(options: bookmarkCreationOptions, includingResourceValuesForKeys: nil, relativeTo: nil)
        try FileManager.default.createDirectory(at: bookmarkFile.deletingLastPathComponent(), withIntermediateDirectories: true)
        try data.write(to: bookmarkFile, options: .atomic)
    }

    #if os(macOS)
    private static let bookmarkCreationOptions: URL.BookmarkCreationOptions = []
    private static let bookmarkResolutionOptions: URL.BookmarkResolutionOptions = []
    #else
    private static let bookmarkCreationOptions: URL.BookmarkCreationOptions = .minimalBookmark
    private static let bookmarkResolutionOptions: URL.BookmarkResolutionOptions = []
    #endif

    public func url() throws -> URL {
        lock.lock()
        defer { lock.unlock() }
        if let resolved {
            return resolved
        }
        return try resolveLocked()
    }

    public func renew() throws {
        lock.lock()
        defer { lock.unlock() }
        if scoped {
            resolved?.stopAccessingSecurityScopedResource()
        }
        resolved = nil
        scoped = false
        _ = try resolveLocked()
    }

    private func resolveLocked() throws -> URL {
        let data: Data
        do {
            data = try Data(contentsOf: bookmarkFile)
        } catch {
            throw ExchangeProviderError.accessExpired("no exchange folder has been picked")
        }
        var stale = false
        let folder: URL
        do {
            folder = try URL(resolvingBookmarkData: data, options: Self.bookmarkResolutionOptions, relativeTo: nil, bookmarkDataIsStale: &stale)
        } catch {
            throw ExchangeProviderError.accessExpired("the exchange folder bookmark no longer resolves: \(error.localizedDescription)")
        }
        scoped = folder.startAccessingSecurityScopedResource()
        if stale {
            // Renewal: a stale bookmark still resolved, so record a fresh one
            // while access holds.
            if let fresh = try? folder.bookmarkData(options: Self.bookmarkCreationOptions, includingResourceValuesForKeys: nil, relativeTo: nil) {
                try? fresh.write(to: bookmarkFile, options: .atomic)
            }
        }
        resolved = folder
        return folder
    }
}

/// Where an app keeps the bookmark of the exchange folder its user picked:
/// beside its other local state, never inside the exchange or the workspace.
public enum ExchangeFolderBookmark {
    public static func defaultURL() throws -> URL {
        try FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
            .appendingPathComponent("exchange-folder.bookmark")
    }
}
