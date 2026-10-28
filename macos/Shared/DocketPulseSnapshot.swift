import Foundation

/// The pulse as Docket.app last heard it, kept in the App Group container so the
/// widget can draw it while Docket.app is not running. It is a cache: `updatedAt`
/// is when it was true, and `day` the local day it describes, after which Today,
/// overdue, and done describe a day that is over and the widget stops presenting
/// them as current. `agenda` names the Today and Overdue tasks when the Docket
/// that reported the pulse also reported them, and is nil otherwise.
struct DocketPulseSnapshot: Codable, Equatable {
    let inbox: Int
    let today: Int
    let overdue: Int
    let done: Int
    let updatedAt: Date
    let day: String
    let agenda: DocketAgenda?

    /// The widget's kind, which Docket.app names to reload its timeline.
    static let widgetKind = "docket.pulse"
    /// The agenda control's kind, reloaded beside the widget.
    static let agendaControlKind = "docket.control.agenda"

    init(inbox: Int, today: Int, overdue: Int, done: Int, updatedAt: Date, agenda: DocketAgenda? = nil, calendar: Calendar = .current) {
        self.inbox = inbox
        self.today = today
        self.overdue = overdue
        self.done = done
        self.updatedAt = updatedAt
        self.agenda = agenda
        day = Self.day(of: updatedAt, calendar: calendar)
    }

    /// The local day `date` falls on, as `yyyy-MM-dd`.
    static func day(of date: Date, calendar: Calendar = .current) -> String {
        let parts = calendar.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", parts.year ?? 0, parts.month ?? 0, parts.day ?? 0)
    }

    /// Whether the counts still describe the day `date` falls on.
    func isCurrent(at date: Date, calendar: Calendar = .current) -> Bool {
        day == Self.day(of: date, calendar: calendar)
    }

    /// Where the widget's one tap goes: Due while anything is overdue, else Today.
    var primaryLink: DocketLink { overdue > 0 ? .view(.due) : .view(.today) }
}

/// The App Group Docket.app and its widget share. Team-prefixed, so a Developer ID
/// or Apple Development signature from that team owns it without a provisioning
/// profile; both entitlements files name the same group.
enum DocketSharedContainer {
    static let appGroup = "MH33GYKDAW.io.github.sglavoie.docket"

    static func root() -> URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: appGroup)
    }
}

struct DocketPulseSnapshotStore {
    let url: URL

    init(url: URL) {
        self.url = url
    }

    init?(container: URL?) {
        guard let container else { return nil }
        url = container.appendingPathComponent("pulse.json")
    }

    /// Nil when there is none or it cannot be read: the widget then says to open Docket.
    func read() -> DocketPulseSnapshot? {
        guard let data = try? Data(contentsOf: url) else { return nil }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try? decoder.decode(DocketPulseSnapshot.self, from: data)
    }

    func write(_ snapshot: DocketPulseSnapshot) throws {
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        try encoder.encode(snapshot).write(to: url, options: [.atomic])
    }
}

/// What the agenda control says and where it goes, read from the snapshot.
struct AgendaControlValue: Equatable {
    let overdue: Int
    let today: Int
    /// False when there is no snapshot for today, so the counts are not shown.
    let known: Bool

    /// Due while anything is overdue, else Today, as the small widget's tap; with
    /// no counts for today, Docket itself.
    var link: DocketLink {
        guard known else { return .show }
        return overdue > 0 ? .view(.due) : .view(.today)
    }

    var title: String {
        guard known else { return "Docket" }
        switch (overdue, today) {
        case (0, 0): return "Nothing due"
        case (0, let today): return "\(today) today"
        case (let overdue, 0): return "\(overdue) overdue"
        case (let overdue, let today): return "\(overdue) overdue · \(today) today"
        }
    }

    init(overdue: Int, today: Int, known: Bool = true) {
        self.overdue = overdue
        self.today = today
        self.known = known
    }

    /// The counts `snapshot` holds while they describe `now`'s day.
    init(snapshot: DocketPulseSnapshot?, now: Date = Date(), calendar: Calendar = .current) {
        guard let snapshot, snapshot.isCurrent(at: now, calendar: calendar) else {
            self.init(overdue: 0, today: 0, known: false)
            return
        }
        self.init(overdue: snapshot.overdue, today: snapshot.today)
    }
}
