import Foundation
import TTEngine
import TTSync
import XCTest

/// Two independent engine processes: the real app and a peer in this runner.
/// Only delivery is controlled; saves, imports, retries and diagnostics use
/// production code. Every test owns a disposable root and retains its evidence.
final class SimulatorQualificationUITests: XCTestCase {
    var root: URL!
    var peer: BridgeClient!
    var transport: ExchangeTransport!
    let timeout: TimeInterval = 40
    var exchange: URL { root.appendingPathComponent("exchange") }
    var workspace: URL { root.appendingPathComponent("workspaces/joined") }
    var peerWorkspace: URL { root.appendingPathComponent("peer/workspace") }

    override func setUpWithError() throws {
        continueAfterFailure = false
        let env = ProcessInfo.processInfo.environment
        let work = try XCTUnwrap(env["TT_UITEST_WORK_DIR"])
        let seed = try XCTUnwrap(env["TT_UITEST_EXCHANGE"])
        root = URL(fileURLWithPath: work).appendingPathComponent("qualification-\(name.filter { $0.isLetter || $0.isNumber })-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: URL(fileURLWithPath: seed), to: exchange)
        try control()
        peer = BridgeClient(call: { GoBridgeTransport().call($0) })
        let provider = CoordinatedFolderProvider(access: DirectFolderAccess(folder: exchange))
        let runtime = root.appendingPathComponent("peer/runtime")
        let engine = JoiningSyncEngine(client: peer, workspaceID: try JoiningSyncEngine.workspaceID(in: provider),
                                      dataDirectory: peerWorkspace, runtimeDirectory: runtime,
                                      deviceID: "simulator-peer", exchange: exchange.path)
        transport = ExchangeTransport(provider: provider, engine: engine, stateDirectory: runtime.appendingPathComponent("transport"))
        _ = try transport.exchange()
        XCTAssertTrue(engine.joined)
    }

    override func tearDownWithError() throws {
        let app = XCUIApplication()
        if app.state != .notRunning {
            let screenshot = XCTAttachment(screenshot: app.screenshot())
            screenshot.name = "qualification-final-screen"
            screenshot.lifetime = .keepAlways
            add(screenshot)
            let hierarchy = XCTAttachment(string: app.debugDescription)
            hierarchy.name = "qualification-accessibility-tree"
            hierarchy.lifetime = .keepAlways
            add(hierarchy)
            app.terminate()
        }
        _ = try? peer?.send("workspace.close")
    }

    func control(_ access: String = "available", withheld: [String] = []) throws {
        let data = try JSONSerialization.data(withJSONObject: ["access": access, "withheldPrefixes": withheld], options: .sortedKeys)
        try data.write(to: root.appendingPathComponent("exchange-control.json"), options: .atomic)
    }

    func launch(_ extra: [String: String] = [:]) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = ["TT_UITEST_ROOT": root.path, "TT_UITEST_DEVICE_ID": "qualification-phone",
                                 "TT_UITEST_EXCHANGE_DIR": exchange.path, "TT_UITEST_SYNC_CONTROL": "1"]
            .merging(extra) { $1 }
        app.launch()
        if app.buttons["onboarding.join"].waitForExistence(timeout: 3) { app.buttons["onboarding.join"].tap() }
        XCTAssertTrue(app.tabBars.buttons["Today"].waitForExistence(timeout: timeout))
        return app
    }

