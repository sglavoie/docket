import Foundation
import DocketSync

/// A deterministic stand-in for iCloud Drive shared by several devices. A file
/// one device writes reaches the others only after an upload lag, then as a
/// placeholder until that device asks for it and a download lag passes. Faults
/// are switched on by the test: an unavailable account, an expired grant that
/// may or may not renew, transient write failures, and truncated reads of a
/// file still in transfer. It records every write in order, so a test can
/// prove a manifest never landed before the blobs it names.
final class FakeCloud: @unchecked Sendable {
    struct File {
        var data: Data
        var writer: String
        var visibleAt: Int
    }

    private let lock = NSLock()
    private var files: [String: File] = [:]
    private var downloaded: [String: [String: Int]] = [:] // device -> path -> ready tick
    private(set) var tick = 0
    var uploadLag = 0
    var downloadLag = 0
    var accountAvailable = true
    var expired: Set<String> = []
    var renewable = true
    var renewals = 0
    var failWrites = 0
    /// Fails every write once this many writes have landed.
    var failWritesAfter: Int?
    var truncatedReads: [String: Int] = [:]
    /// Serves the first remote read of every blob truncated, as a copy still
    /// in transfer would read.
    var truncateFirstBlobRead = false
    private var truncatedOnce: Set<String> = []
    private(set) var writes: [(device: String, path: String)] = []
    private(set) var downloadRequests = 0
    private(set) var truncatedServed = 0
    private(set) var placeholdersServed = 0

    func advance(_ ticks: Int = 1) {
        lock.lock()
        tick += ticks
        lock.unlock()
    }

    func provider(for device: String) -> FakeProvider {
        FakeProvider(cloud: self, device: device)
    }

    func contents(_ path: String) -> Data? {
        lock.lock()
        defer { lock.unlock() }
        return files[path]?.data
    }

    /// Removes a file, as if it vanished from the exchange.
    func remove(_ path: String) {
        lock.lock()
        files[path] = nil
        lock.unlock()
    }

    func paths() -> [String] {
        lock.lock()
        defer { lock.unlock() }
        return files.keys.sorted()
    }

    func withLock<T>(_ body: () throws -> T) rethrows -> T {
        lock.lock()
        defer { lock.unlock() }
        return try body()
    }

    fileprivate func state(of path: String, for device: String) -> ExchangeItemState? {
        guard let file = files[path] else { return nil }
        if file.writer == device { return .current }
        guard file.visibleAt <= tick else { return nil }
        guard let ready = downloaded[device]?[path] else { return .notDownloaded }
        return ready <= tick ? .current : .downloading
    }

    fileprivate func checkAccess(_ device: String) throws {
        guard accountAvailable else { throw ExchangeProviderError.accountUnavailable("signed out") }
        if expired.contains(device) { throw ExchangeProviderError.accessExpired("grant lapsed") }
    }

    fileprivate func renew(_ device: String) throws {
        guard renewable else { throw ExchangeProviderError.accessExpired("bookmark no longer resolves") }
        renewals += 1
        expired.remove(device)
    }

    fileprivate func list(_ directory: String, _ device: String) throws -> [ExchangeItem] {
        try checkAccess(device)
        let prefix = directory.isEmpty ? "" : directory + "/"
        return files.keys.sorted().compactMap { path in
            guard path.hasPrefix(prefix), let state = state(of: path, for: device) else { return nil }
            if state != .current { placeholdersServed += 1 }
            return ExchangeItem(path: path, size: Int64(files[path]!.data.count), state: state)
        }
    }

    fileprivate func read(_ path: String, _ device: String) throws -> Data {
        try checkAccess(device)
        guard let state = state(of: path, for: device), let file = files[path] else {
            throw ExchangeProviderError.notFound(path)
        }
        guard state == .current else { throw ExchangeProviderError.notDownloaded(path) }
        if file.writer != device, truncateFirstBlobRead, path.contains("/blobs/"), !truncatedOnce.contains(device + path) {
            truncatedOnce.insert(device + path)
            truncatedServed += 1
            return file.data.prefix(file.data.count / 2)
        }
        if file.writer != device, let remaining = truncatedReads[path], remaining > 0 {
            truncatedReads[path] = remaining - 1
            truncatedServed += 1
            return file.data.prefix(file.data.count / 2)
        }
        return file.data
    }

    fileprivate func write(_ path: String, _ data: Data, _ device: String) throws {
        try checkAccess(device)
        if let limit = failWritesAfter, writes.count >= limit {
            throw ExchangeProviderError.transient("simulated interruption before \(path)")
        }
        if failWrites > 0 {
            failWrites -= 1
            throw ExchangeProviderError.transient("simulated write failure for \(path)")
        }
        files[path] = File(data: data, writer: device, visibleAt: tick + uploadLag)
        downloaded = downloaded.mapValues { var paths = $0; paths[path] = nil; return paths }
        writes.append((device, path))
    }

    fileprivate func requestDownload(_ path: String, _ device: String) throws {
        try checkAccess(device)
        downloadRequests += 1
        guard files[path] != nil, downloaded[device]?[path] == nil else { return }
        downloaded[device, default: [:]][path] = tick + downloadLag
    }
}

struct FakeProvider: ExchangeProvider {
    let cloud: FakeCloud
    let device: String

    func checkAccess() throws { try cloud.withLock { try cloud.checkAccess(device) } }
    func renewAccess() throws { try cloud.withLock { try cloud.renew(device) } }
    func list(directory: String) throws -> [ExchangeItem] { try cloud.withLock { try cloud.list(directory, device) } }
    func read(_ path: String) throws -> Data { try cloud.withLock { try cloud.read(path, device) } }
    func write(_ path: String, data: Data) throws { try cloud.withLock { try cloud.write(path, data, device) } }
    func requestDownload(_ path: String) throws { try cloud.withLock { try cloud.requestDownload(path, device) } }
}

/// A clock tests move by hand, so retry backoff is exercised without waiting.
final class TestClock: @unchecked Sendable {
    private let lock = NSLock()
    private var current = Date(timeIntervalSince1970: 1_790_000_000)

    var now: Date {
        lock.lock()
        defer { lock.unlock() }
        return current
    }

    func advance(_ seconds: TimeInterval) {
        lock.lock()
        current = current.addingTimeInterval(seconds)
        lock.unlock()
    }
}
