import Foundation
import XCTest

/// The runner compiles this harness against two actual app revisions. The
/// first test saves a task and a draft; the second opens that same data after
/// installing the current app. Neither test synthesizes the expected writes.
final class SimulatorUpgradeUITests: XCTestCase {
    private var root: URL {
        get throws {
            let path = try XCTUnwrap(ProcessInfo.processInfo.environment["TT_UITEST_UPGRADE_ROOT"])
            return URL(fileURLWithPath: path)
        }
    }

    private func launch() throws -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = ["TT_UITEST_ROOT": try root.path, "TT_UITEST_DEVICE_ID": "upgrade-phone"]
        app.launch()
        XCTAssertTrue(app.tabBars.buttons["Today"].waitForExistence(timeout: 40))
        return app
    }

    private func input(_ app: XCUIApplication) -> XCUIElement {
        let view = app.textViews["capture.text"].firstMatch
        return view.exists ? view : app.textFields["capture.text"].firstMatch
    }

    func testSeedUpgrade() throws {
        let root = try root
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.path), "upgrade root must be fresh")
        let seed = try XCTUnwrap(ProcessInfo.processInfo.environment["TT_UITEST_FIXTURE"])
        try FileManager.default.createDirectory(at: root.appendingPathComponent("workspaces"), withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: URL(fileURLWithPath: seed), to: root.appendingPathComponent("workspaces/upgrade"))
        try Data("upgrade".utf8).write(to: root.appendingPathComponent("selected-workspace"))
        let app = try launch()
        app.buttons["capture.open"].tap()
        XCTAssertTrue(app.buttons["capture.save"].waitForExistence(timeout: 20))
        input(app).tap(); input(app).typeText("Upgrade keeps this saved task")
        app.buttons["capture.save"].tap()
        let gone = expectation(for: NSPredicate(format: "exists == false"), evaluatedWith: app.buttons["capture.save"])
        XCTAssertEqual(XCTWaiter.wait(for: [gone], timeout: 20), .completed)
        app.buttons["capture.open"].tap()
        XCTAssertTrue(app.buttons["capture.save"].waitForExistence(timeout: 20))
        input(app).tap(); input(app).typeText("Upgrade keeps this unfinished draft")
        // The durable draft, rather than a fixed sleep, is the stop condition.
        let deadline = Date().addingTimeInterval(20)
        var saved = false
        while Date() < deadline && !saved {
            let files = FileManager.default.enumerator(at: root.appendingPathComponent("devices"), includingPropertiesForKeys: nil)
            saved = files?.compactMap { $0 as? URL }.contains {
                ((try? String(contentsOf: $0, encoding: .utf8)) ?? "").contains("Upgrade keeps this unfinished draft")
            } ?? false
            if !saved { RunLoop.current.run(until: Date().addingTimeInterval(0.1)) }
        }
        XCTAssertTrue(saved, "previous app did not persist the draft")
        app.terminate()
        try Data("seeded".utf8).write(to: root.appendingPathComponent("upgrade-seeded"))
    }

    func testVerifyUpgrade() throws {
        let root = try root
        XCTAssertTrue(FileManager.default.fileExists(atPath: root.appendingPathComponent("upgrade-seeded").path))
        let app = try launch()
        app.tabBars.buttons["Inbox"].tap()
        XCTAssertTrue(app.staticTexts["Upgrade keeps this saved task"].firstMatch.waitForExistence(timeout: 20))
        app.buttons["capture.open"].tap()
        XCTAssertTrue(app.buttons["capture.save"].waitForExistence(timeout: 20))
        XCTAssertEqual(input(app).value as? String, "Upgrade keeps this unfinished draft")
        app.buttons["capture.save"].tap()
        XCTAssertTrue(app.staticTexts["Upgrade keeps this unfinished draft"].firstMatch.waitForExistence(timeout: 20))
        let text = try String(contentsOf: root.appendingPathComponent("workspaces/upgrade/projects/unfiled/tasks.md"), encoding: .utf8)
        for title in ["Upgrade keeps this saved task", "Upgrade keeps this unfinished draft"] {
            XCTAssertEqual(text.components(separatedBy: "\n").filter { $0 == "## Task: \(title)" }.count, 1)
        }
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "upgrade-retained-task-and-draft"; screenshot.lifetime = .keepAlways; add(screenshot)
        app.terminate()
    }
}
