import Foundation
import UniformTypeIdentifiers

/// What a share sheet handed the extension: words, links, and files, each
/// file already copied somewhere the extension owns, since a provider's own
/// copy disappears when its load callback returns.
public struct SharedItems: Hashable, Sendable {
    public var texts: [String] = []
    public var urls: [URL] = []
    public var files: [SharedFile] = []
    /// Items the extension could not read, by type, so it can say so.
    public var unreadable: [String] = []

    public init(texts: [String] = [], urls: [URL] = [], files: [SharedFile] = [], unreadable: [String] = []) {
        self.texts = texts
        self.urls = urls
        self.files = files
        self.unreadable = unreadable
    }

    public var isEmpty: Bool { texts.isEmpty && urls.isEmpty && files.isEmpty }

    /// Reads every provider, copying file bytes under `scratch`. Providers
    /// are read on the main actor, where an extension's view hands them over.
    @MainActor
    public static func load(_ providers: [NSItemProvider], scratch: URL) async -> SharedItems {
        var items = SharedItems()
        try? FileManager.default.createDirectory(at: scratch, withIntermediateDirectories: true)
        for (index, provider) in providers.enumerated() {
            await items.read(provider, index: index, scratch: scratch)
        }
        return items
    }

    @MainActor
    private mutating func read(_ provider: NSItemProvider, index: Int, scratch: URL) async {
        let types = provider.registeredTypeIdentifiers
        let conforms = { (type: UTType) in types.contains { UTType($0)?.conforms(to: type) ?? false } }
        if conforms(.url), !conforms(.fileURL) {
            if let url = await Self.loadURL(provider), !url.isFileURL {
                urls.append(url)
                return
            }
        }
        if conforms(.plainText), !conforms(.fileURL) {
            if let text = await Self.loadText(provider), !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                // A link shared as text is a link.
                let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
                if let url = URL(string: trimmed), let scheme = url.scheme, ["http", "https"].contains(scheme.lowercased()), !trimmed.contains(" ") {
                    urls.append(url)
                } else {
                    texts.append(text)
                }
                return
            }
        }
        if conforms(.fileURL), let url = await Self.loadURL(provider), url.isFileURL,
           let file = Self.copyFile(at: url, named: url.lastPathComponent, index: index, scratch: scratch) {
            files.append(file)
            return
        }
        let fileType = types.first { UTType($0)?.conforms(to: .data) ?? false } ?? types.first { $0 == UTType.fileURL.identifier }
        if let fileType, let file = await Self.loadFile(provider, type: fileType, index: index, scratch: scratch) {
            files.append(file)
            return
        }
        unreadable.append(types.first ?? "unknown")
    }

    @MainActor
    private static func loadURL(_ provider: NSItemProvider) async -> URL? {
        await withCheckedContinuation { continuation in
            _ = provider.loadObject(ofClass: NSURL.self) { object, _ in
                continuation.resume(returning: (object as? NSURL) as URL?)
            }
        }
    }

    @MainActor
    private static func loadText(_ provider: NSItemProvider) async -> String? {
        await withCheckedContinuation { continuation in
            _ = provider.loadObject(ofClass: NSString.self) { object, _ in
                continuation.resume(returning: (object as? NSString) as String?)
            }
        }
    }

    private static func copyFile(at url: URL, named name: String, index: Int, scratch: URL) -> SharedFile? {
        let scoped = url.startAccessingSecurityScopedResource()
        defer { if scoped { url.stopAccessingSecurityScopedResource() } }
        let target = scratch.appendingPathComponent("\(index + 1)-\(UUID().uuidString)", isDirectory: false)
        guard (try? FileManager.default.copyItem(at: url, to: target)) != nil else { return nil }
        return SharedFile(filename: name, url: target)
    }

    @MainActor
    private static func loadFile(_ provider: NSItemProvider, type: String, index: Int, scratch: URL) async -> SharedFile? {
        let suggested = provider.suggestedName
        return await withCheckedContinuation { continuation in
            _ = provider.loadFileRepresentation(forTypeIdentifier: type) { url, _ in
                guard let url else {
                    continuation.resume(returning: nil)
                    return
                }
                var name = url.lastPathComponent
                if let suggested, !suggested.isEmpty {
                    name = (suggested as NSString).pathExtension.isEmpty && !url.pathExtension.isEmpty
                        ? suggested + "." + url.pathExtension : suggested
                }
                continuation.resume(returning: copyFile(at: url, named: name, index: index, scratch: scratch))
            }
        }
    }
}

/// The task a share sheet proposes before the person edits it: the first
/// line of shared words, else the link, else the file, as the title; the rest
/// of the words as the note.
public struct ShareDraft: Hashable, Sendable {
    public var title: String
    public var note: String

    public init(title: String, note: String) {
        self.title = title
        self.note = note
    }

    public init(proposing items: SharedItems) {
        let words = items.texts.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
        var lines = words.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        if let first = lines.first, !first.trimmingCharacters(in: .whitespaces).isEmpty {
            title = String(first.trimmingCharacters(in: .whitespaces).prefix(200))
            lines.removeFirst()
            note = lines.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
        } else if let url = items.urls.first {
            title = url.host.map { "Read: " + $0 + url.path } ?? url.absoluteString
            note = ""
        } else if let file = items.files.first {
            title = file.filename
            note = ""
        } else {
            title = ""
            note = ""
        }
    }

    /// Saves the draft with the shared links and files as one capture.
    @discardableResult
    public func save(
        _ items: SharedItems, into inbox: CaptureInbox, id: UUID, workspace: String?, at date: Date = Date()
    ) throws -> CaptureEnvelope {
        try inbox.save(
            id: id, workspace: workspace, source: .share,
            content: .literal(title: title.trimmingCharacters(in: .whitespacesAndNewlines), note: note),
            urls: items.urls.map(\.absoluteString), files: items.files, createdAt: date
        )
    }
}
