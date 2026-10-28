import AppKit
import UserNotifications

/// Whether Docket.app shows the work Docket reports arriving: a check-in whose day
/// comes, a timed deadline that passes, or a deferral that ends. Persisted in
/// `UserDefaults` key `docketNotifications`; absent reads as on. A Docket with a
/// control socket reports arrivals whatever this says and Docket.app decides, so
/// both directions apply at once; an older Docket is started with `--notify` only
/// when this is on, so turning it on reaches that Docket the next time it starts.
enum DocketNotificationsPreference {
    static let defaultsKey = "docketNotifications"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }

    static let flag = "--notify"

    /// What Docket is started with: the flag only when enabled and the binary
    /// knows it.
    static func arguments(enabled: Bool, supported: Bool = true) -> [String] {
        enabled && supported ? [flag] : []
    }
}

/// Whether the Dock icon carries Docket's overdue count. Persisted in
/// `UserDefaults` key `docketDockBadge`; absent reads as on.
enum DocketDockBadgePreference {
    static let defaultsKey = "docketDockBadge"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }
}

/// Docket's window title is its workspace pulse: `docket`, then `inbox N`, `today N`,
/// and `overdue N` for each nonzero count, joined by ` · `. The spelling is
/// `pulse.WindowTitle` in the Go tree; this reads it back.
enum DocketWindowTitle {
    static let separator = " · "

    /// The overdue count a Docket title names, or nil when it names none: zero
    /// overdue, or a title some other program set.
    static func overdueCount(in title: String) -> Int? {
        let parts = title.components(separatedBy: separator)
        guard parts.first == "Docket" else { return nil }
        for part in parts.dropFirst() {
            let words = part.split(separator: " ")
            if words.count == 2, words[0] == "overdue", let count = Int(words[1]), count > 0 {
                return count
            }
        }
        return nil
    }

    /// The Dock badge for a title: the overdue count, or none.
    static func badgeLabel(for title: String) -> String? {
        overdueCount(in: title).map(String.init)
    }
}

/// The Dock badge for a pulse Docket reported over its control socket: the
/// overdue count, or none at zero.
enum DocketPulseBadge {
    static func label(for pulse: DocketPulse) -> String? {
        pulse.overdue > 0 ? String(pulse.overdue) : nil
    }
}

/// Where the overdue count goes: the Dock icon while there is one, else
/// beside the menu-bar icon, so menu-bar-only Docket still shows it. Neither
/// carries it while `docketDockBadge` is off.
struct DocketBadgePlacement: Equatable {
    let dock: String?
    let statusItem: String?

    init(dock: String?, statusItem: String?) {
        self.dock = dock
        self.statusItem = statusItem
    }

    init(policy: DocketActivationPolicy, enabled: Bool, label: String?) {
        let shown = enabled ? label : nil
        self.init(dock: policy.showsDockIcon ? shown : nil, statusItem: policy.showsDockIcon ? nil : shown)
    }
}

/// The status menu's line for a pulse: the counts still owed, zero ones left
/// out, or none when nothing is.
enum DocketPulseSummary {
    static func line(for pulse: DocketPulse) -> String? {
        let parts = [("inbox", pulse.inbox), ("today", pulse.today), ("overdue", pulse.overdue)]
            .filter { $0.1 > 0 }
            .map { "\($0.0) \($0.1)" }
        return parts.isEmpty ? nil : parts.joined(separator: " · ")
    }

    /// The status menu's items for a pulse: one per count still owed, in the
    /// line's order, each naming the mode that shows it. Overdue opens Due.
    static func items(for pulse: DocketPulse) -> [(title: String, view: DocketView)] {
        [("Inbox", pulse.inbox, DocketView.inbox), ("Today", pulse.today, .today), ("Overdue", pulse.overdue, .due)]
            .filter { $0.1 > 0 }
            .map { (title: "\($0.0) \($0.1)", view: $0.2) }
    }
}

