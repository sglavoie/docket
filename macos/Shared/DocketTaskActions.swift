import Foundation

// Compiled into Docket.app and its widget extension: what may be done to one
// agenda task from outside Docket's window, and how the widget, which cannot
// reach the control socket from its sandbox, hands such a request to the app.

/// One task, by the stored IDs in `tasks.md`.
struct DocketTaskID: Codable, Equatable, Hashable {
    let project: String
    let task: String
}

/// The acts the status menu, Shortcuts, and the widget offer on an agenda
/// task. Each is the one a notification's button makes, answered as its key in
/// Docket would be: Done as `wd`, Acknowledge as `R`, Defer as a deferral by
/// `deferMinutes` from now.
enum DocketTaskAction: String, Codable, CaseIterable {
    case done, deferHour = "defer_hour", acknowledge

    /// How long Defer 1 Hour defers.
    static let deferMinutes = 60

    var title: String {
        switch self {
        case .done: return "Done"
        case .deferHour: return "Defer 1 Hour"
        case .acknowledge: return "Acknowledge"
        }
    }

    var systemImage: String {
        switch self {
        case .done: return "checkmark.circle"
        case .deferHour: return "clock.arrow.circlepath"
        case .acknowledge: return "eye"
        }
    }

    /// What a refusal is headed with.
    var failure: String {
        switch self {
        case .done: return "Docket could not mark it Done"
        case .deferHour: return "Docket could not defer it"
        case .acknowledge: return "Docket could not acknowledge it"
        }
    }

    /// The acts `item` admits: Acknowledge only where Docket would accept it.
    static func offered(for item: DocketAgendaItem) -> [Self] {
        item.reviewable ? [.done, .deferHour, .acknowledge] : [.done, .deferHour]
    }
}

/// A task action the widget asked for, waiting in the App Group container for
/// Docket.app to carry it out. It holds for `lifetime` only: a request the app
/// never saw, because it was not running, is dropped rather than replayed
/// against a workspace that has moved on.
struct DocketWidgetActionRequest: Codable, Equatable {
    let id: UUID
    let action: DocketTaskAction
    let task: DocketTaskID
    let requestedAt: Date

    static let lifetime: TimeInterval = 60

    init(id: UUID = UUID(), action: DocketTaskAction, task: DocketTaskID, requestedAt: Date = Date()) {
        self.id = id
        self.action = action
        self.task = task
        self.requestedAt = requestedAt
    }

    var expiresAt: Date { requestedAt.addingTimeInterval(Self.lifetime) }

    func isLive(at date: Date) -> Bool {
        date >= requestedAt.addingTimeInterval(-Self.lifetime) && date < expiresAt
    }
}

/// The folder of pending widget requests, one JSON file each, and the Darwin
/// notification that says one was added. The widget writes and posts; the app
/// observes, takes each file, and acts on it.
struct DocketWidgetActionStore {
    /// Team-prefixed like the App Group, the form a sandboxed extension may post.
    static let notification = DocketSharedContainer.appGroup + ".widget-action"

    let directory: URL

    init(directory: URL) {
        self.directory = directory
    }

    init?(container: URL?) {
        guard let container else { return nil }
        directory = container.appendingPathComponent("widget-actions", isDirectory: true)
    }

    private static func encoder() -> JSONEncoder {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }

    private static func decoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }

    func add(_ request: DocketWidgetActionRequest) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try Self.encoder().encode(request)
            .write(to: directory.appendingPathComponent(request.id.uuidString + ".json"), options: [.atomic])
    }

    /// Every request on disk, oldest first; an unreadable file is skipped.
    func pending() -> [DocketWidgetActionRequest] {
        let files = (try? FileManager.default.contentsOfDirectory(at: directory, includingPropertiesForKeys: nil)) ?? []
        return files.filter { $0.pathExtension == "json" }
            .compactMap { try? Self.decoder().decode(DocketWidgetActionRequest.self, from: Data(contentsOf: $0)) }
            .sorted { $0.requestedAt < $1.requestedAt }
    }

    /// Removes and returns every request, live ones first in the order they
    /// were made, and discards the rest: an expired request and an unreadable
    /// file are removed unread. Taking is what keeps a request from running twice.
    func take(at date: Date = Date()) -> [DocketWidgetActionRequest] {
        let files = (try? FileManager.default.contentsOfDirectory(at: directory, includingPropertiesForKeys: nil)) ?? []
        var live: [DocketWidgetActionRequest] = []
        for file in files where file.pathExtension == "json" {
            let request = (try? Data(contentsOf: file)).flatMap { try? Self.decoder().decode(DocketWidgetActionRequest.self, from: $0) }
            // Removed before acting, so a second drain cannot take it again.
            guard (try? FileManager.default.removeItem(at: file)) != nil else { continue }
            if let request, request.isLive(at: date) { live.append(request) }
        }
        return live.sorted { $0.requestedAt < $1.requestedAt }
    }

    /// Asks Docket.app to look; it may not be running to hear it.
    static func post() {
        CFNotificationCenterPostNotification(
            CFNotificationCenterGetDarwinNotifyCenter(), CFNotificationName(notification as CFString), nil, nil, true
        )
    }
}

extension DocketPulseSnapshot {
    /// The snapshot as it will read once `pending` is carried out, so a task
    /// the widget just marked Done or deferred leaves its list at once rather
    /// than after Docket's next report. Done and Defer take a task off the
    /// agenda and out of its count, and Done adds one to Done; Acknowledge
    /// leaves it listed. A request for a task the agenda does not name changes
    /// nothing, so a count is never taken twice.
    func applying(_ pending: [DocketWidgetActionRequest], calendar: Calendar = .current) -> DocketPulseSnapshot {
        guard let agenda else { return self }
        let leaving = Dictionary(
            pending.filter { $0.action != .acknowledge }.map { ($0.task, $0.action) },
            uniquingKeysWith: { first, _ in first }
        )
        let gone = agenda.items.filter { leaving[$0.id] != nil }
        guard !gone.isEmpty else { return self }
        let overdueGone = gone.filter { $0.kind == .overdue }.count
        return DocketPulseSnapshot(
            inbox: inbox,
            today: max(0, today - (gone.count - overdueGone)),
            overdue: max(0, overdue - overdueGone),
            done: done + gone.filter { leaving[$0.id] == .done }.count,
            updatedAt: updatedAt,
            agenda: DocketAgenda(items: agenda.items.filter { leaving[$0.id] == nil }, total: max(0, agenda.total - gone.count)),
            calendar: calendar
        )
    }
}
