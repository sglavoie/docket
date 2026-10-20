import Foundation
import XCTest
import TTEngine
@testable import tt

final class ConflictPolishTests: XCTestCase {
    @MainActor func testReadFailureAndRetryAreDifferentFromAnEmptyRead() async {
        let read = ConflictReadModel()
        XCTAssertTrue(read.loading)
        await read.load { .failure(ConflictFailure(message: "Cannot read history", stale: false)) }
        XCTAssertFalse(read.loading)
        XCTAssertEqual(read.failure, "Cannot read history")
        await read.load { .success([]) }
        XCTAssertFalse(read.loading)
        XCTAssertNil(read.failure)
        XCTAssertTrue(read.conflicts.isEmpty)
    }

    @MainActor func testLateReadCannotRestoreAnObsoleteError() async {
        let read = ConflictReadModel()
        var release: CheckedContinuation<Result<[SyncConflict], ConflictFailure>, Never>?
        let old = Task {
            await read.load { await withCheckedContinuation { release = $0 } }
        }
        while release == nil { await Task.yield() }
        await read.load { .success([]) }
        release?.resume(returning: .failure(ConflictFailure(message: "Obsolete failure", stale: false)))
        await old.value
        XCTAssertFalse(read.loading)
        XCTAssertNil(read.failure)
    }

    @MainActor func testConflictDraftSurvivesRelaunchAndFailedWritesWithoutChangingItsRevision() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let location = WorkspaceLocation(name: "conflict-drafts", data: root.appendingPathComponent("data"), runtime: root)
        let store = DraftStore(location: location)
        let path = "projects/example/notes/a.md"
        let draft = ConflictDraft(conflictID: "original-conflict", original: "Original", text: "Carefully merged text")
        try Data().write(to: store.directory)
        store.saveConflict(draft, for: path)
        XCTAssertEqual(store.loadConflict(path), draft)
        XCTAssertTrue(store.hasPendingChanges)
        try FileManager.default.removeItem(at: store.directory)
        store.retry()
        let reopened = DraftStore(location: location)
        XCTAssertEqual(reopened.loadConflict(path), draft)
        XCTAssertNotEqual(reopened.file(for: .conflict("a/b.md")), reopened.file(for: .conflict("a_b.md")))
        reopened.discardConflict(path)
        XCTAssertNil(DraftStore(location: location).loadConflict(path))
    }
}
