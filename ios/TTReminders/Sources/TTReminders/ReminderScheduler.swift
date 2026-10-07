import Foundation

/// Whether the system lets this app notify.
public enum NotificationPermission: String, Codable, Hashable, Sendable {
    case notDetermined
    case denied
    case authorized
    case provisional
    case ephemeral

    /// Whether a scheduled notification would be delivered.
    public var allowsDelivery: Bool {
        switch self {
        case .authorized, .provisional, .ephemeral: true
        case .notDetermined, .denied: false
        }
    }
}

/// One local notification as the scheduler asks for it. The trigger is a
/// calendar date with its zone named explicitly, so what the request means
/// never depends on the zone the device happens to be in when it fires.
public struct ReminderRequest: Hashable, Sendable {
    public var identifier: String
    public var title: String
    public var subtitle: String
    public var body: String
    public var categoryIdentifier: String
    public var threadIdentifier: String
    public var userInfo: [String: String]
    public var fireAt: Date
    public var trigger: DateComponents

    public init(
        identifier: String, title: String, subtitle: String, body: String, categoryIdentifier: String,
        threadIdentifier: String, userInfo: [String: String], fireAt: Date, trigger: DateComponents
    ) {
        self.identifier = identifier
        self.title = title
        self.subtitle = subtitle
        self.body = body
        self.categoryIdentifier = categoryIdentifier
        self.threadIdentifier = threadIdentifier
        self.userInfo = userInfo
        self.fireAt = fireAt
        self.trigger = trigger
    }
}

/// A notification the system holds for this app, pending or delivered.
public struct HeldNotification: Hashable, Sendable {
    public var identifier: String
    public var userInfo: [String: String]

    public init(identifier: String, userInfo: [String: String]) {
        self.identifier = identifier
        self.userInfo = userInfo
    }
}

/// An action button a reminder category offers.
public struct ReminderActionDefinition: Hashable, Sendable {
    public var identifier: String
    public var title: String
    public var opensApp: Bool

    public init(identifier: String, title: String, opensApp: Bool) {
        self.identifier = identifier
        self.title = title
        self.opensApp = opensApp
    }
}

public struct ReminderCategory: Hashable, Sendable {
    public var identifier: String
    public var actions: [ReminderActionDefinition]

    public init(identifier: String, actions: [ReminderActionDefinition]) {
        self.identifier = identifier
        self.actions = actions
    }
}

/// The part of UNUserNotificationCenter the reminders use. The app passes the
/// system center; the tests pass a fake that records every call.
public protocol ReminderNotificationCenter: Sendable {
    func permission() async -> NotificationPermission
    /// Asks once; answers whether delivery is allowed afterwards. A denial is
    /// an answer, not an error.
    func requestPermission() async -> NotificationPermission
    func setCategories(_ categories: Set<ReminderCategory>) async
    func pending() async -> [HeldNotification]
    func add(_ request: ReminderRequest) async throws
    func removePending(_ identifiers: [String]) async
    func removeDelivered(_ identifiers: [String]) async
}

/// What one reconciliation found and did.
public struct ReconcileReport: Hashable, Sendable {
    public enum Outcome: String, Hashable, Sendable {
        /// The pending notifications are exactly the plan.
        case scheduled
        /// Reminders are off on this device; none are pending.
        case disabled
        /// The system does not let this app notify; none are pending, and
        /// nothing else changed.
        case permissionDenied
        /// Permission was never asked; none are pending until it is.
        case permissionNotDetermined
        /// No workspace is open, so no reminder could be acted on; none are
        /// pending.
        case noWorkspace
        /// The engine could not plan; the pending reminders were left as
        /// they were rather than cleared on a guess.
        case planFailed
    }

    public var outcome: Outcome
    public var permission: NotificationPermission
    public var added: [String] = []
    public var replaced: [String] = []
    public var removed: [String] = []
    public var kept: [String] = []
    public var failed: [String: String] = [:]
    /// The soonest planned reminder, when any is scheduled.
    public var next: PlannedReminder?
    public var timeZone: String?
    public var truncated = false
    /// Why planning failed, for `.planFailed`.
    public var error: String?

