import CryptoKit
import Foundation

/// One file a capture carries, readable at `url` while the capture is saved.
public struct SharedFile: Hashable, Sendable {
    public var filename: String
    public var url: URL

    public init(filename: String, url: URL) {
        self.filename = filename
        self.url = url
    }
}

/// Points at which a save can be cut short, for tests standing in for a
/// terminated extension.
public enum CaptureWriteStage: Sendable {
    case filesCopied
    case envelopeWritten
}

public enum CaptureInboxError: Error, Equatable, Sendable {
    /// The App Group container is not available to this process.
    case unavailable
    case nothingToSave
    case unreadable(String)
}

/// The durable capture inbox in the App Group container
/// (docs/mobile/capture-inbox.md). A share extension or a shortcut saves into
/// it and never touches the workspace; the app's engine imports from it.
///
/// A save writes everything under `.staging/<id>` — the files, then the
/// envelope — and publishes it with one rename, so a terminated writer leaves
/// nothing the importer reads, and a retry with the same id replaces its own
/// staging and publishes once. Saving a capture that is already published
/// with the same content changes nothing.
public struct CaptureInbox: Sendable {
    public static let stagingName = ".staging"
    public static let envelopeName = "capture.json"
    public static let statusName = "status.json"

    public let directory: URL

    public init(directory: URL) {
        self.directory = directory
    }

    /// The inbox inside an App Group container root.
    public init(container: URL) {
        directory = container.appendingPathComponent("captures", isDirectory: true)
    }

    /// Saves one capture under `id`, copying each file into its entry.
    @discardableResult
    public func save(
        id: UUID,
        workspace: String?,
        source: CaptureSource,
        content: CaptureContent,
        urls: [String] = [],
        files: [SharedFile] = [],
        createdAt: Date = Date(),
        fault: (@Sendable (CaptureWriteStage) throws -> Void)? = nil
    ) throws -> CaptureEnvelope {
        let identifier = id.uuidString.lowercased()
        switch content {
        case .text(let text) where text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty:
            throw CaptureInboxError.nothingToSave
        case .literal(let title, _) where title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && urls.isEmpty && files.isEmpty:
            throw CaptureInboxError.nothingToSave
        default:
            break
        }
        let manager = FileManager.default
        let staging = directory.appendingPathComponent(Self.stagingName, isDirectory: true)
            .appendingPathComponent(identifier, isDirectory: true)
        if manager.fileExists(atPath: staging.path) {
            try manager.removeItem(at: staging)
        }
        let filesDirectory = staging.appendingPathComponent("files", isDirectory: true)
        try manager.createDirectory(at: filesDirectory, withIntermediateDirectories: true)

        var records: [CaptureFileRecord] = []
        for (index, file) in files.enumerated() {
            let name = Self.safeName(file.filename, index: index)
            let relative = "files/\(index + 1)-\(name)"
            let target = staging.appendingPathComponent(relative)
            let (digest, size) = try Self.copy(file.url, to: target)
            records.append(CaptureFileRecord(path: relative, filename: name, digest: digest, size: size))
        }
        try fault?(.filesCopied)

        let envelope = CaptureEnvelope(
            id: identifier, workspace: workspace, createdAt: createdAt, source: source,
            content: content, urls: urls, files: records
        )
        let data = try CaptureEnvelope.encoder.encode(envelope)
        try Self.writeSynced(data, to: staging.appendingPathComponent(Self.envelopeName))
        try fault?(.envelopeWritten)

        let final = directory.appendingPathComponent(identifier, isDirectory: true)
        if manager.fileExists(atPath: final.path) {
            if let published = try? Data(contentsOf: final.appendingPathComponent(Self.envelopeName)),
               let existing = try? CaptureEnvelope.decoder.decode(CaptureEnvelope.self, from: published),
               existing.withoutTiming == envelope.withoutTiming {
                // Delivered twice: the published copy already says this.
                try? manager.removeItem(at: staging)
                return existing
            }
            try manager.removeItem(at: final)
        }
        try manager.moveItem(at: staging, to: final)
        return envelope
    }

