import Foundation
import XCTest

/// The iPhone shell over realistic seeded workspaces, offline, against the
/// real gomobile-bound engine: onboarding, workspace access, sync status, the
/// Today, Inbox, and Browse destinations, every mode, projects, lenses,
/// search, filters, sorting, hierarchy, and dependencies, and the loading,
/// empty, unavailable, and error states.
///
/// ios/scripts/test-ui.sh runs it and passes, through TEST_RUNNER_ variables:
///   TT_UITEST_FIXTURE   the representative compatibility workspace
///   TT_UITEST_EXCHANGE  an exchange folder a desktop `tt sync enroll` published
///   TT_UITEST_WORK_DIR  a host directory each test makes its own root under
/// The suite fails rather than skips when any is missing.
final class BrowsingUITests: XCTestCase {
    /// A required input is missing; the failure is already recorded.
    private struct MissingInput: Error {}

    private var root: URL!
    private let timeout: TimeInterval = 20

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

    /// How a seeded workspace copy differs from the committed fixture.
    private enum Seed {
        /// The fixture, with the one wall-clock deferral removed so the
        /// project outline is the same on every day the suite runs.
        case representative
        /// As representative, with a dependency target deleted.
        case danglingDependency
        /// As representative, written by a newer catalog schema.
        case newerSchema
    }

    @discardableResult
    private func seed(_ name: String, _ seed: Seed) throws -> URL {
        let target = root.appendingPathComponent("workspaces/\(name)", isDirectory: true)
        try FileManager.default.createDirectory(at: target.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: environmentURL("TT_UITEST_FIXTURE"), to: target)
        let atlas = target.appendingPathComponent("projects/atlas-launch/tasks.md")
        var tasks = try String(contentsOf: atlas, encoding: .utf8)
        tasks = try replacing(", deferred: 2026-10-07T09:00:00Z", with: "", in: tasks)
        switch seed {
        case .representative:
            break
        case .danglingDependency:
            guard let start = tasks.range(of: "## Task: Await signing certificate\n"),
                  let end = tasks.range(of: "## Task:", range: start.upperBound..<tasks.endIndex)
            else {
                XCTFail("the fixture no longer holds the signing task")
                throw MissingInput()
            }
            tasks.removeSubrange(start.lowerBound..<end.lowerBound)
        case .newerSchema:
            let catalog = target.appendingPathComponent(".tt/workspace.json")
            let text = try replacing("\"schema_version\": 21", with: "\"schema_version\": 99", in: String(contentsOf: catalog, encoding: .utf8))
            try text.write(to: catalog, atomically: true, encoding: .utf8)
        }
        try tasks.write(to: atlas, atomically: true, encoding: .utf8)
        return target
    }

    private func replacing(_ old: String, with new: String, in text: String) throws -> String {
        guard text.contains(old) else {
            XCTFail("the fixture no longer holds \(old)")
            throw MissingInput()
        }
        return text.replacingOccurrences(of: old, with: new)
    }

    private func select(_ name: String) throws {
        try Data(name.utf8).write(to: root.appendingPathComponent("selected-workspace"))
    }

