import AppKit
import UserNotifications

/// Whether tt.app shows the work tt reports arriving: a check-in whose day
/// comes, a timed deadline that passes, or a deferral that ends. Persisted in
/// `UserDefaults` key `ttNotifications`; absent reads as on. A tt with a
/// control socket reports arrivals whatever this says and tt.app decides, so
/// both directions apply at once; an older tt is started with `--notify` only
/// when this is on, so turning it on reaches that tt the next time it starts.
enum TTNotificationsPreference {
    static let defaultsKey = "ttNotifications"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }

    static let flag = "--notify"

    /// What tt is started with: the flag only when enabled and the binary
    /// knows it.
    static func arguments(enabled: Bool, supported: Bool = true) -> [String] {
        enabled && supported ? [flag] : []
    }
}

/// Whether the Dock icon carries tt's overdue count. Persisted in
/// `UserDefaults` key `ttDockBadge`; absent reads as on.
enum TTDockBadgePreference {
    static let defaultsKey = "ttDockBadge"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }
}

/// tt's window title is its workspace pulse: `tt`, then `inbox N`, `today N`,
/// and `overdue N` for each nonzero count, joined by ` · `. The spelling is
/// `pulse.WindowTitle` in the Go tree; this reads it back.
enum TTWindowTitle {
    static let separator = " · "

    /// The overdue count a tt title names, or nil when it names none: zero
    /// overdue, or a title some other program set.
    static func overdueCount(in title: String) -> Int? {
        let parts = title.components(separatedBy: separator)
        guard parts.first == "tt" else { return nil }
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

/// The Dock badge for a pulse tt reported over its control socket: the
/// overdue count, or none at zero.
enum TTPulseBadge {
    static func label(for pulse: TTPulse) -> String? {
        pulse.overdue > 0 ? String(pulse.overdue) : nil
    }
}

/// The status menu's line for a pulse: the counts still owed, zero ones left
/// out, or none when nothing is.
enum TTPulseSummary {
    static func line(for pulse: TTPulse) -> String? {
        let parts = [("inbox", pulse.inbox), ("today", pulse.today), ("overdue", pulse.overdue)]
            .filter { $0.1 > 0 }
            .map { "\($0.0) \($0.1)" }
        return parts.isEmpty ? nil : parts.joined(separator: " · ")
    }
}

/// Shows tt's notifications through the Notification Center. Authorization is
/// asked for on the first one, not at launch, so a user who never gets one is
/// never prompted. A notification arriving while tt is already in front is not
/// presented, since tt's own rows say the same thing.
///
/// Two sources feed it. A tt with a control socket reports arrivals with each
/// task's identity, so a notification naming one task offers Acknowledge (when
/// tt would accept it) and Defer 1 Hour, and clicking it shows that task. An
/// older tt run with `--notify` sends one OSC 777 escape per batch, which can
/// only bring tt forward.
final class NotificationPresenter: NSObject, UNUserNotificationCenterDelegate {
    /// Whether to show one at all; read per notification.
    var isEnabled: () -> Bool = { true }
    /// Whether tt is the window in front.
    var isTerminalFrontmost: () -> Bool = { false }
    /// Brings tt forward; run when a notification naming no task is clicked.
    var onActivate: (() -> Void)?
    /// Run when a notification naming a task is clicked, or one of its actions chosen.
    var onReveal: ((TTTaskID) -> Void)?
    var onAcknowledge: ((TTTaskID) -> Void)?
    var onDefer: ((TTTaskID) -> Void)?

    /// More arrivals than this in one batch are worded as one summary.
    static let maxPerTaskNotifications = 3
    static let deferMinutes = 60

    enum Category {
        static let reviewable = "tt.arrival.reviewable"
        static let task = "tt.arrival"
    }

    enum Action {
        static let acknowledge = "tt.acknowledge"
        static let deferHour = "tt.defer"
    }

    enum UserInfo {
        static let project = "project"
        static let task = "task"
    }

    static let threadIdentifier = "tt.arrivals"

    private lazy var center: UNUserNotificationCenter = {
        let center = UNUserNotificationCenter.current()
        center.delegate = self
        center.setNotificationCategories(Self.categories())
        return center
    }()
    private var authorizationRequested = false

    static func categories() -> Set<UNNotificationCategory> {
        let acknowledge = UNNotificationAction(identifier: Action.acknowledge, title: "Acknowledge", options: [])
        let deferHour = UNNotificationAction(identifier: Action.deferHour, title: "Defer 1 Hour", options: [])
        return [
            UNNotificationCategory(identifier: Category.reviewable, actions: [acknowledge, deferHour], intentIdentifiers: []),
            UNNotificationCategory(identifier: Category.task, actions: [deferHour], intentIdentifiers: []),
        ]
    }

    /// The notifications one arrivals event becomes: one per task while there
    /// are few, else tt's own summary of the batch.
    static func contents(for arrivals: [TTArrival], title: String, body: String) -> [UNNotificationContent] {
        guard !arrivals.isEmpty else { return [] }
        guard arrivals.count <= maxPerTaskNotifications else {
            return [content(title: title, body: body)]
        }
        return arrivals.map { arrival in
            let content = content(title: arrival.notificationTitle, body: arrival.title)
            content.categoryIdentifier = arrival.reviewable ? Category.reviewable : Category.task
            content.userInfo = [UserInfo.project: arrival.project, UserInfo.task: arrival.task]
            return content
        }
    }

    private static func content(title: String, body: String) -> UNMutableNotificationContent {
        let content = UNMutableNotificationContent()
        content.title = title.isEmpty ? "tt" : title
        content.body = body
        content.sound = .default
        content.threadIdentifier = threadIdentifier
        return content
    }

    static func taskID(from userInfo: [AnyHashable: Any]) -> TTTaskID? {
        guard let project = userInfo[UserInfo.project] as? String,
              let task = userInfo[UserInfo.task] as? String else { return nil }
        return TTTaskID(project: project, task: task)
    }

    /// An OSC 777 notification from a `--notify` tt.
    func post(title: String, body: String) {
        guard !body.isEmpty || !title.isEmpty else { return }
        post([Self.content(title: title, body: body)])
    }

    /// An arrivals event from the control socket.
    func post(arrivals: [TTArrival], title: String, body: String) {
        post(Self.contents(for: arrivals, title: title, body: body))
    }

    /// A one-off message, such as an action tt refused.
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
                    if let error { NSLog("tt: notification: %@", error.localizedDescription) }
                }
            }
        }
        if authorizationRequested {
            deliver()
            return
        }
        authorizationRequested = true
        center.requestAuthorization(options: [.alert, .sound]) { granted, error in
            if let error { NSLog("tt: notification authorization: %@", error.localizedDescription) }
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

    func respond(to action: String, task: TTTaskID?) {
        guard let task else {
            if action == UNNotificationDefaultActionIdentifier { onActivate?() }
            return
        }
        switch action {
        case Action.acknowledge: onAcknowledge?(task)
        case Action.deferHour: onDefer?(task)
        case UNNotificationDefaultActionIdentifier: onReveal?(task)
        default: break
        }
    }
}

extension TTArrival {
    /// The heading tt's own `--notify` gives one arrival of this kind.
    var notificationTitle: String {
        switch kind {
        case .checkIn: return "tt · check-in"
        case .deadline: return "tt · deadline passed"
        case .wake: return "tt · deferral ended"
        }
    }
}
