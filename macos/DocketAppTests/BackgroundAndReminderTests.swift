import XCTest
@testable import Docket

final class ScheduleWireTests: XCTestCase {
    /// A schedule event lists each arrival's fields beside its instant; an
    /// entry the app cannot read costs only that reminder.
    func testAScheduleEventDecodesAndSkipsAnUnreadableEntry() throws {
        let line = #"{"event":"schedule","schedule":{"items":["#
            + #"{"kind":"check-in","project":"p","task":"t","title":"Call","reviewable":true,"at":"2040-03-05T00:00:00-06:00"},"#
            + #"{"kind":"wake","project":"p","task":"u","title":"Bad","at":"tomorrow"},"#
            + #"{"kind":"deadline","project":"p","task":"v","title":"Ship","at":"2040-03-05T09:30:00Z"}"#
            + #"]}}"#
        guard case .event(.schedule(let items)) = DocketControlLine.decode(Data(line.utf8)) else {
            return XCTFail("not a schedule")
        }
        XCTAssertEqual(items.map(\.arrival.task), ["t", "v"])
        XCTAssertEqual(items[0].arrival, DocketArrival(kind: .checkIn, project: "p", task: "t", title: "Call", reviewable: true))
        XCTAssertEqual(items[0].at, try XCTUnwrap(ISO8601DateFormatter().date(from: "2040-03-05T06:00:00Z")))
        XCTAssertEqual(items[1].arrival.kind, .deadline)
    }

    func testAnEmptyScheduleDecodesAsNothingToBook() {
        let line = #"{"event":"schedule","schedule":{"items":[]}}"#
        XCTAssertEqual(DocketControlLine.decode(Data(line.utf8)), .event(.schedule([])))
    }
}

final class ReminderBookTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 2_000_000_000)

    private func item(_ task: String, kind: DocketArrival.Kind = .wake, in seconds: TimeInterval) -> DocketScheduledArrival {
        DocketScheduledArrival(
            DocketArrival(kind: kind, project: "p", task: task, title: "Task \(task)", reviewable: kind == .checkIn),
            at: now.addingTimeInterval(seconds)
        )
    }

    /// Only what is still to come is booked, each once, and no more than
    /// macOS keeps pending.
    func testBooksWhatIsToComeOnceAndWithinTheLimit() {
        let book = DocketReminderBook(schedule: [item("past", in: -60), item("a", in: 60), item("a", in: 60), item("b", in: 120)], now: now)
        XCTAssertEqual(book.bookings.map(\.arrival.task), ["a", "b"])

        let many = (0..<80).map { item("t\($0)", in: TimeInterval(60 * ($0 + 1))) }
        XCTAssertEqual(DocketReminderBook(schedule: many, now: now).bookings.count, DocketReminderBook.limit)
    }

    /// An identifier names the kind, the task, and the instant, so an unchanged
    /// arrival keeps its booking and a moved one is a new booking.
    func testIdentifiersAreStableAndMoveWithTheInstant() {
        let first = DocketReminderBook.identifier(for: item("a", in: 60))
        XCTAssertEqual(first, DocketReminderBook.identifier(for: item("a", in: 60)))
        XCTAssertNotEqual(first, DocketReminderBook.identifier(for: item("a", in: 120)))
        XCTAssertNotEqual(first, DocketReminderBook.identifier(for: item("a", kind: .deadline, in: 60)))
        XCTAssertTrue(first.hasPrefix(DocketReminderBook.prefix))
    }

    /// A new schedule withdraws only Docket's own bookings it no longer names
    /// and adds only what is not already pending.
    func testChangesWithdrawTheStaleAndAddTheMissing() {
        let kept = item("kept", in: 60)
        let added = item("added", in: 120)
        let book = DocketReminderBook(schedule: [kept, added], now: now)
        let stale = DocketReminderBook.identifier(for: item("gone", in: 90))
        let pending: Set<String> = [DocketReminderBook.identifier(for: kept), stale, "some-live-notification"]
        let changes = book.changes(pending: pending)
        XCTAssertEqual(changes.remove, [stale])
        XCTAssertEqual(changes.add.map(\.arrival.task), ["added"])

        XCTAssertEqual(DocketReminderBook().changes(pending: pending).remove.sorted(),
                       [DocketReminderBook.identifier(for: kept), stale].sorted(),
                       "an empty schedule withdraws every booking and nothing else")
    }

    /// A live arrival is left out when a booking for the same task and reason
    /// came due around it, and shown when none did.
    func testALiveArrivalIsCoveredOnlyByAMatchingBookingThatCameDue() {
        let booked = item("a", kind: .checkIn, in: 0)
        let bookings = DocketReminderBook(schedule: [booked], now: now.addingTimeInterval(-60)).bookings
        let arrival = booked.arrival
        XCTAssertTrue(DocketReminderBook.covers(arrival, bookings: bookings, now: now.addingTimeInterval(45)))
        XCTAssertTrue(DocketReminderBook.covers(arrival, bookings: bookings, now: now.addingTimeInterval(3 * 3600)),
                      "a Mac that slept through the instant shows the booking on waking")
        XCTAssertFalse(DocketReminderBook.covers(arrival, bookings: bookings, now: now.addingTimeInterval(-600)),
                       "a booking still well ahead has not shown anything yet")
        XCTAssertFalse(DocketReminderBook.covers(arrival, bookings: bookings, now: now.addingTimeInterval(DocketReminderBook.coverAfter + 60)))
        let otherReason = DocketArrival(kind: .wake, project: "p", task: "a", title: "Task a", reviewable: false)
        XCTAssertFalse(DocketReminderBook.covers(otherReason, bookings: bookings, now: now))
    }

    /// The notifications a live batch becomes leave out what a booking showed,
    /// and word a thinned batch afresh rather than with Docket's count of all.
    func testCoveredArrivalsAreLeftOutAndAThinnedBatchIsSummarizedAgain() {
        let covered = (0..<2).map { item("c\($0)", in: 0) }
        let bookings = DocketReminderBook(schedule: covered, now: now.addingTimeInterval(-60)).bookings
        let fresh = (0..<4).map { DocketArrival(kind: .deadline, project: "p", task: "d\($0)", title: "D\($0)", reviewable: false) }
        let arrivals = covered.map(\.arrival) + fresh

        let contents = NotificationPresenter.contents(
            for: arrivals, title: "Docket", body: "4 deadlines passed · 2 deferrals ended", covered: bookings, now: now
        )
        XCTAssertEqual(contents.count, 1)
        XCTAssertEqual(contents[0].body, "4 deadlines passed")

        let few = NotificationPresenter.contents(
            for: Array(arrivals.prefix(3)), title: "Docket", body: "unused", covered: bookings, now: now
        )
        XCTAssertEqual(few.map(\.body), ["D0"], "one uncovered arrival still names its task")

        let none = NotificationPresenter.contents(for: covered.map(\.arrival), title: "Docket", body: "2", covered: bookings, now: now)
        XCTAssertTrue(none.isEmpty)

        let untouched = NotificationPresenter.contents(for: fresh, title: "Docket", body: "4 deadlines passed", covered: [], now: now)
        XCTAssertEqual(untouched.map(\.body), ["4 deadlines passed"], "an unthinned batch keeps Docket's own wording")
    }

    func testTheSummaryMatchesDocketsWording() {
        let arrivals = [
            DocketArrival(kind: .wake, project: "p", task: "a", title: "A", reviewable: false),
            DocketArrival(kind: .checkIn, project: "p", task: "b", title: "B", reviewable: true),
            DocketArrival(kind: .checkIn, project: "p", task: "c", title: "C", reviewable: true),
        ]
        let summary = NotificationPresenter.summary(for: arrivals)
        XCTAssertEqual(summary.title, "Docket")
        XCTAssertEqual(summary.body, "2 check-ins · 1 deferral ended")
    }
}

