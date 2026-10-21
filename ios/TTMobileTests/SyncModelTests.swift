import Foundation
import TTEngine
import TTSync
import XCTest
@testable import tt

final class SyncModelTests: XCTestCase {
    @MainActor
    func testAttachingSyncPreservesTheOpenEngineAndCommittedRequestReplay() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let location = WorkspaceLocation(name: "local", data: root.appendingPathComponent("data"), runtime: root.appendingPathComponent("runtime"))
        let engine = TTEngine()
        let opened = try await engine.open(dataDirectory: location.data, runtimeDirectory: location.runtime, deviceID: "local-first-test", createIfMissing: true)
        let first = try await engine.capture(title: "Capture once", requestID: "local-before-sync")
        let model = SyncModel()
        model.attach(location: location, workspaceID: opened.workspace, exchangeOverride: root.appendingPathComponent("exchange"))
        let replay = try await engine.capture(title: "Capture once", requestID: "local-before-sync")
        XCTAssertEqual(replay.task.locator, first.task.locator, "attaching sync must not reopen the shared engine and lose replay memory")
        try await engine.close()
    }

    @MainActor
    func testCompletedExchangeDoesNotWaitForPeerNames() async throws {
        let fixture = try SyncFixture(gateNames: true)
        defer { fixture.release(); fixture.remove() }
        let model = SyncModel(transport: fixture.transport)
        model.exchange()
        for _ in 0..<500 {
            if fixture.provider.isReadingNames { break }
            try await Task.sleep(for: .milliseconds(10))
        }
        XCTAssertTrue(fixture.provider.isReadingNames)
        XCTAssertFalse(model.isExchanging, "optional cloud metadata must not delay the completed pass")
        XCTAssertNotNil(model.lastAttempt)
        XCTAssertNotNil(model.lastSuccessfulExchange)
        // Another exchange may finish without starting a second stalled read.
        model.exchange()
        try await settle(model)
        XCTAssertEqual(fixture.engine.passes, 2)
        fixture.release()
        await model.waitForExchanges()
    }

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
    func testDeletionWaitsForDetachedExchangeWriters() async throws {
        let fixture = try SyncFixture()
        defer { fixture.release(); fixture.remove() }
        let model = SyncModel(transport: fixture.transport)
        model.retry()
        model.detach()
        var finished = false
        let wait = Task {
            await model.waitForExchanges()
            finished = true
        }
        try await Task.sleep(for: .milliseconds(100))
        XCTAssertFalse(finished, "deletion must not race the detached writer")
        fixture.release()
        await wait.value
        XCTAssertTrue(finished)
        XCTAssertEqual(fixture.engine.passes, 1)
        XCTAssertNotNil(try fixture.transport.state().lastSuccessfulExchange)
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

    init(gateNames: Bool = false) throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let exchange = root.appendingPathComponent("exchange")
        try FileManager.default.createDirectory(at: exchange, withIntermediateDirectories: true)
        provider = GatedSyncProvider(base: CoordinatedFolderProvider(access: DirectFolderAccess(folder: exchange)), gateNames: gateNames)
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
    private let gateNames: Bool
    private var readingNames = false
    var isReadingNames: Bool { lock.withLock { readingNames } }
    init(base: any ExchangeProvider, gateNames: Bool = false) {
        self.base = base
        self.gateNames = gateNames
    }
    func checkAccess() throws {
        let wait = lock.withLock { let value = first && !gateNames; first = false; return value }
        if wait, gate.wait(timeout: .now() + 10) == .timedOut {
            throw ExchangeProviderError.transient("test gate timed out")
        }
        try base.checkAccess()
    }
    func renewAccess() throws { try base.renewAccess() }
    func list(directory: String) throws -> [ExchangeItem] {
        if gateNames, directory.hasSuffix("/devices") {
            lock.withLock { readingNames = true }
            if gate.wait(timeout: .now() + 10) == .timedOut {
                throw ExchangeProviderError.transient("peer names gate timed out")
            }
        }
        return try base.list(directory: directory)
    }
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
