import Foundation
import XCTest
import DocketEngine
@testable import Docket

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

    @MainActor func testCaptureCompletionCannotClearALaterRevisionEvenWithTheSameText() throws {
        let store = try store()
        var submitted = CaptureDraft()
        submitted.text = "Send proposal"
        let revision = store.saveCapture(submitted)
        var later = submitted
        later.text = "Send revised proposal"
        store.saveCapture(later)
        XCTAssertFalse(store.finishCapture(submitted, revision: revision))
        XCTAssertEqual(store.loadCapture(), later)
        // A → B → A is still a later edit, not the submitted revision.
        store.saveCapture(submitted)
        XCTAssertFalse(store.finishCapture(submitted, revision: revision))
        XCTAssertEqual(store.loadCapture(), submitted)
        let next = store.saveCapture(submitted)
        XCTAssertTrue(store.finishCapture(submitted, revision: next))
        XCTAssertNil(store.loadCapture())
    }

    @MainActor func testEditCompletionRebasesLaterChangesIncludingARevert() throws {
        let store = try store()
        let locator = TaskLocator(projectID: "project", taskID: "task")
        var original = TaskFields()
        original.title = "Original title"
        var submittedFields = original
        submittedFields.title = "Submitted title"
        let submitted = EditDraft(original: original, fields: submittedFields)
        let revision = store.saveEdit(submitted, for: locator)
        var later = submitted
        later.fields.title = original.title
        later.fields.context = "phone"
        store.saveEdit(later, for: locator)
        let retained = try XCTUnwrap(store.finishEdit(submitted, revision: revision, for: locator))
        XCTAssertEqual(retained.original.title, "Submitted title")
        XCTAssertEqual(retained.fields.title, "Original title")
        let patch = retained.fields.edit(from: retained.original)
        XCTAssertEqual(patch.title, "Original title")
        XCTAssertEqual(patch.context, "phone")
        XCTAssertEqual(store.loadEdit(locator), retained)
        let next = store.saveEdit(retained, for: locator)
        XCTAssertNil(store.finishEdit(retained, revision: next, for: locator))
        XCTAssertNil(store.loadEdit(locator))
    }
}
