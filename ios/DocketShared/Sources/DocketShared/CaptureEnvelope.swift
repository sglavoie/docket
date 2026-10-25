import Foundation

/// Where a saved capture came from.
public enum CaptureSource: String, Codable, Sendable {
    case share
    case shortcut
}

/// One shared file's bytes, beside its envelope in the capture's entry.
public struct CaptureFileRecord: Codable, Hashable, Sendable {
    /// The copy's path inside the entry, `files/<n>-<name>`.
    public var path: String
    public var filename: String
    /// `sha256:<hex>` of the copy, which the engine checks before importing.
    public var digest: String
    public var size: Int64

    public init(path: String, filename: String, digest: String, size: Int64) {
        self.path = path
        self.filename = filename
        self.digest = digest
        self.size = size
    }
}

/// What a capture asks the engine to create: capture grammar read line by
/// line as the composer reads it, or one task with a literal title.
public enum CaptureContent: Hashable, Sendable {
    case text(String)
    case literal(title: String, note: String)
}

/// One saved capture, `tt-capture/v1` (docs/mobile/capture-inbox.md). An
/// extension writes it; only the app's engine imports it.
public struct CaptureEnvelope: Codable, Hashable, Sendable {
    public static let currentFormat = "tt-capture/v1"

    public var format: String
    /// A lowercase UUID, also the entry's directory name. The engine derives
    /// the identities of the tasks it creates from it, which is what makes a
    /// repeated import find them instead of creating them twice.
    public var id: String
    /// The workspace the capture was saved for, when the extension knew it.
    public var workspace: String?
    public var createdAt: Date
    public var source: CaptureSource
    public var text: String?
    public var title: String?
    public var note: String?
    public var urls: [String]?
    public var files: [CaptureFileRecord]?

    enum CodingKeys: String, CodingKey {
        case format, id, workspace, source, text, title, note, urls, files
        case createdAt = "created_at"
    }

    public init(
        id: String, workspace: String?, createdAt: Date, source: CaptureSource,
        content: CaptureContent, urls: [String] = [], files: [CaptureFileRecord] = []
    ) {
        format = Self.currentFormat
        self.id = id
        self.workspace = workspace
        self.createdAt = createdAt
        self.source = source
        switch content {
        case .text(let text):
            self.text = text
        case .literal(let title, let note):
            self.title = title
            self.note = note.isEmpty ? nil : note
        }
        self.urls = urls.isEmpty ? nil : urls
        self.files = files.isEmpty ? nil : files
    }

    public var content: CaptureContent {
        if let text, !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return .text(text)
        }
        return .literal(title: title ?? "", note: note ?? "")
    }

    /// The words a list shows for this capture before it is imported.
    public var summary: String {
        switch content {
        case .text(let text):
            return text.split(whereSeparator: \.isNewline).first.map(String.init) ?? text
        case .literal(let title, _):
            if !title.isEmpty { return title }
            return urls?.first ?? files?.first?.filename ?? "Shared item"
        }
    }

    static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        encoder.dateEncodingStrategy = .custom { date, encoder in
            var container = encoder.singleValueContainer()
            try container.encode(ISO8601DateFormatter.withFractions.string(from: date))
        }
        return encoder
    }()

    static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let raw = try container.decode(String.self)
            if let date = ISO8601DateFormatter.withFractions.date(from: raw) ?? ISO8601DateFormatter.plain.date(from: raw) {
                return date
            }
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "not an RFC 3339 instant: \(raw)")
        }
        return decoder
    }()
}

extension ISO8601DateFormatter {
    nonisolated(unsafe) static let withFractions: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()

    nonisolated(unsafe) static let plain = ISO8601DateFormatter()
}

/// The engine's note beside a capture it could not import.
public struct CaptureStatus: Codable, Hashable, Sendable {
    public struct Line: Codable, Hashable, Sendable {
        public var line: Int
        public var column: Int
        public var message: String
    }

    public var state: String
    public var code: String
    public var message: String
    public var lines: [Line]?
    public var attemptedAt: Date

    enum CodingKeys: String, CodingKey {
        case state, code, message, lines
        case attemptedAt = "attempted_at"
    }
}

/// A capture still in the inbox: saved, and not yet imported.
public struct PendingCapture: Identifiable, Hashable, Sendable {
    public var envelope: CaptureEnvelope
    /// Why the last import left it, or nil when no import has tried yet.
    public var status: CaptureStatus?

    public var id: String { envelope.id }
    public var isRefused: Bool { status?.state == "refused" }
}
