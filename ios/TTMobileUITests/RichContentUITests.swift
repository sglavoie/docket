import Foundation
import XCTest

/// Rich content on the iPhone against the real gomobile-bound engine, with no
/// exchange folder, so every test works offline: task and project comments
/// written, edited, refused when stale, and kept as drafts across a killed
/// app; notes created, edited, moved, and kept as a copy when another writer
/// changed them first; attachments imported through the staging pipeline,
/// resumed after a termination, viewed, shared, and removed; and the
/// availability each attachment kind reports. Each test reads the seeded
/// workspace's files on the host, so what it proves is the durable outcome.
///
/// ios/scripts/test-ui.sh runs it with the same TEST_RUNNER_ inputs as the
/// other UI suites; it fails rather than skips when one is missing.
final class RichContentUITests: XCTestCase {
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
    private var stagingDirectory: URL { root.appendingPathComponent("devices/\(workspaceName)/attachment-imports", isDirectory: true) }

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

    private func launch(_ extra: [String: String] = [:]) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = ["TT_UITEST_ROOT": root.path, "TT_UITEST_DEVICE_ID": "ui-test-phone"].merging(extra) { $1 }
        app.launch()
        require(app.tabBars.buttons["Today"], "the open workspace")
        return app
    }

    private func markdown(_ slug: String) throws -> String {
        try String(contentsOf: workspace.appendingPathComponent("projects/\(slug)/tasks.md"), encoding: .utf8)
    }

    /// Every file under a workspace directory, by name.
    private func files(under relative: String) -> [URL] {
        let base = workspace.appendingPathComponent(relative, isDirectory: true)
        guard let walker = FileManager.default.enumerator(at: base, includingPropertiesForKeys: [.isRegularFileKey]) else { return [] }
        return walker.compactMap { $0 as? URL }.filter { !$0.hasDirectoryPath }
    }

    private func note(titled title: String, under relative: String) -> String? {
        for file in files(under: relative) where file.pathExtension == "md" {
            if let text = try? String(contentsOf: file, encoding: .utf8), text.contains("title: \(title)\n") {
                return text
            }
        }
        return nil
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

    private func openTask(_ app: XCUIApplication, _ title: String) {
        let link = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", ", \(title)")).firstMatch
        require(link, "the \(title) row")
        link.tap()
        require(app.staticTexts["detail.title"], "the \(title) detail")
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
        for _ in 0..<4 where !element.isHittable {
            app.swipeUp()
        }
    }

    private func press(_ app: XCUIApplication, _ identifier: String) {
        let button = app.buttons[identifier].firstMatch
        reach(app, button, identifier)
        button.tap()
    }

    private func choose(_ app: XCUIApplication, _ identifier: String, _ option: String) {
        press(app, identifier)
        let item = app.buttons[option].firstMatch
        if !item.waitForExistence(timeout: 4) {
            press(app, identifier)
        }
        require(item, "the \(option) option")
        item.tap()
    }

    private func input(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        let field = app.textFields[identifier].firstMatch
        if field.waitForExistence(timeout: 2) { return field }
        let view = app.textViews[identifier].firstMatch
        require(view, identifier)
        return view
    }

    /// Replaces what an input holds with `text`.
    private func replace(_ app: XCUIApplication, _ identifier: String, with text: String) {
        let element = input(app, identifier)
        element.tap()
        element.coordinate(withNormalizedOffset: CGVector(dx: 0.98, dy: 0.9)).tap()
        let current = (element.value as? String) ?? ""
        if !current.isEmpty, current != element.placeholderValue {
            element.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count + 4))
        }
        element.typeText(text)
    }

    private func value(_ app: XCUIApplication, _ identifier: String) -> String {
        (input(app, identifier).value as? String) ?? ""
    }

    private func label(_ app: XCUIApplication, _ identifier: String) -> String {
        let text = app.staticTexts[identifier].firstMatch
        reach(app, text, identifier)
        return text.label
    }

    private func answerAlert(_ app: XCUIApplication, _ text: String, _ button: String) {
        let field = app.alerts.textFields.firstMatch
        require(field, "the alert's field")
        field.tap()
        field.typeText(text)
        app.alerts.buttons[button].firstMatch.tap()
    }

    // MARK: Tests

    func testTaskCommentsEditRefuseAndKeepADraftAcrossARelaunch() throws {
        var app = launch()
        openTask(app, "Ship beta build")

        // A comment typed and abandoned by a killed app comes back.
        press(app, "comments.add")
        input(app, "comment.text").tap()
        input(app, "comment.text").typeText("Uploaded build 42 to TestFlight")
        eventually("the comment draft was kept") {
            let drafts = self.root.appendingPathComponent("devices/\(self.workspaceName)/drafts")
            return try FileManager.default.contentsOfDirectory(atPath: drafts.path).contains { $0.hasPrefix("comment-") }
        }
        app.terminate()
        app = launch()
        openTask(app, "Ship beta build")
        press(app, "comments.add")
        eventually("the draft returned") { self.value(app, "comment.text") == "Uploaded build 42 to TestFlight" }
        app.buttons["comment.save"].firstMatch.tap()
        eventually("the comment was written") { try self.markdown("atlas-launch").contains("Uploaded build 42 to TestFlight") }

        choose(app, "comment.0.actions", "Edit Comment")
        replace(app, "comment.text", with: "Branch cut from main at 4f2c")
        app.buttons["comment.save"].firstMatch.tap()
        eventually("the edit was written") {
            let text = try self.markdown("atlas-launch")
            return text.contains("Branch cut from main at 4f2c") && !text.contains("Branch cut from main.\n")
        }

        choose(app, "comment.1.actions", "Delete Comment")
        eventually("the comment was deleted") { try !self.markdown("atlas-launch").contains("Notarization failed once; retrying.") }

        // Another writer changes the first comment under the open screen: the
        // phone's edit is refused and the other text stays.
        let path = workspace.appendingPathComponent("projects/atlas-launch/tasks.md")
        let changed = try markdown("atlas-launch").replacingOccurrences(of: "Branch cut from main at 4f2c", with: "Branch cut on the desktop")
        try changed.write(to: path, atomically: true, encoding: .utf8)
        choose(app, "comment.0.actions", "Edit Comment")
        replace(app, "comment.text", with: "Phone edit over a stale read")
        app.buttons["comment.save"].firstMatch.tap()
        let failure = app.staticTexts["comment.failure"].firstMatch
        require(failure, "the stale refusal")
        XCTAssertTrue(failure.label.contains("changed since it was read"), failure.label)
        let after = try markdown("atlas-launch")
        XCTAssertTrue(after.contains("Branch cut on the desktop"))
        XCTAssertFalse(after.contains("Phone edit over a stale read"))
    }

    func testNotesCreateEditMoveAndKeepAStaleEditAsACopy() throws {
        let app = launch()
        XCTAssertEqual(app.tabBars.buttons.allElementsBoundByIndex.map { $0.label }, ["Today", "Inbox", "Notes", "Browse"])
        tab(app, "Browse")
        XCTAssertFalse(app.buttons["browse.notes"].exists)
        tab(app, "Notes")
        require(app.buttons["note.Weekly review"].firstMatch, "the global note")
        require(app.buttons["note.Release runbook"].firstMatch, "the project note")

        press(app, "notes.new")
        replace(app, "note.title", with: "Packing list")
        replace(app, "note.body", with: "passport")
        app.buttons["note.save"].firstMatch.tap()
        eventually("the note was written") { self.note(titled: "Packing list", under: ".tt/global-notes")?.contains("passport") == true }

        replace(app, "note.body", with: "passport and charger")
        app.buttons["note.save"].firstMatch.tap()
        eventually("the edit was written") { self.note(titled: "Packing list", under: ".tt/global-notes")?.contains("passport and charger") == true }

        // The desktop edits the note while the phone holds it open.
        let file = try XCTUnwrap(files(under: ".tt/global-notes").first {
            (try? String(contentsOf: $0, encoding: .utf8))?.contains("title: Packing list\n") == true
        })
        let desktop = try String(contentsOf: file, encoding: .utf8) + "\nDesktop: add the adapter.\n"
        try desktop.write(to: file, atomically: true, encoding: .utf8)
        replace(app, "note.body", with: "phone rewrite of the list")
        app.buttons["note.save"].firstMatch.tap()
        require(app.staticTexts["note.failure"].firstMatch, "the stale refusal")
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), desktop, "a stale save overwrote the other writer")
        press(app, "note.saveCopy")
        eventually("the phone's text was kept as its own note") {
            self.note(titled: "Packing list (iPhone)", under: ".tt/global-notes")?.contains("phone rewrite of the list") == true
        }
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), desktop)

        // Moving a note carries it between collections.
        let open = app.buttons["note.Weekly review"].firstMatch
        reach(app, open, "the Weekly review note")
        open.tap()
        eventually("the note opened") { self.value(app, "note.body").contains("Look at Waiting") }
        choose(app, "note.actions", "Move to Atlas Launch")
        eventually("the note moved") {
            self.note(titled: "Weekly review", under: "projects/atlas-launch/notes") != nil &&
                self.note(titled: "Weekly review", under: ".tt/global-notes") == nil
        }
    }

    func testAttachmentImportResumesAfterTerminationAndShares() throws {
        let source = root.appendingPathComponent("picked/field-notes.txt")
        try FileManager.default.createDirectory(at: source.deletingLastPathComponent(), withIntermediateDirectories: true)
        let bytes = Data((0..<4000).map { "line \($0) — café ✓\n" }.joined().utf8)
        try bytes.write(to: source)

        // The import is staged and recorded, and the process ends before the
        // commit.
        var app = launch(["TT_UITEST_IMPORT_SOURCE": source.path, "TT_UITEST_HOLD_IMPORTS": "1"])
        openTask(app, "Ship beta build")
        choose(app, "attachments.add", "Import from Files")
        let importing = app.staticTexts["attachment.importing.field-notes.txt"].firstMatch
        reach(app, importing, "the staged import")
        XCTAssertFalse(try markdown("atlas-launch").contains("field-notes.txt"))
        let pending = try String(contentsOf: stagingDirectory.appendingPathComponent("pending.json"), encoding: .utf8)
        XCTAssertTrue(pending.contains("field-notes.txt"), pending)
        app.terminate()

        // The next launch resumes it: one attachment, byte-identical, and no
        // temporary path recorded anywhere in the task Markdown.
        app = launch()
        eventually("the resumed import committed") { try self.markdown("atlas-launch").contains("file|field-notes.txt||0|") }
        let stored = try XCTUnwrap(files(under: "projects/atlas-launch/.attachments").first { $0.lastPathComponent == "field-notes.txt" })
        XCTAssertEqual(try Data(contentsOf: stored), bytes)
        let text = try markdown("atlas-launch")
        XCTAssertFalse(text.contains("attachment-imports"))
        XCTAssertFalse(text.contains("picked"))
        eventually("the staging record was forgotten") {
            let left = (try? String(contentsOf: self.stagingDirectory.appendingPathComponent("pending.json"), encoding: .utf8)) ?? "[]"
            return left == "[]"
        }

        openTask(app, "Ship beta build")
        XCTAssertTrue(label(app, "attachment.field-notes.txt.status").hasPrefix("On this iPhone"))

        // Viewing opens the file in place; sharing offers the system sheet.
        press(app, "attachment.field-notes.txt")
        require(app.otherElements["QLPreviewControllerView"].firstMatch, "the preview")
        let close = app.buttons["QLOverlayDoneButtonAccessibilityIdentifier"].firstMatch
        require(close, "the preview's close button")
        close.tap()
        press(app, "attachment.field-notes.txt.share")
        let sheet = app.otherElements["ActivityListView"].firstMatch
        let copy = app.buttons["Copy"].firstMatch
        eventually("the share sheet") { sheet.exists || copy.exists }
        if app.buttons["Close"].firstMatch.exists {
            app.buttons["Close"].firstMatch.tap()
        } else {
            app.swipeDown(velocity: .fast)
        }

        choose(app, "attachment.field-notes.txt.actions", "Remove Attachment and Its File")
        eventually("the attachment and its bytes were removed") {
            try !self.markdown("atlas-launch").contains("field-notes.txt") && !FileManager.default.fileExists(atPath: stored.path)
        }
    }

    func testAttachmentKindsReportTheirAvailability() throws {
        // The desktop's file attachment has no bytes on this iPhone.
        for file in files(under: "projects/atlas-launch/.attachments") where file.lastPathComponent == "release-checklist.txt" {
            try FileManager.default.removeItem(at: file)
        }
        let app = launch()
        openTask(app, "Ship beta build")
        XCTAssertEqual(label(app, "attachment.release-checklist.txt.status"), "Not on this iPhone")
        reach(app, app.buttons["attachment.release-checklist.txt.download"].firstMatch, "the download action")
        XCTAssertEqual(label(app, "attachment.https://example.com/atlas/release.status"), "Link")
        XCTAssertTrue(label(app, "attachment.cmd/release.go:42.status").contains("cmd/release.go:42"))

        choose(app, "attachments.add", "Add Link")
        answerAlert(app, "https://example.com/beta/notes", "Add")
        eventually("the link was written") { try self.markdown("atlas-launch").contains("url|https%3A%2F%2Fexample.com%2Fbeta%2Fnotes|") }
        // The record without bytes still edits: it can be removed, and the
        // code reference is untouched.
        choose(app, "attachment.release-checklist.txt.actions", "Remove Attachment and Its File")
        eventually("the record was removed") {
            let text = try self.markdown("atlas-launch")
            return !text.contains("release-checklist.txt") && text.contains("code|cmd%2Frelease.go%3A42|cmd%2Frelease.go|42|")
        }
    }

    func testProjectCommentsAndProjectNotes() throws {
        let app = launch()
        tab(app, "Browse")
        press(app, "browse.project.atlas-launch")
        press(app, "list.project.settings")
        require(app.staticTexts["comment.0"].firstMatch, "the project's comments")
        press(app, "comments.add")
        input(app, "comment.text").tap()
        input(app, "comment.text").typeText("Phone: the beta date holds")
        app.buttons["comment.save"].firstMatch.tap()
        eventually("the project comment was written") {
            let text = try self.markdown("atlas-launch")
            guard let block = text.components(separatedBy: "<!-- /PROJECT-COMMENTS -->").first else { return false }
            return block.contains("Phone: the beta date holds")
        }
        choose(app, "comment.0.actions", "Edit Comment")
        replace(app, "comment.text", with: "Kickoff agreed: beta in October")
        app.buttons["comment.save"].firstMatch.tap()
        eventually("the project comment edit kept its identity") {
            let text = try self.markdown("atlas-launch")
            return text.contains("<!-- project-comment: 3c9b2f4e-5d6a-4b7c-8d9e-0f1a2b3c4d01 2026-09-02T10:00:00Z -->\nKickoff agreed: beta in October")
        }

        press(app, "project.notes")
        let runbook = app.buttons["note.Release runbook"].firstMatch
        require(runbook, "the project's note")
        runbook.tap()
        eventually("the project note opened") { !self.value(app, "note.body").isEmpty }
    }
}
