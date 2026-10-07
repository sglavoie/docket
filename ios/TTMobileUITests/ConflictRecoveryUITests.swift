import Foundation
import XCTest

/// Conflict resolution and sync recovery on the iPhone against the real
/// gomobile-bound engine. The conflict tests join an exchange in which two
/// Mac devices each changed one task's title and one note's line while apart,
/// so the phone holds the same two coupled conflicts the Macs do. Each test
/// reads the outcome back from the files on the host.
///
/// ios/scripts/test-ui.sh runs it and passes, through TEST_RUNNER_ variables:
///   TT_UITEST_CONFLICT_EXCHANGE  the exchange holding the two Macs' conflict
///   TT_UITEST_EXCHANGE           a plain enrolled exchange
///   TT_UITEST_WORK_DIR           a host directory each test makes its root under
/// The suite fails rather than skips when any is missing.
final class ConflictRecoveryUITests: XCTestCase {
    private struct MissingInput: Error {}

    private var root: URL!
    private let timeout: TimeInterval = 30
    /// The working copy a join names first.
    private let workspaceName = "joined"

    override func setUpWithError() throws {
        continueAfterFailure = false
        let work = try environmentURL("TT_UITEST_WORK_DIR")
        root = work.appendingPathComponent("\(name.filter { $0.isLetter || $0.isNumber })-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    // MARK: Fixtures

    private func environmentURL(_ variable: String) throws -> URL {
        guard let path = ProcessInfo.processInfo.environment[variable], FileManager.default.fileExists(atPath: path) else {
            XCTFail("\(variable) must name an existing host path")
            throw MissingInput()
        }
        return URL(fileURLWithPath: path, isDirectory: true)
    }

    private var exchange: URL { root.appendingPathComponent("exchange", isDirectory: true) }
    private var workspace: URL { root.appendingPathComponent("workspaces/\(workspaceName)", isDirectory: true) }
    private var atlasTasks: URL { workspace.appendingPathComponent("projects/atlas-launch/tasks.md") }
    private var runbook: URL {
        workspace.appendingPathComponent("projects/atlas-launch/notes/2026-09-03-090000_0a1b2c3d-release-runbook.md")
    }

    /// Joins a copy of `source` through onboarding, as a person would.
    private func join(_ source: String) throws -> XCUIApplication {
        try FileManager.default.copyItem(at: environmentURL(source), to: exchange)
        let app = XCUIApplication()
        app.launchEnvironment = [
            "TT_UITEST_ROOT": root.path,
            "TT_UITEST_DEVICE_ID": "ui-test-phone",
            "TT_UITEST_EXCHANGE_DIR": exchange.path,
        ]
        app.launch()
        require(app.buttons["onboarding.join"], "onboarding")
        app.buttons["onboarding.join"].tap()
        require(app.tabBars.buttons["Today"], "the joined workspace")
        return app
    }

    // MARK: Helpers

    private func require(_ element: XCUIElement, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: timeout), "\(what) never appeared", file: file, line: line)
    }

    private func requireAbsent(_ element: XCUIElement, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        let gone = expectation(for: NSPredicate(format: "exists == false"), evaluatedWith: element)
        XCTAssertEqual(XCTWaiter.wait(for: [gone], timeout: timeout), .completed, "\(what) is still shown", file: file, line: line)
    }