    private func launch(_ extra: [String: String] = [:]) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = ["TT_UITEST_ROOT": root.path, "TT_UITEST_DEVICE_ID": "ui-test-phone"].merging(extra) { $1 }
        app.launch()
        return app
    }

    private func launchSeeded() throws -> XCUIApplication {
        try seed("seeded", .representative)
        try select("seeded")
        let app = launch()
        require(app.staticTexts["Ship beta build"], "the seeded Today list")
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

    private func text(_ app: XCUIApplication, _ value: String) -> XCUIElement {
        app.staticTexts[value].firstMatch
    }

    /// A task row: its label is the state, then the title, then its badges.
    private func row(_ app: XCUIApplication, _ title: String) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label CONTAINS %@", ", \(title)")).firstMatch
    }

    private func tab(_ app: XCUIApplication, _ name: String) {
        let button = app.tabBars.buttons[name]
        require(button, "the \(name) tab")
        button.tap()
    }

    private func open(_ app: XCUIApplication, _ identifier: String) {
        let link = app.buttons[identifier].firstMatch
        if !link.waitForExistence(timeout: 5) {
            app.swipeUp()
        }
        require(link, identifier)
        link.tap()
    }

    /// Pops one screen. A tap that only dismissed an active search is
    /// repeated, so the next step always starts on the screen below.
    private func back(_ app: XCUIApplication) {
        let screen = app.navigationBars.firstMatch.identifier
        for _ in 0..<3 {
            let button = app.navigationBars.buttons["BackButton"].firstMatch
            guard button.waitForExistence(timeout: 2) else { break }
            button.tap()
            let left = expectation(for: NSPredicate(format: "exists == false"), evaluatedWith: app.navigationBars[screen])
            if XCTWaiter.wait(for: [left], timeout: 2) == .completed { return }
        }
    }

    private func search(_ app: XCUIApplication, _ query: String) {
        let field = app.searchFields.firstMatch
        require(field, "the search field")
        field.tap()
        field.typeText(query)
    }

    private func clearSearch(_ app: XCUIApplication) {
        let field = app.searchFields.firstMatch
        let clear = field.buttons.firstMatch
        if clear.exists { clear.tap() }
        // An active search replaces the bar's back button with Close (Cancel
        // before iOS 26); leaving it returns the list unsearched.
        for name in ["Close", "Cancel"] {
            let dismiss = app.navigationBars.buttons[name].firstMatch
            if dismiss.exists {
                dismiss.tap()
                break
            }
        }
    }

    /// Picks a menu option. A picker in a menu draws its options inline or
    /// behind a submenu named for it, depending on the system, so the
    /// submenu is opened only when the option is not already shown.
    private func choose(_ app: XCUIApplication, _ option: String, under submenu: String) {
        let item = app.buttons[option].firstMatch
        if !item.waitForExistence(timeout: 2) {
            app.buttons[submenu].firstMatch.tap()
        }
        require(item, "the \(option) menu option")
        item.tap()
    }

    // MARK: Tests

    func testSeededTodayInboxAndBrowseModes() throws {
        let app = try launchSeeded()
        require(text(app, "Order bathroom tiles"), "Today's second planned task")
        XCTAssertFalse(text(app, "Call the bank").exists, "an inbox task leaked into Today")

        tab(app, "Inbox")
        require(text(app, "Call the bank"), "the Unfiled inbox task")
        require(text(app, "Proofread release notes"), "the project inbox task")

        tab(app, "Browse")
        open(app, "browse.mode.next")
        require(text(app, "Pick paint colors"), "the Next task")
        XCTAssertFalse(text(app, "Finish the novel").exists, "a parked project's Next task is shown")
        back(app)
        open(app, "browse.mode.due")
        require(text(app, "Await signing certificate"), "a dated task in Due")
        require(text(app, "Ship beta build"), "the other dated task in Due")
        back(app)
        open(app, "browse.mode.all")
        require(text(app, "Localize installer"), "a paused task in All")
        XCTAssertFalse(text(app, "Set up CI signing").exists, "a done task is shown in All")
        back(app)
        open(app, "browse.mode.backlog")
        require(text(app, "Pick paint colors"), "a backlog task")
        XCTAssertFalse(text(app, "Order bathroom tiles").exists, "a doing task is shown in Backlog")
        back(app)
        open(app, "browse.mode.recent")
        // Recent reads the wall clock: the fixture's activity ages out of its
        // 30 days, so either answer is right, and an error or a hang is not.
        let deadline = Date().addingTimeInterval(timeout)
        var answered = false
        while !answered, Date() < deadline {
            answered = app.staticTexts["state.empty"].exists || app.buttons.matching(NSPredicate(format: "label CONTAINS %@", ", ")).count > 3
            if !answered { Thread.sleep(forTimeInterval: 0.5) }
        }
        XCTAssertTrue(answered, "Recent answered with neither tasks nor its empty state")
        XCTAssertFalse(app.staticTexts["state.error"].exists)
        XCTAssertFalse(app.staticTexts["state.loading"].exists)
    }

    func testProjectsOutlineAndRelations() throws {
        let app = try launchSeeded()
        tab(app, "Browse")
        open(app, "browse.project.atlas-launch")
        require(row(app, "Ship beta build"), "the project's root task")
        let notes = row(app, "Write release notes")
        require(notes, "a child task in the outline")
        XCTAssertEqual(notes.value as? String, "level 1")
        XCTAssertEqual(row(app, "Proofread release notes").value as? String, "level 2")

        row(app, "Ship beta build").tap()
        require(app.staticTexts["detail.title"], "the task detail")
        XCTAssertEqual(app.staticTexts["detail.title"].label, "Ship beta build")
        require(text(app, "Waiting on notarization log; rerun step 3."), "the resume note")
        let dependency = app.buttons["detail.dependency"].firstMatch
        if !dependency.waitForExistence(timeout: 5) { app.swipeUp() }
        require(dependency, "the dependency link")
        XCTAssertTrue(dependency.label.contains("Pick paint colors"))
        dependency.tap()
        require(app.staticTexts["detail.title"].firstMatch, "the dependency's detail")
        let paintTitle = app.staticTexts.matching(identifier: "detail.title").element(matching: NSPredicate(format: "label == %@", "Pick paint colors"))
        require(paintTitle, "Pick paint colors' detail")
        let dependent = app.buttons["detail.dependent"].firstMatch
        if !dependent.waitForExistence(timeout: 5) { app.swipeUp() }
        require(dependent, "the computed dependent")
        XCTAssertTrue(dependent.label.contains("Ship beta build"))
        XCTAssertTrue(app.buttons["detail.dependency"].firstMatch.label.contains("Await signing certificate"))
        back(app)

        let child = app.buttons["detail.child"].firstMatch
        if !child.waitForExistence(timeout: 5) { app.swipeUp() }
        require(child, "the subtask link")
        XCTAssertTrue(child.label.contains("Write release notes"))
        child.tap()
        let ancestor = app.buttons["detail.ancestor"].firstMatch
        require(ancestor, "the ancestor link")
        XCTAssertTrue(ancestor.label.contains("Ship beta build"))
        XCTAssertEqual(app.buttons.matching(identifier: "detail.child").count, 2)
        back(app)
        back(app)
        back(app)

        open(app, "browse.project.reading-list")
        require(text(app, "Finish the novel"), "the parked project's task")
        back(app)
        open(app, "browse.project.old-website")
        require(text(app, "Hand over credentials"), "the archived project's task")
    }

    func testLensesSearchFiltersAndSorting() throws {
        let app = try launchSeeded()
        tab(app, "Browse")
        open(app, "browse.lens.Atlas release")
        require(text(app, "Ship beta build"), "the lens's match")
        XCTAssertFalse(text(app, "Await signing certificate").exists, "an untagged task matched the lens")
        search(app, "notes")
        require(app.staticTexts["state.empty"], "a lens narrowed to nothing")
        clearSearch(app)
        back(app)

        open(app, "browse.mode.all")
        require(text(app, "Order bathroom tiles"), "All")
        XCTAssertLessThan(row(app, "Ship beta build").frame.minY, row(app, "Await signing certificate").frame.minY,
                          "the default order no longer leads with the priority 1 task")
        search(app, "paint")
        require(text(app, "Pick paint colors"), "the search match")
        requireAbsent(text(app, "Order bathroom tiles"), "a task the search excludes")
        clearSearch(app)
        require(text(app, "Order bathroom tiles"), "All after clearing the search")

        for (query, token, title) in [("#fe", "#feature", "Ship beta build"),
                                      ("Ship #FE", "#feature", "Ship beta build"),
                                      ("##la", "##laptop", "Ship beta build"),
                                      ("$wai", "$waiting", "Await signing certificate"),
                                      ("%AT", "%atlas-launch", "Ship beta build"),
                                      ("Ship %at", "%atlas-launch", "Ship beta build"),
                                      ("!", "!1", "Ship beta build"),
                                      ("*p", "*pin", "Ship beta build"),
                                      ("&a", "&any", "Ship beta build")] {
            search(app, query)
            let suggestion = app.buttons["search.suggestion.\(token)"]
            require(suggestion, "the autocomplete suggestion for \(query)")
            suggestion.tap()
            require(text(app, title), "the selected suggestion's result")
            requireAbsent(text(app, "Order bathroom tiles"), "a task outside the selected suggestion")
            let expected = query.hasPrefix("Ship ") ? "Ship \(token) " : "\(token) "
            XCTAssertEqual(app.searchFields.firstMatch.value as? String, expected)
            clearSearch(app)
            require(text(app, "Order bathroom tiles"), "All after clearing autocomplete")
        }

        for query in ["#feature", "#Feature", "#FEATURE"] {
            search(app, query)
            require(text(app, "Ship beta build"), "the tag search match for \(query)")
            requireAbsent(text(app, "Order bathroom tiles"), "a task without the searched tag")
            clearSearch(app)
            require(text(app, "Order bathroom tiles"), "All after clearing the tag search")
        }

        search(app, "$ready")
        require(app.staticTexts["state.error"], "a refused search")
        XCTAssertTrue(app.staticTexts["state.error.message"].label.contains("ready"))
        clearSearch(app)

        app.buttons["list.filter"].tap()
        choose(app, "Waiting", under: "Status")
        require(text(app, "Await signing certificate"), "the Waiting filter's match")
        requireAbsent(text(app, "Order bathroom tiles"), "a task the filter excludes")
        app.buttons["list.filter"].tap()
        let clearFilters = app.buttons["Clear Filters"]
        // The menu scrolls on the 375-point simulator; its final action may
        // start below the visible status and priority choices.
        if !clearFilters.waitForExistence(timeout: 1) { app.swipeUp() }
        require(clearFilters, "the filter menu's Clear Filters action")
        clearFilters.tap()
        require(text(app, "Order bathroom tiles"), "All after clearing the filter")

        app.buttons["list.sort"].tap()
        choose(app, "Title", under: "Sort by")
        // The first four titles alphabetically, which fit on screen. The
        // default order interleaves them (Collect, Await, Dark, Call).
        let ordered = ["Await signing certificate", "Call the bank", "Collect screenshots", "Dark mode icons"]
        let deadline = Date().addingTimeInterval(timeout)
        func inOrder() -> Bool {
            let rows = ordered.map { row(app, $0) }
            guard rows.allSatisfy({ $0.exists }) else { return false }
            let tops = rows.map { $0.frame.minY }
            return tops == tops.sorted()
        }
        while !inOrder(), Date() < deadline {
            Thread.sleep(forTimeInterval: 0.5)
        }
        XCTAssertTrue(inOrder(), "title order does not lead with \(ordered)")
    }

    func testEmptySearchExplainsAndClearsRestrictionsIndependently() throws {
        let app = try launchSeeded()
        tab(app, "Browse")
        open(app, "browse.mode.all")
        app.buttons["list.filter"].tap()
        choose(app, "Waiting", under: "Status")
        require(text(app, "Await signing certificate"), "the filtered task")
        search(app, "zzzznomatch")
        require(app.staticTexts["state.empty"], "the empty result")
        let message = app.staticTexts["state.empty.message"].label
        XCTAssertTrue(message.contains("zzzznomatch"), message)
        XCTAssertTrue(message.contains("Status: Waiting"), message)
        app.buttons["list.clearSearch"].tap()
        require(text(app, "Await signing certificate"), "the result after clearing only search")
        require(app.staticTexts["list.restrictions"], "the remaining filter")
        search(app, "zzzznomatch")
        require(app.buttons["list.clearFilters"], "filter recovery")
        app.buttons["list.clearFilters"].tap()
        require(app.staticTexts["state.empty"], "the remaining search")
        XCTAssertTrue(app.staticTexts["state.empty.message"].label.contains("zzzznomatch"))
        requireAbsent(app.staticTexts["list.restrictions"], "cleared filter summary")
        app.buttons["list.clearSearch"].tap()
        require(text(app, "Call the bank"), "the recovered full list")
    }

    func testEmptyScopedSearchCanSearchAllAndReturnToItsQuery() throws {
        let app = try launchSeeded()
        search(app, "Call the bank")
        require(app.staticTexts["state.empty"], "the empty Today search")
        require(app.buttons["list.searchAll"], "the explicit wider search")
        app.buttons["list.searchAll"].tap()
        require(text(app, "Call the bank"), "the Inbox task found in All")
        XCTAssertEqual(app.searchFields.firstMatch.value as? String, "Call the bank")
        XCTAssertEqual(app.searchFields.firstMatch.placeholderValue, "Search in All")
        back(app)
        require(app.staticTexts["state.empty"], "the original empty Today search")
        XCTAssertEqual(app.searchFields.firstMatch.value as? String, "Call the bank")
        XCTAssertEqual(app.searchFields.firstMatch.placeholderValue, "Search in Today")
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "Scoped search restored after Search All"
        screenshot.lifetime = .keepAlways
        add(screenshot)
    }

    func testDetailCompletionOffersUndoAndRestoresToday() throws {
        let app = try launchSeeded()
        row(app, "Order bathroom tiles").tap()
        require(app.buttons["detail.status.menu"], "the detail status control")
        app.buttons["detail.status.menu"].tap()
        require(app.buttons["Done"].firstMatch, "the Done status")
        app.buttons["Done"].firstMatch.tap()
        require(app.buttons["detail.undo"], "Undo for the detail action")
        XCTAssertEqual(app.staticTexts["detail.receipt"].label, "Completed “Order bathroom tiles”")
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = "Detail completion with Undo"
        screenshot.lifetime = .keepAlways
        add(screenshot)
        app.buttons["detail.undo"].tap()
        require(app.staticTexts["detail.receipt"], "the undo confirmation")
        XCTAssertTrue(app.staticTexts["detail.receipt"].label.hasPrefix("Undone:"))
        back(app)
        require(text(app, "Order bathroom tiles"), "the restored Today task")
    }

    func testCompletionReceiptCanUndoTheTaskThatLeftToday() throws {
        let app = try launchSeeded()
        XCTAssertEqual(app.searchFields.firstMatch.placeholderValue, "Search in Today")
        row(app, "Order bathroom tiles").swipeLeft()
        let done = app.buttons["task.swipe.done"].firstMatch
        require(done, "the completion action")
        done.tap()
        let receipt = app.staticTexts["list.receipt"]
        require(receipt, "the completion receipt")
        XCTAssertEqual(receipt.label, "Completed “Order bathroom tiles”")
        let confirmation = XCTAttachment(screenshot: app.screenshot())
        confirmation.name = "Completed task receipt"
        confirmation.lifetime = .keepAlways
        add(confirmation)
        requireAbsent(text(app, "Order bathroom tiles"), "the completed task in Today")
        app.buttons["list.undo"].tap()
        require(text(app, "Order bathroom tiles"), "the task and its Today membership restored by Undo")
        XCTAssertTrue(receipt.label.hasPrefix("Undone:"))
        // The parent has open children. A refused completion must not claim
        // success or leave the previous action's Undo button standing.
        row(app, "Ship beta build").swipeLeft()
        app.buttons["task.swipe.done"].firstMatch.tap()
        require(app.staticTexts["list.failure"], "the blocked parent's refusal")
        XCTAssertFalse(app.staticTexts["list.receipt"].exists)
        XCTAssertFalse(app.buttons["list.undo"].exists)
    }

    func testRefreshFailureKeepsTheListAndRetryRecovers() throws {
        let app = try launchSeeded()
        let catalog = root.appendingPathComponent("workspaces/seeded/.tt/workspace.json")
        let permissions = try FileManager.default.attributesOfItem(atPath: catalog.path)[.posixPermissions]!
        try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: catalog.path)
        defer { try? FileManager.default.setAttributes([.posixPermissions: permissions], ofItemAtPath: catalog.path) }
        let list = app.collectionViews.firstMatch
        list.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.4))
            .press(forDuration: 0.1, thenDragTo: list.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.9)))
        require(app.staticTexts["list.refreshFailure"], "the refresh failure beside the retained list")
        let retained = XCTAttachment(screenshot: app.screenshot())
        retained.name = "Refresh failure retains Today"
        retained.lifetime = .keepAlways
        add(retained)
        XCTAssertTrue(text(app, "Ship beta build").exists)
        XCTAssertFalse(app.staticTexts["state.error"].exists)
        try FileManager.default.setAttributes([.posixPermissions: permissions], ofItemAtPath: catalog.path)
        app.buttons["list.retryRefresh"].tap()
        requireAbsent(app.staticTexts["list.refreshFailure"], "the recovered refresh failure")
        XCTAssertTrue(text(app, "Ship beta build").exists)
    }

    func testDetailRefreshAndProjectPickerRecoverFromReadFailures() throws {
        let app = try launchSeeded()
        row(app, "Ship beta build").tap()
        require(app.staticTexts["detail.title"], "the task detail")
        let catalog = root.appendingPathComponent("workspaces/seeded/.tt/workspace.json")
        let permissions = try FileManager.default.attributesOfItem(atPath: catalog.path)[.posixPermissions]!
        defer { try? FileManager.default.setAttributes([.posixPermissions: permissions], ofItemAtPath: catalog.path) }
        try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: catalog.path)
        let list = app.collectionViews.firstMatch
        list.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.3))
            .press(forDuration: 0.1, thenDragTo: list.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.9)))
        require(app.staticTexts["detail.refresh.failure"], "the stale detail explanation")
        XCTAssertTrue(app.staticTexts["detail.title"].exists)
        XCTAssertFalse(app.staticTexts["state.error"].exists)
        let retained = XCTAttachment(screenshot: app.screenshot())
        retained.name = "Detail retained after refresh failure"
        retained.lifetime = .keepAlways
        add(retained)
        try FileManager.default.setAttributes([.posixPermissions: permissions], ofItemAtPath: catalog.path)
        app.buttons["detail.refresh.retry"].tap()
        requireAbsent(app.staticTexts["detail.refresh.failure"], "the recovered detail")

        let move = app.buttons["detail.move"].firstMatch
        for _ in 0..<12 where !move.isHittable { app.swipeUp() }
        require(move, "Move to Project")
        try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: catalog.path)
        move.tap()
        require(app.staticTexts["state.error"], "the project loading error")
        XCTAssertFalse(app.staticTexts["state.empty"].exists)
        try FileManager.default.setAttributes([.posixPermissions: permissions], ofItemAtPath: catalog.path)
        app.buttons["state.action"].tap()
        require(app.buttons["Home Renovation"], "the recovered destination list")
    }

    func testOpeningShowsLoadingThenContent() throws {
        try seed("seeded", .representative)
        try select("seeded")
        // Launch/automation startup can outlast a fixed delay. Release the
        // real workspace open only after the loading screen is observed.
        let gate = root.appendingPathComponent("hold-open")
        try Data().write(to: gate)
        defer { try? FileManager.default.removeItem(at: gate) }
        let app = launch(["TT_UITEST_HOLD_OPEN": "1"])
        let loading = app.descendants(matching: .any)["state.loading"].firstMatch
        require(loading, "the loading state")
        try FileManager.default.removeItem(at: gate)
        require(text(app, "Ship beta build"), "Today after loading")
        XCTAssertFalse(loading.exists)
    }

    func testOnboardingStartsAnEmptyWorkspaceAndCaptures() throws {
        let app = launch()
        require(app.buttons["onboarding.create"], "onboarding")
        app.buttons["onboarding.create"].tap()
        require(app.staticTexts["state.empty"], "Today's empty state")
        XCTAssertEqual(app.staticTexts["state.empty.message"].label, "Nothing is planned for today.")
        app.buttons["list.empty.browse"].tap()
        require(app.navigationBars["Next"], "the destination offered by empty Today")
        back(app)
        tab(app, "Inbox")
        require(app.staticTexts["state.empty"], "Inbox's empty state")

        app.buttons["capture.open"].tap()
        let title = app.textFields["capture.text"].firstMatch
        let field = title.waitForExistence(timeout: 5) ? title : app.textViews["capture.text"].firstMatch
        require(field, "the capture field")
        field.typeText("Buy milk")
        app.buttons["capture.save"].tap()
        require(text(app, "Buy milk"), "the captured task in Inbox")

        app.buttons["sync.open"].tap()
        require(app.staticTexts["sync.local"], "the local save state")
        require(app.staticTexts["sync.noFolder"], "the absent exchange folder")
        XCTAssertTrue(app.buttons["sync.chooseFolder"].isHittable, "folder recovery is not beside the condition")
        XCTAssertFalse(app.staticTexts["Catalog schema"].exists, "technical identifiers should start collapsed")
        let syncScreenshot = XCTAttachment(screenshot: app.screenshot())
        syncScreenshot.name = "sync-folder-recovery-first"
        syncScreenshot.lifetime = .keepAlways
        add(syncScreenshot)
        app.buttons["workspace.switch"].tap()
        require(app.buttons["onboarding.workspace.local"], "the workspace just created, listed for reopening")
        app.buttons["onboarding.workspace.local"].tap()
        tab(app, "Inbox")
        require(text(app, "Buy milk"), "the reopened workspace's capture")
    }

    func testWorkspaceDeletionConfirmsAndPersistsWithoutRemovingOtherWorkspaces() throws {
        let removed = try seed("unused", .representative)
        let kept = try seed("kept", .representative)
        let runtime = root.appendingPathComponent("devices/unused")
        try FileManager.default.createDirectory(at: runtime, withIntermediateDirectories: true)
        try Data("draft recovery".utf8).write(to: runtime.appendingPathComponent("draft"))
        var app = launch()
        require(app.buttons["onboarding.delete.unused"], "the workspace delete control")
        app.buttons["onboarding.delete.unused"].tap()
        let confirmation = app.alerts.firstMatch
        require(confirmation, "the deletion confirmation")
        XCTAssertTrue(confirmation.staticTexts.allElementsBoundByIndex.contains { $0.label.contains("unsaved drafts") })
        confirmation.buttons["Cancel"].tap()
        XCTAssertTrue(FileManager.default.fileExists(atPath: removed.path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: runtime.path))
        app.buttons["onboarding.delete.unused"].tap()
        app.alerts.buttons["Delete from This iPhone"].tap()
        requireAbsent(app.buttons["onboarding.workspace.unused"], "the deleted workspace")
        XCTAssertFalse(FileManager.default.fileExists(atPath: removed.path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: runtime.path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: kept.path))
        app.terminate()
        app = launch()
        require(app.buttons["onboarding.workspace.kept"], "the retained workspace after relaunch")
        XCTAssertFalse(app.buttons["onboarding.workspace.unused"].exists)
        app.buttons["onboarding.workspace.kept"].tap()
        require(text(app, "Ship beta build"), "the retained workspace's tasks")
        app.buttons["sync.open"].tap()
        app.buttons["workspace.switch"].tap()
        require(app.buttons["onboarding.delete.kept"], "deleting the last workspace")
        app.buttons["onboarding.delete.kept"].tap()
        app.alerts.buttons["Delete from This iPhone"].tap()
        requireAbsent(app.buttons["onboarding.workspace.kept"], "the last workspace")
        app.buttons["onboarding.create"].tap()
        require(app.staticTexts["state.empty"], "a fresh workspace after deleting every old one")
    }

    func testDeletedWorkspaceDraftDoesNotReturnWhenNameIsReused() throws {
        let app = launch()
        require(app.buttons["onboarding.create"], "onboarding")
        app.buttons["onboarding.create"].tap()
        require(app.buttons["capture.open"], "the new workspace")
        app.buttons["capture.open"].tap()
        let title = app.textFields["capture.text"].firstMatch
        let field = title.waitForExistence(timeout: 5) ? title : app.textViews["capture.text"].firstMatch
        require(field, "capture draft")
        field.tap()
        field.typeText("Discarded workspace draft")
        app.buttons["capture.close"].tap()
        app.buttons["sync.open"].tap()
        app.buttons["workspace.switch"].tap()
        require(app.buttons["onboarding.delete.local"], "the local workspace's delete control")
        app.buttons["onboarding.delete.local"].tap()
        app.alerts.buttons["Delete from This iPhone"].tap()
        requireAbsent(app.buttons["onboarding.workspace.local"], "the deleted workspace")
        app.buttons["onboarding.create"].tap()
        require(app.buttons["capture.open"], "the replacement workspace")
        app.buttons["capture.open"].tap()
        require(field, "a fresh capture draft")
        XCTAssertFalse((field.value as? String ?? "").contains("Discarded workspace draft"))
    }

    func testUnavailableWorkspaceAndReference() throws {
        try select("gone")
        var app = launch()
        require(app.staticTexts["state.unavailable"], "the unavailable workspace state")
        XCTAssertTrue(app.staticTexts["state.unavailable.message"].label.contains("gone"))
        app.buttons["state.action"].tap()
        require(app.buttons["onboarding.create"], "onboarding after choosing another workspace")
        app.terminate()

        try seed("dangling", .danglingDependency)
        try select("dangling")
        app = launch()
        require(text(app, "Ship beta build"), "the seeded Today list")
        tab(app, "Browse")
        open(app, "browse.project.home-renovation")
        row(app, "Pick paint colors").tap()
        let unavailable = app.otherElements["relation.unavailable"].firstMatch
        let found = unavailable.waitForExistence(timeout: timeout) || app.staticTexts["Unavailable task"].waitForExistence(timeout: 1)
        XCTAssertTrue(found, "the dangling dependency is not shown as unavailable")
        XCTAssertTrue(app.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", "0a7e5d10-2c3b-4f6a-8e9d-000000000105")).firstMatch.exists
            || unavailable.label.contains("0a7e5d10-2c3b-4f6a-8e9d-000000000105"),
            "the unavailable dependency lost its stored locator")
    }

    func testErrorStates() throws {
        try seed("newer", .newerSchema)
        try select("newer")
        let app = launch()
        require(app.staticTexts["state.error"], "the refused workspace")
        XCTAssertTrue(app.staticTexts["state.error.message"].label.contains("newer"))
        app.buttons["state.action"].tap()
        require(app.buttons["onboarding.create"], "onboarding after the refusal")
    }

    func testJoinFromAnExchangeFolder() throws {
        let exchange = root.appendingPathComponent("exchange", isDirectory: true)
        try FileManager.default.copyItem(at: environmentURL("TT_UITEST_EXCHANGE"), to: exchange)
        let app = launch(["TT_UITEST_EXCHANGE_DIR": exchange.path])
        require(app.buttons["onboarding.join"], "onboarding")
        app.buttons["onboarding.join"].tap()
        require(text(app, "Ship beta build"), "the joined workspace's Today")
        tab(app, "Inbox")
        require(text(app, "Call the bank"), "the joined workspace's Inbox")
        app.buttons["sync.open"].tap()
        require(app.staticTexts["sync.local"], "the local save state")
        app.buttons["sync.details"].tap()
        require(app.staticTexts["Conflicts"], "the conflict count")
        XCTAssertFalse(app.staticTexts["sync.noFolder"].exists, "the joined workspace forgot its exchange folder")
    }
}
