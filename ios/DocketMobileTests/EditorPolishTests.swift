import Foundation
import XCTest
import DocketEngine
import DocketSync
@testable import Docket

final class EditorPolishTests: XCTestCase {
    private func preview(_ message: String) throws -> CapturePreview {
        let data = try JSONSerialization.data(withJSONObject: [
            "project_id": "", "lines": [],
            "errors": [["line": 1, "column": 0, "message": message]]
        ])
        return try JSONDecoder().decode(CapturePreview.self, from: data)
    }

    @MainActor func testLatePreviewCannotReplaceANewerRequest() async throws {
        let model = CapturePreviewModel()
        let started = expectation(description: "first request entered engine")
        var continuation: CheckedContinuation<CapturePreview?, Never>?
        let old = Task {
            await model.update(text: "A", quick: true, delay: .zero) { _ in
                await withCheckedContinuation {
                    continuation = $0
                    started.fulfill()
                }
            }
        }
        await fulfillment(of: [started], timeout: 5)
        await model.update(text: "B", quick: true, delay: .zero) { _ in try? self.preview("B") }
        XCTAssertEqual(model.current(for: "B")?.errors.first?.message, "B")
        XCTAssertNil(model.current(for: "changed before the next request starts"))
        // Same text as the first request, but a distinct submission generation.
        await model.update(text: "A", quick: true, delay: .zero) { _ in try? self.preview("new A") }
        continuation?.resume(returning: try preview("old A"))
        await old.value
        XCTAssertEqual(model.current(for: "A")?.errors.first?.message, "new A")
        await model.update(text: "A", quick: false, delay: .zero) { _ in XCTFail("form requested a quick preview"); return nil }
        XCTAssertNil(model.current(for: "A"))
    }

    @MainActor func testCancelledPreviewCannotPublishAfterEngineReturns() async throws {
        let model = CapturePreviewModel()
        let started = expectation(description: "request entered engine")
        var continuation: CheckedContinuation<CapturePreview?, Never>?
        let request = Task {
            await model.update(text: "draft", quick: true, delay: .zero) { _ in
                await withCheckedContinuation {
                    continuation = $0
                    started.fulfill()
                }
            }
        }
        await fulfillment(of: [started], timeout: 5)
        request.cancel()
        continuation?.resume(returning: try preview("cancelled"))
        await request.value
        XCTAssertNil(model.current(for: "draft"))
    }

    @MainActor func testRefusalMapsEngineFieldsAndRuneColumnsWithoutParsingMessages() throws {
        for (name, target) in [("title", TaskField.title), ("resume note", .resume), ("context", .context),
                               ("defer", .deferUntil), ("project_id", .project)] {
            var details = RefusalDetails()
            details.field = name
            let failure = EditorFailure(BridgeError(rawCode: "validation", message: "Anything", retryable: false, details: details))
            XCTAssertEqual(failure.field, target)
        }
        let data = Data(#"{"code":"validation","message":"bad token","retryable":false,"details":{"lines":[{"line":2,"column":5,"message":"bad date"}]}}"#.utf8)
        let failure = EditorFailure(try JSONDecoder().decode(BridgeError.self, from: data))
        // Emoji is two UTF-16 units; decomposed é is two Go runes.
        let text = "First task\n😀 e\u{301} @bad next"
        let selection = try XCTUnwrap(failure.selection(in: text))
        XCTAssertEqual((text as NSString).substring(with: selection), "@bad")
        XCTAssertNil(failure.selection(in: "Only one line"))
    }

    @MainActor func testEditorKeepsStructuredRefusalAndRowsExposePriorityAndContext() async throws {
        let workspace = WorkspaceModel()
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try await workspace.engine.open(dataDirectory: root.appendingPathComponent("workspace"),
                                        runtimeDirectory: root.appendingPathComponent("runtime"),
                                        deviceID: "editor-polish", createIfMissing: true)
        addTeardownBlock {
            try await workspace.engine.close()
            try FileManager.default.removeItem(at: root)
        }
        let task = try await workspace.engine.capture(title: "Send proposal").task
        let original = TaskFields(task)
        var fields = original
        fields.title = ""
        let failure = await workspace.saveEditDraft(task.locator, EditDraft(original: original, fields: fields))
        XCTAssertEqual(failure?.field, .title)
        fields = original
        fields.priority = 2
        fields.context = "phone"
        _ = try await workspace.engine.edit(task.locator, fields.edit(from: original))
        let result = try await workspace.engine.tasks(mode: "inbox")
        let row = try XCTUnwrap(result.items.first)
        let metadata = TaskRowView.metadata(row, projectName: "Unfiled")
        XCTAssertTrue(metadata.contains("P2"))
        XCTAssertTrue(metadata.contains("Context: phone"))
        XCTAssertTrue(TaskRowView.spokenLabel(row, projectName: "Unfiled").contains("context phone"))
    }

    @MainActor func testSyncRecoveryOffersTheActionForTheActualCondition() {
        XCTAssertTrue(SyncStatusSection.needsFolderChoice(hasFolder: false, condition: nil))
        for condition in [ExchangeCondition.needsFolderAccess, .foreignExchange] {
            XCTAssertTrue(SyncStatusSection.needsFolderChoice(hasFolder: true, condition: condition))
        }
        for condition in [ExchangeCondition.reachable, .accountUnavailable, .waitingForExchange] {
            XCTAssertFalse(SyncStatusSection.needsFolderChoice(hasFolder: true, condition: condition))
        }
        XCTAssertEqual(SyncStatusSection.explanation(.needsFolderAccess), "Choose the exchange folder again")
        XCTAssertTrue(SyncStatusSection.explanation(.accountUnavailable).contains("stay on this device"))
    }
}
