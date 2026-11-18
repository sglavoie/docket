import AppKit
import XCTest
@testable import Docket

/// Every route into quick capture from outside it: `docket://` links, the
/// Services menu, the Share menu and Cmd+K. None of them sends a line.
final class CaptureRoutesTests: XCTestCase {
    // MARK: Links

    func testLinksReadTheirRoute() {
        func link(_ text: String) -> DocketLink? { DocketLink(url: URL(string: text)!) }
        XCTAssertEqual(link("docket://"), .show)
        XCTAssertEqual(link("docket://show"), .show)
        XCTAssertEqual(link("DOCKET://Show"), .show)
        XCTAssertEqual(link("docket://capture"), .capture(nil))
        XCTAssertEqual(link("docket://capture?text=Buy%20milk%20%25home"), .capture("Buy milk %home"))
        XCTAssertEqual(link("docket://capture?text=%20%20"), .capture(nil))
        XCTAssertEqual(link("docket://capture?text=two%0Alines"), .capture("two lines"))
        XCTAssertEqual(
            link("docket://capture?text=Read&url=https://a.example/x&url=https://b.example"),
            .capture("Read", urls: ["https://a.example/x", "https://b.example"])
        )
        XCTAssertNil(link("docket://delete"))
        XCTAssertNil(link("docket://capture/extra"))
        XCTAssertNil(link("https://example.com"))
    }

    func testALinkSpellsWhatItReads() {
        for link in [DocketLink.show, .capture(nil), .capture("A #tag & 50% off?"),
                     .capture("Title", urls: ["https://example.com/a?b=c&d=e", "/Users/me/My File.pdf"])] {
            XCTAssertEqual(DocketLink(url: link.url), link, link.url.absoluteString)
        }
    }

    // MARK: Drafts

    func testATitleIsSelectedInFrontOfItsLinks() {
        XCTAssertNil(DocketCaptureDraft.build(title: nil))
        XCTAssertNil(DocketCaptureDraft.build(title: "  \n", links: [" "]))
        XCTAssertEqual(DocketCaptureDraft.build(title: " Buy\n milk "), DocketCaptureDraft(text: "Buy milk"))
        XCTAssertEqual(
            DocketCaptureDraft.build(title: "Café ☕", links: ["https://example.com"]),
            DocketCaptureDraft(text: "Café ☕ https://example.com", caret: 0, selectionLength: 6)
        )
        XCTAssertEqual(
            DocketCaptureDraft.build(title: nil, links: ["https://a.example", "/tmp/b"]),
            DocketCaptureDraft(text: " https://a.example /tmp/b", caret: 0)
        )
    }

    func testAServicesSelectionIsTextOrALink() {
        XCTAssertEqual(DocketCaptureDraft.from(text: "Call the\nbank", url: nil), DocketCaptureDraft(text: "Call the bank"))
        XCTAssertEqual(
            DocketCaptureDraft.from(text: " https://example.com/x ", url: nil),
            DocketCaptureDraft(text: " https://example.com/x", caret: 0)
        )
        XCTAssertEqual(
            DocketCaptureDraft.from(text: nil, url: URL(string: "https://example.com")),
            DocketCaptureDraft(text: " https://example.com", caret: 0)
        )
        XCTAssertNil(DocketCaptureDraft.from(text: nil, url: URL(string: "file:///tmp/a")))
        XCTAssertNil(DocketCaptureDraft.from(text: "   ", url: nil))
    }

    func testTheServiceReadsThePasteboard() {
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("docket-test-\(UUID().uuidString)"))
        defer { pasteboard.releaseGlobally() }
        pasteboard.clearContents()
        pasteboard.setString("Renew the passport", forType: .string)
        let provider = DocketServiceProvider()
        var presented: DocketCaptureDraft?
        provider.present = { presented = $0 }
        var error: NSString?
        provider.captureSelection(pasteboard, userData: nil, error: &error)
        XCTAssertNil(error)
        XCTAssertEqual(presented, DocketCaptureDraft(text: "Renew the passport"))

