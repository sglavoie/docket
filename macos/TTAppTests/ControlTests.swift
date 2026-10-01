import XCTest
import UserNotifications
@testable import tt

/// The wire is `internal/control` in the Go tree; these pin the lines tt.app
/// writes and reads.
final class ControlWireTests: XCTestCase {
    func testRepliesAndEventsDecode() {
        XCTAssertEqual(
            TTControlLine.decode(Data(#"{"reply":3,"ok":true,"message":"Captured: Buy milk"}"#.utf8)),
            .reply(TTControlReply(reply: 3, ok: true, message: "Captured: Buy milk"))
        )
        XCTAssertEqual(
            TTControlLine.decode(Data(#"{"event":"pulse","pulse":{"inbox":1,"today":2,"overdue":3,"done":4}}"#.utf8)),
            .event(.pulse(TTPulse(inbox: 1, today: 2, overdue: 3, done: 4)))
        )
        let arrivals = #"{"event":"arrivals","title":"tt · check-in","body":"Call","arrivals":[{"kind":"check-in","project":"p","task":"t","title":"Call","reviewable":true}]}"#
        XCTAssertEqual(
            TTControlLine.decode(Data(arrivals.utf8)),
            .event(.arrivals(
                [TTArrival(kind: .checkIn, project: "p", task: "t", title: "Call", reviewable: true)],
                title: "tt · check-in", body: "Call"
            ))
        )
    }

    /// A later tt may add events; an older app skips what it does not know.
    func testUnknownLinesAreSkipped() {
        XCTAssertNil(TTControlLine.decode(Data(#"{"event":"something-new"}"#.utf8)))
        XCTAssertNil(TTControlLine.decode(Data("not json".utf8)))
    }

    func testRequestsEncodeTheGoSpelling() throws {
        var request = TTControlRequest.deferTask(TTTaskID(project: "p", task: "t"), minutes: 60)
        request.id = 9
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(request)) as? [String: Any])
        XCTAssertEqual(object["op"] as? String, "defer")
        XCTAssertEqual(object["minutes"] as? Int, 60)
        XCTAssertEqual(object["id"] as? Int, 9)
        XCTAssertNil(object["text"])
    }

    func testLineBufferKeepsAPartialLine() {
        var buffer = TTLineBuffer()
        XCTAssertEqual(buffer.append(Data("{\"a\":1}\n{\"b\"".utf8)), [Data("{\"a\":1}".utf8)])
        XCTAssertEqual(buffer.append(Data(":2}\n\n".utf8)), [Data("{\"b\":2}".utf8)])
    }

    /// A tt with a socket reports arrivals over it, so `--notify` is not passed;
    /// an older one still gets `--notify` when it knows the flag.
    func testLaunchArgumentsPreferTheSocket() {
        XCTAssertEqual(
            TTControlLaunch.arguments(controlPath: "/tmp/s", notificationsEnabled: true, notifySupported: true),
            ["--control", "/tmp/s"]
        )
        XCTAssertEqual(
            TTControlLaunch.arguments(controlPath: nil, notificationsEnabled: true, notifySupported: true),
            ["--notify"]
        )
        XCTAssertLessThan(TTControlLaunch.socketPath(pid: 99999).utf8.count, 104)
    }

    func testPulseBadgeIsTheOverdueCount() {
        XCTAssertEqual(TTPulseBadge.label(for: TTPulse(inbox: 1, today: 0, overdue: 4, done: 0)), "4")
        XCTAssertNil(TTPulseBadge.label(for: TTPulse(inbox: 5, today: 5, overdue: 0, done: 5)))
    }
}

final class ControlNotificationTests: XCTestCase {
    private func arrival(_ task: String, reviewable: Bool = false) -> TTArrival {
        TTArrival(kind: reviewable ? .checkIn : .wake, project: "p", task: task, title: "Task \(task)", reviewable: reviewable)
    }

    /// A few arrivals are one notification each, offering Acknowledge only
    /// where tt would accept it; more are tt's own summary, naming no task.
    func testArrivalsBecomePerTaskNotificationsOrOneSummary() throws {
        let few = NotificationPresenter.contents(for: [arrival("a", reviewable: true), arrival("b")], title: "tt", body: "2")
        XCTAssertEqual(few.map(\.title), ["tt · check-in", "tt · deferral ended"])
        XCTAssertEqual(few.map(\.body), ["Task a", "Task b"])
        XCTAssertEqual(few.map(\.categoryIdentifier), [
            NotificationPresenter.Category.reviewable, NotificationPresenter.Category.task,
        ])
        XCTAssertEqual(NotificationPresenter.taskID(from: few[0].userInfo), TTTaskID(project: "p", task: "a"))

        let many = NotificationPresenter.contents(
            for: (1...4).map { arrival("\($0)") }, title: "tt", body: "4 deferrals ended"
        )
        XCTAssertEqual(many.count, 1)
        XCTAssertEqual(many[0].body, "4 deferrals ended")
        XCTAssertNil(NotificationPresenter.taskID(from: many[0].userInfo))
    }

    func testActionsRouteToTheirTask() {
        let presenter = NotificationPresenter()
        var calls: [String] = []
        presenter.onActivate = { calls.append("activate") }
        presenter.onReveal = { calls.append("reveal \($0.task)") }
        presenter.onAcknowledge = { calls.append("ack \($0.task)") }
        presenter.onDefer = { calls.append("defer \($0.task)") }
        let task = TTTaskID(project: "p", task: "t")

        presenter.respond(to: UNNotificationDefaultActionIdentifier, task: task)
        presenter.respond(to: NotificationPresenter.Action.acknowledge, task: task)
        presenter.respond(to: NotificationPresenter.Action.deferHour, task: task)
        presenter.respond(to: UNNotificationDefaultActionIdentifier, task: nil)
        presenter.respond(to: UNNotificationDismissActionIdentifier, task: task)
        XCTAssertEqual(calls, ["reveal t", "ack t", "defer t", "activate"])
    }

    func testCategoriesOfferAcknowledgeOnlyWhenReviewable() throws {
        let categories = Dictionary(uniqueKeysWithValues: NotificationPresenter.categories().map { ($0.identifier, $0) })
        XCTAssertEqual(try XCTUnwrap(categories[NotificationPresenter.Category.reviewable]).actions.map(\.title),
                       ["Acknowledge", "Defer 1 Hour"])
        XCTAssertEqual(try XCTUnwrap(categories[NotificationPresenter.Category.task]).actions.map(\.title),
                       ["Defer 1 Hour"])
    }
}

final class CaptureHotKeyPreferenceTests: XCTestCase {
    func testDefaultsClearsAndRoundTrips() throws {
        let suite = "tt-tests-\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
        defer { defaults.removePersistentDomain(forName: suite) }

        XCTAssertEqual(TTCaptureHotKeyPreference.load(from: defaults)?.string, "ctrl+opt+cmd+a")
        TTCaptureHotKeyPreference.save(nil, to: defaults)
        XCTAssertNil(TTCaptureHotKeyPreference.load(from: defaults))

        let model = PreferencesModel(defaults: defaults)
        var announced: [HotKeyCombo?] = []
        model.onCaptureHotKeyChange = { announced.append($0) }
        model.captureHotKey = HotKeyCombo(string: "ctrl+opt+n")
        XCTAssertEqual(announced.map { $0?.string }, ["ctrl+opt+n"])
        XCTAssertEqual(TTCaptureHotKeyPreference.load(from: defaults)?.string, "ctrl+opt+n")
    }
}

/// The client against a stand-in tt: one accepted connection that answers each
/// request line and can push events.
final class ControlClientTests: XCTestCase {
    private var path = ""
    private var listener: Int32 = -1

    override func setUpWithError() throws {
        path = "/tmp/ttc-\(UUID().uuidString.prefix(8)).sock"
        listener = socket(AF_UNIX, SOCK_STREAM, 0)
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
        XCTAssertEqual(bound, 0)
        XCTAssertEqual(Darwin.listen(listener, 1), 0)
    }

    override func tearDown() {
        close(listener)
        unlink(path)
    }

    /// Accepts one connection, writes `greeting`, then answers each request line
    /// with `{"reply":ID,"ok":true,"message":OP}`.
    private func serve(greeting: String) {
        let listener = self.listener
        Thread.detachNewThread {
            let connection = accept(listener, nil, nil)
            guard connection >= 0 else { return }
            _ = greeting.withCString { write(connection, $0, strlen($0)) }
            var buffer = TTLineBuffer()
            var chunk = [UInt8](repeating: 0, count: 4096)
            while true {
                let count = read(connection, &chunk, chunk.count)
                if count <= 0 { break }
                for line in buffer.append(Data(chunk[0..<count])) {
                    guard let object = try? JSONSerialization.jsonObject(with: line) as? [String: Any],
                          let id = object["id"] as? Int, let op = object["op"] as? String else { continue }
                    let reply = "{\"reply\":\(id),\"ok\":true,\"message\":\"\(op)\"}\n"
                    _ = reply.withCString { write(connection, $0, strlen($0)) }
                }
            }
            close(connection)
        }
    }

    func testConnectsHearsEventsAndMatchesReplies() {
        serve(greeting: #"{"event":"pulse","pulse":{"inbox":0,"today":0,"overdue":2,"done":0}}"# + "\n")
        let client = TTControlClient(path: path)
        let pulse = expectation(description: "pulse")
        client.onEvent = { event in
            if case .pulse(let value) = event, value.overdue == 2 { pulse.fulfill() }
        }
        // Sent before the connection exists: it waits for it.
        let replied = expectation(description: "reply")
        client.connect()
        client.send(.capture("Buy milk")) { reply in
            XCTAssertTrue(reply.ok)
            XCTAssertEqual(reply.message, "capture")
            replied.fulfill()
        }
        wait(for: [pulse, replied], timeout: 5)
        client.disconnect()
    }

    func testARequestWithNoTtFails() {
        let client = TTControlClient(path: path + ".absent")
        let failed = expectation(description: "failure")
        client.send(.capture("x")) { reply in
            XCTAssertFalse(reply.ok)
            failed.fulfill()
        }
        wait(for: [failed], timeout: 2)
    }
}

final class QuickCapturePanelTests: XCTestCase {
    /// A tt without a control socket still opens the panel, which names the
    /// binary and what to do instead of sending anything.
    func testAnUnavailableTtIsExplainedInThePanelAndNothingIsSent() throws {
        let panel = QuickCapturePanel()
        let reason = TTControlLaunch.unsupportedReason(binary: NSHomeDirectory() + "/.local/bin/tt")
        XCTAssertTrue(reason.hasPrefix("~/.local/bin/tt predates --control"))
        panel.unavailableReason = { reason }
        var sent = false
        panel.submit = { _, _ in sent = true }

        panel.show()
        defer { panel.close() }
        let labels = try XCTUnwrap(panel.panel.contentView).descendants(of: NSTextField.self)
        XCTAssertTrue(labels.contains { $0.stringValue == reason })

        let field = try XCTUnwrap(labels.first { $0.isEditable })
        field.stringValue = "Buy milk"
        field.sendAction(field.action, to: field.target)
        XCTAssertFalse(sent)
    }
}

private extension NSView {
    func descendants<T: NSView>(of type: T.Type) -> [T] {
        subviews.flatMap { ($0 as? T).map { [$0] } ?? [] + $0.descendants(of: type) }
    }
}
