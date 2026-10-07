import Foundation

/// A qualified task identity, as the bridge spells it.
public struct ReminderTask: Codable, Hashable, Sendable {
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

/// What a reminder is about. Each is a moment the workspace already records.
public enum ReminderKind: String, Codable, Hashable, Sendable {
    /// A scheduled check-in's day, announced at the reminder time of day.
    case checkIn = "check_in"
    /// A deadline carrying a time of day, announced at that time.
    case deadline
    /// A deferral lapsing, announced when the task returns to view.
    case deferralEnds = "deferral_ends"
}

/// One reminder the engine planned.
public struct PlannedReminder: Codable, Hashable, Sendable {
    /// The reminder's subject — one task's check-in, deadline, or deferral —
    /// stable across reschedules, so a moved moment replaces its notification.
    public var id: String
    public var kind: ReminderKind
    public var task: ReminderTask
    public var projectName: String
    public var shortID: Int
    public var title: String
    /// The instant the notification is delivered.
    public var fireAt: Date
    /// `fireAt` as wall-clock time in the plan's zone, `yyyy-MM-ddTHH:mm:ss`.
    public var localTime: String
    public var canAcknowledge: Bool
    public var canDefer: Bool
    /// Changes whenever anything the notification shows or does changes.
    public var fingerprint: String

    public init(
        id: String, kind: ReminderKind, task: ReminderTask, projectName: String, shortID: Int, title: String,
        fireAt: Date, localTime: String, canAcknowledge: Bool, canDefer: Bool, fingerprint: String
    ) {
        self.id = id
        self.kind = kind
        self.task = task
        self.projectName = projectName
        self.shortID = shortID
        self.title = title
        self.fireAt = fireAt
        self.localTime = localTime
        self.canAcknowledge = canAcknowledge
        self.canDefer = canDefer
        self.fingerprint = fingerprint
    }

    enum CodingKeys: String, CodingKey {
        case id, kind, task, title, fingerprint
        case projectName = "project_name"
        case shortID = "short_id"
        case fireAt = "fire_at"
        case localTime = "local_time"
        case canAcknowledge = "can_acknowledge"
        case canDefer = "can_defer"
    }
}

/// The reminders this device should hold now, planned in one zone.
public struct ReminderPlan: Codable, Hashable, Sendable {
    /// The IANA zone the plan's wall-clock reminders were placed in.
    public var timeZone: String
    public var checkInTime: String
    public var generatedAt: Date
    public var truncated: Bool
    public var reminders: [PlannedReminder]

    public init(timeZone: String, checkInTime: String, generatedAt: Date, truncated: Bool, reminders: [PlannedReminder]) {
        self.timeZone = timeZone
        self.checkInTime = checkInTime
        self.generatedAt = generatedAt
        self.truncated = truncated
        self.reminders = reminders
    }

    enum CodingKeys: String, CodingKey {
        case reminders, truncated
        case timeZone = "time_zone"
        case checkInTime = "check_in_time"
        case generatedAt = "generated_at"
    }
}

/// The device's reminder preferences. They are machine-local: whether this
/// iPhone notifies, and at what time of day a check-in day is announced, are
/// not workspace facts another device should inherit.
public struct ReminderSettings: Codable, Hashable, Sendable {
    /// Off until someone turns reminders on: notifications are opt-in.
    public var enabled: Bool
    public var checkInHour: Int
    public var checkInMinute: Int

    public static let `default` = ReminderSettings(enabled: false, checkInHour: 9, checkInMinute: 0)

    public init(enabled: Bool, checkInHour: Int, checkInMinute: Int) {
        self.enabled = enabled
        self.checkInHour = checkInHour
        self.checkInMinute = checkInMinute
    }

    /// The bridge's spelling of the check-in time of day.
    public var checkInTime: String {
        String(format: "%02d:%02d", checkInHour, checkInMinute)
    }

    enum CodingKeys: String, CodingKey {
        case enabled
        case checkInHour = "check_in_hour"
        case checkInMinute = "check_in_minute"
    }
}

/// Settings kept as one JSON file in the device's runtime directory.
public struct ReminderSettingsStore: Sendable {
    public let file: URL

    public init(file: URL) {
        self.file = file
    }

    public func load() -> ReminderSettings {
        guard let data = try? Data(contentsOf: file),
              let settings = try? JSONDecoder().decode(ReminderSettings.self, from: data)
        else { return .default }
        return settings
    }

    public func save(_ settings: ReminderSettings) throws {
        try FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        try encoder.encode(settings).write(to: file, options: .atomic)
    }
}
