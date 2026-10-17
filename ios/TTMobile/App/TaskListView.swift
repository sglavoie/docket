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
    @State private var projectNotes: [NoteRecord] = []
    @State private var notesFailure: String?
    @State private var projectNames: [String: String] = [:]
    @State private var diagnostics: [QueryDiagnostic] = []
    @State private var state: ScreenState? = .loading("Loading…")
    @State private var search = ""
    @State private var suggestions: [SearchSuggestion] = []
    @State private var filter = TaskFilter()
    @State private var sort = TaskSort()
    @State private var selecting = false
    @State private var selection = Set<TaskLocator>()
    @State private var actionFailure: String?
    @State private var acting = false
    @State private var editingRow: TaskRow?
    @State private var openedNote: NoteRecord?
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
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .searchSuggestions {
                ForEach(suggestions, id: \.token) { suggestion in
                    Text(suggestion.token)
                        .searchCompletion(suggestion.search)
                        .accessibilityIdentifier("search.suggestion.\(suggestion.token)")
                }
            }
            .onChange(of: search) { suggestions = [] }
            .toolbar {
                ToolbarItemGroup(placement: .topBarTrailing) {
                    if selecting {
                        // The selection's actions sit beside Done: a bottom
                        // bar would sit under the tab bar.
                        BulkActionsButton(workspace: workspace, selection: Array(selection)) { refusal, moved in
                            actionFailure = refusal
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
                        actionFailure = nil
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
                if let actionFailure {
                    Text(actionFailure)
                        .font(.footnote)
                        .foregroundStyle(.red)
                        .padding(8)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(.regularMaterial)
                        .accessibilityIdentifier("list.failure")
                }
            }
            .sheet(item: $editingRow) { row in
                TaskEditSheet(workspace: workspace, task: row.item.task)
            }
            .navigationDestination(item: $openedNote) { note in
                NoteEditorView(workspace: workspace, location: note.location, filename: note.filename)
            }
            .refreshable { await workspace.refresh() }
            .task(id: LoadKey(generation: workspace.generation, search: search, filter: filter, sort: sort)) {
                if !search.isEmpty {
                    try? await Task.sleep(for: .milliseconds(250))
                    if Task.isCancelled { return }
                }
                await loadSuggestions()
                if Task.isCancelled { return }
                await load()
            }
    }

    /// The Inbox also lists what the share extension and shortcuts saved
    /// and the engine has not imported yet.
    private var showsPendingCaptures: Bool {
        source == .inbox && !workspace.captures.pending.isEmpty
    }

    private var showsProjectNotes: Bool {
        source.projectID != nil && source.population != "+archived"
            && search.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && !filter.isActive
    }

    @ViewBuilder
    private var content: some View {
        if let state, !(showsPendingCaptures && state.isEmpty) {
            ScreenStateView(state: state)
        } else {
            List(selection: $selection) {
                if showsPendingCaptures {
                    PendingCapturesSection(workspace: workspace)
                }
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
                    let projectName = projectNames[row.item.task.projectID]
                    NavigationLink(value: Route.task(row.item.task.locator)) {
                        TaskRowView(item: row.item, projectName: projectName)
                            .padding(.leading, CGFloat(row.depth) * 18)
                    }
                    .accessibilityLabel(TaskRowView.spokenLabel(row.item, projectName: projectName))
                    .accessibilityValue(row.depth > 0 ? "level \(row.depth)" : "")
                    .accessibilityIdentifier("task.row")
                    .tag(row.id)
                    .moveDisabled(!canReorder)
                    .swipeActions(edge: .trailing, allowsFullSwipe: false) {
                        if !selecting {
                            statusActions(row)
                                .disabled(acting)
                        }
                    }
                    .swipeActions(edge: .leading, allowsFullSwipe: false) {
                        if !selecting, !row.item.task.state.isTerminal {
                            planActions(row)
                                .disabled(acting)
                        }
                    }
                }
                .onMove(perform: moveRows)
                if showsProjectNotes, !projectNotes.isEmpty || notesFailure != nil {
                    Section("Notes") {
                        ForEach(projectNotes) { note in
                            // An explicit destination keeps the task List's
                            // selection and the note's navigation independent.
                            Button { openedNote = note } label: {
                                HStack {
                                    NoteRowLabel(note: note, showsOwner: false)
                                    Spacer()
                                    Image(systemName: "chevron.right")
                                        .font(.caption.weight(.semibold))
                                        .foregroundStyle(.tertiary)
                                        .accessibilityHidden(true)
                                }
                            }
                            .foregroundStyle(.primary)
                            .accessibilityIdentifier("note.\(note.title)")
                            .disabled(selecting)
                        }
                        if let notesFailure {
                            Label(notesFailure, systemImage: "exclamationmark.triangle")
                                .foregroundStyle(.orange)
                                .accessibilityIdentifier("list.notes.failure")
                        }
                    }
                }
            }
            .accessibilityIdentifier("list.tasks")
            // The last row scrolls clear of the floating capture button.
            .contentMargins(.bottom, 72, for: .scrollContent)
        }
    }

    private var supportsManualOrder: Bool {
        source.lens == nil && source.mode != "due" && source.mode != "recent"
    }

    private var effectiveSort: TaskSort? {
        if sort.field != nil { return sort }
        if source.outline || ["today", "next", "inbox"].contains(source.mode ?? "") {
            return TaskSort(field: "manual")
        }
        return nil
    }

    private var canReorder: Bool {
        supportsManualOrder && effectiveSort?.field == "manual"
            && !selecting && !acting && !filter.isActive
            && search.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && diagnostics.isEmpty
    }

    /// Native List moves begin with a long press, without entering selection
    /// mode. A project drag changes sibling order, never parentage.
    private func moveRows(from offsets: IndexSet, to destination: Int) {
        guard canReorder, offsets.count == 1, let from = offsets.first,
              rows.indices.contains(from), (0...rows.count).contains(destination),
              destination != from, destination != from + 1 else { return }
        let moved = rows[from]
        let kind: TaskOrder = source.mode == "today" ? .today : source.mode == "next" ? .next : .siblings
        var proposed = rows
        proposed.move(fromOffsets: offsets, toOffset: destination)
        var order: [TaskRow]
        if kind == .siblings {
            let task = moved.item.task
            let peer: (TaskRow) -> Bool = {
                $0.item.task.projectID == task.projectID && $0.item.task.parentID == task.parentID
            }
            // Find the sibling whose branch contains the drop location. This
            // keeps a parent's descendants attached when dragging past them.
            let crossed = destination > from
                ? Array(rows[(from + 1)..<destination])
                : Array(rows[destination..<from])
            guard crossed.contains(where: peer) else {
                actionFailure = "Move tasks among siblings in the same project."
                return
            }
            order = proposed.filter(peer)
            if order.map(\.id) == rows.filter(peer).map(\.id) { return }
            // Permute only sibling slots; outline() then moves whole branches.
            var peers = order.makeIterator()
            proposed = rows.map { peer($0) ? peers.next()! : $0 }
            if source.outline {
                proposed = Self.outline(proposed.map(\.item))
            }
        } else {
            order = proposed
        }
        rows = proposed
        acting = true
        actionFailure = nil
        Task {
            actionFailure = await workspace.reorder(moved.id, in: kind, order: order.map(\.id))
            // Requery on both success and refusal, so an optimistic move can
            // never survive a rejected write or hide another device's edits.
            await load()
            acting = false
        }
    }

    @ViewBuilder
    private func statusActions(_ row: TaskRow) -> some View {
        let task = row.item.task
        if task.state.isTerminal {
            Button("Reopen", systemImage: "arrow.uturn.backward") {
                act { await workspace.transition(task.locator, to: .inbox) }
            }
            .tint(.blue)
            .accessibilityIdentifier("task.swipe.reopen")
        } else {
            Button("Done", systemImage: "checkmark") {
                act { await workspace.transition(task.locator, to: .done) }
            }
            .tint(.green)
            .accessibilityIdentifier("task.swipe.done")

            Button(task.state == .doing ? "Pause" : "Start", systemImage: task.state == .doing ? "pause.fill" : "play.fill") {
                act { await workspace.transition(task.locator, to: task.state == .doing ? .paused : .doing) }
            }
            .tint(.orange)
            .accessibilityIdentifier("task.swipe.startPause")
        }
        Button("Edit", systemImage: "pencil") {
            actionFailure = nil
            editingRow = row
        }
        .tint(.gray)
        .accessibilityIdentifier("task.swipe.edit")
    }

    @ViewBuilder
    private func planActions(_ row: TaskRow) -> some View {
        let item = row.item
        Button(item.today ? "Remove Today" : "Today", systemImage: item.today ? "calendar.badge.minus" : "calendar") {
            act { await workspace.plan(item.task.locator, item.today ? .removeToday : .addToday) }
        }
        .tint(.blue)
        .accessibilityLabel(item.today ? "Remove from Today" : "Add to Today")
        .accessibilityIdentifier("task.swipe.today")

        Button(item.next ? "Remove Next" : "Next", systemImage: item.next ? "minus.circle" : "arrow.right.circle") {
            act { await workspace.plan(item.task.locator, item.next ? .removeNext : .addNext) }
        }
        .tint(.indigo)
        .accessibilityLabel(item.next ? "Remove from Next" : "Add to Next")
        .accessibilityIdentifier("task.swipe.next")
    }

    /// Let the engine judge each action and keep refusals beside the list.
    private func act(_ command: @escaping () async -> String?) {
        guard !acting else { return }
        acting = true
        actionFailure = nil
        Task {
            actionFailure = await command()
            acting = false
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
                if supportsManualOrder {
                    Text("Manual order").tag(Optional("manual"))
                }
                ForEach(["priority", "due", "created", "updated", "status", "title"], id: \.self) { field in
                    Text(field.capitalized).tag(Optional(field))
                }
            }
            Toggle("Ascending", isOn: $sort.ascending)
                .disabled(sort.field == nil || sort.field == "manual")
        } label: {
            Image(systemName: "arrow.up.arrow.down.circle")
        }
        .accessibilityLabel("Sort")
        .accessibilityIdentifier("list.sort")
    }

    private var queryClauses: String? {
        var clauses = filter.clauses
        if let population = source.population {
            clauses.insert(population, at: 0)
        }
        return clauses.isEmpty ? nil : clauses.joined(separator: " ")
    }

    private func loadSuggestions() async {
        guard !search.isEmpty else {
            suggestions = []
            return
        }
        do {
            let result = try await workspace.engine.searchSuggestions(
                mode: source.lens == nil ? source.mode : nil,
                scope: source.lens == nil ? source.scope : nil,
                query: source.lens == nil ? queryClauses : nil,
                lens: source.lens, search: search
            )
            if !Task.isCancelled { suggestions = result }
        } catch {
            // Suggestions are optional; the result query reports any refusal.
            if !Task.isCancelled { suggestions = [] }
        }
    }

    private func load() async {
        if rows.isEmpty, state == nil {
            state = .loading("Loading…")
        }
        let trimmed = search.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            let result: TaskQueryResult
            if let lens = source.lens {
                result = try await workspace.engine.tasks(lens: lens, search: trimmed.isEmpty ? nil : trimmed)
            } else {
                result = try await workspace.engine.tasks(
                    mode: source.mode, scope: source.scope,
                    query: queryClauses,
                    sort: effectiveSort,
                    search: trimmed.isEmpty ? nil : trimmed
                )
            }
            var names: [String: String] = [:]
            if source.projectID == nil {
                let projects = try await workspace.engine.projects()
                for project in projects.active + projects.parked + projects.archived {
                    names[project.id] = project.name
                }
            }
            var loadedNotes: [NoteRecord] = []
            var noteError: String?
            if showsProjectNotes, let projectID = source.projectID {
                do {
                    loadedNotes = try await workspace.engine.notes(at: .project(projectID))
                } catch {
                    noteError = "Notes could not load: \(WorkspaceModel.describe(error))"
                }
            }
            if Task.isCancelled { return }
            projectNotes = loadedNotes
            notesFailure = noteError
            projectNames = names
            diagnostics = result.diagnostics
            let reordered = (sort.field != nil && sort.field != "manual") || filter.isActive || !trimmed.isEmpty
            rows = source.outline && !reordered ? Self.outline(result.items) : result.items.map { TaskRow(item: $0, depth: 0) }
            if rows.isEmpty, diagnostics.isEmpty, projectNotes.isEmpty, notesFailure == nil {
                state = trimmed.isEmpty && !filter.isActive
                    ? .empty(title: "No tasks", message: source.emptyMessage)
                    : .empty(title: "No matches", message: "No task matches this search or filter.")
            } else {
                state = nil
            }
        } catch let failure as BridgeError where failure.code == .notFound {
            if Task.isCancelled { return }
            rows = []
            state = .unavailable(title: "Not available", message: failure.message)
        } catch let failure as BridgeError where failure.code == .validation {
            if Task.isCancelled { return }
            rows = []
            state = .error(title: "This search cannot run", message: failure.message)
        } catch {
            if Task.isCancelled { return }
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
    let projectName: String?
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    /// The badges sit on one line until an accessibility text size, where
    /// one line would break them mid-word; then each takes its own.
    private var badgeLayout: AnyLayout {
        dynamicTypeSize.isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: 2))
            : AnyLayout(HStackLayout(spacing: 8))
    }

    private var metadata: String {
        var parts = [item.task.state.rawValue.capitalized]
        if item.today { parts.append("Today") }
        if item.next { parts.append("Next") }
        if let projectName, !projectName.isEmpty { parts.append(projectName) }
        return parts.joined(separator: " · ")
    }

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 10) {
            Image(systemName: item.task.state.symbol)
                .foregroundStyle(item.task.state.isTerminal ? Color.secondary : Color.accentColor)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 2) {
                Text(item.task.title)
                    .strikethrough(item.task.state.isTerminal)
                Text(metadata)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                if !item.task.tags.isEmpty {
                    Text(item.task.tags.map { "#\($0)" }.joined(separator: " "))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                if let context = item.task.context, !context.isEmpty {
                    Text("##\(context)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                if item.task.priority > 0 || item.task.dueDate != nil || item.task.pinned || item.task.review {
                    badgeLayout {
                        if item.task.priority > 0 {
                            Text("P\(item.task.priority)").foregroundStyle(.orange)
                        }
                        if let due = item.task.dueDate {
                            Label(due.formatted(date: .abbreviated, time: .omitted), systemImage: "calendar")
                                .labelStyle(DueLabelStyle(compact: dynamicTypeSize.isAccessibilitySize))
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        if item.task.pinned { Image(systemName: "pin.fill") }
                        if item.task.review { Image(systemName: "flag.fill") }
                    }
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    // Each badge takes the height its text needs rather than
                    // being clipped to the row's first guess.
                    .fixedSize(horizontal: false, vertical: true)
                }
            }
        }
    }

    /// What VoiceOver says for a row, in the order it is read: the state,
    /// the title, then each badge in words rather than its symbol or
    /// abbreviation, followed by tags, context and then the project when the
    /// list spans projects.
    static func spokenLabel(_ item: TaskQueryItem, projectName: String?) -> String {
        var parts = [item.task.state.rawValue, item.task.title]
        if item.task.priority > 0 { parts.append("priority \(item.task.priority)") }
        if let due = item.task.dueDate { parts.append("due \(due.formatted(date: .abbreviated, time: .omitted))") }
        if item.today { parts.append("Today") }
        if item.next { parts.append("Next") }
        if item.task.pinned { parts.append("pinned") }
        if item.task.review { parts.append("review") }
        if !item.task.tags.isEmpty { parts.append("tags \(item.task.tags.joined(separator: ", "))") }
        if let context = item.task.context, !context.isEmpty { parts.append("context \(context)") }
        if let projectName { parts.append(projectName) }
        return parts.joined(separator: ", ")
    }
}

/// The due badge: icon and date, or the date alone where an accessibility
/// text size leaves no room for both.
private struct DueLabelStyle: LabelStyle {
    let compact: Bool

    func makeBody(configuration: Configuration) -> some View {
        if compact {
            configuration.title
        } else {
            Label(configuration)
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
