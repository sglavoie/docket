import Foundation

/// One row a widget draws (query.widget_snapshot).
public struct WidgetTask: Codable, Hashable, Sendable, Identifiable {
    public var projectID: String
    public var taskID: String
    public var shortID: Int
    public var title: String
    public var projectName: String
    public var state: String
    public var priority: Int
    public var dueDate: Date?
    public var overdue: Bool
    public var pinned: Bool

    public var id: String { projectID + "/" + taskID }

    enum CodingKeys: String, CodingKey {
        case title, state, priority, overdue, pinned
        case projectID = "project_id"
        case taskID = "task_id"
        case shortID = "short_id"
        case projectName = "project_name"
        case dueDate = "due_date"
    }

    public init(
        projectID: String, taskID: String, shortID: Int, title: String, projectName: String,
        state: String, priority: Int = 0, dueDate: Date? = nil, overdue: Bool = false, pinned: Bool = false
    ) {
        self.projectID = projectID
        self.taskID = taskID
        self.shortID = shortID
        self.title = title
        self.projectName = projectName
        self.state = state
        self.priority = priority
        self.dueDate = dueDate
        self.overdue = overdue
        self.pinned = pinned
    }
}

/// One view's first rows and how many it holds in all.
public struct WidgetList: Codable, Hashable, Sendable {
    public var count: Int
    public var tasks: [WidgetTask]

    public init(count: Int, tasks: [WidgetTask]) {
        self.count = count
        self.tasks = tasks
    }
}

/// Which list a widget shows.
public enum WidgetView: String, Codable, CaseIterable, Sendable {
    case today
    case inbox

    public var title: String {
        switch self {
        case .today: "Today"
        case .inbox: "Inbox"
        }
    }
}

/// The cache the app writes for its widgets, `tt-widget-snapshot/v1`: Today
/// and Inbox as the engine answered them at `generatedAt`, true for the local
/// day `day` and no longer once `validUntil` (the next local midnight) passes.
public struct WidgetSnapshot: Codable, Hashable, Sendable {
    public static let currentFormat = "tt-widget-snapshot/v1"

    public var format: String
    public var workspace: String
    public var generatedAt: Date
    public var day: String
    public var validUntil: Date
    public var today: WidgetList
    public var inbox: WidgetList

    enum CodingKeys: String, CodingKey {
        case format, workspace, day, today, inbox
        case generatedAt = "generated_at"
        case validUntil = "valid_until"
    }

    public init(workspace: String, generatedAt: Date, day: String, validUntil: Date, today: WidgetList, inbox: WidgetList) {
        format = Self.currentFormat
        self.workspace = workspace
        self.generatedAt = generatedAt
        self.day = day
        self.validUntil = validUntil
        self.today = today
        self.inbox = inbox
    }

    /// Reads the engine's answer, whose instants are RFC 3339 with or
    /// without fractional seconds.
    public static func decode(from data: Data) throws -> WidgetSnapshot {
        try CaptureEnvelope.decoder.decode(WidgetSnapshot.self, from: data)
    }

    public func list(_ view: WidgetView) -> WidgetList {
        switch view {
        case .today: today
        case .inbox: inbox
        }
    }
}

/// How much a widget may trust its snapshot at a moment.
public enum WidgetFreshness: Hashable, Sendable {
    /// Written recently: the rows are what the app last knew, shown as is.
    case current
    /// Older than the stale threshold: still the right day, but the widget
    /// says how old it is, since another device may have changed it since.
    case stale
    /// The day it describes is over: Today and every overdue mark are about
    /// yesterday, so the rows are not presented as current.
    case expired
    /// No snapshot yet: the app has not opened a workspace.
    case missing
}

/// What a widget shows at one moment: the snapshot, how fresh it is, and how
/// many captures wait to be imported.
public struct WidgetState: Hashable, Sendable {
    public static let staleAfter: TimeInterval = 3 * 60 * 60

    public var snapshot: WidgetSnapshot?
    public var freshness: WidgetFreshness
    public var pendingCaptures: Int
    public var date: Date

    public init(snapshot: WidgetSnapshot?, pendingCaptures: Int, at date: Date) {
        self.snapshot = snapshot
        self.pendingCaptures = pendingCaptures
        self.date = date
        freshness = Self.freshness(of: snapshot, at: date)
    }

    public static func freshness(of snapshot: WidgetSnapshot?, at date: Date) -> WidgetFreshness {
        guard let snapshot else { return .missing }
        if date >= snapshot.validUntil { return .expired }
        if date.timeIntervalSince(snapshot.generatedAt) >= staleAfter { return .stale }
        return .current
    }

    /// The rows to draw: none once the day they describe is over.
    public func rows(_ view: WidgetView) -> [WidgetTask] {
        guard let snapshot, freshness != .expired, freshness != .missing else { return [] }
        return snapshot.list(view).tasks
    }

    /// The count to draw, or nil when it would describe another day.
    public func count(_ view: WidgetView) -> Int? {
        guard let snapshot, freshness != .expired, freshness != .missing else { return nil }
        return snapshot.list(view).count
    }

    /// One line saying how far the widget can be trusted.
    public var freshnessLine: String {
        switch freshness {
        case .missing:
            return "Open tt to set up"
        case .expired:
            return "Out of date · open tt"
        case .current, .stale:
            guard let snapshot else { return "" }
            let age = max(0, date.timeIntervalSince(snapshot.generatedAt))
            return "Updated " + Self.describe(age: age) + (freshness == .stale ? " · may be out of date" : "")
        }
    }

    static func describe(age: TimeInterval) -> String {
        switch age {
        case ..<60: "just now"
        case ..<3600: "\(Int(age / 60))m ago"
        case ..<86400: "\(Int(age / 3600))h ago"
        default: "\(Int(age / 86400))d ago"
        }
    }

    /// The moments after `date` at which what a widget shows changes on its
    /// own: when the snapshot turns stale and when its day ends.
    public static func transitions(of snapshot: WidgetSnapshot?, after date: Date) -> [Date] {
        guard let snapshot else { return [] }
        return [snapshot.generatedAt.addingTimeInterval(staleAfter), snapshot.validUntil]
            .filter { $0 > date }
            .sorted()
    }
}

/// The snapshot file in the App Group container. The app writes it after
/// every change it observes; widgets only read it.
public struct WidgetSnapshotStore: Sendable {
    public let file: URL

    public init(container: URL) {
        file = container.appendingPathComponent("widget/snapshot.json")
    }

    public func read() -> WidgetSnapshot? {
        guard let data = try? Data(contentsOf: file),
              let snapshot = try? CaptureEnvelope.decoder.decode(WidgetSnapshot.self, from: data),
              snapshot.format == WidgetSnapshot.currentFormat
        else { return nil }
        return snapshot
    }

    /// Replaces the snapshot. Writing what is already there is skipped,
    /// which keeps widget reloads to real changes.
    @discardableResult
    public func write(_ snapshot: WidgetSnapshot) throws -> Bool {
        try FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        let data = try CaptureEnvelope.encoder.encode(snapshot)
        if let existing = try? Data(contentsOf: file), existing == data { return false }
        try CaptureInbox.writeSynced(data, to: file)
        return true
    }

    /// Removes the snapshot, for a device that closed its workspace.
    public func clear() {
        try? FileManager.default.removeItem(at: file)
    }
}
