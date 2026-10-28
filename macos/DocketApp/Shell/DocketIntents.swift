import AppIntents

/// What Shortcuts, Spotlight, and Siri ask of Docket.app. AppDelegate answers, and
/// registers itself before any intent can run. Callbacks run on the main queue.
protocol DocketShortcutHost: AnyObject {
    /// Captures `text` through Docket's grammar as one transaction, each line its
    /// own task, and calls back with whether it did and what Docket said.
    func shortcutCapture(_ text: String, done: @escaping (Bool, String) -> Void)
    /// The live pulse, or why there is none.
    func shortcutPulse(_ done: @escaping (DocketPulse?, String?) -> Void)
    /// Shows Docket, in `view` when one is named.
    func shortcutShow(_ view: DocketView?)
    /// The live agenda, or why there is none. Like the pulse, it never starts Docket.
    func shortcutAgenda(_ done: @escaping (DocketAgenda?, String?) -> Void)
    /// Does `action` to `task`, as the agenda menu's act does, and calls back
    /// with whether Docket did and what it said. It never starts Docket.
    func shortcutPerform(_ action: DocketShortcutTaskAction, on task: DocketTaskID, done: @escaping (Bool, String) -> Void)
    /// Ranks every task against `text` as Docket's `/` does, at most `limit` of
    /// them, or says why it could not. Like a count, it starts Docket in the
    /// background only.
    func shortcutSearch(_ text: String, limit: Int, done: @escaping (DocketSearchResults?, String?) -> Void)
    /// Shows Docket with the cursor on `task`.
    func shortcutShowTask(_ task: DocketTaskID)
    /// The projects a Focus may keep, or why there are none. Like a count, it
    /// starts Docket in the background only.
    func shortcutProjects(_ done: @escaping ([DocketProject]?, String?) -> Void)
    /// The saved Focus changed: tell the running Docket.
    func focusChanged()
}

/// What a shortcut may do to one task: the agenda menu's acts, with the
/// deferral's length the shortcut's to choose.
enum DocketShortcutTaskAction: Equatable {
    case done, acknowledge
    case deferFor(minutes: Int)

    var request: (DocketTaskID) -> DocketControlRequest {
        switch self {
        case .done: return { .done($0) }
        case .acknowledge: return { .acknowledge($0) }
        case .deferFor(let minutes): return { .deferTask($0, minutes: minutes) }
        }
    }
}

enum DocketShortcuts {
    static weak var host: DocketShortcutHost?

    static let unavailable = "Docket.app is still starting. Try again in a moment."

    @MainActor
    static func capture(_ text: String) async throws -> String {
        guard let host else { throw DocketShortcutError(unavailable) }
        let (ok, message) = await withCheckedContinuation { continuation in
            host.shortcutCapture(text) { continuation.resume(returning: ($0, $1)) }
        }
        guard ok else { throw DocketShortcutError(message) }
        return message
    }

    @MainActor
    static func pulse() async throws -> DocketPulse {
        guard let host else { throw DocketShortcutError(unavailable) }
        let (pulse, reason) = await withCheckedContinuation { continuation in
            host.shortcutPulse { continuation.resume(returning: ($0, $1)) }
        }
        guard let pulse else { throw DocketShortcutError(reason ?? "Docket did not report its counts.") }
        return pulse
    }

    @MainActor
    static func agenda() async throws -> DocketAgenda {
        guard let host else { throw DocketShortcutError(unavailable) }
        let (agenda, reason) = await withCheckedContinuation { continuation in
            host.shortcutAgenda { continuation.resume(returning: ($0, $1)) }
        }
        guard let agenda else { throw DocketShortcutError(reason ?? "Docket did not report its tasks.") }
        return agenda
    }

    @MainActor
    static func search(_ text: String, limit: Int) async throws -> DocketSearchResults {
        guard let host else { throw DocketShortcutError(unavailable) }
        let (results, reason) = await withCheckedContinuation { continuation in
            host.shortcutSearch(text, limit: limit) { continuation.resume(returning: ($0, $1)) }
        }
        guard let results else { throw DocketShortcutError(reason ?? "Docket did not answer the search.") }
        return results
    }

    @MainActor
    static func projects() async throws -> [DocketProject] {
        guard let host else { throw DocketShortcutError(unavailable) }
        let (projects, reason) = await withCheckedContinuation { continuation in
            host.shortcutProjects { continuation.resume(returning: ($0, $1)) }
        }
        guard let projects else { throw DocketShortcutError(reason ?? "Docket did not list its projects.") }
        return projects
    }

