import GhosttyKit
import XCTest
@testable import tt

/// The hosting app has already run `ghostty_init`, so configs load here.
final class GhosttyConfigTests: XCTestCase {
    private var dir: URL!

    override func setUpWithError() throws {
        dir = FileManager.default.temporaryDirectory.appendingPathComponent("tt-config-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: dir)
    }

    private func writeUserConfig(_ text: String) throws -> URL {
        let url = dir.appendingPathComponent("ghostty.conf")
        try text.write(to: url, atomically: true, encoding: .utf8)
        return url
    }

    private let overrides = Ghostty.ConfigOverrides(
        command: URL(fileURLWithPath: "/usr/local/bin/tt"),
        environment: ["PATH": "/opt/bin:/usr/bin", "HOME": "/Users/someone"]
    )

    func testUserConfigPathIsUnderTTAppConfigDir() {
        let home = URL(fileURLWithPath: "/Users/someone")
        XCTAssertEqual(Ghostty.Config.userConfigURL(home: home).path, "/Users/someone/.config/tt-app/ghostty.conf")
    }

    func testOverrideLinesForceCommandAndCloseBehavior() {
        XCTAssertEqual(overrides.lines, [
            "command = direct:/usr/local/bin/tt",
            "wait-after-command = false",
            "confirm-close-surface = false",
            "env = HOME=/Users/someone",
            "env = PATH=/opt/bin:/usr/bin",
        ])
    }

    func testCommandWithSpacesIsShellQuoted() {
        let url = URL(fileURLWithPath: "/Users/some one/it's/tt")
        XCTAssertEqual(Ghostty.ConfigOverrides.commandValue(for: url), #"shell:exec '/Users/some one/it'\''s/tt'"#)
    }

    func testMultilineEnvironmentValuesAreDropped() {
        let o = Ghostty.ConfigOverrides(command: URL(fileURLWithPath: "/tt"), environment: ["X": "a\ncommand = /bin/sh"])
        XCTAssertFalse(o.lines.contains { $0.hasPrefix("env = X") })
        XCTAssertEqual(o.lines.filter { $0.hasPrefix("command") }.count, 1)
    }

    func testDefaultsApplyWithoutAUserFile() {
        let config = Ghostty.Config(userConfig: dir.appendingPathComponent("missing.conf"), overrides: overrides)
        XCTAssertTrue(config.loaded)
        XCTAssertEqual(config.diagnostics, [])
        XCTAssertEqual(config.bool("wait-after-command"), false)
        XCTAssertEqual(config.string("confirm-close-surface"), "false")
    }

    func testUserFileLoadsAfterDefaultsAndBeforeOverrides() throws {
        let user = try writeUserConfig("""
            wait-after-command = true
            confirm-close-surface = always
            font-size = 17
            """)
        let config = Ghostty.Config(userConfig: user, overrides: overrides)
        XCTAssertTrue(config.loaded)
        XCTAssertEqual(config.diagnostics, [])
        // The user file beats the defaults...
        XCTAssertEqual(config.get("font-size", as: Float.self, initial: 0), 17)
        // ...and the forced overrides beat the user file.
        XCTAssertEqual(config.bool("wait-after-command"), false)
        XCTAssertEqual(config.string("confirm-close-surface"), "false")
    }

    func testUserConfigDiagnosticsAreReported() throws {
        let user = try writeUserConfig("not-a-real-ghostty-key = 1\n")
        let config = Ghostty.Config(userConfig: user, overrides: overrides)
        XCTAssertTrue(config.loaded)
        XCTAssertFalse(config.diagnostics.isEmpty)
    }

    func testAppCreatesGhosttyAppObject() {
        let app = Ghostty.App { Ghostty.Config(userConfig: nil, overrides: self.overrides) }
        XCTAssertEqual(app.readiness, .ready)
        XCTAssertNotNil(app.app)
    }

    func testTabSplitQuickTerminalAndNewWindowActionsAreIgnored() {
        let ignored = [
            GHOSTTY_ACTION_NEW_WINDOW,
            GHOSTTY_ACTION_NEW_TAB,
            GHOSTTY_ACTION_CLOSE_TAB,
            GHOSTTY_ACTION_GOTO_TAB,
            GHOSTTY_ACTION_MOVE_TAB,
            GHOSTTY_ACTION_NEW_SPLIT,
            GHOSTTY_ACTION_GOTO_SPLIT,
            GHOSTTY_ACTION_RESIZE_SPLIT,
            GHOSTTY_ACTION_EQUALIZE_SPLITS,
            GHOSTTY_ACTION_TOGGLE_SPLIT_ZOOM,
            GHOSTTY_ACTION_TOGGLE_QUICK_TERMINAL,
            GHOSTTY_ACTION_TOGGLE_WINDOW_DECORATIONS,
        ]
        for tag in ignored {
            XCTAssertEqual(Ghostty.disposition(of: tag), .ignored, "action \(tag.rawValue)")
        }
        XCTAssertEqual(Ghostty.disposition(of: GHOSTTY_ACTION_QUIT), .app)
        XCTAssertEqual(Ghostty.disposition(of: GHOSTTY_ACTION_SET_TITLE), .surface)
    }
}