/// The status and Dock menus' agenda: the tasks behind the Overdue and Today
/// counts, each showing Docket with the cursor on it, under a heading per kind,
/// then one item for whatever did not fit, showing the mode that lists it.
enum DocketAgendaMenu {
    /// A menu has room for this many tasks; Docket sends more than that.
    static let maxTasks = 8
    /// A longer title is cut here, so one task cannot widen the whole menu.
    static let maxTitleLength = 60

    enum Entry: Equatable {
        case heading(String)
        case task(DocketAgendaItem)
        case more(Int, DocketView)
    }

    /// What the menu lists for `agenda`, given the pulse its counts came with.
    static func entries(for agenda: DocketAgenda, pulse: DocketPulse?) -> [Entry] {
        let shown = Array(agenda.items.prefix(maxTasks))
        var entries: [Entry] = []
        for (kind, heading) in [(DocketAgendaItem.Kind.overdue, "Overdue"), (.today, "Today")] {
            let tasks = shown.filter { $0.kind == kind }
            guard !tasks.isEmpty else { continue }
            entries.append(.heading(heading))
            entries += tasks.map(Entry.task)
        }
        let remainder = agenda.total - shown.count
        if remainder > 0 {
            // Overdue tasks lead the agenda, so the ones left out are overdue
            // only while the shown ones fall short of the overdue count.
            let overdueShown = shown.filter { $0.kind == .overdue }.count
            let overdueTotal = pulse?.overdue ?? agenda.items.filter { $0.kind == .overdue }.count
            entries.append(.more(remainder, overdueShown < overdueTotal ? .due : .today))
        }
        return entries
    }

    /// The task's title, cut to `maxTitleLength`.
    static func title(_ item: DocketAgendaItem) -> String {
        item.title.count > maxTitleLength ? String(item.title.prefix(maxTitleLength - 1)) + "…" : item.title
    }

    /// Menu items for `agenda`. A task's item opens a submenu: Show in Docket,
    /// which carries its `DocketTaskID` as the represented object and runs
    /// `taskAction`, then the acts `DocketTaskAction.offered(for:)` admits, each
    /// carrying a `DocketTaskActionChoice` and running `actAction`. The remainder
    /// carries its mode's raw value and runs `viewAction`, as a count item does.
    static func items(for agenda: DocketAgenda?, pulse: DocketPulse?, now: Date = Date(), target: AnyObject,
                      taskAction: Selector, actAction: Selector, viewAction: Selector) -> [NSMenuItem] {
        guard let agenda else { return [] }
        return entries(for: agenda, pulse: pulse).map { entry in
            switch entry {
            case .heading(let title):
                return NSMenuItem.sectionHeader(title: title)
            case .task(let task):
                let item = NSMenuItem(title: title(task), action: nil, keyEquivalent: "")
                let detail = task.detail(now: now)
                if !detail.isEmpty {
                    let styled = NSMutableAttributedString(string: item.title)
                    styled.append(NSAttributedString(string: "  " + detail, attributes: [
                        .foregroundColor: task.kind == .overdue ? NSColor.systemRed : NSColor.secondaryLabelColor,
                        .font: NSFont.menuFont(ofSize: NSFont.smallSystemFontSize),
                    ]))
                    item.attributedTitle = styled
                }
                item.toolTip = detail.isEmpty ? task.title : task.title + " — " + detail
                item.submenu = submenu(for: task, target: target, taskAction: taskAction, actAction: actAction)
                return item
            case .more(let count, let view):
                let item = NSMenuItem(title: "\(count) more…", action: viewAction, keyEquivalent: "")
                item.target = target
                item.representedObject = view.rawValue
                return item
            }
        }
    }

    /// One agenda task's submenu: Show in Docket, then what may be done to it.
    static func submenu(for task: DocketAgendaItem, target: AnyObject, taskAction: Selector, actAction: Selector) -> NSMenu {
        let menu = NSMenu(title: task.title)
        menu.autoenablesItems = false
        let show = menu.addItem(withTitle: "Show in Docket", action: taskAction, keyEquivalent: "")
        show.target = target
        show.representedObject = task.id
        menu.addItem(.separator())
        for action in DocketTaskAction.offered(for: task) {
            let item = menu.addItem(withTitle: action.title, action: actAction, keyEquivalent: "")
            item.target = target
            item.representedObject = DocketTaskActionChoice(action: action, task: task.id)
            item.image = NSImage(systemSymbolName: action.systemImage, accessibilityDescription: nil)
        }
        return menu
    }
}

