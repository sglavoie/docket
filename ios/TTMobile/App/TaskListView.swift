import SwiftUI
import TTEngine

/// What a task list shows: a mode, a project, or a saved lens. Every list is
/// one engine query; nothing here filters or orders tasks on its own beyond
/// laying a project's rows out as its outline.
struct ListSource: Hashable {
    var title: String
    var mode: String?
    var scope: String?
    var lens: String?
    /// A population clause a project list needs: `+parked` or `+archived`.
    var population: String?
    /// Lay the rows out as the project's outline when no sort, filter, or
    /// search reorders them.
    var outline = false

    /// The project a project list shows, which its settings manage.
    var projectID: String? { outline ? scope : nil }

    static let today = ListSource(title: "Today", mode: "today")
    static let inbox = ListSource(title: "Inbox", mode: "inbox")

    static func mode(_ mode: String, title: String) -> ListSource {
        ListSource(title: title, mode: mode)
    }

    static func project(_ project: ProjectSummary, population: String?) -> ListSource {
        ListSource(title: project.name, mode: "all", scope: project.id, population: population, outline: true)
    }

    static func lens(_ lens: LensSummary) -> ListSource {
        ListSource(title: lens.name, lens: lens.id)
    }

    var emptyMessage: String {
        switch mode {
        case "today": "Nothing is planned for today."
        case "inbox": "Your inbox is clear."
        case "next": "Nothing is lined up next."
        case "due": "No open task has a deadline."
        case "backlog": "The backlog is empty."
        case "recent": "Nothing changed in the last 30 days."
        default: lens != nil ? "No task matches this lens." : "No open tasks here."
        }
    }
}

/// The narrowing a list adds on top of its source, spelled as canonical query
/// clauses the engine evaluates.
struct TaskFilter: Hashable {
    var state: WorkflowState?
    var priority: Int?
    var pinned = false
    var review = false

    var isActive: Bool { state != nil || priority != nil || pinned || review }

    var clauses: [String] {
        var clauses: [String] = []
        if let state { clauses.append("$\(state.rawValue)") }
        if let priority { clauses.append("!\(priority)") }
        if pinned { clauses.append("*pin") }
        if review { clauses.append("=review") }
        return clauses
    }
}

/// One row as a list lays it out.
struct TaskRow: Identifiable, Hashable {
    let item: TaskQueryItem
    let depth: Int
    var id: TaskLocator { item.task.locator }
}

struct TaskListView: View {
    let workspace: WorkspaceModel
    let source: ListSource

    @State private var rows: [TaskRow] = []
    @State private var diagnostics: [QueryDiagnostic] = []
    @State private var state: ScreenState? = .loading("Loading…")
    @State private var search = ""
    @State private var filter = TaskFilter()
    @State private var sort = TaskSort()
    @State private var selecting = false
    @State private var selection = Set<TaskLocator>()
    @State private var bulkFailure: String?
    @State private var managingProject = false
    @Environment(\.dismiss) private var dismiss

    private struct LoadKey: Hashable {
        var generation: Int
        var search: String
        var filter: TaskFilter
        var sort: TaskSort
    }

    var body: some View {
        content
            .navigationTitle(source.title)
            .searchable(text: $search, placement: .navigationBarDrawer(displayMode: .always), prompt: "Search titles, #tags, $states…")
            .toolbar {
                ToolbarItemGroup(placement: .topBarTrailing) {
                    if selecting {
                        // The selection's actions sit beside Done: a bottom
                        // bar would sit under the tab bar.
                        BulkActionsButton(workspace: workspace, selection: Array(selection)) { refusal, moved in
                            bulkFailure = refusal
                            // A moved task has a new address, so the selection
                            // naming the old one is spent; every other action
                            // leaves it in place for the next.
                            if refusal == nil, moved {
                                selection = []
                            }
                        }
                    }
                    Button(selecting ? "Done" : "Select") {
                        selecting.toggle()
                        selection = []
                        bulkFailure = nil
                    }
                    .accessibilityIdentifier("list.select")
                    if source.lens == nil, !selecting {
                        filterMenu
                        sortMenu
                    }
                    if let projectID = source.projectID, !selecting {
                        Button {
                            managingProject = true
                        } label: {
                            Image(systemName: "folder.badge.gearshape")
                        }
                        .accessibilityLabel("Project settings")
                        .accessibilityIdentifier("list.project.settings")
                        .sheet(isPresented: $managingProject) {
                            ProjectSettingsView(workspace: workspace, projectID: projectID) { dismiss() }
                        }
                    }
                }
            }
            .environment(\.editMode, .constant(selecting ? .active : .inactive))
            .safeAreaInset(edge: .top) {
                if let bulkFailure {
                    Text(bulkFailure)
                        .font(.footnote)
                        .foregroundStyle(.red)
                        .padding(8)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(.regularMaterial)
                        .accessibilityIdentifier("list.failure")
                }
            }
            .refreshable { await workspace.refresh() }
            .task(id: LoadKey(generation: workspace.generation, search: search, filter: filter, sort: sort)) {
                if !search.isEmpty {
                    try? await Task.sleep(for: .milliseconds(250))
                    if Task.isCancelled { return }
                }
                await load()
            }
    }

