import Foundation
import UserNotifications

/// Docket.app's two daily notices. Each is posted at most once a day, at the
/// first minute's read at or after its time, with what `docket ls` says then,
/// and is silent when there is nothing to say. A time that passed while the
/// Mac slept or Docket.app was not running is caught up at the next read that
/// day, never for a day that is over. Choosing one shows Docket on Today.
enum DocketNotice: String, CaseIterable {
    /// In the morning, when nothing is picked for today.
    case plan
    /// In the afternoon, while Today still holds open picks.
    case checkIn

    var title: String {
        switch self {
        case .plan: return "Plan the day"
        case .checkIn: return "Check in"
        }
    }

    /// What `docket ls` reads for it, in the order `body` takes the answers.
    var filters: [String] {
        switch self {
        case .plan: return DocketNoticePlan.filters
        case .checkIn: return ["$today"]
        }
    }

    /// What the notice says, from the listings `filters` asked for; nil when
    /// it has nothing to say, or the listings are not the ones it asked for.
    func body(_ answers: [[DocketRow]]) -> String? {
        guard answers.count == filters.count else { return nil }
        switch self {
        case .plan:
            return DocketNoticePlan.body(picked: answers[0], carried: answers[1], dueToday: answers[2], overdue: answers[3])
        case .checkIn:
            return DocketNoticeCheckIn.body(today: answers[0])
        }
    }

    /// Settings › General's switch, its time, and the day it last fired.
    var enabledKey: String { self == .plan ? "docketNoticePlan" : "docketNoticeCheckIn" }
    var timeKey: String { enabledKey + "Time" }
    var firedKey: String { enabledKey + "Fired" }

    var defaultTime: DocketNoticeTime {
        self == .plan ? DocketNoticeTime(hour: 9, minute: 0) : DocketNoticeTime(hour: 14, minute: 0)
    }

    /// The identifier a posted notice carries: one a day each, so a second
    /// would replace the first rather than stack.
    var identifier: String { "io.github.sglavoie.docket.notice." + rawValue }

    init?(identifier: String) {
        guard let notice = Self.allCases.first(where: { $0.identifier == identifier }) else { return nil }
        self = notice
    }

    /// What choosing a notice types into Docket: Go to Today.
    static let place = "today"

    /// Whether `notice` is due at `now`: on, its time reached on this day,
    /// and not yet fired on it. A day that is over is never caught up, since
    /// only the day `now` falls on is ever asked about.
    static func isDue(
        enabled: Bool, time: DocketNoticeTime, lastFired: String?, now: Date, calendar: Calendar = .current
    ) -> Bool {
        guard enabled else { return false }
        let day = DocketAgendaSnapshot.dayString(now, calendar: calendar)
        guard lastFired != day else { return false }
        let c = calendar.dateComponents([.hour, .minute], from: now)
        return (c.hour ?? 0) * 60 + (c.minute ?? 0) >= time.minutes
    }
}

/// Plan the day: said when nothing is picked for today, that is when
/// `docket ls '$today -$missed'` prints nothing, with what applies of what is
/// due today, overdue and carried. With a day already picked it says only
/// what is due or overdue outside Today, and is silent when nothing is.
enum DocketNoticePlan {
    static let filters = [
        "$today -$missed",
        "$missed",
        "@today $open|doing|waiting -$today",
        "@overdue -$today",
    ]

    static func body(picked: [DocketRow], carried: [DocketRow], dueToday: [DocketRow], overdue: [DocketRow]) -> String? {
        var parts: [String] = []
        if !dueToday.isEmpty { parts.append("\(dueToday.count) due today") }
        if !overdue.isEmpty { parts.append("\(overdue.count) overdue") }
        if picked.isEmpty {
            if !carried.isEmpty { parts.append("\(carried.count) carried") }
            return (["Nothing picked for Today"] + parts).joined(separator: " · ")
        }
        return parts.isEmpty ? nil : parts.joined(separator: " · ")
    }
}

/// Check in: said while Today still holds picks not closed, as `3 of 5 picks
/// still open`; silent with none open.
enum DocketNoticeCheckIn {
    static func body(today: [DocketRow]) -> String? {
        let open = today.filter { !$0.closed }.count
        guard open > 0 else { return nil }
        return "\(open) of \(today.count) \(today.count == 1 ? "pick" : "picks") still open"
    }
}

/// A notice's time of day, stored as `HH:mm`.
struct DocketNoticeTime: Equatable {
    let hour: Int
    let minute: Int

    init(hour: Int, minute: Int) {
        self.hour = min(max(hour, 0), 23)
        self.minute = min(max(minute, 0), 59)
    }

    /// `HH:mm`; nil for anything else.
    init?(_ text: String) {
        let parts = text.split(separator: ":", omittingEmptySubsequences: false)
        guard parts.count == 2, parts.allSatisfy({ $0.count == 2 }),
              let hour = Int(parts[0]), let minute = Int(parts[1]),
              (0...23).contains(hour), (0...59).contains(minute) else { return nil }
        self.init(hour: hour, minute: minute)
    }

    var minutes: Int { hour * 60 + minute }
    var text: String { String(format: "%02d:%02d", hour, minute) }

    /// The time on the day `date` falls on, for a time picker.
    func date(on date: Date, calendar: Calendar = .current) -> Date {
        calendar.date(bySettingHour: hour, minute: minute, second: 0, of: date) ?? date
    }

