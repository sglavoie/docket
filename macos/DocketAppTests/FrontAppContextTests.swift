import AppKit
import XCTest
@testable import Docket

final class FrontAppContextTests: XCTestCase {
    private typealias Answer = Result<DocketFrontAppAnswer, DocketFrontAppRefusal>

    /// A panel opened over `bundleID` whose Cmd+K answers are held for the test to give.
    private func panel(over bundleID: String?) -> (QuickCapturePanel, () -> [(DocketFrontApp, (Answer) -> Void)]) {
        let panel = QuickCapturePanel()
        panel.frontmostBundleID = { bundleID }
        var asked: [(DocketFrontApp, (Answer) -> Void)] = []
        panel.readFrontApp = { app, done in asked.append((app, done)) }
        return (panel, { asked })
    }

    private func field(_ panel: QuickCapturePanel) throws -> NSTextField {
        try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
    }

    private func shows(_ panel: QuickCapturePanel, _ text: String) -> Bool {
        panel.panel.contentView!.descendants(of: NSTextField.self).contains { !$0.isEditable && $0.stringValue == text }
    }

    private func privacyButtonShown(_ panel: QuickCapturePanel) -> Bool {
        panel.panel.contentView!.descendants(of: NSButton.self)
            .contains { $0.title == "Open Privacy Settings" && !$0.isHidden }
    }

    private func editor(_ panel: QuickCapturePanel) throws -> NSTextView {
        try XCTUnwrap(try field(panel).currentEditor() as? NSTextView)
    }

    func testRecognizesOnlyTheSupportedApps() {
        XCTAssertEqual(DocketFrontApp(bundleIdentifier: "com.apple.Safari"), .safari)
        XCTAssertEqual(DocketFrontApp(bundleIdentifier: "com.google.Chrome"), .chrome)
        XCTAssertEqual(DocketFrontApp(bundleIdentifier: "com.apple.mail"), .mail)
        XCTAssertEqual(DocketFrontApp(bundleIdentifier: "com.apple.finder"), .finder)
        XCTAssertNil(DocketFrontApp(bundleIdentifier: "org.mozilla.firefox"))
        XCTAssertNil(DocketFrontApp(bundleIdentifier: nil))
    }

    func testTheHintOffersCmdKOnlyOverASupportedApp() throws {
        let (safari, _) = panel(over: "com.apple.Safari")
        defer { safari.close() }
        safari.show()
        XCTAssertEqual(safari.frontApp, .safari)
        XCTAssertTrue(shows(safari, "Enter capture · ⌘K attach Safari tab · Ctrl+N capture another · Tab complete · Esc keep draft"))
        XCTAssertEqual(try field(safari).accessibilityHelp(), safari.currentHint)

        for (bundle, item) in [("com.google.Chrome", "Chrome tab"), ("com.apple.mail", "Mail message"),
                               ("com.apple.finder", "Finder selection")] {
            let (other, _) = panel(over: bundle)
            other.show()
            XCTAssertTrue(other.currentHint.contains("⌘K attach \(item)"), bundle)
            other.close()
        }

        let (unsupported, asked) = panel(over: "com.apple.Terminal")
        defer { unsupported.close() }
        unsupported.show()
        XCTAssertNil(unsupported.frontApp)
        XCTAssertTrue(shows(unsupported, QuickCapturePanel.hint))
        XCTAssertFalse(QuickCapturePanel.hint.contains("⌘K"))
        XCTAssertFalse(unsupported.attachFromFrontApp(), "Cmd+K is left to the system")
        XCTAssertTrue(asked().isEmpty, "nothing is asked")
    }

    func testTheFrontAppIsRecordedWhenThePanelOpensNotOnEveryShow() {
        var front = "com.apple.Safari"
        let panel = QuickCapturePanel()
        panel.frontmostBundleID = { front }
        defer { panel.close() }
        panel.show()
        front = "io.github.sglavoie.docket"
        panel.show()
        XCTAssertEqual(panel.frontApp, .safari, "showing an open panel again keeps the app it opened over")
        panel.close()
        panel.show()
        XCTAssertNil(panel.frontApp)
    }

