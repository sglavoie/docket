import Foundation

/// The highest `bridge_version` these models speak.
public let supportedBridgeVersion = 1

public struct SearchSuggestion: Decodable, Hashable, Sendable {
    public var token: String
    public var search: String
}

struct SearchSuggestionsResult: Decodable {
    var suggestions: [SearchSuggestion]
}

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
    public var description: String?
    public var tags: [String]
    public var archived: Bool
    /// Whether the project carries the `parked` tag, which keeps its work
    /// out of the task workspace.
    public var parked: Bool?
    public var systemUnfiled: Bool

    public var isParked: Bool { parked ?? tags.contains("parked") }

    enum CodingKeys: String, CodingKey {
        case id, slug, name, description, tags, archived, parked
        case systemUnfiled = "system_unfiled"
    }
}

public extension Sequence where Element == ProjectSummary {
    /// Display order shared by project lists and destination pickers.
    func sortedByName() -> [ProjectSummary] {
        sorted { lhs, rhs in
            let order = lhs.name.localizedStandardCompare(rhs.name)
            return order == .orderedSame ? lhs.id < rhs.id : order == .orderedAscending
        }
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
public struct ChecklistItem: Codable, Hashable, Sendable {
    public var text: String
    public var done: Bool

    public init(text: String, done: Bool) {
        self.text = text
        self.done = done
    }
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

/// One task with its project, the project's revision, its relations, and its
/// planning memberships.
public struct TaskDetail: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var project: ProjectSummary
    public var revision: String
    public var relations: TaskRelations?
    /// Absent from an engine older than the editing ops, so optional.
    public var today: Bool?
    public var next: Bool?
    /// The task's comments and attachments whole, and the workspace path its
    /// attachment bytes live under. Absent from an engine older than the
    /// rich-content ops.
    public var comments: [CommentRecord]?
    public var attachments: [AttachmentRecord]?
    public var attachmentDir: String?

    enum CodingKeys: String, CodingKey {
        case task, project, revision, relations, today, next, comments, attachments
        case attachmentDir = "attachment_dir"
    }
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

/// Every task one capture created, in line order; `task` is the first.
public struct CaptureMutation: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var tasks: [TaskSnapshot]
    public var committed: CommittedEffect
}

/// A structured edit's outcome. An edit that changed nothing committed nothing.
public struct EditMutation: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var changed: Bool
    public var committed: CommittedEffect
}

/// One refusal against one capture line. `column` is zero-based in runes.
public struct CaptureLineError: Decodable, Hashable, Sendable {
    public var line: Int
    public var column: Int
    public var message: String
}

/// What one capture line would create, named as the desktop names it: a plan
/// beside a status.
public struct CapturePreviewLine: Decodable, Hashable, Sendable {
    public var line: Int
    public var title: String
    public var projectID: String
    public var projectName: String
    public var destination: CaptureDestination
    public var plan: String?
    public var state: WorkflowState
    public var priority: Int
    public var tags: [String]
    public var context: String?
    public var due: Date?
    public var deferUntil: Date?
    public var pinned: Bool
    public var review: Bool
    public var parentID: String?
    public var children: Int
    public var dependencies: Int
    public var attachments: Int
    public var comments: Int

    enum CodingKeys: String, CodingKey {
        case line, title, destination, plan, state, priority, tags, context, due, pinned, review
        case children, dependencies, attachments, comments
        case projectID = "project_id"
        case projectName = "project_name"
        case deferUntil = "defer"
        case parentID = "parent_id"
    }
}

/// A capture draft read without committing: what each line would create, or
/// every refused line.
public struct CapturePreview: Decodable, Hashable, Sendable {
    public var projectID: String
    public var lines: [CapturePreviewLine]
    public var errors: [CaptureLineError]

    enum CodingKeys: String, CodingKey {
        case lines, errors
        case projectID = "project_id"
    }
}

/// One structured task to create. The title is literal. `due` and `defer`
/// are read by the engine's own date readers, so `2026-10-21`, `tomorrow`,
/// and `+3d` all work.
public struct CaptureFields: Encodable, Hashable, Sendable {
    public var projectID: String?
    public var title: String
    public var description: String?
    public var resumeNote: String?
    public var destination: CaptureDestination?
    public var priority: Int?
    public var tags: [String]?
    public var context: String?
    public var due: String?
    public var deferUntil: String?
    public var pinned: Bool?
    public var review: Bool?

