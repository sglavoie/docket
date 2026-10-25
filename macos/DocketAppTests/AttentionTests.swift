import XCTest
import GhosttyKit
@testable import Docket

/// Docket's window title is `pulse.WindowTitle` in the Go tree; these pin the
/// reading Docket.app's Dock badge depends on.
final class WindowTitleTests: XCTestCase {
    func testOverdueCountIsReadFromTtsPulseTitle() {
        XCTAssertEqual(DocketWindowTitle.overdueCount(in: "Docket · inbox 2 · today 3 · overdue 1"), 1)
        XCTAssertEqual(DocketWindowTitle.overdueCount(in: "Docket · overdue 12"), 12)
        XCTAssertEqual(DocketWindowTitle.badgeLabel(for: "Docket · overdue 12"), "12")
    }

    func testNoBadgeWithoutAnOverdueCountOrForAnotherProgramsTitle() {
        for title in ["", "Docket", "Docket · inbox 2 · today 3", "vim · overdue 3", "Docket · overdue", "Docket · overdue 0", "Docket · overdue x"] {
            XCTAssertNil(DocketWindowTitle.overdueCount(in: title), title)
            XCTAssertNil(DocketWindowTitle.badgeLabel(for: title), title)
        }
    }
}

final class AttentionPreferenceTests: XCTestCase {
    private var suite = ""
    private var defaults: UserDefaults!

    override func setUpWithError() throws {
        suite = "tt-tests-\(UUID().uuidString)"
        defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suite)
    }

    func testNotificationsDefaultOnAndPassNotifyToTt() {
        XCTAssertTrue(DocketNotificationsPreference.load(from: defaults))
        XCTAssertEqual(DocketNotificationsPreference.arguments(enabled: true), ["--notify"])
        DocketNotificationsPreference.save(false, to: defaults)
        XCTAssertFalse(DocketNotificationsPreference.load(from: defaults))
        XCTAssertEqual(DocketNotificationsPreference.arguments(enabled: false), [])
    }

    /// An older Docket refuses an unknown flag and never starts, so the flag is
    /// passed only to a Docket whose help lists it.
    func testNotifyIsPassedOnlyToATtThatListsIt() {
        XCTAssertEqual(DocketNotificationsPreference.arguments(enabled: true, supported: false), [])
        let help = """
        Flags:
              --no-advisories   hide ambient advisory rows
              --notify          send a desktop notification (OSC 777) when …
          -h, --help            help for docket
        """
        XCTAssertTrue(DocketVersion.helpLists(flag: "--notify", in: help))
        XCTAssertFalse(DocketVersion.helpLists(flag: "--notify", in: "Flags:\n      --no-advisories   use --notify elsewhere"))
    }

    func testDockBadgeDefaultsOnAndRoundTrips() {
        XCTAssertTrue(DocketDockBadgePreference.load(from: defaults))
        DocketDockBadgePreference.save(false, to: defaults)
        XCTAssertFalse(DocketDockBadgePreference.load(from: defaults))
    }

    func testPreferencesModelSavesBothAndAnnouncesTheBadge() {
        let model = PreferencesModel(defaults: defaults)
        var announced: [Bool] = []
        model.onDockBadgeChange = { announced.append($0) }
        model.dockBadge = false
        model.notificationsEnabled = false
        XCTAssertEqual(announced, [false])
        XCTAssertFalse(DocketDockBadgePreference.load(from: defaults))
        XCTAssertFalse(DocketNotificationsPreference.load(from: defaults))
    }

    /// Docket's OSC 777 reaches the delegate, which presents it; it is no longer
    /// one of the refused Ghostty-app actions.
    func testDesktopNotificationsAreForwardedToTheApp() {
        XCTAssertEqual(Ghostty.disposition(of: GHOSTTY_ACTION_DESKTOP_NOTIFICATION), .surface)
    }

    func testANotificationIsDroppedWhilePreferencesSayOff() {
        let presenter = NotificationPresenter()
        var asked = false
        presenter.isEnabled = { asked = true; return false }
        presenter.post(title: "Docket · check-in", body: "Call the bank")
        XCTAssertTrue(asked)
    }
}