    func testCmdKIsBoundInThePanel() throws {
        let (panel, asked) = panel(over: "com.apple.Safari")
        defer { panel.close() }
        panel.show()
        let event = try XCTUnwrap(NSEvent.keyEvent(
            with: .keyDown, location: .zero, modifierFlags: .command, timestamp: 0,
            windowNumber: panel.panel.windowNumber, context: nil, characters: "k",
            charactersIgnoringModifiers: "k", isARepeat: false, keyCode: 40
        ))
        XCTAssertTrue(panel.panel.performKeyEquivalent(with: event))
        XCTAssertEqual(asked().map(\.0), [.safari])
        XCTAssertTrue(shows(panel, "Asking Safari…"))
    }

    func testTextGoesOnlyIntoAnEmptyDraftAndIsSelected() throws {
        let (panel, asked) = panel(over: "com.apple.Safari")
        defer { panel.close() }
        panel.show()
        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[0].1(.success(DocketFrontAppAnswer(attachments: ["https://example.com/a"], text: "Example  page")))
        let field = try field(panel)
        XCTAssertEqual(field.stringValue, "Example page &https://example.com/a")
        XCTAssertEqual(try editor(panel).selectedRange(), NSRange(location: 0, length: 12), "typing replaces the title")
        XCTAssertTrue(shows(panel, "Attached the Safari tab"))
    }

    func testWithADraftOnlyTheTokensGoInAtTheCaret() throws {
        let (panel, asked) = panel(over: "com.apple.Safari")
        defer { panel.close() }
        XCTAssertTrue(panel.present(draft: "Read later #web", caret: 10))
        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[0].1(.success(DocketFrontAppAnswer(attachments: ["https://example.com/a"], text: "Example")))
        XCTAssertEqual(try field(panel).stringValue, "Read later &https://example.com/a #web",
                       "the title is not used, and the token is spaced from its neighbors")

        try editor(panel).setSelectedRange(NSRange(location: 0, length: 4))
        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[1].1(.success(DocketFrontAppAnswer(attachments: ["https://example.com/b"], text: nil)))
        XCTAssertEqual(try field(panel).stringValue, "Read &https://example.com/b later &https://example.com/a #web",
                       "a selection is kept and the token goes after it")
    }

    func testSeveralMailMessagesFillAnEmptyDraftOneTaskPerLine() throws {
        let (panel, asked) = panel(over: "com.apple.mail")
        defer { panel.close() }
        panel.show()
        let field = try field(panel)
        let singleHeight = panel.panel.frame.height
        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[0].1(DocketMailSource.answer(ids: ["one@x", "two@x"], subjects: ["First  subject", nil]))
        XCTAssertEqual(field.stringValue, "First subject &message://%3Cone@x%3E\n &message://%3Ctwo@x%3E")
        XCTAssertFalse(field.usesSingleLineMode, "the field shows every line")
        XCTAssertGreaterThan(panel.panel.frame.height, singleHeight, "and grows to fit them")
        XCTAssertTrue(shows(panel, "Added 2 tasks from Mail, one per line"))

        panel.close()
        panel.show()
        XCTAssertFalse(field.usesSingleLineMode, "a reopened multi-line draft stays multi-line")
        field.stringValue = ""
        panel.close()
        panel.show()
        XCTAssertTrue(field.usesSingleLineMode)

        XCTAssertTrue(panel.present(draft: "Reply"))
        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[1].1(DocketMailSource.answer(ids: ["one@x", "two@x"], subjects: ["First", "Second"]))
        XCTAssertEqual(field.stringValue, "Reply &message://%3Cone@x%3E &message://%3Ctwo@x%3E",
                       "a draft with text takes the tokens alone, on its one task")
    }

