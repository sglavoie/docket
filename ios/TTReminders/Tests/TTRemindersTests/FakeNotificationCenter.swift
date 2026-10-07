import Foundation
@testable import TTReminders

/// A notification center that keeps its state in memory and records every
/// call, standing in for UNUserNotificationCenter.
actor FakeNotificationCenter: ReminderNotificationCenter {
    var permissionState: NotificationPermission
    /// What a permission prompt answers.
    var promptAnswer: NotificationPermission
    private(set) var prompts = 0
    private(set) var categories: Set<ReminderCategory> = []
    private(set) var pendingRequests: [String: ReminderRequest] = [:]
    /// Notifications another part of the app scheduled, which reminders must
    /// never touch.
    private(set) var foreign: [String: [String: String]] = [:]
    private(set) var removedPending: [[String]] = []
    private(set) var adds = 0
    var failAdding: Set<String> = []

    init(permission: NotificationPermission = .authorized, promptAnswer: NotificationPermission = .authorized) {
        permissionState = permission
        self.promptAnswer = promptAnswer
    }

    func setPermission(_ permission: NotificationPermission) { permissionState = permission }
    func setPromptAnswer(_ answer: NotificationPermission) { promptAnswer = answer }
    func addForeign(_ identifier: String) { foreign[identifier] = ["other": "yes"] }
    func failAdding(_ identifier: String) { failAdding.insert(identifier) }
    func allowAdding() { failAdding = [] }

    func permission() async -> NotificationPermission { permissionState }

    func requestPermission() async -> NotificationPermission {
        prompts += 1
        if permissionState == .notDetermined {
            permissionState = promptAnswer
        }
        return permissionState
    }

    func setCategories(_ categories: Set<ReminderCategory>) async {
        self.categories = categories
    }

    func pending() async -> [HeldNotification] {
        pendingRequests.values.map { HeldNotification(identifier: $0.identifier, userInfo: $0.userInfo) }
            + foreign.map { HeldNotification(identifier: $0.key, userInfo: $0.value) }
    }

    func add(_ request: ReminderRequest) async throws {
        if failAdding.contains(request.identifier) {
            throw FakeFailure(message: "the system refused \(request.identifier)")
        }
        adds += 1
        pendingRequests[request.identifier] = request
    }

    func removePending(_ identifiers: [String]) async {
        removedPending.append(identifiers)
        for identifier in identifiers {
            pendingRequests[identifier] = nil
            foreign[identifier] = nil
        }
    }

    func removeDelivered(_ identifiers: [String]) async {}

    /// The pending reminders' identifiers, sorted.
    var reminderIDs: [String] { pendingRequests.keys.sorted() }
    var foreignIDs: [String] { foreign.keys.sorted() }
}

struct FakeFailure: Error, LocalizedError {
    let message: String
    var errorDescription: String? { message }
}

/// Engine commands that record what a reminder action asked for.
actor FakeCommands: ReminderCommands {
    var workspace: String?
    private(set) var acknowledged: [ReminderTask] = []
    private(set) var deferred: [(ReminderTask, String)] = []
    private(set) var shown: [ReminderTask] = []
    var refusal: String?

    init(workspace: String?) {
        self.workspace = workspace
    }

    func refuse(with message: String?) { refusal = message }

    func openWorkspaceID() async -> String? { workspace }

    func acknowledge(_ task: ReminderTask) async throws {
        if let refusal { throw FakeFailure(message: refusal) }
        acknowledged.append(task)
    }

    func deferTask(_ task: ReminderTask, until value: String) async throws {
        if let refusal { throw FakeFailure(message: refusal) }
        deferred.append((task, value))
    }

    func show(_ task: ReminderTask) async {
        shown.append(task)
    }

    var writes: Int { acknowledged.count + deferred.count }
}

/// A stand-in for the engine's planner: the same placement rules the bridge
/// applies (check-ins at the reminder time on their day in the device's zone,
/// timed deadlines at their wall-clock time there, deferrals at their
/// instant), over a small editable task list.
final class FakeEngine: @unchecked Sendable {
    struct Item {
        var task: ReminderTask
        var title: String
        var checkIn: DateComponents?
        var deadline: DateComponents?
        var deferral: Date?
        var done = false
    }

    private let lock = NSLock()
    private var items: [Item]
    var workspaceID: String? = "workspace-a"
    var failure: String?
    private(set) var zonesAsked: [String] = []
    let now: Date

    init(now: Date, items: [Item]) {
        self.now = now
        self.items = items
    }

    func update(_ change: (inout [Item]) -> Void) {
        lock.lock()
        defer { lock.unlock() }
        change(&items)
    }

    func plan(zone identifier: String, settings: ReminderSettings) throws -> (workspaceID: String, plan: ReminderPlan)? {
        lock.lock()
        defer { lock.unlock() }
        zonesAsked.append(identifier)
        if let failure { throw FakeFailure(message: failure) }
        guard let workspaceID else { return nil }
        let zone = TimeZone(identifier: identifier)!
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = zone
        var reminders: [PlannedReminder] = []
        func add(_ kind: ReminderKind, _ item: Item, _ at: Date, acknowledge: Bool) {
            guard at > now else { return }
            let local = DateFormatter()
            local.calendar = calendar
            local.timeZone = zone
            local.locale = Locale(identifier: "en_US_POSIX")
            local.dateFormat = "yyyy-MM-dd'T'HH:mm:ss"
            reminders.append(PlannedReminder(
                id: "\(kind.rawValue)/\(item.task.projectID)/\(item.task.taskID)", kind: kind, task: item.task,
                projectName: "Home", shortID: 1, title: item.title, fireAt: at, localTime: local.string(from: at),
                canAcknowledge: acknowledge, canDefer: true,
                fingerprint: "\(kind.rawValue)|\(at.timeIntervalSince1970)|\(item.title)"
            ))
        }
        for item in items where !item.done {
            if var day = item.checkIn {
                day.hour = settings.checkInHour
                day.minute = settings.checkInMinute
                add(.checkIn, item, calendar.date(from: day)!, acknowledge: true)
            }
            if let deadline = item.deadline {
                add(.deadline, item, calendar.date(from: deadline)!, acknowledge: false)
            }
            if let deferral = item.deferral {
                add(.deferralEnds, item, deferral, acknowledge: item.checkIn != nil)
            }
        }
        reminders.sort { $0.fireAt < $1.fireAt }
        return (workspaceID, ReminderPlan(
            timeZone: identifier, checkInTime: settings.checkInTime, generatedAt: now, truncated: false, reminders: reminders
        ))
    }
}
