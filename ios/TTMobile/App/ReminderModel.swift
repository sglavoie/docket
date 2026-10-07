import Foundation
import Observation
import TTEngine
import TTReminders
import UIKit
import UserNotifications

/// The app's reminders: the device's settings, the reconciliation that keeps
/// the system's pending notifications equal to the engine's plan, and the
/// routing of notification actions back to engine commands.
///
/// It reconciles only while a workspace is open — after the open, every
/// local commit, every import, each return to the foreground, a time-zone or
/// clock change, a settings change, and an action's own commit — and clears
/// every pending reminder when the workspace closes, so no notification can
/// act on data that is not open.
@MainActor
@Observable
final class ReminderModel {
    private(set) var settings: ReminderSettings = .default
    private(set) var report: ReconcileReport?
    private(set) var activity: [ReminderActivity] = []
    private(set) var isAttached = false
    /// A task a notification asked to show; the shell presents it.
    var openRequest: TaskLocator?

    @ObservationIgnored private let root: URL
    @ObservationIgnored private var reconciler: ReminderReconciler?
    @ObservationIgnored private var router: ReminderActionRouter?
    @ObservationIgnored private var observers: [NSObjectProtocol] = []
    @ObservationIgnored private let center = SystemNotificationCenter()

    init(root: URL) {
        self.root = root
        settings = store.load()
        activity = log.entries()
        let names: [Notification.Name] = [.NSSystemTimeZoneDidChange, UIApplication.significantTimeChangeNotification]
        observers = names.map { name in
            Foundation.NotificationCenter.default.addObserver(forName: name, object: nil, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.reconcile(.timeZoneChange) }
            }
        }
        Task { await ReminderScheduler(center: center).registerCategories() }
    }

    private var store: ReminderSettingsStore {
        ReminderSettingsStore(file: root.appendingPathComponent("reminders/settings.json"))
    }

    private var log: ReminderActivityLog {
        ReminderActivityLog(file: root.appendingPathComponent("reminders/activity.json"))
    }

    /// The device's zone now, as the engine is told it.
    var timeZone: String { TimeZone.current.identifier }

    /// Starts reconciling for the open workspace.
    func attach(engine: TTEngine, commands: any ReminderCommands) {
        reconciler = ReminderReconciler(
            scheduler: ReminderScheduler(center: center), settingsStore: store,
            planSource: { zone, settings in
                guard let workspace = await engine.workspaceID else { return nil }
                let plan = try await engine.reminders(timeZone: zone, checkInTime: settings.checkInTime)
                return (workspace, Self.plan(plan))
            }
        )
        router = ReminderActionRouter(commands: commands, log: log)
        isAttached = true
        reconcile(.launch)
    }

    /// Stops reconciling and clears every pending reminder: the workspace
    /// they would act on is no longer open.
    func detach() {
        isAttached = false
        let scheduler = ReminderScheduler(center: center)
        let settings = self.settings
        Task {
            let cleared = await scheduler.reconcile(plan: nil, settings: settings, workspaceID: nil)
            self.report = cleared
        }
    }

    func reconcile(_ reason: ReconcileReason) {
        guard isAttached, let reconciler else { return }
        Task {
            let report = await reconciler.reconcile(reason)
            self.report = report
        }
    }

    /// Saves new settings; turning reminders on asks for permission once.
    /// A denial is shown, never thrown.
    func update(_ settings: ReminderSettings) async {
        self.settings = settings
        if let reconciler {
            report = try? await reconciler.update(settings)
        } else {
            try? store.save(settings)
        }
    }

    /// Re-reads permission and the plan, as returning from Settings does.
    func refresh() async {
        activity = log.entries()
        guard isAttached, let reconciler else { return }
        report = await reconciler.reconcile(.foreground)
    }

    /// Handles one notification response, then reconciles: an action that
    /// committed changed the plan.
    func handle(actionIdentifier: String, userInfo: [String: String]) async {
        guard let router else {
            log.append(ReminderActivity(
                at: Date(), result: .refused, kind: userInfo[UserInfoKey.kind] ?? "",
                title: userInfo[UserInfoKey.title] ?? "a task", task: nil, detail: "no workspace is open"
            ))
            activity = log.entries()
            return
        }
        let handled = await router.handle(actionIdentifier: actionIdentifier, userInfo: userInfo)
        activity = log.entries()
        if let handled, handled.result == .acknowledged || handled.result == .deferred {
            if let reconciler { report = await reconciler.reconcile(.action) }
        }
    }

    /// The engine's wire plan in the scheduler's terms.
    nonisolated static func plan(_ plan: EngineReminderPlan) -> ReminderPlan {
        ReminderPlan(
            timeZone: plan.timeZone, checkInTime: plan.checkInTime, generatedAt: plan.generatedAt,
            truncated: plan.truncated,
            reminders: plan.reminders.compactMap { reminder in
                guard let kind = ReminderKind(rawValue: reminder.kind) else { return nil }
                return PlannedReminder(
                    id: reminder.id, kind: kind,
                    task: ReminderTask(projectID: reminder.task.projectID, taskID: reminder.task.taskID),
                    projectName: reminder.projectName, shortID: reminder.shortID, title: reminder.title,
                    fireAt: reminder.fireAt, localTime: reminder.localTime,
                    canAcknowledge: reminder.canAcknowledge, canDefer: reminder.canDefer,
                    fingerprint: reminder.fingerprint
                )
            }
        )
    }
}

/// The engine commands a notification action runs, through the workspace so
/// each commit is reread and published like any other.
struct WorkspaceReminderCommands: ReminderCommands {
    let workspace: WorkspaceModel

    func openWorkspaceID() async -> String? {
        await workspace.ensureReady() ? await workspace.status?.workspace : nil
    }

    func acknowledge(_ task: ReminderTask) async throws {
        if let refusal = await workspace.checkIn([locator(task)], .acknowledge) {
            throw ReminderRefusal(message: refusal)
        }
    }

    func deferTask(_ task: ReminderTask, until value: String) async throws {
        if let refusal = await workspace.edit(locator(task), TaskEdit(deferUntil: value)) {
            throw ReminderRefusal(message: refusal)
        }
    }

    func show(_ task: ReminderTask) async {
        await MainActor.run { workspace.reminders.openRequest = locator(task) }
    }

    private func locator(_ task: ReminderTask) -> TaskLocator {
        TaskLocator(projectID: task.projectID, taskID: task.taskID)
    }
}

struct ReminderRefusal: Error, LocalizedError {
    let message: String
    var errorDescription: String? { message }
}

/// Receives notification presentations and responses for the whole app. The
/// system delivers a response to a background action by launching the app
/// without a scene, so the workspace lives here rather than in a view.
@MainActor
final class AppDelegate: NSObject, UIApplicationDelegate, UNUserNotificationCenterDelegate {
    let workspace = WorkspaceModel()

    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
    ) -> Bool {
        UNUserNotificationCenter.current().delegate = self
        return true
    }

    /// A reminder that fires while tt is open is still shown.
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter, willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .list, .sound])
    }

    /// Handles a response on the main actor and only then tells the system it
    /// is done, on the main thread as UIKit requires; the async form of this
    /// method completes off the main thread and UIKit aborts.
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let action = response.actionIdentifier
        let userInfo = SystemNotificationCenter.strings(response.notification.request.content.userInfo)
        nonisolated(unsafe) let done = completionHandler
        Task { @MainActor in
            await self.workspace.reminders.handle(actionIdentifier: action, userInfo: userInfo)
            done()
        }
    }
}