    func testTaskLinesAreCappedAndNeedTextForEveryItem() {
        let ids = (1...12).map { "m\($0)@x" }
        let lines = QuickCapturePanel.taskLines(DocketFrontAppAnswer(
            attachments: ids.map(DocketMailSource.link), text: nil, itemTexts: ids.map { $0 }
        ))
        XCTAssertEqual(lines?.lines.count, DocketCaptureDraft.attachmentLimit)
        XCTAssertEqual(lines?.dropped, ["Left out 2 past the 10-attachment limit"])
        XCTAssertNil(QuickCapturePanel.taskLines(DocketFrontAppAnswer(attachments: ["/a", "/b"], text: nil)),
                     "Finder files share one task")
    }

    func testAStaleAnswerIsDropped() throws {
        let (panel, asked) = panel(over: "com.apple.Safari")
        defer { panel.close() }
        panel.show()
        XCTAssertTrue(panel.attachFromFrontApp())
        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[0].1(.success(DocketFrontAppAnswer(attachments: ["https://example.com/old"], text: "Old")))
        XCTAssertEqual(try field(panel).stringValue, "", "a superseded answer is dropped")
        asked()[1].1(.success(DocketFrontAppAnswer(attachments: ["https://example.com/new"], text: nil)))
        XCTAssertEqual(try field(panel).stringValue, " &https://example.com/new")

        XCTAssertTrue(panel.attachFromFrontApp())
        panel.close()
        asked()[2].1(.success(DocketFrontAppAnswer(attachments: ["https://example.com/late"], text: nil)))
        panel.show()
        XCTAssertEqual(try field(panel).stringValue, " &https://example.com/new", "an answer after closing is dropped")

        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[3].1(.failure(.notPermitted))
        panel.close()
        panel.show()
        XCTAssertFalse(privacyButtonShown(panel), "reopening says the hint again")
    }

    func testAnAnswerAfterSendingIsDropped() throws {
        let (panel, asked) = panel(over: "com.apple.Safari")
        panel.submit = { _, _, _ in }
        panel.captureContext = { _ in }
        defer { panel.close() }
        XCTAssertTrue(panel.present(draft: "Send me"))
        let field = try field(panel)
        XCTAssertTrue(panel.attachFromFrontApp())
        field.sendAction(field.action, to: field.target)
        XCTAssertTrue(panel.sending)
        asked()[0].1(.success(DocketFrontAppAnswer(attachments: ["https://example.com"], text: nil)))
        XCTAssertEqual(field.stringValue, "Send me")
    }

