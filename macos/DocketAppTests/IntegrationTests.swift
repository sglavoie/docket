import AppKit
import Security
import XCTest
@testable import Docket

final class DocketLinkTests: XCTestCase {
    private func link(_ string: String) -> DocketLink? {
        DocketLink(url: URL(string: string)!)
    }

    func testRoutes() {
        XCTAssertEqual(link("docket://"), .show)
        XCTAssertEqual(link("docket://show"), .show)
        XCTAssertEqual(link("DOCKET://Open"), .show)
        XCTAssertEqual(link("docket://capture"), .capture(nil))
        XCTAssertEqual(link("docket://capture?text="), .capture(nil))
        XCTAssertEqual(link("docket://capture?text=Call%20Sam%20%23home%20@tomorrow"), .capture("Call Sam #home @tomorrow"))
        XCTAssertEqual(link("docket://capture?text=caf%C3%A9%20!1+"), .capture("café !1+"))
        XCTAssertEqual(link("docket://capture?text=a+b"), .capture("a+b"), "a plus is literal, never a space")
        XCTAssertEqual(link("docket://view/inbox"), .view(.inbox))
        XCTAssertEqual(link("docket://view/Today"), .view(.today))
        XCTAssertEqual(link("docket://view/due"), .view(.due))
        XCTAssertEqual(link("docket://view/overdue"), .view(.due))
        XCTAssertEqual(link("docket://view/done"), .view(.done))
        XCTAssertEqual(link("docket://task/p-1/t%2F2"), .task(project: "p-1", task: "t/2"))
    }

    /// Every route but capture brings Docket's window forward, so only those
    /// ask LaunchServices to activate the app; capture opens over the app in front.
    func testOnlyCaptureLeavesDocketWhereItIs() {
        XCTAssertTrue(DocketLink.show.showsDocket)
        XCTAssertTrue(DocketLink.view(.done).showsDocket)
        XCTAssertTrue(DocketLink.task(project: "p", task: "t").showsDocket)
        XCTAssertFalse(DocketLink.capture("x").showsDocket)
        XCTAssertFalse(DocketLink.capture(nil).showsDocket)
    }

    func testRefusals() {
        for string in [
            "https://show", "docket://delete", "docket://show/extra", "docket://view", "docket://view/recent",
            "docket://view/today/more", "docket://task/only-project", "docket://task/a/b/c", "docket://capture/x",
        ] {
            XCTAssertNil(link(string), string)
        }
    }

    func testEveryRouteRoundTrips() {
        let links: [DocketLink] = [
            .show, .capture(nil), .capture("Buy milk #home & eggs?"), .view(.inbox), .view(.today), .view(.due),
            .view(.done), .task(project: "proj id", task: "a/b"),
        ]
        for value in links {
            XCTAssertEqual(DocketLink(url: value.url), value, value.url.absoluteString)
        }
    }

    func testTheAppClaimsTheScheme() throws {
        let types = try XCTUnwrap(Bundle.main.infoDictionary?["CFBundleURLTypes"] as? [[String: Any]])
        XCTAssertTrue(types.contains { ($0["CFBundleURLSchemes"] as? [String])?.contains(DocketLink.scheme) == true })
    }
}

final class CaptureDraftTests: XCTestCase {
    func testTextCollapsesToOneLine() {
        XCTAssertEqual(DocketCaptureDraft.from(text: "  Reply to\n\tSam  about  #work \r\n", url: nil),
                       DocketCaptureDraft(text: "Reply to Sam about #work"))
        XCTAssertNil(DocketCaptureDraft.from(text: " \n ", url: nil))
        XCTAssertNil(DocketCaptureDraft.from(text: nil, url: URL(fileURLWithPath: "/tmp/a")))
    }

    func testAWebAddressAloneBecomesALinkTheTitleGoesBefore() {
        let draft = DocketCaptureDraft.from(text: " https://example.com/a?b=1 ", url: nil)
        XCTAssertEqual(draft, DocketCaptureDraft(text: " &https://example.com/a?b=1", caret: 0))
        XCTAssertEqual(DocketCaptureDraft.from(text: nil, url: URL(string: "http://example.com")),
                       DocketCaptureDraft(text: " &http://example.com", caret: 0))
        XCTAssertEqual(DocketCaptureDraft.from(text: "Read https://example.com later", url: nil),
                       DocketCaptureDraft(text: "Read https://example.com later"), "a link inside prose stays prose")
        XCTAssertEqual(DocketCaptureDraft.from(text: "mailto:sam@example.com", url: nil),
                       DocketCaptureDraft(text: "mailto:sam@example.com"))
    }

    func testTheServiceHandsTheSelectionToCapture() {
        let provider = DocketServiceProvider()
        var presented: [DocketCaptureDraft] = []
        provider.present = { presented.append($0) }
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("docket-test-\(UUID().uuidString)"))
        defer { pasteboard.releaseGlobally() }

