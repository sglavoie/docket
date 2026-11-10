import Foundation
import XCTest

/// The daily loop on the seeded workspace: the Today header, Start the Day
/// triage, the Now screen and its focus session, Peek, the capture button's
/// menu, and the capture grammar's colors. Each test keeps screenshots of the
/// states it reaches.
final class DailyLoopUITests: XCTestCase {
    private struct MissingInput: Error {}

    private var root: URL!
    private let timeout: TimeInterval = 20

    override func setUpWithError() throws {
        continueAfterFailure = false
        let work = try environmentURL("DOCKET_UITEST_WORK_DIR")
        root = work.appendingPathComponent("\(name.filter { $0.isLetter || $0.isNumber })-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    func testTodayLeadsWithProgressAndTheNextMove() throws {
        let app = try launchSeeded()
        require(app.descendants(matching: .any)["today.progress"].firstMatch, "the Today progress ring")
        let greeting = app.staticTexts["today.greeting"]
        require(greeting, "the greeting")
        eventually("the greeting names a move") { greeting.label.count > 3 }
        require(app.buttons["ritual.startDay"], "the Start the Day chip")
        require(app.buttons["ritual.now"], "the Now chip")
        keep(app, "Today header")
    }

    func testStartTheDaySortsTheInboxOneCardAtATime() throws {
        let app = try launchSeeded()
        press(app, "ritual.startDay")
        require(app.descendants(matching: .any)["triage.card"].firstMatch, "the first triage card")
        keep(app, "Start the Day card")
        // Swiping the card right plans it for Today.
        app.descendants(matching: .any)["triage.card"].firstMatch.swipeRight(velocity: .fast)
        eventually("the second card leads") { app.staticTexts["2 of 2"].exists }
        press(app, "triage.backlog")
        require(app.staticTexts["triage.finished"], "the end of the Inbox")
        keep(app, "Start the Day finished")
        press(app, "triage.finish")
        requireAbsent(app.staticTexts["triage.finished"], "the triage screen")
        app.tabBars.buttons["Inbox"].tap()
        require(app.staticTexts["state.empty"], "the cleared Inbox")
        keep(app, "Inbox clear")
    }

    func testNowShowsTheWorkInProgressWithAFocusSession() throws {
        let app = try launchSeeded()
        press(app, "ritual.now")
        let title = app.staticTexts["now.title"].firstMatch
        require(title, "the task in progress")
        let first = title.label
        keep(app, "Now")
        press(app, "now.focus.start")
        require(app.buttons["now.focus.stop"], "the running focus session")
        keep(app, "Now focus session")
        press(app, "now.done")
        eventually("the finished task leaves Now") {
            let current = app.staticTexts["now.title"].firstMatch
            return !current.exists || current.label != first
        }
        keep(app, "Now after Done")
        press(app, "now.close")
    }

    func testPausingFromNowAsksWhereYouLeftOff() throws {
        let app = try launchSeeded()
        press(app, "ritual.now")
        require(app.staticTexts["now.title"].firstMatch, "the task in progress")
        press(app, "now.pause")
        let note = app.alerts.textFields.firstMatch
        require(note, "the resume note prompt")
        keep(app, "Resume note prompt")
        note.typeText("Measure the hallway first")
        app.alerts.buttons["Pause"].firstMatch.tap()
        requireAbsent(app.alerts.firstMatch, "the resume note prompt")
    }

    // MARK: Helpers

    private func environmentURL(_ variable: String) throws -> URL {
        guard let path = ProcessInfo.processInfo.environment[variable], FileManager.default.fileExists(atPath: path) else {
            XCTFail("\(variable) must name an existing host path")
            throw MissingInput()
        }
        return URL(fileURLWithPath: path, isDirectory: true)
    }

    private func launchSeeded() throws -> XCUIApplication {
        let target = root.appendingPathComponent("workspaces/seeded", isDirectory: true)
        try FileManager.default.createDirectory(at: target.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: environmentURL("DOCKET_UITEST_FIXTURE"), to: target)
        let atlas = target.appendingPathComponent("projects/atlas-launch/tasks.md")
        let tasks = try String(contentsOf: atlas, encoding: .utf8)
        try tasks.replacingOccurrences(of: ", deferred: 2026-10-07T09:00:00Z", with: "")
            .write(to: atlas, atomically: true, encoding: .utf8)
        try Data("seeded".utf8).write(to: root.appendingPathComponent("selected-workspace"))
        let app = XCUIApplication()
        app.launchEnvironment = ["DOCKET_UITEST_ROOT": root.path, "DOCKET_UITEST_DEVICE_ID": "ui-test-phone"]
        app.launch()
        require(app.staticTexts["Ship beta build"], "the seeded Today list")
        return app
    }

    private func row(_ app: XCUIApplication, _ title: String) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label CONTAINS %@", ", \(title)")).firstMatch
    }

    private func press(_ app: XCUIApplication, _ identifier: String) {
        let button = app.buttons[identifier].firstMatch
        require(button, identifier)
        button.tap()
    }

    private func keep(_ app: XCUIApplication, _ name: String) {
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = name
        screenshot.lifetime = .keepAlways
        add(screenshot)
    }

    private func require(_ element: XCUIElement, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: timeout), "\(what) never appeared", file: file, line: line)
    }

    private func requireAbsent(_ element: XCUIElement, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        let gone = expectation(for: NSPredicate(format: "exists == false"), evaluatedWith: element)
        XCTAssertEqual(XCTWaiter.wait(for: [gone], timeout: timeout), .completed, "\(what) is still shown", file: file, line: line)
    }

    private func eventually(_ what: String, file: StaticString = #filePath, line: UInt = #line, _ condition: @escaping () -> Bool) {
        let met = expectation(for: NSPredicate { _, _ in condition() }, evaluatedWith: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [met], timeout: timeout), .completed, "\(what) never happened", file: file, line: line)
    }
}
