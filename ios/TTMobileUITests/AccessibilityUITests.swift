import Foundation
import XCTest

/// The main workflows held to accessibility: the largest Dynamic Type size,
/// VoiceOver labels on every control, dark mode, a small screen, a hardware
/// keyboard, and drafts that survive a relaunch under all of them. Each test
/// reads the app's semantic state export to prove the setting it exercises
/// took effect, rather than trusting that a launch argument was honored, and
/// keeps a screenshot of what it checked.
///
/// ios/scripts/test-ui.sh runs it and passes, through TEST_RUNNER_ variables:
///   TT_UITEST_FIXTURE   the representative compatibility workspace
///   TT_UITEST_WORK_DIR  a host directory each test makes its own root under
///   TT_UITEST_MAX_WIDTH optional: the widest window, in points, the small
///                       screen test accepts (set for the small simulator)
/// The suite fails rather than skips when a required input is missing.
final class AccessibilityUITests: XCTestCase {
    private struct MissingInput: Error {}

    private var root: URL!
    private let workspaceName = "seeded"
    private let timeout: TimeInterval = 20

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

    // MARK: Fixture and launch

    /// The representative fixture with its one wall-clock deferral removed,
    /// selected so the app opens it.
    private func seed() throws {
        guard let fixture = ProcessInfo.processInfo.environment["TT_UITEST_FIXTURE"], FileManager.default.fileExists(atPath: fixture) else {
            XCTFail("TT_UITEST_FIXTURE must name the representative workspace")
            throw MissingInput()
        }
        let workspace = root.appendingPathComponent("workspaces/\(workspaceName)", isDirectory: true)
        try FileManager.default.createDirectory(at: workspace.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: URL(fileURLWithPath: fixture, isDirectory: true), to: workspace)
        let atlas = workspace.appendingPathComponent("projects/atlas-launch/tasks.md")
        let tasks = try String(contentsOf: atlas, encoding: .utf8)
        try tasks.replacingOccurrences(of: ", deferred: 2026-10-07T09:00:00Z", with: "").write(to: atlas, atomically: true, encoding: .utf8)
        try Data(workspaceName.utf8).write(to: root.appendingPathComponent("selected-workspace"))
    }

    private var stateFile: URL { root.appendingPathComponent("state.json") }

    private enum TextSize: String {
        case standard = "UICTContentSizeCategoryL"
        case accessibility2 = "UICTContentSizeCategoryAccessibilityL"
        case largest = "UICTContentSizeCategoryAccessibilityXXXL"
    }

