import AppKit
import UserNotifications

/// Whether tt runs with `--notify`, so it reports arrived check-ins, passed
/// timed deadlines, and ended deferrals as system notifications. Persisted in
/// `UserDefaults` key `ttNotifications`; absent reads as on. tt reads its flags
/// when it starts, so turning this on reaches the next tt, while turning it off
/// also silences the running one at once.
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

/// Shows tt's desktop notifications (OSC 777, forwarded by Ghostty) through
/// the Notification Center. Authorization is asked for on the first one, not
/// at launch, so a user who never enables `--notify` is never prompted. A
/// notification arriving while tt is already in front is not presented, since
/// tt's own rows say the same thing; clicking one brings tt forward.
final class NotificationPresenter: NSObject, UNUserNotificationCenterDelegate {
    /// Whether to show one at all; read per notification.
    var isEnabled: () -> Bool = { true }
    /// Whether tt is the window in front.
    var isTerminalFrontmost: () -> Bool = { false }
    /// Brings tt forward; run when a notification is clicked.
    var onActivate: (() -> Void)?

    private lazy var center: UNUserNotificationCenter = {
        let center = UNUserNotificationCenter.current()
        center.delegate = self
        return center
    }()
    private var authorizationRequested = false

    func post(title: String, body: String) {
        guard isEnabled(), !body.isEmpty || !title.isEmpty else { return }
        let content = UNMutableNotificationContent()
        content.title = title.isEmpty ? "tt" : title
        content.body = body
        content.sound = .default
        let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
        let center = self.center
        let deliver = {
            center.add(request) { error in
                if let error { NSLog("tt: notification: %@", error.localizedDescription) }
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
        DispatchQueue.main.async { [weak self] in
            self?.onActivate?()
            completionHandler()
        }
    }
}
