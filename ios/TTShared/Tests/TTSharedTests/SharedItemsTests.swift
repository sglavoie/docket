import Foundation
import TTShared
import UniformTypeIdentifiers
import XCTest

/// The share extension reads what a share sheet hands it — words, links, and
/// files — copies the files while it may, proposes a task, and saves it.
final class SharedItemsTests: XCTestCase {
    private var root: URL!

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("tt-share-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: root)
    }

    func testTextLinksAndFilesAreReadAndFilesCopied() async throws {
        let source = root.appendingPathComponent("Boarding Pass.pdf")
        try Data("boarding".utf8).write(to: source)
        let providers = [
            NSItemProvider(object: "Pack for the trip\nPassport\nCharger" as NSString),
            NSItemProvider(object: URL(string: "https://example.com/itinerary")! as NSURL),
            NSItemProvider(object: "https://example.com/hotel" as NSString),
            try XCTUnwrap(NSItemProvider(contentsOf: source)),
        ]
        let items = await SharedItems.load(providers, scratch: root.appendingPathComponent("scratch"))
        XCTAssertEqual(items.texts, ["Pack for the trip\nPassport\nCharger"])
        XCTAssertEqual(items.urls.map(\.absoluteString), ["https://example.com/itinerary", "https://example.com/hotel"])
        XCTAssertEqual(items.files.map(\.filename), ["Boarding Pass.pdf"])
        let copy = try XCTUnwrap(items.files.first?.url)
        XCTAssertNotEqual(copy, source, "the file must be copied while the provider's grant lasts")
        XCTAssertEqual(try Data(contentsOf: copy), Data("boarding".utf8))
        XCTAssertTrue(items.unreadable.isEmpty)

        let draft = ShareDraft(proposing: items)
        XCTAssertEqual(draft.title, "Pack for the trip")
        XCTAssertEqual(draft.note, "Passport\nCharger")

        let inbox = CaptureInbox(container: root.appendingPathComponent("group"))
        let saved = try draft.save(items, into: inbox, id: UUID(), workspace: "ws")
        XCTAssertEqual(saved.source, .share)
        XCTAssertEqual(saved.urls, ["https://example.com/itinerary", "https://example.com/hotel"])
        XCTAssertEqual(saved.files?.map(\.filename), ["Boarding Pass.pdf"])
        XCTAssertEqual(inbox.pending().count, 1)
    }

    func testADraftIsProposedFromALinkOrAFileAlone() {
        let link = SharedItems(urls: [URL(string: "https://example.com/release/notes")!])
        XCTAssertEqual(ShareDraft(proposing: link).title, "Read: example.com/release/notes")
        let file = SharedItems(files: [SharedFile(filename: "IMG_0001.HEIC", url: URL(fileURLWithPath: "/tmp/x"))])
        XCTAssertEqual(ShareDraft(proposing: file).title, "IMG_0001.HEIC")
        XCTAssertEqual(ShareDraft(proposing: SharedItems()).title, "")
    }
}
