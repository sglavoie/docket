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
    }
}
