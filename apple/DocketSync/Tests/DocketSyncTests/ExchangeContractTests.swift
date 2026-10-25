import Foundation
import DocketSync
import DocketSyncContract
import XCTest

/// The exchange contract against the fake iCloud provider and against a real
/// coordinated folder, with the real Go engine behind each device as a
/// bridgehost process. DOCKET_BRIDGEHOST names that binary; the suite fails rather
/// than skips without it.
final class ExchangeContractTests: XCTestCase {
    private var root: URL!
    private var bridges: [ProcessBridge] = []

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("ttsync-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        bridges.forEach { $0.terminate() }
        bridges = []
        try? FileManager.default.removeItem(at: root)
    }

    private static var repoRoot: URL {
        URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    }

    private func bridgehost() throws -> URL {
        guard let path = ProcessInfo.processInfo.environment["DOCKET_BRIDGEHOST"], FileManager.default.isExecutableFile(atPath: path) else {
            throw ContractFailure("DOCKET_BRIDGEHOST must name a built mobile/bridgehost binary")
        }
        return URL(fileURLWithPath: path)
    }

    private var fixture: URL {
        Self.repoRoot.appendingPathComponent("internal/compat/testdata/workspaces/representative", isDirectory: true)
    }

    /// One device: a disposable workspace (the fixture, or a received seed),
    /// its own bridgehost, and a transport over provider.
    private func party(
        _ role: String, device: String, provider: any ExchangeProvider, clock: TestClock? = nil,
        dataDirectory: URL? = nil, runtimeDirectory: URL? = nil
    ) throws -> ContractParty {
        let bridge = try ProcessBridge(executable: bridgehost())
        bridges.append(bridge)
        let data = dataDirectory ?? root.appendingPathComponent("\(device)/workspace", isDirectory: true)
        let runtime = runtimeDirectory ?? root.appendingPathComponent("\(device)/runtime", isDirectory: true)
        let now: @Sendable () -> Date
        if let clock {
            now = { clock.now }
        } else {
            now = { Date() }
        }
        return try ContractParty(
            role: role, deviceID: device, dataDirectory: data, runtimeDirectory: runtime,
            call: { bridge.call($0) }, provider: provider, now: now
        )
    }