    @ViewBuilder
    private var content: some View {
        if let state {
            ScreenStateView(state: state)
        } else {
            List(selection: $selection) {
                if !diagnostics.isEmpty {
                    Section {
                        ForEach(diagnostics, id: \.projectID) { diagnostic in
                            Label("\(diagnostic.projectName) could not be read", systemImage: "exclamationmark.triangle")
                                .foregroundStyle(.orange)
                                .accessibilityIdentifier("list.diagnostic")
                        }
                    }
                }
                ForEach(rows) { row in
                    NavigationLink(value: Route.task(row.item.task.locator)) {
                        TaskRowView(item: row.item)
                            .padding(.leading, CGFloat(row.depth) * 18)
                    }
                    .accessibilityValue(row.depth > 0 ? "level \(row.depth)" : "")
                    .tag(row.id)
                    .swipeActions {
                        if !row.item.task.state.isTerminal {
                            Button("Done") { Task { await workspace.complete(row.item.task) } }
                                .tint(.green)
                        }
                    }
                }
            }
            .accessibilityIdentifier("list.tasks")
        }
    }

    private var filterMenu: some View {
        Menu {
            Picker("Status", selection: $filter.state) {
                Text("Any status").tag(WorkflowState?.none)
                ForEach(WorkflowState.allCases, id: \.self) { state in
                    Text(state.rawValue.capitalized).tag(Optional(state))
                }
            }
            Picker("Priority", selection: $filter.priority) {
                Text("Any priority").tag(Int?.none)
                ForEach(1...3, id: \.self) { priority in
                    Text("Priority \(priority)").tag(Optional(priority))
                }
            }
            Toggle("Pinned", isOn: $filter.pinned)
            Toggle("Flagged for review", isOn: $filter.review)
            if filter.isActive {
                Button("Clear Filters", role: .destructive) { filter = TaskFilter() }
            }
        } label: {
            Image(systemName: filter.isActive ? "line.3.horizontal.decrease.circle.fill" : "line.3.horizontal.decrease.circle")
        }
        .accessibilityLabel("Filter")
        .accessibilityIdentifier("list.filter")
    }

    private var sortMenu: some View {
        Menu {
            Picker("Sort by", selection: $sort.field) {
                Text("Default order").tag(String?.none)
                ForEach(["priority", "due", "created", "updated", "status", "title"], id: \.self) { field in
                    Text(field.capitalized).tag(Optional(field))
                }
            }
            Toggle("Ascending", isOn: $sort.ascending)
                .disabled(sort.field == nil)
        } label: {
            Image(systemName: "arrow.up.arrow.down.circle")
        }
        .accessibilityLabel("Sort")
        .accessibilityIdentifier("list.sort")
    }

    private func load() async {
        if rows.isEmpty, state == nil {
            state = .loading("Loading…")
        }
        var clauses = filter.clauses
        if let population = source.population {
            clauses.insert(population, at: 0)
        }
        let trimmed = search.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            let result: TaskQueryResult
            if let lens = source.lens {
                result = try await workspace.engine.tasks(lens: lens, search: trimmed.isEmpty ? nil : trimmed)
            } else {
                result = try await workspace.engine.tasks(
                    mode: source.mode, scope: source.scope,
                    query: clauses.isEmpty ? nil : clauses.joined(separator: " "),
                    sort: sort.field == nil ? nil : sort,
                    search: trimmed.isEmpty ? nil : trimmed
                )
            }
            diagnostics = result.diagnostics
            let reordered = sort.field != nil || filter.isActive || !trimmed.isEmpty
            rows = source.outline && !reordered ? Self.outline(result.items) : result.items.map { TaskRow(item: $0, depth: 0) }
            if rows.isEmpty, diagnostics.isEmpty {
                state = trimmed.isEmpty && !filter.isActive
                    ? .empty(title: "No tasks", message: source.emptyMessage)
                    : .empty(title: "No matches", message: "No task matches this search or filter.")
            } else {
                state = nil
            }
        } catch let failure as BridgeError where failure.code == .notFound {
            rows = []
            state = .unavailable(title: "Not available", message: failure.message)
        } catch let failure as BridgeError where failure.code == .validation {
            rows = []
            state = .error(title: "This search cannot run", message: failure.message)
        } catch {
            rows = []
            state = .error(title: "Something went wrong", message: WorkspaceModel.describe(error))
        }
    }

    /// The project's outline: each task under its parent, in the engine's
    /// order among siblings. A task whose parent the query left out starts its
    /// own branch rather than disappearing.
    static func outline(_ items: [TaskQueryItem]) -> [TaskRow] {
        let present = Set(items.map(\.task.taskID))
        var children: [String: [TaskQueryItem]] = [:]
        var roots: [TaskQueryItem] = []
        for item in items {
            if let parent = item.task.parentID, present.contains(parent) {
                children[parent, default: []].append(item)
            } else {
                roots.append(item)
            }
        }
        var rows: [TaskRow] = []
        func visit(_ item: TaskQueryItem, depth: Int) {
            rows.append(TaskRow(item: item, depth: depth))
            for child in children[item.task.taskID] ?? [] {
                visit(child, depth: depth + 1)
            }
        }
        roots.forEach { visit($0, depth: 0) }
        return rows
    }
}

