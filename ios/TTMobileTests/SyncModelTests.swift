import Foundation
import TTSync
import XCTest
@testable import tt

final class SyncModelTests: XCTestCase {
    @MainActor
    func testManualExchangeShowsProgressBeforeIOAndQueuesOneFollowup() async throws {
        let fixture = try SyncFixture()
        defer { fixture.release(); fixture.remove() }
        let model = SyncModel(transport: fixture.transport)
        model.retry()
        XCTAssertTrue(model.isExchanging, "a tap must show progress synchronously")
        model.retry()
        model.exchange()
        fixture.release()
        try await settle(model)
        XCTAssertEqual(fixture.engine.passes, 2, "requests during a pass coalesce into one followup")
        XCTAssertNotNil(model.lastAttempt)
        XCTAssertNil(model.lastError)
    }

    @MainActor
    func testDetachedPassCannotRestoreTheOldWorkspacesStatus() async throws {
        let fixture = try SyncFixture()
        defer { fixture.release(); fixture.remove() }
        let model = SyncModel(transport: fixture.transport)
        model.retry()
        model.retry()
        model.detach()
        XCTAssertFalse(model.isExchanging)
        fixture.release()
        for _ in 0..<500 {
            if (try? fixture.transport.state().lastSuccessfulExchange) != nil { break }
            try await Task.sleep(for: .milliseconds(10))
        }
        // Let the completed worker deliver its main-actor callback.
        try await Task.sleep(for: .milliseconds(100))
        XCTAssertEqual(fixture.engine.passes, 1)
        XCTAssertFalse(model.hasFolder)
        XCTAssertFalse(model.isExchanging)
        XCTAssertNil(model.condition)
        XCTAssertNil(model.lastAttempt)
    }

    @MainActor
    private func settle(_ model: SyncModel) async throws {
        for _ in 0..<500 {
            if !model.isExchanging { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        XCTFail("exchange did not finish")
    }
}

private final class SyncFixture {
    let root: URL
    let engine = CountingSyncEngine()
    let provider: GatedSyncProvider
    let transport: ExchangeTransport

    init() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let exchange = root.appendingPathComponent("exchange")
        try FileManager.default.createDirectory(at: exchange, withIntermediateDirectories: true)
        provider = GatedSyncProvider(base: CoordinatedFolderProvider(access: DirectFolderAccess(folder: exchange)))
        transport = ExchangeTransport(provider: provider, engine: engine, stateDirectory: root.appendingPathComponent("runtime"))
    }
    func release() { provider.gate.signal() }
    func remove() { try? FileManager.default.removeItem(at: root) }
}

private final class GatedSyncProvider: ExchangeProvider, @unchecked Sendable {
    let base: any ExchangeProvider
    let gate = DispatchSemaphore(value: 0)
    private let lock = NSLock()
    private var first = true
    init(base: any ExchangeProvider) { self.base = base }
    func checkAccess() throws {
        let wait = lock.withLock { let value = first; first = false; return value }
        if wait, gate.wait(timeout: .now() + 10) == .timedOut {
            throw ExchangeProviderError.transient("test gate timed out")
        }
        try base.checkAccess()
    }
    func renewAccess() throws { try base.renewAccess() }
    func list(directory: String) throws -> [ExchangeItem] { try base.list(directory: directory) }
    func read(_ path: String) throws -> Data { try base.read(path) }
    func write(_ path: String, data: Data) throws { try base.write(path, data: data) }
    func requestDownload(_ path: String) throws { try base.requestDownload(path) }
}

private final class CountingSyncEngine: SyncEngine, @unchecked Sendable {
    let workspaceID = "sync-model-tests"
    private let lock = NSLock()
    private var count = 0
    var passes: Int { lock.withLock { count } }
    func exportPending() throws -> ExportedOutbox {
        lock.withLock { count += 1 }
        return ExportedOutbox(deviceID: "phone", pending: [])
    }
    func importInbox(_ directory: URL) throws -> ImportReport {
        try JSONDecoder().decode(ImportReport.self, from: Data(#"{"results":[]}"#.utf8))
    }
    func acknowledge(_ txnIDs: [String]) throws {}
    func status() throws -> EngineSyncStatus { EngineSyncStatus(deviceID: "phone") }
}
