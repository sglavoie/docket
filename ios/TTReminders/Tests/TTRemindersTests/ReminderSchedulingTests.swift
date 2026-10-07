import Foundation
import XCTest
@testable import TTReminders

/// Scheduling against a fake notification center: what is scheduled for a
/// plan and with which zone, how a later plan replaces, keeps, and cancels,
/// and how opt-in, permission denial, a time-zone change, incoming changes,
/// and a failed plan behave.
final class ReminderSchedulingTests: XCTestCase {
    // 2026-10-06T09:00:00Z: 05:00 in Toronto, 11:00 in Paris.
    let now = Date(timeIntervalSince1970: 1_791_277_200)
    let bank = ReminderTask(projectID: "home", taskID: "bank")
    let report = ReminderTask(projectID: "home", taskID: "report")
    let wake = ReminderTask(projectID: "home", taskID: "wake")
    var directory: URL!

    override func setUpWithError() throws {
        directory = FileManager.default.temporaryDirectory.appendingPathComponent("tt-reminders-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: directory)
    }

    func engine() -> FakeEngine {
        FakeEngine(now: now, items: [
            .init(task: bank, title: "Call the bank", checkIn: DateComponents(year: 2026, month: 10, day: 7)),
            .init(task: report, title: "Send the report", deadline: DateComponents(year: 2026, month: 10, day: 6, hour: 17, minute: 30)),
            .init(task: wake, title: "Renew the passport", deferral: Date(timeIntervalSince1970: 1_791_460_800)), // 2026-10-08T12:00Z
        ])
    }

    func store(enabled: Bool = true) throws -> ReminderSettingsStore {
        let store = ReminderSettingsStore(file: directory.appendingPathComponent("reminders/settings.json"))
        try store.save(ReminderSettings(enabled: enabled, checkInHour: 9, checkInMinute: 0))
        return store
    }

    func reconciler(
        center: FakeNotificationCenter, engine: FakeEngine, store: ReminderSettingsStore,
        zone: ZoneBox = ZoneBox("America/Toronto")
    ) -> ReminderReconciler {
        ReminderReconciler(
            scheduler: ReminderScheduler(center: center), settingsStore: store, timeZone: { zone.current },
            planSource: { identifier, settings in try engine.plan(zone: identifier, settings: settings) }
        )
    }

    func utc(_ text: String) -> Date {
        ISO8601DateFormatter().date(from: text)!
    }

    /// The instant a calendar trigger names.
    func fireDate(_ request: ReminderRequest) -> Date? {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = request.trigger.timeZone!
        return calendar.date(from: request.trigger)
    }

    func testSchedulesThePlanWithExplicitZones() async throws {
        let center = FakeNotificationCenter()
        let reconciler = reconciler(center: center, engine: engine(), store: try store())
        let report = await reconciler.reconcile(.launch)
        XCTAssertEqual(report.outcome, .scheduled)
        XCTAssertEqual(report.added.count, 3)
        XCTAssertEqual(report.timeZone, "America/Toronto")
        XCTAssertEqual(report.next?.title, "Send the report")

        let pending = await center.pendingRequests
        let checkIn = try XCTUnwrap(pending["tt.reminder/workspace-a/check_in/home/bank"])
        XCTAssertEqual(checkIn.trigger.timeZone?.identifier, "America/Toronto")
        XCTAssertEqual(checkIn.trigger.hour, 9)
        XCTAssertEqual(checkIn.trigger.day, 7)
        XCTAssertEqual(fireDate(checkIn), utc("2026-10-07T13:00:00Z"))
        XCTAssertEqual(checkIn.title, "Check in: Call the bank")
        XCTAssertEqual(checkIn.subtitle, "Home")
        XCTAssertEqual(checkIn.categoryIdentifier, ReminderScheduler.acknowledgeCategory)
        XCTAssertEqual(checkIn.userInfo[UserInfoKey.workspace], "workspace-a")
        XCTAssertEqual(checkIn.userInfo[UserInfoKey.task], "bank")
        XCTAssertEqual(checkIn.userInfo[UserInfoKey.project], "home")

        let deadline = try XCTUnwrap(pending["tt.reminder/workspace-a/deadline/home/report"])
        XCTAssertEqual(deadline.trigger.timeZone?.identifier, "America/Toronto")
        XCTAssertEqual(deadline.trigger.hour, 17)
        XCTAssertEqual(deadline.trigger.minute, 30)
        XCTAssertEqual(deadline.categoryIdentifier, ReminderScheduler.deferCategory)
        XCTAssertEqual(deadline.title, "Due now: Send the report")

        // A deferral is an instant: its trigger names GMT.
        let deferral = try XCTUnwrap(pending["tt.reminder/workspace-a/deferral_ends/home/wake"])
        XCTAssertEqual(deferral.trigger.timeZone?.secondsFromGMT(), 0)
        XCTAssertEqual(deferral.trigger.hour, 12)
        XCTAssertEqual(fireDate(deferral), utc("2026-10-08T12:00:00Z"))
        XCTAssertEqual(deferral.title, "Back from deferral: Renew the passport")
    }

    func testReconcileReplacesMovedRemovesStaleAndKeepsUnchanged() async throws {
        let center = FakeNotificationCenter()
        await center.addForeign("calendar-alarm")
        let engine = engine()
        let reconciler = reconciler(center: center, engine: engine, store: try store())
        await reconciler.reconcile(.launch)
        let addsBefore = await center.adds

        // An unchanged plan adds nothing.
        let again = await reconciler.reconcile(.foreground)
        XCTAssertEqual(again.kept.count, 3)
        XCTAssertTrue(again.added.isEmpty && again.replaced.isEmpty && again.removed.isEmpty)
        let addsAfterRepeat = await center.adds
        XCTAssertEqual(addsAfterRepeat, addsBefore)

        // The deadline moves, the deferral goes, a new check-in arrives.
        engine.update { items in
            items[1].deadline?.hour = 18
            items[2].deferral = nil
            items.append(.init(task: ReminderTask(projectID: "home", taskID: "plants"), title: "Water the plants",
                               checkIn: DateComponents(year: 2026, month: 10, day: 9)))
        }
        let changed = await reconciler.reconcile(.localChange)
        XCTAssertEqual(changed.replaced, ["tt.reminder/workspace-a/deadline/home/report"])
        XCTAssertEqual(changed.removed, ["tt.reminder/workspace-a/deferral_ends/home/wake"])
        XCTAssertEqual(changed.added, ["tt.reminder/workspace-a/check_in/home/plants"])
        XCTAssertEqual(changed.kept, ["tt.reminder/workspace-a/check_in/home/bank"])
        let ids = await center.reminderIDs
        XCTAssertEqual(ids, [
            "tt.reminder/workspace-a/check_in/home/bank",
            "tt.reminder/workspace-a/check_in/home/plants",
            "tt.reminder/workspace-a/deadline/home/report",
        ])
        let moved = await center.pendingRequests["tt.reminder/workspace-a/deadline/home/report"]
        XCTAssertEqual(moved?.trigger.hour, 18)
        // Another feature's notification is never touched.
        let foreign = await center.foreignIDs
        XCTAssertEqual(foreign, ["calendar-alarm"])
    }

    func testTimeZoneChangeReplansWallClockReminders() async throws {
        let center = FakeNotificationCenter()
        let engine = engine()
        let zone = ZoneBox("America/Toronto")
        let reconciler = reconciler(center: center, engine: engine, store: try store(), zone: zone)
        await reconciler.reconcile(.launch)

        zone.current = TimeZone(identifier: "Europe/Paris")!
        let moved = await reconciler.reconcile(.timeZoneChange)
        XCTAssertEqual(engine.zonesAsked, ["America/Toronto", "Europe/Paris"])
        XCTAssertEqual(moved.timeZone, "Europe/Paris")
        XCTAssertEqual(Set(moved.replaced), [
            "tt.reminder/workspace-a/check_in/home/bank",
            "tt.reminder/workspace-a/deadline/home/report",
        ])
        XCTAssertEqual(moved.kept, ["tt.reminder/workspace-a/deferral_ends/home/wake"])
        let pending = await center.pendingRequests
        // Wall-clock reminders keep their wall-clock time in the new zone ...
        let checkIn = try XCTUnwrap(pending["tt.reminder/workspace-a/check_in/home/bank"])
        XCTAssertEqual(checkIn.trigger.timeZone?.identifier, "Europe/Paris")
        XCTAssertEqual(checkIn.trigger.hour, 9)
        XCTAssertEqual(fireDate(checkIn), utc("2026-10-07T07:00:00Z"))
        let deadline = try XCTUnwrap(pending["tt.reminder/workspace-a/deadline/home/report"])
        XCTAssertEqual(fireDate(deadline), utc("2026-10-06T15:30:00Z"))
        // ... and the deferral keeps its instant.
        let deferral = try XCTUnwrap(pending["tt.reminder/workspace-a/deferral_ends/home/wake"])
        XCTAssertEqual(fireDate(deferral), utc("2026-10-08T12:00:00Z"))
    }

    func testIncomingChangesReconcile() async throws {
        let center = FakeNotificationCenter()
        let engine = engine()
        let reconciler = reconciler(center: center, engine: engine, store: try store())
        await reconciler.reconcile(.launch)
        // Another device finishes the report and moves the check-in.
        engine.update { items in
            items[1].done = true
            items[0].checkIn = DateComponents(year: 2026, month: 10, day: 10)
        }
        let report = await reconciler.reconcile(.incomingChanges)
        XCTAssertEqual(report.removed, ["tt.reminder/workspace-a/deadline/home/report"])
        XCTAssertEqual(report.replaced, ["tt.reminder/workspace-a/check_in/home/bank"])
        let checkIn = await center.pendingRequests["tt.reminder/workspace-a/check_in/home/bank"]
        XCTAssertEqual(checkIn?.trigger.day, 10)
        let lastReason = await reconciler.lastReason
        XCTAssertEqual(lastReason, .incomingChanges)
    }

    func testRemindersAreOptInAndNeverPromptUntilTurnedOn() async throws {
        let center = FakeNotificationCenter(permission: .notDetermined)
        let store = try store(enabled: false)
        let reconciler = reconciler(center: center, engine: engine(), store: store)
        for reason in ReconcileReason.allCases where reason != .settingsChange {
            let report = await reconciler.reconcile(reason)
            XCTAssertEqual(report.outcome, .disabled)
        }
        let prompts = await center.prompts
        XCTAssertEqual(prompts, 0)
        let ids = await center.reminderIDs
        XCTAssertTrue(ids.isEmpty)
        XCTAssertEqual(ReminderSettings.default.enabled, false)

        // Turning them on asks once and schedules.
        let on = try await reconciler.update(ReminderSettings(enabled: true, checkInHour: 9, checkInMinute: 0))
        XCTAssertEqual(on.outcome, .scheduled)
        let promptsAfter = await center.prompts
        XCTAssertEqual(promptsAfter, 1)
        XCTAssertEqual(store.load().enabled, true)

        // Turning them off removes every reminder.
        let off = try await reconciler.update(ReminderSettings(enabled: false, checkInHour: 9, checkInMinute: 0))
        XCTAssertEqual(off.outcome, .disabled)
        XCTAssertEqual(off.removed.count, 3)
        let left = await center.reminderIDs
        XCTAssertTrue(left.isEmpty)
    }

    func testPermissionDenialIsHarmless() async throws {
        let center = FakeNotificationCenter(permission: .notDetermined, promptAnswer: .denied)
        await center.addForeign("calendar-alarm")
        let store = try store(enabled: false)
        let engine = engine()
        let reconciler = reconciler(center: center, engine: engine, store: store)

        let denied = try await reconciler.update(ReminderSettings(enabled: true, checkInHour: 9, checkInMinute: 0))
        XCTAssertEqual(denied.outcome, .permissionDenied)
        XCTAssertEqual(denied.permission, .denied)
        XCTAssertNil(denied.error)
        let ids = await center.reminderIDs
        XCTAssertTrue(ids.isEmpty)
        let foreign = await center.foreignIDs
        XCTAssertEqual(foreign, ["calendar-alarm"])
        // The choice is kept, and later triggers neither prompt again nor fail.
        XCTAssertEqual(store.load().enabled, true)
        for reason in [ReconcileReason.foreground, .localChange, .incomingChanges, .timeZoneChange] {
            let report = await reconciler.reconcile(reason)
            XCTAssertEqual(report.outcome, .permissionDenied)
        }
        let prompts = await center.prompts
        XCTAssertEqual(prompts, 1)

        // Allowing notifications in Settings is picked up at the next foreground.
        await center.setPermission(.authorized)
        let allowed = await reconciler.reconcile(.foreground)
        XCTAssertEqual(allowed.outcome, .scheduled)
        XCTAssertEqual(allowed.added.count, 3)

        // Revoking it later clears what was pending rather than leaving it.
        await center.setPermission(.denied)
        let revoked = await reconciler.reconcile(.foreground)
        XCTAssertEqual(revoked.outcome, .permissionDenied)
        XCTAssertEqual(revoked.removed.count, 3)
    }

    func testPlanFailureLeavesPendingRemindersAlone() async throws {
        let center = FakeNotificationCenter()
        let engine = engine()
        let reconciler = reconciler(center: center, engine: engine, store: try store())
        await reconciler.reconcile(.launch)
        engine.failure = "the workspace is being migrated"
        let failed = await reconciler.reconcile(.localChange)
        XCTAssertEqual(failed.outcome, .planFailed)
        XCTAssertEqual(failed.error, "the workspace is being migrated")
        let ids = await center.reminderIDs
        XCTAssertEqual(ids.count, 3)
    }

    func testClosedOrSwitchedWorkspaceRemovesItsReminders() async throws {
        let center = FakeNotificationCenter()
        let engine = engine()
        let reconciler = reconciler(center: center, engine: engine, store: try store())
        await reconciler.reconcile(.launch)

        engine.workspaceID = "workspace-b"
        let switched = await reconciler.reconcile(.launch)
        XCTAssertEqual(switched.removed.count, 3)
        XCTAssertEqual(switched.added.count, 3)
        let ids = await center.reminderIDs
        XCTAssertTrue(ids.allSatisfy { $0.hasPrefix("tt.reminder/workspace-b/") })

        engine.workspaceID = nil
        let closed = await reconciler.reconcile(.launch)
        XCTAssertEqual(closed.outcome, .noWorkspace)
        let none = await center.reminderIDs
        XCTAssertTrue(none.isEmpty)
    }

    func testAFailedAddIsReportedAndTheRestScheduled() async throws {
        let center = FakeNotificationCenter()
        await center.failAdding("tt.reminder/workspace-a/deadline/home/report")
        let reconciler = reconciler(center: center, engine: engine(), store: try store())
        let report = await reconciler.reconcile(.launch)
        XCTAssertEqual(report.outcome, .scheduled)
        XCTAssertEqual(report.added.count, 2)
        XCTAssertEqual(Array(report.failed.keys), ["tt.reminder/workspace-a/deadline/home/report"])
        let ids = await center.reminderIDs
        XCTAssertEqual(ids.count, 2)
        // The next reconciliation tries it again.
        await center.allowAdding()
        let retried = await reconciler.reconcile(.foreground)
        XCTAssertEqual(retried.added, ["tt.reminder/workspace-a/deadline/home/report"])
        XCTAssertEqual(retried.kept.count, 2)
    }

    func testSettingsPersistAsMachineLocalJSON() throws {
        let store = ReminderSettingsStore(file: directory.appendingPathComponent("nested/settings.json"))
        XCTAssertEqual(store.load(), .default)
        try store.save(ReminderSettings(enabled: true, checkInHour: 7, checkInMinute: 45))
        XCTAssertEqual(store.load().checkInTime, "07:45")
        let plan = try JSONDecoder.bridge.decode(ReminderPlan.self, from: Data(Self.bridgePlan.utf8))
        XCTAssertEqual(plan.reminders.first?.kind, .checkIn)
        XCTAssertEqual(plan.reminders.first?.task, bank)
        XCTAssertEqual(plan.reminders.first?.fireAt, utc("2026-10-07T13:00:00Z"))
    }

    /// One reminder in the bridge's wire form.
    static let bridgePlan = """
    {"time_zone":"America/Toronto","check_in_time":"09:00","generated_at":"2026-10-06T09:00:00Z","truncated":false,
     "reminders":[{"id":"check_in/home/bank","kind":"check_in","task":{"project_id":"home","task_id":"bank"},
     "project_name":"Home","short_id":4,"title":"Call the bank","fire_at":"2026-10-07T13:00:00Z",
     "local_time":"2026-10-07T09:00:00","can_acknowledge":true,"can_defer":true,"fingerprint":"0011"}]}
    """
}

/// A mutable zone the reconciler reads, as the device's would change.
final class ZoneBox: @unchecked Sendable {
    var current: TimeZone
    init(_ identifier: String) { current = TimeZone(identifier: identifier)! }
}

extension JSONDecoder {
    static var bridge: JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }
}