    @MainActor
    static func perform(_ action: DocketShortcutTaskAction, on task: DocketTaskEntity) async throws -> String {
        guard let host else { throw DocketShortcutError(unavailable) }
        guard let id = task.taskID else { throw DocketShortcutError("That task is not one Docket.app can name.") }
        let (ok, message) = await withCheckedContinuation { continuation in
            host.shortcutPerform(action, on: id) { continuation.resume(returning: ($0, $1)) }
        }
        guard ok else { throw DocketShortcutError(message.isEmpty ? "Docket refused." : message) }
        return message
    }
}

/// A refusal Shortcuts shows as the action's error, worded as Docket worded it.
struct DocketShortcutError: Error, CustomLocalizedStringResourceConvertible {
    let message: String

    init(_ message: String) {
        self.message = message
    }

    var localizedStringResource: LocalizedStringResource { "\(message)" }
}

struct CaptureTaskIntent: AppIntent {
    static let title: LocalizedStringResource = "Capture Task"
    static let description = IntentDescription(
        "Adds a task to Docket, read with its capture grammar: #tag, @date, %project, $today, !1. Each line becomes its own task, all in one undo step."
    )
    static let openAppWhenRun = false

    @Parameter(title: "Task", inputOptions: String.IntentInputOptions(multiline: true))
    var text: String

    static var parameterSummary: some ParameterSummary {
        Summary("Capture \(\.$text) in Docket")
    }

    init() {}

    init(text: String) {
        self.text = text
    }

    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<String> & ProvidesDialog {
        let message = try await DocketShortcuts.capture(text)
        return .result(value: message, dialog: "\(message)")
    }
}

/// The modes a shortcut may open, as the status menu's counts name them.
enum DocketViewOption: String, AppEnum {
    case inbox, today, overdue

    static let typeDisplayRepresentation: TypeDisplayRepresentation = "Docket View"
    static let caseDisplayRepresentations: [Self: DisplayRepresentation] = [
        .inbox: "Inbox", .today: "Today", .overdue: "Overdue",
    ]

    var view: DocketView {
        switch self {
        case .inbox: return .inbox
        case .today: return .today
        case .overdue: return .due
        }
    }
}

struct ShowDocketIntent: AppIntent {
    static let title: LocalizedStringResource = "Show Docket"
    static let description = IntentDescription("Brings Docket forward, in Inbox, Today, or Overdue when one is chosen.")
    static let openAppWhenRun = true

    @Parameter(title: "View")
    var view: DocketViewOption?

    static var parameterSummary: some ParameterSummary {
        Summary("Show Docket in \(\.$view)")
    }

    @MainActor
    func perform() async throws -> some IntentResult {
        guard let host = DocketShortcuts.host else { throw DocketShortcutError(DocketShortcuts.unavailable) }
        host.shortcutShow(view?.view)
        return .result()
    }
}

/// The pulse's four counts.
enum DocketCountOption: String, AppEnum {
    case inbox, today, overdue, done

    static let typeDisplayRepresentation: TypeDisplayRepresentation = "Docket Count"
    static let caseDisplayRepresentations: [Self: DisplayRepresentation] = [
        .inbox: "Inbox", .today: "Today", .overdue: "Overdue", .done: "Done Today",
    ]

    func tally(in pulse: DocketPulse) -> Int {
        switch self {
        case .inbox: return pulse.inbox
        case .today: return pulse.today
        case .overdue: return pulse.overdue
        case .done: return pulse.done
        }
    }

    func sentence(_ count: Int) -> String {
        switch self {
        case .inbox: return "\(count) in Inbox"
        case .today: return "\(count) for Today"
        case .overdue: return "\(count) overdue"
        case .done: return "\(count) done today"
        }
    }
}

struct CountTasksIntent: AppIntent {
    static let title: LocalizedStringResource = "Count Docket Tasks"
    static let description = IntentDescription(
        "Returns one of the counts Docket's menu-bar icon shows: Inbox, Today, Overdue, or Done Today."
    )
    static let openAppWhenRun = false

    @Parameter(title: "Count", default: .overdue)
    var count: DocketCountOption

    static var parameterSummary: some ParameterSummary {
        Summary("Count \(\.$count) in Docket")
    }

    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<Int> & ProvidesDialog {
        let value = count.tally(in: try await DocketShortcuts.pulse())
        return .result(value: value, dialog: "\(count.sentence(value))")
    }
}

/// One Docket task: from the agenda, overdue or planned for today, or from a
/// search over every task. Its identifier carries the stored project and task
/// IDs, so a shortcut that saved one still names the same task once it has
/// left the agenda.
struct DocketTaskEntity: AppEntity {
    static let typeDisplayRepresentation: TypeDisplayRepresentation = "Docket Task"
    static let defaultQuery = DocketTaskQuery()

    let id: String

    @Property(title: "Title")
    var title: String

    @Property(title: "Project")
    var project: String

    @Property(title: "Overdue")
    var isOverdue: Bool

    @Property(title: "Deadline")
    var due: Date?

    @Property(title: "Priority")
    var priority: Int?