        pasteboard.clearContents()
        pasteboard.setString("Fix the\nlogin bug", forType: .string)
        var error: NSString?
        provider.captureSelection(pasteboard, userData: nil, error: &error)
        XCTAssertNil(error)
        XCTAssertEqual(presented, [DocketCaptureDraft(text: "Fix the login bug")])

        pasteboard.clearContents()
        provider.captureSelection(pasteboard, userData: nil, error: &error)
        XCTAssertEqual(error, "There is no text to capture.")
        XCTAssertEqual(presented.count, 1)
    }

    func testTheServiceIsDeclared() throws {
        let services = try XCTUnwrap(Bundle.main.infoDictionary?["NSServices"] as? [[String: Any]])
        let service = try XCTUnwrap(services.first)
        XCTAssertEqual(service["NSMessage"] as? String, "captureSelection")
        XCTAssertTrue(DocketServiceProvider().responds(to: Selector(("captureSelection:userData:error:"))))
        XCTAssertNotNil(service["NSRequiredContext"], "without it macOS leaves the service off until enabled")
    }
}

final class QuickCapturePresentTests: XCTestCase {
    private func field(_ panel: QuickCapturePanel) throws -> NSTextField {
        try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
    }

    func testAnEmptyPanelTakesTheDraftWithItsCaret() throws {
        let panel = QuickCapturePanel()
        var saved: [QuickCaptureRecovery] = []
        panel.persistRecovery = { saved.append($0) }
        defer { panel.close() }
        XCTAssertTrue(panel.present(draft: " &https://example.com", caret: 0))
        let field = try field(panel)
        XCTAssertEqual(field.stringValue, " &https://example.com")
        XCTAssertEqual((field.currentEditor() as? NSTextView)?.selectedRange(), NSRange(location: 0, length: 0))
        XCTAssertEqual(saved.last, QuickCaptureRecovery(draft: " &https://example.com", pendingSubmission: nil),
                       "the draft is recoverable like a typed one")
    }

    func testAnUnsavedDraftIsNeverReplaced() throws {
        let panel = QuickCapturePanel()
        defer { panel.close() }
        let field = try field(panel)
        XCTAssertTrue(panel.present(draft: "First"))
        XCTAssertTrue(panel.present(draft: "First"), "the same text again is no conflict")
        XCTAssertFalse(panel.present(draft: "Second"))
        XCTAssertEqual(field.stringValue, "First")
        XCTAssertTrue(panel.isVisible)
        XCTAssertTrue(panel.panel.contentView!.descendants(of: NSTextField.self)
            .contains { $0.stringValue == QuickCapturePanel.draftKeptMessage })
        field.stringValue = "   "
        XCTAssertTrue(panel.present(draft: "Second"), "a blank field holds nothing to keep")
        XCTAssertEqual(field.stringValue, "Second")
    }
}

