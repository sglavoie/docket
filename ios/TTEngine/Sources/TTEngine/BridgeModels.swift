import Foundation

/// The highest `bridge_version` these models speak.
public let supportedBridgeVersion = 1

/// A task's qualified identity. The bridge never addresses a task by a bare
/// id, a short id, or a title.
public struct TaskLocator: Codable, Hashable, Sendable {
    public var projectID: String
    public var taskID: String

    public init(projectID: String, taskID: String) {
        self.projectID = projectID
        self.taskID = taskID
    }

    enum CodingKeys: String, CodingKey {
        case projectID = "project_id"
        case taskID = "task_id"
    }
}

/// The eight lifecycle states.
public enum WorkflowState: String, Codable, Sendable, CaseIterable {
    case inbox, backlog, doing, waiting, paused, someday, done, canceled
}

/// Where a capture lands: a lifecycle state, or the Today or Next plan.
public enum CaptureDestination: String, Codable, Sendable {
    case inbox, backlog, doing, waiting, paused, someday, done, canceled, today, next
}

/// Revision preconditions for a mutation: canonical project revisions by
/// project id, raw domain artifact revisions by domain name.
public struct Expectation: Codable, Hashable, Sendable {
    public var projects: [String: String]
    public var domains: [String: String]

    public init(projects: [String: String] = [:], domains: [String: String] = [:]) {
        self.projects = projects
        self.domains = domains
    }
}

/// Every revision a client may name as a precondition.
public struct Revisions: Decodable, Hashable, Sendable {
    public var projects: [String: String]
    public var domains: [String: String]
}

public struct SchemaVersions: Decodable, Hashable, Sendable {
    public var bridge: Int
    public var catalog: Int
    public var taskMarkdown: Int
    public var domainEnvelope: Int

    enum CodingKeys: String, CodingKey {
        case bridge, catalog
        case taskMarkdown = "task_markdown"
        case domainEnvelope = "domain_envelope"
    }
}

public struct WorkspaceStatus: Decodable, Hashable, Sendable {
    public var workspace: String
    public var dataDir: String
    public var deviceID: String
    public var schemas: SchemaVersions
    public var revisions: Revisions

    enum CodingKeys: String, CodingKey {
        case workspace, schemas, revisions
        case dataDir = "data_dir"
        case deviceID = "device_id"
    }
}

public struct ProjectSummary: Decodable, Hashable, Sendable, Identifiable {
    public var id: String
    public var slug: String
    public var name: String
    public var tags: [String]
    public var archived: Bool
    public var systemUnfiled: Bool

    enum CodingKeys: String, CodingKey {
        case id, slug, name, tags, archived
        case systemUnfiled = "system_unfiled"
    }
}

/// The three project populations, reported apart.
public struct ProjectPopulations: Decodable, Hashable, Sendable {
    public var active: [ProjectSummary]
    public var parked: [ProjectSummary]
    public var archived: [ProjectSummary]
}

public struct TaskSnapshot: Decodable, Hashable, Sendable, Identifiable {
    public var projectID: String
    public var taskID: String
    public var shortID: Int
    public var title: String
    public var description: String?
    public var state: WorkflowState
    public var priority: Int
    public var parentID: String?
    public var tags: [String]
    public var context: String?
    public var pinned: Bool
    public var review: Bool
    public var createdAt: Date
    public var updatedAt: Date
    public var stateChangedAt: Date
    public var completedAt: Date?
    public var dueDate: Date?
    public var deferredUntil: Date?
    public var resumeNote: String?
    public var nextCheckIn: Date?
    /// Absent from an engine older than the browsing ops, so optional.
    public var checklist: [ChecklistItem]?
    public var commentCount: Int?
    public var attachmentCount: Int?
    public var dependencyCount: Int?

    public var locator: TaskLocator { TaskLocator(projectID: projectID, taskID: taskID) }
    public var id: TaskLocator { locator }

    enum CodingKeys: String, CodingKey {
        case title, description, state, priority, tags, context, pinned, review, checklist
        case projectID = "project_id"
        case taskID = "task_id"
        case shortID = "short_id"
        case parentID = "parent_id"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
        case stateChangedAt = "state_changed_at"
        case completedAt = "completed_at"
        case dueDate = "due_date"
        case deferredUntil = "deferred_until"
        case resumeNote = "resume_note"
        case nextCheckIn = "next_check_in"
        case commentCount = "comment_count"
        case attachmentCount = "attachment_count"
        case dependencyCount = "dependency_count"
    }
}

