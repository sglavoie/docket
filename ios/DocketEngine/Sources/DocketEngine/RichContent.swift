import Foundation

// MARK: Comments

/// One task or project comment. A task comment is addressed by its index and
/// the text read there; a project comment by its stored id and that text.
public struct CommentRecord: Decodable, Hashable, Sendable, Identifiable {
    public var index: Int
    public var commentID: String?
    public var text: String
    public var createdAt: Date

    public var id: String { commentID ?? "\(index)" }

    enum CodingKeys: String, CodingKey {
        case index, text
        case commentID = "id"
        case createdAt = "created_at"
    }
}

/// Whose comment: a task's or a project's.
public enum CommentOwner: Hashable, Sendable {
    case task(TaskLocator)
    case project(String)
}

/// One comment action. `original` is the text the person read, so an edit
/// that arrived from another device in the meantime refuses the action.
public enum CommentAction: Sendable {
    case add(String)
    case edit(CommentRecord, text: String)
    case delete(CommentRecord)
}

public struct CommentMutation: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot?
    public var projectID: String?
    public var commentID: String?
    public var comments: [CommentRecord]
    public var committed: CommittedEffect

    enum CodingKeys: String, CodingKey {
        case task, comments, committed
        case projectID = "project_id"
        case commentID = "comment_id"
    }
}

// MARK: Attachments

/// One attachment. Only a file has bytes in the workspace: `available` says
/// whether this device holds them and `localPath` where, read-only. A URL and
/// a code reference are kept whole even though a phone cannot open a code
/// reference's target.
public struct AttachmentRecord: Decodable, Hashable, Sendable, Identifiable {
    public enum Kind: String, Decodable, Sendable {
        case file, url, code
    }

    public var index: Int
    public var kind: Kind
    public var name: String
    public var url: String?
    public var codePath: String?
    public var line: Int?
    public var addedAt: Date
    public var available: Bool
    public var size: Int64?
    public var localPath: String?
    public var workspacePath: String?

    public var id: String { "\(index)-\(name)" }

    enum CodingKeys: String, CodingKey {
        case index, kind, name, url, line, available, size
        case codePath = "code_path"
        case addedAt = "added_at"
        case localPath = "local_path"
        case workspacePath = "workspace_path"
    }
}

/// One attachment action. An imported file travels as a copy staged under
/// the device's runtime directory with its digest, never as a provider URL.
public enum AttachmentAction: Sendable {
    case importFile(stagedPath: String, filename: String, digest: String)
    case addURL(String)
    case remove(AttachmentRecord)
}

public struct AttachmentMutation: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var attachment: AttachmentRecord?
    public var attachments: [AttachmentRecord]
    /// added, present (those bytes were already attached; nothing
    /// committed), or removed.
    public var outcome: String
    public var committed: CommittedEffect
}

// MARK: Notes

/// A note collection: the workspace's global notes or one project's.
public struct NoteLocation: Codable, Hashable, Sendable {
    public var scope: String
    public var projectID: String?

    public static let global = NoteLocation(scope: "global", projectID: nil)

    public static func project(_ id: String) -> NoteLocation {
        NoteLocation(scope: "project", projectID: id)
    }

    public init(scope: String, projectID: String?) {
        self.scope = scope
        self.projectID = projectID
    }

    public var isGlobal: Bool { scope == "global" }

    enum CodingKeys: String, CodingKey {
        case scope
        case projectID = "project_id"
    }
}

/// One note. A list carries an excerpt; a read and a save carry the body.
/// `revision` is the revision of the bytes read, which every change hands
/// back so another device's edit is never overwritten unseen.
public struct NoteRecord: Decodable, Hashable, Sendable, Identifiable {
    public var location: NoteLocation
    public var projectName: String?
    public var filename: String
    public var title: String
    public var createdAt: Date
    public var modifiedAt: Date
    public var revision: String
    public var parseError: String?
    public var excerpt: String
    public var body: String?

    public var id: String { "\(location.projectID ?? "global")/\(filename)" }

    enum CodingKeys: String, CodingKey {
        case location, filename, title, revision, excerpt, body
        case projectName = "project_name"
        case createdAt = "created_at"
        case modifiedAt = "modified_at"
        case parseError = "parse_error"
    }
}

public struct NoteList: Decodable, Hashable, Sendable {
    public var notes: [NoteRecord]
}

public struct NoteRead: Decodable, Hashable, Sendable {
    public var note: NoteRecord
}

/// One note action. Every action but create names the note as read.
public enum NoteAction: Sendable {
    case create(NoteLocation, title: String, body: String)
    case update(NoteRecord, title: String, body: String)
    case rename(NoteRecord, title: String)
    case move(NoteRecord, to: NoteLocation)
    case delete(NoteRecord)
}

