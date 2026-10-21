import Foundation
import XCTest

/// Capture and daily editing on the iPhone against the real gomobile-bound
/// engine: shorthand capture with its live preview, a structured capture, the
/// structured edit, plan and status changes, the shared engine's lifecycle
/// refusals explained in place, and drafts that survive a refused save and a
/// killed app. Each test reads the seeded workspace's files on the host
/// afterwards, so what it proves is the durable outcome, not only the screen.
///
/// ios/scripts/test-ui.sh runs it with the same TEST_RUNNER_ inputs as
/// BrowsingUITests; it fails rather than skips when one is missing.
final class CaptureEditingUITests: XCTestCase {
    private struct MissingInput: Error {}

    private var root: URL!
    private let timeout: TimeInterval = 20
    private let workspaceName = "seeded"

    override func setUpWithError() throws {
        continueAfterFailure = false
        guard let path = ProcessInfo.processInfo.environment["TT_UITEST_WORK_DIR"], FileManager.default.fileExists(atPath: path) else {
            XCTFail("TT_UITEST_WORK_DIR must name an existing host path")
            throw MissingInput()
        }
        root = URL(fileURLWithPath: path, isDirectory: true)
            .appendingPathComponent("\(name.filter { $0.isLetter || $0.isNumber })-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try seed()
    }

    // MARK: Fixture

    private var workspace: URL { root.appendingPathComponent("workspaces/\(workspaceName)", isDirectory: true) }

    /// The representative fixture with its one wall-clock deferral removed,
    /// selected so the app opens it.
    private func seed() throws {
        guard let fixture = ProcessInfo.processInfo.environment["TT_UITEST_FIXTURE"], FileManager.default.fileExists(atPath: fixture) else {
            XCTFail("TT_UITEST_FIXTURE must name the representative workspace")
            throw MissingInput()
        }
        try FileManager.default.createDirectory(at: workspace.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: URL(fileURLWithPath: fixture, isDirectory: true), to: workspace)
        let atlas = workspace.appendingPathComponent("projects/atlas-launch/tasks.md")
        let tasks = try String(contentsOf: atlas, encoding: .utf8)
        try tasks.replacingOccurrences(of: ", deferred: 2026-10-07T09:00:00Z", with: "").write(to: atlas, atomically: true, encoding: .utf8)
        try Data(workspaceName.utf8).write(to: root.appendingPathComponent("selected-workspace"))
    }

    private func launch(holdEditorSave: Bool = false) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = ["TT_UITEST_ROOT": root.path, "TT_UITEST_DEVICE_ID": "ui-test-phone"]
        if holdEditorSave { app.launchEnvironment["TT_UITEST_EDITOR_SAVE_GATE"] = "1" }
        app.launch()
        require(app.tabBars.buttons["Today"], "the open workspace")
        return app
    }

    /// One project's task Markdown as the engine left it on disk.
    private func markdown(_ slug: String) throws -> String {
        try String(contentsOf: workspace.appendingPathComponent("projects/\(slug)/tasks.md"), encoding: .utf8)
    }

    private func planning() throws -> String {
        try String(contentsOf: workspace.appendingPathComponent(".tt/planning.json"), encoding: .utf8)
    }

    /// The device-local draft files, which never live inside the workspace.
    private func draftFiles() -> [String] {
        let drafts = root.appendingPathComponent("devices/\(workspaceName)/drafts", isDirectory: true)
        return ((try? FileManager.default.contentsOfDirectory(atPath: drafts.path)) ?? []).sorted()
    }

    /// The `## Task:` block for one title, through the next heading.
    private func block(_ title: String, in text: String) -> String? {
        guard let start = text.range(of: "## Task: \(title)\n") else { return nil }
        let end = text.range(of: "\n## Task:", range: start.upperBound..<text.endIndex)?.lowerBound ?? text.endIndex
        return String(text[start.lowerBound..<end])
    }

    // MARK: Helpers

    private func require(_ element: XCUIElement, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(element.waitForExistence(timeout: timeout), "\(what) never appeared", file: file, line: line)
    }

    private func requireAbsent(_ element: XCUIElement, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        let gone = expectation(for: NSPredicate(format: "exists == false"), evaluatedWith: element)
        XCTAssertEqual(XCTWaiter.wait(for: [gone], timeout: timeout), .completed, "\(what) is still shown", file: file, line: line)
    }