    private func copyFixture(to url: URL) throws {
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: fixture, to: url)
    }

    /// Seeds the originator from the fixture and the responder from the seed
    /// the originator published, through the given providers.
    private func seededPair(
        run: String, originator: any ExchangeProvider, responder: any ExchangeProvider,
        clock: TestClock? = nil, pause: () -> Void
    ) throws -> (ContractParty, ContractParty) {
        let macData = root.appendingPathComponent("mac/workspace", isDirectory: true)
        try copyFixture(to: macData)
        let mac = try party(ExchangeContract.originator, device: "mac-\(run)", provider: originator, clock: clock, dataDirectory: macData)
        _ = try SeedTransfer.publish(macData, runID: run, workspaceID: mac.workspaceID, to: originator)
        let phoneData = root.appendingPathComponent("phone/workspace", isDirectory: true)
        var received: SeedTransfer.Index?
        for _ in 0..<50 where received == nil {
            received = try SeedTransfer.receive(runID: run, into: phoneData, from: responder)
            if received == nil { pause() }
        }
        let index = try XCTUnwrap(received, "the seed never arrived")
        XCTAssertEqual(index.workspaceID, mac.workspaceID)
        let phone = try party(ExchangeContract.responder, device: "phone-\(run)", provider: responder, clock: clock, dataDirectory: phoneData)
        XCTAssertEqual(phone.workspaceID, mac.workspaceID)
        return (mac, phone)
    }

    /// Asserts that no manifest was written before every blob it names.
    private func assertManifestsFollowTheirBlobs(_ cloud: FakeCloud, file: StaticString = #filePath, line: UInt = #line) throws {
        var written: Set<String> = []
        var manifests = 0
        for write in cloud.writes {
            if write.path.contains("/tt-sync/v1/txns/") {
                manifests += 1
                let data = try XCTUnwrap(cloud.contents(write.path))
                let object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
                let prefix = write.path.components(separatedBy: "/tt-sync/v1/")[0] + "/tt-sync/v1/"
                for effect in object["effects"] as? [[String: Any]] ?? [] {
                    for key in ["base_blob", "blob"] {
                        guard let digest = effect[key] as? String else { continue }
                        XCTAssertTrue(written.contains(prefix + ExchangeTransport.blobPath(digest)),
                                      "\(write.path) was written before its blob \(digest)", file: file, line: line)
                    }
                }
            }
            written.insert(write.path)
        }
        XCTAssertGreaterThan(manifests, 0, file: file, line: line)
    }

    // MARK: Fake provider

    func testLocalReadsAndWritesFinishWhileExchangeIsStalled() throws {
        let cloud = FakeCloud()
        let provider = StalledProvider(base: cloud.provider(for: "local-first"))
        let data = root.appendingPathComponent("local-first/workspace")
        try copyFixture(to: data)
        let device = try party("local", device: "local-first", provider: provider, dataDirectory: data)
        let exchangeFinished = expectation(description: "exchange resumes")
        DispatchQueue.global(qos: .utility).async {
            defer { exchangeFinished.fulfill() }
            do { _ = try device.transport.exchange() }
            catch { XCTFail("exchange failed: \(error)") }
        }
        defer { provider.release.signal(); wait(for: [exchangeFinished], timeout: 30) }
        XCTAssertEqual(provider.entered.wait(timeout: .now() + 5), .success)
        let localFinished = expectation(description: "local read and durable save finish before network resumes")
        DispatchQueue.global(qos: .userInitiated).async {
            defer { localFinished.fulfill() }
            do {
                XCTAssertFalse(try device.titles().isEmpty)
                try device.capture("Saved during a stalled exchange")
                XCTAssertTrue(try device.titles().contains("Saved during a stalled exchange"))
                XCTAssertFalse(try device.transport.engine.status().outbox.isEmpty)
            } catch { XCTFail("local work failed: \(error)") }
        }
        wait(for: [localFinished], timeout: 5)
    }

    func testContractRoundTripOverFakeICloud() throws {
        let cloud = FakeCloud()
        cloud.uploadLag = 1
        cloud.downloadLag = 2
        cloud.truncateFirstBlobRead = true
        let run = "fake-roundtrip"
        let originator = ScopedProvider(base: cloud.provider(for: "mac"), prefix: "runs/\(run)")
        let responder = ScopedProvider(base: cloud.provider(for: "phone"), prefix: "runs/\(run)")
        let pause = { cloud.advance() }
        let (mac, phone) = try seededPair(run: run, originator: originator, responder: responder, pause: pause)

        var macReport = try ExchangeContract.originate(mac, run: run, timeout: 120, pause: pause)
        XCTAssertGreaterThanOrEqual(macReport.published.count, 2, "the task and the attachment publish as two transactions")
        let phoneReport = try ExchangeContract.respond(phone, run: run, timeout: 120, pause: pause)
        macReport = try ExchangeContract.conclude(mac, run: run, report: macReport, timeout: 120, pause: pause)

        XCTAssertEqual(phoneReport.imported.sorted(), macReport.published.sorted())
        XCTAssertEqual(macReport.imported.sorted(), phoneReport.published.sorted())
        XCTAssertEqual(macReport.workspaceDigest, phoneReport.workspaceDigest, "the two devices did not converge")
        XCTAssertTrue(phoneReport.unavailableObserved.contains(ExchangeContract.attachment(run: run, role: ExchangeContract.originator)),
                      "the attachment was never reported unavailable while its content downloaded: \(phoneReport.unavailableObserved)")
        XCTAssertGreaterThan(cloud.truncatedServed, 0, "no transfer was served incomplete")
        XCTAssertGreaterThan(cloud.placeholdersServed, 0, "no placeholder was listed")
        XCTAssertGreaterThan(cloud.downloadRequests, 0, "no download was requested")
        try assertManifestsFollowTheirBlobs(cloud)
        XCTAssertNotNil(macReport.lastSuccessfulExchange)
        XCTAssertNotNil(phoneReport.lastSuccessfulExchange)
    }

    func testAccountUnavailabilityKeepsLocalWorkAndQueues() throws {
        let cloud = FakeCloud()
        let run = "account"
        let pause = { cloud.advance() }
        let (mac, _) = try seededPair(
            run: run, originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run), pause: pause
        )
        cloud.accountAvailable = false
        try mac.capture("captured while signed out")
        let before = cloud.writes.count
        let report = try mac.transport.exchange()
        XCTAssertEqual(report.condition, .accountUnavailable)
        XCTAssertEqual(cloud.writes.count, before, "nothing reaches an unavailable account")
        XCTAssertFalse(try mac.transport.engine.status().outbox.isEmpty, "the commit waits in the outbox")
        XCTAssertTrue(try mac.titles().contains("captured while signed out"), "local work stays usable")
        XCTAssertEqual(try mac.transport.state().condition, .accountUnavailable)

        cloud.accountAvailable = true
        let resumed = try mac.transport.exchange()
        XCTAssertEqual(resumed.condition, .reachable)
        XCTAssertFalse(resumed.published.isEmpty)
        XCTAssertTrue(try mac.published())
    }

    func testExpiredFolderAccessIsRenewedOrReported() throws {
        let cloud = FakeCloud()
        let run = "access"
        let (mac, _) = try seededPair(
            run: run, originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run), pause: { cloud.advance() }
        )
        try mac.capture("after the grant lapsed")
        cloud.expired.insert("mac")
        let renewed = try mac.transport.exchange()
        XCTAssertEqual(renewed.condition, .reachable)
        XCTAssertEqual(cloud.renewals, 1)
        XCTAssertFalse(renewed.published.isEmpty)

        try mac.capture("after the bookmark broke")
        cloud.expired.insert("mac")
        cloud.renewable = false
        let refused = try mac.transport.exchange()
        XCTAssertEqual(refused.condition, .needsFolderAccess)
        XCTAssertFalse(try mac.transport.engine.status().outbox.isEmpty)

        cloud.renewable = true
        XCTAssertEqual(try mac.transport.exchange().condition, .reachable)
        XCTAssertTrue(try mac.published())
    }

    func testFailedTransfersRetryWithBackoff() throws {
        let cloud = FakeCloud()
        let clock = TestClock()
        let run = "retry"
        let (mac, _) = try seededPair(
            run: run, originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run), clock: clock, pause: { cloud.advance() }
        )
        _ = try mac.transport.exchange()
        try mac.capture("retried")
        cloud.failWrites = 1
        let failed = try mac.transport.exchange()
        XCTAssertFalse(failed.errors.isEmpty)
        let entry = try XCTUnwrap(mac.transport.state().uploads.first)
        XCTAssertEqual(entry.attempts, 1)
        XCTAssertEqual(entry.nextAttempt, clock.now.addingTimeInterval(RetryPolicy.standard.delay(afterAttempts: 1)))

        let writes = cloud.writes.count
        let early = try mac.transport.exchange()
        XCTAssertEqual(cloud.writes.count, writes, "a transfer was retried before its backoff elapsed")
        XCTAssertTrue(early.published.isEmpty)

        clock.advance(RetryPolicy.standard.delay(afterAttempts: 1) + 1)
        let retried = try mac.transport.exchange()
        XCTAssertFalse(retried.published.isEmpty)
        XCTAssertTrue(try mac.published())
    }

    func testRestartResumesTheDurableUploadQueue() throws {
        let cloud = FakeCloud()
        let run = "restart"
        let macData = root.appendingPathComponent("mac/workspace", isDirectory: true)
        let macRuntime = root.appendingPathComponent("mac-\(run)/runtime", isDirectory: true)
        let provider = ScopedProvider(base: cloud.provider(for: "mac"), prefix: run)
        let (mac, _) = try seededPair(
            run: run, originator: provider,
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run), pause: { cloud.advance() }
        )
        try mac.writeWorkspaceFile("projects/atlas-launch/.attachments/restart.txt", contents: "restart bytes\n")
        try mac.capture("survives a restart")
        // The first pass writes the identity and one blob, then every write
        // fails, as if the process died mid-upload.
        let identityAndFirstBlob = cloud.writes.count + 2
        cloud.failWritesAfter = identityAndFirstBlob
        _ = try mac.transport.exchange()
        XCTAssertEqual(cloud.writes.count, identityAndFirstBlob, "the pass was not interrupted after its first blob")
        let queued = try mac.transport.state().uploads
        XCTAssertFalse(queued.isEmpty, "nothing was left to resume")

        // A new process: a fresh bridgehost and transport over the same
        // directories resume from the saved queue.
        bridges.forEach { $0.terminate() }
        cloud.failWritesAfter = nil
        let restarted = try party(ExchangeContract.originator, device: "mac-\(run)", provider: provider,
                                  dataDirectory: macData, runtimeDirectory: macRuntime)
        let resumed = try restarted.transport.state().uploads
        XCTAssertEqual(resumed.map(\.transaction.txnID), queued.map(\.transaction.txnID))
        let uploadedBefore = Set(resumed.flatMap(\.uploaded))
        let writesBefore = cloud.writes.count
        try restarted.exchange(until: { try restarted.published() }, describe: "the resumed queue", timeout: 30, pause: { cloud.advance() })
        let rewritten = cloud.writes[writesBefore...].map { $0.path.replacingOccurrences(of: "\(run)/tt-sync/v1/", with: "") }
        XCTAssertTrue(Set(rewritten).isDisjoint(with: uploadedBefore), "a verified upload was written again: \(Set(rewritten).intersection(uploadedBefore))")
        try assertManifestsFollowTheirBlobs(cloud)
        _ = mac
    }

    func testManualExchangeBypassesUploadBackoffWithoutAdvancingTime() throws {
        let cloud = FakeCloud()
        let clock = TestClock()
        let run = "manual-upload"
        let (mac, _) = try seededPair(run: run,
            originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run),
            clock: clock, pause: { cloud.advance() })
        _ = try mac.transport.exchange()
        try mac.capture("Publish immediately after retry")
        cloud.failWrites = 1
        XCTAssertFalse(try mac.transport.exchange().errors.isEmpty)
        XCTAssertTrue(try mac.transport.exchange().published.isEmpty)
        let now = clock.now
        let retried = try mac.transport.exchange(retryNow: true)
        XCTAssertEqual(clock.now, now)
        XCTAssertEqual(retried.published.count, 1)
        XCTAssertEqual(retried.outstandingUploads, 0)
    }

    func testDownloadedDescendantReportsWaitingForHistoryRatherThanDownload() throws {
        let cloud = FakeCloud()
        let run = "missing-parent"
        let (mac, phone) = try seededPair(run: run,
            originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run),
            pause: { cloud.advance() })
        try mac.capture("Parent")
        _ = try mac.transport.exchange()
        cloud.advance()
        let parent = try XCTUnwrap(cloud.paths().first { $0.contains("/txns/") })
        let parentBytes = try XCTUnwrap(cloud.contents(parent))
        try mac.capture("Descendant")
        _ = try mac.transport.exchange()
        cloud.advance()
        cloud.remove(parent)
        for path in cloud.paths() { try cloud.provider(for: "phone").requestDownload(path) }
        let blocked = try phone.transport.exchange()
        XCTAssertEqual(blocked.outstandingDownloads, 1)
        XCTAssertEqual(blocked.waitingForDownload, 0)
        XCTAssertEqual(blocked.waitingToApply, 1)
        XCTAssertNotNil(blocked.blockedReason)
        XCTAssertNil(blocked.lastSuccessfulExchange)
        XCTAssertFalse(try phone.titles().contains("Descendant"))
        try cloud.provider(for: "mac").write(parent, data: parentBytes)
        try cloud.provider(for: "phone").requestDownload(parent)
        cloud.advance()
        let recovered = try phone.transport.exchange()
        XCTAssertEqual(recovered.imported.count, 2)
        XCTAssertEqual(recovered.waitingToApply, 0)
        XCTAssertNil(recovered.blockedReason)
        XCTAssertNotNil(recovered.lastSuccessfulExchange)
    }

    func testDuplicateAuthorSequenceIsReportedEvenAfterItWasReceived() throws {
        let cloud = FakeCloud()
        let run = "forked-author"
        let (mac, phone) = try seededPair(run: run,
            originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run),
            pause: { cloud.advance() })
        try mac.capture("Original sequence")
        _ = try mac.transport.exchange()
        cloud.advance()
        for path in cloud.paths() { try cloud.provider(for: "phone").requestDownload(path) }
        XCTAssertEqual(try phone.transport.exchange().imported.count, 1)
        let path = try XCTUnwrap(cloud.paths().first { $0.contains("/txns/") })
        let name = URL(fileURLWithPath: path).lastPathComponent
        let duplicate = String(path.dropLast(name.count)) + String(name.prefix(11)) + String(repeating: "a", count: 64) + ".json"
        try cloud.provider(for: "mac").write(duplicate, data: Data("still downloading".utf8))
        cloud.advance()
        try phone.capture("Keep local work queued")
        let before = cloud.writes.count
        let refused = try phone.transport.exchange(retryNow: true)
        XCTAssertEqual(refused.condition, .identityConflict)
        XCTAssertTrue(refused.errors.first?.contains("reused sequence") == true)
        XCTAssertEqual(cloud.writes.count, before)
        XCTAssertFalse(try phone.transport.engine.status().outbox.isEmpty)
    }

    func testUploadRefusesToIntroduceASequenceCollision() throws {
        let cloud = FakeCloud()
        let run = "forked-upload"
        let (mac, _) = try seededPair(run: run,
            originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run),
            pause: { cloud.advance() })
        _ = try mac.transport.exchange()
        try mac.capture("Retain this unpublished edit")
        let transaction = try XCTUnwrap(mac.transport.engine.exportPending().pending.first)
        let directory = transaction.manifest.path.split(separator: "/").dropLast().joined(separator: "/")
        let name = String(format: "%010llu-", transaction.deviceSeq) + String(repeating: "b", count: 64) + ".json"
        let collision = "\(run)/\(ExchangeTransport.root)/\(directory)/\(name)"
        try cloud.provider(for: "old-copy").write(collision, data: Data("placeholder".utf8))
        let before = cloud.writes.count
        let refused = try mac.transport.exchange()
        XCTAssertEqual(refused.condition, .identityConflict)
        XCTAssertEqual(cloud.writes.count, before)
        XCTAssertEqual(try mac.transport.state().uploads.count, 1)
        cloud.remove(collision)
        XCTAssertEqual(try mac.transport.exchange(retryNow: true).published.count, 1)
    }

    func testMissingContentIsUnavailableAndNeverDeletes() throws {
        let cloud = FakeCloud()
        let run = "unavailable"
        let pause = { cloud.advance() }
        let (mac, phone) = try seededPair(
            run: run, originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run), pause: pause
        )
        let attachment = "projects/atlas-launch/.attachments/large.bin"
        let contents = String(repeating: "content ", count: 512)
        try mac.writeWorkspaceFile(attachment, contents: contents)
        try mac.exchange(until: { try mac.published() }, describe: "publish", timeout: 30, pause: pause)
        let blob = "\(run)/\(ExchangeTransport.root)/\(ExchangeTransport.blobPath(contentDigest(Data(contents.utf8))))"
        XCTAssertNotNil(cloud.contents(blob), "the attachment's blob was not published at \(blob)")
        let before = try SeedTransfer.workspaceDigest(phone.dataDirectory)
        // Every read of the blob comes back short, as an upload still in
        // transfer would.
        cloud.truncatedReads[blob] = 1_000
        try phone.exchange(until: { try phone.transport.state().downloads.contains { $0.unavailable.contains(attachment) } },
                           describe: "the attachment to be reported unavailable", timeout: 30, pause: pause)
        for _ in 0..<3 {
            let report = try phone.transport.exchange()
            XCTAssertTrue(report.unavailable.contains(attachment), "missing content is not reported unavailable: \(report)")
            XCTAssertGreaterThan(report.outstandingDownloads, 0)
            pause()
        }
        XCTAssertNil(phone.workspaceFile(attachment))
        XCTAssertEqual(try SeedTransfer.workspaceDigest(phone.dataDirectory), before, "an incomplete transfer changed the workspace")

        cloud.truncatedReads[blob] = 0
        try phone.exchange(until: { phone.workspaceFile(attachment) != nil }, describe: "the attachment", timeout: 30, pause: pause)
        XCTAssertTrue(try phone.transport.exchange().unavailable.isEmpty)

        // Exchange files vanishing is never a deletion.
        for path in cloud.paths() where path.contains("/tt-sync/v1/") && !path.hasSuffix("workspace.json") {
            cloud.remove(path)
        }
        let after = try SeedTransfer.workspaceDigest(phone.dataDirectory)
        _ = try phone.transport.exchange()
        XCTAssertNotNil(phone.workspaceFile(attachment))
        XCTAssertEqual(try SeedTransfer.workspaceDigest(phone.dataDirectory), after)
    }

    /// An explicit download asks the provider for exactly the content a
    /// person is waiting on, and retries now a transfer the queue had put off,
    /// rather than at its backoff.
    func testExplicitDownloadRequestsUnavailableContentNow() throws {
        let cloud = FakeCloud()
        let clock = TestClock()
        let run = "explicit-download"
        let pause = { cloud.advance() }
        let (mac, phone) = try seededPair(
            run: run, originator: ScopedProvider(base: cloud.provider(for: "mac"), prefix: run),
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run), clock: clock, pause: pause
        )
        let attachment = "projects/atlas-launch/.attachments/scan.pdf"
        let contents = String(repeating: "scan ", count: 400)
        let before = Set(cloud.paths())
        try mac.writeWorkspaceFile(attachment, contents: contents)
        try mac.exchange(until: { try mac.published() }, describe: "publish", timeout: 30, pause: pause)
        let blob = "\(run)/\(ExchangeTransport.root)/\(ExchangeTransport.blobPath(contentDigest(Data(contents.utf8))))"
        let manifest = try XCTUnwrap(cloud.paths().first { !before.contains($0) && $0.contains("/txns/") })

        // The blob keeps arriving short: the attachment is unavailable, and a
        // request for another path asks for nothing.
        cloud.truncatedReads[blob] = 1_000
        try phone.exchange(until: { try phone.transport.state().downloads.contains { $0.unavailable.contains(attachment) } },
                           describe: "the attachment to be reported unavailable", timeout: 30, pause: pause)
        let requestsBefore = cloud.downloadRequests
        XCTAssertEqual(try phone.transport.requestDownloads(paths: ["projects/other/.attachments/x"]), 0)
        XCTAssertEqual(cloud.downloadRequests, requestsBefore)
        XCTAssertEqual(try phone.transport.requestDownloads(paths: [attachment]), 1)
        XCTAssertGreaterThan(cloud.downloadRequests, requestsBefore, "the waiting content was not requested")
        cloud.truncatedReads[blob] = 0
        try phone.exchange(until: { phone.workspaceFile(attachment) == contents }, describe: "the attachment", timeout: 30, pause: pause)

        // A manifest that read short is put off by the backoff; the frozen
        // clock never lets it retry on its own, and an explicit download does.
        try mac.writeWorkspaceFile("projects/atlas-launch/.attachments/second.txt", contents: "second")
        let known = Set(cloud.paths())
        try mac.exchange(until: { try mac.published() }, describe: "publish again", timeout: 30, pause: pause)
        let second = try XCTUnwrap(cloud.paths().first { !known.contains($0) && $0.contains("/txns/") })
        XCTAssertNotEqual(second, manifest)
        cloud.truncatedReads[second] = 1
        try phone.exchange(until: { try phone.transport.state().downloads.contains { $0.nextAttempt != nil } },
                           describe: "the short manifest to back off", timeout: 30, pause: pause)
        for _ in 0..<3 {
            pause()
            XCTAssertTrue(try phone.transport.exchange().imported.isEmpty, "a backed-off transfer retried before its delay")
        }
        XCTAssertEqual(try phone.transport.requestDownloads(), 1)
        XCTAssertNil(try phone.transport.state().downloads.first?.nextAttempt)
        try phone.exchange(until: { phone.workspaceFile("projects/atlas-launch/.attachments/second.txt") == "second" },
                           describe: "the explicitly requested transfer", timeout: 30, pause: pause)
    }

    func testForeignExchangeIsRefused() throws {
        let cloud = FakeCloud()
        let run = "foreign"
        let provider = ScopedProvider(base: cloud.provider(for: "mac"), prefix: run)
        let (mac, _) = try seededPair(
            run: run, originator: provider,
            responder: ScopedProvider(base: cloud.provider(for: "phone"), prefix: run), pause: { cloud.advance() }
        )
        try provider.write("tt-sync/v1/workspace.json", data: Data(#"{"format":"tt-sync-exchange","format_version":1,"workspace_id":"someone-else"}"#.utf8))
        try mac.capture("stays local")
        let writes = cloud.writes.count
        let report = try mac.transport.exchange()
        XCTAssertEqual(report.condition, .foreignExchange)
        XCTAssertEqual(cloud.writes.count, writes)
    }

    // MARK: Coordinated folder

    func testContractRoundTripOverACoordinatedFolder() throws {
        let folder = root.appendingPathComponent("Exchange Folder", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let bookmark = root.appendingPathComponent("phone/exchange.bookmark")
        try BookmarkFolderAccess.save(folder: folder, to: bookmark)
        let run = "coordinated"
        let originator = ScopedProvider(base: CoordinatedFolderProvider(access: DirectFolderAccess(folder: folder)), prefix: "runs/\(run)")
        let responder = ScopedProvider(base: CoordinatedFolderProvider(access: BookmarkFolderAccess(bookmarkFile: bookmark)), prefix: "runs/\(run)")
        let (mac, phone) = try seededPair(run: run, originator: originator, responder: responder, pause: {})
        var macReport = try ExchangeContract.originate(mac, run: run, timeout: 60, pause: {})
        let phoneReport = try ExchangeContract.respond(phone, run: run, timeout: 60, pause: {})
        macReport = try ExchangeContract.conclude(mac, run: run, report: macReport, timeout: 60, pause: {})
        XCTAssertEqual(macReport.workspaceDigest, phoneReport.workspaceDigest)
        XCTAssertEqual(phoneReport.imported.sorted(), macReport.published.sorted())
        XCTAssertEqual(macReport.imported.sorted(), phoneReport.published.sorted())
        try ExchangeContract.publishReport(phoneReport, to: responder)
        let read = try ExchangeContract.decodeReport(originator.read("reports/responder.json"))
        XCTAssertEqual(try ExchangeContract.encode(read), try ExchangeContract.encode(phoneReport))
    }

    func testBookmarkAccessFollowsAMoveAndReportsLoss() throws {
        let bookmark = root.appendingPathComponent("exchange.bookmark")
        let missing = CoordinatedFolderProvider(access: BookmarkFolderAccess(bookmarkFile: bookmark))
        XCTAssertThrowsError(try missing.checkAccess()) { error in
            guard case ExchangeProviderError.accessExpired = error else { return XCTFail("\(error)") }
        }
        let folder = root.appendingPathComponent("picked", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try BookmarkFolderAccess.save(folder: folder, to: bookmark)
        let provider = CoordinatedFolderProvider(access: BookmarkFolderAccess(bookmarkFile: bookmark))
        try provider.write("probe.txt", data: Data("one".utf8))

        let moved = root.appendingPathComponent("moved", isDirectory: true)
        try FileManager.default.moveItem(at: folder, to: moved)
        XCTAssertThrowsError(try provider.checkAccess(), "a moved folder must not read as the old path")
        try provider.renewAccess()
        try provider.checkAccess()
        XCTAssertEqual(try provider.read("probe.txt"), Data("one".utf8), "renewal did not follow the move")

        try FileManager.default.removeItem(at: moved)
        let lost = CoordinatedFolderProvider(access: BookmarkFolderAccess(bookmarkFile: bookmark))
        XCTAssertThrowsError(try lost.checkAccess()) { error in
            guard case ExchangeProviderError.accessExpired = error else { return XCTFail("\(error)") }
        }
    }

    func testCoordinatedListingReportsNestedFilesAndNothingForAMissingFolder() throws {
        let folder = root.appendingPathComponent("listing", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let provider = CoordinatedFolderProvider(access: DirectFolderAccess(folder: folder))
        XCTAssertEqual(try provider.list(directory: "absent"), [])
        try provider.write("a/b/c.json", data: Data("{}".utf8))
        try provider.write("a/d.json", data: Data("[]".utf8))
        XCTAssertEqual(try provider.list(directory: "a").map(\.path), ["a/b/c.json", "a/d.json"])
        XCTAssertEqual(try provider.list(directory: "a").map(\.state), [.current, .current])
        XCTAssertThrowsError(try provider.read("a/missing.json")) { error in
            XCTAssertEqual(error as? ExchangeProviderError, .notFound("a/missing.json"))
        }
    }
}

/// Holds provider I/O open instead of failing immediately, as a poor connection
/// can do. Local work has to finish before the test releases the exchange.
private final class StalledProvider: ExchangeProvider, @unchecked Sendable {
    let base: any ExchangeProvider
    let entered = DispatchSemaphore(value: 0)
    let release = DispatchSemaphore(value: 0)
    private let lock = NSLock()
    private var first = true
    init(base: any ExchangeProvider) { self.base = base }
    func checkAccess() throws {
        let block = lock.withLock { let value = first; first = false; return value }
        if block {
            entered.signal()
            guard release.wait(timeout: .now() + 30) == .success else {
                throw ExchangeProviderError.transient("test provider was never released")
            }
        }
        try base.checkAccess()
    }
    func renewAccess() throws { try base.renewAccess() }
    func list(directory: String) throws -> [ExchangeItem] { try base.list(directory: directory) }
    func read(_ path: String) throws -> Data { try base.read(path) }
    func write(_ path: String, data: Data) throws { try base.write(path, data: data) }
    func requestDownload(_ path: String) throws { try base.requestDownload(path) }
}