    /// Every published capture still waiting for the engine, oldest first.
    public func pending() -> [PendingCapture] {
        let manager = FileManager.default
        guard let names = try? manager.contentsOfDirectory(atPath: directory.path) else { return [] }
        var captures: [PendingCapture] = []
        for name in names where !name.hasPrefix(".") {
            let entry = directory.appendingPathComponent(name, isDirectory: true)
            guard let data = try? Data(contentsOf: entry.appendingPathComponent(Self.envelopeName)),
                  let envelope = try? CaptureEnvelope.decoder.decode(CaptureEnvelope.self, from: data)
            else { continue }
            let status = (try? Data(contentsOf: entry.appendingPathComponent(Self.statusName)))
                .flatMap { try? CaptureEnvelope.decoder.decode(CaptureStatus.self, from: $0) }
            captures.append(PendingCapture(envelope: envelope, status: status))
        }
        return captures.sorted { ($0.envelope.createdAt, $0.id) < ($1.envelope.createdAt, $1.id) }
    }

    /// Removes a capture the person chose not to keep.
    public func discard(id: String) throws {
        let entry = directory.appendingPathComponent(id, isDirectory: true)
        guard FileManager.default.fileExists(atPath: entry.path) else { return }
        try FileManager.default.removeItem(at: entry)
    }

    /// Rewrites a pending capture's words in place, clearing the refusal the
    /// engine left so the next import tries it afresh.
    public func revise(id: String, content: CaptureContent) throws {
        let entry = directory.appendingPathComponent(id, isDirectory: true)
        let file = entry.appendingPathComponent(Self.envelopeName)
        guard let data = try? Data(contentsOf: file) else { throw CaptureInboxError.unreadable(id) }
        var envelope = try CaptureEnvelope.decoder.decode(CaptureEnvelope.self, from: data)
        let revised = CaptureEnvelope(
            id: envelope.id, workspace: envelope.workspace, createdAt: envelope.createdAt, source: envelope.source,
            content: content, urls: envelope.urls ?? [], files: envelope.files ?? []
        )
        envelope = revised
        try Self.writeSynced(try CaptureEnvelope.encoder.encode(envelope), to: file)
        try? FileManager.default.removeItem(at: entry.appendingPathComponent(Self.statusName))
    }

    // MARK: Files

    static func safeName(_ filename: String, index: Int) -> String {
        let base = (filename as NSString).lastPathComponent
        let cleaned = String(base.map { character -> Character in
            switch character {
            case "/", "\\", ":", ";", "|": return "-"
            default: return character.isNewline ? "-" : character
            }
        }).trimmingCharacters(in: .whitespaces)
        if cleaned.isEmpty || cleaned == "." || cleaned == ".." || cleaned.hasPrefix(".") {
            return "shared-\(index + 1)" + (cleaned.hasPrefix(".") && cleaned.count > 1 ? cleaned : "")
        }
        return cleaned
    }

    /// Copies a file, hashing what it wrote, and syncs the copy.
    static func copy(_ source: URL, to target: URL) throws -> (String, Int64) {
        guard let input = InputStream(url: source) else { throw CaptureInboxError.unreadable(source.lastPathComponent) }
        FileManager.default.createFile(atPath: target.path, contents: nil)
        let output = try FileHandle(forWritingTo: target)
        defer { try? output.close() }
        input.open()
        defer { input.close() }
        var hasher = SHA256()
        var size: Int64 = 0
        var buffer = [UInt8](repeating: 0, count: 64 * 1024)
        while true {
            let read = input.read(&buffer, maxLength: buffer.count)
            if read < 0 { throw input.streamError ?? CaptureInboxError.unreadable(source.lastPathComponent) }
            if read == 0 { break }
            let chunk = Data(buffer[0..<read])
            hasher.update(data: chunk)
            try output.write(contentsOf: chunk)
            size += Int64(read)
        }
        try output.synchronize()
        return ("sha256:" + hasher.finalize().map { String(format: "%02x", $0) }.joined(), size)
    }

    static func writeSynced(_ data: Data, to url: URL) throws {
        let temp = url.deletingLastPathComponent().appendingPathComponent(".\(url.lastPathComponent).\(UUID().uuidString)")
        FileManager.default.createFile(atPath: temp.path, contents: nil)
        do {
            let handle = try FileHandle(forWritingTo: temp)
            try handle.write(contentsOf: data)
            try handle.synchronize()
            try handle.close()
            _ = try FileManager.default.replaceItemAt(url, withItemAt: temp)
        } catch {
            try? FileManager.default.removeItem(at: temp)
            throw error
        }
    }
}

extension CaptureEnvelope {
    /// The envelope without its save time, which a retried save restamps.
    var withoutTiming: CaptureEnvelope {
        var copy = self
        copy.createdAt = Date(timeIntervalSince1970: 0)
        return copy
    }
}