    /// Waits until `condition` holds, polling the host files the app writes.
    private func eventually(_ what: String, file: StaticString = #filePath, line: UInt = #line, _ condition: () throws -> Bool) {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if (try? condition()) == true { return }
            RunLoop.current.run(until: Date().addingTimeInterval(0.25))
        }
        XCTFail("\(what) never held", file: file, line: line)
    }

    /// A text input by identifier, whether SwiftUI drew it as a field or a
    /// view.
    private func input(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        let field = app.textFields[identifier].firstMatch
        if field.waitForExistence(timeout: 2) { return field }
        let view = app.textViews[identifier].firstMatch
        require(view, identifier)
        return view
    }

    private func type(_ app: XCUIApplication, _ identifier: String, _ text: String) {
        let element = input(app, identifier)
        element.tap()
        element.typeText(text)
    }

    /// Empties a text input. The caret is put after the text first, since a
    /// tap may land it anywhere, and the deletes are repeated until nothing
    /// but the placeholder reads back.
    private func clear(_ app: XCUIApplication, _ identifier: String) {
        let element = input(app, identifier)
        let placeholder = element.placeholderValue ?? ""
        for _ in 0..<3 {
            let current = (element.value as? String) ?? ""
            if current.isEmpty || current == placeholder { return }
            element.tap()
            _ = app.keyboards.firstMatch.waitForExistence(timeout: 5)
            element.coordinate(withNormalizedOffset: CGVector(dx: 0.98, dy: 0.5)).tap()
            element.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count + 2))
        }
        let left = (element.value as? String) ?? ""
        XCTAssertTrue(left.isEmpty || left == placeholder, "\(identifier) still reads \(left)")
    }

    private func value(_ app: XCUIApplication, _ identifier: String) -> String {
        (input(app, identifier).value as? String) ?? ""
    }

    private func toggle(_ app: XCUIApplication, _ identifier: String) {
        let element = app.switches[identifier].firstMatch
        if !element.waitForExistence(timeout: 3) { app.swipeUp() }
        require(element, identifier)
        // The keyboard a text field raised can cover the row; scrolling the
        // form brings it out from under it.
        for _ in 0..<3 where !element.isHittable {
            app.swipeUp()
        }
        // The switch nested in the row flips it; on a system that draws none,
        // the row's trailing edge is where the switch sits.
        let before = element.value as? String
        let inner = element.switches.firstMatch
        if inner.exists {
            inner.tap()
        } else {
            element.coordinate(withNormalizedOffset: CGVector(dx: 0.95, dy: 0.5)).tap()
        }
        if (element.value as? String) == before {
            element.coordinate(withNormalizedOffset: CGVector(dx: 0.95, dy: 0.5)).tap()
        }
        eventually("\(identifier) flipped") { (element.value as? String) != before }
    }

    private func tab(_ app: XCUIApplication, _ name: String) {
        let button = app.tabBars.buttons[name]
        require(button, "the \(name) tab")
        button.tap()
    }

    private func row(_ app: XCUIApplication, _ title: String) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label CONTAINS %@", ", \(title)")).firstMatch
    }

    private func openTask(_ app: XCUIApplication, _ title: String) {
        let link = row(app, title)
        require(link, "the \(title) row")
        link.tap()
        require(app.staticTexts["detail.title"], "the \(title) detail")
    }

    private func menu(_ app: XCUIApplication, _ identifier: String, _ option: String) {
        let button = app.buttons[identifier].firstMatch
        if !button.waitForExistence(timeout: 3) { app.swipeUp() }
        require(button, identifier)
        let item = app.buttons[option].firstMatch
        // A reload that lands while the menu opens can close it again, so an
        // unopened menu is opened once more before the option is required.
        for _ in 0..<3 where !item.exists {
            button.tap()
            if item.waitForExistence(timeout: 4) { break }
        }
        if !item.exists {
            let tree = XCTAttachment(string: app.debugDescription)
            tree.name = "hierarchy without \(option)"
            tree.lifetime = .keepAlways
            add(tree)
        }
        require(item, "the \(option) option")
        item.tap()
    }

    private func openCapture(_ app: XCUIApplication) {
        let open = app.buttons["capture.open"]
        require(open, "the capture button")
        open.tap()
        require(app.buttons["capture.save"], "the capture sheet")
    }

    private func discardCapture(_ app: XCUIApplication) {
        app.buttons["capture.actions"].tap()
        app.buttons["capture.discard"].tap()
        let discard = app.alerts.buttons["Discard Draft"]
        require(discard, "the capture discard confirmation")
        discard.tap()
    }

    // MARK: Tests

    private func insertSigil(_ app: XCUIApplication, _ symbol: String) {
        let bar = app.scrollViews["capture.sigils"]
        require(bar, "the keyboard sigil shortcuts")
        let button = app.buttons["capture.sigil.\(symbol)"]
        // A previous insertion may have left the horizontal bar scrolled.
        for _ in 0..<5 where !button.isHittable { bar.swipeRight() }
        for _ in 0..<5 where !button.isHittable { bar.swipeLeft() }
        XCTAssertTrue(button.isHittable, "the \(symbol) shortcut is unreachable")
        button.tap()
    }

    func testCaptureAutocompleteAllSigils() throws {
        let app = launch()
        openCapture(app)
        let cases: [(String, String)] = [
            ("!", "!1"), ("#rel", "#release"), ("##lap", "##laptop"),
            ("@tomo", "@tomorrow"), ("@@+2h", "@@+2h"), ("$tod", "$today"),
            ("*p", "*pin"), ("=rev", "=review"), ("%atl", "%atlas-launch"),
            (">Ship", ">1"), ("<Ship", "<1"), ("~home-renovation:1", "~home-renovation:1"),
            ("&https://example.com/atlas", "&https://example.com/atlas/release"), (";", ";comment;")
        ]
        for (partial, token) in cases {
            // Exercise replacement in one live editor. Draft discard and
            // reopening have their own tests; cycling nested presentations
            // for every token also strands XCTest's animation-idle tracking.
            clear(app, "capture.text")
            let prefix = partial.hasPrefix("%") ? "Suggested " : "Suggested %atlas-launch "
            type(app, "capture.text", prefix)
            let symbol = partial.hasPrefix("##") || partial.hasPrefix("@@")
                ? String(partial.prefix(2)) : String(partial.prefix(1))
            insertSigil(app, symbol)
            input(app, "capture.text").typeText(String(partial.dropFirst(symbol.count)))
            XCTAssertEqual(value(app, "capture.text"), prefix + partial)
            let suggestion = app.buttons["capture.suggestion.\(token)"]
            require(suggestion, "completion for \(partial)")
            suggestion.tap()
            XCTAssertEqual(value(app, "capture.text"), prefix + token + " ")
            if partial == ";" {
                input(app, "capture.text").typeText("Remember #literal")
                XCTAssertEqual(value(app, "capture.text"), prefix + ";Remember #literal; ")
            }
        }
        discardCapture(app)
        requireAbsent(app.buttons["capture.save"], "the dismissed capture")
    }

    func testCaptureAutocompleteSavesCompletedMetadata() throws {
        let app = launch()
        app.open(URL(string: "tt://capture")!)
        require(app.buttons["capture.save"], "capture opened by the quick-capture link")
        type(app, "capture.text", "Autocomplete 🥛")
        insertSigil(app, "#")
        input(app, "capture.text").typeText("rel")
        for (partial, token) in [("", "#release"), ("##lap", "##laptop"), ("$tod", "$today")] {
            if !partial.isEmpty { input(app, "capture.text").typeText(partial) }
            let suggestion = app.buttons["capture.suggestion.\(token)"]
            require(suggestion, token)
            suggestion.tap()
        }
        XCTAssertEqual(value(app, "capture.text"), "Autocomplete 🥛 #release ##laptop $today ")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the saved capture")
        eventually("completed metadata persisted") {
            guard let task = self.block("Autocomplete 🥛", in: try self.markdown("unfiled")) else { return false }
            return task.contains("tags: release") && task.contains("context: laptop") && task.contains("state: backlog")
        }
    }

    func testSwipePlanningAndLifecycleActions() throws {
        let app = launch()
        tab(app, "Inbox")
        let bankID = "0a7e5d10-2c3b-4f6a-8e9d-000000000501"

        row(app, "Call the bank").swipeRight()
        require(app.buttons["task.swipe.today"], "the Today swipe action")
        require(app.buttons["task.swipe.next"], "the Next swipe action")
        XCTAssertFalse(try planning().contains(bankID), "revealing actions changed the plan")
        app.buttons["task.swipe.today"].tap()
        requireAbsent(row(app, "Call the bank"), "the planned task in Inbox")
        tab(app, "Today")
        require(row(app, "Call the bank"), "the planned task in Today")
        row(app, "Call the bank").swipeRight()
        XCTAssertEqual(app.buttons["task.swipe.today"].label, "Remove from Today")
        app.buttons["task.swipe.next"].tap()
        requireAbsent(row(app, "Call the bank"), "the task moved out of Today")

        tab(app, "Browse")
        app.buttons["browse.mode.next"].tap()
        require(row(app, "Call the bank"), "the task in Next")
        row(app, "Call the bank").swipeRight()
        XCTAssertEqual(app.buttons["task.swipe.next"].label, "Remove from Next")
        app.buttons["task.swipe.next"].tap()
        requireAbsent(row(app, "Call the bank"), "the task removed from Next")
        XCTAssertFalse(try planning().contains(bankID))

        app.navigationBars.buttons["BackButton"].firstMatch.tap()
        for _ in 0..<4 where !app.buttons["browse.project.unfiled"].exists { app.swipeUp() }
        require(app.buttons["browse.project.unfiled"], "the Unfiled project")
        app.buttons["browse.project.unfiled"].tap()
        require(row(app, "Call the bank"), "the unplanned task in its project")
        row(app, "Call the bank").swipeRight()
        app.buttons["task.swipe.today"].tap()
        eventually("the project row shows Today") { row(app, "Call the bank").label.contains("Today") }
        row(app, "Call the bank").swipeRight()
        app.buttons["task.swipe.today"].tap()
        eventually("the project row no longer shows Today") { !row(app, "Call the bank").label.contains("Today") }
        XCTAssertFalse(try planning().contains(bankID))

        row(app, "Call the bank").swipeLeft()
        require(app.buttons["task.swipe.done"], "the Done swipe action")
        require(app.buttons["task.swipe.edit"], "the Edit swipe action")
        XCTAssertEqual(app.buttons["task.swipe.startPause"].label, "Start")
        XCTAssertTrue(try markdown("unfiled").contains("state: backlog"), "revealing actions changed the status")
        app.buttons["task.swipe.startPause"].tap()
        eventually("the task started") { row(app, "Call the bank").label.hasPrefix("doing,") }
        row(app, "Call the bank").swipeLeft()
        XCTAssertEqual(app.buttons["task.swipe.startPause"].label, "Pause")
        app.buttons["task.swipe.startPause"].tap()
        eventually("the task paused") { row(app, "Call the bank").label.hasPrefix("paused,") }
        XCTAssertTrue(try markdown("unfiled").contains("state: paused"))
        row(app, "Call the bank").swipeLeft()
        XCTAssertEqual(app.buttons["task.swipe.startPause"].label, "Start")
        app.buttons["task.swipe.done"].tap()
        requireAbsent(row(app, "Call the bank"), "the completed task in the open project list")
        XCTAssertTrue(try markdown("unfiled").contains("state: done"))

        // Lists show open tasks until the query explicitly includes completed work.
        app.searchFields.firstMatch.tap()
        app.searchFields.firstMatch.typeText("?completed\n")
        require(row(app, "Call the bank"), "the completed task in search results")
        row(app, "Call the bank").swipeLeft()
        XCTAssertFalse(app.buttons["task.swipe.today"].exists)
        XCTAssertFalse(app.buttons["task.swipe.next"].exists)
        require(app.buttons["task.swipe.reopen"], "the Reopen swipe action")
        XCTAssertFalse(app.buttons["task.swipe.done"].exists)
        XCTAssertFalse(app.buttons["task.swipe.startPause"].exists)
        app.buttons["task.swipe.reopen"].tap()
        eventually("the task reopened") { try markdown("unfiled").contains("state: inbox") }
        tab(app, "Inbox")
        require(row(app, "Call the bank"), "the reopened task in Inbox")
    }

    func testSwipeRefusalEditingAndSelection() throws {
        let app = launch()
        let before = try markdown("atlas-launch")
        row(app, "Ship beta build").swipeLeft()
        app.buttons["task.swipe.done"].tap()
        let failure = app.staticTexts["list.failure"]
        require(failure, "the swipe completion refusal")
        XCTAssertTrue(failure.label.contains("open subtasks"), failure.label)
        XCTAssertTrue(failure.label.contains("Write release notes"), failure.label)
        XCTAssertEqual(try markdown("atlas-launch"), before)

        row(app, "Ship beta build").swipeLeft()
        app.buttons["task.swipe.edit"].tap()
        require(app.buttons["edit.save"], "the edit sheet opened from a swipe")
        type(app, "form.resume", "Check signing before upload")
        app.buttons["edit.save"].tap()
        requireAbsent(app.buttons["edit.save"], "the edit sheet")
        XCTAssertFalse(failure.exists)
        eventually("the swipe edit was saved") {
            try block("Ship beta build", in: markdown("atlas-launch"))?.contains("Check signing before upload") == true
        }

        app.buttons["list.select"].tap()
        row(app, "Ship beta build").swipeLeft()
        XCTAssertFalse(app.buttons["task.swipe.done"].exists)
        XCTAssertFalse(app.buttons["task.swipe.edit"].exists)
        row(app, "Ship beta build").swipeRight()
        XCTAssertFalse(app.buttons["task.swipe.today"].exists)
        XCTAssertFalse(app.buttons["task.swipe.next"].exists)
        XCTAssertFalse(app.buttons["task.swipe.organize"].exists)
    }

    func testShorthandCaptureWithLivePreview() throws {
        let app = launch()
        openCapture(app)
        type(app, "capture.text", "Plan the offsite !1 #ops $next\nBook the venue %atlas-launch $today")
        let lines = app.descendants(matching: .any).matching(identifier: "capture.preview.line")
        eventually("the preview drew both lines") { lines.count == 2 }
        XCTAssertTrue(lines.element(boundBy: 0).label.contains("Plan: Next"), lines.element(boundBy: 0).label)
        XCTAssertTrue(lines.element(boundBy: 1).label.contains("Atlas Launch"), lines.element(boundBy: 1).label)
        XCTAssertTrue(lines.element(boundBy: 1).label.contains("Plan: Today"), lines.element(boundBy: 1).label)
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the capture sheet")
        require(row(app, "Book the venue"), "the Today capture in Today")

        eventually("both captures reached their projects") {
            try block("Plan the offsite", in: markdown("unfiled"))?.contains("priority: 1") == true &&
                block("Book the venue", in: markdown("atlas-launch")) != nil
        }
        let offsite = try XCTUnwrap(block("Plan the offsite", in: markdown("unfiled")))
        XCTAssertTrue(offsite.contains("tags: ops"), offsite)
        XCTAssertTrue(offsite.contains("state: backlog"), offsite)
        XCTAssertEqual(draftFiles(), [], "a committed capture left a draft behind")
    }

    func testQuickCaptureRoutesByItsSigilAlone() throws {
        let app = launch()
        openCapture(app)
        type(app, "capture.text", "Book the venue %atlas-launch")
        let lines = app.descendants(matching: .any).matching(identifier: "capture.preview.line")
        eventually("the preview named the sigil's project") { lines.firstMatch.label.contains("Atlas Launch") }
        XCTAssertFalse(app.descendants(matching: .any)["capture.project"].exists, "quick capture drew a project picker")
    }

    func testDraftStorageFailureKeepsTextAndRetrySurvivesRelaunch() throws {
        var app = launch()
        let drafts = root.appendingPathComponent("devices/\(workspaceName)/drafts")
        try Data("unavailable".utf8).write(to: drafts)
        openCapture(app)
        type(app, "capture.text", "Keep this unsaved thought")
        require(app.descendants(matching: .any)["draft.recovery.message"].firstMatch, "the draft storage warning")
        app.buttons["capture.close"].tap()
        requireAbsent(app.buttons["capture.save"], "the closed capture")
        require(app.descendants(matching: .any)["draft.recovery.pending"].firstMatch, "the pending draft banner")
        openCapture(app)
        XCTAssertEqual(value(app, "capture.text"), "Keep this unsaved thought")
        try FileManager.default.removeItem(at: drafts)
        app.buttons["draft.recovery.retry"].tap()
        requireAbsent(app.descendants(matching: .any)["draft.recovery.message"].firstMatch, "the recovered storage warning")
        app.terminate()
        app = launch()
        openCapture(app)
        XCTAssertEqual(value(app, "capture.text"), "Keep this unsaved thought")
        XCTAssertNil(try block("Keep this unsaved thought", in: markdown("unfiled")), "draft retry submitted the task")
    }

    func testCaptureSaveKeepsTextTypedWhileTheSubmissionIsWaiting() throws {
        let app = launch(holdEditorSave: true)
        openCapture(app)
        type(app, "capture.text", "Submitted capture")
        app.buttons["capture.save"].tap()
        require(app.descendants(matching: .any)["capture.saving"].firstMatch, "the pending save")
        XCTAssertFalse(app.buttons["capture.close"].isEnabled)
        clear(app, "capture.text")
        type(app, "capture.text", "Newer capture")
        XCTAssertNil(try block("Submitted capture", in: markdown("unfiled")))
        try Data().write(to: root.appendingPathComponent("editor-save.release"))
        require(app.staticTexts["capture.savedEarlier"], "the retained-draft confirmation")
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "capture-newer-draft-after-save"
        screenshot.lifetime = .keepAlways
        add(screenshot)
        XCTAssertEqual(value(app, "capture.text"), "Newer capture")
        XCTAssertNotNil(try block("Submitted capture", in: markdown("unfiled")))
        XCTAssertNil(try block("Newer capture", in: markdown("unfiled")))
        app.buttons["capture.close"].tap()
        openCapture(app)
        XCTAssertEqual(value(app, "capture.text"), "Newer capture")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the saved capture")
        let text = try markdown("unfiled")
        XCTAssertEqual(text.components(separatedBy: "## Task: Submitted capture\n").count - 1, 1)
        XCTAssertEqual(text.components(separatedBy: "## Task: Newer capture\n").count - 1, 1)
        XCTAssertEqual(draftFiles(), [])
    }

    func testEditSaveKeepsAndRebasesChangesTypedWhileWaiting() throws {
        let app = launch(holdEditorSave: true)
        tab(app, "Inbox")
        openTask(app, "Call the bank")
        app.buttons["detail.edit"].tap()
        clear(app, "form.title")
        type(app, "form.title", "Submitted bank title")
        app.buttons["edit.save"].tap()
        require(app.descendants(matching: .any)["edit.saving"].firstMatch, "the pending edit")
        XCTAssertFalse(app.buttons["edit.close"].isEnabled)
        clear(app, "form.title")
        type(app, "form.title", "Call the bank")
        try Data().write(to: root.appendingPathComponent("editor-save.release"))
        require(app.staticTexts["edit.savedEarlier"], "the retained-edit confirmation")
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "edit-newer-draft-after-save"
        screenshot.lifetime = .keepAlways
        add(screenshot)
        XCTAssertEqual(value(app, "form.title"), "Call the bank")
        XCTAssertNotNil(try block("Submitted bank title", in: markdown("unfiled")))
        app.buttons["edit.save"].tap()
        requireAbsent(app.buttons["edit.save"], "the saved edit")
        let text = try markdown("unfiled")
        XCTAssertNotNil(block("Call the bank", in: text))
        XCTAssertNil(block("Submitted bank title", in: text))
        XCTAssertEqual(draftFiles(), [])
    }

    func testRefusedCaptureSelectsTheTokenForImmediateCorrection() throws {
        let app = launch()
        openCapture(app)
        type(app, "capture.text", "Renew passport $ready")
        app.buttons["capture.save"].tap()
        require(app.staticTexts["capture.failure"], "the save refusal")
        XCTAssertTrue(app.staticTexts["capture.failure"].isHittable)
        // No tap or manual selection: the refused token already owns the caret.
        app.typeText("$today")
        XCTAssertEqual(value(app, "capture.text"), "Renew passport $today")
        requireAbsent(app.staticTexts["capture.failure"], "the corrected refusal")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the saved capture")
        XCTAssertNotNil(try block("Renew passport", in: markdown("unfiled")))
    }

    func testRefusedCaptureKeepsItsDraftAcrossARelaunch() throws {
        var app = launch()
        openCapture(app)
        type(app, "capture.text", "Renew passport $ready")
        require(app.descendants(matching: .any)["capture.preview.error"].firstMatch, "the preview's refusal")
        app.buttons["capture.save"].tap()
        let failure = app.staticTexts["capture.failure"].firstMatch
        require(failure, "the save's refusal")
        XCTAssertTrue(failure.label.hasPrefix("Couldn’t save. Your draft is still open. Line 1:"), failure.label)
        XCTAssertTrue(failure.isHittable, "the refusal is not visible above the keyboard")
        XCTAssertTrue(app.buttons["capture.save"].exists, "a refused save closed the sheet")
        XCTAssertEqual(value(app, "capture.text"), "Renew passport $ready")
        XCTAssertNil(try block("Renew passport", in: markdown("unfiled")), "a refused capture was written")
        XCTAssertEqual(draftFiles(), ["capture.json"])
        app.buttons["capture.failure.details"].tap()
        require(app.navigationBars["Save Error"], "the complete refusal")
        app.buttons["Done"].tap()
        XCTAssertEqual(value(app, "capture.text"), "Renew passport $ready")

        // Killed mid-draft, the app comes back to the same text.
        app.terminate()
        app = launch()
        openCapture(app)
        require(app.descendants(matching: .any)["capture.restored"].firstMatch, "the restored-draft notice")
        XCTAssertEqual(value(app, "capture.text"), "Renew passport $ready")

        // Fixing the line saves it, and the draft goes with the commit.
        clear(app, "capture.text")
        type(app, "capture.text", "Renew passport $someday")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the capture sheet")
        eventually("the fixed capture was written") {
            try block("Renew passport", in: markdown("unfiled"))?.contains("state: someday") == true
        }
        XCTAssertEqual(draftFiles(), [])

        // Closing preserves a draft; only a confirmed discard removes it.
        openCapture(app)
        type(app, "capture.text", "Never mind")
        eventually("the draft was written") { self.draftFiles() == ["capture.json"] }
        app.buttons["capture.close"].tap()
        requireAbsent(app.buttons["capture.save"], "the closed capture")
        XCTAssertEqual(draftFiles(), ["capture.json"])
        openCapture(app)
        XCTAssertEqual(value(app, "capture.text"), "Never mind")
        app.buttons["capture.actions"].tap()
        app.buttons["capture.discard"].tap()
        app.buttons["Keep Editing"].tap()
        XCTAssertEqual(value(app, "capture.text"), "Never mind")
        XCTAssertEqual(draftFiles(), ["capture.json"])
        discardCapture(app)
        requireAbsent(app.buttons["capture.save"], "the discarded capture")
        XCTAssertEqual(draftFiles(), [])
        openCapture(app)
        XCTAssertFalse(app.descendants(matching: .any)["capture.restored"].firstMatch.waitForExistence(timeout: 2))
        XCTAssertEqual(value(app, "capture.text"), "")
    }

    func testStructuredCaptureCarriesEveryField() throws {
        let app = launch()
        openCapture(app)
        app.buttons["Form"].firstMatch.tap()
        type(app, "form.title", "Call the plumber #literal")
        type(app, "form.description", "Ask about the boiler.")
        type(app, "form.resume", "left a voicemail")
        type(app, "form.tags", "home, repairs")
        type(app, "form.context", "phone")
        toggle(app, "form.pinned")
        toggle(app, "form.review")
        toggle(app, "form.due.toggle")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the capture sheet")
        // A deadline falling today plans the task for Today, as on the desktop.
        require(row(app, "Call the plumber #literal"), "the structured capture in Today")

        eventually("the structured capture was written") {
            try block("Call the plumber #literal", in: markdown("unfiled")) != nil
        }
        let written = try XCTUnwrap(block("Call the plumber #literal", in: markdown("unfiled")))
        for expected in ["tags: home;repairs", "context: phone", "pinned: true", "review: true", "due: ", "Ask about the boiler.", "left a voicemail"] {
            XCTAssertTrue(written.contains(expected), "missing \(expected) in\n\(written)")
        }
        let taskID = try XCTUnwrap(written.range(of: "id: ").map { String(written[$0.upperBound...].prefix(36)) })
        XCTAssertTrue(try planning().contains(taskID), "the Today capture is not in planning.json")
    }

    func testDailyEditSavesOnceAndKeepsARefusedDraft() throws {
        var app = launch()
        tab(app, "Inbox")
        openTask(app, "Call the bank")
        app.buttons["detail.edit"].tap()
        require(app.buttons["edit.save"], "the edit sheet")
        XCTAssertEqual(value(app, "form.title"), "Call the bank")
        clear(app, "form.title")
        type(app, "form.title", "Call the bank about the mortgage")
        type(app, "form.resume", "have the account number ready")
        type(app, "form.tags", "money")
        toggle(app, "form.review")
        app.buttons["edit.save"].tap()
        requireAbsent(app.buttons["edit.save"], "the edit sheet")
        require(app.staticTexts["Call the bank about the mortgage"].firstMatch, "the edited title")
        eventually("the edit was written") {
            try block("Call the bank about the mortgage", in: markdown("unfiled")) != nil
        }
        let edited = try XCTUnwrap(block("Call the bank about the mortgage", in: markdown("unfiled")))
        for expected in ["tags: money", "review: true", "have the account number ready"] {
            XCTAssertTrue(edited.contains(expected), "missing \(expected) in\n\(edited)")
        }

        // The engine refuses an empty title; the sheet and the draft stay.
        let before = try markdown("unfiled")
        app.buttons["detail.edit"].tap()
        require(app.buttons["edit.save"], "the edit sheet")
        clear(app, "form.title")
        type(app, "form.context", "phone")
        app.buttons["edit.save"].tap()
        let failure = app.staticTexts["edit.failure"].firstMatch
        require(failure, "the edit's refusal")
        XCTAssertTrue(failure.label.lowercased().contains("title"), failure.label)
        XCTAssertTrue(failure.isHittable, "the refusal scrolled out of view")
        XCTAssertTrue(input(app, "form.title").isHittable, "the refused title was not brought into view")
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "edit-refusal-focuses-title"
        screenshot.lifetime = .keepAlways
        add(screenshot)
        XCTAssertEqual(try markdown("unfiled"), before, "a refused edit was written")
        XCTAssertEqual(draftFiles().count, 1)

        // Killed with the refused draft open, the edit reopens on it.
        app.terminate()
        app = launch()
        tab(app, "Inbox")
        openTask(app, "Call the bank about the mortgage")
        app.buttons["detail.edit"].tap()
        require(app.descendants(matching: .any)["edit.restored"].firstMatch, "the restored-edit notice")
        XCTAssertEqual(value(app, "form.context"), "phone")
        type(app, "form.title", "Call the bank today")
        app.buttons["edit.save"].tap()
        requireAbsent(app.buttons["edit.save"], "the edit sheet")
        eventually("the restored edit was written") {
            try block("Call the bank today", in: markdown("unfiled"))?.contains("context: phone") == true
        }
        XCTAssertEqual(draftFiles(), [])
    }

    func testTimedDeadlineIsVisibleAndSurvivesAnUnrelatedEdit() throws {
        let file = workspace.appendingPathComponent("projects/unfiled/tasks.md")
        let original = try String(contentsOf: file, encoding: .utf8)
        try original.replacingOccurrences(of: "priority: 0,", with: "priority: 0, due: 2030-06-10, due_time: 15:30,")
            .write(to: file, atomically: true, encoding: .utf8)
        let app = launch()
        tab(app, "Inbox")
        let taskRow = row(app, "Call the bank")
        require(taskRow, "the task with a timed deadline")
        XCTAssertTrue(taskRow.label.contains(":30"), taskRow.label)
        openTask(app, "Call the bank")
        let due = app.descendants(matching: .any)["detail.due"].firstMatch
        for _ in 0..<3 where !due.isHittable { app.swipeUp() }
        require(due, "the full deadline")
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "timed-deadline-detail"
        screenshot.lifetime = .keepAlways
        add(screenshot)
        app.buttons["detail.edit"].tap()
        type(app, "form.resume", "Bring the account number")
        app.buttons["edit.save"].tap()
        requireAbsent(app.buttons["edit.save"], "the saved editor")
        let saved = try markdown("unfiled")
        XCTAssertTrue(saved.contains("due: 2030-06-10"), saved)
        XCTAssertTrue(saved.contains("due_time: 15:30"), saved)
    }

    func testClosingAndDiscardingAnEditedTask() throws {
        let app = launch()
        tab(app, "Inbox")
        openTask(app, "Call the bank")
        let before = try markdown("unfiled")
        app.buttons["detail.edit"].tap()
        type(app, "form.resume", "Bring the account number")
        app.buttons["edit.close"].tap()
        requireAbsent(app.buttons["edit.save"], "the closed editor")
        XCTAssertEqual(try markdown("unfiled"), before)
        XCTAssertEqual(draftFiles().count, 1)
        app.buttons["detail.edit"].tap()
        require(app.descendants(matching: .any)["edit.restored"].firstMatch, "the restored edit")
        XCTAssertEqual(value(app, "form.resume"), "Bring the account number")
        app.buttons["edit.actions"].tap()
        app.buttons["edit.discard"].tap()
        app.buttons["Keep Editing"].tap()
        XCTAssertEqual(value(app, "form.resume"), "Bring the account number")
        app.buttons["edit.actions"].tap()
        app.buttons["edit.discard"].tap()
        let discard = app.alerts.buttons["Discard Draft"]
        require(discard, "the edit discard confirmation")
        discard.tap()
        requireAbsent(app.buttons["edit.save"], "the discarded editor")
        XCTAssertEqual(draftFiles(), [])
        XCTAssertEqual(try markdown("unfiled"), before)
    }

    func testPlanStatusAndLifecycleRefusals() throws {
        let app = launch()

        // A parent cannot finish over an open subtask, and says which one.
        openTask(app, "Ship beta build")
        XCTAssertEqual(app.staticTexts["detail.plan.value"].firstMatch.label.contains("Today"), true)
        menu(app, "detail.status.menu", "Done")
        let failure = app.staticTexts["detail.failure"].firstMatch
        require(failure, "the completion refusal")
        XCTAssertTrue(failure.label.contains("open subtasks"), failure.label)
        XCTAssertTrue(failure.label.contains("Write release notes"), failure.label)
        XCTAssertTrue(try markdown("atlas-launch").contains("short_id: 1, state: doing"), "a refused completion was written")

        // Today and Next exclude each other.
        menu(app, "detail.plan", "Add to Next")
        eventually("Ship beta build moved to Next") {
            app.staticTexts["detail.plan.value"].firstMatch.label.contains("Next")
        }
        let shipID = "0a7e5d10-2c3b-4f6a-8e9d-000000000101"
        let plan = try planning()
        let next = try XCTUnwrap(plan.range(of: "\"next\""))
        XCTAssertTrue(plan[next.upperBound...].contains(shipID), "not in Next:\n\(plan)")
        XCTAssertFalse(plan[..<next.lowerBound].contains(shipID), "still in Today:\n\(plan)")

        // Completing a planned task clears its plan.
        tab(app, "Inbox")
        openTask(app, "Call the bank")
        menu(app, "detail.plan", "Add to Today")
        eventually("Call the bank joined Today") {
            app.staticTexts["detail.plan.value"].firstMatch.label.contains("Today")
        }
        let bankID = "0a7e5d10-2c3b-4f6a-8e9d-000000000501"
        XCTAssertTrue(try planning().contains(bankID))
        menu(app, "detail.status.menu", "Done")
        eventually("the completion cleared the plan") {
            app.staticTexts["detail.plan.value"].firstMatch.label.contains("None")
        }
        XCTAssertFalse(try planning().contains(bankID), "a completed task kept its plan")
        XCTAssertTrue(try block("Call the bank", in: markdown("unfiled"))?.contains("state: done") == true)

        // A finished task joins no plan and moves only by reopening.
        menu(app, "detail.plan", "Add to Next")
        require(app.staticTexts["detail.failure"].firstMatch, "the terminal plan refusal")
        XCTAssertTrue(app.staticTexts["detail.failure"].firstMatch.label.contains("terminal"), app.staticTexts["detail.failure"].firstMatch.label)
        menu(app, "detail.status.menu", "Waiting")
        eventually("the terminal move was refused by name") {
            app.staticTexts["detail.failure"].firstMatch.label.contains("Can’t move from done to waiting")
        }
        XCTAssertTrue(try block("Call the bank", in: markdown("unfiled"))?.contains("state: done") == true)
    }
}
