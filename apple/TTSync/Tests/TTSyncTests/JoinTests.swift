import Foundation
import TTSync
import TTSyncContract
import XCTest

/// Joining an enrolled workspace the way the iPhone onboards: the ordinary
/// transport downloads the exchange's whole history into the inbox and the
/// first import is workspace.join. DOCKET_ENROLLED_EXCHANGE names an exchange a
/// desktop `tt sync enroll` published, DOCKET_BRIDGEHOST the engine; the suite
/// fails rather than skips without either.
final class JoinTests: XCTestCase {
    private var root: URL!
    private var bridges: [ProcessBridge] = []

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("ttjoin-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        bridges.forEach { $0.terminate() }
        bridges = []
        try? FileManager.default.removeItem(at: root)
    }

    private func environment(_ name: String) throws -> URL {
        guard let path = ProcessInfo.processInfo.environment[name], FileManager.default.fileExists(atPath: path) else {
            throw ContractFailure("\(name) must name an existing path")
        }
        return URL(fileURLWithPath: path)
    }

    private func client() throws -> BridgeClient {
        let bridge = try ProcessBridge(executable: environment("DOCKET_BRIDGEHOST"))
        bridges.append(bridge)
        return BridgeClient(call: { bridge.call($0) })
    }

    /// A private copy of the enrolled exchange, so a run never writes to the
    /// shared one.
    private func exchangeCopy() throws -> URL {
        let copy = root.appendingPathComponent("exchange", isDirectory: true)
        try FileManager.default.copyItem(at: environment("DOCKET_ENROLLED_EXCHANGE"), to: copy)
        return copy
    }

    private func titles(_ client: BridgeClient, mode: String) throws -> [String] {
        let data = try client.send("query.tasks", payload: ["mode": mode])
        let object = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        let items = object?["items"] as? [[String: Any]] ?? []
        return items.compactMap { ($0["task"] as? [String: Any])?["title"] as? String }
    }

    func testJoinBuildsAWorkingCopyFromTheExchange() throws {
        let exchange = try exchangeCopy()
        let provider = CoordinatedFolderProvider(access: DirectFolderAccess(folder: exchange))
        let workspaceID = try JoiningSyncEngine.workspaceID(in: provider)
        let client = try client()
        let data = root.appendingPathComponent("phone/workspace", isDirectory: true)
        let runtime = root.appendingPathComponent("phone/device", isDirectory: true)
        let engine = JoiningSyncEngine(
            client: client, workspaceID: workspaceID, dataDirectory: data,
            runtimeDirectory: runtime, deviceID: "phone-join", exchange: exchange.path
        )
        let transport = ExchangeTransport(
            provider: provider, engine: engine,
            stateDirectory: runtime.appendingPathComponent("sync-transport", isDirectory: true)
        )
        let first = try transport.exchange()
        XCTAssertTrue(engine.joined, "the first pass did not join: \(first.errors)")
        XCTAssertFalse(first.imported.isEmpty)
        XCTAssertEqual(first.condition, .reachable)
        XCTAssertEqual(client.workspace, workspaceID)
        XCTAssertEqual(try titles(client, mode: "today"), ["Ship beta build", "Order bathroom tiles"])

        // Joined, the same transport is the workspace's ordinary exchange:
        // a capture publishes under this device.
        _ = try client.send("command.capture", payload: ["title": "Captured on the phone"])
        let second = try transport.exchange()
        XCTAssertEqual(second.published.count, 1, "\(second.errors)")
        XCTAssertTrue(second.imported.isEmpty)
        let published = try provider.list(directory: "\(ExchangeTransport.root)/txns/phone-join")
        XCTAssertEqual(published.count, 1)
    }

    func testJoinRefusesAFolderWithoutAnExchange() throws {
        let empty = root.appendingPathComponent("empty", isDirectory: true)
        try FileManager.default.createDirectory(at: empty, withIntermediateDirectories: true)
        let provider = CoordinatedFolderProvider(access: DirectFolderAccess(folder: empty))
        XCTAssertThrowsError(try JoiningSyncEngine.workspaceID(in: provider)) { error in
            XCTAssertEqual(error as? JoinError, .notAnExchange)
        }
    }

