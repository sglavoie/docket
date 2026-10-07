import Foundation

// MARK: Sync conflicts

/// One open sync conflict as the engine describes it to a person: the file,
/// the version every side started from, each competing version with the
/// devices and changes that wrote it, and what the workspace holds meanwhile.
/// Every version stays retained on this device until a resolution.
public struct SyncConflict: Decodable, Hashable, Sendable, Identifiable {
    public var id: String
    public var path: String
    public var kind: String
    public var projectID: String?
    public var label: String
    /// Nil when the file did not exist before the competing changes.
    public var base: ConflictContent?
    public var current: ConflictContent?
    public var versions: [ConflictVersion]
    public var details: [ConflictDetail]
    /// Whether replacement text may be written: tasks and notes.
    public var replaceable: Bool

    enum CodingKeys: String, CodingKey {
        case id, path, kind, label, base, current, versions, details, replaceable
        case projectID = "project_id"
    }
}

/// A version's content as far as this device can show it.
public struct ConflictContent: Decodable, Hashable, Sendable {
    public var digest: String?
    public var deleted: Bool
    public var size: Int
    public var text: String?
    public var available: Bool
}

/// One competing version.
public struct ConflictVersion: Decodable, Hashable, Sendable, Identifiable {
    public var index: Int
    public var content: ConflictContent
    /// The version the workspace holds meanwhile.
    public var current: Bool
    public var writers: [ConflictWriter]
    /// The other conflicted files the same change left; keeping this version
    /// keeps that change's side of them too, in one resolution.
    public var alsoResolves: [ConflictReference]

    public var id: Int { index }

    enum CodingKeys: String, CodingKey {
        case index, current, writers
        case alsoResolves = "also_resolves"
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        index = try container.decode(Int.self, forKey: .index)
        current = try container.decode(Bool.self, forKey: .current)
        writers = try container.decode([ConflictWriter].self, forKey: .writers)
        alsoResolves = try container.decode([ConflictReference].self, forKey: .alsoResolves)
        content = try ConflictContent(from: decoder)
    }
}

/// One change that wrote a competing version.
public struct ConflictWriter: Decodable, Hashable, Sendable {
    public var txnID: String
    public var deviceID: String
    public var deviceName: String?
    public var thisDevice: Bool
    /// What kind of change: an edit, an edit made outside tt, a resolution,
    /// an undo.
    public var change: String
    public var createdAt: String
    public var files: Int

    enum CodingKeys: String, CodingKey {
        case change, files
        case txnID = "txn_id"
        case deviceID = "device_id"
        case deviceName = "device_name"
        case thisDevice = "this_device"
        case createdAt = "created_at"
    }
}

public struct ConflictReference: Decodable, Hashable, Sendable {
    public var id: String
    public var path: String
    public var label: String
}

/// One part of the file the merge could not reconcile on its own.
public struct ConflictDetail: Decodable, Hashable, Sendable {
    public var subject: String?
    public var field: String?
    public var detail: String
}

/// How a conflict ends.
public enum ConflictChoice: Sendable, Equatable {
    /// Keep a competing version, and its change's side of every conflict the
    /// same change left.
    case version(Int)
    /// Keep what the workspace holds now.
    case current
    /// Write replacement text.
    case replace(String)
}

public struct ConflictResolution: Decodable, Hashable, Sendable {
    public var txnID: String
    public var resolved: [ConflictReference]
    public var remaining: Int

    enum CodingKeys: String, CodingKey {
        case resolved, remaining
        case txnID = "txn_id"
    }
}

/// Any JSON value, kept whole so a diagnostics document passes through the
/// app unchanged.
public enum JSONValue: Codable, Hashable, Sendable {
    case null
    case bool(Bool)
    case number(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() {
            self = .null
        } else if let value = try? container.decode(Bool.self) {
            self = .bool(value)
        } else if let value = try? container.decode(Double.self) {
            self = .number(value)
        } else if let value = try? container.decode(String.self) {
            self = .string(value)
        } else if let value = try? container.decode([JSONValue].self) {
            self = .array(value)
        } else {
            self = .object(try container.decode([String: JSONValue].self))
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .null: try container.encodeNil()
        case .bool(let value): try container.encode(value)
        case .number(let value): try container.encode(value)
        case .string(let value): try container.encode(value)
        case .array(let value): try container.encode(value)
        case .object(let value): try container.encode(value)
        }
    }
}

private struct ConflictList: Decodable {
    var conflicts: [SyncConflict]
}

private struct ConflictShown: Decodable {
    var conflict: SyncConflict
}

private struct DeviceNamesPayload: Encodable {
    var deviceNames: [String: String]
    enum CodingKeys: String, CodingKey {
        case deviceNames = "device_names"
    }
}

private struct ConflictShowPayload: Encodable {
    var conflictID: String
    var deviceNames: [String: String]
    enum CodingKeys: String, CodingKey {
        case conflictID = "conflict_id"
        case deviceNames = "device_names"
    }
}

private struct ResolvePayload: Encodable {
    var conflictID: String
    var choice: ConflictChoice

    enum CodingKeys: String, CodingKey {
        case choice, version, text
        case conflictID = "conflict_id"
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(conflictID, forKey: .conflictID)
        switch choice {
        case .version(let index):
            try container.encode("version", forKey: .choice)
            try container.encode(index, forKey: .version)
        case .current:
            try container.encode("current", forKey: .choice)
        case .replace(let text):
            try container.encode("replace", forKey: .choice)
            try container.encode(text, forKey: .text)
        }
    }
}

extension TTEngine {
    /// Every open sync conflict, read from this device's own history, so it
    /// answers with the exchange unreachable. `deviceNames` attributes
    /// versions by name where the caller knows one.
    public func conflicts(deviceNames: [String: String] = [:]) throws -> [SyncConflict] {
        let list: ConflictList = try call("conflict.list", payload: DeviceNamesPayload(deviceNames: deviceNames))
        return list.conflicts
    }

    /// One open conflict; `notFound` once it closed or changed.
    public func conflict(id: String, deviceNames: [String: String] = [:]) throws -> SyncConflict {
        let shown: ConflictShown = try call("conflict.show", payload: ConflictShowPayload(conflictID: id, deviceNames: deviceNames))
        return shown.conflict
    }

    /// Resolves a conflict as a new change that syncs. A conflict that
    /// changed since it was read refuses `staleRevision` and writes nothing.
    public func resolve(
        conflict id: String,
        _ choice: ConflictChoice,
        requestID: String = UUID().uuidString
    ) throws -> ConflictResolution {
        try call("conflict.resolve", payload: ResolvePayload(conflictID: id, choice: choice), requestID: requestID)
    }

    /// The engine's half of a diagnostics export: status, outbox, and the
    /// conflicts' structure without content.
    public func syncDiagnostics(deviceNames: [String: String] = [:]) throws -> JSONValue {
        try call("sync.diagnostics", payload: DeviceNamesPayload(deviceNames: deviceNames))
    }
}
