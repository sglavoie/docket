import AppIntents
import CoreSpotlight
import Foundation
import DocketEngine
import DocketShared

/// A task as Siri, Shortcuts, and Spotlight know it. Its identifier is the
/// workspace-qualified locator a `docket://task` link spells, so an entity
/// minted for another workspace is refused rather than guessed at.
struct TaskEntity: AppEntity, IndexedEntity {
    static let typeDisplayRepresentation: TypeDisplayRepresentation = TypeDisplayRepresentation(
        name: "Docket Task", numericFormat: "\(placeholder: .int) tasks"
    )
    static let defaultQuery = TaskEntityQuery()

    let id: String
    @Property(title: "Title") var title: String
    @Property(title: "Project") var projectName: String
    @Property(title: "Status") var state: String

    init(task: TaskSnapshot, projectName: String, workspace: String) {
        id = [workspace, task.projectID, task.taskID].joined(separator: "|")
        self.title = task.title
        self.projectName = projectName
        self.state = task.state.rawValue
    }

    var displayRepresentation: DisplayRepresentation {
        DisplayRepresentation(
            title: "\(title)",
            subtitle: "\(projectName.isEmpty ? state.capitalized : "\(projectName) · \(state.capitalized)")",
            image: DisplayRepresentation.Image(systemName: DocketPalette.stateSymbol(state))
        )
    }

    var attributeSet: CSSearchableItemAttributeSet {
        let attributes = CSSearchableItemAttributeSet(contentType: .content)
        attributes.title = title
        attributes.contentDescription = [projectName, state.capitalized].filter { !$0.isEmpty }.joined(separator: " · ")
        attributes.keywords = ["docket", "task", projectName, state]
        return attributes
    }

    var parts: (workspace: String, locator: TaskLocator)? {
        let pieces = id.split(separator: "|", omittingEmptySubsequences: false).map(String.init)
        guard pieces.count == 3 else { return nil }
        return (pieces[0], TaskLocator(projectID: pieces[1], taskID: pieces[2]))
    }
}

struct TaskEntityQuery: EntityStringQuery {
    func entities(for identifiers: [String]) async throws -> [TaskEntity] {
        try await Self.lookup(identifiers)
    }

    func entities(matching string: String) async throws -> [TaskEntity] {
        try await Self.search(string)
    }

    func suggestedEntities() async throws -> [TaskEntity] {
        try await Self.suggestions()
    }

    @MainActor
    static func ready() async throws -> (WorkspaceModel, String) {
        guard let workspace = AppDelegate.current?.workspace, await workspace.ensureReady(),
              let workspaceID = workspace.status?.workspace
        else { throw TaskIntentError.unavailable }
        return (workspace, workspaceID)
    }

    @MainActor
    static func lookup(_ identifiers: [String]) async throws -> [TaskEntity] {
        let (workspace, workspaceID) = try await ready()
        let names = workspace.pulse?.projectNames ?? [:]
        var out: [TaskEntity] = []
        for identifier in identifiers {
            let pieces = identifier.split(separator: "|", omittingEmptySubsequences: false).map(String.init)
            guard pieces.count == 3, pieces[0] == workspaceID else { continue }
            let locator = TaskLocator(projectID: pieces[1], taskID: pieces[2])
            if let detail = try? await workspace.engine.task(locator) {
                out.append(TaskEntity(task: detail.task, projectName: names[locator.projectID] ?? detail.project.name,
                                      workspace: workspaceID))
            }
        }
        return out
    }

    @MainActor
    static func search(_ text: String) async throws -> [TaskEntity] {
        let (workspace, workspaceID) = try await ready()
        let result = try await workspace.engine.tasks(mode: "all", search: text)
        let names = workspace.pulse?.projectNames ?? [:]
        return result.items.prefix(25).map {
            TaskEntity(task: $0.task, projectName: names[$0.task.projectID] ?? "", workspace: workspaceID)
        }
    }

