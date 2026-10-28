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

    func testCaptureCarriesRepeatableAttachments() {
        XCTAssertEqual(link("docket://capture?text=Spec&attach=/tmp/a%20b.pdf"), .capture("Spec", attach: ["/tmp/a b.pdf"]))
        XCTAssertEqual(link("docket://capture?attach=https://example.com/a%3Fb%3D1&attach=&attach=/tmp/c"),
                       .capture(nil, attach: ["https://example.com/a?b=1", "/tmp/c"]), "an empty attach is no value")
        XCTAssertEqual(link("docket://capture?attach=message://%253Cabc@example.com%253E"),
                       .capture(nil, attach: ["message://%3Cabc@example.com%3E"]))
    }

    func testEveryRouteRoundTrips() {
        let links: [DocketLink] = [
            .show, .capture(nil), .capture("Buy milk #home & eggs?"),
            .capture("Spec + notes", attach: ["/tmp/a b.pdf", "https://example.com/?q=1&r=2", "message://%3Cid@x%3E"]),
            .capture(nil, attach: ["/tmp/a"]), .view(.inbox), .view(.today), .view(.due),
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
        XCTAssertEqual(draft, DocketCaptureDraft(text: " &https://example.com/a?b=1", caret: 0,
                                                 attachmentTokens: " &https://example.com/a?b=1"))
        XCTAssertEqual(DocketCaptureDraft.from(text: nil, url: URL(string: "http://example.com")),
                       DocketCaptureDraft(text: " &http://example.com", caret: 0, attachmentTokens: " &http://example.com"))
        XCTAssertEqual(DocketCaptureDraft.from(text: "Read https://example.com later", url: nil),
                       DocketCaptureDraft(text: "Read https://example.com later"), "a link inside prose stays prose")
        XCTAssertEqual(DocketCaptureDraft.from(text: "mailto:sam@example.com", url: nil),
                       DocketCaptureDraft(text: "mailto:sam@example.com"))
    }

    private func build(_ text: String?, _ attachments: [String], folders: Set<String> = []) -> DocketCaptureDraft {
        DocketCaptureDraft.build(text: text, attachments: attachments, isDirectory: { folders.contains($0) })
    }

    func testAURLAloneLeavesTheCaretForTheTitle() {
        XCTAssertEqual(build(nil, ["https://example.com/spec"]),
                       DocketCaptureDraft(text: " &https://example.com/spec", caret: 0,
                                          attachmentTokens: " &https://example.com/spec"))
    }

    func testATitleIsKeptFirstAndSelected() {
        let draft = build("  Spec\nreview ", ["https://example.com/spec"])
        XCTAssertEqual(draft.text, "Spec review &https://example.com/spec")
        XCTAssertEqual(draft.caret, 0)
        XCTAssertEqual(draft.selectionLength, ("Spec review" as NSString).length)
        XCTAssertEqual(draft.attachmentTokens, " &https://example.com/spec")
        XCTAssertEqual(draft.dropped, [])
    }

    /// Docket reads `&` values through `util.NormalizePastedPathInput`, which
    /// takes a double-quoted value with `\"` and `\\` escaped.
    func testFilesAreQuotedTheWayTheGrammarReadsThem() {
        let draft = build(nil, ["/tmp/a.pdf", "/tmp/Q3 plan.pdf"])
        XCTAssertEqual(draft.text, #" &/tmp/a.pdf &"/tmp/Q3 plan.pdf""#)
        XCTAssertEqual(draft.caret, 0)
        XCTAssertEqual(DocketCaptureDraft.quotedPath(#"/tmp/it's "x"\y"#), #""/tmp/it's \"x\"\\y""#)
        XCTAssertEqual(build("Spec", ["/tmp/a b.pdf"]).text, #"Spec &"/tmp/a b.pdf""#)
    }

    func testAFolderIsDroppedByName() {
        let draft = build("Read", ["/tmp/Reports", "/tmp/r.pdf"], folders: ["/tmp/Reports"])
        XCTAssertEqual(draft.text, "Read &/tmp/r.pdf")
        XCTAssertEqual(draft.dropped, ["Left out folder Reports: Docket attaches files"])
        let folder = FileManager.default.temporaryDirectory.path
        XCTAssertEqual(DocketCaptureDraft.build(text: nil, attachments: [folder]).dropped.count, 1,
                       "the default answers from the file system")
    }

    func testAMailLinkIsAttachedAsGiven() {
        let mail = "message://%3Cabc@example.com%3E"
        XCTAssertEqual(build("Reply", [mail]).text, "Reply &" + mail)
        XCTAssertEqual(build(nil, ["message:"]).dropped, ["Left out message:: not a Mail message link"])
        XCTAssertEqual(build(nil, ["mailto:sam@example.com", "notes/a.md"]).dropped.count, 2,
                       "other schemes and relative paths are dropped")
    }

    func testAttachmentsPastTheLimitAreDroppedAndReported() {
        let files = (1...11).map { "/tmp/f\($0)" }
        let draft = build("Batch", files)
        XCTAssertEqual(draft.attachmentTokens, (1...10).map { " &/tmp/f\($0)" }.joined())
        XCTAssertEqual(draft.dropped, ["Left out 1 past the 10-attachment limit"])
    }

    func testEmptyInputIsAnEmptyDraft() {
        XCTAssertEqual(build(nil, []), DocketCaptureDraft(text: ""))
        XCTAssertEqual(build(" \n ", ["", "  "]), DocketCaptureDraft(text: ""))
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
        XCTAssertTrue(DocketServiceProvider().responds(to: #selector(DocketServiceProvider.captureSelection(_:userData:error:))))
        XCTAssertNotNil(service["NSRequiredContext"], "without it macOS leaves the service off until enabled")
    }
}

/// What a Share menu hands over, standing in for `NSItemProvider`.
private struct FakeShareProvider: DocketShareItemProvider {
    let types: [String]
    let value: Any?

    func hasItemConformingToTypeIdentifier(_ typeIdentifier: String) -> Bool {
        types.contains(typeIdentifier)
    }

    func loadDocketItem(_ typeIdentifier: String, completion: @escaping (Any?) -> Void) {
        let value = value
        DispatchQueue.global().async { completion(value) }
    }
}

final class ShareItemsTests: XCTestCase {
    private func link(_ inputs: [DocketShareInput]) -> DocketLink? {
        let done = expectation(description: "loaded")
        var made: DocketLink?
        DocketShare.load(inputs) { link in
            XCTAssertTrue(Thread.isMainThread)
            made = link
            done.fulfill()
        }
        wait(for: [done], timeout: 5)
        return made
    }

    private func draft(_ link: DocketLink?, folders: Set<String> = []) -> DocketCaptureDraft? {
        guard case .capture(let text, let attach)? = link else { return nil }
        return DocketCaptureDraft.build(text: text, attachments: attach, isDirectory: { folders.contains($0) })
    }

    func testASafariPageIsItsTitleSelectedWithTheURLAttached() throws {
        let page = FakeShareProvider(types: [DocketShare.urlType, DocketShare.plainTextType],
                                     value: URL(string: "https://example.com/spec?a=1&b=2")!)
        let made = link([DocketShareInput(contentText: " Spec\nreview ", providers: [page])])
        XCTAssertEqual(made, .capture("Spec review", attach: ["https://example.com/spec?a=1&b=2"]))
        let url = try XCTUnwrap(made?.url)
        XCTAssertEqual(DocketLink(url: url), made, "the link round-trips through docket://")
        let draft = try XCTUnwrap(draft(made))
        XCTAssertEqual(draft.text, "Spec review &https://example.com/spec?a=1&b=2")
        XCTAssertEqual(draft.selectionLength, ("Spec review" as NSString).length)
    }

    func testFinderFilesAreAttachedByPathAndAFolderIsLeftToTheApp() throws {
        let file = FakeShareProvider(types: [DocketShare.fileURLType, DocketShare.urlType],
                                     value: URL(fileURLWithPath: "/tmp/Q3 plan.pdf"))
        let data = FakeShareProvider(types: [DocketShare.fileURLType],
                                     value: URL(fileURLWithPath: "/tmp/b.png").dataRepresentation)
        let folder = FakeShareProvider(types: [DocketShare.fileURLType], value: URL(fileURLWithPath: "/tmp/Reports"))
        let made = link([DocketShareInput(contentText: nil, providers: [file, data, folder])])
        XCTAssertEqual(made, .capture(nil, attach: ["/tmp/Q3 plan.pdf", "/tmp/b.png", "/tmp/Reports"]))
        let draft = try XCTUnwrap(draft(made, folders: ["/tmp/Reports"]))
        XCTAssertEqual(draft.text, #" &"/tmp/Q3 plan.pdf" &/tmp/b.png"#)
        XCTAssertEqual(draft.caret, 0)
        XCTAssertEqual(draft.dropped, ["Left out folder Reports: Docket attaches files"])
    }

    func testSharedTextIsTheTextWhenNoTitleCame() throws {
        let text = FakeShareProvider(types: [DocketShare.plainTextType], value: "Call Sam\nabout #work")
        let attributed = FakeShareProvider(types: [DocketShare.plainTextType],
                                           value: NSAttributedString(string: "ignored second"))
        XCTAssertEqual(link([DocketShareInput(contentText: "  ", providers: [text, attributed])]),
                       .capture("Call Sam about #work", attach: []))
        XCTAssertEqual(draft(.capture("Call Sam about #work", attach: []))?.text, "Call Sam about #work")
    }

    func testNothingReadableMakesNoLink() {
        let image = FakeShareProvider(types: ["public.png"], value: Data([1, 2]))
        let failed = FakeShareProvider(types: [DocketShare.urlType], value: nil)
        XCTAssertNil(link([DocketShareInput(contentText: nil, providers: [image, failed])]))
        XCTAssertNil(link([]))
    }

    func testEveryItemsAttachmentsAreKeptInOrder() {
        let first = FakeShareProvider(types: [DocketShare.urlType], value: URL(string: "https://a.example")!)
        let second = FakeShareProvider(types: [DocketShare.urlType], value: URL(string: "message://%3Cid@x%3E")!)
        XCTAssertEqual(link([DocketShareInput(contentText: nil, providers: [first]),
                             DocketShareInput(contentText: "Title", providers: [second])]),
                       .capture("Title", attach: ["https://a.example", "message://%3Cid@x%3E"]))
    }

    func testTheShareExtensionIsEmbeddedForURLsFilesAndText() throws {
        let plugIns = try XCTUnwrap(Bundle.main.builtInPlugInsURL)
        let share = try XCTUnwrap(Bundle(url: plugIns.appendingPathComponent("DocketShare.appex")))
        XCTAssertEqual(share.bundleIdentifier, "io.github.sglavoie.docket.share")
        XCTAssertEqual(share.infoDictionary?["CFBundleVersion"] as? String,
                       Bundle.main.infoDictionary?["CFBundleVersion"] as? String)
        let ext = try XCTUnwrap(share.infoDictionary?["NSExtension"] as? [String: Any])
        XCTAssertEqual(ext["NSExtensionPointIdentifier"] as? String, "com.apple.share-services")
        let attributes = try XCTUnwrap(ext["NSExtensionAttributes"] as? [String: Any])
        let rule = try XCTUnwrap(attributes["NSExtensionActivationRule"] as? [String: Any])
        XCTAssertEqual(rule["NSExtensionActivationSupportsWebURLWithMaxCount"] as? Int, 10)
        XCTAssertEqual(rule["NSExtensionActivationSupportsFileWithMaxCount"] as? Int, 10)
        XCTAssertEqual(rule["NSExtensionActivationSupportsText"] as? Bool, true)
    }
}

final class QuickCapturePresentTests: XCTestCase {
    private func field(_ panel: QuickCapturePanel) throws -> NSTextField {
        try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
    }

    /// A share sheet opens the link, then closes and gives its app the keyboard
    /// back; a panel a link opened stays for that, and only for a moment.
    func testAPanelALinkOpenedOutlastsTheSharingAppTakingTheKeyboardBack() {
        let panel = QuickCapturePanel()
        defer { panel.close() }
        var clock = Date()
        panel.now = { clock }
        XCTAssertTrue(panel.present(draft: "Spec &https://example.com"))
        panel.keepKeyAfterLink()
        panel.windowDidResignKey(Notification(name: NSWindow.didResignKeyNotification))
        XCTAssertTrue(panel.isVisible, "the sharing app taking the keyboard back does not put it away")
        clock += QuickCapturePanel.linkKeyGrace + 0.1
        panel.windowDidResignKey(Notification(name: NSWindow.didResignKeyNotification))
        XCTAssertFalse(panel.isVisible, "clicking away later still does")
        panel.show()
        panel.windowDidResignKey(Notification(name: NSWindow.didResignKeyNotification))
        XCTAssertFalse(panel.isVisible, "putting it away ends the grace")
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

    func testALinkWithTextSelectsItSoTypingReplacesIt() throws {
        let panel = QuickCapturePanel()
        defer { panel.close() }
        let draft = DocketCaptureDraft.build(text: "Spec", attachments: ["/tmp/a b.pdf"])
        XCTAssertTrue(panel.present(draft))
        let field = try field(panel)
        XCTAssertEqual(field.stringValue, #"Spec &"/tmp/a b.pdf""#)
        XCTAssertEqual((field.currentEditor() as? NSTextView)?.selectedRange(), NSRange(location: 0, length: 4))
    }

    func testAttachmentsJoinAnUnsavedDraftAtItsEnd() throws {
        let panel = QuickCapturePanel()
        var saved: [QuickCaptureRecovery] = []
        panel.persistRecovery = { saved.append($0) }
        defer { panel.close() }
        let field = try field(panel)
        XCTAssertTrue(panel.present(draft: "Call Sam"))
        XCTAssertTrue(panel.present(DocketCaptureDraft.build(text: "Spec", attachments: ["https://example.com/spec", "/tmp/a b.pdf"])))
        XCTAssertEqual(field.stringValue, #"Call Sam &https://example.com/spec &"/tmp/a b.pdf""#,
                       "the draft's text is kept and the link's text is not used")
        XCTAssertEqual(saved.last?.draft, field.stringValue)
        XCTAssertTrue(panel.panel.contentView!.descendants(of: NSTextField.self)
            .contains { $0.stringValue == QuickCapturePanel.draftAppendedMessage })
    }

    func testATextOnlyLinkLeavesAnUnsavedDraftUnchanged() throws {
        let panel = QuickCapturePanel()
        defer { panel.close() }
        let field = try field(panel)
        XCTAssertTrue(panel.present(draft: "Call Sam"))
        XCTAssertFalse(panel.present(DocketCaptureDraft.build(text: "Spec", attachments: [])))
        XCTAssertEqual(field.stringValue, "Call Sam")
        XCTAssertTrue(panel.panel.contentView!.descendants(of: NSTextField.self)
            .contains { $0.stringValue == QuickCapturePanel.draftKeptMessage })
    }

    func testWhatADraftDroppedIsSaid() throws {
        let panel = QuickCapturePanel()
        defer { panel.close() }
        XCTAssertTrue(panel.present(DocketCaptureDraft.build(text: nil, attachments: ["notes/a.md"])))
        XCTAssertTrue(panel.panel.contentView!.descendants(of: NSTextField.self)
            .contains { $0.stringValue.hasPrefix("Left out notes/a.md") })
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
