import AppKit
import XCTest
@testable import Docket

final class DocketVocabularyTests: XCTestCase {
    static let export = """
    Inbox
    \tCall the bank #money #home ##phone
    \tFix the shed #home @2026-12-01 $doing
    \t> a body line #notatag
    \tRead pull request #212 #home #p=1
    Alpha ~teal
    \tTUI:
    \t\tDraw the list #home ##office
    \tDocs:
    Client Work $parked
    \tTUI:
    """

    func testItReadsTagsContextsAndPlacesFromTheExport() {
        let v = DocketVocabulary(export: Self.export)
        XCTAssertEqual(v.tags.map(\.name), ["home", "money", "p"])
        XCTAssertEqual(v.tags.map(\.count), [4, 1, 1])
        XCTAssertEqual(v.contexts.map(\.name), ["office", "phone"])
        XCTAssertEqual(v.places, ["Inbox", "Alpha", "Client Work", "Alpha › TUI", "Alpha › Docs", "Client Work › TUI"])
        XCTAssertEqual(v.projects.map(\.name), ["Alpha", "Client Work"])
        XCTAssertEqual(v.projects.map(\.word), ["alpha", "client"])
    }
}

final class DocketSigilCompleterTests: XCTestCase {
    private let vocabulary = DocketVocabulary(export: DocketVocabularyTests.export)

    private func complete(_ before: String) -> DocketCompletion? {
        DocketSigilCompleter.complete(before, vocabulary: vocabulary)
    }

    func testATagListsTheTagsInUseMostUsedFirst() {
        XCTAssertEqual(complete("Call Sam #")?.items.map(\.insert), ["#home", "#money", "#p"])
        let ho = complete("Call Sam #HO")
        XCTAssertEqual(ho?.token, "#HO")
        XCTAssertEqual(ho?.items, [.init(label: "#home", detail: "4", insert: "#home")])
        XCTAssertEqual(ho?.exact, false)
    }

    func testAContextStateDateAndColourComplete() {
        XCTAssertEqual(complete("x ##o")?.items.map(\.insert), ["##office"])
        XCTAssertEqual(complete("x $wa")?.items.map(\.insert), ["$waiting"])
        XCTAssertEqual(complete("x @t")?.items.map(\.insert), ["@today", "@tomorrow", "@tue", "@thu"])
        XCTAssertEqual(complete("x @@fr")?.items.map(\.insert), ["@@fri"])
        XCTAssertEqual(complete("x ~te")?.items.map(\.insert), ["~teal"])
    }

    func testNothingCompletesAfterASpaceOrAPlainWordOrAFinishedSigil() {
        XCTAssertNil(complete("Call Sam #home "))
        XCTAssertNil(complete("Call Sam"))
        XCTAssertNil(complete("Call Sam #home"))
        XCTAssertNil(complete("Call Sam #nothing"))
        XCTAssertNil(complete(""))
    }

    func testAWordThatStandsAsTypedIsExact() {
        let d = complete("x $do")
        XCTAssertEqual(d?.items.map(\.insert), ["$doing", "$done"])
        XCTAssertEqual(d?.exact, false)
        XCTAssertEqual(complete("x @t")?.exact, false)
    }

    func testANameOffersOnlyTopLevelProjects() {
        XCTAssertEqual(complete("Call Sam %")?.items, [
            .init(label: "Alpha", detail: "", insert: "%alpha"),
            .init(label: "Client Work", detail: "", insert: "%client"),
        ])
        let cl = complete("Call Sam %cl")
        XCTAssertEqual(cl?.items.map(\.label), ["Client Work"])
        XCTAssertEqual(cl?.exact, true)
        // Headings are not offered, nor the Inbox, where a line goes anyway.
        XCTAssertNil(complete("Call Sam %tu"))
        XCTAssertNil(complete("Call Sam %do"))
        XCTAssertNil(complete("Call Sam %inb"))
    }