    /// The agenda line it was read from, for the subtitle; nil once rebuilt
    /// from an identifier alone.
    private let detail: String?

    init(_ item: DocketAgendaItem, now: Date = Date()) {
        id = Self.identifier(for: item.id)
        detail = item.detail(now: now)
        title = item.title
        project = item.projectName
        isOverdue = item.kind == .overdue
        due = item.dueDate
        priority = item.priority
    }

    init(_ item: DocketSearchItem, now: Date = Date()) {
        id = Self.identifier(for: item.id)
        let due = DocketAgendaItem.dueLabel(item.dueDate, hasTime: item.dueHasTime, now: now)
        detail = [item.projectName.isEmpty ? nil : item.projectName, due].compactMap { $0 }.joined(separator: " · ")
        title = item.title
        project = item.projectName
        isOverdue = item.overdue
        self.due = item.dueDate
        priority = item.priority
    }

    var displayRepresentation: DisplayRepresentation {
        guard let detail, !detail.isEmpty else { return DisplayRepresentation(title: "\(title)") }
        return DisplayRepresentation(title: "\(title)", subtitle: "\(detail)")
    }

    /// The stored IDs, each percent-encoded so the slash between them is the
    /// only one.
    static func identifier(for task: DocketTaskID) -> String {
        let allowed = CharacterSet.urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))
        let encode = { (part: String) in part.addingPercentEncoding(withAllowedCharacters: allowed) ?? part }
        return encode(task.project) + "/" + encode(task.task)
    }

    static func taskID(from identifier: String) -> DocketTaskID? {
        let parts = identifier.split(separator: "/", omittingEmptySubsequences: false)
        guard parts.count == 2,
              let project = parts[0].removingPercentEncoding, let task = parts[1].removingPercentEncoding,
              !project.isEmpty, !task.isEmpty else { return nil }
        return DocketTaskID(project: project, task: task)
    }

    var taskID: DocketTaskID? { Self.taskID(from: id) }
}

/// Resolves tasks from the live agenda, which is also what a picker suggests,
/// and finds any other task by asking Docket to search, so a shortcut can name
/// work that is neither overdue nor planned for today.
struct DocketTaskQuery: EntityStringQuery {
    /// How many tasks one typed search offers.
    static let searchLimit = 30
    @MainActor
    func entities(for identifiers: [String]) async throws -> [DocketTaskEntity] {
        let agenda = try await DocketShortcuts.agenda()
        let byID = Dictionary(agenda.items.map { (DocketTaskEntity.identifier(for: $0.id), $0) }, uniquingKeysWith: { first, _ in first })
        return identifiers.compactMap { byID[$0].map { DocketTaskEntity($0) } }
    }

    @MainActor
    func suggestedEntities() async throws -> [DocketTaskEntity] {
        try await DocketShortcuts.agenda().items.map { DocketTaskEntity($0) }
    }

    @MainActor
    func entities(matching string: String) async throws -> [DocketTaskEntity] {
        guard !string.trimmingCharacters(in: .whitespaces).isEmpty else { return try await suggestedEntities() }
        let now = Date()
        return try await DocketShortcuts.search(string, limit: Self.searchLimit).items.map { DocketTaskEntity($0, now: now) }
    }
}

/// Which part of the agenda a shortcut asks for.
enum DocketAgendaOption: String, AppEnum {
    case all, overdue, today

    static let typeDisplayRepresentation: TypeDisplayRepresentation = "Docket Agenda"
    static let caseDisplayRepresentations: [Self: DisplayRepresentation] = [
        .all: "Overdue and Today", .overdue: "Overdue", .today: "Today",
    ]

    func includes(_ item: DocketAgendaItem) -> Bool {
        switch self {
        case .all: return true
        case .overdue: return item.kind == .overdue
        case .today: return item.kind == .today
        }
    }
}

struct GetAgendaIntent: AppIntent {
    static let title: LocalizedStringResource = "Get Docket Agenda"
    static let description = IntentDescription(
        "Returns the tasks behind Docket's Overdue and Today counts, overdue first by oldest deadline, then Today by priority, as the menu-bar icon lists them. Docket names at most 20."
    )
    static let openAppWhenRun = false

    @Parameter(title: "Tasks", default: .all)
    var include: DocketAgendaOption

    static var parameterSummary: some ParameterSummary {
        Summary("Get \(\.$include) tasks from Docket")
    }

    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<[DocketTaskEntity]> {
        let now = Date()
        return .result(value: try await DocketShortcuts.agenda().items.filter(include.includes).map { DocketTaskEntity($0, now: now) })
    }
}

struct FindTasksIntent: AppIntent {
    static let title: LocalizedStringResource = "Find Docket Tasks"
    static let description = IntentDescription(
        "Searches every task as Docket's / search does, best match first, whatever Docket is showing: words match titles, descriptions, checklists, and comments, and #tag, %project, $doing, @today, or ?blocked narrow the answer. Finished and deferred work answers only a query that asks for it."
    )
    static let openAppWhenRun = false

