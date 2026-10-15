import AppKit
import GhosttyKit
import XCTest
@testable import tt

final class ExitPolicyTests: XCTestCase {
    func testShellStatusSplitsExitCodesFromSignals() {
        XCTAssertEqual(TTExitStatus(shellStatus: 0), .exited(0))
        XCTAssertEqual(TTExitStatus(shellStatus: 2), .exited(2))
        XCTAssertEqual(TTExitStatus(shellStatus: 128), .exited(128))
        XCTAssertEqual(TTExitStatus(shellStatus: 137), .signaled(9))
        XCTAssertEqual(TTExitStatus(shellStatus: 129), .signaled(1))
        XCTAssertEqual(TTExitStatus(shellStatus: nil), .unknown)
        XCTAssertEqual(TTExitStatus(shellStatus: 300), .unknown)
        XCTAssertEqual(TTExitStatus(shellStatus: -1), .unknown)
    }

    func testStatusFileIsReadTrimmedAndMissingIsUnknown() throws {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("tt-status-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: url) }
        XCTAssertEqual(TTExitStatus(contentsOf: url), .unknown)
        try "3\n".write(to: url, atomically: true, encoding: .utf8)
        XCTAssertEqual(TTExitStatus(contentsOf: url), .exited(3))
        try "garbage".write(to: url, atomically: true, encoding: .utf8)
        XCTAssertEqual(TTExitStatus(contentsOf: url), .unknown)
    }

    func testCleanExitFollowsTheBehaviorAndEveryFailureStaysOnScreen() {
        XCTAssertEqual(TTExitAction.action(for: .exited(0), behavior: .relaunchOnShow), .hideAndRelaunchOnShow)
        XCTAssertEqual(TTExitAction.action(for: .exited(0), behavior: .quitApp), .quitApp)
        for behavior in TTExitBehavior.allCases {
            XCTAssertEqual(TTExitAction.action(for: .exited(1), behavior: behavior),
                           .showFailure("tt exited with status 1."))
            XCTAssertEqual(TTExitAction.action(for: .unknown, behavior: behavior),
                           .showFailure(TTExitStatus.unknown.summary))
        }
        guard case .showFailure(let summary) = TTExitAction.action(for: .signaled(9), behavior: .quitApp) else {
            return XCTFail("a signal is a failure")
        }
        XCTAssertTrue(summary.hasPrefix("tt was terminated by signal 9 (Killed"), summary)
    }

    func testBehaviorDefaultsToRelaunchAndRoundTrips() throws {
        let suite = "tt-tests-\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
        defer { defaults.removePersistentDomain(forName: suite) }
        XCTAssertEqual(TTExitBehavior.load(from: defaults), .relaunchOnShow)
        defaults.set("nonsense", forKey: TTExitBehavior.defaultsKey)
        XCTAssertEqual(TTExitBehavior.load(from: defaults), .relaunchOnShow)
        TTExitBehavior.quitApp.save(to: defaults)
        XCTAssertEqual(TTExitBehavior.load(from: defaults), .quitApp)
    }
}

final class MainMenuTests: XCTestCase {
    private func titles(_ menu: NSMenu?) -> [String] {
        (menu?.items ?? []).filter { !$0.isSeparatorItem }.map(\.title)
    }

    private func submenu(_ main: NSMenu, _ title: String) -> NSMenu? {
        main.items.first { $0.submenu?.title == title }?.submenu
    }

    func testAppMenuAndTheStandardEditMenu() {
        let main = MainMenu.make()
        let app = titles(main.items.first?.submenu)
        for title in ["About tt", "Settings…", "Relaunch tt", "Hide tt", "Quit tt"] {
            XCTAssertTrue(app.contains(title), "\(title) missing from \(app)")
        }
        XCTAssertEqual(titles(submenu(main, "Edit")), ["Undo", "Redo", "Cut", "Copy", "Paste", "Select All", "Find…"])
        XCTAssertEqual(main.items.compactMap { $0.submenu?.title }, ["", "Edit", "View", "Window", "Help"])
    }

    /// The zoom keys Ghostty already answers are named, so they can be found.
    func testViewMenuNamesTheZoomKeys() throws {
        let view = try XCTUnwrap(submenu(MainMenu.make(), "View"))
        XCTAssertEqual(titles(view), ["Actual Size", "Zoom In", "Zoom Out", "Enter Full Screen"])
        let items = view.items.filter { !$0.isSeparatorItem }
        XCTAssertEqual(items.map(\.keyEquivalent), ["0", "=", "-", "f"])
        XCTAssertEqual(items.map(\.action), [
            #selector(Ghostty.SurfaceView.resetZoom(_:)),
            #selector(Ghostty.SurfaceView.zoomIn(_:)),
            #selector(Ghostty.SurfaceView.zoomOut(_:)),
            #selector(NSWindow.toggleFullScreen(_:)),
        ])
        XCTAssertEqual(items.last?.keyEquivalentModifierMask, [.control, .command])
    }