    func testRankFollowsTheCaptureLine() {
        XCTAssertEqual(DocketSigilCompleter.rank("Client Work", "cli"), 0)
        XCTAssertEqual(DocketSigilCompleter.rank("Client Work", "work"), 1)
        XCTAssertEqual(DocketSigilCompleter.rank("Client Work", "ork"), 2)
        XCTAssertEqual(DocketSigilCompleter.rank("Client Work", "cwk"), 3)
        XCTAssertNil(DocketSigilCompleter.rank("Client Work", "zz"))
    }
}

final class QuickCaptureCompletionPanelTests: XCTestCase {
    private var panel: QuickCapturePanel!

    override func setUp() {
        panel = QuickCapturePanel()
        panel.frontmostBundleID = { nil }
        panel.readVocabulary = { done in done(DocketVocabulary(export: DocketVocabularyTests.export)) }
    }

    override func tearDown() {
        panel.hide()
        panel = nil
    }

    private func type(_ text: String) {
        guard let editor = panel.panel.firstResponder as? NSTextView else {
            return XCTFail("the field is not being edited")
        }
        editor.insertText(text, replacementRange: editor.selectedRange)
    }

    private func press(_ selector: Selector) -> Bool {
        guard let editor = panel.panel.firstResponder as? NSTextView else { return false }
        return panel.control(NSTextField(), textView: editor, doCommandBy: selector)
    }

    func testTypingASigilListsItsValuesAndTabTakesOne() {
        panel.show()
        type("Call Sam #")
        XCTAssertEqual(panel.statusText, QuickCapturePanel.listHint)
        // A key typed while the list stays open keeps saying how to use it.
        type("h")
        XCTAssertEqual(panel.completion?.items.map(\.insert), ["#home"])
        XCTAssertEqual(panel.statusText, QuickCapturePanel.listHint)
        XCTAssertTrue(press(#selector(NSResponder.insertTab(_:))))
        XCTAssertEqual(panel.text, "Call Sam #home ")
        XCTAssertNil(panel.completion)
        XCTAssertEqual(panel.statusText, QuickCapturePanel.hint)
    }

    func testArrowsChooseAndEnterTakesTheChoice() {
        var sent: [String] = []
        panel.submit = { line, done in
            sent.append(line)
            done(true, "")
        }
        panel.show()
        type("x $do")
        XCTAssertTrue(press(#selector(NSResponder.moveDown(_:))))
        XCTAssertTrue(press(#selector(NSResponder.insertNewline(_:))))
        XCTAssertEqual(panel.text, "x $done ")
        XCTAssertEqual(sent, [])
    }

    func testEnterSendsAPlaceTheNameAlreadyReaches() {
        var sent: [String] = []
        panel.submit = { line, done in
            sent.append(line)
            done(true, "")
        }
        panel.show()
        type("Call Sam %cl")
        XCTAssertNotNil(panel.completion)
        XCTAssertTrue(press(#selector(NSResponder.insertNewline(_:))))
        XCTAssertEqual(sent, ["Call Sam %cl"])
    }

    func testEscClosesTheListBeforeThePanel() {
        panel.show()
        type("x ~te")
        XCTAssertNotNil(panel.completion)
        XCTAssertTrue(press(#selector(NSResponder.cancelOperation(_:))))
        XCTAssertNil(panel.completion)
        XCTAssertTrue(panel.isVisible)
        XCTAssertEqual(panel.text, "x ~te")
        XCTAssertTrue(press(#selector(NSResponder.cancelOperation(_:))))
        XCTAssertFalse(panel.isVisible)
    }

    func testTakingOneMidLineKeepsWhatFollows() {
        panel.show()
        type("Call #m Sam")
        guard let editor = panel.panel.firstResponder as? NSTextView else { return XCTFail() }
        editor.selectedRange = NSRange(location: 7, length: 0)
        panel.refreshCompletion()
        XCTAssertEqual(panel.completion?.items.map(\.insert), ["#money"])
        XCTAssertTrue(press(#selector(NSResponder.insertTab(_:))))
        XCTAssertEqual(panel.text, "Call #money Sam")
        XCTAssertEqual(editor.selectedRange.location, 12)
    }
}
