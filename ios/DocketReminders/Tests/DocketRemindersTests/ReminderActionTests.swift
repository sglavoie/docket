import Foundation
import XCTest
@testable import DocketReminders

/// Notification actions routed to engine commands through a fake center's
/// own requests: acknowledging, deferring, and opening reach the task the
/// reminder named, in the workspace it was scheduled for, and a refusal is
/// recorded rather than thrown.
final class ReminderActionTests: XCTestCase {
    let task = ReminderTask(projectID: "home", taskID: "bank")
    var log: ReminderActivityLog!

    override func setUp() {
        log = ReminderActivityLog(file: FileManager.default.temporaryDirectory
            .appendingPathComponent("tt-reminder-activity-\(UUID().uuidString).json"))
    }

    override func tearDown() {
        try? FileManager.default.removeItem(at: log.file)
    }

    /// The userInfo the scheduler put on a check-in reminder.
    func scheduledUserInfo(kind: ReminderKind = .checkIn, workspace: String = "workspace-a") -> [String: String] {
        let reminder = PlannedReminder(
            id: "\(kind.rawValue)/home/bank", kind: kind, task: task, projectName: "Home", shortID: 4,
            title: "Call the bank", fireAt: Date(timeIntervalSince1970: 1_791_378_000), localTime: "2026-10-07T09:00:00",
            canAcknowledge: kind == .checkIn, canDefer: true, fingerprint: "f1"
        )
        return ReminderScheduler.request(for: reminder, workspaceID: workspace, zone: TimeZone(identifier: "America/Toronto")!).userInfo
    }

    func router(_ commands: FakeCommands) -> ReminderActionRouter {
        ReminderActionRouter(commands: commands, log: log, now: { Date(timeIntervalSince1970: 1_791_378_060) })
    }

    func testAcknowledgeRoutesToTheEngine() async {
        let commands = FakeCommands(workspace: "workspace-a")
        let activity = await router(commands).handle(actionIdentifier: ReminderAction.acknowledgeIdentifier, userInfo: scheduledUserInfo())
        XCTAssertEqual(activity?.result, .acknowledged)
        let acknowledged = await commands.acknowledged
        XCTAssertEqual(acknowledged, [task])
        XCTAssertEqual(log.entries().first?.summary, "Acknowledged “Call the bank”")
    }

    func testDeferActionsUseTheEngineDeferralGrammar() async {
        let commands = FakeCommands(workspace: "workspace-a")
        let router = router(commands)
        let hour = await router.handle(actionIdentifier: ReminderAction.deferHourIdentifier, userInfo: scheduledUserInfo(kind: .deadline))
        let tomorrow = await router.handle(actionIdentifier: ReminderAction.deferTomorrowIdentifier, userInfo: scheduledUserInfo(kind: .deferralEnds))
        XCTAssertEqual(hour?.result, .deferred)
        XCTAssertEqual(tomorrow?.result, .deferred)
        let deferred = await commands.deferred
        XCTAssertEqual(deferred.map(\.1), ["+1h", "tomorrow"])
        XCTAssertEqual(deferred.map(\.0), [task, task])
        XCTAssertEqual(log.entries().map(\.summary), [
            "Deferred “Call the bank” (until tomorrow)",
            "Deferred “Call the bank” (for an hour)",
        ])
    }

    func testOpeningShowsTheTaskAndWritesNothing() async {
        let commands = FakeCommands(workspace: "workspace-a")
        let activity = await router(commands).handle(actionIdentifier: ReminderAction.defaultIdentifier, userInfo: scheduledUserInfo())
        XCTAssertEqual(activity?.result, .opened)
        let shown = await commands.shown
        let writes = await commands.writes
        XCTAssertEqual(shown, [task])
        XCTAssertEqual(writes, 0)
    }

    func testAnotherWorkspacesReminderIsRefused() async {
        let commands = FakeCommands(workspace: "workspace-b")
        let router = router(commands)
        for identifier in [ReminderAction.acknowledgeIdentifier, ReminderAction.deferHourIdentifier, ReminderAction.defaultIdentifier] {
            let activity = await router.handle(actionIdentifier: identifier, userInfo: scheduledUserInfo())
            XCTAssertEqual(activity?.result, .refused)
        }
        let writes = await commands.writes
        let shown = await commands.shown
        XCTAssertEqual(writes, 0)
        XCTAssertTrue(shown.isEmpty)
    }

    func testAnEngineRefusalIsRecordedNotThrown() async {
        let commands = FakeCommands(workspace: "workspace-a")
        await commands.refuse(with: "task changed since it was read")
        let activity = await router(commands).handle(actionIdentifier: ReminderAction.acknowledgeIdentifier, userInfo: scheduledUserInfo())
        XCTAssertEqual(activity?.result, .refused)
        XCTAssertEqual(activity?.detail, "task changed since it was read")
        XCTAssertEqual(log.entries().first?.summary, "Could not act on “Call the bank”: task changed since it was read")
    }

    func testResponsesThisAppDidNotScheduleAreIgnored() async {
        let commands = FakeCommands(workspace: "workspace-a")
        let router = router(commands)
        let unknownAction = await router.handle(actionIdentifier: "com.example.snooze", userInfo: scheduledUserInfo())
        let foreign = await router.handle(actionIdentifier: ReminderAction.acknowledgeIdentifier, userInfo: ["other": "yes"])
        XCTAssertNil(unknownAction)
        XCTAssertNil(foreign)
        XCTAssertTrue(log.entries().isEmpty)
        let dismissed = await router.handle(actionIdentifier: ReminderAction.dismissIdentifier, userInfo: scheduledUserInfo())
        XCTAssertEqual(dismissed?.result, .dismissed)
        let writes = await commands.writes
        XCTAssertEqual(writes, 0)
    }

    func testCategoriesOfferOnlyWhatTheReminderAllows() async {
        let center = FakeNotificationCenter()
        await ReminderScheduler(center: center).registerCategories()
        let categories = await center.categories
        func actions(_ identifier: String) -> [String] {
            categories.first { $0.identifier == identifier }?.actions.map(\.title) ?? []
        }
        XCTAssertEqual(actions(ReminderScheduler.acknowledgeCategory), ["Acknowledge", "Defer 1 Hour", "Defer to Tomorrow"])
        XCTAssertEqual(actions(ReminderScheduler.deferCategory), ["Defer 1 Hour", "Defer to Tomorrow"])
        XCTAssertTrue(categories.flatMap(\.actions).allSatisfy { !$0.opensApp })
    }

    func testActivityLogKeepsTheMostRecentEntries() {
        for index in 0..<(ReminderActivityLog.capacity + 5) {
            log.append(ReminderActivity(at: Date(timeIntervalSince1970: Double(index)), result: .opened, kind: "check_in", title: "T\(index)", task: task))
        }
        let entries = log.entries()
        XCTAssertEqual(entries.count, ReminderActivityLog.capacity)
        XCTAssertEqual(entries.first?.title, "T\(ReminderActivityLog.capacity + 4)")
    }
}