    init(_ date: Date, calendar: Calendar = .current) {
        let c = calendar.dateComponents([.hour, .minute], from: date)
        self.init(hour: c.hour ?? 0, minute: c.minute ?? 0)
    }
}

/// A notice's switch and time in Settings › General. Absent, the switch reads
/// as on and the time as the notice's default.
struct DocketNoticePreference: Equatable {
    var enabled: Bool
    var time: DocketNoticeTime

    static func load(_ notice: DocketNotice, from defaults: UserDefaults = .standard) -> DocketNoticePreference {
        DocketNoticePreference(
            enabled: defaults.object(forKey: notice.enabledKey) as? Bool ?? true,
            time: defaults.string(forKey: notice.timeKey).flatMap(DocketNoticeTime.init) ?? notice.defaultTime
        )
    }

    func save(_ notice: DocketNotice, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: notice.enabledKey)
        defaults.set(time.text, forKey: notice.timeKey)
    }
}

/// Whether macOS lets Docket.app post: asked the first time a notice has
/// something to say, not at launch.
enum DocketNoticePermission: Equatable {
    case notAsked, allowed, refused
}

/// Checks once a minute whether a notice is due, and only then reads what it
/// needs with `docket ls`; switched off, or already fired today, it runs
/// nothing. Each notice is marked fired for the day once its read succeeded,
/// whether it had something to say or not, so neither is posted twice.
final class DocketNoticeMonitor {
    var target: () -> DocketCommandTarget? = { nil }
    var preference: (DocketNotice) -> DocketNoticePreference = { DocketNoticePreference.load($0) }
    var defaults: UserDefaults = .standard
    var now: () -> Date = Date.init
    var calendar: Calendar = .current
    var read: (DocketCommandTarget, [String], @escaping ([[DocketRow]]?) -> Void) -> Void = { target, filters, done in
        DocketListing.read(target, filters: filters, completion: done)
    }
    /// Asks macOS, the first time only, and answers whether posting is allowed.
    var authorize: (@escaping (DocketNoticePermission) -> Void) -> Void = DocketNoticeCenter.authorize
    var post: (DocketNotice, String) -> Void = DocketNoticeCenter.post
    /// Told what macOS answered, for Settings to say.
    var onPermission: (DocketNoticePermission) -> Void = { _ in }

    private var timer: Timer?
    private var running = false

    func start(interval: TimeInterval = 60) {
        timer?.invalidate()
        let timer = Timer(timeInterval: interval, repeats: true) { [weak self] _ in self?.refresh() }
        timer.tolerance = 5
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
        refresh()
    }

    func stop() {
        timer?.invalidate()
        timer = nil
    }

    func lastFired(_ notice: DocketNotice) -> String? { defaults.string(forKey: notice.firedKey) }

    func isDue(_ notice: DocketNotice) -> Bool {
        let preference = preference(notice)
        return DocketNotice.isDue(
            enabled: preference.enabled, time: preference.time, lastFired: lastFired(notice), now: now(), calendar: calendar
        )
    }

    /// Reads for the first notice due, if any; one read at a time, the next
    /// one due waits for the next minute.
    func refresh() {
        guard !running, let notice = DocketNotice.allCases.first(where: isDue), let target = target() else { return }
        running = true
        let day = DocketAgendaSnapshot.dayString(now(), calendar: calendar)
        read(target, notice.filters) { [weak self] answers in
            guard let self else { return }
            self.running = false
            // A read that failed is tried again next minute; one that ended
            // on another day describes a day that is over.
            guard let answers, DocketAgendaSnapshot.dayString(self.now(), calendar: self.calendar) == day,
                  self.isDue(notice) else { return }
            self.defaults.set(day, forKey: notice.firedKey)
            if let body = notice.body(answers) {
                self.authorize { [weak self] permission in
                    self?.onPermission(permission)
                    if permission == .allowed { self?.post(notice, body) }
                }
            }
            if DocketNotice.allCases.contains(where: self.isDue) { self.refresh() }
        }
    }
}

/// `UNUserNotificationCenter`, as the monitor and Settings use it.
enum DocketNoticeCenter {
    static func authorize(_ done: @escaping (DocketNoticePermission) -> Void) {
        let center = UNUserNotificationCenter.current()
        center.requestAuthorization(options: [.alert, .sound]) { granted, _ in
            DispatchQueue.main.async { done(granted ? .allowed : .refused) }
        }
    }

    /// What macOS has said, without asking.
    static func permission(_ done: @escaping (DocketNoticePermission) -> Void) {
        UNUserNotificationCenter.current().getNotificationSettings { settings in
            let permission: DocketNoticePermission
            switch settings.authorizationStatus {
            case .notDetermined: permission = .notAsked
            case .denied: permission = .refused
            default: permission = .allowed
            }
            DispatchQueue.main.async { done(permission) }
        }
    }

    static func post(_ notice: DocketNotice, body: String) {
        let content = UNMutableNotificationContent()
        content.title = notice.title
        content.body = body
        content.sound = .default
        let request = UNNotificationRequest(identifier: notice.identifier, content: content, trigger: nil)
        UNUserNotificationCenter.current().add(request) { error in
            if let error { NSLog("Docket: notice: %@", error.localizedDescription) }
        }
    }
}
