import Foundation
import XCTest

/// Real local notifications from the app, driven through SpringBoard: each
/// test launches Docket over a disposable workspace inside the app's own
/// container holding one task whose reminder fires shortly, lets the system
/// deliver it while Docket is in the background, then acts on the delivered
/// notification — acknowledging a check-in, deferring a task that came back
/// from deferral, or tapping the notification to open the task — and reads
/// the outcome back in the app.
///
/// It needs no host path, so it runs on a physical iPhone:
/// ios/scripts/record-reminder-evidence.sh runs it there and records the
/// evidence the session-15 verifier validates. `DOCKET_EXPECTED_APP_EXECUTABLES`
/// (passed as TEST_RUNNER_DOCKET_EXPECTED_APP_EXECUTABLES) names the build the
/// script installed; the running app must report the same digests.
@MainActor
final class ReminderDeliveryUITests: XCTestCase {
    private let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")
    private let timeout: TimeInterval = 30
    /// How long a reminder may take to arrive: a check-in fires on a whole
    /// minute up to two minutes out.
    private let deliveryTimeout: TimeInterval = 200

    override func setUp() async throws {
        continueAfterFailure = false
    }

    // MARK: Tests

    func testAcknowledgingACheckInFromItsNotification() throws {
        let title = "Check in evidence \(stamp)"
        let app = launchSeeded(kind: "check_in", title: title, delay: 30)
        _ = waitForNotification(containing: title)
        tapAction("Acknowledge", title: title)
        let status = reopenReminders(app, expecting: "Acknowledged “\(title)”")
        // A one-off check-in clears once acknowledged, so nothing remains.
        XCTAssertTrue(status.label.contains("0 reminders scheduled"), status.label)
    }

    func testDeferringFromADeferralNotification() throws {
        let title = "Deferral evidence \(stamp)"
        let app = launchSeeded(kind: "deferral", title: title, delay: 30)
        _ = waitForNotification(containing: title)
        tapAction("Defer 1 Hour", title: title)
        let status = reopenReminders(app, expecting: "Deferred “\(title)” (for an hour)")
        // The task is deferred again, so its next wake-up is scheduled.
        XCTAssertTrue(status.label.contains("1 reminder scheduled") && status.label.contains(title), status.label)
    }

    func testTappingANotificationOpensItsTask() throws {
        let title = "Open evidence \(stamp)"
        let app = launchSeeded(kind: "deferral", title: title, delay: 30)
        let notification = waitForNotification(containing: title)
        notification.tap()
        let done = app.buttons["reminder.opened.done"]
        XCTAssertTrue(done.waitForExistence(timeout: timeout), "the opened task sheet")
        XCTAssertTrue(app.staticTexts[title].waitForExistence(timeout: timeout), "the opened task's title")
        done.tap()
        _ = reopenReminders(app, expecting: "Opened “\(title)”", activate: false)
    }

    // MARK: Helpers

    private var stamp: String {
        String(UUID().uuidString.prefix(6))
    }

