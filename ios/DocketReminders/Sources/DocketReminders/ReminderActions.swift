import Foundation

/// What a person did with a reminder notification.
public enum ReminderAction: Hashable, Sendable {
    /// Tapped the notification: show the task.
    case open
    /// Acknowledged the review the reminder raised.
    case acknowledge
    /// Deferred the task; `value` is the deferral as the engine reads it.
    case deferTask(String)
    /// Swiped it away.
    case dismiss

    public static let acknowledgeIdentifier = "docket.reminder.acknowledge"
    public static let deferHourIdentifier = "docket.reminder.defer-hour"
    public static let deferTomorrowIdentifier = "docket.reminder.defer-tomorrow"
    /// UNNotificationDefaultActionIdentifier and UNNotificationDismissActionIdentifier.
    public static let defaultIdentifier = "com.apple.UNNotificationDefaultActionIdentifier"
    public static let dismissIdentifier = "com.apple.UNNotificationDismissActionIdentifier"

    public init?(identifier: String) {
        switch identifier {
        case Self.defaultIdentifier: self = .open
        case Self.dismissIdentifier: self = .dismiss
        case Self.acknowledgeIdentifier: self = .acknowledge
        // The engine's own deferral grammar: an hour from now, or the start
        // of tomorrow in the device's zone.
        case Self.deferHourIdentifier: self = .deferTask("+1h")
        case Self.deferTomorrowIdentifier: self = .deferTask("tomorrow")
        default: return nil
        }
    }
}

/// The engine commands a reminder action runs. The app passes its workspace;
/// the tests pass a fake.
public protocol ReminderCommands: Sendable {
    /// The workspace open now, or nil; an action for another workspace is
    /// refused rather than applied to whatever is open.
    func openWorkspaceID() async -> String?
    func acknowledge(_ task: ReminderTask) async throws
    func deferTask(_ task: ReminderTask, until value: String) async throws
    /// Shows the task. Opening writes nothing.
    func show(_ task: ReminderTask) async
}

/// One handled notification response, as the activity log keeps it.
public struct ReminderActivity: Codable, Hashable, Sendable, Identifiable {
    public enum Result: String, Codable, Hashable, Sendable {
        case opened
        case acknowledged
        case deferred
        case dismissed
        case refused
    }

    public var id: UUID
    public var at: Date
    public var result: Result
    public var kind: String
    public var title: String
    public var task: ReminderTask?
    public var detail: String?

    public init(id: UUID = UUID(), at: Date, result: Result, kind: String, title: String, task: ReminderTask?, detail: String? = nil) {
        self.id = id
        self.at = at
        self.result = result
        self.kind = kind
        self.title = title
        self.task = task
        self.detail = detail
    }

    /// One line a person reads.
    public var summary: String {
        switch result {
        case .opened: "Opened “\(title)”"
        case .acknowledged: "Acknowledged “\(title)”"
        case .deferred: "Deferred “\(title)” (\(detail ?? ""))"
        case .dismissed: "Dismissed “\(title)”"
        case .refused: "Could not act on “\(title)”: \(detail ?? "")"
        }
    }
}

/// The last few handled responses, kept in the device's runtime directory so
/// a response handled while the app was in the background is still readable
/// when it is next opened.
public struct ReminderActivityLog: Sendable {
    public static let capacity = 20
    public let file: URL

    public init(file: URL) {
        self.file = file
    }

    public func entries() -> [ReminderActivity] {
        guard let data = try? Data(contentsOf: file) else { return [] }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return (try? decoder.decode([ReminderActivity].self, from: data)) ?? []
    }

    public func append(_ activity: ReminderActivity) {
        var all = entries()
        all.insert(activity, at: 0)
        all = Array(all.prefix(Self.capacity))
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? encoder.encode(all).write(to: file, options: .atomic)
    }
}

/// Routes a notification response to the engine command it names.
public struct ReminderActionRouter: Sendable {
    public let commands: any ReminderCommands
    public let log: ReminderActivityLog?
    public let now: @Sendable () -> Date

    public init(commands: any ReminderCommands, log: ReminderActivityLog?, now: @escaping @Sendable () -> Date = Date.init) {
        self.commands = commands
        self.log = log
        self.now = now
    }

    /// Handles one response. Nothing is written for a response this app did
    /// not schedule, for another workspace's reminder, or for an action the
    /// engine refuses; the refusal is logged instead.
    @discardableResult
    public func handle(actionIdentifier: String, userInfo: [String: String]) async -> ReminderActivity? {
        guard let action = ReminderAction(identifier: actionIdentifier),
              let workspace = userInfo[UserInfoKey.workspace],
              let project = userInfo[UserInfoKey.project],
              let taskID = userInfo[UserInfoKey.task]
        else { return nil }
        let task = ReminderTask(projectID: project, taskID: taskID)
        let title = userInfo[UserInfoKey.title] ?? "a task"
        let kind = userInfo[UserInfoKey.kind] ?? ""
        func record(_ result: ReminderActivity.Result, _ detail: String? = nil) -> ReminderActivity {
            let activity = ReminderActivity(at: now(), result: result, kind: kind, title: title, task: task, detail: detail)
            log?.append(activity)
            return activity
        }
        if action == .dismiss {
            return record(.dismissed)
        }
        guard await commands.openWorkspaceID() == workspace else {
            return record(.refused, "it belongs to a workspace that is not open")
        }
        do {
            switch action {
            case .open:
                await commands.show(task)
                return record(.opened)
            case .acknowledge:
                try await commands.acknowledge(task)
                return record(.acknowledged)
            case .deferTask(let value):
                try await commands.deferTask(task, until: value)
                return record(.deferred, value == "+1h" ? "for an hour" : "until \(value)")
            case .dismiss:
                return record(.dismissed)
            }
        } catch {
            return record(.refused, Self.describe(error))
        }
    }

    private static func describe(_ error: Error) -> String {
        (error as? LocalizedError)?.errorDescription ?? String(describing: error)
    }
}
