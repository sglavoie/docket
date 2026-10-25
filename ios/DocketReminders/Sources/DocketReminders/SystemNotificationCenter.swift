#if canImport(UserNotifications)
import Foundation
import UserNotifications

/// The system notification center behind the reminders' protocol.
public final class SystemNotificationCenter: ReminderNotificationCenter, @unchecked Sendable {
    private let center: UNUserNotificationCenter

    public init(center: UNUserNotificationCenter = .current()) {
        self.center = center
    }

    public func permission() async -> NotificationPermission {
        Self.permission(await center.notificationSettings().authorizationStatus)
    }

    public func requestPermission() async -> NotificationPermission {
        // A denial, or a failure to ask, is reported by the status that
        // follows; neither is an error a caller has to handle.
        _ = try? await center.requestAuthorization(options: [.alert, .sound, .badge])
        return await permission()
    }

    public func setCategories(_ categories: Set<ReminderCategory>) async {
        center.setNotificationCategories(Set(categories.map { category in
            UNNotificationCategory(
                identifier: category.identifier,
                actions: category.actions.map { action in
                    var options: UNNotificationActionOptions = [.authenticationRequired]
                    if action.opensApp { options.insert(.foreground) }
                    return UNNotificationAction(identifier: action.identifier, title: action.title, options: options)
                },
                intentIdentifiers: [],
                options: [.customDismissAction]
            )
        }))
    }

    public func pending() async -> [HeldNotification] {
        await center.pendingNotificationRequests().map { request in
            HeldNotification(identifier: request.identifier, userInfo: Self.strings(request.content.userInfo))
        }
    }

    public func add(_ request: ReminderRequest) async throws {
        let content = UNMutableNotificationContent()
        content.title = request.title
        content.subtitle = request.subtitle
        content.body = request.body
        content.sound = .default
        content.categoryIdentifier = request.categoryIdentifier
        content.threadIdentifier = request.threadIdentifier
        content.userInfo = request.userInfo
        let trigger = UNCalendarNotificationTrigger(dateMatching: request.trigger, repeats: false)
        try await center.add(UNNotificationRequest(identifier: request.identifier, content: content, trigger: trigger))
    }

    public func removePending(_ identifiers: [String]) async {
        center.removePendingNotificationRequests(withIdentifiers: identifiers)
    }

    public func removeDelivered(_ identifiers: [String]) async {
        center.removeDeliveredNotifications(withIdentifiers: identifiers)
    }

    /// A response's userInfo in the string form the router reads.
    public static func strings(_ userInfo: [AnyHashable: Any]) -> [String: String] {
        var strings: [String: String] = [:]
        for (key, value) in userInfo {
            if let key = key as? String, let value = value as? String {
                strings[key] = value
            }
        }
        return strings
    }

    static func permission(_ status: UNAuthorizationStatus) -> NotificationPermission {
        switch status {
        case .authorized: .authorized
        case .denied: .denied
        case .provisional: .provisional
        case .ephemeral: .ephemeral
        case .notDetermined: .notDetermined
        @unknown default: .denied
        }
    }
}
#endif
