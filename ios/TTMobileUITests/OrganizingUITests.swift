import Foundation
import XCTest

/// Organizing work on the iPhone against the real gomobile-bound engine:
/// checklists edited and promoted, a repeating check-in scheduled and
/// acknowledged, subtasks to the hierarchy limit and a parent that cannot
/// finish over them, a move that carries a reference with it, actions over a
/// selection, and a project's whole lifecycle ending in a confirmed delete.
/// Each test reads the seeded workspace's files on the host afterwards, so
/// what it proves is the durable outcome, not only the screen.
///
/// ios/scripts/test-ui.sh runs it with the same TEST_RUNNER_ inputs as the
/// other UI suites; it fails rather than skips when one is missing.
final class OrganizingUITests: XCTestCase {
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

    private func launch() -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = ["TT_UITEST_ROOT": root.path, "TT_UITEST_DEVICE_ID": "ui-test-phone"]
        app.launch()
        require(app.tabBars.buttons["Today"], "the open workspace")
        return app
    }

    private func markdown(_ slug: String) throws -> String {
        try String(contentsOf: workspace.appendingPathComponent("projects/\(slug)/tasks.md"), encoding: .utf8)
    }

    private func exists(_ relative: String) -> Bool {
        FileManager.default.fileExists(atPath: workspace.appendingPathComponent(relative).path)
    }

    /// One project's catalog record, read as JSON.
    private func project(_ slug: String) throws -> [String: Any]? {
        let data = try Data(contentsOf: workspace.appendingPathComponent(".tt/workspace.json"))
        let document = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        let projects = (document?["data"] as? [String: Any])?["projects"] as? [[String: Any]] ?? []
        return projects.first { $0["slug"] as? String == slug }
    }

    private func projectTags(_ slug: String) throws -> [String] {
        try project(slug)?["tags"] as? [String] ?? []
    }

    private func projectArchived(_ slug: String) throws -> Bool {
        try project(slug)?["archived"] as? Bool ?? false
    }

    private func planning() throws -> String {
        try String(contentsOf: workspace.appendingPathComponent(".tt/planning.json"), encoding: .utf8)
    }

    /// One task's heading, at any depth, through its metadata line.
    private func taskLine(_ title: String, in text: String) -> String? {
        let lines = text.components(separatedBy: "\n")
        guard let heading = lines.firstIndex(where: { $0.hasPrefix("#") && $0.hasSuffix(" Task: \(title)") }),
              heading + 1 < lines.count else { return nil }
        return lines[heading + 1]
    }

    private func taskID(_ title: String, in text: String) -> String? {
        taskLine(title, in: text)?.range(of: "id: ").map { String(taskLine(title, in: text)![$0.upperBound...].prefix(36)) }
    }

    /// The local calendar day `days` from now, as the engine stores it.
    private func day(_ days: Int) -> String {
        let date = Calendar.current.date(byAdding: .day, value: days, to: Date())!
        let parts = Calendar.current.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", parts.year!, parts.month!, parts.day!)
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

    private func openTask(_ app: XCUIApplication, _ title: String) {
        let link = row(app, title)
        require(link, "the \(title) row")
        link.tap()
        require(app.staticTexts["detail.title"], "the \(title) detail")
    }

    /// Opens a related task from the detail screen by its link.
    private func openRelated(_ app: XCUIApplication, _ identifier: String, _ title: String) {
        let link = app.buttons.matching(NSPredicate(format: "identifier == %@ AND label CONTAINS %@", identifier, title)).firstMatch
        for _ in 0..<4 where !link.isHittable {
            app.swipeUp()
        }
        require(link, "the \(title) link")
        link.tap()
        eventually("the \(title) detail") { app.staticTexts["detail.title"].firstMatch.label == title }
    }

    /// Taps a button by identifier, scrolling it into reach first.
    private func press(_ app: XCUIApplication, _ identifier: String) {
        let button = app.buttons[identifier].firstMatch
        // A list draws only the rows near the screen, so a control further
        // down exists only once it is scrolled toward.
        for _ in 0..<5 where !button.waitForExistence(timeout: 2) {
            app.swipeUp()
        }
        require(button, identifier)
        for _ in 0..<4 where !button.isHittable {
            app.swipeUp()
        }
        button.tap()
    }

    /// Picks an option in a confirmation dialog a button raised.
    private func choose(_ app: XCUIApplication, _ identifier: String, _ option: String) {
        press(app, identifier)
        let item = app.buttons[option].firstMatch
        if !item.waitForExistence(timeout: 4) {
            press(app, identifier)
        }
        require(item, "the \(option) option")
        item.tap()
    }

    /// Types into the one text field an alert shows, replacing what it holds,
    /// and confirms with `button`.
    private func answerAlert(_ app: XCUIApplication, _ text: String, _ button: String) {
        let field = app.alerts.textFields.firstMatch
        require(field, "the alert's field")
        field.tap()
        let current = (field.value as? String) ?? ""
        if !current.isEmpty, current != field.placeholderValue {
            field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count + 2))
        }
        field.typeText(text)
        app.alerts.buttons[button].firstMatch.tap()
    }

    private func input(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        let field = app.textFields[identifier].firstMatch
        if field.waitForExistence(timeout: 2) { return field }
        let view = app.textViews[identifier].firstMatch
        require(view, identifier)
        return view
    }

    private func replace(_ app: XCUIApplication, _ identifier: String, with text: String) {
        let element = input(app, identifier)
        element.tap()
        element.coordinate(withNormalizedOffset: CGVector(dx: 0.98, dy: 0.5)).tap()
        let current = (element.value as? String) ?? ""
        if !current.isEmpty, current != element.placeholderValue {
            element.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count + 2))
        }
        element.typeText(text)
    }

    private func failureText(_ app: XCUIApplication, _ identifier: String) -> String {
        let failure = app.staticTexts[identifier].firstMatch
        require(failure, identifier)
        return failure.label
    }

    // MARK: Tests

    func testChecklistEditingAndPromotion() throws {
        let app = launch()
        openTask(app, "Ship beta build")
        openRelated(app, "detail.child", "Write release notes")
        let checklist = { try XCTUnwrap(self.taskLine("Write release notes", in: self.markdown("atlas-launch"))) }

        press(app, "checklist.item.1")
        eventually("the tick was written") { try checklist().contains("checklist: 1|Gather+changelog;1|Draft+summary") }

        press(app, "checklist.add")
        answerAlert(app, "Proofread", "Add")
        eventually("the item was added") { try checklist().contains("1|Draft+summary;0|Proofread") }

        choose(app, "checklist.item.2.actions", "Move Up")
        eventually("the item moved up") { try checklist().contains("checklist: 1|Gather+changelog;0|Proofread;1|Draft+summary") }

        choose(app, "checklist.item.1.actions", "Edit Item")
        answerAlert(app, "Proofread twice", "Save")
        eventually("the item was renamed") { try checklist().contains("0|Proofread+twice") }

        choose(app, "checklist.item.1.actions", "Promote to Subtask")
        eventually("the item became a subtask") {
            let text = try self.markdown("atlas-launch")
            let owner = try checklist()
            return self.taskLine("Proofread twice", in: text) != nil && !owner.contains("Proofread")
        }
        let child = try XCTUnwrap(taskLine("Proofread twice", in: markdown("atlas-launch")))
        XCTAssertTrue(child.contains("parent: 0a7e5d10-2c3b-4f6a-8e9d-000000000102"), child)
        require(app.buttons.matching(NSPredicate(format: "identifier == 'detail.child' AND label CONTAINS 'Proofread twice'")).firstMatch, "the promoted subtask's link")
    }

    func testRecurringCheckInScheduleAndAcknowledge() throws {
        let app = launch()
        tab(app, "Inbox")
        openTask(app, "Call the bank")
        press(app, "detail.checkin.set")
        replace(app, "checkin.repeat", with: "7")
        app.buttons["checkin.save"].firstMatch.tap()
        let today = day(0), nextWeek = day(7)
        eventually("the weekly check-in was written") {
            try self.taskLine("Call the bank", in: self.markdown("unfiled"))?.contains("check_in: \(today), check_in_repeat: 7") == true
        }

        press(app, "detail.checkin.acknowledge")
        eventually("the acknowledgement moved the check-in on a week") {
            let line = try self.taskLine("Call the bank", in: self.markdown("unfiled")) ?? ""
            return line.contains("check_in: \(nextWeek), check_in_repeat: 7, reviewed: ")
        }
        let line = try XCTUnwrap(taskLine("Call the bank", in: markdown("unfiled")))
        XCTAssertTrue(line.contains("state: inbox"), "acknowledging moved the state: \(line)")
        XCTAssertFalse(app.staticTexts["detail.failure"].exists)

        // The schedule opens on its stored day; saved without a repeat it is
        // a one-off, which an early acknowledgement answers and clears.
        press(app, "detail.checkin.set")
        replace(app, "checkin.repeat", with: "")
        app.buttons["checkin.save"].firstMatch.tap()
        eventually("the one-off was written") {
            let line = try self.taskLine("Call the bank", in: self.markdown("unfiled")) ?? ""
            return line.contains("check_in: \(nextWeek)") && !line.contains("check_in_repeat")
        }
        press(app, "detail.checkin.acknowledge")
        eventually("the one-off was answered and cleared") {
            !(try self.taskLine("Call the bank", in: self.markdown("unfiled")) ?? "check_in").contains("check_in")
        }
    }

    func testSubtasksHierarchyLimitAndBlockedCompletion() throws {
        let app = launch()
        tab(app, "Browse")
        press(app, "browse.project.atlas-launch")
        openTask(app, "Dark mode icons")
        press(app, "detail.subtask.add")
        answerAlert(app, "Draw the moon", "Add")
        eventually("the subtask was written") { try self.taskLine("Draw the moon", in: self.markdown("atlas-launch")) != nil }

        // The parent cannot finish over its open subtask, and says which one.
        choose(app, "detail.status.menu", "Done")
        let refusal = failureText(app, "detail.failure")
        XCTAssertTrue(refusal.contains("open subtasks") && refusal.contains("Draw the moon"), refusal)
        XCTAssertTrue(try taskLine("Dark mode icons", in: markdown("atlas-launch"))?.contains("state: someday") == true)

        // Nesting stops at the engine's limit: Proofread release notes sits at
        // level 2, so four more levels fit and a fifth is refused.
        tab(app, "Inbox")
        openTask(app, "Proofread release notes")
        for level in 3...6 {
            press(app, "detail.subtask.add")
            answerAlert(app, "Level \(level)", "Add")
            eventually("level \(level) was written") { try self.taskLine("Level \(level)", in: self.markdown("atlas-launch")) != nil }
            openRelated(app, "detail.child", "Level \(level)")
        }
        let before = try markdown("atlas-launch")
        press(app, "detail.subtask.add")
        answerAlert(app, "Level 7", "Add")
        let limit = failureText(app, "detail.failure")
        XCTAssertTrue(limit.lowercased().contains("depth"), limit)
        XCTAssertEqual(try markdown("atlas-launch"), before, "a refused subtask was written")

        // Outdenting the deepest task lifts it a level.
        press(app, "detail.outdent")
        let levelFour = try XCTUnwrap(taskID("Level 4", in: before))
        eventually("Level 6 moved up beside Level 5") {
            try self.taskLine("Level 6", in: self.markdown("atlas-launch"))?.contains("parent: \(levelFour)") == true
        }
    }

    func testMoveToProjectCarriesItsReferences() throws {
        let app = launch()
        tab(app, "Browse")
        press(app, "browse.project.atlas-launch")
        openTask(app, "Await signing certificate")
        choose(app, "detail.move", "Home Renovation")
        let signing = "0a7e5d10-2c3b-4f6a-8e9d-000000000105"
        eventually("the task moved to Home Renovation") {
            try self.taskLine("Await signing certificate", in: self.markdown("home-renovation")) != nil
        }
        XCTAssertNil(try taskLine("Await signing certificate", in: markdown("atlas-launch")))
        // The dependent in Home Renovation now names the task's new address.
        let paint = try XCTUnwrap(taskLine("Pick paint colors", in: markdown("home-renovation")))
        XCTAssertTrue(paint.contains("depends: 6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a02:\(signing)"), paint)
        // The screen follows the task to its new project rather than calling
        // it unavailable; its project row is at the top of the detail.
        XCTAssertFalse(app.staticTexts["Task unavailable"].exists, "the detail lost the moved task")
        let project = app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'Project, Home Renovation'")).firstMatch
        for _ in 0..<5 where !project.exists {
            app.swipeDown()
        }
        require(project, "the moved task's new project")
        XCTAssertFalse(app.staticTexts["detail.failure"].exists)
    }

    func testBulkActionsOverASelection() throws {
        let app = launch()
        let open = app.buttons["capture.open"]
        require(open, "the capture button")
        open.tap()
        let text = app.textViews["capture.text"].firstMatch.waitForExistence(timeout: 3) ? app.textViews["capture.text"].firstMatch : app.textFields["capture.text"].firstMatch
        text.tap()
        text.typeText("Fold laundry\nWater plants")
        app.buttons["capture.save"].tap()
        eventually("both captures were written") {
            let unfiled = try self.markdown("unfiled")
            return self.taskLine("Fold laundry", in: unfiled) != nil && self.taskLine("Water plants", in: unfiled) != nil
        }
        let unfiled = try markdown("unfiled")
        let ids = try ["Fold laundry", "Water plants"].map { try XCTUnwrap(taskID($0, in: unfiled)) }

        tab(app, "Inbox")
        let select = { (titles: [String]) in
            self.press(app, "list.select")
            for title in titles {
                let link = self.row(app, title)
                self.require(link, "the \(title) row")
                link.tap()
            }
        }
        select(["Fold laundry", "Water plants"])
        choose(app, "list.bulk", "Add to Next")
        eventually("both joined Next") {
            let plan = try self.planning()
            guard let next = plan.range(of: "\"next\"") else { return false }
            return ids.allSatisfy { plan[next.upperBound...].contains($0) }
        }

        choose(app, "list.bulk", "Add Tag…")
        answerAlert(app, "weekend", "Add")
        eventually("both carry the tag") {
            let unfiled = try self.markdown("unfiled")
            return ["Fold laundry", "Water plants"].allSatisfy { self.taskLine($0, in: unfiled)?.contains("tags: weekend") == true }
        }

        choose(app, "list.bulk", "Move to Project…")
        let destination = app.buttons["Atlas Launch"].firstMatch
        require(destination, "the destination project")
        destination.tap()
        eventually("both moved to Atlas Launch") {
            let atlas = try self.markdown("atlas-launch")
            return ["Fold laundry", "Water plants"].allSatisfy { self.taskLine($0, in: atlas) != nil }
        }
        let plan = try planning()
        XCTAssertTrue(ids.allSatisfy { plan.contains($0) }, "the move dropped the plan:\n\(plan)")
        XCTAssertTrue(plan.contains("6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01"), plan)

        // A selection one task of which cannot take the action is refused whole.
        press(app, "list.select")
        tab(app, "Today")
        let before = try markdown("atlas-launch")
        select(["Ship beta build", "Order bathroom tiles"])
        choose(app, "list.bulk", "Mark Done")
        let refusal = failureText(app, "list.failure")
        XCTAssertTrue(refusal.contains("open subtasks"), refusal)
        XCTAssertEqual(try markdown("atlas-launch"), before)
        XCTAssertTrue(try taskLine("Order bathroom tiles", in: markdown("home-renovation"))?.contains("state: done") == false)
    }

    func testProjectLifecycleAndConfirmedDelete() throws {
        let app = launch()
        tab(app, "Browse")
        press(app, "browse.project.new")
        answerAlert(app, "Garden Shed", "Create")
        eventually("the project was created") { self.exists("projects/garden-shed/tasks.md") }
        press(app, "browse.project.garden-shed")

        press(app, "list.project.settings")
        replace(app, "project.name", with: "Garden Workshop")
        replace(app, "project.description", with: "Out back")
        replace(app, "project.tags", with: "home")
        app.buttons["project.save"].firstMatch.tap()
        eventually("the rename moved the project") {
            self.exists("projects/garden-workshop/tasks.md") && !self.exists("projects/garden-shed")
        }
        XCTAssertEqual(try project("garden-workshop")?["description"] as? String, "Out back")
        XCTAssertEqual(try projectTags("garden-workshop"), ["home"])

        // Parking confirms first, then hides the project from Active Projects.
        press(app, "list.project.settings")
        choose(app, "project.park", "Park")
        eventually("the project was parked") { try self.projectTags("garden-workshop").contains("parked") }
        press(app, "project.park")
        eventually("the project was unparked") { try !self.projectTags("garden-workshop").contains("parked") }
        choose(app, "project.archive", "Archive")
        eventually("the project was archived") { try self.projectArchived("garden-workshop") }
        press(app, "project.archive")
        eventually("the project was restored") { try !self.projectArchived("garden-workshop") }
        app.buttons["project.close"].firstMatch.tap()

        // The delete names what it takes; declining keeps everything.
        tab(app, "Browse")
        let browseBack = app.navigationBars.buttons["BackButton"].firstMatch
        if browseBack.waitForExistence(timeout: 2) { browseBack.tap() }
        press(app, "browse.project.home-renovation")
        press(app, "list.project.settings")
        press(app, "project.delete")
        require(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'Deletes 2 tasks'")).firstMatch, "the delete's cascade")
        let cancel = app.buttons["Cancel"].firstMatch
        if cancel.exists { cancel.tap() } else { app.tap() }
        XCTAssertTrue(exists("projects/home-renovation/tasks.md"), "declining deleted the project")

        // Confirming deletes the project and everything it owns.
        press(app, "project.delete")
        let confirm = app.buttons["project.delete.confirm"].firstMatch.waitForExistence(timeout: 4)
            ? app.buttons["project.delete.confirm"].firstMatch : app.buttons["Delete"].firstMatch
        require(confirm, "the delete confirmation")
        confirm.tap()
        eventually("the project was deleted") { !self.exists("projects/home-renovation/tasks.md") }
        XCTAssertNil(try project("home-renovation"))
        XCTAssertFalse(try planning().contains("6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a02"))
    }
}