extension WorkflowState {
    var isTerminal: Bool { self == .done || self == .canceled }

    var symbol: String {
        switch self {
        case .inbox: "tray"
        case .backlog: "circle"
        case .doing: "play.circle"
        case .waiting: "hourglass"
        case .paused: "pause.circle"
        case .someday: "moon"
        case .done: "checkmark.circle.fill"
        case .canceled: "xmark.circle"
        }
    }
}

struct TaskRowView: View {
    let item: TaskQueryItem

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 10) {
            Image(systemName: item.task.state.symbol)
                .foregroundStyle(item.task.state.isTerminal ? Color.secondary : Color.accentColor)
                .accessibilityLabel(item.task.state.rawValue)
            VStack(alignment: .leading, spacing: 2) {
                Text(item.task.title)
                    .strikethrough(item.task.state.isTerminal)
                HStack(spacing: 8) {
                    if item.task.priority > 0 {
                        Text("P\(item.task.priority)").foregroundStyle(.orange)
                    }
                    if let due = item.task.dueDate {
                        Label(due.formatted(date: .abbreviated, time: .omitted), systemImage: "calendar")
                    }
                    if item.today { Text("Today") }
                    if item.next { Text("Next") }
                    if item.task.pinned { Image(systemName: "pin.fill").accessibilityLabel("pinned") }
                    if item.task.review { Image(systemName: "flag.fill").accessibilityLabel("review") }
                }
                .font(.caption)
                .foregroundStyle(.secondary)
            }
        }
    }
}

/// The actions a selection takes: a plan, a status, a tag, or a project, each
/// one transaction for every selected task, refused whole if any one task
/// cannot take it.
struct BulkActionsButton: View {
    let workspace: WorkspaceModel
    let selection: [TaskLocator]
    /// Called with the refusal, if any, and whether the action moved tasks.
    let finished: (String?, Bool) -> Void

    @State private var choosing = false
    @State private var tagging = false
    @State private var tag = ""
    @State private var choosingProject = false
    @State private var projects: [ProjectSummary] = []

    var body: some View {
        Button("Actions") { choosing = true }
            .disabled(selection.isEmpty)
            .accessibilityIdentifier("list.bulk")
            .confirmationDialog("\(selection.count) selected", isPresented: $choosing, titleVisibility: .visible) {
                Button("Add to Today") { run(.plan(.addToday)) }
                Button("Add to Next") { run(.plan(.addNext)) }
                Button("Move to Backlog") { run(.plan(.backlog)) }
                Button("Start") { run(.state(.doing)) }
                Button("Mark Done") { run(.state(.done)) }
                Button("Add Tag…") {
                    tag = ""
                    afterDialogCloses { tagging = true }
                }
                Button("Move to Project…") {
                    Task {
                        projects = (try? await workspace.engine.projects())?.active ?? []
                        afterDialogCloses { choosingProject = true }
                    }
                }
            }
            .alert("Add Tag", isPresented: $tagging) {
                TextField("Tag", text: $tag)
                    .accessibilityIdentifier("bulk.tag")
                Button("Cancel", role: .cancel) {}
                Button("Add") { run(.tags(add: [tag], remove: [])) }
            }
            .confirmationDialog("Move to Project", isPresented: $choosingProject, titleVisibility: .visible) {
                ForEach(projects) { project in
                    Button(project.name) { run(.move(projectID: project.id)) }
                }
            }
    }

    private func run(_ action: BulkAction) {
        let targets = selection
        let moves: Bool
        if case .move = action { moves = true } else { moves = false }
        Task {
            finished(await workspace.bulk(targets, action).refusal, moves)
        }
    }
}
