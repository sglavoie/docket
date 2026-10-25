import Foundation

/// The exchange folder on disk, read and written through NSFileCoordinator so
/// iCloud Drive (or any file presenter) never observes a half-written file and
/// never hands back one it is still replacing. Placeholders of files iCloud
/// has not downloaded are listed as notDownloaded and are never read through,
/// since a coordinated read of a dataless file would block on the network.
public final class CoordinatedFolderProvider: ExchangeProvider, @unchecked Sendable {
    public let access: any FolderAccess
    private let fileManager = FileManager.default

    public init(access: any FolderAccess) {
        self.access = access
    }

    private func root() throws -> URL {
        try access.url()
    }

    private func url(_ path: String) throws -> URL {
        var url = try root()
        for component in path.split(separator: "/") where !component.isEmpty {
            url.appendPathComponent(String(component))
        }
        return url
    }

    public func checkAccess() throws {
        let root = try root()
        if fileManager.isUbiquitousItem(at: root), fileManager.ubiquityIdentityToken == nil {
            throw ExchangeProviderError.accountUnavailable("no iCloud account is signed in with iCloud Drive on")
        }
        var isDirectory: ObjCBool = false
        guard fileManager.fileExists(atPath: root.path, isDirectory: &isDirectory), isDirectory.boolValue else {
            throw ExchangeProviderError.accessExpired("\(root.path) is not reachable")
        }
        guard fileManager.isReadableFile(atPath: root.path), fileManager.isWritableFile(atPath: root.path) else {
            throw ExchangeProviderError.accessExpired("\(root.path) is not readable and writable")
        }
    }

    public func renewAccess() throws {
        try access.renew()
    }

    public func list(directory: String) throws -> [ExchangeItem] {
        var items: [ExchangeItem] = []
        try walk(try url(directory), relative: directory, into: &items)
        return items.sorted { $0.path < $1.path }
    }

    private static let listingKeys: [URLResourceKey] = [
        .isRegularFileKey, .isDirectoryKey, .fileSizeKey, .isUbiquitousItemKey, .ubiquitousItemDownloadingStatusKey,
    ]

    /// Lists one directory under its own coordinated read — which is what asks
    /// a file provider such as iCloud Drive to materialize the directory's
    /// entries, placeholders included — and recurses into its subdirectories.
    private func walk(_ dir: URL, relative: String, into items: inout [ExchangeItem]) throws {
        var children: [URL] = []
        var failure: Error?
        try coordinate { coordinator, error in
            coordinator.coordinate(readingItemAt: dir, options: [], error: error) { readable in
                do {
                    children = try self.fileManager.contentsOfDirectory(at: readable, includingPropertiesForKeys: Self.listingKeys, options: [])
                } catch let error as NSError where error.domain == NSCocoaErrorDomain && (error.code == NSFileReadNoSuchFileError || error.code == NSFileNoSuchFileError) {
                    children = []
                } catch {
                    failure = error
                }
            }
        }
        if let failure {
            throw ExchangeProviderError.transient("listing \(relative): \(failure.localizedDescription)")
        }
        let prefix = relative.isEmpty ? "" : relative + "/"
        for child in children {
            let values = try? child.resourceValues(forKeys: Set(Self.listingKeys))
            var name = child.lastPathComponent
            if values?.isDirectory == true {
                if !name.hasPrefix(".") {
                    try walk(child, relative: prefix + name, into: &items)
                }
                continue
            }
            var state = ExchangeItemState.current
            if name.hasPrefix("."), name.hasSuffix(".icloud") {
                // A legacy placeholder stands for ".name.icloud".
                name = String(name.dropFirst().dropLast(".icloud".count))
                state = .notDownloaded
            } else if name.hasPrefix(".") || values?.isRegularFile != true {
                continue
            } else if values?.isUbiquitousItem == true {
                switch values?.ubiquitousItemDownloadingStatus {
                case .some(.current), .some(.downloaded): state = .current
                default: state = .notDownloaded
                }
            }
            items.append(ExchangeItem(path: prefix + name, size: values?.fileSize.map(Int64.init), state: state))
        }
    }

    public func read(_ path: String) throws -> Data {
        let file = try url(path)
        if let state = try downloadState(file), state != .current {
            throw ExchangeProviderError.notDownloaded(path)
        }
        guard fileManager.fileExists(atPath: file.path) else {
            throw ExchangeProviderError.notFound(path)
        }
        var data: Data?
        var readError: Error?
        try coordinate { coordinator, error in
            coordinator.coordinate(readingItemAt: file, options: [.withoutChanges], error: error) { readable in
                do {
                    data = try Data(contentsOf: readable)
                } catch {
                    readError = error
                }
            }
        }
        if let readError {
            throw ExchangeProviderError.transient("reading \(path): \(readError.localizedDescription)")
        }
        guard let data else { throw ExchangeProviderError.transient("reading \(path) returned nothing") }
        return data
    }

    public func write(_ path: String, data: Data) throws {
        let file = try url(path)
        do {
            try fileManager.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        } catch {
            throw ExchangeProviderError.transient("creating the folder for \(path): \(error.localizedDescription)")
        }
        var writeError: Error?
        try coordinate { coordinator, error in
            coordinator.coordinate(writingItemAt: file, options: [.forReplacing], error: error) { writable in
                do {
                    try data.write(to: writable, options: .atomic)
                } catch {
                    writeError = error
                }
            }
        }
        if let writeError {
            throw ExchangeProviderError.transient("writing \(path): \(writeError.localizedDescription)")
        }
    }

    public func requestDownload(_ path: String) throws {
        let file = try url(path)
        guard fileManager.isUbiquitousItem(at: file) else { return }
        do {
            try fileManager.startDownloadingUbiquitousItem(at: file)
        } catch {
            throw ExchangeProviderError.transient("requesting \(path): \(error.localizedDescription)")
        }
    }

    /// The iCloud download state of a ubiquitous file, or nil for a local one.
    private func downloadState(_ file: URL) throws -> ExchangeItemState? {
        guard fileManager.isUbiquitousItem(at: file) else { return nil }
        let values = try? file.resourceValues(forKeys: [.ubiquitousItemDownloadingStatusKey])
        switch values?.ubiquitousItemDownloadingStatus {
        case .some(.current), .some(.downloaded): return .current
        case .none: return nil
        default: return .notDownloaded
        }
    }

    /// Runs one coordination and turns its NSError into a provider error.
    private func coordinate(_ body: (NSFileCoordinator, NSErrorPointer) -> Void) throws {
        let coordinator = NSFileCoordinator(filePresenter: nil)
        var error: NSError?
        body(coordinator, &error)
        if let error {
            if error.domain == NSCocoaErrorDomain, error.code == NSFileReadNoSuchFileError || error.code == NSFileNoSuchFileError {
                return
            }
            if error.domain == NSCocoaErrorDomain, error.code == NSFileReadNoPermissionError || error.code == NSFileWriteNoPermissionError {
                throw ExchangeProviderError.accessExpired(error.localizedDescription)
            }
            throw ExchangeProviderError.transient(error.localizedDescription)
        }
    }
}