    public init(
        title: String, projectID: String? = nil, description: String? = nil, resumeNote: String? = nil,
        destination: CaptureDestination? = nil, priority: Int? = nil, tags: [String]? = nil,
        context: String? = nil, due: String? = nil, deferUntil: String? = nil,
        pinned: Bool? = nil, review: Bool? = nil
    ) {
        self.projectID = projectID
        self.title = title
        self.description = description
        self.resumeNote = resumeNote
        self.destination = destination
        self.priority = priority
        self.tags = tags
        self.context = context
        self.due = due
        self.deferUntil = deferUntil
        self.pinned = pinned
        self.review = review
    }

    enum CodingKeys: String, CodingKey {
        case title, description, destination, priority, tags, context, due, pinned, review
        case projectID = "project_id"
        case resumeNote = "resume_note"
        case deferUntil = "defer"
    }
}

/// A structured edit of one task. A nil field keeps the stored value; an
/// empty string clears a clearable one, and `tags` is the complete new set.
public struct TaskEdit: Encodable, Hashable, Sendable {
    public var title: String?
    public var description: String?
    public var resumeNote: String?
    public var priority: Int?
    public var tags: [String]?
    public var context: String?
    public var pinned: Bool?
    public var review: Bool?
    public var due: String?
    public var deferUntil: String?

    public init(
        title: String? = nil, description: String? = nil, resumeNote: String? = nil, priority: Int? = nil,
        tags: [String]? = nil, context: String? = nil, pinned: Bool? = nil, review: Bool? = nil,
        due: String? = nil, deferUntil: String? = nil
    ) {
        self.title = title
        self.description = description
        self.resumeNote = resumeNote
        self.priority = priority
        self.tags = tags
        self.context = context
        self.pinned = pinned
        self.review = review
        self.due = due
        self.deferUntil = deferUntil
    }

    enum CodingKeys: String, CodingKey {
        case title, description, priority, tags, context, pinned, review, due
        case resumeNote = "resume_note"
        case deferUntil = "defer"
    }
}

/// The planning boundary's intents. Today and Next exclude each other.
public enum PlanAction: String, Codable, Sendable, CaseIterable {
    case addToday = "add_today"
    case removeToday = "remove_today"
    case addNext = "add_next"
    case removeNext = "remove_next"
    case backlog
}

/// What the shared engine judged when it refused: a field and why, the two
/// states of a forbidden move, or the open subtasks blocking a completion.
public struct RefusalDetails: Decodable, Hashable, Sendable {
    public struct BlockingChild: Decodable, Hashable, Sendable {
        public var shortID: Int
        public var title: String
        public var state: String

        enum CodingKeys: String, CodingKey {
            case title, state
            case shortID = "short_id"
        }
    }

    public var kind: String?
    public var field: String?
    public var reason: String?
    public var from: String?
    public var to: String?
    public var shortID: Int?
    public var children: [BlockingChild]?
    public var line: Int?
    public var lines: [CaptureLineError]?

    enum CodingKeys: String, CodingKey {
        case kind, field, reason, from, to, children, line, lines
        case shortID = "short_id"
    }

    /// Decodes each field on its own, so a detail another op spells
    /// differently never costs the ones this client reads.
    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        kind = try? container.decodeIfPresent(String.self, forKey: .kind)
        field = try? container.decodeIfPresent(String.self, forKey: .field)
        reason = try? container.decodeIfPresent(String.self, forKey: .reason)
        from = try? container.decodeIfPresent(String.self, forKey: .from)
        to = try? container.decodeIfPresent(String.self, forKey: .to)
        shortID = try? container.decodeIfPresent(Int.self, forKey: .shortID)
        children = try? container.decodeIfPresent([BlockingChild].self, forKey: .children)
        line = try? container.decodeIfPresent(Int.self, forKey: .line)
        lines = try? container.decodeIfPresent([CaptureLineError].self, forKey: .lines)
    }

    public init() {}
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
    /// What the engine judged, when it said. Absent for most codes.
    public var details: RefusalDetails?

    public var code: Code { Code(rawValue: rawCode) ?? .unrecognized }
    public var description: String { "\(rawCode): \(message)" }

    public init(rawCode: String, message: String, retryable: Bool, details: RefusalDetails? = nil) {
        self.rawCode = rawCode
        self.message = message
        self.retryable = retryable
        self.details = details
    }