    @Parameter(title: "Search")
    var query: String

    @Parameter(title: "Limit", default: 20, inclusiveRange: (1, 200))
    var limit: Int

    static var parameterSummary: some ParameterSummary {
        Summary("Find \(\.$query) in Docket") {
            \.$limit
        }
    }

    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<[DocketTaskEntity]> {
        let now = Date()
        return .result(value: try await DocketShortcuts.search(query, limit: limit).items.map { DocketTaskEntity($0, now: now) })
    }
}

struct ShowTaskIntent: AppIntent {
    static let title: LocalizedStringResource = "Show Docket Task"
    static let description = IntentDescription("Brings Docket forward with the cursor on a task, as a notification does.")
    static let openAppWhenRun = true

    @Parameter(title: "Task")
    var task: DocketTaskEntity

    static var parameterSummary: some ParameterSummary {
        Summary("Show \(\.$task) in Docket")
    }

    @MainActor
    func perform() async throws -> some IntentResult {
        guard let host = DocketShortcuts.host else { throw DocketShortcutError(DocketShortcuts.unavailable) }
        guard let id = task.taskID else { throw DocketShortcutError("That task is not one Docket.app can name.") }
        host.shortcutShowTask(id)
        return .result()
    }
}

struct MarkTaskDoneIntent: AppIntent {
    static let title: LocalizedStringResource = "Mark Docket Task Done"
    static let description = IntentDescription("Marks a task Done, as wd does on its row, in one undo step. Docket refuses it while something is open in it.")
    static let openAppWhenRun = false

    @Parameter(title: "Task")
    var task: DocketTaskEntity

    static var parameterSummary: some ParameterSummary {
        Summary("Mark \(\.$task) Done")
    }

    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<String> & ProvidesDialog {
        let message = try await DocketShortcuts.perform(.done, on: task)
        return .result(value: message, dialog: "\(message)")
    }
}

struct DeferTaskIntent: AppIntent {
    static let title: LocalizedStringResource = "Defer Docket Task"
    static let description = IntentDescription("Defers a task by a number of minutes from now, so it leaves Today and Overdue until then.")
    static let openAppWhenRun = false

    @Parameter(title: "Task")
    var task: DocketTaskEntity

    @Parameter(title: "Minutes", default: 60, inclusiveRange: (1, 525_600))
    var minutes: Int

    static var parameterSummary: some ParameterSummary {
        Summary("Defer \(\.$task) by \(\.$minutes) minutes")
    }

    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<String> & ProvidesDialog {
        let message = try await DocketShortcuts.perform(.deferFor(minutes: minutes), on: task)
        return .result(value: message, dialog: "\(message)")
    }
}

struct AcknowledgeTaskIntent: AppIntent {
    static let title: LocalizedStringResource = "Acknowledge Docket Task"
    static let description = IntentDescription(
        "Reviews a task, as R does on its row: clears its review flag and moves its check-in on. Docket refuses a task with neither."
    )
    static let openAppWhenRun = false

    @Parameter(title: "Task")
    var task: DocketTaskEntity

    static var parameterSummary: some ParameterSummary {
        Summary("Acknowledge \(\.$task)")
    }

    @MainActor
    func perform() async throws -> some IntentResult & ReturnsValue<String> & ProvidesDialog {
        let message = try await DocketShortcuts.perform(.acknowledge, on: task)
        return .result(value: message, dialog: "\(message)")
    }
}

struct DocketAppShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: CaptureTaskIntent(),
            phrases: ["Capture a task in \(.applicationName)", "Add a task to \(.applicationName)"],
            shortTitle: "Capture Task",
            systemImageName: "plus.circle"
        )
        AppShortcut(
            intent: ShowDocketIntent(),
            phrases: ["Show \(.applicationName)", "Show \(\.$view) in \(.applicationName)"],
            shortTitle: "Show Docket",
            systemImageName: "checklist"
        )
        AppShortcut(
            intent: CountTasksIntent(),
            phrases: ["Count \(\.$count) in \(.applicationName)", "How much is overdue in \(.applicationName)"],
            shortTitle: "Count Tasks",
            systemImageName: "number"
        )
        AppShortcut(
            intent: GetAgendaIntent(),
            phrases: ["Get my \(.applicationName) agenda", "What is due in \(.applicationName)"],
            shortTitle: "Get Agenda",
            systemImageName: "list.bullet"
        )
        AppShortcut(
            intent: FindTasksIntent(),
            phrases: ["Find tasks in \(.applicationName)", "Search \(.applicationName)"],
            shortTitle: "Find Tasks",
            systemImageName: "magnifyingglass"
        )
    }
}