    func testRejoinUsesANewAuthorAndImportsThePreviousCopiesHistory() throws {
        let exchange = try exchangeCopy()
        let provider = CoordinatedFolderProvider(access: DirectFolderAccess(folder: exchange))
        let workspaceID = try JoiningSyncEngine.workspaceID(in: provider)
        func join(_ name: String, device: String) throws -> (BridgeClient, JoiningSyncEngine, ExchangeTransport) {
            let client = try client()
            let runtime = root.appendingPathComponent("\(name)/runtime")
            let engine = JoiningSyncEngine(client: client, workspaceID: workspaceID,
                dataDirectory: root.appendingPathComponent("\(name)/workspace"), runtimeDirectory: runtime,
                deviceID: device, exchange: exchange.path)
            let transport = ExchangeTransport(provider: provider, engine: engine,
                stateDirectory: runtime.appendingPathComponent("sync-transport"))
            _ = try transport.exchange()
            return (client, engine, transport)
        }
        func identity(_ name: String) throws -> String {
            try WorkingCopyIdentity.resolve(dataDirectory: root.appendingPathComponent("\(name)/workspace"),
                runtimeDirectory: root.appendingPathComponent("\(name)/runtime"),
                legacyDeviceID: "same-installation", creating: true)
        }

        let firstID = try identity("first")
        let (first, _, firstTransport) = try join("first", device: firstID)
        _ = try first.send("command.capture", payload: ["title": "Before rejoining"])
        XCTAssertEqual(try firstTransport.exchange().published.count, 1)

        // Previously the transport skipped this author's files, bypassing the
        // engine's existing protection and leaving a partially joined copy.
        let (_, reused, reusedTransport) = try join("reused", device: firstID)
        XCTAssertFalse(reused.joined)
        XCTAssertEqual(try reusedTransport.state().condition, .identityConflict)
        XCTAssertFalse(FileManager.default.fileExists(atPath: reused.dataDirectory.path))

        let secondID = try identity("second")
        XCTAssertNotEqual(firstID, secondID)
        let (second, joined, secondTransport) = try join("second", device: secondID)
        XCTAssertTrue(joined.joined)
        XCTAssertTrue(try titles(second, mode: "inbox").contains("Before rejoining"))
        _ = try second.send("command.capture", payload: ["title": "After rejoining"])
        XCTAssertEqual(try secondTransport.exchange().published.count, 1)
        XCTAssertEqual(try firstTransport.exchange().imported.count, 1)
        XCTAssertTrue(try titles(first, mode: "inbox").contains("After rejoining"))
        XCTAssertEqual(try secondTransport.state().downloads.count, 0)
    }

    func testFailedJoinLeavesNoWorkingCopy() throws {
        let exchange = try exchangeCopy()
        let provider = CoordinatedFolderProvider(access: DirectFolderAccess(folder: exchange))
        let data = root.appendingPathComponent("phone/workspace", isDirectory: true)
        let engine = JoiningSyncEngine(
            client: try client(), workspaceID: try JoiningSyncEngine.workspaceID(in: provider), dataDirectory: data,
            runtimeDirectory: root.appendingPathComponent("phone/device", isDirectory: true),
            deviceID: "phone-join", exchange: exchange.path
        )
        let emptyInbox = root.appendingPathComponent("inbox", isDirectory: true)
        try FileManager.default.createDirectory(at: emptyInbox, withIntermediateDirectories: true)
        XCTAssertThrowsError(try engine.importInbox(emptyInbox))
        XCTAssertFalse(engine.joined)
        XCTAssertFalse(FileManager.default.fileExists(atPath: data.path), "a failed join left its working copy behind")
        XCTAssertEqual(try engine.status().received, [:])
    }
}
