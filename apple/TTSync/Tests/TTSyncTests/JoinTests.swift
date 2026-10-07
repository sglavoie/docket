import Foundation
import TTSync
import TTSyncContract
import XCTest

/// Joining an enrolled workspace the way the iPhone onboards: the ordinary
/// transport downloads the exchange's whole history into the inbox and the
/// first import is workspace.join. TT_ENROLLED_EXCHANGE names an exchange a
/// desktop `tt sync enroll` published, TT_BRIDGEHOST the engine; the suite
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
        let bridge = try ProcessBridge(executable: environment("TT_BRIDGEHOST"))
        bridges.append(bridge)
        return BridgeClient(call: { bridge.call($0) })
    }

    /// A private copy of the enrolled exchange, so a run never writes to the
    /// shared one.
    private func exchangeCopy() throws -> URL {
        let copy = root.appendingPathComponent("exchange", isDirectory: true)
        try FileManager.default.copyItem(at: environment("TT_ENROLLED_EXCHANGE"), to: copy)
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
