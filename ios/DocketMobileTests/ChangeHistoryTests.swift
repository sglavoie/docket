import Foundation
import XCTest
import DocketEngine
@testable import Docket

final class ChangeHistoryTests: XCTestCase {
    @MainActor func testHistorySurvivesRestartAndRecoversExactTransactions() async throws {
        let workspace = WorkspaceModel()
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let data = root.appendingPathComponent("workspace")
        let runtime = root.appendingPathComponent("runtime")
        try await workspace.engine.open(dataDirectory: data, runtimeDirectory: runtime,
                                        deviceID: "history-tests", createIfMissing: true)
        addTeardownBlock {
            try await workspace.engine.close()
            try FileManager.default.removeItem(at: root)
        }
        let task = try await workspace.engine.capture(title: "Original title").task
        var edit = TaskEdit()
        edit.title = "Accidental edit"
        let edited = try await workspace.engine.edit(task.locator, edit)
        let transactionID = edited.committed.transactionID
        try await workspace.engine.close()
        try await workspace.engine.open(dataDirectory: data, runtimeDirectory: runtime, deviceID: "history-tests")

        let model = ChangeHistoryModel()
        await model.load(engine: workspace.engine)
        XCTAssertNil(model.failure)
        XCTAssertFalse(model.loading)
        XCTAssertEqual(model.history?.entries.count, 2)
        XCTAssertEqual(model.history?.nextUndo, transactionID)
        XCTAssertEqual(model.history?.entries.first?.undone, false)
        XCTAssertFalse(model.history?.entries.first?.projects.isEmpty ?? true)
        let details = try await workspace.engine.historyDetails(transactionID: transactionID)
        XCTAssertTrue(details.contains { $0.field == "Title" && $0.before == "Original title" && $0.after == "Accidental edit" })

        let generation = workspace.generation
        let undoFailure = await model.restore(transactionID: transactionID, redo: false, description: "Edit title", workspace: workspace)
        XCTAssertNil(undoFailure)
        XCTAssertGreaterThan(workspace.generation, generation)
        XCTAssertFalse(workspace.undoing)
        XCTAssertFalse(model.refused)
        await model.load(engine: workspace.engine)
        XCTAssertEqual(model.history?.nextRedo, transactionID)
        XCTAssertEqual(model.history?.entries.first?.undone, true)
        let restored = try await workspace.engine.task(task.locator)
        XCTAssertEqual(restored.task.title, "Original title")

        let redoFailure = await model.restore(transactionID: transactionID, redo: true, description: "Edit title", workspace: workspace)
        XCTAssertNil(redoFailure)
        let redone = try await workspace.engine.task(task.locator)
        XCTAssertEqual(redone.task.title, "Accidental edit")
        await model.load(engine: workspace.engine)
        XCTAssertEqual(model.history?.nextUndo, transactionID)
        XCTAssertEqual(model.history?.nextRedo, "")

        // A newer commit supersedes the advertised undo, without changing it to
        // an instruction to restore that newer transaction instead.
        _ = try await workspace.engine.capture(title: "Newer work")
        let beforeRefusal = workspace.generation
        let refusal = await model.restore(transactionID: transactionID, redo: false, description: "Edit title", workspace: workspace)
        XCTAssertNotNil(refusal)
        XCTAssertTrue(model.refused)
        XCTAssertEqual(model.outcome, refusal)
        XCTAssertEqual(workspace.generation, beforeRefusal)
        let unchanged = try await workspace.engine.task(task.locator)
        XCTAssertEqual(unchanged.task.title, "Accidental edit")
        await model.load(engine: workspace.engine)
        XCTAssertNotEqual(model.history?.nextUndo, transactionID)
        XCTAssertEqual(model.outcome, refusal, "refreshing history dropped the restore refusal")

        // Failure is distinct from empty history and drops stale restore offers.
        try await workspace.engine.close()
        await model.load(engine: workspace.engine)
        XCTAssertNotNil(model.failure)
        XCTAssertNil(model.history)
        XCTAssertFalse(model.loading)
        try await workspace.engine.open(dataDirectory: data, runtimeDirectory: runtime, deviceID: "history-tests")
        await model.load(engine: workspace.engine)
        XCTAssertNil(model.failure)
        XCTAssertEqual(model.history?.entries.count, 3)
    }
}