public struct NoteMutation: Decodable, Hashable, Sendable {
    public var note: NoteRecord?
    public var committed: CommittedEffect
}

// MARK: Engine calls

extension DocketEngine {
    /// Adds, edits, or deletes one task or project comment.
    public func comment(
        _ owner: CommentOwner,
        _ action: CommentAction,
        requestID: String = UUID().uuidString
    ) throws -> CommentMutation {
        try call("command.comment", payload: CommentPayload(owner: owner, action: action), requestID: requestID)
    }

    /// Imports a staged file, adds a URL, or removes an attachment.
    public func attachment(
        _ locator: TaskLocator,
        _ action: AttachmentAction,
        requestID: String = UUID().uuidString
    ) throws -> AttachmentMutation {
        try call("command.attachment", payload: AttachmentPayload(task: locator, action: action), requestID: requestID)
    }

    /// Every reachable note, newest first, or one collection's.
    public func notes(at location: NoteLocation? = nil) throws -> [NoteRecord] {
        let list: NoteList = try call("query.notes", payload: NotesPayload(location: location))
        return list.notes
    }

    /// One note whole.
    public func note(at location: NoteLocation, filename: String) throws -> NoteRecord {
        let read: NoteRead = try call("query.note", payload: NotePayload(location: location, filename: filename))
        return read.note
    }

    /// Creates, edits, renames, moves, or deletes one note.
    public func note(_ action: NoteAction, requestID: String = UUID().uuidString) throws -> NoteMutation {
        try call("command.note", payload: NoteCommandPayload(action: action), requestID: requestID)
    }
}

// MARK: Wire shapes

private struct CommentPayload: Encodable {
    var owner: CommentOwner
    var action: CommentAction

    enum CodingKeys: String, CodingKey {
        case task, action, text, index, original
        case projectID = "project_id"
        case commentID = "comment_id"
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        let isTask: Bool
        switch owner {
        case .task(let locator):
            try container.encode(locator, forKey: .task)
            isTask = true
        case .project(let id):
            try container.encode(id, forKey: .projectID)
            isTask = false
        }
        func address(_ comment: CommentRecord) throws {
            if isTask {
                try container.encode(comment.index, forKey: .index)
            } else {
                try container.encode(comment.commentID ?? "", forKey: .commentID)
            }
            try container.encode(comment.text, forKey: .original)
        }
        switch action {
        case .add(let text):
            try container.encode("add", forKey: .action)
            try container.encode(text, forKey: .text)
        case .edit(let comment, let text):
            try container.encode("edit", forKey: .action)
            try address(comment)
            try container.encode(text, forKey: .text)
        case .delete(let comment):
            try container.encode("delete", forKey: .action)
            try address(comment)
        }
    }
}

private struct AttachmentPayload: Encodable {
    var task: TaskLocator
    var action: AttachmentAction

    enum CodingKeys: String, CodingKey {
        case task, action, path, filename, digest, url, index, original
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(task, forKey: .task)
        switch action {
        case .importFile(let path, let filename, let digest):
            try container.encode("add_file", forKey: .action)
            try container.encode(path, forKey: .path)
            try container.encode(filename, forKey: .filename)
            try container.encode(digest, forKey: .digest)
        case .addURL(let url):
            try container.encode("add_url", forKey: .action)
            try container.encode(url, forKey: .url)
        case .remove(let attachment):
            try container.encode("remove", forKey: .action)
            try container.encode(attachment.index, forKey: .index)
            try container.encode(attachment.name, forKey: .original)
        }
    }
}

private struct NotesPayload: Encodable {
    var location: NoteLocation?
}

private struct NotePayload: Encodable {
    var location: NoteLocation
    var filename: String
}

private struct NoteCommandPayload: Encodable {
    var action: NoteAction

    enum CodingKeys: String, CodingKey {
        case action, location, filename, revision, title, body, destination
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        func address(_ note: NoteRecord) throws {
            try container.encode(note.location, forKey: .location)
            try container.encode(note.filename, forKey: .filename)
            try container.encode(note.revision, forKey: .revision)
        }
        switch action {
        case .create(let location, let title, let body):
            try container.encode("create", forKey: .action)
            try container.encode(location, forKey: .location)
            try container.encode(title, forKey: .title)
            try container.encode(body, forKey: .body)
        case .update(let note, let title, let body):
            try container.encode("update", forKey: .action)
            try address(note)
            try container.encode(title, forKey: .title)
            try container.encode(body, forKey: .body)
        case .rename(let note, let title):
            try container.encode("rename", forKey: .action)
            try address(note)
            try container.encode(title, forKey: .title)
        case .move(let note, let destination):
            try container.encode("move", forKey: .action)
            try address(note)
            try container.encode(destination, forKey: .destination)
        case .delete(let note):
            try container.encode("delete", forKey: .action)
            try address(note)
        }
    }
}
