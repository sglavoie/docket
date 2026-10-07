import Foundation
import TTShared
import XCTest

/// Share capture, shortcuts, and widgets on the iPhone against the real
/// gomobile-bound engine. Captures are written with the share extension's
/// own inbox writer into a stand-in App Group container on the host (the UI
/// test build is unsigned, so it holds no App Group), and the widget
/// snapshot is read with the widgets' own reader, so what each test proves
/// is the durable outcome on both sides of the container: a saved capture
/// shows as pending until the engine imports it, lands exactly once however
/// it was delivered or interrupted, and widgets and links reach the task.
///
/// ios/scripts/test-ui.sh runs it with the same TEST_RUNNER_ inputs as the
/// other UI suites; it fails rather than skips when one is missing.
final class ShareCaptureUITests: XCTestCase {
    private struct MissingInput: Error {}
    private struct Terminated: Error {}

    private var root: URL!
    private let timeout: TimeInterval = 25
    private let workspaceName = "seeded"
    private let workspaceID = "806a60c1-081b-420d-a4a8-7733b6201f71"

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
    private var shared: URL { root.appendingPathComponent("app-group", isDirectory: true) }
    private var inbox: CaptureInbox { CaptureInbox(container: shared) }

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
        try FileManager.default.createDirectory(at: shared, withIntermediateDirectories: true)
    }

    private func launch(_ extra: [String: String] = [:]) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = [
            "TT_UITEST_ROOT": root.path, "TT_UITEST_DEVICE_ID": "ui-test-phone",
            "TT_UITEST_SHARED_CONTAINER": shared.path,
        ].merging(extra) { $1 }
        app.launch()
        require(app.tabBars.buttons["Today"], "the open workspace")
        return app
    }

    /// How many tasks across the workspace carry `title`.
    private func titled(_ title: String) -> Int {
        let projects = workspace.appendingPathComponent("projects")
        let slugs = (try? FileManager.default.contentsOfDirectory(atPath: projects.path)) ?? []
        return slugs.reduce(0) { count, slug in
            let text = (try? String(contentsOf: projects.appendingPathComponent("\(slug)/tasks.md"), encoding: .utf8)) ?? ""
            return count + text.split(separator: "\n").filter { $0 == "## Task: \(title)" }.count
        }
    }

    private func sourceFile(_ name: String, _ contents: String) throws -> SharedFile {
        let url = root.appendingPathComponent("shared-\(name)")
        try Data(contents.utf8).write(to: url)
        return SharedFile(filename: name, url: url)
    }

    // MARK: Helpers

    private func require(_ element: XCUIElement, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        if !element.waitForExistence(timeout: timeout) {
            let tree = XCTAttachment(string: XCUIApplication().debugDescription)
            tree.name = "hierarchy without \(what)"
            tree.lifetime = .keepAlways
            add(tree)
        }
        XCTAssertTrue(element.exists, "\(what) never appeared", file: file, line: line)
    }

    private func eventually(_ what: String, file: StaticString = #filePath, line: UInt = #line, _ condition: () throws -> Bool) {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if (try? condition()) == true { return }
            RunLoop.current.run(until: Date().addingTimeInterval(0.25))
        }
        XCTFail("\(what) never held", file: file, line: line)
    }

    private func tab(_ app: XCUIApplication, _ name: String) {
        let button = app.tabBars.buttons[name]
        require(button, "the \(name) tab")
        button.tap()
    }

    private func row(_ app: XCUIApplication, _ title: String) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label CONTAINS %@", ", \(title)")).firstMatch
    }

    private func pendingRow(_ app: XCUIApplication, _ summary: String) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "identifier == 'captures.pending.row' AND label CONTAINS %@", summary)).firstMatch
    }

    // MARK: Tests

    func testSavedCapturesStayPendingUntilImportedAndLandOnce() throws {
        let shortcut = UUID()
        try inbox.save(id: shortcut, workspace: workspaceID, source: .shortcut, content: .text("Call the venue !1 #events"))
        let draft = ShareDraft(title: "Read: release checklist", note: "From Safari")
        let items = SharedItems(urls: [URL(string: "https://example.com/release")!], files: [try sourceFile("scan.pdf", "scan bytes")])
        try draft.save(items, into: inbox, id: UUID(), workspace: workspaceID)
        let refused = UUID()
        try inbox.save(id: refused, workspace: nil, source: .shortcut, content: .text("Plan the offsite $ready"))
        // Delivered twice before any import, and one share whose extension was
        // terminated before it published.
        try inbox.save(id: shortcut, workspace: workspaceID, source: .shortcut, content: .text("Call the venue !1 #events"))
        XCTAssertThrowsError(try inbox.save(
            id: UUID(), workspace: workspaceID, source: .share, content: .literal(title: "Half saved", note: ""),
            files: [try sourceFile("half.pdf", "half")], fault: { if $0 == .filesCopied { throw Terminated() } }
        ))

        // Held: every saved capture is shown as waiting, and none is a task.
        var app = launch(["TT_UITEST_HOLD_CAPTURES": "1"])
        tab(app, "Inbox")
        require(app.staticTexts["captures.pending.header"], "the pending captures section")
        require(pendingRow(app, "Call the venue"), "the pending shortcut capture")
        require(pendingRow(app, "Read: release checklist"), "the pending shared capture")
        require(pendingRow(app, "Plan the offsite"), "the pending refused-to-be capture")
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'Waiting to be added · 1 link · 1 file'")).firstMatch.exists)
        XCTAssertEqual(app.buttons.matching(identifier: "captures.pending.row").count, 3, "the duplicate or the unpublished share was listed")
        XCTAssertEqual(titled("Call the venue"), 0, "a held capture became a task")
        app.terminate()

        // Imported: two tasks, once each; the refused one stays pending with
        // the engine's reason.
        app = launch()
        tab(app, "Inbox")
        require(row(app, "Call the venue"), "the imported shortcut task")
        require(row(app, "Read: release checklist"), "the imported shared task")
        require(app.staticTexts["captures.pending.refusal"], "the refusal beside the capture the engine refused")
        XCTAssertFalse(pendingRow(app, "Call the venue").exists)
        XCTAssertEqual(titled("Call the venue"), 1)
        XCTAssertEqual(titled("Read: release checklist"), 1)
        XCTAssertEqual(titled("Half saved"), 0)
        let unfiled = try String(contentsOf: workspace.appendingPathComponent("projects/unfiled/tasks.md"), encoding: .utf8)
        XCTAssertTrue(unfiled.contains("url|https%3A%2F%2Fexample.com%2Frelease") && unfiled.contains("file|scan.pdf|"), "the shared link and file were not attached")
        XCTAssertFalse(unfiled.contains(shared.path), "the workspace recorded the App Group path")
        app.terminate()

        // Delivered again after its import, then relaunched twice: still one.
        try inbox.save(id: shortcut, workspace: workspaceID, source: .shortcut, content: .text("Call the venue !1 #events"))
        app = launch()
        tab(app, "Inbox")
        require(row(app, "Call the venue"), "the shortcut task")
        eventually("the repeated delivery was consumed") { !self.inbox.pending().contains { $0.id == shortcut.uuidString.lowercased() } }
        app.terminate()
        app = launch()
        XCTAssertEqual(titled("Call the venue"), 1, "a repeated delivery or relaunch created a second task")

        // Corrected in the app, the refused capture imports.
        tab(app, "Inbox")
        let refusedRow = pendingRow(app, "Plan the offsite")
        require(refusedRow, "the refused capture")
        refusedRow.tap()
        let field = app.textFields["captures.revise.field"]
        require(field, "the correction field")
        field.clearAndType("Plan the offsite $next")
        app.buttons["captures.revise.save"].tap()
        eventually("the corrected capture imported") { self.titled("Plan the offsite") == 1 }
        eventually("the pending section emptied") { !app.staticTexts["captures.pending.header"].exists }
        XCTAssertTrue(inbox.pending().isEmpty)
    }

    func testTheWidgetSnapshotFollowsTheWorkspaceAndItsRowsOpenTheirTasks() throws {
        let store = WidgetSnapshotStore(container: shared)
        let app = launch()
        eventually("the app wrote the widget snapshot") { store.read() != nil }
        let first = try XCTUnwrap(store.read())
        XCTAssertEqual(first.workspace, workspaceID)
        XCTAssertLessThan(abs(first.generatedAt.timeIntervalSinceNow), 120, "the snapshot is not from this launch")
        XCTAssertGreaterThan(first.validUntil, Date())
        XCTAssertLessThanOrEqual(first.validUntil.timeIntervalSince(first.generatedAt), 25 * 3600)
        XCTAssertEqual(WidgetState(snapshot: first, pendingCaptures: 0, at: Date()).freshness, .current)
        let inboxBefore = first.inbox.count

        // A capture made in the app reaches the snapshot.
        app.buttons["capture.open"].tap()
        require(app.buttons["capture.save"], "the capture sheet")
        var text = app.textFields["capture.text"].firstMatch
        if !text.waitForExistence(timeout: 2) { text = app.textViews["capture.text"].firstMatch }
        require(text, "the capture text field")
        text.tap()
        text.typeText("Widget probe task")
        app.buttons["capture.save"].tap()
        eventually("the snapshot gained the captured task") {
            guard let snapshot = store.read() else { return false }
            return snapshot.inbox.count == inboxBefore + 1 && snapshot.generatedAt > first.generatedAt
        }

        // Each row's link opens its task, read from the snapshot the widget draws.
        let snapshot = try XCTUnwrap(store.read())
        let rows = snapshot.today.tasks.prefix(1) + snapshot.inbox.tasks.prefix(2)
        XCTAssertFalse(rows.isEmpty)
        for widgetRow in rows {
            app.open(DeepLink.task(workspace: snapshot.workspace, projectID: widgetRow.projectID, taskID: widgetRow.taskID).url)
            let title = app.staticTexts["detail.title"]
            require(title, "the linked task's title")
            eventually("the linked task is \(widgetRow.title)") { title.label == widgetRow.title }
            app.buttons["reminder.opened.done"].tap()
        }
    }

    func testLinksOpenViewsAndCaptureAndRefuseAnotherWorkspace() throws {
        let app = launch()
        app.open(DeepLink.view(.inbox).url)
        eventually("the Inbox tab is selected") { app.tabBars.buttons["Inbox"].isSelected }
        app.open(DeepLink.view(.today).url)
        eventually("the Today tab is selected") { app.tabBars.buttons["Today"].isSelected }
        app.open(DeepLink.capture.url)
        require(app.segmentedControls["capture.mode"], "the capture sheet")
        app.buttons["capture.cancel"].tap()

        app.open(DeepLink.task(workspace: "00000000-0000-4000-8000-0000000000ff", projectID: "p", taskID: "t").url)
        require(app.staticTexts["workspace.failure"], "the refusal of another workspace's link")
        XCTAssertTrue(app.staticTexts["workspace.failure"].label.contains("another workspace"))
        XCTAssertFalse(app.staticTexts["detail.title"].exists)
    }
}

private extension XCUIElement {
    func clearAndType(_ text: String) {
        tap()
        if let current = value as? String, !current.isEmpty {
            typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count))
        }
        typeText(text)
    }
}