final class BackgroundSessionTests: XCTestCase {
    private var suite = ""
    private var defaults: UserDefaults!

    override func setUpWithError() throws {
        suite = "docket-tests-\(UUID().uuidString)"
        defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suite)
    }

    func testThePreferenceDefaultsOnAndStartsHeadlessOnTheWindowsSocket() {
        XCTAssertTrue(DocketBackgroundPreference.load(from: defaults))
        DocketBackgroundPreference.save(false, to: defaults)
        XCTAssertFalse(DocketBackgroundPreference.load(from: defaults))
        XCTAssertEqual(DocketBackgroundPreference.arguments(controlPath: "/tmp/s"), ["--control", "/tmp/s", "--headless"])
        XCTAssertFalse(PreferencesModel(defaults: defaults).backgroundSession)
    }

    func testABinaryRunsHeadlessOnlyWhenItsHelpListsTheFlag() throws {
        let resolution = DocketBinaryResolver.Resolution(url: URL(fileURLWithPath: "/usr/local/bin/docket"), source: .path)
        XCTAssertFalse(DocketSessionBinary(resolution: resolution, supported: ["--control"]).headlessSupported)
        XCTAssertTrue(DocketSessionBinary(resolution: resolution, supported: ["--control", "--headless"]).headlessSupported)
    }

    /// Stopping ends the process and answers every caller once it has; a
    /// second start while one runs starts nothing.
    func testStopEndsTheProcessAndAnswersEveryCaller() {
        let session = DocketBackgroundSession()
        var unexpected = false
        session.onUnexpectedExit = { _, _ in unexpected = true }
        XCTAssertTrue(session.start(executable: "/bin/sleep", arguments: ["30"], environment: [:]))
        XCTAssertTrue(session.isRunning)
        XCTAssertTrue(session.start(executable: "/bin/sleep", arguments: ["30"], environment: [:]))

        let first = expectation(description: "first stop")
        let second = expectation(description: "second stop")
        session.stop { first.fulfill() }
        session.stop { second.fulfill() }
        wait(for: [first, second], timeout: 5)
        XCTAssertFalse(session.isRunning)
        XCTAssertFalse(unexpected, "an exit stop asked for is not unexpected")

        let idle = expectation(description: "idle stop")
        session.stop { idle.fulfill() }
        wait(for: [idle], timeout: 1)
    }

    /// A process that ignores SIGTERM is killed once the stop has waited.
    func testAStopThatIsIgnoredEndsInAKill() {
        let session = DocketBackgroundSession()
        session.stopTimeout = 0.3
        XCTAssertTrue(session.start(executable: "/bin/sh", arguments: ["-c", "trap '' TERM; sleep 30"], environment: [:]))
        // Let the shell install its trap before asking it to stop.
        RunLoop.main.run(until: Date().addingTimeInterval(0.3))
        let stopped = expectation(description: "stopped")
        session.stop { stopped.fulfill() }
        wait(for: [stopped], timeout: 5)
        XCTAssertFalse(session.isRunning)
    }

    /// An exit nobody asked for is reported with its status and what the
    /// process wrote to standard error, and the session can start again.
    func testAnUnexpectedExitIsReportedWithItsReason() {
        let session = DocketBackgroundSession()
        let exited = expectation(description: "exited")
        session.onUnexpectedExit = { status, error in
            XCTAssertEqual(status, 3)
            XCTAssertEqual(error, "docket is already running")
            exited.fulfill()
        }
        XCTAssertTrue(session.start(executable: "/bin/sh", arguments: ["-c", "echo 'docket is already running' >&2; sleep 0.2; exit 3"], environment: [:]))
        wait(for: [exited], timeout: 5)
        XCTAssertFalse(session.isRunning)
    }

    func testAMissingExecutableStartsNothing() {
        let session = DocketBackgroundSession()
        XCTAssertFalse(session.start(executable: "/nonexistent/docket", arguments: [], environment: [:]))
        XCTAssertFalse(session.isRunning)
    }

    func testTheErrorTailKeepsTheEnd() {
        var tail = DocketErrorTail()
        tail.append(Data(String(repeating: "x", count: DocketErrorTail.limit).utf8))
        tail.append(Data("the reason\n".utf8))
        XCTAssertTrue(tail.text.hasSuffix("the reason"))
        XCTAssertLessThanOrEqual(tail.text.utf8.count, DocketErrorTail.limit)
    }
}