/// What an agenda submenu's act carries: the act and the task it is for.
final class DocketTaskActionChoice: NSObject {
    let action: DocketTaskAction
    let task: DocketTaskID

    init(action: DocketTaskAction, task: DocketTaskID) {
        self.action = action
        self.task = task
    }
}

/// Shows Docket's notifications through the Notification Center. Authorization is
/// asked for on the first one, not at launch, so a user who never gets one is
/// never prompted. A notification arriving while Docket is already in front is not
/// presented, since Docket's own rows say the same thing.
///
/// Three sources feed it. A Docket with a control socket reports its schedule,
/// the arrivals still to come, and each is booked with the system for its
/// instant, so it appears then whether or not any Docket is running; and it
/// reports arrivals as they happen, with each task's identity, which are shown
/// unless a booking already showed them. A notification naming one task offers
/// Acknowledge (when Docket would accept it), Done, and Defer 1 Hour, and
/// clicking it shows that task. An older Docket run with `--notify` sends one
/// OSC 777 escape per batch, which can only bring Docket forward.
final class NotificationPresenter: NSObject, UNUserNotificationCenterDelegate {
    /// Whether to show one at all; read per notification.
    var isEnabled: () -> Bool = { true }
    /// Whether Docket is the window in front.
    var isTerminalFrontmost: () -> Bool = { false }
    /// Brings Docket forward; run when a notification naming no task is clicked.
    var onActivate: (() -> Void)?
    /// Run when a notification naming a task is clicked, or one of its actions chosen.
    var onReveal: ((DocketTaskID) -> Void)?
    var onAcknowledge: ((DocketTaskID) -> Void)?
    var onDone: ((DocketTaskID) -> Void)?
    var onDefer: ((DocketTaskID) -> Void)?

    /// More arrivals than this in one batch are worded as one summary.
    static let maxPerTaskNotifications = 3
    static let deferMinutes = DocketTaskAction.deferMinutes

    enum Category {
        static let reviewable = "docket.arrival.reviewable"
        static let task = "docket.arrival"
    }

    enum Action {
        static let acknowledge = "docket.acknowledge"
        static let done = "docket.done"
        static let deferHour = "docket.defer"
    }

    enum UserInfo {
        static let project = "project"
        static let task = "task"
    }

    static let threadIdentifier = "docket.arrivals"

    private lazy var center: UNUserNotificationCenter = {
        let center = UNUserNotificationCenter.current()
        center.delegate = self
        center.setNotificationCategories(Self.categories())
        return center
    }()
    private var authorizationRequested = false
    private var authorizationWaiters: [(Bool) -> Void] = []

    /// The schedule Docket last reported, as bookings.
    private(set) var reminders = DocketReminderBook()
    /// Bookings a newer schedule left out because their instant came, kept
    /// while they can still cover a live arrival.
    private var firedBookings: [DocketReminderBook.Booking] = []
    /// Whether the pending bookings are being brought in step, and whether
    /// another pass is owed once it finishes; passes never overlap, so one
    /// cannot withdraw what a later one just booked.
    private var bookingInFlight = false
    private var bookingOwed = false

    /// Sets the center's delegate before anything else can, so a notification
    /// action that launched the app reaches it.
    func activate() {
        _ = center
    }

    static func categories() -> Set<UNNotificationCategory> {
        let acknowledge = UNNotificationAction(identifier: Action.acknowledge, title: "Acknowledge", options: [])
        let done = UNNotificationAction(identifier: Action.done, title: "Done", options: [])
        let deferHour = UNNotificationAction(identifier: Action.deferHour, title: "Defer 1 Hour", options: [])
        return [
            UNNotificationCategory(identifier: Category.reviewable, actions: [acknowledge, done, deferHour], intentIdentifiers: []),
            UNNotificationCategory(identifier: Category.task, actions: [done, deferHour], intentIdentifiers: []),
        ]
    }

