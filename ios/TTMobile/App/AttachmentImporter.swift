import CryptoKit
import Foundation
import TTEngine

/// Brings a Files or Photos item into a task's attachments without ever
/// keeping the provider's URL. The item is copied, while its grant is valid,
/// into a staging directory under this device's runtime directory; a record
/// naming the task, the copy, and its digest is written beside it; and only
/// then is the engine asked to commit the copy's bytes with the task's
/// metadata in one transaction. The record is removed after the engine
/// answers. A termination at any point leaves either nothing (the copy never
/// completed, and its directory is swept), or a record the next launch
/// resumes: the engine answers `present` when the earlier commit had landed,
/// so a resumed import never attaches the same bytes twice.
struct AttachmentImporter: Sendable {
    /// One staged import waiting for its commit.
    struct Pending: Codable, Equatable, Sendable {
        var id: String
        var task: TaskLocator
        var filename: String
        var digest: String
        var stagedPath: String
    }

    struct Failure: Error, CustomStringConvertible {
        let description: String
    }

    let directory: URL

    init(location: WorkspaceLocation) {
        directory = location.runtime.appendingPathComponent("attachment-imports", isDirectory: true)
    }

    private var recordsFile: URL { directory.appendingPathComponent("pending.json") }

    /// Every staged import whose commit has not been confirmed.
    func pending() -> [Pending] {
        guard let data = try? Data(contentsOf: recordsFile) else { return [] }
        return (try? JSONDecoder().decode([Pending].self, from: data)) ?? []
    }

    /// Copies a picked file into staging through a coordinated read under its
    /// security scope, then records it. The returned record is what `commit`
    /// takes.
    func stage(file url: URL, for task: TaskLocator) throws -> Pending {
        let scoped = url.startAccessingSecurityScopedResource()
        defer { if scoped { url.stopAccessingSecurityScopedResource() } }
        let id = UUID().uuidString.lowercased()
        let target = try stagingFile(id: id, filename: url.lastPathComponent)
        var coordinationError: NSError?
        var copyError: Error?
        NSFileCoordinator().coordinate(readingItemAt: url, options: [.withoutChanges], error: &coordinationError) { readable in
            do {
                try FileManager.default.copyItem(at: readable, to: target)
            } catch {
                copyError = error
            }
        }
        if let failure = coordinationError ?? copyError {
            try? FileManager.default.removeItem(at: target.deletingLastPathComponent())
            throw Failure(description: "The file could not be read: \(failure.localizedDescription)")
        }
        return try record(id: id, task: task, filename: url.lastPathComponent, staged: target)
    }

    /// Stages bytes a picker handed over in memory, such as a photo.
    func stage(data: Data, filename: String, for task: TaskLocator) throws -> Pending {
        let id = UUID().uuidString.lowercased()
        let target = try stagingFile(id: id, filename: filename)
        try data.write(to: target, options: .atomic)
        return try record(id: id, task: task, filename: filename, staged: target)
    }

    /// Commits one staged import and forgets it. A refusal that retrying
    /// cannot change (the task is gone, the copy is damaged) also forgets it;
    /// anything else leaves the record for the next attempt.
    func commit(_ pending: Pending, engine: TTEngine) async throws -> AttachmentMutation {
        do {
            let result = try await engine.attachment(
                pending.task, .importFile(stagedPath: pending.stagedPath, filename: pending.filename, digest: pending.digest)
            )
            forget(pending)
            return result
        } catch let refusal as BridgeError where [.notFound, .validation, .invalidRequest].contains(refusal.code) {
            forget(pending)
            throw refusal
        }
    }

    /// Removes staging directories no record names: copies a termination
    /// interrupted before they were recorded.
    func sweep() {
        let recorded = Set(pending().map(\.id))
        let entries = (try? FileManager.default.contentsOfDirectory(at: directory, includingPropertiesForKeys: nil)) ?? []
        for entry in entries where entry.hasDirectoryPath && !recorded.contains(entry.lastPathComponent) {
            try? FileManager.default.removeItem(at: entry)
        }
    }

    private func stagingFile(id: String, filename: String) throws -> URL {
        let folder = directory.appendingPathComponent(id, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let name = filename.isEmpty ? "attachment" : filename.replacingOccurrences(of: "/", with: "-")
        return folder.appendingPathComponent(name)
    }

    private func record(id: String, task: TaskLocator, filename: String, staged: URL) throws -> Pending {
        let pending = Pending(id: id, task: task, filename: filename, digest: try Self.digest(of: staged), stagedPath: staged.path)
        var all = self.pending()
        all.append(pending)
        try save(all)
        return pending
    }

    private func forget(_ pending: Pending) {
        try? save(self.pending().filter { $0.id != pending.id })
        try? FileManager.default.removeItem(at: directory.appendingPathComponent(pending.id, isDirectory: true))
    }

    private func save(_ records: [Pending]) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try JSONEncoder().encode(records).write(to: recordsFile, options: .atomic)
    }

    /// The content address the engine and the exchange name bytes by.
    static func digest(of file: URL) throws -> String {
        let handle = try FileHandle(forReadingFrom: file)
        defer { try? handle.close() }
        var hash = SHA256()
        while let chunk = try handle.read(upToCount: 1 << 20), !chunk.isEmpty {
            hash.update(data: chunk)
        }
        return "sha256:" + hash.finalize().map { String(format: "%02x", $0) }.joined()
    }
}
