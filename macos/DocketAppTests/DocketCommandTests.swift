import AppKit
import XCTest
@testable import Docket

final class DocketOverdueTests: XCTestCase {
    /// One line per overdue row; blank lines are not rows.
    func testTheCountIsTheNonBlankLines() {
        XCTAssertEqual(DocketOverdue.count(in: ""), 0)
        XCTAssertEqual(DocketOverdue.count(in: "\n  \n"), 0)
        XCTAssertEqual(DocketOverdue.count(in: "Pay rent @2026-11-01\n"), 1)
        XCTAssertEqual(DocketOverdue.count(in: "a\n\nb\r\n  \nc"), 3)
        XCTAssertEqual(DocketOverdue.arguments, ["ls", "@overdue"])
    }

    func testTheBadgeShowsOnlyAPositiveCount() {
        XCTAssertNil(DocketOverdue.label(nil))
        XCTAssertNil(DocketOverdue.label(0))
        XCTAssertEqual(DocketOverdue.label(1), "1")
        XCTAssertEqual(DocketOverdue.label(42), "42")
    }

    func testTheSummarySaysNothingOverdueButNothingWhenUnknown() {
        XCTAssertNil(DocketOverdue.summary(nil))
        XCTAssertEqual(DocketOverdue.summary(0), "Nothing overdue")
        XCTAssertEqual(DocketOverdue.summary(3), "3 overdue")
    }
}

final class DocketCommandResultTests: XCTestCase {
    func testTheFailureIsDocketsFirstLineWithoutItsPrefix() {
        let result = DocketCommandResult(status: 1, stdout: "", stderr: "\n docket: unknown project %nope\nusage: …\n")
        XCTAssertFalse(result.succeeded)
        XCTAssertEqual(result.failure, "unknown project %nope")
        XCTAssertEqual(DocketCommandResult(status: 2, stdout: "", stderr: "something else").failure, "something else")
        XCTAssertEqual(DocketCommandResult(status: 2, stdout: "", stderr: "a docket: in the middle").failure,
                       "a docket: in the middle", "only a leading prefix is dropped")
    }

    func testASilentFailureNamesItsStatus() {
        XCTAssertEqual(DocketCommandResult(status: 4, stdout: "out", stderr: " \n").failure, "Docket exited with status 4.")
    }

    func testARunThatTimedOutSaysSo() {
        let result = DocketCommandResult(status: nil, stdout: "", stderr: "docket: ignored")
        XCTAssertFalse(result.succeeded)
        XCTAssertEqual(result.failure, "Docket did not answer in time.")
        XCTAssertTrue(DocketCommandResult(status: 0, stdout: "", stderr: "").succeeded)
    }
}

final class DocketCommandTests: XCTestCase {
    func testARunGetsTheTargetsEnvironmentAndBothStreams() {
        let target = DocketCommandTarget(binary: "/bin/sh", environment: ["DOCKET_DB": "/tmp/x.db"])
        let result = DocketCommand.runNow(
            target, arguments: ["-c", #"printf '%s' "$DOCKET_DB"; echo 'docket: bad line' >&2; exit 3"#], timeout: 10
        )
        XCTAssertEqual(result.status, 3)
        XCTAssertEqual(result.stdout, "/tmp/x.db")
        XCTAssertEqual(result.failure, "bad line")
    }

    func testARunPastItsTimeoutIsEnded() {
        let started = Date()
        let result = DocketCommand.runNow(
            DocketCommandTarget(binary: "/bin/sleep", environment: [:]), arguments: ["10"], timeout: 0.2
        )
        XCTAssertNil(result.status)
        XCTAssertLessThan(Date().timeIntervalSince(started), 5)
    }

    func testABinaryThatCannotStartFails() {
        let result = DocketCommand.runNow(
            DocketCommandTarget(binary: "/no/such/docket", environment: [:]), arguments: [], timeout: 1
        )
        XCTAssertFalse(result.succeeded)
        XCTAssertFalse(result.failure.isEmpty)
    }

    func testTheAsyncRunAnswersOnTheMainThread() {
        let answered = expectation(description: "answered")
        DocketCommand.run(DocketCommandTarget(binary: "/bin/echo", environment: [:]), arguments: ["hi"], timeout: 10) { result in
            XCTAssertTrue(Thread.isMainThread)
            XCTAssertEqual(result.stdout, "hi\n")
            answered.fulfill()
        }
        wait(for: [answered], timeout: 10)
    }
}

final class DocketOverdueMonitorTests: XCTestCase {
    private let target = DocketCommandTarget(binary: "/usr/local/bin/docket", environment: [:])

    private func monitor(
        published: @escaping (Int?) -> Void,
        run: @escaping (DocketCommandTarget, @escaping (DocketCommandResult) -> Void) -> Void
    ) -> DocketOverdueMonitor {
        let monitor = DocketOverdueMonitor()
        let target = self.target
        monitor.target = { target }
        monitor.onChange = published
        monitor.run = run
        return monitor
    }

    func testARefreshPublishesTheCountOnlyWhenItChanges() {
        var published: [Int?] = []
        var output = "a\nb\n"
        var asked: [DocketCommandTarget] = []
        let monitor = monitor(published: { published.append($0) }) { target, done in
            asked.append(target)
            done(DocketCommandResult(status: 0, stdout: output, stderr: ""))
        }
        monitor.refresh()
        XCTAssertEqual(monitor.count, 2)
        monitor.refresh()
        output = ""
        monitor.refresh()
        XCTAssertEqual(published, [2, 0])
        XCTAssertEqual(monitor.count, 0)
        XCTAssertEqual(asked, [target, target, target])
    }

