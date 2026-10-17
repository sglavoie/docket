import Foundation
import XCTest
import TTEngine
@testable import tt

final class DraftStoreTests: XCTestCase {
    @MainActor private func store() throws -> DraftStore {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try FileManager.default.removeItem(at: root) }
        return DraftStore(location: WorkspaceLocation(name: "draft-tests", data: root.appendingPathComponent("data"), runtime: root))
    }

    @MainActor func testFailedWriteKeepsLatestTextAndRetryPersistsIt() throws {
        let store = try store()
        // A regular file where a directory belongs reliably fails, even as root.
        try Data("unavailable".utf8).write(to: store.directory)
        var draft = CaptureDraft()
        draft.text = "First version"
        store.saveCapture(draft)
        draft.text = "Latest version"
        store.saveCapture(draft)
        XCTAssertEqual(store.loadCapture()?.text, "Latest version")
        XCTAssertNotNil(store.issues[.capture])
        XCTAssertTrue(store.hasPendingChanges)
        try FileManager.default.removeItem(at: store.directory)
        store.retry()
        XCTAssertNil(store.issues[.capture])
        XCTAssertFalse(store.hasPendingChanges)
        let saved = try JSONDecoder().decode(CaptureDraft.self, from: Data(contentsOf: store.file(for: .capture)))
        XCTAssertEqual(saved.text, "Latest version")
    }

    @MainActor func testMissingDraftIsQuietButCorruptDraftIsPreservedBeforeReplacement() throws {
        let store = try store()
        XCTAssertNil(store.loadCapture())
        XCTAssertTrue(store.issues.isEmpty)
        try FileManager.default.createDirectory(at: store.directory, withIntermediateDirectories: true)
        let broken = Data("{unfinished recovery data".utf8)
        try broken.write(to: store.file(for: .capture))
        XCTAssertNil(store.loadCapture())
        XCTAssertNotNil(store.issues[.capture])
        var draft = CaptureDraft()
        draft.text = "New text"
        store.saveCapture(draft)
        let recovery = try XCTUnwrap(store.issues[.capture]?.recovery)
        XCTAssertEqual(try Data(contentsOf: recovery), broken)
        XCTAssertEqual(store.loadCapture()?.text, "New text")
        store.discardCapture()
        XCTAssertEqual(try Data(contentsOf: recovery), broken)
    }

    @MainActor func testAllEditorDraftsShareFailureAndRecoveryBehavior() throws {
        let store = try store()
        let locator = TaskLocator(projectID: "project", taskID: "task")
        try Data().write(to: store.directory)
        var fields = TaskFields()
        fields.title = "Edited title"
        store.saveEdit(EditDraft(original: TaskFields(), fields: fields), for: locator)
        store.saveComment(CommentDraft(editingIndex: nil, editingID: nil, original: nil, text: "Comment"), for: "comment")
        store.saveNote(NoteDraft(location: .global, filename: nil, revision: nil, title: "Note", body: "Body"), for: "note")
        XCTAssertEqual(store.loadEdit(locator)?.fields.title, "Edited title")
        XCTAssertEqual(store.loadComment("comment")?.text, "Comment")
        XCTAssertEqual(store.loadNote("note")?.body, "Body")
        XCTAssertEqual(store.issues.count, 3)
        try FileManager.default.removeItem(at: store.directory)
        store.retry()
        XCTAssertTrue(store.issues.isEmpty)
        XCTAssertFalse(store.hasPendingChanges)
    }

    @MainActor func testRetryAfterUnreadableOriginalWasRemoved() throws {
        let store = try store()
        try FileManager.default.createDirectory(at: store.directory, withIntermediateDirectories: true)
        try Data("broken".utf8).write(to: store.file(for: .capture))
        XCTAssertNil(store.loadCapture())
        try FileManager.default.removeItem(at: store.file(for: .capture))
        var draft = CaptureDraft()
        draft.text = "Recovered editing session"
        store.saveCapture(draft)
        XCTAssertEqual(store.loadCapture()?.text, draft.text)
        XCTAssertFalse(store.hasPendingChanges)
    }

    @MainActor func testFailedDiscardIsRetriedWithoutRestoringTheDraft() throws {
        let store = try store()
        try Data("unavailable".utf8).write(to: store.directory)
        var draft = CaptureDraft()
        draft.text = "Already committed"
        store.saveCapture(draft)
        store.discardCapture()
        XCTAssertNil(store.loadCapture())
        XCTAssertTrue(store.hasPendingChanges)
        try FileManager.default.removeItem(at: store.directory)
        store.retry()
        XCTAssertFalse(store.hasPendingChanges)
        XCTAssertNil(store.loadCapture())
    }
}