    private func eventually(_ what: String, file: StaticString = #filePath, line: UInt = #line, _ condition: () throws -> Bool) {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if (try? condition()) == true { return }
            RunLoop.current.run(until: Date().addingTimeInterval(0.25))
        }
        XCTFail("\(what) never held", file: file, line: line)
    }

    private func element(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        app.descendants(matching: .any).matching(identifier: identifier).firstMatch
    }

    /// Brings an element into reach by scrolling the screen it is on.
    private func reach(_ app: XCUIApplication, _ element: XCUIElement, _ what: String) {
        for _ in 0..<6 where !element.waitForExistence(timeout: 2) {
            app.swipeUp()
        }
        for _ in 0..<8 where !element.waitForExistence(timeout: 2) {
            app.swipeDown()
        }
        require(element, what)
        for _ in 0..<6 where !element.isHittable {
            app.swipeUp()
        }
    }

    private func press(_ app: XCUIApplication, _ identifier: String) {
        let button = app.buttons[identifier].firstMatch
        reach(app, button, identifier)
        button.tap()
    }

    private func openSync(_ app: XCUIApplication) {
        let button = app.buttons["sync.open"].firstMatch
        require(button, "the sync button")
        button.tap()
        require(app.staticTexts["sync.local"], "the sync sheet")
    }

    /// Opens the open conflict whose label is `label` from the sync sheet.
    private func openConflict(_ app: XCUIApplication, _ label: String) {
        let row = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'conflict.row.' AND label CONTAINS %@", label)).firstMatch
        reach(app, row, "the \(label) conflict")
        row.tap()
        require(element(app, "conflict.label"), "the \(label) conflict")
    }

    /// The index of the version whose text contains `marker`.
    private func version(_ app: XCUIApplication, holding marker: String) -> Int {
        // A list draws only the rows on screen, so each version is scrolled
        // to before it is read.
        for index in 0..<2 {
            let text = element(app, "conflict.version.\(index).text")
            reach(app, text, "version \(index + 1)")
            if text.label.contains(marker) { return index }
        }
        XCTFail("no version holds \(marker)")
        return -1
    }

    private func read(_ url: URL) -> String {
        (try? String(contentsOf: url, encoding: .utf8)) ?? ""
    }

    /// Manifests this phone published to the exchange.
    private func published() -> [String] {
        let directory = exchange.appendingPathComponent("tt-sync/v1/txns/ui-test-phone", isDirectory: true)
        let names = (try? FileManager.default.contentsOfDirectory(atPath: directory.path)) ?? []
        return names.filter { $0.hasSuffix(".json") }.map { read(directory.appendingPathComponent($0)) }
    }

    // MARK: Tests

    /// The conflict shows where it started, both Macs' versions with who
    /// wrote each, and the other file the same change touched; keeping one
    /// Mac's version resolves both files in one change, which the phone
    /// publishes for the Macs to converge on (the verifier applies it on the
    /// studio Mac and compares).
    func testKeepingAVersionResolvesTheWholeChangeAndPublishesIt() throws {
        let app = try join("TT_UITEST_CONFLICT_EXCHANGE")
        openSync(app)
        // The conflict rows sit below the status sections; a list draws only
        // the rows on screen.
        reach(app, app.buttons["conflict.row.1"].firstMatch, "the second conflict")
        let rows = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'conflict.row.'"))
        eventually("both conflicts listed") { rows.count == 2 }
        openConflict(app, "Tasks in Atlas Launch")

        reach(app, element(app, "conflict.base"), "the base")
        XCTAssertTrue(element(app, "conflict.base").label.contains("## Task: Ship beta build\n"), "the base is not the shared original")
        let laptop = version(app, holding: "## Task: Ship beta build (laptop)")
        let studio = version(app, holding: "## Task: Ship beta build (studio)")
        XCTAssertNotEqual(laptop, studio)
        var writers: [Int: String] = [:]
        for index in [laptop, studio] {
            let writer = element(app, "conflict.version.\(index).writer")
            reach(app, writer, "version \(index + 1)'s writer")
            let line = writer.label
            writers[index] = line
            let also = element(app, "conflict.version.\(index).also")
            reach(app, also, "version \(index + 1)'s coupled files")
            XCTAssertTrue(line.hasPrefix("Edited outside tt on "), line)
            XCTAssertFalse(line.contains("this iPhone"), "a Mac's change is attributed to the phone: \(line)")
            XCTAssertTrue(line.contains("changed 2 files"), line)
            XCTAssertTrue(also.label.contains("Note “Release runbook”"),
                          "the coupled note is not named")
        }
        XCTAssertNotEqual(writers[laptop], writers[studio], "the two Macs read alike")

        press(app, "conflict.keep.\(laptop)")
        let notice = element(app, "conflicts.notice")
        require(notice, "the resolution notice")
        XCTAssertTrue(notice.label.contains("Tasks in Atlas Launch") && notice.label.contains("Note “Release runbook”"), notice.label)
        require(element(app, "conflicts.none"), "the empty conflict list")

        XCTAssertTrue(read(atlasTasks).contains("## Task: Ship beta build (laptop)\n"))
        XCTAssertFalse(read(atlasTasks).contains("(studio)"))
        XCTAssertTrue(read(runbook).contains("3. Upload from the laptop"))
        eventually("the resolution reached the exchange") {
            published().contains { $0.contains("resolution") }
        }
    }

    /// A note takes replacement text; a resolution read before an edit made
    /// elsewhere is refused without writing, read again, and then kept beside
    /// that edit.
    func testReplacementTextAndAStaleResolutionKeepEveryEdit() throws {
        let app = try join("TT_UITEST_CONFLICT_EXCHANGE")
        openSync(app)
        openConflict(app, "Release runbook")
        press(app, "conflict.replace")
        press(app, "conflict.replacement.from.\(version(app, holding: "from the studio"))")
        let editor = app.textViews["conflict.replacement.editor"].firstMatch
        require(editor, "the replacement editor")
        eventually("the editor holds the studio's version") { ((editor.value as? String) ?? "").contains("from the studio") }
        editor.tap()
        editor.coordinate(withNormalizedOffset: CGVector(dx: 0.95, dy: 0.97)).tap()
        editor.typeText("\nAgreed on the phone.")
        app.buttons["conflict.replacement.save"].tap()
        require(element(app, "conflicts.notice"), "the resolution notice")
        eventually("the note holds the replacement") {
            let note = read(runbook)
            return note.contains("3. Upload from the studio") && note.contains("Agreed on the phone.") && !note.contains("from the laptop")
        }

        // The task conflict is still open: the replacement resolved the note alone.
        openConflict(app, "Tasks in Atlas Launch")
        let studio = version(app, holding: "## Task: Ship beta build (studio)")
        // Another writer edits the file after the conflict was read.
        let edited = read(atlasTasks).replacingOccurrences(of: "### Task: Write release notes\n", with: "### Task: Write release notes (edited elsewhere)\n")
        XCTAssertTrue(edited.contains("(edited elsewhere)"))
        try edited.write(to: atlasTasks, atomically: true, encoding: .utf8)

        press(app, "conflict.keep.\(studio)")
        // A pass that observed the edit before the tap gives the conflict a
        // new reading and the tap resolves it; otherwise the tap is refused
        // as stale and writes nothing.
        if element(app, "conflict.failure").waitForExistence(timeout: 8) {
            XCTAssertTrue(read(atlasTasks).contains("(edited elsewhere)"), "the refusal wrote over the edit")
            XCTAssertFalse(read(atlasTasks).contains("## Task: Ship beta build (studio)\n") && !read(atlasTasks).contains("(laptop)"),
                           "a refused resolution was written")
        }
        // Read again, the conflict carries the edit; keeping the version now
        // keeps both.
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline, !element(app, "conflicts.notice").exists {
            if app.buttons["conflict.keep.\(studio)"].exists, app.buttons["conflict.keep.\(studio)"].isEnabled {
                app.buttons["conflict.keep.\(studio)"].tap()
            }
            RunLoop.current.run(until: Date().addingTimeInterval(1))
        }
        require(element(app, "conflicts.notice"), "the resolution after reading again")
        require(element(app, "conflicts.none"), "the empty conflict list")
        let tasks = read(atlasTasks)
        XCTAssertTrue(tasks.contains("## Task: Ship beta build (studio)\n"), "the kept version is missing")
        XCTAssertTrue(tasks.contains("### Task: Write release notes (edited elsewhere)\n"), "the racing edit was dropped")
    }

    /// With the exchange folder gone, local work goes on: content stays
    /// readable, a capture commits, the failure is shown with a retry, and a
    /// diagnostics export records it. Once the folder is back, a retry
    /// publishes what waited.
    func testTransportFailureKeepsLocalContentRetriesAndExportsDiagnostics() throws {
        let app = try join("TT_UITEST_EXCHANGE")
        require(app.staticTexts["Ship beta build"].firstMatch, "the joined Today")
        let away = root.appendingPathComponent("exchange-away", isDirectory: true)
        try FileManager.default.moveItem(at: exchange, to: away)

        app.buttons["capture.open"].tap()
        require(app.buttons["capture.save"], "the capture sheet")
        let field = app.textViews["capture.text"].firstMatch.exists ? app.textViews["capture.text"].firstMatch : app.textFields["capture.text"].firstMatch
        require(field, "the capture field")
        field.tap()
        field.typeText("Captured while offline")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the capture sheet")
        require(element(app, "sync.banner"), "the sync failure banner")
        app.tabBars.buttons["Inbox"].tap()
        require(app.staticTexts["Captured while offline"].firstMatch, "the offline capture in Inbox")
        require(app.staticTexts["Call the bank"].firstMatch, "existing local content")
        XCTAssertTrue(read(workspace.appendingPathComponent("projects/unfiled/tasks.md")).contains("## Task: Captured while offline\n"))

        openSync(app)
        reach(app, element(app, "sync.failure"), "the failure explanation")
        press(app, "sync.diagnostics")
        require(element(app, "sync.diagnostics.file"), "the exported diagnostics")
        require(app.buttons["sync.diagnostics.share"].firstMatch, "the share button")
        let directory = root.appendingPathComponent("devices/\(workspaceName)/diagnostics", isDirectory: true)
        let files = try FileManager.default.contentsOfDirectory(atPath: directory.path).filter { $0.hasSuffix(".json") }
        XCTAssertEqual(files.count, 1)
        let report = read(directory.appendingPathComponent(files[0]))
        for want in ["tt-mobile-sync-diagnostics/v1", "tt-sync-diagnostics/v1", "needsFolderAccess", "is not reachable", "\"outbox\""] {
            XCTAssertTrue(report.contains(want), "the diagnostics lack \(want)")
        }
        XCTAssertFalse(report.contains("Captured while offline"), "diagnostics carry task content")
        XCTAssertTrue(published().isEmpty)

        try FileManager.default.moveItem(at: away, to: exchange)
        press(app, "sync.retry")
        requireAbsent(element(app, "sync.failure"), "the failure after the retry")
        eventually("the offline capture reached the exchange") { !published().isEmpty }
    }
}