    /// The notifications one arrivals event becomes: one per task while there
    /// are few, else Docket's own summary of the batch.
    static func contents(for arrivals: [DocketArrival], title: String, body: String) -> [UNNotificationContent] {
        guard !arrivals.isEmpty else { return [] }
        guard arrivals.count <= maxPerTaskNotifications else {
            return [content(title: title, body: body)]
        }
        return arrivals.map(content(for:))
    }

    /// One arrival's notification, naming its task.
    static func content(for arrival: DocketArrival) -> UNMutableNotificationContent {
        let content = content(title: arrival.notificationTitle, body: arrival.title)
        content.categoryIdentifier = arrival.reviewable ? Category.reviewable : Category.task
        content.userInfo = [UserInfo.project: arrival.project, UserInfo.task: arrival.task]
        return content
    }

    /// The summary Docket gives a batch, for a batch the bookings thinned: the
    /// arrivals counted by reason, in Docket's wording and order.
    static func summary(for arrivals: [DocketArrival]) -> (title: String, body: String) {
        let reasons: [(DocketArrival.Kind, String, String)] = [
            (.checkIn, "check-in", "check-ins"),
            (.deadline, "deadline passed", "deadlines passed"),
            (.wake, "deferral ended", "deferrals ended"),
        ]
        let parts = reasons.compactMap { kind, one, many -> String? in
            let count = arrivals.filter { $0.kind == kind }.count
            return count == 0 ? nil : "\(count) \(count == 1 ? one : many)"
        }
        return ("Docket", parts.joined(separator: " · "))
    }

    private static func content(title: String, body: String) -> UNMutableNotificationContent {
        let content = UNMutableNotificationContent()
        content.title = title.isEmpty ? "Docket" : title
        content.body = body
        content.sound = .default
        content.threadIdentifier = threadIdentifier
        return content
    }

    static func taskID(from userInfo: [AnyHashable: Any]) -> DocketTaskID? {
        guard let project = userInfo[UserInfo.project] as? String,
              let task = userInfo[UserInfo.task] as? String else { return nil }
        return DocketTaskID(project: project, task: task)
    }

    /// An OSC 777 notification from a `--notify` Docket.
    func post(title: String, body: String) {
        guard !body.isEmpty || !title.isEmpty else { return }
        post([Self.content(title: title, body: body)])
    }

    /// An arrivals event from the control socket, less what a booking
    /// already showed.
    func post(arrivals: [DocketArrival], title: String, body: String, now: Date = Date()) {
        post(Self.contents(for: arrivals, title: title, body: body, covered: reminders.bookings + firedBookings, now: now))
    }

    /// The notifications an arrivals event becomes once the arrivals a booking
    /// in `covered` already showed are left out; a thinned batch is summarized
    /// afresh, since Docket's own summary counts what was left out too.
    static func contents(
        for arrivals: [DocketArrival], title: String, body: String,
        covered: [DocketReminderBook.Booking], now: Date
    ) -> [UNNotificationContent] {
        let shown = arrivals.filter { !DocketReminderBook.covers($0, bookings: covered, now: now) }
        guard shown.count != arrivals.count else {
            return contents(for: arrivals, title: title, body: body)
        }
        let summary = summary(for: shown)
        return contents(for: shown, title: summary.title, body: summary.body)
    }

    /// A schedule event from the control socket: books what is to come,
    /// withdraws what no longer is, and keeps the bookings whose instant came
    /// able to cover the live arrival Docket reports for them.
    func book(_ schedule: [DocketScheduledArrival], now: Date = Date()) {
        let next = DocketReminderBook(schedule: schedule, now: now)
        let earliest = now.addingTimeInterval(-DocketReminderBook.coverAfter)
        let latest = now.addingTimeInterval(DocketReminderBook.coverBefore)
        let wanted = next.identifiers
        firedBookings = (firedBookings + reminders.bookings).filter {
            $0.at >= earliest && $0.at <= latest && !wanted.contains($0.identifier)
        }
        reminders = next
        refreshBookings()
    }

