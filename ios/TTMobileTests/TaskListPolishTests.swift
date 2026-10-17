import Foundation
import XCTest
import TTEngine
@testable import tt

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
        XCTAssertEqual(TaskListLoadFailure.classify(TTEngineError.notOpen, hasMatchingContent: true, cancelled: false), .replace)
    }
}
