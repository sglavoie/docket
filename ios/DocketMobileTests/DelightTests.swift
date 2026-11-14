import Foundation
import XCTest
import DocketEngine
@testable import Docket

/// The small decisions behind the app's moments: how a missed deadline is
/// worded, when Today offers Wrap Up, and how the week's rhythm is counted
/// from the real engine.
final class DelightTests: XCTestCase {
    private func date(_ day: Int, _ hour: Int = 0, _ minute: Int = 0) -> Date {
        TaskSchedule.calendar.date(from: DateComponents(year: 2030, month: 6, day: day, hour: hour, minute: minute))!
    }

    private func openEngine() async throws -> DocketEngine {
        let engine = DocketEngine()
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try await engine.open(dataDirectory: root.appendingPathComponent("workspace"),
                              runtimeDirectory: root.appendingPathComponent("runtime"),
                              deviceID: "delight-tests", createIfMissing: true)
        addTeardownBlock {
            try await engine.close()
            try FileManager.default.removeItem(at: root)
        }
        return engine
    }

    func testAMissedDeadlineSaysHowLongAgoInWords() {
        XCTAssertEqual(TaskSchedule.overdueSpan(date(10, 9, 0), now: date(10, 9, 25)), "by 25 min")
        XCTAssertEqual(TaskSchedule.overdueSpan(date(10, 9, 0), now: date(10, 12, 10)), "by 3 h")
        XCTAssertEqual(TaskSchedule.overdueSpan(date(10), now: date(11, 8)), "since yesterday")
        XCTAssertEqual(TaskSchedule.overdueSpan(date(1), now: date(5)), "by 4 days")
        XCTAssertEqual(TaskSchedule.overdueSpan(date(1), now: date(29)), "by 4 weeks")
        let spring = TaskSchedule.calendar.date(from: DateComponents(year: 2030, month: 2, day: 1))!
        XCTAssertEqual(TaskSchedule.overdueSpan(spring, now: date(10)), "by 4 months")
    }

    func testWrapUpIsOfferedFromFiveUntilTheSmallHours() {
        XCTAssertFalse(TodayHeaderView.isEvening(date(10, 16, 59), calendar: TaskSchedule.calendar))
        XCTAssertTrue(TodayHeaderView.isEvening(date(10, 17), calendar: TaskSchedule.calendar))
        XCTAssertTrue(TodayHeaderView.isEvening(date(10, 23, 30), calendar: TaskSchedule.calendar))
        XCTAssertTrue(TodayHeaderView.isEvening(date(10, 2), calendar: TaskSchedule.calendar))
        XCTAssertFalse(TodayHeaderView.isEvening(date(10, 4), calendar: TaskSchedule.calendar))
    }

    func testTheWeeksRhythmCountsTodaysCompletionsLast() async throws {
        let engine = try await openEngine()
        let first = try await engine.capture(title: "Write the brief").task
        let second = try await engine.capture(title: "Send the brief").task
        _ = try await engine.transition(first.locator, to: .done)
        _ = try await engine.transition(second.locator, to: .done)
        let recap = try await DayRecap.load(engine)
        XCTAssertEqual(recap.week.count, 7)
        XCTAssertEqual(recap.week.last?.done, 2)
        XCTAssertEqual(recap.doneToday.map(\.title), ["Write the brief", "Send the brief"])
        XCTAssertEqual(RecapView.headline(done: 2), "2 things done today.")
    }
}