        pasteboard.clearContents()
        presented = nil
        provider.captureSelection(pasteboard, userData: nil, error: &error)
        XCTAssertNil(presented)
        XCTAssertEqual(error, "There is no text to capture.")
    }

    // MARK: Share

    private struct FakeProvider: DocketShareItemProvider {
        var types: [String]
        var value: Any?

        func hasItemConformingToTypeIdentifier(_ typeIdentifier: String) -> Bool { types.contains(typeIdentifier) }
        func loadDocketItem(_ typeIdentifier: String, completion: @escaping (Any?) -> Void) {
            DispatchQueue.global().async { completion(value) }
        }
    }

    func testAShareBecomesOneCaptureLink() {
        let page = FakeProvider(types: [DocketShare.urlType, DocketShare.plainTextType], value: URL(string: "https://example.com/post")!)
        let file = FakeProvider(types: [DocketShare.fileURLType, DocketShare.urlType], value: URL(fileURLWithPath: "/tmp/Report 2.pdf"))
        let text = FakeProvider(types: [DocketShare.plainTextType], value: "Some\nquote")
        let done = expectation(description: "loaded")
        DocketShare.load([DocketShareInput(contentText: "A post", providers: [page, file, text])]) { link in
            XCTAssertTrue(Thread.isMainThread)
            XCTAssertEqual(link, .capture("A post", urls: ["https://example.com/post", "/tmp/Report 2.pdf"]))
            done.fulfill()
        }
        wait(for: [done], timeout: 5)

        XCTAssertEqual(DocketShare.link(contentText: nil, items: [.text(" \n"), .text("Quote here")]), .capture("Quote here"))
        XCTAssertNil(DocketShare.link(contentText: "  ", items: []))
        XCTAssertNil(DocketShare.item(42, type: DocketShare.urlType))
    }

    // MARK: Cmd+K

    func testTheAppsCmdKReads() {
        XCTAssertEqual(DocketFrontApp(bundleIdentifier: "com.apple.safari"), .safari)
        XCTAssertEqual(DocketFrontApp(bundleIdentifier: "com.brave.Browser"), .brave)
        XCTAssertNil(DocketFrontApp(bundleIdentifier: "io.github.sglavoie.docket"))
        XCTAssertNil(DocketFrontApp(bundleIdentifier: nil))
        XCTAssertEqual(QuickCapturePanel.hint(for: nil), QuickCapturePanel.hint)
        XCTAssertEqual(QuickCapturePanel.hint(for: .mail), "Enter adds it · ⌘K adds the Mail message · Esc keeps it for later")
    }

    func testEachAppAnswersWithLinksAndATitle() {
        XCTAssertEqual(
            try DocketBrowserTabSource.answer(url: "https://example.com", title: " Example ").get(),
            DocketFrontAppAnswer(links: ["https://example.com"], title: "Example")
        )
        XCTAssertEqual(
            try DocketBrowserTabSource.answer(url: "file:///tmp/a%20b.html", title: "").get(),
            DocketFrontAppAnswer(links: ["/tmp/a b.html"], title: nil)
        )
        XCTAssertEqual(DocketBrowserTabSource.answer(url: nil, title: "x"), .failure(.nothingSelected))

        XCTAssertEqual(DocketMailSource.link("<abc@example.com>"), "message://%3Cabc@example.com%3E")
        XCTAssertEqual(DocketMailSource.link("a b%c"), "message://%3Ca%20b%25c%3E")
        XCTAssertEqual(
            try DocketMailSource.answer(ids: ["<1@x>", "<2@x>"], subjects: ["Invoice", "Re: Invoice"]).get(),
            DocketFrontAppAnswer(links: ["message://%3C1@x%3E", "message://%3C2@x%3E"], title: "Invoice")
        )
        XCTAssertEqual(DocketMailSource.answer(ids: [], subjects: []), .failure(.nothingSelected))

        XCTAssertEqual(
            try DocketFinderSource.answer(paths: ["/tmp/a.txt", "", "/tmp/dir/"]).get(),
            DocketFrontAppAnswer(links: ["/tmp/a.txt", "/tmp/dir/"], title: nil)
        )
        XCTAssertEqual(DocketFinderSource.answer(paths: [""]), .failure(.nothingSelected))
        XCTAssertEqual(
            DocketFrontAppRefusal.notPermitted.status(for: .chrome),
            "Docket isn’t allowed to read Chrome: allow it in System Settings › Privacy & Security › Automation."
        )
    }

    func testAnAnswerFillsAnEmptyLineAndExtendsATypedOne() {
        let answer = DocketFrontAppAnswer(links: ["https://example.com"], title: "Example")
        XCTAssertEqual(answer.applied(to: " "), DocketCaptureDraft(text: "Example https://example.com", caret: 0, selectionLength: 7))
        XCTAssertEqual(answer.applied(to: "Read this #web"), DocketCaptureDraft(text: "Read this #web https://example.com"))
        XCTAssertNil(answer.applied(to: "Read https://example.com later"))
    }

    // MARK: The panel

    private func panel(front: String? = "com.apple.Safari") -> QuickCapturePanel {
        let panel = QuickCapturePanel()
        panel.frontmostBundleID = { front }
        addTeardownBlock { panel.hide() }
        return panel
    }

    func testPresentingFillsAnEmptyLineAndKeepsATypedOne() {
        let panel = panel(front: nil)
        panel.present(DocketCaptureDraft(text: "Call the bank"))
        XCTAssertTrue(panel.isVisible)
        XCTAssertEqual(panel.text, "Call the bank")
        XCTAssertEqual(panel.statusText, QuickCapturePanel.hint)

        panel.hide()
        panel.text = "Half typed"
        panel.present(DocketCaptureDraft(text: " https://example.com", caret: 0))
        XCTAssertEqual(panel.text, "Half typed https://example.com")
        XCTAssertEqual(panel.statusText, "Added after the line you had not sent.")
    }

    func testCmdKAddsWhatTheAppInFrontShows() {
        let panel = panel()
        var asked: [DocketFrontApp] = []
        panel.readFrontApp = { app, done in
            asked.append(app)
            done(.success(DocketFrontAppAnswer(links: ["https://example.com"], title: "Example")))
        }
        panel.show()
        XCTAssertEqual(panel.frontApp, .safari)
        XCTAssertEqual(panel.statusText, "Enter adds it · ⌘K adds the Safari tab · Esc keeps it for later")
        XCTAssertTrue(panel.addFrontAppContext())
        XCTAssertEqual(asked, [.safari])
        XCTAssertEqual(panel.text, "Example https://example.com")

        XCTAssertTrue(panel.addFrontAppContext())
        XCTAssertEqual(panel.text, "Example https://example.com")
        XCTAssertEqual(panel.statusText, "The line already holds the Safari tab.")
    }

    func testCmdKSaysWhyItAddedNothing() {
        let panel = panel(front: "com.apple.mail")
        panel.readFrontApp = { _, done in done(.failure(.nothingSelected)) }
        panel.show()
        XCTAssertTrue(panel.addFrontAppContext())
        XCTAssertEqual(panel.text, "")
        XCTAssertEqual(panel.statusText, "No message is selected in Mail.")

        let other = self.panel(front: "com.apple.TextEdit")
        other.show()
        XCTAssertNil(other.frontApp)
        XCTAssertFalse(other.addFrontAppContext())
    }

    func testALateAnswerFindsThePanelGone() {
        let panel = panel()
        var answer: ((Result<DocketFrontAppAnswer, DocketFrontAppRefusal>) -> Void)?
        panel.readFrontApp = { _, done in answer = done }
        panel.show()
        panel.addFrontAppContext()
        panel.hide()
        answer?(.success(DocketFrontAppAnswer(links: ["https://example.com"], title: nil)))
        XCTAssertEqual(panel.text, "")
    }

    // MARK: The bundle

    func testTheBundleDeclaresItsRoutes() throws {
        let info = try XCTUnwrap(Bundle.main.infoDictionary)
        let schemes = (info["CFBundleURLTypes"] as? [[String: Any]])?.flatMap { $0["CFBundleURLSchemes"] as? [String] ?? [] }
        XCTAssertEqual(schemes, ["docket"])
        let services = try XCTUnwrap(info["NSServices"] as? [[String: Any]])
        XCTAssertEqual(services.first?["NSMessage"] as? String, "captureSelection")
        XCTAssertNotNil(info["NSAppleEventsUsageDescription"])
        let share = Bundle.main.builtInPlugInsURL?.appendingPathComponent("DocketShare.appex")
        XCTAssertTrue(FileManager.default.fileExists(atPath: share?.path ?? ""), "DocketShare.appex is embedded")
    }
}