    public init(outcome: Outcome, permission: NotificationPermission) {
        self.outcome = outcome
        self.permission = permission
    }

    public var scheduledCount: Int { added.count + replaced.count + kept.count }
}

/// Keeps the system's pending notifications equal to the engine's plan.
///
/// Every reminder notification this app schedules carries an identifier under
/// `ReminderScheduler.prefix`, so a reconciliation can tell its own from any
/// other and removes every one the plan no longer names — including another
/// workspace's, whose actions would otherwise reach the wrong data.
public struct ReminderScheduler: Sendable {
    public static let prefix = "tt.reminder/"
    public static let acknowledgeCategory = "tt.reminder.acknowledge"
    public static let deferCategory = "tt.reminder.defer"
    public static let openCategory = "tt.reminder.open"

    public let center: any ReminderNotificationCenter

    public init(center: any ReminderNotificationCenter) {
        self.center = center
    }

    /// The categories and buttons a reminder may show. Acknowledging and
    /// deferring commit in the background; tapping the notification opens
    /// the task.
    public static let categories: Set<ReminderCategory> = {
        let acknowledge = ReminderActionDefinition(identifier: ReminderAction.acknowledgeIdentifier, title: "Acknowledge", opensApp: false)
        let hour = ReminderActionDefinition(identifier: ReminderAction.deferHourIdentifier, title: "Defer 1 Hour", opensApp: false)
        let tomorrow = ReminderActionDefinition(identifier: ReminderAction.deferTomorrowIdentifier, title: "Defer to Tomorrow", opensApp: false)
        return [
            ReminderCategory(identifier: acknowledgeCategory, actions: [acknowledge, hour, tomorrow]),
            ReminderCategory(identifier: deferCategory, actions: [hour, tomorrow]),
            ReminderCategory(identifier: openCategory, actions: []),
        ]
    }()

    /// Registers the action categories. Safe to call at every launch.
    public func registerCategories() async {
        await center.setCategories(Self.categories)
    }

    /// Turns reminders on: asks for permission if it was never asked, then
    /// reconciles. A denial schedules nothing and is reported, not thrown.
    public func enable(plan: ReminderPlan?, settings: ReminderSettings, workspaceID: String?) async -> ReconcileReport {
        if settings.enabled, await center.permission() == .notDetermined {
            _ = await center.requestPermission()
        }
        return await reconcile(plan: plan, settings: settings, workspaceID: workspaceID)
    }

    /// Makes the pending reminders exactly `plan`. With reminders off, or
    /// without permission, it only removes this app's pending reminders.
    public func reconcile(plan: ReminderPlan?, settings: ReminderSettings, workspaceID: String?) async -> ReconcileReport {
        let permission = await center.permission()
        let held = await center.pending().filter { $0.identifier.hasPrefix(Self.prefix) }
        func clear(_ outcome: ReconcileReport.Outcome) async -> ReconcileReport {
            var report = ReconcileReport(outcome: outcome, permission: permission)
            report.removed = held.map(\.identifier).sorted()
            if !report.removed.isEmpty {
                await center.removePending(report.removed)
            }
            return report
        }
        guard settings.enabled else { return await clear(.disabled) }
        guard let plan, let workspaceID else { return await clear(.noWorkspace) }
        switch permission {
        case .denied: return await clear(.permissionDenied)
        case .notDetermined: return await clear(.permissionNotDetermined)
        case .authorized, .provisional, .ephemeral: break
        }

        var report = ReconcileReport(outcome: .scheduled, permission: permission)
        report.timeZone = plan.timeZone
        report.truncated = plan.truncated
        let zone = TimeZone(identifier: plan.timeZone) ?? .current
        let wanted = Dictionary(
            plan.reminders.map { (Self.identifier(workspaceID: workspaceID, reminder: $0), $0) },
            uniquingKeysWith: { first, _ in first }
        )
        let heldByID = Dictionary(held.map { ($0.identifier, $0) }, uniquingKeysWith: { first, _ in first })

        let stale = held.filter { notification in
            guard let reminder = wanted[notification.identifier] else { return true }
            return notification.userInfo[UserInfoKey.fingerprint] != reminder.fingerprint
        }.map(\.identifier).sorted()
        if !stale.isEmpty {
            await center.removePending(stale)
        }
        for (identifier, reminder) in wanted.sorted(by: { ($0.value.fireAt, $0.key) < ($1.value.fireAt, $1.key) }) {
            if let existing = heldByID[identifier], !stale.contains(existing.identifier) {
                report.kept.append(identifier)
                continue
            }
            do {
                try await center.add(Self.request(for: reminder, workspaceID: workspaceID, zone: zone))
                if heldByID[identifier] != nil {
                    report.replaced.append(identifier)
                } else {
                    report.added.append(identifier)
                }
            } catch {
                report.failed[identifier] = String(describing: error)
            }
        }
        report.removed = stale.filter { wanted[$0] == nil }
        report.next = plan.reminders.min { $0.fireAt < $1.fireAt }
        return report
    }