    func testWindowMenuHasTheStandardItems() throws {
        let window = try XCTUnwrap(submenu(MainMenu.make(), "Window"))
        XCTAssertEqual(titles(window), ["Minimize", "Zoom", "Close", "Bring All to Front"])
        XCTAssertEqual(window.item(withTitle: "Zoom")?.action, #selector(NSWindow.performZoom(_:)))
        XCTAssertEqual(window.item(withTitle: "Bring All to Front")?.action,
                       #selector(NSApplication.arrangeInFront(_:)))
    }

    /// Quick Capture shows the live global shortcut, and none once it is off.
    func testQuickCaptureShowsItsGlobalShortcut() throws {
        let combo = try XCTUnwrap(HotKeyCombo(string: "ctrl+opt+cmd+a"))
        let main = MainMenu.make(captureHotKey: combo)
        let capture = try XCTUnwrap(main.items.first?.submenu?.item(withTitle: "Quick Capture…"))
        XCTAssertEqual(capture.keyEquivalent, "a")
        XCTAssertEqual(capture.keyEquivalentModifierMask, [.control, .option, .command])

        MainMenu.showCaptureHotKey(try XCTUnwrap(HotKeyCombo(string: "shift+cmd+f5")), in: main)
        XCTAssertEqual(capture.keyEquivalent, String(UnicodeScalar(UInt32(NSF5FunctionKey))!))
        XCTAssertEqual(capture.keyEquivalentModifierMask, [.shift, .command])

        MainMenu.showCaptureHotKey(nil, in: main)
        XCTAssertEqual(capture.keyEquivalent, "")
        XCTAssertEqual(MainMenu.make().items.first?.submenu?.item(withTitle: "Quick Capture…")?.keyEquivalent, "")
    }

    func testHelpMenuOpensTheGuideAndTheIssues() throws {
        let help = try XCTUnwrap(submenu(MainMenu.make(), "Help"))
        XCTAssertEqual(titles(help), ["tt Guide", "Report an Issue…"])
        XCTAssertEqual(help.item(withTitle: "Report an Issue…")?.action, MainMenu.reportIssue)
        XCTAssertEqual(MainMenu.guideURL.absoluteString, "https://github.com/sglavoie/tt/blob/main/docs/guide/README.md")
        XCTAssertEqual(MainMenu.issuesURL.absoluteString, "https://github.com/sglavoie/tt/issues")
    }

    /// Undo, Redo, and Cut are text-field edits: the surface keeps their keys,
    /// while Copy, Paste, and Select All stay the menu's.
    func testTerminalEditingRoutesThroughTheResponderChain() throws {
        let edit = try XCTUnwrap(submenu(MainMenu.make(), "Edit"))
        let undo = try XCTUnwrap(edit.item(withTitle: "Undo"))
        let redo = try XCTUnwrap(edit.item(withTitle: "Redo"))
        let cut = try XCTUnwrap(edit.item(withTitle: "Cut"))
        XCTAssertNil(undo.target)
        XCTAssertEqual([undo.action, redo.action, cut.action], [Selector(("undo:")), Selector(("redo:")), #selector(NSText.cut(_:))])
        let main = MainMenu.make()
        XCTAssertFalse(MainMenu.isTextOnlyEdit(MainMenu.item(in: main, key: "z", modifiers: .command)))
        XCTAssertFalse(MainMenu.isTextOnlyEdit(MainMenu.item(in: main, key: "z", modifiers: [.shift, .command])))
        XCTAssertTrue(MainMenu.isTextOnlyEdit(MainMenu.item(in: main, key: "x", modifiers: .command)))
        for key in ["c", "v", "a", "q"] {
            let item = MainMenu.item(in: main, key: key, modifiers: .command)
            XCTAssertNotNil(item, key)
            XCTAssertFalse(MainMenu.isTextOnlyEdit(item), key)
        }

        // Fields keep their native editing; the terminal answers Undo, Redo, Find.
        for action in MainMenu.terminalEditActions {
            XCTAssertTrue(Ghostty.SurfaceView.instancesRespond(to: action))
        }
        XCTAssertTrue(NSTextView.instancesRespond(to: MainMenu.cut))
        XCTAssertTrue(NSWindow.instancesRespond(to: MainMenu.undo))
        XCTAssertTrue(NSWindow.instancesRespond(to: MainMenu.redo))
        for action in MainMenu.textOnlyEditActions {
            XCTAssertFalse(Ghostty.SurfaceView.instancesRespond(to: action), NSStringFromSelector(action))
        }
    }

    func testAboutCreditsNameTheGhosttyVersionAndNotice() {
        XCTAssertFalse(Ghostty.versionDescription.hasPrefix("unknown"), Ghostty.versionDescription)
        let credits = MainMenu.aboutCredits(
            ttVersion: nil, ghosttyVersion: Ghostty.versionDescription, notice: "MIT License"
        ).string
        XCTAssertTrue(credits.contains("Ghostty \(Ghostty.versionDescription)"))
        XCTAssertTrue(credits.contains("MIT License"))
        XCTAssertTrue(credits.hasPrefix("tt: version unknown\n"), credits)
    }

    func testAboutCreditsLeadWithTheRunningTTVersion() {
        let line = "tt version 1.2.3 (commit: abc, built: 2026-10-14T00:00:00Z)"
        let credits = MainMenu.aboutCredits(ttVersion: line, ghosttyVersion: "1.0", notice: nil).string
        XCTAssertTrue(credits.hasPrefix("tt: \(line)\nTerminal engine: Ghostty 1.0"), credits)
    }

    func testTheBundleShipsTheLauncherAndNotice() throws {
        let launcher = try XCTUnwrap(Bundle.main.url(forResource: "tt-launch", withExtension: nil))
        XCTAssertTrue(FileManager.default.isExecutableFile(atPath: launcher.path))
        let notice = try XCTUnwrap(Bundle.main.url(forResource: "NOTICE-ghostty", withExtension: nil))
        XCTAssertTrue(try String(contentsOf: notice, encoding: .utf8).contains("MIT License"))
    }
}

final class LaunchErrorTests: XCTestCase {
    private func buttons(in view: NSView) -> [NSButton] {
        (view as? NSButton).map { [$0] } ?? view.subviews.flatMap(buttons(in:))
    }

    func testTheWindowOffersSettingsTryAgainAndQuit() throws {
        var calls: [String] = []
        let window = LaunchErrorWindow.make(
            title: "tt could not be found.",
            message: "Looked in: …",
            openSettings: { calls.append("settings") },
            tryAgain: { calls.append("retry") }
        )
        let all = buttons(in: try XCTUnwrap(window.contentView))
        XCTAssertEqual(all.map(\.title), ["Open Settings…", "Try Again", "Quit"])
        XCTAssertEqual(all.first { $0.keyEquivalent == "\r" }?.title, "Try Again")
        all[0].performClick(nil)
        all[1].performClick(nil)
        XCTAssertEqual(calls, ["settings", "retry"])
        XCTAssertEqual(all[2].action, #selector(NSApplication.terminate(_:)))
    }

    /// A failure a retry cannot fix offers none, and Return opens Settings:
    /// it never quits.
    func testWithoutARetryReturnOpensSettings() throws {
        let window = LaunchErrorWindow.make(title: "The terminal could not start.", message: "", openSettings: {})
        let all = buttons(in: try XCTUnwrap(window.contentView))
        XCTAssertEqual(all.map(\.title), ["Open Settings…", "Quit"])
        XCTAssertEqual(all.filter { $0.keyEquivalent == "\r" }.map(\.title), ["Open Settings…"])
    }

    /// The launch errors speak to the user: the config file is named, and
    /// what Ghostty said about it listed, rather than "see the log".
    func testTerminalFailuresNameTheConfigAndItsProblems() {
        let none = AppDelegate.terminalStartFailureMessage(configPath: "~/.config/tt-app/ghostty.conf", diagnostics: [])
        XCTAssertTrue(none.contains("~/.config/tt-app/ghostty.conf"), none)
        XCTAssertFalse(none.lowercased().contains("log"), none)
        let one = AppDelegate.terminalStartFailureMessage(
            configPath: "~/c.conf", diagnostics: ["unknown field: fnot-size"]
        )
        XCTAssertTrue(one.contains("has 1 problem; fix it"), one)
        XCTAssertTrue(one.hasSuffix("\n\n• unknown field: fnot-size"), one)
        let two = AppDelegate.terminalStartFailureMessage(configPath: "~/c.conf", diagnostics: ["a", "b"])
        XCTAssertTrue(two.contains("has 2 problems; fix them"), two)
        XCTAssertTrue(two.hasSuffix("• a\n• b"), two)
        XCTAssertFalse(AppDelegate.engineInitFailureMessage.contains("ghostty_init"))
    }

    /// With no terminal, a Dock click or switching to tt.app brings the
    /// launch error back rather than nothing.
    /// With no tt to ask, Quit ends the app at once; tt, if any, takes the
    /// hangup as before.
    func testQuitWithNoControlSocketTerminatesNow() {
        let delegate = AppDelegate()
        XCTAssertEqual(delegate.applicationShouldTerminate(NSApp), .terminateNow)
    }

    /// A busy tt brings the question to the user, and Cancel keeps the app.
    func testABusyQuitAsksBeforeQuittingAnyway() {
        let delegate = AppDelegate()
        var asked = 0
        delegate.confirmQuitAnyway = { reason in
            XCTAssertEqual(reason, "tt is busy: finish what is open in tt first")
            asked += 1; return false
        }
        delegate.finishQuit(.busy("tt is busy: finish what is open in tt first"))
        XCTAssertEqual(asked, 1)
    }

    func testReopeningWithoutATerminalShowsTheLaunchError() throws {
        let delegate = AppDelegate()
        XCTAssertNil(delegate.terminal)
        delegate.showLaunchError(title: "tt could not be found.", message: "", tryAgain: {})
        let window = try XCTUnwrap(delegate.launchErrorWindow)
        defer { window.orderOut(nil) }
        XCTAssertTrue(window.isVisible)

        window.orderOut(nil)
        XCTAssertFalse(delegate.applicationShouldHandleReopen(NSApp, hasVisibleWindows: false))
        XCTAssertTrue(window.isVisible)

        window.orderOut(nil)
        delegate.applicationDidBecomeActive(Notification(name: NSApplication.didBecomeActiveNotification))
        XCTAssertTrue(window.isVisible)

        delegate.showLaunchError(title: "tt could not be found.", message: "again")
        XCTAssertFalse(window.isVisible, "a second error replaces the first")
        delegate.launchErrorWindow?.orderOut(nil)
    }
}

/// A real surface running a stand-in for tt through the bundled launcher.
final class TerminalWindowControllerTests: XCTestCase {
    private var dir: URL!
    private var ghostty: Ghostty.App!
    private var controller: TerminalWindowController!

    override func setUpWithError() throws {
        dir = FileManager.default.temporaryDirectory.appendingPathComponent("tt-lifecycle-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        controller?.teardown()
        controller?.window.orderOut(nil)
        controller = nil
        ghostty = nil
        try? FileManager.default.removeItem(at: dir)
    }

    /// The stand-in prints READY and its pid, then waits for a line on the
    /// pty: `exit N` exits with N, anything else is echoed back.
    private func writeFake(named name: String = "fake-tt", banner: String = "READY") throws -> URL {
        let script = dir.appendingPathComponent(name)
        try """
        #!/bin/sh
        printf '\(banner):%s:\\n' "$$"
        while read -r line; do
          case "$line" in
            exit*) exit "${line#exit }" ;;
            bg*) printf '\\033]11;%s\\033\\\\' "${line#bg }" ;;
            *) printf 'GOT:%s:\\n' "$line" ;;
          esac
        done
        """.write(to: script, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: script.path)
        return script
    }

    private func start(hidesApp: Bool = false) throws {
        let script = try writeFake()

        let launcher = try XCTUnwrap(Bundle.main.url(forResource: "tt-launch", withExtension: nil))
        let statusFile = dir.appendingPathComponent("status")
        let overrides = Ghostty.ConfigOverrides(
            command: launcher, arguments: [statusFile.path, script.path], waitAfterCommand: true
        )
        ghostty = Ghostty.App { Ghostty.Config(userConfig: nil, overrides: overrides) }
        let delegate = Delegate()
        ghostty.delegate = delegate
        self.delegate = delegate
        controller = TerminalWindowController(
            ghostty: ghostty, statusFile: statusFile, exitBehavior: { .relaunchOnShow },
            frameAutosaveName: nil, hidesApp: hidesApp
        )
        delegate.controller = controller
        controller.window.setFrameOrigin(NSPoint(x: -10_000, y: -10_000))
        controller.show()
        try waitForScreen { $0.contains("READY:") }
    }

    private var delegate: Delegate?

    /// Stands in for AppDelegate's routing of the two exit signals.
    private final class Delegate: GhosttyAppDelegate {
        weak var controller: TerminalWindowController?

        func ghosttyApp(_ app: Ghostty.App, perform action: ghostty_action_s, target: ghostty_target_s) -> Bool {
            switch action.tag {
            case GHOSTTY_ACTION_SHOW_CHILD_EXITED:
                controller?.childExited(surface: target.target.surface)
            case GHOSTTY_ACTION_COLOR_CHANGE:
                let change = action.action.color_change
                DispatchQueue.main.async { [weak self] in self?.controller?.colorChanged(change) }
            case GHOSTTY_ACTION_CELL_SIZE:
                DispatchQueue.main.async { [weak self] in self?.controller?.cellSizeChanged() }
            default:
                return false
            }
            return true
        }

        func ghosttyApp(_ app: Ghostty.App, closeSurface host: GhosttySurfaceHost?, needsConfirm: Bool) {
            controller?.closeRequested(host: host)
        }
    }

    private func screenText() -> String {
        guard let surface = controller.surfaceView?.surface else { return "" }
        var text = ghostty_text_s()
        let selection = ghostty_selection_s(
            top_left: ghostty_point_s(tag: GHOSTTY_POINT_SCREEN, coord: GHOSTTY_POINT_COORD_TOP_LEFT, x: 0, y: 0),
            bottom_right: ghostty_point_s(tag: GHOSTTY_POINT_SCREEN, coord: GHOSTTY_POINT_COORD_BOTTOM_RIGHT, x: 0, y: 0),
            rectangle: false
        )
        guard ghostty_surface_read_text(surface, selection, &text) else { return "" }
        defer { ghostty_surface_free_text(surface, &text) }
        return String(cString: text.text)
    }

    private struct Timeout: Error {}

    private func wait(timeout: TimeInterval = 10, _ what: String, until done: () -> Bool) throws {
        let deadline = Date().addingTimeInterval(timeout)
        while !done() {
            if Date() > deadline {
                XCTFail("timed out waiting for \(what); the screen reads: \(screenText())")
                throw Timeout()
            }
            ghostty.appTick()
            RunLoop.current.run(until: Date().addingTimeInterval(0.02))
        }
    }

    private func waitForScreen(_ done: (String) -> Bool) throws {
        try wait("the screen to match") { done(screenText()) }
    }

    private func type(_ line: String) {
        guard let surface = controller.surfaceView?.surface else { return }
        let text = line + "\r"
        ghostty_surface_text(surface, text, UInt(text.utf8.count))
    }

    private func fakePID(after banner: String = "READY:") throws -> pid_t {
        let screen = screenText()
        let parts = try XCTUnwrap(screen.components(separatedBy: banner).last?.split(separator: ":").first)
        return try XCTUnwrap(pid_t(parts))
    }

    /// tt announces its theme background with OSC 11; the title bar follows,
    /// with title text that reads on it.
    func testTheWindowTakesTheBackgroundTheProgramSets() throws {
        try start()
        XCTAssertTrue(controller.window.titlebarAppearsTransparent)

        type("bg #f6f6f0")
        try wait("a light background") { controller.window.backgroundColor.usingColorSpace(.sRGB)?.redComponent ?? 0 > 0.9 }
        XCTAssertEqual(controller.window.appearance?.name, .aqua)

        type("bg #1a1b26")
        try wait("a dark background") { controller.window.backgroundColor.usingColorSpace(.sRGB)?.redComponent ?? 1 < 0.2 }
        XCTAssertEqual(controller.window.appearance?.name, .darkAqua)
    }

    func testHideAndShowKeepTheSameSession() throws {
        try start()
        let surface = controller.surfaceView
        XCTAssertFalse(controller.windowShouldClose(controller.window))
        XCTAssertFalse(controller.window.isVisible)
        type("still here")
        controller.show()
        XCTAssertTrue(controller.window.isVisible)
        XCTAssertTrue(controller.surfaceView === surface)
        try waitForScreen { $0.contains("GOT:still here:") }
    }

    /// Cmd+W is Ghostty's close_surface binding; its callback must hide, not
    /// read as an exit.
    func testCloseBindingHidesWithoutEndingTheSession() throws {
        try start()
        let surfaceView = try XCTUnwrap(controller.surfaceView)
        let action = "close_surface"
        XCTAssertTrue(ghostty_surface_binding_action(surfaceView.surface, action, UInt(action.utf8.count)))
        try wait("the close binding to hide the window") { !controller.window.isVisible }
        XCTAssertTrue(controller.surfaceView === surfaceView)
        XCTAssertNil(controller.exitAction)
        controller.show()
        type("after close")
        try waitForScreen { $0.contains("GOT:after close:") }
    }

    func testCleanExitHidesAndTheNextShowStartsFresh() throws {
        try start()
        let firstPID = try fakePID()
        type("exit 0")
        try wait("the clean exit to hide the window") { controller.surfaceView == nil }
        XCTAssertFalse(controller.window.isVisible)
        controller.show()
        XCTAssertNotNil(controller.surfaceView)
        try waitForScreen { $0.contains("READY:") }
        XCTAssertNotEqual(try fakePID(), firstPID)
    }

    func testNonZeroExitKeepsTheOutputUnderARelaunchBanner() throws {
        try start()
        type("before the crash")
        try waitForScreen { $0.contains("GOT:before the crash:") }
        type("exit 3")
        try wait("the failure banner") { controller.isShowingFailure }
        XCTAssertEqual(controller.exitAction, .showFailure("tt exited with status 3."))
        XCTAssertTrue(controller.window.isVisible)
        XCTAssertTrue(screenText().contains("GOT:before the crash:"))

        // A key on the dead surface asks Ghostty to close it; the banner stays.
        controller.closeRequested(host: controller.surfaceView)
        XCTAssertTrue(controller.isShowingFailure)

        let surfaceBefore = controller.surfaceView
        let returnKey = try XCTUnwrap(NSEvent.keyEvent(
            with: .keyDown, location: .zero, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
            windowNumber: controller.window.windowNumber, context: nil,
            characters: "\r", charactersIgnoringModifiers: "\r", isARepeat: false, keyCode: 36
        ))
        NSApp.sendEvent(returnKey)
        XCTAssertFalse(controller.isShowingFailure)
        XCTAssertFalse(controller.surfaceView === surfaceBefore)
        XCTAssertNil(controller.exitAction)
        try waitForScreen { $0.contains("READY:") && !$0.contains("before the crash") }
    }

    /// Each session asks again which tt runs, off the main thread, so a
    /// relaunch runs one installed since; a tt that can no longer be found
    /// leaves the last one running.
    func testRelaunchRunsTheBinaryResolvedForThatSession() throws {
        let old = try writeFake(named: "old-tt", banner: "OLD")
        let new = try writeFake(named: "new-tt", banner: "NEW")
        let launcher = try XCTUnwrap(Bundle.main.url(forResource: "tt-launch", withExtension: nil))
        let statusFile = dir.appendingPathComponent("status")

        var installed: URL? = old
        var askedOnMain = false
        let binaries = TTSessionBinaryStore()
        binaries.resolve = { _ in
            askedOnMain = askedOnMain || Thread.isMainThread
            guard let installed else { return .failure(.init(invalidOverride: nil, tried: [])) }
            return .success(.init(url: installed, source: .localBin))
        }
        binaries.supportedFlags = { path in
            askedOnMain = askedOnMain || Thread.isMainThread
            return path == new.path ? [TTControlLaunch.flag] : []
        }

        ghostty = Ghostty.App {
            var overrides = Ghostty.ConfigOverrides(
                command: launcher, arguments: [statusFile.path], waitAfterCommand: true
            )
            if let binary = binaries.current {
                overrides.arguments.append(binary.path)
            }
            return Ghostty.Config(userConfig: nil, overrides: overrides)
        }
        let delegate = Delegate()
        ghostty.delegate = delegate
        self.delegate = delegate
        controller = TerminalWindowController(
            ghostty: ghostty, statusFile: statusFile, exitBehavior: { .relaunchOnShow },
            frameAutosaveName: nil, hidesApp: false
        )
        delegate.controller = controller
        controller.window.setFrameOrigin(NSPoint(x: -10_000, y: -10_000))
        var failures = 0
        controller.onWillStartSession = { [weak self] ready in
            binaries.refresh(searchPath: "") { notFound in
                XCTAssertTrue(Thread.isMainThread)
                failures += notFound == nil ? 0 : 1
                self?.ghostty.reloadConfig()
                ready()
            }
        }

        controller.show()
        XCTAssertNil(controller.surfaceView, "the session waits for the answer")
        XCTAssertFalse(controller.window.isVisible)
        try waitForScreen { $0.contains("OLD:") }
        XCTAssertTrue(controller.window.isVisible)
        XCTAssertEqual(binaries.current?.controlSupported, false)

        installed = new
        controller.relaunch()
        try waitForScreen { $0.contains("NEW:") }
        XCTAssertEqual(binaries.current?.path, new.path)
        XCTAssertEqual(binaries.current?.controlSupported, true)
        let newPID = try fakePID(after: "NEW:")

        installed = nil
        controller.relaunch()
        try waitForScreen { $0.contains("NEW:") }
        XCTAssertNotEqual(try fakePID(after: "NEW:"), newPID)
        XCTAssertEqual(binaries.current?.path, new.path)
        XCTAssertEqual(failures, 1)
        XCTAssertFalse(askedOnMain)
    }

    /// A relaunch asked for while a start is still waiting replaces it: one
    /// session begins, not two.
    func testASupersededStartBeginsNothing() throws {
        try start()
        var waiting: [() -> Void] = []
        controller.onWillStartSession = { waiting.append($0) }
        controller.relaunch()
        controller.relaunch()
        XCTAssertEqual(waiting.count, 2)
        waiting[0]()
        XCTAssertNil(controller.surfaceView)
        waiting[1]()
        let surface = try XCTUnwrap(controller.surfaceView)
        waiting[1]()
        XCTAssertTrue(controller.surfaceView === surface)
        try waitForScreen { $0.contains("READY:") }
    }

    /// Closing the window while a relaunch is still being prepared stands:
    /// the session begins hidden, and the next show brings it forward.
    func testHideWhileSessionPreparationIsPendingDoesNotPresentOnReady() throws {
        try start()
        var waiting: [() -> Void] = []
        controller.onWillStartSession = { waiting.append($0) }
        controller.relaunch()
        XCTAssertEqual(waiting.count, 1)
        XCTAssertFalse(controller.windowShouldClose(controller.window))
        XCTAssertFalse(controller.window.isVisible)

        waiting[0]()
        let surface = try XCTUnwrap(controller.surfaceView)
        XCTAssertFalse(controller.window.isVisible)
        try waitForScreen { $0.contains("READY:") }
        XCTAssertFalse(controller.window.isVisible)

        controller.show()
        XCTAssertTrue(controller.window.isVisible)
        XCTAssertTrue(controller.surfaceView === surface)
        XCTAssertEqual(waiting.count, 1)
        type("after the hide")
        try waitForScreen { $0.contains("GOT:after the hide:") }

        // A show asked for again while the next start waits wins over the hide.
        controller.relaunch()
        controller.hide()
        controller.show()
        XCTAssertFalse(controller.window.isVisible)
        waiting[1]()
        XCTAssertTrue(controller.window.isVisible)
    }

    /// A crash while the window is hidden leaves it hidden: the app is told
    /// once, and the banner is waiting at the next show.
    func testAFailureWhileHiddenStaysHiddenUntilTheNextShow() throws {
        try start()
        var announced: [String] = []
        controller.onHiddenFailure = { announced.append($0) }
        XCTAssertFalse(controller.windowShouldClose(controller.window))
        XCTAssertFalse(controller.window.isVisible)

        type("exit 3")
        try wait("the failure banner") { controller.isShowingFailure }
        XCTAssertFalse(controller.window.isVisible)
        XCTAssertEqual(announced, ["tt exited with status 3."])
        let banner = try XCTUnwrap(controller.failureBanner)
        XCTAssertEqual(banner.accessibilityLabel(), "tt stopped")
        XCTAssertEqual(banner.accessibilityHelp(), "tt exited with status 3.")

        controller.show()
        XCTAssertTrue(controller.window.isVisible)
        XCTAssertTrue(controller.isShowingFailure)
        XCTAssertEqual(announced.count, 1)
    }

    /// A failure with the window on screen shows the banner and says nothing else.
    func testAFailureWhileShownAnnouncesNothing() throws {
        try start()
        var announced: [String] = []
        controller.onHiddenFailure = { announced.append($0) }
        type("exit 3")
        try wait("the failure banner") { controller.isShowingFailure }
        XCTAssertTrue(controller.window.isVisible)
        XCTAssertEqual(announced, [])
    }

    /// Hiding the terminal with Settings open leaves the app shown, so the
    /// Settings window stays where it was.
    func testHidingWithAnotherTTWindowOpenKeepsTheAppShown() throws {
        try start(hidesApp: true)
        var hides = 0
        controller.hideApp = { hides += 1 }

        let settings = NSWindow(
            contentRect: NSRect(x: -10_000, y: -10_000, width: 200, height: 100),
            styleMask: [.titled, .closable], backing: .buffered, defer: false
        )
        settings.isReleasedWhenClosed = false
        settings.orderFront(nil)
        defer { settings.orderOut(nil) }

        controller.hide()
        XCTAssertFalse(controller.window.isVisible)
        XCTAssertEqual(hides, 0)

        settings.orderOut(nil)
        controller.show()
        controller.hide()
        XCTAssertFalse(controller.window.isVisible)
        XCTAssertEqual(hides, 1)
    }

    /// The window resizes in whole cells and never below 80×24 of them, and
    /// follows the cell size when the font zooms.
    func testTheWindowSizesInCellsWithAnEightyByTwentyFourFloor() throws {
        try start()
        let surfaceView = try XCTUnwrap(controller.surfaceView)
        let cell = surfaceView.cellSize
        XCTAssertGreaterThan(cell.width, 0)
        XCTAssertEqual(controller.window.contentResizeIncrements, cell)
        XCTAssertEqual(controller.window.contentMinSize, NSSize(width: cell.width * 80, height: cell.height * 24))
        // Summoned onto the Space in use, and still able to go full screen.
        XCTAssertTrue(controller.window.collectionBehavior.contains(.moveToActiveSpace))
        XCTAssertTrue(controller.window.collectionBehavior.contains(.fullScreenPrimary))
        XCTAssertFalse(controller.restoresFullScreen)

        let action = "increase_font_size:4"
        XCTAssertTrue(ghostty_surface_binding_action(surfaceView.surface, action, UInt(action.utf8.count)))
        try wait("the cell size to grow") { controller.window.contentResizeIncrements.width > cell.width }
        let bigger = surfaceView.cellSize
        // A zoomed cell can be a fractional point (45 pixels at 2x), and AppKit
        // keeps resize increments in whole points.
        XCTAssertEqual(controller.window.contentResizeIncrements.width, bigger.width, accuracy: 1)
        XCTAssertEqual(controller.window.contentResizeIncrements.height, bigger.height, accuracy: 1)
        XCTAssertEqual(controller.window.contentMinSize, TerminalWindowController.minimumContentSize(cell: bigger))
    }

    func testFullScreenIntentSurvivesRepeatedHideAndRapidShow() throws {
        try start()
        var fullScreen = true
        var toggles = 0
        controller.isFullScreen = { fullScreen }
        controller.toggleFullScreen = { toggles += 1 }
        let exited = Notification(name: NSWindow.didExitFullScreenNotification)
        controller.hide()
        controller.hide()
        XCTAssertEqual(toggles, 1, "hiding twice must not reverse the transition")
        fullScreen = false
        controller.windowDidExitFullScreen(exited)
        XCTAssertFalse(controller.window.isVisible)
        controller.hide()
        XCTAssertTrue(controller.restoresFullScreen)
        controller.show()
        XCTAssertEqual(toggles, 2)
        fullScreen = true
        controller.windowDidEnterFullScreen(Notification(name: NSWindow.didEnterFullScreenNotification))
        controller.hide()
        controller.show()
        fullScreen = false
        controller.windowDidExitFullScreen(exited)
        XCTAssertTrue(controller.window.isVisible)
        XCTAssertEqual(toggles, 4, "show during exit returns to full screen after exit")
    }

    func testHiddenDuringFullScreenEntryWaitsForBothTransitions() throws {
        try start()
        var fullScreen = false
        var toggles = 0
        controller.isFullScreen = { fullScreen }
        controller.toggleFullScreen = { toggles += 1 }
        controller.windowWillEnterFullScreen(Notification(name: NSWindow.willEnterFullScreenNotification))
        controller.hide()
        XCTAssertEqual(toggles, 0)
        fullScreen = true
        controller.windowDidEnterFullScreen(Notification(name: NSWindow.didEnterFullScreenNotification))
        XCTAssertEqual(toggles, 1)
        fullScreen = false
        controller.windowDidExitFullScreen(Notification(name: NSWindow.didExitFullScreenNotification))
        XCTAssertFalse(controller.window.isVisible)
        XCTAssertTrue(controller.restoresFullScreen)
    }

    func testMissingDisplayFrameFitsTheRemainingScreen() {
        let visible = NSRect(x: -1440, y: 24, width: 1440, height: 876)
        let fitted = TerminalWindowController.frame(NSRect(x: 3000, y: -500, width: 1800, height: 1100), fitting: visible)
        XCTAssertEqual(fitted, visible)
        let small = TerminalWindowController.frame(NSRect(x: 3000, y: -500, width: 800, height: 600), fitting: visible)
        XCTAssertTrue(visible.contains(small))
        XCTAssertEqual(small.size, NSSize(width: 800, height: 600))
    }

    func testNavigationNoticeKeepsTheSessionAndFocus() throws {
        try start()
        let surface = controller.surfaceView
        let responder = controller.window.firstResponder
        controller.showNotice("Close search in tt, then try opening Today again.")
        XCTAssertEqual(controller.notice?.stringValue, "Close search in tt, then try opening Today again.")
        XCTAssertTrue(controller.surfaceView === surface)
        XCTAssertTrue(controller.window.firstResponder === responder)
    }

    func testNativeEditMenuUsesTUIAvailabilityAndLabels() throws {
        try start()
        let surface = try XCTUnwrap(controller.surfaceView)
        var edits: [String] = []
        controller.onDesktopEdit = { edits.append($0) }
        surface.onDesktopEdit = controller.onDesktopEdit
        surface.desktopEditing = TTEditing(undo: "Typing", redo: "", search: false)
        let undo = NSMenuItem(title: "Undo", action: MainMenu.undo, keyEquivalent: "z")
        let redo = NSMenuItem(title: "Redo", action: MainMenu.redo, keyEquivalent: "z")
        let find = NSMenuItem(title: "Find…", action: MainMenu.find, keyEquivalent: "f")
        XCTAssertTrue(surface.validateMenuItem(undo))
        XCTAssertEqual(undo.title, "Undo: Typing")
        XCTAssertFalse(surface.validateMenuItem(redo))
        XCTAssertFalse(surface.validateMenuItem(find))
        surface.undo(nil)
        XCTAssertEqual(edits, ["undo"])
        surface.desktopEditing = TTEditing(undo: "Complete task", redo: "", search: true)
        XCTAssertTrue(surface.validateMenuItem(undo))
        XCTAssertEqual(undo.title, "Undo: Complete task")
        XCTAssertTrue(surface.validateMenuItem(find))
        surface.desktopEditing = nil
        XCTAssertFalse(surface.validateMenuItem(undo))
        XCTAssertEqual(undo.title, "Undo")
    }

    func testOnlyAPlainReturnOrEnterRelaunches() {
        XCTAssertTrue(TerminalWindowController.isRelaunchKey("\r", []))
        XCTAssertTrue(TerminalWindowController.isRelaunchKey("\u{3}", [.numericPad, .function]))
        XCTAssertFalse(TerminalWindowController.isRelaunchKey("\r", [.command]))
        XCTAssertFalse(TerminalWindowController.isRelaunchKey("q", []))
    }

    func testKillingTTShowsTheRelaunchAffordance() throws {
        try start()
        kill(try fakePID(), SIGKILL)
        try wait("the failure banner") { controller.isShowingFailure }
        guard case .showFailure(let summary) = controller.exitAction else {
            return XCTFail("expected a failure, got \(String(describing: controller.exitAction))")
        }
        XCTAssertTrue(summary.contains("signal 9"), summary)
    }
}
