import AppKit
import XCTest
import UserNotifications
@testable import tt

/// The wire is `internal/control` in the Go tree; these pin the lines tt.app
/// writes and reads.
final class ControlWireTests: XCTestCase {
    func testCapturePreviewWireFormat() throws {
        let summary = "Project: Web · Plan: Today · Status: Backlog"
        let line = #"{"reply":7,"ok":true,"preview":"Project: Web · Plan: Today · Status: Backlog"}"#
        XCTAssertEqual(TTControlLine.decode(Data(line.utf8)),
                       .reply(TTControlReply(reply: 7, ok: true, message: nil, preview: summary)))
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(TTControlRequest.preview("Buy %web"))) as? [String: Any])
        XCTAssertEqual(object["op"] as? String, "preview")
        XCTAssertEqual(object["text"] as? String, "Buy %web")
    }

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

    func testACompletionReplyDecodesAndTheRequestEncodes() throws {
        let line = #"{"reply":4,"ok":true,"completion":{"token":"%cl","exact":true,"items":[{"label":"%Client Work","insert":"%client-work "}]}}"#
        XCTAssertEqual(
            TTControlLine.decode(Data(line.utf8)),
            .reply(TTControlReply(reply: 4, ok: true, message: nil, completion: TTCompletion(
                token: "%cl",
                items: [.init(label: "%Client Work", insert: "%client-work ")],
                exact: true
            )))
        )
        // Prose answers with no completion at all.
        XCTAssertEqual(
            TTControlLine.decode(Data(#"{"reply":5,"ok":true}"#.utf8)),
            .reply(TTControlReply(reply: 5, ok: true, message: nil))
        )
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(TTControlRequest.complete("Buy %cl"))) as? [String: Any])
        XCTAssertEqual(object["op"] as? String, "complete")
        XCTAssertEqual(object["text"] as? String, "Buy %cl")
    }

    func testAppearanceEncodesTheThemeTtTakes() throws {
        for scheme in [TTAppearanceScheme.dark, .light] {
            let data = try JSONEncoder().encode(TTControlRequest.appearance(scheme))
            let object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
            XCTAssertEqual(object["op"] as? String, "appearance")
            XCTAssertEqual(object["text"] as? String, scheme.rawValue)
            XCTAssertNil(object["project"])
        }
        XCTAssertEqual(TTAppearanceScheme(try XCTUnwrap(NSAppearance(named: .darkAqua))), .dark)
        XCTAssertEqual(TTAppearanceScheme(try XCTUnwrap(NSAppearance(named: .aqua))), .light)
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

    func testQuitEncodesTheGoSpelling() throws {
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(TTControlRequest.quit)) as? [String: Any])
        XCTAssertEqual(object["op"] as? String, "quit")
        XCTAssertNil(object["text"])
        XCTAssertNil(object["task"])
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

    func testPulseSummaryNamesWhatIsOwedAndOmitsZeros() {
        XCTAssertEqual(TTPulseSummary.line(for: TTPulse(inbox: 3, today: 2, overdue: 1, done: 9)),
                       "inbox 3 · today 2 · overdue 1")
        XCTAssertEqual(TTPulseSummary.line(for: TTPulse(inbox: 0, today: 4, overdue: 0, done: 0)), "today 4")
        XCTAssertNil(TTPulseSummary.line(for: TTPulse(inbox: 0, today: 0, overdue: 0, done: 7)))
    }
}

final class QuitHandshakeTests: XCTestCase {
    private func handshake() -> (TTQuitHandshake, () -> [TTQuitHandshake.Outcome]) {
        var outcomes: [TTQuitHandshake.Outcome] = []
        let handshake = TTQuitHandshake { outcomes.append($0) }
        return (handshake, { outcomes })
    }

