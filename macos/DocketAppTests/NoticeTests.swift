@testable import Docket
import XCTest

final class NoticeTests: XCTestCase {
    private static let utc: Calendar = {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "UTC")!
        return calendar
    }()

    /// 2026-11-20 at `hour`:`minute`, UTC.
    private static func at(_ hour: Int, _ minute: Int = 0, day: Int = 20) -> Date {
        utc.date(from: DateComponents(year: 2026, month: 11, day: day, hour: hour, minute: minute))!
    }

    private static func rows(_ count: Int, state: String = "open") -> [DocketRow] {
        (0..<count).map { DocketRow(text: "R\($0)", place: "P", state: state) }
    }

    // MARK: What each says

    func testPlanTheDaySaysNothingIsPickedWithWhatApplies() {
        XCTAssertEqual(
            DocketNoticePlan.body(picked: [], carried: Self.rows(4), dueToday: Self.rows(2), overdue: Self.rows(1)),
            "Nothing picked for Today · 2 due today · 1 overdue · 4 carried"
        )
        XCTAssertEqual(
            DocketNoticePlan.body(picked: [], carried: [], dueToday: [], overdue: Self.rows(1)),
            "Nothing picked for Today · 1 overdue"
        )
        XCTAssertEqual(DocketNoticePlan.body(picked: [], carried: [], dueToday: [], overdue: []), "Nothing picked for Today")
    }

    func testPlanTheDayCountsWhatIsDueTodayWithNoCheck() {
        XCTAssertEqual(DocketNoticePlan.filters.last, "@today $unchecked -$today")
        XCTAssertEqual(DocketNotice.plan.filters.count, 5)
        XCTAssertEqual(
            DocketNoticePlan.body(
                picked: [], carried: [], dueToday: Self.rows(2), overdue: Self.rows(1), unchecked: Self.rows(1)),
            "Nothing picked for Today · 2 due today · 1 without a check · 1 overdue"
        )
        XCTAssertEqual(
            DocketNoticePlan.body(
                picked: Self.rows(3), carried: [], dueToday: Self.rows(1), overdue: [], unchecked: Self.rows(1)),
            "1 due today · 1 without a check"
        )
        // Nothing due today, nothing to check before it goes.
        XCTAssertNil(
            DocketNoticePlan.body(picked: Self.rows(3), carried: [], dueToday: [], overdue: [], unchecked: Self.rows(1)))
        // The monitor hands the fifth listing through.
        let answers = [[], [], Self.rows(1), [], Self.rows(1)]
        XCTAssertEqual(DocketNotice.plan.body(answers), "Nothing picked for Today · 1 due today · 1 without a check")
    }

    func testPlanTheDayWithADayPickedSaysOnlyWhatIsDueOrOverdue() {
        let picked = Self.rows(3)
        XCTAssertNil(DocketNoticePlan.body(picked: picked, carried: Self.rows(2), dueToday: [], overdue: []))
        XCTAssertEqual(
            DocketNoticePlan.body(picked: picked, carried: Self.rows(2), dueToday: Self.rows(1), overdue: Self.rows(2)),
            "1 due today · 2 overdue"
        )
    }

    func testCheckInCountsOpenPicksAndIsSilentWithNone() {
        let today = Self.rows(3) + Self.rows(1, state: "done") + Self.rows(1, state: "canceled")
        XCTAssertEqual(DocketNoticeCheckIn.body(today: today), "3 of 5 picks still open")
        XCTAssertEqual(DocketNoticeCheckIn.body(today: Self.rows(1)), "1 of 1 pick still open")
        XCTAssertNil(DocketNoticeCheckIn.body(today: Self.rows(2, state: "done")))
        XCTAssertNil(DocketNoticeCheckIn.body(today: []))
    }

    func testEachReadsItsOwnListingsInOrder() {
        XCTAssertEqual(DocketNotice.plan.filters.first, "$today -$missed")
        XCTAssertEqual(DocketNotice.checkIn.filters, ["$today"])
        XCTAssertEqual(
            DocketNotice.plan.body([[], Self.rows(4), Self.rows(2), Self.rows(1), []]),
            "Nothing picked for Today · 2 due today · 1 overdue · 4 carried"
        )
        XCTAssertNil(DocketNotice.plan.body([[]]), "a short answer says nothing")
    }

    // MARK: When each is due

    func testDueAtOrAfterItsTimeOnceADay() {
        let nine = DocketNoticeTime(hour: 9, minute: 0)
        XCTAssertFalse(DocketNotice.isDue(enabled: true, time: nine, lastFired: nil, now: Self.at(8, 59), calendar: Self.utc))
        XCTAssertTrue(DocketNotice.isDue(enabled: true, time: nine, lastFired: nil, now: Self.at(9), calendar: Self.utc))
        XCTAssertTrue(DocketNotice.isDue(enabled: true, time: nine, lastFired: "2026-11-19", now: Self.at(9, 1), calendar: Self.utc))
        XCTAssertFalse(DocketNotice.isDue(enabled: true, time: nine, lastFired: "2026-11-20", now: Self.at(9, 1), calendar: Self.utc))
    }

    func testALateReadCatchesUpTheSameDayOnly() {
        let nine = DocketNoticeTime(hour: 9, minute: 0)
        // Asleep from 8:00 to 17:30: the next read posts.
        XCTAssertTrue(DocketNotice.isDue(enabled: true, time: nine, lastFired: "2026-11-19", now: Self.at(17, 30), calendar: Self.utc))
        // A day that went by unread is not caught up the next morning before nine.
        XCTAssertFalse(DocketNotice.isDue(enabled: true, time: nine, lastFired: "2026-11-18", now: Self.at(7, day: 21), calendar: Self.utc))
    }

    func testSwitchedOffIsNeverDue() {
        XCTAssertFalse(DocketNotice.isDue(
            enabled: false, time: DocketNoticeTime(hour: 0, minute: 0), lastFired: nil, now: Self.at(23), calendar: Self.utc
        ))
    }

    // MARK: Settings

    func testPreferencesDefaultOnAtNineAndTwoAndKeepWhatIsSaved() throws {
        let defaults = try XCTUnwrap(UserDefaults(suiteName: "NoticeTests-\(UUID().uuidString)"))
        XCTAssertEqual(DocketNoticePreference.load(.plan, from: defaults), DocketNoticePreference(enabled: true, time: DocketNoticeTime(hour: 9, minute: 0)))
        XCTAssertEqual(DocketNoticePreference.load(.checkIn, from: defaults).time.text, "14:00")
        DocketNoticePreference(enabled: false, time: DocketNoticeTime(hour: 7, minute: 30)).save(.plan, to: defaults)
        XCTAssertEqual(defaults.string(forKey: "docketNoticePlanTime"), "07:30")
        XCTAssertEqual(defaults.object(forKey: "docketNoticePlan") as? Bool, false)
        XCTAssertEqual(DocketNoticePreference.load(.plan, from: defaults), DocketNoticePreference(enabled: false, time: DocketNoticeTime(hour: 7, minute: 30)))
        defaults.set("9am", forKey: "docketNoticeCheckInTime")
        XCTAssertEqual(DocketNoticePreference.load(.checkIn, from: defaults).time.text, "14:00")
        let model = PreferencesModel(defaults: defaults)
        XCTAssertFalse(model.noticePlan.enabled)
        model.noticeCheckIn.enabled = false
        XCTAssertEqual(defaults.object(forKey: "docketNoticeCheckIn") as? Bool, false)
    }

    // MARK: The monitor

    private final class Harness {
        let monitor = DocketNoticeMonitor()
        var clock = NoticeTests.at(9)
        var preferences: [DocketNotice: DocketNoticePreference] = [
            .plan: DocketNoticePreference(enabled: true, time: DocketNoticeTime(hour: 9, minute: 0)),
            .checkIn: DocketNoticePreference(enabled: true, time: DocketNoticeTime(hour: 14, minute: 0)),
        ]
        var answers: [String: [DocketRow]] = [:]
        var reads: [[String]] = []
        var asked = 0
        var permission = DocketNoticePermission.allowed
        var posted: [String] = []

        init() throws {
            monitor.defaults = try XCTUnwrap(UserDefaults(suiteName: "NoticeTests-\(UUID().uuidString)"))
            monitor.calendar = NoticeTests.utc
            monitor.now = { [unowned self] in self.clock }
            monitor.target = { DocketCommandTarget(binary: "/bin/false", environment: [:]) }
            monitor.preference = { [unowned self] in self.preferences[$0]! }
            monitor.read = { [unowned self] _, filters, done in
                self.reads.append(filters)
                done(filters.map { self.answers[$0] ?? [] })
            }
            monitor.authorize = { [unowned self] done in
                self.asked += 1
                done(self.permission)
            }
            monitor.post = { [unowned self] notice, body in self.posted.append(notice.title + ": " + body) }
        }
    }

    func testTheMonitorPostsEachOnceADayAndRunsNothingOtherwise() throws {
        let h = try Harness()
        h.clock = Self.at(8, 59)
        h.monitor.refresh()
        XCTAssertTrue(h.reads.isEmpty, "nothing due, nothing read")
        XCTAssertEqual(h.asked, 0, "macOS is not asked at launch")

        h.clock = Self.at(9)
        h.answers["$missed"] = Self.rows(4)
        h.monitor.refresh()
        h.monitor.refresh()
        XCTAssertEqual(h.posted, ["Plan the day: Nothing picked for Today · 4 carried"])
        XCTAssertEqual(h.reads.count, 1)
        XCTAssertEqual(h.asked, 1)

        h.clock = Self.at(14, 5)
        h.answers["$today"] = Self.rows(3) + Self.rows(2, state: "done")
        h.monitor.refresh()
        h.monitor.refresh()
        XCTAssertEqual(h.posted.last, "Check in: 3 of 5 picks still open")
        XCTAssertEqual(h.posted.count, 2)
        XCTAssertEqual(h.reads.count, 2)

        // The day turned over: each is due again at its time.
        h.clock = Self.at(9, 30, day: 21)
        h.monitor.refresh()
        XCTAssertEqual(h.posted.count, 3)
    }

    func testASilentNoticeIsSpentForTheDayWithoutAskingMacOS() throws {
        let h = try Harness()
        h.answers["$today -$missed"] = Self.rows(2)
        h.monitor.refresh()
        h.monitor.refresh()
        XCTAssertTrue(h.posted.isEmpty)
        XCTAssertEqual(h.reads.count, 1)
        XCTAssertEqual(h.asked, 0)
        XCTAssertEqual(h.monitor.lastFired(.plan), "2026-11-20")
    }

    func testALateLaunchPostsBothThatDayAndSwitchedOffRunsNothing() throws {
        let h = try Harness()
        h.clock = Self.at(16)
        h.answers["$today"] = Self.rows(1)
        h.monitor.refresh()
        XCTAssertEqual(h.posted, ["Plan the day: Nothing picked for Today", "Check in: 1 of 1 pick still open"])

        let off = try Harness()
        off.preferences[.plan]?.enabled = false
        off.preferences[.checkIn]?.enabled = false
        off.clock = Self.at(16)
        off.monitor.refresh()
        XCTAssertTrue(off.reads.isEmpty)
        XCTAssertTrue(off.posted.isEmpty)
    }

    func testRefusedPostsNothingAndSaysSo() throws {
        let h = try Harness()
        h.permission = .refused
        var told: [DocketNoticePermission] = []
        h.monitor.onPermission = { told.append($0) }
        h.monitor.refresh()
        XCTAssertTrue(h.posted.isEmpty)
        XCTAssertEqual(told, [.refused])
    }

    func testAFailedReadIsTriedAgainNextMinute() throws {
        let h = try Harness()
        var fail = true
        h.monitor.read = { _, filters, done in
            h.reads.append(filters)
            done(fail ? nil : filters.map { _ in [] })
        }
        h.monitor.refresh()
        XCTAssertNil(h.monitor.lastFired(.plan))
        fail = false
        h.clock = Self.at(9, 1)
        h.monitor.refresh()
        XCTAssertEqual(h.posted, ["Plan the day: Nothing picked for Today"])
    }

    func testAReadThatEndsOnTheNextDayPostsNothing() throws {
        let h = try Harness()
        h.clock = Self.at(23, 59)
        h.monitor.read = { _, filters, done in
            h.clock = NoticeTests.at(0, 0, day: 21)
            done(filters.map { _ in [] })
        }
        h.monitor.refresh()
        XCTAssertTrue(h.posted.isEmpty)
        XCTAssertNil(h.monitor.lastFired(.plan))
    }

    func testChoosingANoticeGoesToToday() {
        XCTAssertEqual(DocketGoTo.steps(for: DocketNotice.place), [.escape, .key("f"), .text("today"), .enter])
        XCTAssertEqual(DocketNotice(identifier: DocketNotice.checkIn.identifier), .checkIn)
        XCTAssertNil(DocketNotice(identifier: "something else"))
    }
}
