import XCTest
import GhosttyKit
@testable import tt

/// tt's window title is `pulse.WindowTitle` in the Go tree; these pin the
/// reading tt.app's Dock badge depends on.
final class WindowTitleTests: XCTestCase {
    func testOverdueCountIsReadFromTtsPulseTitle() {
        XCTAssertEqual(TTWindowTitle.overdueCount(in: "tt · inbox 2 · today 3 · overdue 1"), 1)
        XCTAssertEqual(TTWindowTitle.overdueCount(in: "tt · overdue 12"), 12)
        XCTAssertEqual(TTWindowTitle.badgeLabel(for: "tt · overdue 12"), "12")
    }

    func testNoBadgeWithoutAnOverdueCountOrForAnotherProgramsTitle() {
        for title in ["", "tt", "tt · inbox 2 · today 3", "vim · overdue 3", "tt · overdue", "tt · overdue 0", "tt · overdue x"] {
            XCTAssertNil(TTWindowTitle.overdueCount(in: title), title)
            XCTAssertNil(TTWindowTitle.badgeLabel(for: title), title)
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
        XCTAssertTrue(TTNotificationsPreference.load(from: defaults))
        XCTAssertEqual(TTNotificationsPreference.arguments(enabled: true), ["--notify"])
        TTNotificationsPreference.save(false, to: defaults)
        XCTAssertFalse(TTNotificationsPreference.load(from: defaults))
        XCTAssertEqual(TTNotificationsPreference.arguments(enabled: false), [])
    }

    /// An older tt refuses an unknown flag and never starts, so the flag is
    /// passed only to a tt whose help lists it.
    func testNotifyIsPassedOnlyToATtThatListsIt() {
        XCTAssertEqual(TTNotificationsPreference.arguments(enabled: true, supported: false), [])
        let help = """
        Flags:
              --no-advisories   hide ambient advisory rows
              --notify          send a desktop notification (OSC 777) when …
          -h, --help            help for tt
        """
        XCTAssertTrue(TTVersion.helpLists(flag: "--notify", in: help))
        XCTAssertFalse(TTVersion.helpLists(flag: "--notify", in: "Flags:\n      --no-advisories   use --notify elsewhere"))
    }

    func testDockBadgeDefaultsOnAndRoundTrips() {
        XCTAssertTrue(TTDockBadgePreference.load(from: defaults))
        TTDockBadgePreference.save(false, to: defaults)
        XCTAssertFalse(TTDockBadgePreference.load(from: defaults))
    }

    func testPreferencesModelSavesBothAndAnnouncesTheBadge() {
        let model = PreferencesModel(defaults: defaults)
        var announced: [Bool] = []
        model.onDockBadgeChange = { announced.append($0) }
        model.dockBadge = false
        model.notificationsEnabled = false
        XCTAssertEqual(announced, [false])
        XCTAssertFalse(TTDockBadgePreference.load(from: defaults))
        XCTAssertFalse(TTNotificationsPreference.load(from: defaults))
    }

    /// tt's OSC 777 reaches the delegate, which presents it; it is no longer
    /// one of the refused Ghostty-app actions.
    func testDesktopNotificationsAreForwardedToTheApp() {
        XCTAssertEqual(Ghostty.disposition(of: GHOSTTY_ACTION_DESKTOP_NOTIFICATION), .surface)
    }

    func testANotificationIsDroppedWhilePreferencesSayOff() {
        let presenter = NotificationPresenter()
        var asked = false
        presenter.isEnabled = { asked = true; return false }
        presenter.post(title: "tt · check-in", body: "Call the bank")
        XCTAssertTrue(asked)
    }
}