    /// An OK means tt is on its way out: the app waits for it to end, and
    /// ends exactly once however many signals follow.
    func testAnOKWaitsForTTToEnd() {
        let (handshake, outcomes) = handshake()
        handshake.replied(TTControlReply(reply: 1, ok: true, message: "quitting"))
        XCTAssertEqual(outcomes(), [])
        handshake.ttEnded()
        handshake.ttEnded()
        handshake.replied(.failure("The connection to tt ended"))
        XCTAssertEqual(outcomes(), [.quit])
    }

    func testABusyRefusalAsksTheUser() {
        let (handshake, outcomes) = handshake()
        let busy = "tt is busy: finish what is open in tt first"
        handshake.replied(TTControlReply(reply: 1, ok: false, message: busy))
        XCTAssertEqual(outcomes(), [.busy(busy)])
    }

    /// An older tt does not know the request, and a dropped connection means
    /// tt is gone: either way the app ends and tt takes the hangup.
    func testATTThatCannotBeAskedQuits() {
        for reply in [
            TTControlReply(reply: 1, ok: false, message: #"unknown operation "quit""#),
            TTControlReply.failure("The connection to tt ended"),
            TTControlReply(reply: 1, ok: false, message: "tt did not answer in time"),
        ] {
            let (handshake, outcomes) = handshake()
            handshake.replied(reply)
            XCTAssertEqual(outcomes(), [.quit], "\(reply)")
        }
    }

    func testTheTimeoutQuits() {
        let quit = expectation(description: "timeout")
        let handshake = TTQuitHandshake { outcome in
            XCTAssertEqual(outcome, .quit)
            quit.fulfill()
        }
        handshake.replied(TTControlReply(reply: 1, ok: true, message: nil))
        handshake.start(timeout: 0.05)
        wait(for: [quit], timeout: 2)
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

    /// tt's OK reply may be lost to its exit: the connection ending finishes
    /// the quit as surely as the reply would.
    func testQuitFinishesWhenTheConnectionEnds() {
        let listener = self.listener
        Thread.detachNewThread {
            let connection = accept(listener, nil, nil)
            guard connection >= 0 else { return }
            var chunk = [UInt8](repeating: 0, count: 4096)
            _ = read(connection, &chunk, chunk.count)
            close(connection)
        }
        let client = TTControlClient(path: path)
        let quit = expectation(description: "quit")
        let handshake = TTQuitHandshake { outcome in
            XCTAssertEqual(outcome, .quit)
            quit.fulfill()
        }
        client.onConnectionChange = { connected in
            if !connected { handshake.ttEnded() }
        }
        client.connect()
        client.send(.quit) { reply in handshake.replied(reply) }
        wait(for: [quit], timeout: 5)
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
    func testControlNCapturesRepeatedlyAndKeepsRefusedDrafts() throws {
        let panel = QuickCapturePanel()
        var callbacks: [(Bool, String) -> Void] = []
        var submitted: [String] = []
        var dismissals = 0
        panel.scheduleDismissal = { _ in dismissals += 1 }
        panel.submit = { text, done in submitted.append(text); callbacks.append(done) }
        panel.show()
        defer { panel.close() }
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        let key = try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero,
            modifierFlags: .control, timestamp: 0, windowNumber: panel.panel.windowNumber,
            context: nil, characters: "\u{0e}", charactersIgnoringModifiers: "n", isARepeat: false, keyCode: 45))
        field.stringValue = "First"
        XCTAssertTrue(panel.panel.performKeyEquivalent(with: key))
        XCTAssertTrue(panel.panel.performKeyEquivalent(with: key))
        XCTAssertEqual(submitted, ["First"], "an in-flight request cannot be duplicated")
        callbacks[0](true, "Captured")
        XCTAssertEqual(field.stringValue, "")
        XCTAssertTrue(panel.isVisible)
        XCTAssertEqual(dismissals, 0)
        field.stringValue = "Second"
        XCTAssertTrue(panel.panel.performKeyEquivalent(with: key))
        callbacks[1](false, "Please fix this")
        XCTAssertEqual(field.stringValue, "Second")
        XCTAssertTrue(panel.isVisible)
        XCTAssertEqual(dismissals, 0)
        XCTAssertTrue(panel.panel.performKeyEquivalent(with: key))
        callbacks[2](true, "Captured")
        XCTAssertEqual(field.stringValue, "")
        XCTAssertEqual(submitted, ["First", "Second", "Second"])
    }

    func testReopeningAnInterruptedDraftRestoresCaretAndSelection() throws {
        let panel = QuickCapturePanel()
        panel.show()
        defer { panel.close() }
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        let draft = "Buy 🥛 milk"
        let ranges = [NSRange(location: 7, length: 0), NSRange(location: 4, length: 2)]
        for range in ranges {
            field.stringValue = draft
            let editor = try XCTUnwrap(field.currentEditor() as? NSTextView)
            editor.setSelectedRange(range)
            // Exercise both explicit cancellation and losing focus.
            if range.length == 0 {
                panel.close()
            } else {
                panel.windowDidResignKey(Notification(name: NSWindow.didResignKeyNotification))
            }
            panel.show()
            let reopened = try XCTUnwrap(field.currentEditor() as? NSTextView)
            XCTAssertEqual(reopened.string, draft)
            XCTAssertEqual(reopened.selectedRange(), range)
            reopened.insertText("fresh", replacementRange: reopened.selectedRange())
            XCTAssertEqual(field.stringValue, (draft as NSString).replacingCharacters(in: range, with: "fresh"))
        }
    }

    func testSuccessfulCaptureClearsTheRememberedSelection() throws {
        let panel = QuickCapturePanel()
        panel.scheduleDismissal = { _ in }
        panel.submit = { _, done in done(true, "Captured") }
        panel.show()
        defer { panel.close() }
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        field.stringValue = "Buy milk"
        let editor = try XCTUnwrap(field.currentEditor() as? NSTextView)
        editor.setSelectedRange(NSRange(location: 4, length: 4))
        panel.close()
        panel.show()
        field.sendAction(field.action, to: field.target)
        panel.close()
        panel.show()
        let reopened = try XCTUnwrap(field.currentEditor() as? NSTextView)
        XCTAssertEqual(reopened.string, "")
        XCTAssertEqual(reopened.selectedRange(), NSRange(location: 0, length: 0))
        reopened.insertText("Next task", replacementRange: reopened.selectedRange())
        XCTAssertEqual(field.stringValue, "Next task")
    }

    func testPreviewUsesTheWholeDraftAndRejectsOldReplies() throws {
        let panel = QuickCapturePanel()
        var requests: [(String, (String?) -> Void)] = []
        panel.preview = { text, done in requests.append((text, done)) }
        panel.show()
        defer { panel.close() }
        XCTAssertEqual(requests.map(\.0), [""])
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        let editor = try XCTUnwrap(field.currentEditor() as? NSTextView)
        editor.insertText("Buy milk %web $today", replacementRange: editor.selectedRange())
        XCTAssertEqual(requests.last?.0, "Buy milk %web $today")
        let current = try XCTUnwrap(requests.last).1
        current("Project: Web · Plan: Today · Status: Backlog")
        requests[0].1("Project: Unfiled · Status: Inbox")
        let visible = panel.panel.contentView!.descendants(of: NSTextField.self).filter { !$0.isHidden }.map(\.stringValue)
        XCTAssertTrue(visible.contains("Project: Web · Plan: Today · Status: Backlog"))
        XCTAssertFalse(visible.contains("Project: Unfiled · Status: Inbox"))
        // Moving the caret does not replace the full draft with its prefix.
        let count = requests.count
        editor.setSelectedRange(NSRange(location: 3, length: 0))
        XCTAssertEqual(requests.count, count)
        let previewHeight = panel.panel.frame.height
        editor.insertText(" more", replacementRange: editor.selectedRange())
        XCTAssertEqual(panel.panel.frame.height, previewHeight, accuracy: 1,
                       "a pending preview keeps its rows instead of bouncing on every keystroke")
        XCTAssertFalse(panel.panel.contentView!.descendants(of: NSTextField.self).contains {
            $0.stringValue == "Project: Web · Plan: Today · Status: Backlog"
        })
        panel.close()
        panel.show()
        current("stale preview")
        XCTAssertFalse(panel.panel.contentView!.descendants(of: NSTextField.self).contains { $0.stringValue == "stale preview" })
    }

    func testAnOldDismissalCannotCloseAReopenedPanel() throws {
        let panel = QuickCapturePanel()
        var dismiss: (() -> Void)?
        panel.scheduleDismissal = { dismiss = $0 }
        panel.submit = { _, done in done(true, "Captured") }
        panel.show()
        defer { panel.close() }
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        field.stringValue = "Buy milk"
        field.sendAction(field.action, to: field.target)
        let oldDismissal = try XCTUnwrap(dismiss)
        panel.close()
        panel.show()
        oldDismissal()
        XCTAssertTrue(panel.isVisible)

        field.stringValue = "Buy bread"
        field.sendAction(field.action, to: field.target)
        try XCTUnwrap(dismiss)()
        XCTAssertFalse(panel.isVisible, "the current capture still dismisses normally")
    }

    func testLongRefusalsGrowThePanelAndTheNextOpeningShrinksIt() throws {
        let panel = QuickCapturePanel()
        let reason = String(repeating: "The selected project is unavailable; choose an existing project. ", count: 8)
        panel.submit = { _, done in done(false, reason) }
        panel.show()
        defer { panel.close() }
        let originalHeight = panel.panel.frame.height
        let originalTop = panel.panel.frame.maxY
        let fields = try XCTUnwrap(panel.panel.contentView).descendants(of: NSTextField.self)
        let field = try XCTUnwrap(fields.first { $0.isEditable })
        field.stringValue = "Buy milk"
        field.sendAction(field.action, to: field.target)
        panel.panel.contentView?.layoutSubtreeIfNeeded()
        let feedback = try XCTUnwrap(fields.first { $0.stringValue == reason })
        XCTAssertEqual(feedback.maximumNumberOfLines, 0)
        XCTAssertEqual(feedback.lineBreakMode, .byWordWrapping)
        XCTAssertGreaterThan(panel.panel.frame.height, originalHeight)
        XCTAssertEqual(panel.panel.frame.maxY, originalTop, accuracy: 1)
        XCTAssertGreaterThan(feedback.frame.height, 30)
        XCTAssertEqual(field.stringValue, "Buy milk")

        panel.close()
        panel.show()
        XCTAssertEqual(panel.panel.frame.height, originalHeight, accuracy: 1)
    }

    /// The status line keeps the keys; the sigil examples show only while the
    /// field is empty, as its placeholder.
    func testTheHintKeepsTheKeysAndThePlaceholderTheSigils() throws {
        let panel = QuickCapturePanel()
        panel.show()
        defer { panel.close() }
        let labels = try XCTUnwrap(panel.panel.contentView).descendants(of: NSTextField.self)
        let field = try XCTUnwrap(labels.first { $0.isEditable })
        XCTAssertEqual(field.placeholderString, QuickCapturePanel.placeholder)
        XCTAssertTrue(labels.contains { $0.stringValue == QuickCapturePanel.hint })
        for sigil in ["#tag", "@date", "%project", "$state", "!1"] {
            XCTAssertTrue(QuickCapturePanel.placeholder.contains(sigil), sigil)
            XCTAssertFalse(QuickCapturePanel.hint.contains(sigil), sigil)
        }
    }

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

final class QuickCapturePanelCompletionTests: XCTestCase {
    private var panel: QuickCapturePanel!
    private var asked: [String] = []
    private var answer: TTCompletion?

    override func setUp() {
        panel = QuickCapturePanel()
        asked = []
        panel.complete = { [unowned self] text, done in
            asked.append(text)
            done(answer)
        }
        panel.show()
    }

    override func tearDown() {
        panel.close()
    }

    private func editor() throws -> NSTextView {
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        return try XCTUnwrap(field.currentEditor() as? NSTextView)
    }

    private func type(_ text: String) throws {
        let editor = try editor()
        editor.insertText(text, replacementRange: editor.selectedRange())
    }

    /// Typing asks tt about the draft up to the caret, and accepting replaces
    /// just the token, keeping what follows the caret.
    func testTypingAsksAndAcceptingReplacesTheToken() throws {
        try type("Buy milk tomorrow")
        let editor = try editor()
        editor.setSelectedRange(NSRange(location: 8, length: 0))
        answer = TTCompletion(token: "%cl", items: [.init(label: "%Client Work", insert: "%client-work ")])
        try type(" %cl")
        XCTAssertEqual(asked.last, "Buy milk %cl")
        XCTAssertEqual(panel.completion?.items.count, 1)

        answer = nil
        panel.acceptSuggestion()
        XCTAssertEqual(editor.string, "Buy milk %client-work  tomorrow")
        XCTAssertNil(panel.completion)
    }

    /// An answer for a token the draft no longer ends with is stale.
    func testAStaleAnswerShowsNothing() throws {
        try type("Buy %h")
        panel.showCompletion(TTCompletion(token: "%cl", items: [.init(label: "%Client", insert: "%client ")]), caret: 6)
        XCTAssertNil(panel.completion)
    }

    func testMovingTheCaretAndSelectingTextDismissSuggestions() throws {
        answer = TTCompletion(token: "%cl", items: [.init(label: "%Client", insert: "%client ")])
        try type("Buy %cl later")
        let editor = try editor()
        editor.setSelectedRange(NSRange(location: 7, length: 0))
        XCTAssertEqual(asked.last, "Buy %cl")
        XCTAssertNotNil(panel.completion)

        editor.setSelectedRange(NSRange(location: 3, length: 0))
        XCTAssertNil(panel.completion)
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        XCTAssertFalse(panel.control(field, textView: editor, doCommandBy: #selector(NSResponder.insertTab(_:))))
        panel.acceptSuggestion()
        XCTAssertEqual(editor.string, "Buy %cl later")

        editor.setSelectedRange(NSRange(location: 7, length: 0))
        XCTAssertNotNil(panel.completion)
        editor.setSelectedRange(NSRange(location: 4, length: 3))
        XCTAssertNil(panel.completion)
        panel.acceptSuggestion()
        XCTAssertEqual(editor.string, "Buy %cl later")
    }

    func testAReplyForAnEarlierCaretCannotAppearLater() throws {
        var delayedReplies: [(TTCompletion?) -> Void] = []
        panel.complete = { _, done in delayedReplies.append(done) }
        try type("Buy %cl")
        let editor = try editor()
        XCTAssertEqual(delayedReplies.count, 1)

        editor.setSelectedRange(NSRange(location: 0, length: 0))
        delayedReplies[0](TTCompletion(token: "%cl", items: [.init(label: "%Client", insert: "%client ")]))
        XCTAssertNil(panel.completion)

        editor.setSelectedRange(NSRange(location: 7, length: 0))
        XCTAssertEqual(delayedReplies.count, 2)
        delayedReplies[1](TTCompletion(token: "%cl", items: [.init(label: "%Client", insert: "%client ")]))
        XCTAssertNotNil(panel.completion)
    }
}

private extension NSView {
    func descendants<T: NSView>(of type: T.Type) -> [T] {
        subviews.flatMap { ($0 as? T).map { [$0] } ?? [] + $0.descendants(of: type) }
    }
}