    func testAFailedRunLeavesTheCountUnknown() {
        var published: [Int?] = []
        var status: Int32? = 0
        let monitor = monitor(published: { published.append($0) }) { _, done in
            done(DocketCommandResult(status: status, stdout: "a\n", stderr: "docket: database is locked"))
        }
        monitor.refresh()
        status = 1
        monitor.refresh()
        XCTAssertNil(monitor.count)
        status = 0
        monitor.refresh()
        status = nil
        monitor.refresh()
        XCTAssertEqual(published, [1, nil, 1, nil], "a timeout is a failure too")
    }

    func testTurnedOffItAsksNothingAndClearsTheCount() {
        var published: [Int?] = []
        var runs = 0
        var enabled = true
        let monitor = monitor(published: { published.append($0) }) { _, done in
            runs += 1
            done(DocketCommandResult(status: 0, stdout: "a\n", stderr: ""))
        }
        monitor.isEnabled = { enabled }
        monitor.refresh()
        enabled = false
        monitor.refresh()
        monitor.refresh()
        XCTAssertNil(monitor.count)
        XCTAssertEqual(published, [1, nil])
        XCTAssertEqual(runs, 1)
    }

    func testWithNoDocketResolvedItAsksNothing() {
        var runs = 0
        let monitor = monitor(published: { _ in XCTFail("nothing to publish") }) { _, _ in runs += 1 }
        monitor.target = { nil }
        monitor.refresh()
        XCTAssertEqual(runs, 0)
        XCTAssertNil(monitor.count)
    }

    /// One run at a time: refreshes asked for meanwhile become a single run after it.
    func testRefreshesDuringARunBecomeOneMore() {
        var pending: [(DocketCommandResult) -> Void] = []
        var published: [Int?] = []
        let monitor = monitor(published: { published.append($0) }) { _, done in pending.append(done) }
        monitor.refresh()
        monitor.refresh()
        monitor.refresh()
        XCTAssertEqual(pending.count, 1)
        pending[0](DocketCommandResult(status: 0, stdout: "a\n", stderr: ""))
        XCTAssertEqual(pending.count, 2)
        pending[1](DocketCommandResult(status: 0, stdout: "a\nb\n", stderr: ""))
        XCTAssertEqual(pending.count, 2)
        XCTAssertEqual(published, [1, 2])
    }
}

final class QuickCapturePanelTests: XCTestCase {
    private var panel: QuickCapturePanel!

    override func setUp() {
        panel = QuickCapturePanel()
    }

    override func tearDown() {
        panel.hide()
        panel = nil
    }

    func testASentLineClearsTheFieldAndSaysWhereItWent() {
        var sent: [String] = []
        var captured = 0
        panel.submit = { line, done in
            sent.append(line)
            done(true, "Added to Inbox")
        }
        panel.onCaptured = { captured += 1 }
        panel.show()
        panel.text = "  Call Sam #home  "
        panel.send()
        XCTAssertEqual(sent, ["Call Sam #home"])
        XCTAssertEqual(panel.text, "")
        XCTAssertEqual(panel.statusText, "Added to Inbox")
        XCTAssertFalse(panel.isSending)
        XCTAssertEqual(captured, 1)
    }

    func testARefusedLineStaysWithDocketsReason() {
        var captured = 0
        panel.submit = { _, done in done(false, "unknown project %nope") }
        panel.onCaptured = { captured += 1 }
        panel.show()
        panel.text = "Call Sam %nope"
        panel.send()
        XCTAssertEqual(panel.text, "Call Sam %nope")
        XCTAssertEqual(panel.statusText, "unknown project %nope")
        XCTAssertFalse(panel.isSending)
        XCTAssertTrue(panel.isVisible)
        XCTAssertEqual(captured, 0)
    }

    func testAnEmptyLinePutsThePanelAway() {
        var sent = 0
        panel.submit = { _, done in
            sent += 1
            done(true, "")
        }
        panel.show()
        XCTAssertTrue(panel.isVisible)
        panel.text = "  \n "
        panel.send()
        XCTAssertFalse(panel.isVisible)
        XCTAssertEqual(sent, 0)
    }

    /// A second Enter while a line is on its way sends nothing more.
    func testALineOnItsWayIsSentOnce() {
        var pending: [(Bool, String) -> Void] = []
        panel.submit = { _, done in pending.append(done) }
        panel.show()
        panel.text = "Call Sam"
        panel.send()
        XCTAssertTrue(panel.isSending)
        panel.send()
        XCTAssertEqual(pending.count, 1)
        pending[0](true, "")
        XCTAssertEqual(panel.statusText, "Added.")
    }

    func testOpeningSaysWhyNothingCanBeSent() {
        panel.unavailableReason = { "Docket could not be found." }
        panel.show()
        XCTAssertEqual(panel.statusText, "Docket could not be found.")
        panel.hide()
        panel.unavailableReason = { nil }
        panel.show()
        XCTAssertEqual(panel.statusText, QuickCapturePanel.hint)
    }
}