    @MainActor
    static func suggestions() async throws -> [TaskEntity] {
        let (workspace, workspaceID) = try await ready()
        let names = workspace.pulse?.projectNames ?? [:]
        var tasks = workspace.pulse?.doing ?? []
        if let today = try? await workspace.engine.tasks(mode: "today") {
            tasks += today.items.map(\.task).filter { task in !tasks.contains { $0.locator == task.locator } }
        }
        return tasks.prefix(20).map { TaskEntity(task: $0, projectName: names[$0.projectID] ?? "", workspace: workspaceID) }
    }
}

enum TaskIntentError: Error, CustomLocalizedStringResourceConvertible {
    case unavailable
    case otherWorkspace
    case refused(String)

    var localizedStringResource: LocalizedStringResource {
        switch self {
        case .unavailable: "Docket could not open its workspace. Open Docket and try again."
        case .otherWorkspace: "That task belongs to another Docket workspace."
        case .refused(let message): "\(message)"
        }
    }
}

/// Marks a task done from Siri, Shortcuts, or Spotlight.
struct CompleteTaskEntityIntent: AppIntent {
    static let title: LocalizedStringResource = "Complete Task"
    static let description = IntentDescription("Marks a Docket task done.")

    @Parameter(title: "Task") var task: TaskEntity

    static var parameterSummary: some ParameterSummary { Summary("Complete \(\.$task)") }

    init() {}
    init(task: TaskEntity) { self.task = task }

    @MainActor
    func perform() async throws -> some IntentResult & ProvidesDialog {
        let (workspace, workspaceID) = try await TaskEntityQuery.ready()
        guard let parts = task.parts, parts.workspace == workspaceID else { throw TaskIntentError.otherWorkspace }
        if let refusal = await workspace.transition(parts.locator, to: .done) { throw TaskIntentError.refused(refusal) }
        return .result(dialog: "Completed “\(task.title)”.")
    }
}

/// Starts a task: it moves to Doing and becomes what Now shows.
struct StartTaskEntityIntent: AppIntent {
    static let title: LocalizedStringResource = "Start Task"
    static let description = IntentDescription("Moves a Docket task to Doing.")

    @Parameter(title: "Task") var task: TaskEntity

    static var parameterSummary: some ParameterSummary { Summary("Start \(\.$task)") }

    @MainActor
    func perform() async throws -> some IntentResult & ProvidesDialog {
        let (workspace, workspaceID) = try await TaskEntityQuery.ready()
        guard let parts = task.parts, parts.workspace == workspaceID else { throw TaskIntentError.otherWorkspace }
        if let refusal = await workspace.transition(parts.locator, to: .doing) { throw TaskIntentError.refused(refusal) }
        return .result(dialog: "Started “\(task.title)”.")
    }
}

/// Opens a task's details, which is also what a Spotlight result does.
struct OpenTaskEntityIntent: OpenIntent {
    static let title: LocalizedStringResource = "Open Task"
    static let description = IntentDescription("Opens a Docket task.")

    @Parameter(title: "Task") var target: TaskEntity

    @MainActor
    func perform() async throws -> some IntentResult {
        guard let parts = target.parts else { throw TaskIntentError.otherWorkspace }
        AppDelegate.current?.workspace.linkRequest = .task(
            workspace: parts.workspace, projectID: parts.locator.projectID, taskID: parts.locator.taskID
        )
        return .result()
    }
}

/// Keeps Spotlight's copy of the open tasks current: a few seconds after the
/// last change, it replaces the whole set, so a finished or deleted task
/// leaves search as soon as it leaves the lists.
@MainActor
final class SpotlightIndexer {
    static let shared = SpotlightIndexer()
    private var pending: Task<Void, Never>?

    func schedule(engine: DocketEngine, workspace: String?) {
        pending?.cancel()
        guard let workspace else { return }
        pending = Task {
            try? await Task.sleep(for: .seconds(3))
            guard !Task.isCancelled,
                  let result = try? await engine.tasks(mode: "all"),
                  let projects = try? await engine.projects()
            else { return }
            var names: [String: String] = [:]
            for project in projects.active + projects.parked { names[project.id] = project.name }
            let entities = result.items.prefix(1000).map {
                TaskEntity(task: $0.task, projectName: names[$0.task.projectID] ?? "", workspace: workspace)
            }
            let index = CSSearchableIndex.default()
            try? await index.deleteAllSearchableItems()
            try? await index.indexAppEntities(entities)
        }
    }
}
