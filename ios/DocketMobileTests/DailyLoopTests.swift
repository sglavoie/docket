import Foundation
import XCTest
import DocketEngine
@testable import Docket

/// The daily loop's decisions: what the greeting proposes, what a review pass
/// visits, what a resume note may hold, where a triage swipe lands, which
/// capture tokens are colored, and the pulse read from the real engine.
final class DailyLoopTests: XCTestCase {
    private func date(hour: Int) -> Date {
        Calendar.current.date(bySettingHour: hour, minute: 0, second: 0, of: Date())!
    }

    func testTheGreetingNamesOneMoveInTheTerminalAppsOrder() {
        var pulse = DailyPulse(todayOpen: 2, doneToday: 1, inbox: 3, overdue: 1)
        XCTAssertEqual(pulse.move(now: date(hour: 9)), .overdue(1), "overdue outranks everything")
        pulse.overdue = 0
        XCTAssertEqual(pulse.move(now: date(hour: 9)), .startDay(3), "a morning Inbox is Start the Day")
        XCTAssertEqual(pulse.move(now: date(hour: 15)), .triage(3), "an afternoon Inbox is plain triage")
        pulse.inbox = 0
        XCTAssertEqual(pulse.move(now: date(hour: 15)), .plan(open: 2, done: 1))
        XCTAssertEqual(DailyPulse.sentence(.plan(open: 2, done: 1)), "1 done, 2 to go.")
        pulse.todayOpen = 0
        XCTAssertEqual(pulse.move(now: date(hour: 15)), .planDone(1))
        XCTAssertTrue(pulse.planComplete)
        XCTAssertEqual(pulse.progress, 1)
        XCTAssertEqual(DailyPulse().move(now: date(hour: 15)), .pickSomething)
        XCTAssertNil(DailyPulse().progress, "nothing planned or done measures nothing")
        XCTAssertEqual(DailyPulse.salutation(now: date(hour: 8)), "Good morning")
        XCTAssertEqual(DailyPulse.salutation(now: date(hour: 20)), "Good evening")
    }

    func testResumeNotesAreOneTrimmedLineWithinTheEnginesLimit() {
        XCTAssertEqual(ResumeNotePrompt.normalize("  call back\nafter lunch  "), "call back after lunch")
        XCTAssertEqual(ResumeNotePrompt.normalize(String(repeating: "é", count: 300)).unicodeScalars.count, 240)
        XCTAssertEqual(ResumeNotePrompt.normalize("   "), "")
    }

    func testTriageSwipesLandOnlyPastTheThreshold() {
        XCTAssertNil(TriageView.choice(for: CGSize(width: 60, height: 0)))
        XCTAssertEqual(TriageView.choice(for: CGSize(width: 140, height: 10)), .today)
        XCTAssertEqual(TriageView.choice(for: CGSize(width: -140, height: 10)), .backlog)
        XCTAssertEqual(TriageView.choice(for: CGSize(width: 20, height: -160)), .next)
        XCTAssertEqual(TriageView.choice(for: CGSize(width: 150, height: -120)), .today, "a mostly sideways throw is sideways")
    }

    func testCaptureHighlightingFollowsTheParsersWordBoundaries() {
        XCTAssertEqual(CaptureHighlighter.tokens(in: "Call Sam #work @tomorrow !1 %home ##phone"),
                       ["#work", "@tomorrow", "!1", "%home", "##phone"])
        XCTAssertEqual(CaptureHighlighter.tokens(in: "R&D budget; then review"), [], "mid-word and lone sigils stay prose")
        XCTAssertEqual(CaptureHighlighter.tokens(in: "Fix it ;see the log; -#old"), [";see the log;", "-#old"])
        XCTAssertEqual(CaptureHighlighter.tokens(in: "first\n#second"), ["#second"], "a token may start a line")
    }

    func testReviewPassVisitsFlaggedWorkAndArrivedCheckInsOnly() throws {
        let now = Date()
        let json = """
        {"project_id":"p","task_id":"t","short_id":1,"title":"T","state":"waiting","priority":0,"tags":[],
         "pinned":false,"review":false,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z",
         "state_changed_at":"2026-01-01T00:00:00Z"}
        """
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        var task = try decoder.decode(TaskSnapshot.self, from: Data(json.utf8))
        XCTAssertFalse(ReviewPass.needsAttention(task, now: now), "Waiting alone asks for nothing")
        task.review = true
        XCTAssertTrue(ReviewPass.needsAttention(task, now: now))
        task.review = false
        task.nextCheckIn = now.addingTimeInterval(-86_400)
        XCTAssertTrue(ReviewPass.needsAttention(task, now: now), "an arrived check-in asks")
        task.nextCheckIn = now.addingTimeInterval(3 * 86_400)
        XCTAssertFalse(ReviewPass.needsAttention(task, now: now), "a check-in still ahead does not")
        task.review = true
        task.state = .done
        XCTAssertFalse(ReviewPass.needsAttention(task, now: now), "finished work owes nothing")
    }

    /// The pulse as the real engine answers it: Today's open work, what was
    /// finished since midnight, the Inbox, and the task in progress.
    @MainActor func testThePulseReadsTheDayFromTheEngine() async throws {
        let engine = DocketEngine()
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try await engine.open(dataDirectory: root.appendingPathComponent("workspace"),
                              runtimeDirectory: root.appendingPathComponent("runtime"),
                              deviceID: "pulse-tests", createIfMissing: true)
        addTeardownBlock {
            try await engine.close()
            try FileManager.default.removeItem(at: root)
        }
        let first = try await engine.capture(title: "Plan the week").task
        let second = try await engine.capture(title: "Water plants").task
        _ = try await engine.capture(title: "Sort later").task
        _ = try await engine.plan(first.locator, .addToday)
        _ = try await engine.plan(second.locator, .addToday)
        _ = try await engine.transition(first.locator, to: .doing)

        var pulse = await DailyPulse.load(engine)
        XCTAssertEqual(pulse.todayOpen, 2)
        XCTAssertEqual(pulse.doneToday, 0)
        XCTAssertEqual(pulse.doing.map(\.title), ["Plan the week"])

        _ = try await engine.transition(second.locator, to: .done)
        pulse = await DailyPulse.load(engine)
        XCTAssertEqual(pulse.todayOpen, 1)
        XCTAssertEqual(pulse.doneToday, 1, "a task finished today counts toward the ring")
        XCTAssertEqual(pulse.progress, 0.5)
        XCTAssertGreaterThanOrEqual(pulse.inbox, 1)
    }
}