    func testEachRefusalHasItsStatusLine() throws {
        let cases: [(DocketFrontApp, DocketFrontAppRefusal, String)] = [
            (.safari, .notPermitted, "Docket isn’t allowed to read Safari. Allow it in Privacy & Security › Automation."),
            (.chrome, .notAnswering, "Chrome didn’t answer within 2 seconds."),
            (.safari, .nothingSelected, "Safari has no tab open."),
            (.chrome, .nothingSelected, "Chrome has no tab open."),
            (.mail, .nothingSelected, "No message is selected in Mail."),
            (.finder, .nothingSelected, "Nothing is selected in Finder."),
            (.finder, .foldersDropped(["Projects"]), "Left out folder Projects: Docket attaches files"),
            (.finder, .foldersDropped(["A", "B"]), "Left out 2 folders: Docket attaches files"),
        ]
        for (app, refusal, text) in cases {
            XCTAssertEqual(refusal.status(for: app), text)
        }

        let (panel, asked) = panel(over: "com.apple.Safari")
        var announced: [String] = []
        panel.announce = { announced.append($0) }
        var opened = 0
        panel.openPrivacySettings = { opened += 1 }
        defer { panel.close() }
        panel.show()
        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[0].1(.failure(.notPermitted))
        let denied = DocketFrontAppRefusal.notPermitted.status(for: .safari)
        XCTAssertTrue(shows(panel, denied))
        XCTAssertEqual(announced.suffix(2), ["Asking Safari…", denied], "VoiceOver hears both")
        XCTAssertTrue(privacyButtonShown(panel))
        let button = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSButton.self)
            .first { $0.title == "Open Privacy Settings" })
        button.performClick(nil)
        XCTAssertEqual(opened, 1)

        XCTAssertTrue(panel.attachFromFrontApp())
        asked()[1].1(.failure(.notAnswering))
        XCTAssertTrue(shows(panel, "Safari didn’t answer within 2 seconds."))
        XCTAssertFalse(privacyButtonShown(panel), "only a denial offers the setting")
    }

    func testMailLinksEncodeTheMessageIDInAngleBrackets() {
        XCTAssertEqual(DocketMailSource.link("abc.123@example.com"), "message://%3Cabc.123@example.com%3E")
        XCTAssertEqual(DocketMailSource.link("<abc@example.com>"), "message://%3Cabc@example.com%3E")
        XCTAssertEqual(DocketMailSource.link("a<b>%c@x"), "message://%3Ca%3Cb%3E%25c@x%3E")
        let answer = DocketMailSource.answer(ids: ["one@x", "two@x"], subjects: ["First subject", "Second"])
        XCTAssertEqual(answer, .success(DocketFrontAppAnswer(
            attachments: ["message://%3Cone@x%3E", "message://%3Ctwo@x%3E"], text: "First subject",
            itemTexts: ["First subject", "Second"]
        )))
        XCTAssertEqual(DocketMailSource.answer(ids: [], subjects: []), .failure(.nothingSelected))
        let draft = DocketCaptureDraft.build(text: nil, attachments: ["message://%3Cone@x%3E"])
        XCTAssertEqual(draft.attachmentTokens, " &message://%3Cone@x%3E", "the builder takes it as is")
    }

    func testFinderLeavesOutFolders() {
        let source = DocketFinderSource(isDirectory: { $0.hasPrefix("/dir") })
        XCTAssertEqual(source.answer(paths: ["/tmp/a.pdf", "/Users/me/Projects/", "/dir"]),
                       .success(DocketFrontAppAnswer(attachments: ["/tmp/a.pdf"], text: nil, dropped: [
                           "Left out folder Projects: Docket attaches files",
                           "Left out folder dir: Docket attaches files",
                       ])))
        XCTAssertEqual(source.answer(paths: ["/Users/me/Projects/"]), .failure(.foldersDropped(["Projects"])))
        XCTAssertEqual(source.answer(paths: []), .failure(.nothingSelected))
    }

    func testABrowserTabAttachesItsURLAndALocalFileByPath() {
        XCTAssertEqual(DocketBrowserTabSource.answer(url: "https://example.com", title: " Example "),
                       .success(DocketFrontAppAnswer(attachments: ["https://example.com"], text: "Example")))
        XCTAssertEqual(DocketBrowserTabSource.answer(url: "file:///tmp/a%20b.pdf", title: nil),
                       .success(DocketFrontAppAnswer(attachments: ["/tmp/a b.pdf"], text: nil)))
        XCTAssertEqual(DocketBrowserTabSource.answer(url: nil, title: "Untitled"), .failure(.nothingSelected))
        XCTAssertEqual(DocketBrowserTabSource.answer(url: "", title: nil), .failure(.nothingSelected))
    }

    func testScriptErrorsAndPermissionsMapToRefusals() {
        XCTAssertNil(DocketFrontAppQuery.refusal(forPermission: OSStatus(noErr)))
        XCTAssertEqual(DocketFrontAppQuery.refusal(forPermission: OSStatus(errAEEventNotPermitted)), .notPermitted)
        XCTAssertEqual(DocketFrontAppQuery.refusal(forPermission: OSStatus(errAEEventWouldRequireUserConsent)), .notPermitted)
        XCTAssertEqual(DocketFrontAppQuery.refusal(forPermission: OSStatus(procNotFound)), .notAnswering)
        XCTAssertEqual(DocketFrontAppQuery.refusal(forScriptError: Int(errAEEventNotPermitted)), .notPermitted)
        XCTAssertEqual(DocketFrontAppQuery.refusal(forScriptError: Int(errAETimeout)), .notAnswering)
        XCTAssertEqual(DocketFrontAppQuery.refusal(forScriptError: Int(errAENoSuchObject)), .nothingSelected)
        XCTAssertEqual(DocketFrontAppQuery.refusal(forScriptError: Int(errAEIllegalIndex)), .nothingSelected)
    }
}