    public static func identifier(workspaceID: String, reminder: PlannedReminder) -> String {
        "\(prefix)\(workspaceID)/\(reminder.id)"
    }

    /// The notification for one reminder. Wall-clock reminders name the
    /// plan's zone in their trigger and a deferral names GMT, so each fires at
    /// the instant the engine planned until the next reconciliation replans
    /// it for a zone the device moved to.
    public static func request(for reminder: PlannedReminder, workspaceID: String, zone: TimeZone) -> ReminderRequest {
        let triggerZone = reminder.kind == .deferralEnds ? TimeZone(secondsFromGMT: 0)! : zone
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = triggerZone
        var trigger = calendar.dateComponents([.year, .month, .day, .hour, .minute, .second], from: reminder.fireAt)
        trigger.calendar = calendar
        trigger.timeZone = triggerZone

        let category: String
        if reminder.canAcknowledge {
            category = acknowledgeCategory
        } else if reminder.canDefer {
            category = deferCategory
        } else {
            category = openCategory
        }
        let title: String
        switch reminder.kind {
        case .checkIn: title = "Check in: \(reminder.title)"
        case .deadline: title = "Due now: \(reminder.title)"
        case .deferralEnds: title = "Back from deferral: \(reminder.title)"
        }
        return ReminderRequest(
            identifier: identifier(workspaceID: workspaceID, reminder: reminder),
            title: title,
            subtitle: reminder.projectName,
            body: body(for: reminder),
            categoryIdentifier: category,
            threadIdentifier: "\(prefix)\(workspaceID)",
            userInfo: [
                UserInfoKey.workspace: workspaceID,
                UserInfoKey.reminder: reminder.id,
                UserInfoKey.kind: reminder.kind.rawValue,
                UserInfoKey.project: reminder.task.projectID,
                UserInfoKey.task: reminder.task.taskID,
                UserInfoKey.title: reminder.title,
                UserInfoKey.fingerprint: reminder.fingerprint,
            ],
            fireAt: reminder.fireAt,
            trigger: trigger
        )
    }

    private static func body(for reminder: PlannedReminder) -> String {
        switch reminder.kind {
        case .checkIn: "Its check-in day has arrived."
        case .deadline: "Its deadline is now."
        case .deferralEnds: "It is back in view."
        }
    }
}

/// The keys a reminder notification's userInfo carries.
public enum UserInfoKey {
    public static let workspace = "tt.workspace"
    public static let reminder = "tt.reminder"
    public static let kind = "tt.kind"
    public static let project = "tt.project"
    public static let task = "tt.task"
    public static let title = "tt.title"
    public static let fingerprint = "tt.fingerprint"
}