/// One checklist row in stored order.
public struct ChecklistItem: Decodable, Hashable, Sendable {
    public var text: String
    public var done: Bool
}

/// A task another task points at. A reference the workspace cannot resolve
/// keeps its stored locator and arrives with `available` false.
public struct RelatedTask: Decodable, Hashable, Sendable, Identifiable {
    public var projectID: String
    public var taskID: String
    public var available: Bool
    public var title: String?
    public var state: WorkflowState?
    public var projectName: String?

    public var locator: TaskLocator { TaskLocator(projectID: projectID, taskID: taskID) }
    public var id: TaskLocator { locator }

    enum CodingKeys: String, CodingKey {
        case available, title, state
        case projectID = "project_id"
        case taskID = "task_id"
        case projectName = "project_name"
    }
}

/// What a task detail navigates: ancestors root first, direct children in
/// stored order, the tasks this one depends on, and the computed dependents.
public struct TaskRelations: Decodable, Hashable, Sendable {
    public var ancestors: [RelatedTask]
    public var children: [RelatedTask]
    public var dependencies: [RelatedTask]
    public var dependents: [RelatedTask]
}

/// One task with its project, the project's revision, and its relations.
public struct TaskDetail: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var project: ProjectSummary
    public var revision: String
    public var relations: TaskRelations?
}

/// A manual order: a field and a direction. A nil field is the mode's own
/// order.
public struct TaskSort: Codable, Hashable, Sendable {
    public var field: String?
    public var ascending: Bool

    public init(field: String? = nil, ascending: Bool = true) {
        self.field = field
        self.ascending = ascending
    }
}

/// One saved lens: the projection it reopens into.
public struct LensSummary: Decodable, Hashable, Sendable, Identifiable {
    public var id: String
    public var name: String
    public var mode: String
    public var scope: String?
    public var query: String
    public var sort: TaskSort
}

public struct LensList: Decodable, Hashable, Sendable {
    public var lenses: [LensSummary]
}

/// The verdict on one staged manifest a join imported.
public struct StagedImport: Decodable, Hashable, Sendable {
    public var path: String
    public var outcome: String
    public var reason: String?
}

public struct StagedImportReport: Decodable, Hashable, Sendable {
    public var results: [StagedImport]
}

public struct TaskQueryItem: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var today: Bool
    public var next: Bool
}

public struct QueryDiagnostic: Decodable, Hashable, Sendable {
    public var projectID: String
    public var projectName: String
    public var detail: String

    enum CodingKeys: String, CodingKey {
        case detail
        case projectID = "project_id"
        case projectName = "project_name"
    }
}

public struct TaskQueryResult: Decodable, Hashable, Sendable {
    public var query: String
    public var items: [TaskQueryItem]
    public var diagnostics: [QueryDiagnostic]
}

/// What a successful mutation committed: the local transaction and the new
/// revisions of every artifact it touched, the client's next expectation.
public struct CommittedEffect: Decodable, Hashable, Sendable {
    public var transactionID: String
    public var projects: [String: String]
    public var domains: [String: String]

    enum CodingKeys: String, CodingKey {
        case projects, domains
        case transactionID = "transaction_id"
    }
}

public struct TaskMutation: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var committed: CommittedEffect
}

/// A structured engine failure.
public struct BridgeError: Error, Decodable, Hashable, Sendable, CustomStringConvertible {
    /// The contract's error codes. A code this client does not know decodes as
    /// `unrecognized`, keeping its spelling in `rawCode`.
    public enum Code: String, Sendable {
        case invalidRequest = "invalid_request"
        case unsupportedVersion = "unsupported_version"
        case unknownOp = "unknown_op"
        case workspaceMismatch = "workspace_mismatch"
        case notOpen = "not_open"
        case notFound = "not_found"
        case validation
        case refused
        case staleRevision = "stale_revision"
        case conflict
        case locked
        case migrationRequired = "migration_required"
        case schemaTooNew = "schema_too_new"
        case io
        case `internal`
        case unrecognized
    }

    public var rawCode: String
    public var message: String
    public var retryable: Bool

    public var code: Code { Code(rawValue: rawCode) ?? .unrecognized }
    public var description: String { "\(rawCode): \(message)" }

    public init(rawCode: String, message: String, retryable: Bool) {
        self.rawCode = rawCode
        self.message = message
        self.retryable = retryable
    }

    enum CodingKeys: String, CodingKey {
        case message, retryable
        case rawCode = "code"
    }
}
