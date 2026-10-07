import Foundation

/// Why a reconciliation ran. Every moment the plan may have changed is one:
/// the workspace opening, the app returning to the foreground, a local
/// commit, changes another device sent, the device changing zone or clock,
/// the reminder settings, and a notification action's own commit.
public enum ReconcileReason: String, Hashable, Sendable, CaseIterable {
    case launch
    case foreground
    case localChange
    case incomingChanges
    case timeZoneChange
    case settingsChange
    case action
}

/// Asks the engine for a plan in the device's zone now and makes the pending
/// notifications match it. One reconciliation runs at a time, so a burst of
/// triggers never interleaves removals and additions.
public actor ReminderReconciler {
    /// Plans in the named IANA zone with the given settings, or answers nil
    /// when no workspace is open.
    public typealias PlanSource = @Sendable (_ timeZone: String, _ settings: ReminderSettings) async throws -> (workspaceID: String, plan: ReminderPlan)?

    public let scheduler: ReminderScheduler
    private let settingsStore: ReminderSettingsStore
    private let planSource: PlanSource
    private let timeZone: @Sendable () -> TimeZone

    public private(set) var lastReport: ReconcileReport?
    public private(set) var lastReason: ReconcileReason?
    public private(set) var runs = 0

    public init(
        scheduler: ReminderScheduler,
        settingsStore: ReminderSettingsStore,
        timeZone: @escaping @Sendable () -> TimeZone = { TimeZone.current },
        planSource: @escaping PlanSource
    ) {
        self.scheduler = scheduler
        self.settingsStore = settingsStore
        self.timeZone = timeZone
        self.planSource = planSource
    }

    public var settings: ReminderSettings { settingsStore.load() }

    /// Saves new settings and reconciles under them, asking for permission
    /// when reminders are turned on for the first time.
    @discardableResult
    public func update(_ settings: ReminderSettings) async throws -> ReconcileReport {
        try settingsStore.save(settings)
        return await run(.settingsChange, settings: settings, askPermission: settings.enabled)
    }

    @discardableResult
    public func reconcile(_ reason: ReconcileReason) async -> ReconcileReport {
        await run(reason, settings: settingsStore.load(), askPermission: false)
    }

    private func run(_ reason: ReconcileReason, settings: ReminderSettings, askPermission: Bool) async -> ReconcileReport {
        runs += 1
        lastReason = reason
        let report: ReconcileReport
        if !settings.enabled {
            report = await scheduler.reconcile(plan: nil, settings: settings, workspaceID: nil)
        } else {
            do {
                let planned = try await planSource(timeZone().identifier, settings)
                if askPermission {
                    report = await scheduler.enable(plan: planned?.plan, settings: settings, workspaceID: planned?.workspaceID)
                } else {
                    report = await scheduler.reconcile(plan: planned?.plan, settings: settings, workspaceID: planned?.workspaceID)
                }
            } catch {
                var failed = ReconcileReport(outcome: .planFailed, permission: await scheduler.center.permission())
                failed.error = (error as? LocalizedError)?.errorDescription ?? String(describing: error)
                report = failed
            }
        }
        lastReport = report
        return report
    }
}