final class PulseSnapshotTests: XCTestCase {
    private var calendar: Calendar {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "America/Toronto")!
        return calendar
    }

    func testRoundTripsThroughTheStore() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("docket-pulse-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: dir) }
        let store = try XCTUnwrap(DocketPulseSnapshotStore(container: dir))
        XCTAssertNil(store.read())
        let snapshot = DocketPulseSnapshot(
            DocketPulse(inbox: 1, today: 2, overdue: 3, done: 4), at: Date(timeIntervalSince1970: 1_800_000_000)
        )
        try store.write(snapshot)
        XCTAssertEqual(store.read(), snapshot)
        let agenda = DocketAgenda(items: [DocketAgendaItem(kind: .today, project: "p", task: "t", title: "Call", projectName: "Work")], total: 4)
        let listed = DocketPulseSnapshot(
            DocketPulse(inbox: 1, today: 2, overdue: 3, done: 4), agenda: agenda, at: Date(timeIntervalSince1970: 1_800_000_000)
        )
        try store.write(listed)
        XCTAssertEqual(store.read()?.agenda, agenda)
        // A snapshot an older Docket.app wrote has no agenda and still reads.
        try Data(#"{"inbox":1,"today":2,"overdue":3,"done":4,"updatedAt":"2027-01-15T08:00:00Z","day":"2027-01-15"}"#.utf8).write(to: store.url)
        XCTAssertEqual(store.read()?.today, 2)
        XCTAssertNil(store.read()?.agenda)
        try Data("not json".utf8).write(to: store.url)
        XCTAssertNil(store.read(), "an unreadable snapshot reads as none")
        XCTAssertNil(DocketPulseSnapshotStore(container: nil))
    }

    func testCountsGoStaleAtLocalMidnight() throws {
        let evening = try XCTUnwrap(calendar.date(from: DateComponents(year: 2026, month: 10, day: 27, hour: 23, minute: 59)))
        let snapshot = DocketPulseSnapshot(inbox: 0, today: 1, overdue: 0, done: 0, updatedAt: evening, calendar: calendar)
        XCTAssertEqual(snapshot.day, "2026-10-27")
        XCTAssertTrue(snapshot.isCurrent(at: evening.addingTimeInterval(59), calendar: calendar))
        XCTAssertFalse(snapshot.isCurrent(at: evening.addingTimeInterval(61), calendar: calendar))
    }

    func testTheWidgetTapOpensWhatNeedsAttention() {
        let quiet = DocketPulseSnapshot(inbox: 4, today: 2, overdue: 0, done: 0, updatedAt: Date())
        XCTAssertEqual(quiet.primaryLink, .view(.today))
        let late = DocketPulseSnapshot(inbox: 0, today: 0, overdue: 1, done: 0, updatedAt: Date())
        XCTAssertEqual(late.primaryLink, .view(.due))
    }

    /// The widget reads the pulse from this group; without it in the app's own
    /// signature, the app could not write there once macOS enforces the group.
    func testTheAppIsSignedIntoTheWidgetsAppGroup() throws {
        var code: SecCode?
        XCTAssertEqual(SecCodeCopySelf([], &code), errSecSuccess)
        var staticCode: SecStaticCode?
        XCTAssertEqual(SecCodeCopyStaticCode(try XCTUnwrap(code), [], &staticCode), errSecSuccess)
        var info: CFDictionary?
        XCTAssertEqual(
            SecCodeCopySigningInformation(try XCTUnwrap(staticCode), SecCSFlags(rawValue: kSecCSSigningInformation), &info),
            errSecSuccess
        )
        let entitlements = try XCTUnwrap((info as? [String: Any])?[kSecCodeInfoEntitlementsDict as String] as? [String: Any])
        let groups = try XCTUnwrap(entitlements["com.apple.security.application-groups"] as? [String])
        XCTAssertEqual(groups, [DocketSharedContainer.appGroup])
    }

    func testTheWidgetShipsInsideTheApp() throws {
        let plugins = try XCTUnwrap(Bundle.main.builtInPlugInsURL)
        let widget = try XCTUnwrap(Bundle(url: plugins.appendingPathComponent("DocketWidgets.appex")))
        let point = (widget.infoDictionary?["NSExtension"] as? [String: Any])?["NSExtensionPointIdentifier"] as? String
        XCTAssertEqual(point, "com.apple.widgetkit-extension")
    }
}

final class ShortcutTests: XCTestCase {
    func testCountsNameEachPulseField() {
        let pulse = DocketPulse(inbox: 1, today: 2, overdue: 3, done: 4)
        XCTAssertEqual([DocketCountOption.inbox, .today, .overdue, .done].map { $0.tally(in: pulse) }, [1, 2, 3, 4])
        XCTAssertEqual(DocketCountOption.overdue.sentence(3), "3 overdue")
        XCTAssertEqual(DocketViewOption.overdue.view, .due)
    }

    func testTheAppHostsTheShortcuts() {
        XCTAssertTrue(DocketShortcuts.host === NSApp.delegate as AnyObject)
    }

    func testABlankCaptureIsRefusedAtOnce() {
        let delegate = AppDelegate()
        var outcome: (Bool, String)?
        delegate.shortcutCapture(" \n ") { outcome = ($0, $1) }
        XCTAssertEqual(outcome?.0, false)
        XCTAssertEqual(outcome?.1, "There is nothing to capture.")
    }

    func testWithNoDocketACaptureSaysWhyOnceTheWaitEnds() {
        let delegate = AppDelegate()
        delegate.controlWaitTimeout = 0.05
        let answered = expectation(description: "answered")
        delegate.shortcutCapture("Buy milk") { ok, message in
            XCTAssertFalse(ok)
            XCTAssertEqual(message, "Docket is not running.")
            answered.fulfill()
        }
        wait(for: [answered], timeout: 2)

        // Settled now, so the next one answers at once.
        var pulseReason: String?
        delegate.shortcutPulse { _, reason in pulseReason = reason }
        XCTAssertEqual(pulseReason, "Docket is not running.")
    }

    func testACaptureLinkOpensThePanelWithoutSending() throws {
        let delegate = AppDelegate()
        delegate.quickCapture.persistRecovery = { _ in }
        var sent = false
        delegate.quickCapture.submit = { _, _, _ in sent = true }
        defer { delegate.quickCapture.close() }
        delegate.controlWaitTimeout = 0.05
        delegate.presentCapture(DocketCaptureDraft(text: "Call Sam"))
        XCTAssertFalse(delegate.quickCapture.isVisible, "a starting Docket would take the keyboard and put it away")
        let shown = expectation(description: "shown")
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { shown.fulfill() }
        wait(for: [shown], timeout: 2)
        XCTAssertTrue(delegate.quickCapture.isVisible)
        let field = try XCTUnwrap(delegate.quickCapture.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        XCTAssertEqual(field.stringValue, "Call Sam")
        XCTAssertFalse(sent)
    }
}
