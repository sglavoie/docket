import Foundation
import XCTest

@testable import DocketEngine

/// Simulator tests over the real gomobile-bound engine. scripts/verify_mobile_session.sh 3
/// runs them with two directories passed through TEST_RUNNER_ variables: a
/// private copy of the representative fixture to open, and an evidence
/// directory whose evidence.json Go then checks against the Markdown written.
final class BridgeSimulatorTests: XCTestCase {
    private static let deviceID = "ios-simulator-bridge-test"

    private func requiredDirectory(_ name: String) throws -> URL {
        let value = try XCTUnwrap(
            ProcessInfo.processInfo.environment[name],
            "\(name) is unset; run these tests through scripts/verify_mobile_session.sh 3"
        )
        var isDirectory: ObjCBool = false
        XCTAssertTrue(
            FileManager.default.fileExists(atPath: value, isDirectory: &isDirectory) && isDirectory.boolValue,
            "\(name)=\(value) is not a directory"
        )
        return URL(fileURLWithPath: value, isDirectory: true)
    }

    private func scratchDirectory() throws -> URL {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: url) }
        return url
    }

    func testLinkedEngineSpeaksTheModelsBridgeVersion() async {
        let engine = DocketEngine()
        let version = await engine.bridgeVersion
        XCTAssertEqual(version, supportedBridgeVersion)
    }

    func testEngineRefusalsArriveAsTypedErrors() async throws {
        let engine = DocketEngine()
        let scratch = try scratchDirectory()

        do {
            _ = try await engine.status()
            XCTFail("status before open must refuse")
        } catch let error as DocketEngineError {
            XCTAssertEqual(error, .notOpen)
        }

        do {
            try await engine.open(
                dataDirectory: scratch.appendingPathComponent("absent"),
                runtimeDirectory: scratch.appendingPathComponent("device"),
                deviceID: Self.deviceID
            )
            XCTFail("opening a directory holding no workspace must refuse")
        } catch let error as BridgeError {
            XCTAssertEqual(error.code, .notFound, "\(error)")
            XCTAssertFalse(error.retryable)
        }

        let status = try await engine.open(
            dataDirectory: scratch.appendingPathComponent("fresh"),
            runtimeDirectory: scratch.appendingPathComponent("device"),
            deviceID: Self.deviceID,
            createIfMissing: true
        )
        let first = try await engine.capture(title: "First")
        do {
            _ = try await engine.transition(
                first.task.locator, to: .done,
                expect: Expectation(projects: [first.task.projectID: status.revisions.projects[first.task.projectID] ?? ""])
            )
            XCTFail("a stale expectation must refuse")
        } catch let error as BridgeError {
            XCTAssertEqual(error.code, .staleRevision, "\(error)")
            XCTAssertTrue(error.retryable)
        }
        let unchanged = try await engine.task(first.task.locator)
        XCTAssertEqual(unchanged.task.state, .inbox)
        try await engine.close()
    }

    func testOpenFixtureCaptureCompleteReopen() async throws {
        let fixture = try requiredDirectory("DOCKET_BRIDGE_FIXTURE_DIR")
        let evidence = try requiredDirectory("DOCKET_BRIDGE_EVIDENCE_DIR")
        let runtime = evidence.appendingPathComponent("device", isDirectory: true)
        let title = "Ship the iOS bridge"

        let engine = DocketEngine()
        let opened = try await engine.open(dataDirectory: fixture, runtimeDirectory: runtime, deviceID: Self.deviceID)
        XCTAssertEqual(opened.schemas.bridge, supportedBridgeVersion)
        XCTAssertEqual(opened.deviceID, Self.deviceID)

        let populations = try await engine.projects()
        let unfiled = try XCTUnwrap(populations.active.first(where: \.systemUnfiled), "the fixture has no Unfiled project")
        XCTAssertFalse(populations.archived.isEmpty, "the representative fixture carries an archived project")

        let captured = try await engine.capture(title: title, priority: 2)
        XCTAssertEqual(captured.task.title, title)
        XCTAssertEqual(captured.task.state, .inbox)
        XCTAssertEqual(captured.task.projectID, unfiled.id)
        XCTAssertFalse(captured.committed.transactionID.isEmpty)

        let completed = try await engine.transition(
            captured.task.locator, to: .done, expect: Expectation(projects: captured.committed.projects)
        )
        XCTAssertEqual(completed.task.state, .done)
        XCTAssertNotNil(completed.task.completedAt)
        XCTAssertNotEqual(completed.committed.transactionID, captured.committed.transactionID)
        try await engine.close()
        let closedWorkspace = await engine.workspaceID
        XCTAssertNil(closedWorkspace)

        // A second actor over the same process-wide engine: a cold reopen.
        let reopenedEngine = DocketEngine()
        let reopened = try await reopenedEngine.open(dataDirectory: fixture, runtimeDirectory: runtime, deviceID: Self.deviceID)
        XCTAssertEqual(reopened.workspace, opened.workspace)
        let read = try await reopenedEngine.task(captured.task.locator)
        XCTAssertEqual(read.task.state, .done)
        XCTAssertEqual(read.task.title, title)
        XCTAssertEqual(read.revision, completed.committed.projects[unfiled.id])
        let completedQuery = try await reopenedEngine.tasks(mode: "all", query: "?completed")
        XCTAssertTrue(completedQuery.items.contains { $0.task.locator == captured.task.locator })
        try await reopenedEngine.close()

        let record: [String: Any] = [
            "bridge_version": supportedBridgeVersion,
            "data_dir": fixture.path,
            "workspace": opened.workspace,
            "device_id": Self.deviceID,
            "project_id": unfiled.id,
            "project_slug": unfiled.slug,
            "task_id": captured.task.taskID,
            "title": title,
            "capture_transaction": captured.committed.transactionID,
            "complete_transaction": completed.committed.transactionID,
            "reopened_state": read.task.state.rawValue,
            "reopened_revision": read.revision,
        ]
        let data = try JSONSerialization.data(withJSONObject: record, options: [.prettyPrinted, .sortedKeys])
        try data.write(to: evidence.appendingPathComponent("evidence.json"), options: .atomic)
    }
}
