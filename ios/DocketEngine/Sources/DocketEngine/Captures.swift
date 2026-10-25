import Foundation

/// One capture the engine imported from the inbox (capture.import).
public struct ImportedCapture: Decodable, Hashable, Sendable {
    public var captureID: String
    /// `imported`, `resumed` (an earlier attempt's commit had landed), or
    /// `already_imported` (nothing ran; the entry was removed).
    public var outcome: String
    public var tasks: [TaskSnapshot]

    enum CodingKeys: String, CodingKey {
        case outcome, tasks
        case captureID = "capture_id"
    }
}

/// One capture the engine left in the inbox.
public struct HeldCapture: Decodable, Hashable, Sendable {
    public var captureID: String
    /// `refused`, `other_workspace`, or `incomplete`.
    public var reason: String
    public var code: String?
    public var message: String
    public var lines: [CaptureLineError]?

    enum CodingKeys: String, CodingKey {
        case reason, code, message, lines
        case captureID = "capture_id"
    }
}

/// What one pass over the capture inbox did.
public struct CaptureImportReport: Decodable, Hashable, Sendable {
    public var imported: [ImportedCapture]
    public var pending: [HeldCapture]
    public var transactionIDs: [String]

    enum CodingKeys: String, CodingKey {
        case imported, pending
        case transactionIDs = "transaction_ids"
    }

    /// Whether the pass committed anything.
    public var changedWorkspace: Bool { !transactionIDs.isEmpty }
}

private struct ImportCapturesPayload: Encodable {
    var capturesDir: String

    enum CodingKeys: String, CodingKey {
        case capturesDir = "captures_dir"
    }
}

private struct WidgetSnapshotPayload: Encodable {
    var limit: Int?
}

extension DocketEngine {
    /// Imports every saved capture in the inbox at `directory`, each exactly
    /// once however often it was delivered or interrupted.
    public func importCaptures(from directory: URL) throws -> CaptureImportReport {
        try call("capture.import", payload: ImportCapturesPayload(capturesDir: directory.path))
    }

    /// Today and Inbox as the widgets show them, in the engine's
    /// `tt-widget-snapshot/v1` shape, left undecoded for the App Group copy.
    public func widgetSnapshot(limit: Int? = nil) throws -> JSONValue {
        try call("query.widget_snapshot", payload: WidgetSnapshotPayload(limit: limit))
    }
}