    private func launch(dark: Bool = false, textSize: TextSize = .standard) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = [
            "TT_UITEST_ROOT": root.path,
            "TT_UITEST_DEVICE_ID": "ui-test-phone",
            "TT_UITEST_STATE_EXPORT": stateFile.path,
            "TT_UITEST_APPEARANCE": dark ? "dark" : "light",
        ]
        app.launchArguments = ["-UIPreferredContentSizeCategoryName", textSize.rawValue]
        app.launch()
        require(app.tabBars.buttons["Today"], "the open workspace")
        return app
    }

    // MARK: Semantic state

    private func state() -> [String: Any] {
        guard let data = try? Data(contentsOf: stateFile),
              let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else { return [:] }
        return object
    }

    private func stateValue(_ path: String) -> Any? {
        var node: Any? = state()
        for key in path.split(separator: ".") {
            node = (node as? [String: Any])?[String(key)]
        }
        return node is NSNull ? nil : node
    }

    /// Waits until the export reports VALUE at PATH.
    private func requireState(_ path: String, _ value: String, file: StaticString = #filePath, line: UInt = #line) {
        eventually("the state export reports \(path) = \(value) (last: \(String(describing: stateValue(path))))", file: file, line: line) {
            String(describing: self.stateValue(path) ?? "nil") == value
        }
    }

    private func todayTitles() -> [String] {
        ((stateValue("lists.today.tasks") as? [[String: Any]]) ?? []).compactMap { $0["title"] as? String }
    }

    private func drafts() -> [String] {
        (stateValue("drafts") as? [String]) ?? []
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
            Thread.sleep(forTimeInterval: 0.25)
        }
        XCTFail("\(what) never happened", file: file, line: line)
    }

    private func row(_ app: XCUIApplication, _ title: String) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "identifier == 'task.row' AND label CONTAINS %@", title)).firstMatch
    }

    /// A text input by identifier, scrolled into view: at the largest text
    /// sizes a form runs well past one screen.
    private func input(_ app: XCUIApplication, _ identifier: String) -> XCUIElement {
        let field = app.textFields[identifier].firstMatch
        for _ in 0..<8 where !(field.exists || app.textViews[identifier].exists) {
            app.swipeUp()
        }
        if field.waitForExistence(timeout: 2) { return field }
        let view = app.textViews[identifier].firstMatch
        require(view, identifier)
        return view
    }

    /// Keeps a screenshot of what a test checked in the result bundle.
    private func screenshot(_ app: XCUIApplication, _ name: String) {
        let attachment = XCTAttachment(screenshot: app.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }

    /// Runs one accessibility audit on what is showing and fails on any issue
    /// it finds in the app's own content, naming the element. The navigation
    /// bar's toolbar and the search field are drawn by the system, which the
    /// app cannot restyle, as is the software keyboard, and content scrolled
    /// under the translucent tab bar is measured against the bar rather than
    /// its own background, so those issues are kept as an attachment instead.
    private func audit(_ app: XCUIApplication, _ types: XCUIAccessibilityAuditType, _ screen: String,
                       file: StaticString = #filePath, line: UInt = #line) throws {
        let systemChrome = app.navigationBars.allElementsBoundByIndex.map { $0.frame }
        // The software keyboard and its prediction bar are the system's too.
        let overlays = (app.tabBars.allElementsBoundByIndex + [app.keyboards.firstMatch, app.otherElements["SystemInputAssistantView"].firstMatch])
            .filter { $0.exists }.map { $0.frame }
        // UIKit reports the sigil accessory separately from the keyboard.
        // Ignore content fully behind it, but still audit its own buttons.
        let sigils = app.scrollViews["capture.sigils"]
        let sigilFrame = sigils.exists ? sigils.frame : .null
        let failures = AuditLog(), systemIssues = AuditLog()
        try app.performAccessibilityAudit(for: types) { @Sendable issue in
            // The audit calls back on the main thread, where elements are read.
            nonisolated(unsafe) let issue = issue
            MainActor.assumeIsolated {
                let element = issue.element
                let described = "\(issue.compactDescription): \(element?.debugDescription.prefix(200) ?? "no element")"
                let frame = element?.frame ?? .null
                if element?.elementType == .searchField || systemChrome.contains(where: { $0.contains(frame) })
                    || overlays.contains(where: { $0.intersects(frame) })
                    || (sigilFrame.contains(frame) && !(element?.identifier.hasPrefix("capture.sigil.") ?? false)) {
                    systemIssues.append(described)
                } else {
                    failures.append(described)
                }
            }
            return true
        }
        if !systemIssues.lines.isEmpty {
            let note = XCTAttachment(string: systemIssues.lines.joined(separator: "\n\n"))
            note.name = "system chrome issues on \(screen)"
            note.lifetime = .keepAlways
            add(note)
        }
        let issues = failures.lines
        XCTAssertEqual(issues, [], "the \(screen) audit found issues", file: file, line: line)
    }

    /// The issues one audit reported, collected from its handler.
    private final class AuditLog: @unchecked Sendable {
        private let lock = NSLock()
        private var collected: [String] = []

        func append(_ line: String) {
            lock.withLock { collected.append(line) }
        }

        var lines: [String] { lock.withLock { collected } }
    }

    private func openCapture(_ app: XCUIApplication) {
        let open = app.buttons["capture.open"]
        require(open, "the capture button")
        open.tap()
        require(app.buttons["capture.save"], "the capture sheet")
    }

    /// Opens a task from the list showing, scrolling to its row first: at
    /// the largest text sizes on a small screen, one row fills the screen.
    private func openTask(_ app: XCUIApplication, _ title: String) {
        let link = row(app, title)
        for _ in 0..<6 where !(link.exists && link.isHittable) {
            app.swipeUp()
        }
        require(link, "the \(title) row")
        link.tap()
        require(app.staticTexts["detail.title"], "the \(title) detail")
    }

    private func back(_ app: XCUIApplication) {
        let button = app.navigationBars.buttons["BackButton"].firstMatch
        require(button, "the back button")
        button.tap()
    }

    // MARK: Tests

    func testTheLargestTextSizeKeepsTheMainWorkflowsWhole() throws {
        let app = launch(textSize: .largest)
        requireState("environment.dynamic_type", "accessibility5")
        requireState("phase", "ready")

        // Today's first row reads whole: its title, its priority, and its due
        // date, each named for VoiceOver, none clipped.
        let ship = row(app, "Ship beta build")
        require(ship, "the Ship beta build row")
        XCTAssertTrue(ship.label.contains("priority 1"), ship.label)
        XCTAssertTrue(ship.label.contains("due "), ship.label)
        screenshot(app, "today-largest-text")
        try audit(app, [.dynamicType, .textClipped], "Today at the largest text size")

        openTask(app, "Ship beta build")
        XCTAssertEqual(app.staticTexts["detail.title"].label, "Ship beta build")
        screenshot(app, "detail-largest-text")
        try audit(app, [.dynamicType], "the task detail at the largest text size")
        back(app)

        // Capture still saves at this size.
        openCapture(app)
        let text = input(app, "capture.text")
        text.tap()
        text.typeText("Large text capture $today")
        screenshot(app, "capture-largest-text")
        try audit(app, [.dynamicType], "capture at the largest text size")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the capture sheet")
        // At this size the new row is below the fold of a lazy list, so the
        // semantic export is what says Today holds it.
        eventually("Today lists the capture") { self.todayTitles().contains("Large text capture") }
    }

    func testVoiceOverLabelsNameEveryControl() throws {
        let app = launch()
        requireState("phase", "ready")

        // Icon-only controls carry spoken names.
        XCTAssertEqual(app.buttons["capture.open"].label, "Capture")
        XCTAssertEqual(app.buttons["sync.open"].firstMatch.label, "Sync status")
        for tab in ["Today", "Inbox", "Browse"] {
            XCTAssertTrue(app.tabBars.buttons[tab].exists, "the \(tab) tab has no spoken name")
        }

        // A row speaks its state, its title, then its badges in words.
        let ship = row(app, "Ship beta build")
        require(ship, "the Ship beta build row")
        XCTAssertTrue(ship.label.hasPrefix("doing, Ship beta build"), ship.label)
        for word in ["priority 1", "due ", "Today", "pinned"] {
            XCTAssertTrue(ship.label.contains(word), "the row does not say \(word): \(ship.label)")
        }
        XCTAssertGreaterThanOrEqual(app.buttons.matching(identifier: "task.row").count, 2)

        // No button anywhere on the main screens is unnamed, and the audits
        // find nothing described too little.
        func requireNamedButtons(_ screen: String) {
            // The software keyboard and its prediction bar are the system's.
            let keyboard = [app.keyboards.firstMatch, app.otherElements["SystemInputAssistantView"].firstMatch]
                .filter { $0.exists }.map { $0.frame }
            for button in app.buttons.allElementsBoundByIndex where button.exists && button.isHittable {
                if keyboard.contains(where: { $0.intersects(button.frame) }) { continue }
                XCTAssertFalse(button.label.trimmingCharacters(in: .whitespaces).isEmpty,
                               "an unnamed button on \(screen): \(button.debugDescription)")
            }
        }
        requireNamedButtons("Today")
        try audit(app, [.sufficientElementDescription], "Today")

        app.tabBars.buttons["Inbox"].tap()
        require(row(app, "Call the bank"), "the Inbox")
        requireNamedButtons("Inbox")
        try audit(app, [.sufficientElementDescription], "Inbox")

        app.tabBars.buttons["Browse"].tap()
        require(app.buttons["browse.mode.all"].firstMatch, "Browse")
        requireNamedButtons("Browse")
        try audit(app, [.sufficientElementDescription], "Browse")

        app.tabBars.buttons["Today"].tap()
        openTask(app, "Ship beta build")
        requireNamedButtons("the task detail")
        try audit(app, [.sufficientElementDescription], "the task detail")
        back(app)

        openCapture(app)
        // The keyboard-only shortcuts draw nothing and VoiceOver never lands
        // on them.
        for hidden in ["Close Window"] {
            XCTAssertFalse(app.buttons[hidden].exists, "VoiceOver can reach the hidden \(hidden) shortcut")
        }
        XCTAssertEqual(app.buttons.matching(NSPredicate(format: "label == 'Save'")).count, 1, "VoiceOver finds a second Save")
        requireNamedButtons("capture")
        try audit(app, [.sufficientElementDescription], "capture")
        app.buttons["capture.close"].tap()

        app.buttons["sync.open"].firstMatch.tap()
        require(app.navigationBars["Sync"], "the sync sheet")
        requireNamedButtons("sync")
        try audit(app, [.sufficientElementDescription], "sync")
        screenshot(app, "sync-voiceover-labels")
    }

    func testDarkModeDrawsEveryMainScreenReadably() throws {
        let app = launch(dark: true)
        requireState("environment.color_scheme", "dark")
        requireState("phase", "ready")
        require(row(app, "Ship beta build"), "Today in dark mode")
        screenshot(app, "today-dark")
        try audit(app, [.contrast], "Today in dark mode")

        openTask(app, "Ship beta build")
        screenshot(app, "detail-dark")
        try audit(app, [.contrast], "the task detail in dark mode")
        back(app)

        openCapture(app)
        screenshot(app, "capture-dark")
        try audit(app, [.contrast], "capture in dark mode")
        app.buttons["capture.close"].tap()

        app.tabBars.buttons["Browse"].tap()
        require(app.buttons["browse.mode.all"].firstMatch, "Browse in dark mode")
        screenshot(app, "browse-dark")
        try audit(app, [.contrast], "Browse in dark mode")
    }

    func testActionReceiptKeepsUndoReachableAtLargestTextSize() throws {
        let app = launch(textSize: .largest)
        requireState("environment.dynamic_type", "accessibility5")
        row(app, "Ship beta build").swipeLeft()
        let start = app.buttons["task.swipe.startPause"].firstMatch
        require(start, "the Start action")
        start.tap()
        let receipt = app.staticTexts["list.receipt"]
        require(receipt, "the action receipt")
        let undo = app.buttons["list.undo"]
        XCTAssertTrue(undo.isHittable)
        XCTAssertTrue(app.windows.firstMatch.frame.contains(undo.frame))
        screenshot(app, "action-receipt-largest-text")
        undo.tap()
        XCTAssertTrue(receipt.waitForExistence(timeout: timeout))
        XCTAssertTrue(receipt.label.hasPrefix("Undone:"))
    }

    func testASmallScreenKeepsEveryMainActionReachable() throws {
        let app = launch(textSize: .accessibility2)
        requireState("phase", "ready")
        let window = app.windows.firstMatch.frame
        if let limit = ProcessInfo.processInfo.environment["TT_UITEST_MAX_WIDTH"].flatMap(Double.init) {
            XCTAssertLessThanOrEqual(window.width, limit, "this run was meant for a small screen")
        }
        let note = XCTAttachment(string: "window \(Int(window.width)) x \(Int(window.height)) points")
        note.name = "window size"
        note.lifetime = .keepAlways
        add(note)

        func requireReachable(_ element: XCUIElement, _ what: String) {
            require(element, what)
            XCTAssertTrue(element.isHittable, "\(what) cannot be tapped")
            XCTAssertTrue(window.contains(element.frame), "\(what) lies outside the \(Int(window.width))-point window")
        }
        requireReachable(app.buttons["capture.open"], "the capture button")
        requireReachable(app.buttons["sync.open"].firstMatch, "the sync button")
        for tab in ["Today", "Inbox", "Browse"] {
            requireReachable(app.tabBars.buttons[tab], "the \(tab) tab")
        }
        requireReachable(row(app, "Ship beta build"), "the first Today row")
        screenshot(app, "today-small-screen")

        openCapture(app)
        requireReachable(app.buttons["capture.save"], "Save")
        requireReachable(app.buttons["capture.close"], "Close")
        let text = input(app, "capture.text")
        text.tap()
        text.typeText("Small screen capture $today")
        screenshot(app, "capture-small-screen")
        app.buttons["capture.save"].tap()
        requireAbsent(app.buttons["capture.save"], "the capture sheet")
        let captured = row(app, "Small screen capture")
        require(captured, "the capture in Today")
        if !captured.isHittable { app.swipeUp() }
        captured.tap()
        require(app.staticTexts["detail.title"], "the captured task's detail")
        requireReachable(app.buttons["detail.edit"].firstMatch, "Edit")
    }

    func testAHardwareKeyboardDrivesTabsAndCapture() throws {
        let app = launch()
        requireState("phase", "ready")

        app.typeKey("2", modifierFlags: .command)
        requireState("screen.tab", "inbox")
        require(row(app, "Call the bank"), "the Inbox after Command-2")
        app.typeKey("3", modifierFlags: .command)
        requireState("screen.tab", "notes")
        require(app.buttons["notes.new"].firstMatch, "New Note after Command-3")
        app.typeKey("4", modifierFlags: .command)
        requireState("screen.tab", "browse")
        app.typeKey("1", modifierFlags: .command)
        requireState("screen.tab", "today")

        // Command-N opens capture with the text field focused; Command-W
        // closes it and keeps the draft, as a swipe down does.
        app.typeKey("n", modifierFlags: .command)
        requireState("screen.sheet", "capture")
        require(app.buttons["capture.save"], "the capture sheet")
        app.typeText("Keyboard capture $today")
        XCTAssertEqual(input(app, "capture.text").value as? String, "Keyboard capture $today")
        app.typeKey("w", modifierFlags: .command)
        requireAbsent(app.buttons["capture.save"], "the capture sheet after Command-W")
        requireState("screen.sheet", "nil")
        eventually("the closed sheet's draft was kept") { self.drafts() == ["capture.json"] }

        // Command-N brings the draft back, and Command-S saves it.
        app.typeKey("n", modifierFlags: .command)
        require(app.descendants(matching: .any)["capture.restored"].firstMatch, "the restored-draft notice")
        XCTAssertEqual(input(app, "capture.text").value as? String, "Keyboard capture $today")
        screenshot(app, "capture-keyboard-restored")
        app.typeKey("s", modifierFlags: .command)
        requireAbsent(app.buttons["capture.save"], "the capture sheet after Command-S")
        require(row(app, "Keyboard capture"), "the keyboard capture in Today")
        eventually("the saved capture took its draft with it") { self.drafts() == [] }
    }

    func testDraftsComeBackAfterARelaunchAtTheLargestTextSizeInDarkMode() throws {
        var app = launch(dark: true, textSize: .largest)
        requireState("environment.color_scheme", "dark")
        requireState("environment.dynamic_type", "accessibility5")

        // An edit draft, then the process is killed with its sheet open.
        openTask(app, "Order bathroom tiles")
        app.buttons["detail.edit"].firstMatch.tap()
        require(app.buttons["edit.save"], "the edit sheet")
        let context = input(app, "form.context")
        for _ in 0..<3 where !context.isHittable { app.swipeUp() }
        context.tap()
        context.typeText("phone")
        eventually("the edit draft was written") { self.drafts().contains { $0.hasPrefix("edit-") } }
        app.terminate()

        // A capture draft, killed the same way.
        app = launch(dark: true, textSize: .largest)
        requireState("phase", "ready")
        openCapture(app)
        let text = input(app, "capture.text")
        text.tap()
        text.typeText("Unsent capture #errand")
        eventually("the capture draft was written") { self.drafts().contains("capture.json") }
        XCTAssertEqual(drafts().count, 2, "\(drafts())")
        app.terminate()

        app = launch(dark: true, textSize: .largest)
        requireState("phase", "ready")
        XCTAssertEqual(drafts().count, 2, "a relaunch lost a draft: \(drafts())")

        openCapture(app)
        require(app.descendants(matching: .any)["capture.restored"].firstMatch, "the restored-draft notice")
        XCTAssertEqual(input(app, "capture.text").value as? String, "Unsent capture #errand")
        screenshot(app, "capture-restored-dark-largest")
        app.buttons["capture.actions"].tap()
        app.buttons["capture.discard"].tap()
        app.buttons["Discard Draft"].tap()
        eventually("Confirmed discard removed the capture draft") { !self.drafts().contains("capture.json") }

        openTask(app, "Order bathroom tiles")
        app.buttons["detail.edit"].firstMatch.tap()
        require(app.descendants(matching: .any)["edit.restored"].firstMatch, "the restored-edit notice")
        let restored = input(app, "form.context")
        if !restored.isHittable { app.swipeUp() }
        XCTAssertEqual(restored.value as? String, "phone")
        screenshot(app, "edit-restored-dark-largest")
    }
}
