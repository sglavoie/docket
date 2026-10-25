import Foundation
import XCTest
import DocketEngine
@testable import Docket

final class TaskListPolishTests: XCTestCase {
    @MainActor func testReceiptNamesCommittedTaskAndUndoRestoresItsPlan() async throws {
        let workspace = WorkspaceModel()
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try await workspace.engine.open(dataDirectory: root.appendingPathComponent("workspace"),
                                        runtimeDirectory: root.appendingPathComponent("runtime"),
                                        deviceID: "receipt-tests", createIfMissing: true)
        addTeardownBlock {
            try await workspace.engine.close()
            try FileManager.default.removeItem(at: root)
        }
        let task = try await workspace.engine.capture(title: "Send proposal", destination: .today).task
        let receipt = try await workspace.listAction(task.locator, .transition(.done)).get()
        XCTAssertEqual(receipt.message, "Completed “Send proposal”")
        XCTAssertNotNil(receipt.transactionID)
        let failure = await workspace.undoReceipt(receipt)
        XCTAssertNil(failure)
        let restored = try await workspace.engine.task(task.locator)
        XCTAssertEqual(restored.task.state, .backlog)
        XCTAssertEqual(restored.today, true)

        let planned = try await workspace.listAction(task.locator, .plan(.addNext)).get()
        XCTAssertEqual(planned.message, "Moved to Next: “Send proposal”")
        _ = try await workspace.engine.capture(title: "A newer action")
        let stale = await workspace.undoReceipt(planned)
        XCTAssertNotNil(stale)
        let unchanged = try await workspace.engine.task(task.locator)
        XCTAssertEqual(unchanged.next, true)
    }

    @MainActor func testRefreshFailureKeepsOnlyMatchingContentAndIgnoresCancellation() {
        let transient = CocoaError(.fileReadNoPermission)
        guard case .refresh(let message) = TaskListLoadFailure.classify(transient, hasMatchingContent: true, cancelled: false) else {
            return XCTFail("a transient refresh discarded its last successful content")
        }
        XCTAssertTrue(message.contains("Showing previous results"))
        XCTAssertEqual(TaskListLoadFailure.classify(transient, hasMatchingContent: false, cancelled: false), .replace)
        XCTAssertEqual(TaskListLoadFailure.classify(transient, hasMatchingContent: true, cancelled: true), .ignored)
        XCTAssertEqual(TaskListLoadFailure.classify(CancellationError(), hasMatchingContent: true, cancelled: false), .ignored)
        for code in ["validation", "not_found", "not_open", "workspace_mismatch", "schema_too_new", "migration_required"] {
            let failure = BridgeError(rawCode: code, message: "Unavailable", retryable: false)
            XCTAssertEqual(TaskListLoadFailure.classify(failure, hasMatchingContent: true, cancelled: false), .replace, code)
        }
        XCTAssertEqual(TaskListLoadFailure.classify(DocketEngineError.notOpen, hasMatchingContent: true, cancelled: false), .replace)
    }
}

extension TaskListPolishTests {
    @MainActor func testDetailAndBulkActionsPublishReceiptsAndUndoOnlyTheirTransaction() async throws {
        let workspace = WorkspaceModel()
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try await workspace.engine.open(dataDirectory: root.appendingPathComponent("workspace"),
                                        runtimeDirectory: root.appendingPathComponent("runtime"),
                                        deviceID: "detail-receipt-tests", createIfMissing: true)
        addTeardownBlock {
            try await workspace.engine.close()
            try FileManager.default.removeItem(at: root)
        }
        let first = try await workspace.engine.capture(title: "First", destination: .today).task
        let second = try await workspace.engine.capture(title: "Second", destination: .today).task
        let completed = await workspace.transition(first.locator, to: .done)
        XCTAssertNil(completed)
        let detailReceipt = try XCTUnwrap(workspace.actionReceipt)
        XCTAssertEqual(detailReceipt.message, "Completed “First”")
        let detailUndo = await workspace.undoReceipt(detailReceipt)
        XCTAssertNil(detailUndo)
        let restored = try await workspace.engine.task(first.locator)
        XCTAssertTrue(restored.today == true)
        XCTAssertEqual(restored.task.state, .backlog)

        let bulk = await workspace.bulk([first.locator, second.locator], .plan(.addNext))
        XCTAssertNil(bulk.refusal)
        let bulkReceipt = try XCTUnwrap(workspace.actionReceipt)
        XCTAssertEqual(bulkReceipt.message, "Moved to Next: 2 tasks")
        // An old view's timer or dismissal must not erase a newer receipt.
        workspace.dismissReceipt(detailReceipt.id)
        XCTAssertEqual(workspace.actionReceipt?.id, bulkReceipt.id)
        let bulkUndo = await workspace.undoReceipt(bulkReceipt)
        XCTAssertNil(bulkUndo)
        for task in [first, second] {
            let restored = try await workspace.engine.task(task.locator)
            XCTAssertTrue(restored.today == true)
            XCTAssertFalse(restored.next == true)
        }
        let planned = await workspace.plan(first.locator, .addNext)
        XCTAssertNil(planned)
        let previous = try XCTUnwrap(workspace.actionReceipt)
        let checklist = await workspace.checklist(second.locator, .add("Check wording"), seen: [])
        XCTAssertNil(checklist)
        let latest = try XCTUnwrap(workspace.actionReceipt)
        let refused = await workspace.undoReceipt(previous)
        XCTAssertNotNil(refused)
        XCTAssertEqual(workspace.actionReceipt?.id, latest.id)
        let checklistUndo = await workspace.undoReceipt(latest)
        XCTAssertNil(checklistUndo)
        let unchanged = try await workspace.engine.task(first.locator)
        XCTAssertTrue(unchanged.next == true)
        let cleared = try await workspace.engine.task(second.locator)
        XCTAssertTrue(cleared.task.checklist?.isEmpty ?? true)
    }
}