final class ControlClientHoldTests: XCTestCase {
    /// A request sent while the client holds reaches the Docket that answers
    /// after the next connect, never the one listening during the hold.
    func testARequestSentWhileHeldWaitsForTheNextConnect() {
        let path = "/tmp/ttc-\(UUID().uuidString.prefix(8)).sock"
        defer { unlink(path) }
        let client = DocketControlClient(path: path)
        client.hold()
        let replied = expectation(description: "reply")
        var answered = false
        client.send(.capture("held")) { reply in
            XCTAssertTrue(reply.ok)
            answered = true
            replied.fulfill()
        }
        let listener = ControlClientHoldTests.listen(at: path)
        XCTAssertGreaterThanOrEqual(listener, 0)
        defer { close(listener) }
        ControlClientHoldTests.answer(on: listener)
        RunLoop.main.run(until: Date().addingTimeInterval(0.6))
        XCTAssertFalse(answered, "a held client does not look for a Docket")
        client.connect()
        wait(for: [replied], timeout: 5)
        client.disconnect()
    }

    private static func listen(at path: String) -> Int32 {
        let listener = socket(AF_UNIX, SOCK_STREAM, 0)
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { raw in
            let bytes = Array(path.utf8)
            raw.copyBytes(from: bytes)
            raw[bytes.count] = 0
        }
        let bound = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.bind(listener, $0, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }
        guard bound == 0, Darwin.listen(listener, 1) == 0 else {
            close(listener)
            return -1
        }
        return listener
    }

    /// Answers every request on one connection with an OK.
    private static func answer(on listener: Int32) {
        Thread.detachNewThread {
            let connection = accept(listener, nil, nil)
            guard connection >= 0 else { return }
            var buffer = DocketLineBuffer()
            var chunk = [UInt8](repeating: 0, count: 4096)
            while true {
                let count = read(connection, &chunk, chunk.count)
                if count <= 0 { break }
                for line in buffer.append(Data(chunk[0..<count])) {
                    guard let object = try? JSONSerialization.jsonObject(with: line) as? [String: Any],
                          let id = object["id"] as? Int else { continue }
                    let reply = "{\"reply\":\(id),\"ok\":true}\n"
                    _ = reply.withCString { write(connection, $0, strlen($0)) }
                }
            }
            close(connection)
        }
    }
}
