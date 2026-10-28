import Foundation

// Compiled into Docket.app and its widget extension: the app hears the agenda
// over the control socket and keeps it in the pulse snapshot, and the widget
// draws it from there, so both read one shape and word a deadline alike.

/// The tasks behind the pulse's Today and Overdue counts, as Docket reports them:
/// overdue first, oldest deadline leading, then Today by priority. `items` holds
/// at most a bounded number of them; `total` is how many there are.
struct DocketAgenda: Codable, Equatable {
    let items: [DocketAgendaItem]
    let total: Int

    init(items: [DocketAgendaItem], total: Int) {
        self.items = items
        self.total = total
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        items = try c.decodeIfPresent([DocketAgendaItem].self, forKey: .items) ?? []
        total = try c.decodeIfPresent(Int.self, forKey: .total) ?? items.count
    }

    /// How many tasks `items` leaves out.
    var remainder: Int { max(0, total - items.count) }
}

/// One agenda task, named by its stored IDs as an arrival is.
struct DocketAgendaItem: Codable, Equatable {
    enum Kind: String, Codable {
        case overdue, today
    }

    let kind: Kind
    let project: String
    let task: String
    let title: String
    let projectName: String
    /// 1–3, or nil when the task has none.
    let priority: Int?
    /// RFC 3339, as Docket sent it; nil without a deadline.
    let due: String?
    /// Whether `due` names an instant rather than a whole day.
    let dueHasTime: Bool
    /// Whether Docket would accept acknowledging it now, as `R` would.
    let reviewable: Bool

    enum CodingKeys: String, CodingKey {
        case kind, project, task, title, priority, due, reviewable
        case projectName = "project_name"
        case dueHasTime = "due_has_time"
    }

    init(kind: Kind, project: String, task: String, title: String, projectName: String,
         priority: Int? = nil, due: String? = nil, dueHasTime: Bool = false, reviewable: Bool = false) {
        self.kind = kind
        self.project = project
        self.task = task
        self.title = title
        self.projectName = projectName
        self.priority = priority
        self.due = due
        self.dueHasTime = dueHasTime
        self.reviewable = reviewable
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        kind = try c.decode(Kind.self, forKey: .kind)
        project = try c.decode(String.self, forKey: .project)
        task = try c.decode(String.self, forKey: .task)
        title = try c.decode(String.self, forKey: .title)
        projectName = try c.decodeIfPresent(String.self, forKey: .projectName) ?? ""
        priority = try c.decodeIfPresent(Int.self, forKey: .priority)
        due = try c.decodeIfPresent(String.self, forKey: .due)
        dueHasTime = try c.decodeIfPresent(Bool.self, forKey: .dueHasTime) ?? false
        reviewable = try c.decodeIfPresent(Bool.self, forKey: .reviewable) ?? false
    }

    var id: DocketTaskID { DocketTaskID(project: project, task: task) }

    /// Where choosing it goes: Docket, with the cursor on this task.
    var link: DocketLink { .task(project: project, task: task) }

    var dueDate: Date? {
        guard let due else { return nil }
        return ISO8601DateFormatter().date(from: due)
    }

    /// The deadline as a short phrase relative to `now`: a time for today's
    /// timed deadline, else Yesterday, Today, Tomorrow, a weekday within the
    /// week, or a date, with the time when there is one. Nil without a deadline.
    func dueLabel(now: Date = Date(), calendar: Calendar = .current, locale: Locale = .current) -> String? {
        Self.dueLabel(dueDate, hasTime: dueHasTime, now: now, calendar: calendar, locale: locale)
    }

    /// `dueLabel(now:)` for any deadline Docket names, so a search result words
    /// one as the agenda does.
    static func dueLabel(
        _ date: Date?, hasTime dueHasTime: Bool, now: Date = Date(), calendar: Calendar = .current, locale: Locale = .current
    ) -> String? {
        guard let date else { return nil }
        let time = Date.FormatStyle(date: .omitted, time: .shortened, locale: locale, calendar: calendar, timeZone: calendar.timeZone)
        let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: now), to: calendar.startOfDay(for: date)).day ?? 0
        if days == 0, dueHasTime {
            return date.formatted(time)
        }
        let day: String
        switch days {
        case 0: day = "Today"
        case -1: day = "Yesterday"
        case 1: day = "Tomorrow"
        case 2...6:
            day = date.formatted(Date.FormatStyle(locale: locale, calendar: calendar, timeZone: calendar.timeZone).weekday(.wide))
        default:
            let sameYear = calendar.component(.year, from: date) == calendar.component(.year, from: now)
            var style = Date.FormatStyle(locale: locale, calendar: calendar, timeZone: calendar.timeZone).month(.abbreviated).day()
            if !sameYear { style = style.year() }
            day = date.formatted(style)
        }
        return dueHasTime ? day + " " + date.formatted(time) : day
    }

    /// The secondary line a menu or widget row shows: the project, then the
    /// deadline when there is one.
    func detail(now: Date = Date(), calendar: Calendar = .current, locale: Locale = .current) -> String {
        [projectName.isEmpty ? nil : projectName, dueLabel(now: now, calendar: calendar, locale: locale)]
            .compactMap { $0 }
            .joined(separator: " · ")
    }
}
