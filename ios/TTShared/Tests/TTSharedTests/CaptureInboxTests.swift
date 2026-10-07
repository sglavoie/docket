import CryptoKit
import Foundation
import TTShared
import XCTest

/// The inbox an extension saves into: a save publishes once and whole, a
/// terminated save leaves nothing the importer reads, and a retry or a
/// second delivery of the same capture leaves exactly one entry.
final class CaptureInboxTests: XCTestCase {
    private var root: URL!
    private var inbox: CaptureInbox!

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("tt-shared-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        inbox = CaptureInbox(container: root)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: root)
    }

    private func file(_ name: String, _ contents: String) throws -> SharedFile {
        let url = root.appendingPathComponent("source-\(UUID().uuidString)-\(name)")
        try Data(contents.utf8).write(to: url)
        return SharedFile(filename: name, url: url)
    }

    private struct Interrupted: Error {}

    private func entries() throws -> [String] {
        try FileManager.default.contentsOfDirectory(atPath: inbox.directory.path).filter { !$0.hasPrefix(".") }
    }

    private func staging() -> [String] {
        (try? FileManager.default.contentsOfDirectory(
            atPath: inbox.directory.appendingPathComponent(CaptureInbox.stagingName).path
        )) ?? []
    }

    func testSavePublishesTheEnvelopeAndVerifiableFiles() throws {
        let id = UUID()
        let saved = try inbox.save(
            id: id, workspace: "ws-1", source: .share,
            content: .literal(title: "Read: release notes", note: "From Safari"),
            urls: ["https://example.com/notes"], files: [try file("scan.pdf", "PDF bytes")]
        )
        XCTAssertEqual(saved.id, id.uuidString.lowercased())
        XCTAssertEqual(try entries(), [saved.id])
        XCTAssertTrue(staging().isEmpty, "a published save leaves no staging")

        let entry = inbox.directory.appendingPathComponent(saved.id)
        let raw = try Data(contentsOf: entry.appendingPathComponent(CaptureInbox.envelopeName))
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: raw) as? [String: Any])
        XCTAssertEqual(object["format"] as? String, "tt-capture/v1")
        XCTAssertEqual(object["title"] as? String, "Read: release notes")
        XCTAssertNil(object["text"], "a literal capture carries no grammar text")
        XCTAssertNotNil(object["created_at"] as? String)

        let record = try XCTUnwrap(saved.files?.first)
        XCTAssertEqual(record.path, "files/1-scan.pdf")
        let copy = try Data(contentsOf: entry.appendingPathComponent(record.path))
        XCTAssertEqual(String(decoding: copy, as: UTF8.self), "PDF bytes")
        XCTAssertEqual(record.size, 9)
        let expected = SHA256.hash(data: Data("PDF bytes".utf8)).map { String(format: "%02x", $0) }.joined()
        XCTAssertEqual(record.digest, "sha256:" + expected)

        let pending = inbox.pending()
        XCTAssertEqual(pending.map(\.id), [saved.id])
        XCTAssertNil(pending[0].status)
        XCTAssertEqual(pending[0].envelope.summary, "Read: release notes")
    }

    func testATerminatedSaveLeavesNothingPendingAndItsRetryPublishesOnce() throws {
        let id = UUID()
        for stage in [CaptureWriteStage.filesCopied, .envelopeWritten] {
            XCTAssertThrowsError(try inbox.save(
                id: id, workspace: nil, source: .share, content: .literal(title: "Plan the trip", note: ""),
                files: [try file("map.png", "png")],
                fault: { if $0 == stage { throw Interrupted() } }
            ))
            XCTAssertEqual(try entries(), [], "a save terminated at \(stage) published something")
            XCTAssertTrue(inbox.pending().isEmpty)
            XCTAssertEqual(staging(), [id.uuidString.lowercased()])
        }
        try inbox.save(id: id, workspace: nil, source: .share, content: .literal(title: "Plan the trip", note: ""), files: [try file("map.png", "png")])
        XCTAssertEqual(try entries(), [id.uuidString.lowercased()])
        XCTAssertTrue(staging().isEmpty)
        let files = try FileManager.default.contentsOfDirectory(atPath: inbox.directory.appendingPathComponent(id.uuidString.lowercased()).appendingPathComponent("files").path)
        XCTAssertEqual(files, ["1-map.png"], "the retry left a second copy of the file")
    }

    func testDeliveringTheSameCaptureTwiceKeepsOneEntry() throws {
        let id = UUID()
        let first = try inbox.save(
            id: id, workspace: "ws", source: .shortcut, content: .text("Call the dentist #health"),
            createdAt: Date(timeIntervalSince1970: 1_000)
        )
        let second = try inbox.save(
            id: id, workspace: "ws", source: .shortcut, content: .text("Call the dentist #health"),
            createdAt: Date(timeIntervalSince1970: 2_000)
        )
        XCTAssertEqual(second, first, "the second delivery should answer the published capture")
        XCTAssertEqual(try entries(), [first.id])
        XCTAssertEqual(inbox.pending().count, 1)
        XCTAssertTrue(staging().isEmpty)
    }

    func testPendingCapturesShowTheirRefusalUntilRevised() throws {
        let saved = try inbox.save(id: UUID(), workspace: nil, source: .shortcut, content: .text("Offsite $ready"))
        let older = try inbox.save(
            id: UUID(), workspace: nil, source: .share, content: .literal(title: "Older", note: ""),
            createdAt: Date(timeIntervalSince1970: 0)
        )
        let status = """
        {"state": "refused", "code": "validation", "message": "line 1: $ready is retired", \
        "lines": [{"line": 1, "column": 8, "message": "$ready is retired"}], "attempted_at": "2026-10-06T09:00:00.123Z"}
        """
        try Data(status.utf8).write(to: inbox.directory.appendingPathComponent(saved.id).appendingPathComponent(CaptureInbox.statusName))

        let pending = inbox.pending()
        XCTAssertEqual(pending.map(\.id), [older.id, saved.id], "pending captures read oldest first")
        XCTAssertTrue(pending[1].isRefused)
        XCTAssertEqual(pending[1].status?.lines?.first?.column, 8)

        try inbox.revise(id: saved.id, content: .text("Offsite $next"))
        let revised = try XCTUnwrap(inbox.pending().first { $0.id == saved.id })
        XCTAssertNil(revised.status, "revising clears the refusal so the next import tries again")
        XCTAssertEqual(revised.envelope.text, "Offsite $next")
        XCTAssertEqual(revised.envelope.createdAt.timeIntervalSince1970, saved.createdAt.timeIntervalSince1970, accuracy: 0.001)

        try inbox.discard(id: older.id)
        XCTAssertEqual(inbox.pending().map(\.id), [saved.id])
    }

    func testFilenamesStayInsideTheirEntry() throws {
        let saved = try inbox.save(
            id: UUID(), workspace: nil, source: .share, content: .literal(title: "", note: ""),
            files: [try file("evil.txt", "x"), try file("b.txt", "y")].enumerated().map { index, file in
                SharedFile(filename: index == 0 ? "../../escape.txt" : ".hidden", url: file.url)
            }
        )
        XCTAssertEqual(saved.files?.map(\.path), ["files/1-escape.txt", "files/2-shared-2.hidden"])
        XCTAssertEqual(saved.summary, "escape.txt")
    }

    func testNothingToSaveIsRefused() {
        XCTAssertThrowsError(try inbox.save(id: UUID(), workspace: nil, source: .shortcut, content: .text("  \n")))
        XCTAssertThrowsError(try inbox.save(id: UUID(), workspace: nil, source: .share, content: .literal(title: " ", note: "x")))
        XCTAssertTrue(inbox.pending().isEmpty)
    }
}