    /// The refusal in a person's words, built from what the engine judged
    /// rather than from its message wherever the details say enough.
    public var explanation: String {
        guard let details else { return message }
        if let lines = details.lines, !lines.isEmpty {
            return lines.map { "Line \($0.line): \($0.message)" }.joined(separator: "\n")
        }
        let prefix = details.line.map { "Line \($0): " } ?? ""
        switch details.kind {
        case "incomplete_children":
            let children = (details.children ?? []).map { "#\($0.shortID) \($0.title) (\($0.state))" }
            let owner = details.shortID.map { "#\($0)" } ?? "This task"
            return prefix + "\(owner) still has open subtasks: " + children.joined(separator: ", ")
        case "invalid_transition":
            return prefix + "Can’t move from \(details.from ?? "?") to \(details.to ?? "?"): \(details.reason ?? message)"
        case "invalid_input":
            if let field = details.field, let reason = details.reason {
                return prefix + "\(field.prefix(1).uppercased() + field.dropFirst()): \(reason)"
            }
            return prefix + message
        default:
            return prefix + message
        }
    }

    enum CodingKeys: String, CodingKey {
        case message, retryable, details
        case rawCode = "code"
    }
}

// MARK: Organizing

/// A checklist action. Every action naming an item by position carries the
/// checklist as it was read, so the engine refuses it if another edit moved
/// the item.
public enum ChecklistAction: Sendable {
    case add(String)
    case edit(index: Int, text: String)
    case toggle(index: Int)
    case remove(index: Int)
    case move(index: Int, to: Int)
    case promote(index: Int)
}

/// A checklist action's outcome: the owner, and the subtask a promotion made.
public struct ChecklistMutation: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var child: TaskSnapshot?
    public var committed: CommittedEffect
}

/// A check-in action: one schedule over a selection, its clear, or one
/// task's acknowledgement.
public enum CheckInAction: Sendable {
    /// `date` is a calendar day as `YYYY-MM-DD`; zero repeat days is a
    /// one-off.
    case set(date: String, repeatDays: Int)
    case clear
    case acknowledge
}

/// A change to a task's place in its project's outline.
public enum HierarchyAction: Sendable {
    case addChild(String)
    case setParent(TaskLocator)
    case clearParent
    case indent
    case outdent
    case moveUp
    case moveDown
}

public struct HierarchyMutation: Decodable, Hashable, Sendable {
    public var task: TaskSnapshot
    public var created: TaskSnapshot?
    public var committed: CommittedEffect
}

/// One action over a selection of any size, one transaction for all of it.
public enum BulkAction: Sendable {
    case plan(PlanAction)
    case state(WorkflowState)
    case tags(add: [String], remove: [String])
    case move(projectID: String)
}

/// A task's address before and after a move between projects.
public struct AddressMove: Decodable, Hashable, Sendable {
    public var from: TaskLocator
    public var to: TaskLocator
}

/// What a selection action committed: each task as it now stands and every
/// address a move changed.
public struct TasksMutation: Decodable, Hashable, Sendable {
    public var tasks: [TaskSnapshot]
    public var moves: [AddressMove]
    public var committed: CommittedEffect
}

/// What deleting a project would remove. A delete must echo the impact the
/// person confirmed, digest included.
public struct ProjectImpact: Codable, Hashable, Sendable {
    public var tasks: Int
    public var notes: Int
    public var attachments: Int
    public var digest: String
}

public struct ProjectDetail: Decodable, Hashable, Sendable {
    public var project: ProjectSummary
    public var impact: ProjectImpact
    /// The project's own comments in stored order.
    public var comments: [CommentRecord]?
}

/// A project lifecycle action.
public enum ProjectAction: Sendable {
    case create(name: String)
    /// Saves the name, description, and complete tag set together.
    case edit(name: String, description: String, tags: [String])
    case setTags([String])
    case archive
    case unarchive
    /// Deletes the project and everything it owns, if it still holds exactly
    /// what `confirm` describes.
    case delete(confirm: ProjectImpact)
}

public struct ProjectMutation: Decodable, Hashable, Sendable {
    public var project: ProjectSummary
    public var committed: CommittedEffect
}

/// The durable ordering domain a list edits.
public enum TaskOrder: String, Encodable, Sendable {
    case today, next, siblings
}