    func eventually(_ reason: String, _ body: () throws -> Bool) throws {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if try body() { return }
            RunLoop.current.run(until: Date().addingTimeInterval(0.2))
        }
        XCTFail(reason)
        throw NSError(domain: "SimulatorQualification", code: 1, userInfo: [NSLocalizedDescriptionKey: reason])
    }

    func press(_ app: XCUIApplication, _ identifier: String) {
        let button = app.buttons[identifier].firstMatch
        for step in 0..<16 {
            if button.exists && button.isHittable { break }
            if step < 8 { app.swipeUp() } else { app.swipeDown() }
        }
        XCTAssertTrue(button.waitForExistence(timeout: timeout), identifier)
        button.tap()
    }

    func syncSheet(_ app: XCUIApplication) {
        if !app.navigationBars["Sync"].exists {
            app.buttons["sync.open"].tap()
            XCTAssertTrue(app.staticTexts["sync.local"].waitForExistence(timeout: timeout))
        }
    }

    func capture(_ app: XCUIApplication, _ title: String) throws {
        app.buttons["capture.open"].tap()
        XCTAssertTrue(app.buttons["capture.save"].waitForExistence(timeout: timeout))
        let field = app.textViews["capture.text"].firstMatch.exists ? app.textViews["capture.text"].firstMatch : app.textFields["capture.text"].firstMatch
        field.tap()
        field.typeText(title)
        app.buttons["capture.save"].tap()
        try eventually("capture did not close") { !app.buttons["capture.save"].exists }
        XCTAssertEqual(try count(title, in: workspace), 1)
    }

    func count(_ title: String, in directory: URL) throws -> Int {
        let text = try String(contentsOf: directory.appendingPathComponent("projects/unfiled/tasks.md"), encoding: .utf8)
        return text.components(separatedBy: "\n").filter { $0 == "## Task: \(title)" }.count
    }

    func diagnostics(_ app: XCUIApplication) throws -> [String: Any] {
        syncSheet(app)
        let directory = root.appendingPathComponent("devices/joined/diagnostics")
        // A prior export must never satisfy this checkpoint.
        if FileManager.default.fileExists(atPath: directory.path) {
            let archive = root.appendingPathComponent("diagnostics-\(UUID().uuidString)")
            try FileManager.default.moveItem(at: directory, to: archive)
        }
        press(app, "sync.diagnostics")
        var files: [URL] = []
        try eventually("diagnostics were not exported") {
            files = (try? FileManager.default.contentsOfDirectory(at: directory, includingPropertiesForKeys: nil)) ?? []
            return files.contains { $0.pathExtension == "json" }
        }
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: XCTUnwrap(files.first { $0.pathExtension == "json" }))) as? [String: Any])
    }

    func converge(_ app: XCUIApplication, titles: [String]) throws {
        syncSheet(app)
        // Retry clears backoff after a simulated delivery failure.
        if app.buttons["sync.retry"].exists { press(app, "sync.retry") }
        else { press(app, "Exchange Now") }
        try eventually("peer never received the phone edits") {
            _ = try self.transport.exchange()
            return try titles.allSatisfy { try self.count($0, in: self.peerWorkspace) == 1 }
        }
        var document: [String: Any] = [:]
        try eventually("devices did not converge") {
            document = try self.diagnostics(app)
            let phone = try XCTUnwrap(document["engine"] as? [String: Any])
            let other = try XCTUnwrap(JSONSerialization.jsonObject(with: self.peer.send("sync.diagnostics")) as? [String: Any])
            let digest = try XCTUnwrap(phone["workspace_digest"] as? String)
            return !digest.isEmpty && digest == other["workspace_digest"] as? String
        }
        let engine = try XCTUnwrap(document["engine"] as? [String: Any])
        let status = try XCTUnwrap(engine["status"] as? [String: Any])
        XCTAssertEqual(status["outstanding_up"] as? Int, 0)
        XCTAssertEqual((engine["conflicts"] as? [Any])?.count, 0)
        let delivery = try XCTUnwrap(document["transport"] as? [String: Any])
        XCTAssertEqual(delivery["outstanding_uploads"] as? Int, 0)
        XCTAssertEqual(delivery["outstanding_downloads"] as? Int, 0)
        let peerStatus = try transport.engine.status()
        XCTAssertTrue(peerStatus.outbox.isEmpty)
        XCTAssertTrue(peerStatus.conflicts.isEmpty)
        for title in titles { XCTAssertEqual(try count(title, in: workspace), 1) }
        let evidence = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try evidence.write(to: root.appendingPathComponent("converged.json"), options: .atomic)
    }

    func testOfflineCapturesSurviveTerminationAndConverge() throws {
        var app = launch()
        try control("offline")
        try capture(app, "Offline first")
        app.terminate()
        app = launch()
        XCTAssertEqual(try count("Offline first", in: workspace), 1)
        try capture(app, "Offline second")
        app.terminate()
        app = launch()
        _ = try peer.send("command.capture", payload: ["title": "Peer while offline"])
        _ = try transport.exchange()
        XCTAssertEqual(try count("Offline first", in: peerWorkspace), 0)
        XCTAssertEqual(try count("Peer while offline", in: workspace), 0)
        try control()
        try converge(app, titles: ["Offline first", "Offline second", "Peer while offline"])
    }

    func testAccountAndFolderAccessFailuresKeepLocalSaves() throws {
        let app = launch()
        for access in ["accountUnavailable", "accessExpired"] {
            try control(access)
            try capture(app, "Saved with \(access)")
            XCTAssertTrue(app.buttons["sync.banner.retry"].waitForExistence(timeout: timeout))
            XCTAssertTrue(app.staticTexts["Sync is paused. Everything is saved on this iPhone."].exists)
            let report = try diagnostics(app)
            let transportState = try XCTUnwrap(report["transport"] as? [String: Any])
            XCTAssertEqual(transportState["condition"] as? String, access == "accessExpired" ? "needsFolderAccess" : "accountUnavailable")
            XCTAssertEqual(try count("Saved with \(access)", in: peerWorkspace), 0)
            try control()
            try converge(app, titles: ["Saved with \(access)"])
            app.buttons["Done"].tap()
        }
    }

    func testWithheldBlobsWaitThenArriveWithoutDuplicates() throws {
        let app = launch()
        try control(withheld: ["tt-sync/v1/blobs/"])
        _ = try peer.send("command.capture", payload: ["title": "Delayed peer task"])
        _ = try transport.exchange()
        syncSheet(app)
        press(app, "Exchange Now")
        var waiting: [String: Any] = [:]
        try eventually("withheld content was not reported") {
            waiting = try self.diagnostics(app)
            let state = waiting["transport"] as? [String: Any]
            return (state?["outstanding_downloads"] as? Int ?? 0) > 0
        }
        XCTAssertEqual(try count("Delayed peer task", in: workspace), 0)
        try JSONSerialization.data(withJSONObject: waiting, options: .prettyPrinted).write(to: root.appendingPathComponent("withheld.json"))
        try control()
        try converge(app, titles: ["Delayed peer task"])
        press(app, "Exchange Now")
        try converge(app, titles: ["Delayed peer task"])
    }

    func testLargeAttachmentResumesAndReachesPeerIntact() throws {
        let source = root.appendingPathComponent("qualification-large.bin")
        let bytes = Data(repeating: 0xA7, count: 64 * 1024 * 1024)
        try bytes.write(to: source)
        var app = launch(["TT_UITEST_IMPORT_SOURCE": source.path, "TT_UITEST_HOLD_IMPORTS": "1"])
        let task = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", ", Ship beta build")).firstMatch
        XCTAssertTrue(task.waitForExistence(timeout: timeout)); task.tap()
        press(app, "attachments.add"); app.buttons["Import from Files"].tap()
        XCTAssertTrue(app.staticTexts["attachment.importing.qualification-large.bin"].waitForExistence(timeout: timeout))
        app.terminate()
        app = launch()
        let suffix = "qualification-large.bin"
        var received: URL?
        try eventually("large attachment never reached the peer") {
            _ = try self.transport.exchange()
            let files = FileManager.default.enumerator(at: self.peerWorkspace.appendingPathComponent("projects/atlas-launch/.attachments"), includingPropertiesForKeys: nil)
            received = files?.compactMap { $0 as? URL }.first { $0.lastPathComponent == suffix }
            return received != nil
        }
        XCTAssertEqual(contentDigest(try Data(contentsOf: XCTUnwrap(received))), contentDigest(bytes))
        try converge(app, titles: [])
    }
}