    /// Launches Docket over a fresh scratch workspace seeded with one reminder,
    /// allows notifications if asked, checks the reminder is scheduled and the
    /// running build is the expected one, then leaves Docket for the home screen.
    private func launchSeeded(kind: String, title: String, delay: Int) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = [
            "DOCKET_UITEST_SCRATCH": "\(name.filter { $0.isLetter || $0.isNumber })-\(stamp)",
            "DOCKET_UITEST_SEED_REMINDER": kind,
            "DOCKET_UITEST_SEED_TITLE": title,
            "DOCKET_UITEST_SEED_DELAY_SECONDS": "\(delay)",
        ]
        app.launch()
        allowNotificationsIfAsked()
        XCTAssertTrue(app.tabBars.buttons["Today"].waitForExistence(timeout: timeout), "the seeded workspace")
        openReminders(app)
        let status = app.staticTexts["reminders.status"]
        let scheduled = NSPredicate(format: "label CONTAINS %@", "1 reminder scheduled")
        if !status.waitForExistence(timeout: timeout) || XCTWaiter.wait(for: [expectation(for: scheduled, evaluatedWith: status)], timeout: timeout) != .completed {
            allowNotificationsIfAsked()
            XCTFail("the seeded reminder was not scheduled: \(status.exists ? status.label : "no status")")
        }
        XCTAssertTrue(status.label.contains(title), status.label)
        if let expected = ProcessInfo.processInfo.environment["DOCKET_EXPECTED_APP_EXECUTABLES"], !expected.isEmpty {
            let build = app.staticTexts["reminders.build"]
            scrollTo(build, in: app)
            XCTAssertEqual(build.label, expected, "the running app is not the build that was installed")
        }
        XCUIDevice.shared.press(.home)
        return app
    }

    private func allowNotificationsIfAsked() {
        let allow = springboard.alerts.buttons["Allow"]
        if allow.waitForExistence(timeout: 8) {
            allow.tap()
        }
    }

    private func openReminders(_ app: XCUIApplication) {
        let sync = app.buttons["sync.open"].firstMatch
        XCTAssertTrue(sync.waitForExistence(timeout: timeout), "the sync button")
        sync.tap()
        let row = app.descendants(matching: .any)["reminders.open"].firstMatch
        XCTAssertTrue(row.waitForExistence(timeout: timeout), "the Reminders row")
        row.tap()
    }

    private func scrollTo(_ element: XCUIElement, in app: XCUIApplication) {
        for _ in 0..<6 where !element.isHittable {
            app.swipeUp()
        }
        XCTAssertTrue(element.waitForExistence(timeout: timeout), "\(element)")
    }

    /// The delivered notification, as a banner or, once the banner has
    /// gone, in Notification Center.
    private func waitForNotification(containing title: String) -> XCUIElement {
        let match = NSPredicate(format: "label CONTAINS %@", title)
        let banner = springboard.descendants(matching: .any).matching(match).firstMatch
        if banner.waitForExistence(timeout: deliveryTimeout) {
            return banner
        }
        // Missed the banner: look in Notification Center.
        let top = springboard.coordinate(withNormalizedOffset: CGVector(dx: 0.25, dy: 0.005))
        top.press(forDuration: 0.1, thenDragTo: springboard.coordinate(withNormalizedOffset: CGVector(dx: 0.25, dy: 0.7)))
        XCTAssertTrue(banner.waitForExistence(timeout: timeout), "no notification for \(title) was delivered")
        return banner
    }

    /// Expands the notification for `title` and taps one of its action
    /// buttons, finding it again in Notification Center if the banner went.
    private func tapAction(_ label: String, title: String) {
        let button = springboard.buttons[label]
        for _ in 0..<3 where !button.exists {
            let notification = waitForNotification(containing: title)
            notification.press(forDuration: 1.5)
            _ = button.waitForExistence(timeout: 5)
        }
        XCTAssertTrue(button.exists, "the notification offers no \(label) action")
        button.tap()
    }

    /// Brings Docket back and reads the Reminders screen: the latest activity
    /// must be `summary`. Answers the status line.
    @discardableResult
    private func reopenReminders(_ app: XCUIApplication, expecting summary: String, activate: Bool = true) -> XCUIElement {
        if activate {
            // Give the background action time to commit before Docket returns.
            sleep(3)
            app.activate()
        }
        if !app.staticTexts["reminders.status"].exists {
            openReminders(app)
        }
        let latest = app.descendants(matching: .any)["reminders.activity.0"]
        let matches = NSPredicate(format: "label CONTAINS %@", summary)
        for _ in 0..<5 {
            if latest.exists, matches.evaluate(with: latest) { break }
            // Leave and re-enter, which re-reads the activity log and plan.
            app.navigationBars.buttons.element(boundBy: 0).tap()
            openRemindersRow(app)
        }
        XCTAssertTrue(latest.waitForExistence(timeout: timeout), "no reminder activity")
        XCTAssertTrue(latest.label.contains(summary), "latest activity: \(latest.label)")
        let status = app.staticTexts["reminders.status"]
        XCTAssertTrue(status.waitForExistence(timeout: timeout))
        return status
    }

    private func openRemindersRow(_ app: XCUIApplication) {
        let row = app.descendants(matching: .any)["reminders.open"].firstMatch
        if row.waitForExistence(timeout: timeout) {
            row.tap()
        }
        sleep(1)
    }
}