    /// Brings the pending bookings in step with the schedule, or withdraws them
    /// all while notifications are off. Withdrawing needs no permission, so
    /// bookings an earlier run left go even before anything is asked; permission
    /// is asked for the first time there is something to add.
    func refreshBookings() {
        guard !bookingInFlight else {
            bookingOwed = true
            return
        }
        bookingInFlight = true
        let book = isEnabled() ? reminders : DocketReminderBook()
        let center = self.center
        center.getPendingNotificationRequests { [weak self] pending in
            let changes = book.changes(pending: Set(pending.map(\.identifier)))
            if !changes.remove.isEmpty {
                center.removePendingNotificationRequests(withIdentifiers: changes.remove)
            }
            DispatchQueue.main.async {
                guard let self else { return }
                guard !changes.add.isEmpty else {
                    self.finishBooking()
                    return
                }
                self.authorize { granted in
                    if granted {
                        for booking in changes.add { Self.add(booking, to: center) }
                    }
                    self.finishBooking()
                }
            }
        }
    }

    private func finishBooking() {
        bookingInFlight = false
        if bookingOwed {
            bookingOwed = false
            refreshBookings()
        }
    }

    private static func add(_ booking: DocketReminderBook.Booking, to center: UNUserNotificationCenter) {
        let components = Calendar.current.dateComponents(
            [.calendar, .timeZone, .year, .month, .day, .hour, .minute, .second], from: booking.at
        )
        let trigger = UNCalendarNotificationTrigger(dateMatching: components, repeats: false)
        let request = UNNotificationRequest(identifier: booking.identifier, content: content(for: booking.arrival), trigger: trigger)
        center.add(request) { error in
            if let error { NSLog("Docket: reminder: %@", error.localizedDescription) }
        }
    }

    /// Asks for permission once, answering the callers waiting on that request
    /// with its outcome. Later callers go ahead and leave delivery to the
    /// system, so permission granted afterwards in System Settings applies.
    private func authorize(_ then: @escaping (Bool) -> Void) {
        if authorizationRequested, authorizationWaiters.isEmpty {
            then(true)
            return
        }
        authorizationWaiters.append(then)
        guard !authorizationRequested else { return }
        authorizationRequested = true
        center.requestAuthorization(options: [.alert, .sound]) { [weak self] granted, error in
            if let error { NSLog("Docket: notification authorization: %@", error.localizedDescription) }
            DispatchQueue.main.async { [weak self] in
                guard let self else { return }
                let waiting = self.authorizationWaiters
                self.authorizationWaiters = []
                for body in waiting { body(granted) }
            }
        }
    }

    /// A one-off message, such as an action Docket refused.
    func postNotice(title: String, body: String) {
        post([Self.content(title: title, body: body)])
    }

    private func post(_ contents: [UNNotificationContent]) {
        guard isEnabled(), !contents.isEmpty else { return }
        let center = self.center
        let deliver = {
            for content in contents {
                let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
                center.add(request) { error in
                    if let error { NSLog("Docket: notification: %@", error.localizedDescription) }
                }
            }
        }
        authorize { granted in
            if granted { deliver() }
        }
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        DispatchQueue.main.async { [weak self] in
            let inFront = self?.isTerminalFrontmost() ?? false
            completionHandler(inFront ? [] : [.banner, .list, .sound])
        }
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let task = Self.taskID(from: response.notification.request.content.userInfo)
        let action = response.actionIdentifier
        DispatchQueue.main.async { [weak self] in
            self?.respond(to: action, task: task)
            completionHandler()
        }
    }

    func respond(to action: String, task: DocketTaskID?) {
        guard let task else {
            if action == UNNotificationDefaultActionIdentifier { onActivate?() }
            return
        }
        switch action {
        case Action.acknowledge: onAcknowledge?(task)
        case Action.done: onDone?(task)
        case Action.deferHour: onDefer?(task)
        case UNNotificationDefaultActionIdentifier: onReveal?(task)
        default: break
        }
    }
}

extension DocketArrival {
    /// The heading Docket's own `--notify` gives one arrival of this kind.
    var notificationTitle: String {
        switch kind {
        case .checkIn: return "Docket · check-in"
        case .deadline: return "Docket · deadline passed"
        case .wake: return "Docket · deferral ended"
        }
    }
}
